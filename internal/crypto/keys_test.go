package crypto

import (
	"bytes"
	"os"
	"testing"
)

func TestGenerateECDSAKeyPair(t *testing.T) {
	privKey, err := GenerateECDSAKeyPair()
	if err != nil {
		t.Fatalf("GenerateECDSAKeyPair() error = %v", err)
	}
	if privKey == nil {
		t.Fatalf("GenerateECDSAKeyPair() private key is nil")
	}
	if privKey.PublicKey.X == nil || privKey.PublicKey.Y == nil {
		t.Fatalf("GenerateECDSAKeyPair() public key coordinates are nil")
	}
	// Check curve is P256
	if privKey.PublicKey.Curve.Params().Name != "P-256" {
		t.Errorf("Expected P256 curve, got %s", privKey.PublicKey.Curve.Params().Name)
	}
}

func TestPublicKeySerializationDeserialization(t *testing.T) {
	privKey, _ := GenerateECDSAKeyPair()
	pubKey := &privKey.PublicKey

	serialized := SerializePublicKeyToBytes(pubKey)
	if serialized == nil {
		t.Fatalf("SerializePublicKeyToBytes() returned nil")
	}

	deserializedPubKey, err := DeserializePublicKeyFromBytes(serialized)
	if err != nil {
		t.Fatalf("DeserializePublicKeyFromBytes() error = %v", err)
	}

	if !bytes.Equal(SerializePublicKeyToBytes(pubKey), SerializePublicKeyToBytes(deserializedPubKey)) {
		t.Errorf("Original and deserialized public keys do not match")
	}
	if pubKey.X.Cmp(deserializedPubKey.X) != 0 || pubKey.Y.Cmp(deserializedPubKey.Y) != 0 {
		t.Errorf("Original and deserialized public key coordinates do not match")
	}
}

func TestAddressConversion(t *testing.T) {
	privKey, _ := GenerateECDSAKeyPair()
	pubKeyBytes := SerializePublicKeyToBytes(&privKey.PublicKey)

	address := PublicKeyBytesToAddress(pubKeyBytes)
	if address == "" {
		t.Fatalf("PublicKeyBytesToAddress() returned empty string")
	}
	if len(address) != 130 { // 04 + 32 bytes for X + 32 bytes for Y, hex encoded = 2 + 64 + 64
		t.Errorf("Expected address length 130, got %d", len(address))
	}


	retrievedPubKeyBytes, err := AddressToPublicKeyBytes(address)
	if err != nil {
		t.Fatalf("AddressToPublicKeyBytes() error = %v", err)
	}
	if !bytes.Equal(pubKeyBytes, retrievedPubKeyBytes) {
		t.Errorf("Original public key bytes and bytes from address do not match")
	}
}

func TestPrivateKeyPEMSerialization(t *testing.T) {
	privKey, _ := GenerateECDSAKeyPair()

	// Test without password
	pemBytes, err := SerializePrivateKeyToPEM(privKey, nil) // Pass nil for no password
	if err != nil {
		t.Fatalf("SerializePrivateKeyToPEM() without password error = %v", err)
	}
	deserializedPrivKey, err := DeserializePrivateKeyFromPEM(pemBytes, nil) // Pass nil for no password
	if err != nil {
		t.Fatalf("DeserializePrivateKeyFromPEM() without password error = %v", err)
	}
	if privKey.D.Cmp(deserializedPrivKey.D) != 0 ||
		privKey.PublicKey.X.Cmp(deserializedPrivKey.PublicKey.X) != 0 ||
		privKey.PublicKey.Y.Cmp(deserializedPrivKey.PublicKey.Y) != 0 {
		t.Errorf("Original and deserialized private keys (no password) do not match")
	}

	// Test with password: Current Go SerializePrivateKeyToPEM ignores the password and saves unencrypted.
	// So, deserializing with or without the password (or a wrong password) should still succeed
	// because the key is not actually encrypted in the PEM data from Go's serialization.
	password := "testpassword"
	pemBytesWithPwdIgnored, err := SerializePrivateKeyToPEM(privKey, []byte(password))
	if err != nil {
		t.Fatalf("SerializePrivateKeyToPEM() with password (but ignored) error = %v", err)
	}

	// Attempt to deserialize with correct password (should work, as it's unencrypted)
	deserializedPrivKeyPwd, err := DeserializePrivateKeyFromPEM(pemBytesWithPwdIgnored, []byte(password))
	if err != nil {
		t.Fatalf("DeserializePrivateKeyFromPEM() with password (for unencrypted key) error = %v", err)
	}
	if privKey.D.Cmp(deserializedPrivKeyPwd.D) != 0 {
		t.Errorf("Deserialized key (with password, from unencrypted PEM) does not match original")
	}

	// Attempt to deserialize with wrong password (should still work, as it's unencrypted)
	deserializedPrivKeyWrongPwd, err := DeserializePrivateKeyFromPEM(pemBytesWithPwdIgnored, []byte("wrongpassword"))
	if err != nil {
		t.Fatalf("DeserializePrivateKeyFromPEM() with wrong password (for unencrypted key) error = %v", err)
	}
	if privKey.D.Cmp(deserializedPrivKeyWrongPwd.D) != 0 {
		t.Errorf("Deserialized key (with wrong password, from unencrypted PEM) does not match original")
	}

	// Attempt to deserialize with nil password (should still work)
	deserializedPrivKeyNilPwd, err := DeserializePrivateKeyFromPEM(pemBytesWithPwdIgnored, nil)
	if err != nil {
		t.Fatalf("DeserializePrivateKeyFromPEM() with nil password (for unencrypted key) error = %v", err)
	}
	if privKey.D.Cmp(deserializedPrivKeyNilPwd.D) != 0 {
		t.Errorf("Deserialized key (with nil password, from unencrypted PEM) does not match original")
	}
}

