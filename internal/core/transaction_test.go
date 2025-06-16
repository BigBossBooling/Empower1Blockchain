package core

import (
	"bytes"
	"empower1.com/core/internal/crypto" // Assuming your keys.go is in internal/crypto
	"testing"
	"time"
	"encoding/hex"
)

func TestNewTransaction(t *testing.T) {
	// Generate sender and receiver key pairs using your crypto package
	senderPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}
	receiverPrivKey, err := crypto.GenerateECDSAKeyPair() // Just for a valid public key
	if err != nil {
		t.Fatalf("Failed to generate receiver key pair: %v", err)
	}

	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey

	amount := uint64(100)
	fee := uint64(10)

	tx, err := NewTransaction(senderPubKey, receiverPubKey, amount, fee)
	if err != nil {
		t.Fatalf("NewTransaction() error = %v", err)
	}

	if tx.Amount != amount {
		t.Errorf("tx.Amount = %d; want %d", tx.Amount, amount)
	}
	if tx.Fee != fee {
		t.Errorf("tx.Fee = %d; want %d", tx.Fee, fee)
	}
	if tx.Timestamp == 0 {
		t.Errorf("tx.Timestamp not set")
	}

	expectedFrom := crypto.SerializePublicKeyToBytes(senderPubKey)
	if !bytes.Equal(tx.From, expectedFrom) {
		t.Errorf("tx.From is incorrect")
	}
	if !bytes.Equal(tx.PublicKey, expectedFrom) { // PublicKey should be sender's
		t.Errorf("tx.PublicKey is incorrect, should be sender's")
	}

	expectedTo := crypto.SerializePublicKeyToBytes(receiverPubKey)
	if !bytes.Equal(tx.To, expectedTo) {
		t.Errorf("tx.To is incorrect")
	}
}

func TestTransactionSignAndVerify(t *testing.T) {
	senderPrivKey, _ := crypto.GenerateECDSAKeyPair()
	receiverPrivKey, _ := crypto.GenerateECDSAKeyPair()
	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey

	tx, _ := NewTransaction(senderPubKey, receiverPubKey, 100, 10)

	// Sign the transaction
	err := tx.Sign(senderPrivKey)
	if err != nil {
		t.Fatalf("tx.Sign() error = %v", err)
	}

	if tx.Signature == nil {
		t.Errorf("tx.Signature is nil after signing")
	}
	if tx.ID == nil { // ID should be set after signing (it's the hash of content)
		t.Errorf("tx.ID is nil after signing")
	}

	// Verify the signature
	valid, err := tx.VerifySignature()
	if err != nil {
		t.Fatalf("tx.VerifySignature() error = %v", err)
	}
	if !valid {
		t.Errorf("tx.VerifySignature() = false; want true")
	}

	// Tamper with the transaction and expect verification to fail
	originalAmount := tx.Amount
	tx.Amount = 200
	valid, err = tx.VerifySignature()
	if err != nil {
		// This might error if Hash() fails due to modification, or Verify simply returns false
		// t.Logf("tx.VerifySignature() after tampering (Amount) error = %v (expected if hash changes)", err)
	}
	if valid {
		t.Errorf("tx.VerifySignature() = true after tampering Amount; want false")
	}
	tx.Amount = originalAmount // Restore

	// Tamper with PublicKey
	fakePrivKey, _ := crypto.GenerateECDSAKeyPair()
	originalPubKeyBytes := tx.PublicKey
	tx.PublicKey = crypto.SerializePublicKeyToBytes(&fakePrivKey.PublicKey)
	valid, err = tx.VerifySignature()
	if valid {
		t.Errorf("tx.VerifySignature() = true after tampering PublicKey; want false")
	}
	tx.PublicKey = originalPubKeyBytes // Restore

	// Ensure original signature is still valid after restoring
	valid, err = tx.VerifySignature()
	if !valid {
		t.Errorf("tx.VerifySignature() = false after restoring; want true, err: %v", err)
	}
}