func TestPublicKeyPEMSerialization(t *testing.T) {
	privKey, _ := GenerateECDSAKeyPair()
	pubKey := &privKey.PublicKey

	pemBytes, err := SerializePublicKeyToPEM(pubKey)
	if err != nil {
		t.Fatalf("SerializePublicKeyToPEM() error = %v", err)
	}
	deserializedPubKey, err := DeserializePublicKeyFromPEM(pemBytes)
	if err != nil {
		t.Fatalf("DeserializePublicKeyFromPEM() error = %v", err)
	}
	if pubKey.X.Cmp(deserializedPubKey.X) != 0 || pubKey.Y.Cmp(deserializedPubKey.Y) != 0 {
		t.Errorf("Original and deserialized public keys (PEM) do not match")
	}
}

func TestFileOperations(t *testing.T) {
	privKey, _ := GenerateECDSAKeyPair()
	pubKey := &privKey.PublicKey
	password := "fileopstest"
	privKeyFile := "test_priv.pem"
	pubKeyFile := "test_pub.pem"

	defer os.Remove(privKeyFile)
	defer os.Remove(pubKeyFile)

	// Test Save/Load Private Key (unencrypted, password nil for save)
	err := SavePrivateKeyPEM(privKey, privKeyFile, nil)
	if err != nil {
		t.Fatalf("SavePrivateKeyPEM() without password error = %v", err)
	}
	loadedPrivKey, err := LoadPrivateKeyPEM(privKeyFile, nil)
	if err != nil {
		t.Fatalf("LoadPrivateKeyPEM() without password error = %v", err)
	}
	if privKey.D.Cmp(loadedPrivKey.D) != 0 {
		t.Errorf("Saved and loaded unencrypted private keys (nil password) do not match")
	}
	os.Remove(privKeyFile)

	// Test Save/Load Private Key (unencrypted, password provided to save but ignored by Go's save)
	err = SavePrivateKeyPEM(privKey, privKeyFile, []byte(password))
	if err != nil {
		t.Fatalf("SavePrivateKeyPEM() with password (but ignored) error = %v", err)
	}
	// Load with correct, wrong, and nil password - all should work as it's saved unencrypted
	loadedPrivKeyCorrectPwd, err := LoadPrivateKeyPEM(privKeyFile, []byte(password))
	if err != nil {
		t.Fatalf("LoadPrivateKeyPEM() with correct password (for unencrypted file) error = %v", err)
	}
	if privKey.D.Cmp(loadedPrivKeyCorrectPwd.D) != 0 {
		t.Errorf("Saved (pwd ignored) and loaded (correct pwd) private keys do not match")
	}

	loadedPrivKeyWrongPwd, err := LoadPrivateKeyPEM(privKeyFile, []byte("stillwrong"))
	if err != nil {
		t.Fatalf("LoadPrivateKeyPEM() with wrong password (for unencrypted file) error = %v", err)
	}
	if privKey.D.Cmp(loadedPrivKeyWrongPwd.D) != 0 {
		t.Errorf("Saved (pwd ignored) and loaded (wrong pwd) private keys do not match")
	}

	loadedPrivKeyNilPwd, err := LoadPrivateKeyPEM(privKeyFile, nil)
	if err != nil {
		t.Fatalf("LoadPrivateKeyPEM() with nil password (for unencrypted file) error = %v", err)
	}
	if privKey.D.Cmp(loadedPrivKeyNilPwd.D) != 0 {
		t.Errorf("Saved (pwd ignored) and loaded (nil pwd) private keys do not match")
	}


	// Test Save/Load Public Key
	err = SavePublicKeyPEM(pubKey, pubKeyFile)
	if err != nil {
		t.Fatalf("SavePublicKeyPEM() error = %v", err)
	}
	loadedPubKey, err := LoadPublicKeyPEM(pubKeyFile)
	if err != nil {
		t.Fatalf("LoadPublicKeyPEM() error = %v", err)
	}
	if pubKey.X.Cmp(loadedPubKey.X) != 0 || pubKey.Y.Cmp(loadedPubKey.Y) != 0 {
		t.Errorf("Saved and loaded public keys do not match")
	}
}