func TestTransactionHashing(t *testing.T) {
	senderPrivKey, _ := crypto.GenerateECDSAKeyPair()
	receiverPrivKey, _ := crypto.GenerateECDSAKeyPair()
	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey

	tx1, _ := NewTransaction(senderPubKey, receiverPubKey, 100, 10)
	tx1.Timestamp = 1234567890 // Fixed timestamp for consistent hashing

	hash1, err := tx1.Hash()
	if err != nil {
		t.Fatalf("tx1.Hash() error = %v", err)
	}
	if len(hash1) == 0 {
		t.Errorf("tx1.Hash() returned empty hash")
	}

	// Create another transaction with same data, should have same hash
	tx2, _ := NewTransaction(senderPubKey, receiverPubKey, 100, 10)
	tx2.Timestamp = 1234567890

	hash2, err := tx2.Hash()
	if err != nil {
		t.Fatalf("tx2.Hash() error = %v", err)
	}
	if !bytes.Equal(hash1, hash2) {
		t.Errorf("Hashes of identical transactions do not match. Hash1: %x, Hash2: %x", hash1, hash2)
		// For debugging the JSON:
		tx1Json, _ := tx1.prepareDataForHashing()
		tx2Json, _ := tx2.prepareDataForHashing()
		t.Logf("TX1 JSON for hash: %s", string(tx1Json))
		t.Logf("TX2 JSON for hash: %s", string(tx2Json))

	}

	// Change some data and expect a different hash
	tx3, _ := NewTransaction(senderPubKey, receiverPubKey, 200, 10) // Different amount
	tx3.Timestamp = 1234567890
	hash3, err := tx3.Hash()
	if err != nil {
		t.Fatalf("tx3.Hash() error = %v", err)
	}
	if bytes.Equal(hash1, hash3) {
		t.Errorf("Hashes of different transactions match. Hash1: %x, Hash3: %x", hash1, hash3)
	}

	// Test that ID is set by Sign to the hash
	err = tx1.Sign(senderPrivKey)
	if err != nil { t.Fatalf("tx1.Sign() error: %v", err) }
	if !bytes.Equal(tx1.ID, hash1) {
		t.Errorf("tx1.ID (%x) does not match pre-sign hash (%x)", tx1.ID, hash1)
	}

	t.Logf("TestTransactionHashing: Hash1 (JSON based): %s", hex.EncodeToString(hash1))
}


func TestTransactionSerialization(t *testing.T) {
	senderPrivKey, _ := crypto.GenerateECDSAKeyPair()
	receiverPrivKey, _ := crypto.GenerateECDSAKeyPair()
	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey

	tx, _ := NewTransaction(senderPubKey, receiverPubKey, 100, 10)
	tx.Timestamp = time.Now().UnixNano() // Ensure timestamp is set
	err := tx.Sign(senderPrivKey)
	if err != nil {
		t.Fatalf("tx.Sign() error = %v", err)
	}

	serializedTx, err := tx.Serialize()
	if err != nil {
		t.Fatalf("tx.Serialize() error = %v", err)
	}
	if len(serializedTx) == 0 {
		t.Errorf("tx.Serialize() returned empty byte slice")
	}

	deserializedTx, err := DeserializeTransaction(serializedTx)
	if err != nil {
		t.Fatalf("DeserializeTransaction() error = %v", err)
	}

	if !bytes.Equal(tx.ID, deserializedTx.ID) {
		t.Errorf("ID mismatch: original %x, deserialized %x", tx.ID, deserializedTx.ID)
	}
	if tx.Timestamp != deserializedTx.Timestamp {
		t.Errorf("Timestamp mismatch: original %d, deserialized %d", tx.Timestamp, deserializedTx.Timestamp)
	}
	if !bytes.Equal(tx.From, deserializedTx.From) {
		t.Errorf("From mismatch")
	}
	if !bytes.Equal(tx.To, deserializedTx.To) {
		t.Errorf("To mismatch")
	}
	if tx.Amount != deserializedTx.Amount {
		t.Errorf("Amount mismatch")
	}
	if tx.Fee != deserializedTx.Fee {
		t.Errorf("Fee mismatch")
	}
	if !bytes.Equal(tx.Signature, deserializedTx.Signature) {
		t.Errorf("Signature mismatch")
	}
	if !bytes.Equal(tx.PublicKey, deserializedTx.PublicKey) {
		t.Errorf("PublicKey mismatch")
	}

	// Verify signature of deserialized transaction
	valid, err := deserializedTx.VerifySignature()
	if err != nil {
		t.Fatalf("deserializedTx.VerifySignature() error = %v", err)
	}
	if !valid {
		t.Errorf("deserializedTx.VerifySignature() = false; want true")
	}
}
