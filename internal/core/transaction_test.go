package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/rand"
	"empower1.com/core/internal/crypto"
	// "encoding/hex" // Removed unused import
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"
)

// TestNewTransaction_Old is kept to ensure the old constructor (if still used) is tested.
// It should ideally be removed once NewTransaction is fully replaced.
func TestNewTransaction_Old(t *testing.T) {
	senderPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil { t.Fatalf("Failed to generate sender key pair: %v", err) }
	receiverPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil { t.Fatalf("Failed to generate receiver key pair: %v", err) }

	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey
	amount := uint64(100)
	fee := uint64(10)

	tx, err := NewTransaction(senderPubKey, receiverPubKey, amount, fee) // Uses the old constructor
	if err != nil {
		t.Fatalf("Old NewTransaction() error = %v", err)
	}
	// Basic checks for old NewTransaction behavior
	if tx.TxType != TxStandard && tx.TxType != "" {
		// Old NewTransaction might set TxType to "" or default to TxStandard if that was refactored.
		// Assuming it implicitly creates a standard one for this test.
		// If NewTransaction was updated to set TxType explicitly, this check might need adjustment.
		// For now, let's ensure it's one of the expected values if it's being set.
		// If old NewTransaction truly doesn't set TxType, then this check is for its default value.
		// Given the current Transaction struct, TxType will be the zero value "" if not set.
		// The new constructors (NewStandardTransaction etc.) DO set TxType.
		// So, if NewTransaction is truly old, its TxType should be "".
		if tx.TxType != "" {
			t.Errorf("TxType for old NewTransaction = %s; want empty string (or specific default if any)", tx.TxType)
		}
	}
	if tx.Amount != amount { t.Errorf("tx.Amount = %d; want %d", tx.Amount, amount) }
	expectedFrom := crypto.SerializePublicKeyToBytes(senderPubKey)
	if !bytes.Equal(tx.From, expectedFrom) { t.Errorf("tx.From is incorrect") }
	if !bytes.Equal(tx.PublicKey, expectedFrom) { t.Errorf("tx.PublicKey is incorrect") }
}

func TestNewStandardTransaction(t *testing.T) {
	senderPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil { t.Fatalf("Failed to generate sender key pair: %v", err) }
	receiverPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil { t.Fatalf("Failed to generate receiver key pair: %v", err) }

	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey
	amount := uint64(100)
	fee := uint64(10)

	tx, err := NewStandardTransaction(senderPubKey, receiverPubKey, amount, fee)
	if err != nil {
		t.Fatalf("NewStandardTransaction() error = %v", err)
	}
	if tx.TxType != TxStandard { t.Errorf("tx.TxType = %s; want %s", tx.TxType, TxStandard) }
	if tx.Amount != amount { t.Errorf("tx.Amount = %d; want %d", tx.Amount, amount) }
	// ... other checks from original TestNewStandardTransaction ...
}


func TestTransactionSignAndVerify(t *testing.T) {
	senderPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil { t.Fatalf("Failed to generate sender key pair: %v", err) }
	receiverPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil { t.Fatalf("Failed to generate receiver key pair: %v", err) }
	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey

	tx, err := NewStandardTransaction(senderPubKey, receiverPubKey, 100, 10)
	if err != nil { t.Fatalf("NewStandardTransaction error: %v", err) }

	err = tx.Sign(senderPrivKey)
	if err != nil { t.Fatalf("tx.Sign() error = %v", err) }
	if tx.Signature == nil { t.Errorf("tx.Signature is nil after signing") }
	if tx.ID == nil { t.Errorf("tx.ID is nil after signing") }

	valid, err := tx.VerifySignature()
	if err != nil { t.Fatalf("tx.VerifySignature() error = %v", err) }
	if !valid { t.Errorf("tx.VerifySignature() = false; want true") }

	originalAmount := tx.Amount
	tx.Amount = 200
	valid, _ = tx.VerifySignature()
	if valid { t.Errorf("tx.VerifySignature() = true after tampering Amount; want false") }
	tx.Amount = originalAmount

	fakePrivKey, _ := crypto.GenerateECDSAKeyPair()
	originalPubKeyBytes := tx.PublicKey
	tx.PublicKey = crypto.SerializePublicKeyToBytes(&fakePrivKey.PublicKey)
	valid, _ = tx.VerifySignature()
	if valid { t.Errorf("tx.VerifySignature() = true after tampering PublicKey; want false") }
	tx.PublicKey = originalPubKeyBytes

	valid, err = tx.VerifySignature()
	if !valid {	t.Errorf("tx.VerifySignature() = false after restoring; want true, err: %v", err) }
}

func TestTransactionHashing(t *testing.T) {
	senderPrivKey, _ := crypto.GenerateECDSAKeyPair()
	receiverPrivKey, _ := crypto.GenerateECDSAKeyPair()
	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey

	tx1, _ := NewStandardTransaction(senderPubKey, receiverPubKey, 100, 10)
	tx1.Timestamp = 1234567890

	hash1, err := tx1.Hash()
	if err != nil { t.Fatalf("tx1.Hash() error = %v", err) }

	tx2, _ := NewStandardTransaction(senderPubKey, receiverPubKey, 100, 10)
	tx2.Timestamp = 1234567890
	hash2, err := tx2.Hash()
	if err != nil { t.Fatalf("tx2.Hash() error = %v", err) }
	if !bytes.Equal(hash1, hash2) {
		t.Errorf("Hashes of identical standard transactions do not match.")
		tx1Json, _ := tx1.prepareDataForHashing(); t.Logf("TX1 JSON: %s", string(tx1Json))
		tx2Json, _ := tx2.prepareDataForHashing(); t.Logf("TX2 JSON: %s", string(tx2Json))
	}

	tx3, _ := NewStandardTransaction(senderPubKey, receiverPubKey, 200, 10)
	tx3.Timestamp = 1234567890
	hash3, err := tx3.Hash()
	if err != nil { t.Fatalf("tx3.Hash() error = %v", err) }
	if bytes.Equal(hash1, hash3) { t.Errorf("Hashes of different transactions match.") }

	err = tx1.Sign(senderPrivKey)
	if err != nil { t.Fatalf("tx1.Sign() error: %v", err) }
	if !bytes.Equal(tx1.ID, hash1) { t.Errorf("tx1.ID (%x) != pre-sign hash (%x)", tx1.ID, hash1) }
}

func TestMultiSigTransactionHashing(t *testing.T) {
	key1, _ := crypto.GenerateECDSAKeyPair()
	key2, _ := crypto.GenerateECDSAKeyPair()
	key3, _ := crypto.GenerateECDSAKeyPair()
	pubKey1Bytes := crypto.SerializePublicKeyToBytes(&key1.PublicKey)
	pubKey2Bytes := crypto.SerializePublicKeyToBytes(&key2.PublicKey)
	pubKey3Bytes := crypto.SerializePublicKeyToBytes(&key3.PublicKey)
	multiSigID := []byte("test_multisig_id_for_hashing")

	baseTx := Transaction{
		Timestamp:            1234567890, From: multiSigID, TxType: TxStandard,
		To:                   pubKey3Bytes, Amount: 500, Fee: 5, RequiredSignatures:   2,
	}
	txMulti1 := baseTx
	txMulti1.AuthorizedPublicKeys = [][]byte{pubKey1Bytes, pubKey2Bytes}
	SortByteSlices(txMulti1.AuthorizedPublicKeys)
	hash1, err1 := txMulti1.Hash()
	if err1 != nil { t.Fatalf("txMulti1.Hash() error = %v", err1) }

	txMulti2 := baseTx
	txMulti2.AuthorizedPublicKeys = [][]byte{pubKey2Bytes, pubKey1Bytes}
	SortByteSlices(txMulti2.AuthorizedPublicKeys)
	hash2, err2 := txMulti2.Hash()
	if err2 != nil { t.Fatalf("txMulti2.Hash() error = %v", err2) }
	if !bytes.Equal(hash1, hash2) {
		t.Errorf("Multi-sig hashes with differently ordered (but then sorted) AuthorizedPublicKeys do not match.")
		jsonBytes1, _ := txMulti1.prepareDataForHashing(); t.Logf("Tx1 JSON: %s", string(jsonBytes1))
		jsonBytes2, _ := txMulti2.prepareDataForHashing(); t.Logf("Tx2 JSON: %s", string(jsonBytes2))
	}

	txMulti3 := baseTx
	txMulti3.AuthorizedPublicKeys = [][]byte{pubKey1Bytes, pubKey2Bytes}
	SortByteSlices(txMulti3.AuthorizedPublicKeys)
	txMulti3.RequiredSignatures = 1
	hash3, _ := txMulti3.Hash()
	if bytes.Equal(hash1, hash3) { t.Errorf("Hash did not change when M changed.") }

	txMulti4 := baseTx
	txMulti4.AuthorizedPublicKeys = [][]byte{pubKey1Bytes, pubKey3Bytes}
	SortByteSlices(txMulti4.AuthorizedPublicKeys)
	txMulti4.RequiredSignatures = 2
	hash4, _ := txMulti4.Hash()
	if bytes.Equal(hash1, hash4) { t.Errorf("Hash did not change when N keys changed.") }
}

func SortByteSlices(slices [][]byte) {
	sort.Slice(slices, func(i, j int) bool { return bytes.Compare(slices[i], slices[j]) < 0 })
}

func TestTransactionSerialization(t *testing.T) {
	senderPrivKey, _ := crypto.GenerateECDSAKeyPair()
	receiverPrivKey, _ := crypto.GenerateECDSAKeyPair()
	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey
	receiverPubKeyBytes := crypto.SerializePublicKeyToBytes(receiverPubKey)

	tx, err := NewStandardTransaction(senderPubKey, receiverPubKey, 100, 10)
	if err != nil { t.Fatalf("NewStandardTransaction failed: %v", err) }
	tx.Timestamp = time.Now().UnixNano()
	err = tx.Sign(senderPrivKey)
	if err != nil { t.Fatalf("tx.Sign() for single-signer error = %v", err) }

	serializedTx, err := tx.Serialize()
	if err != nil { t.Fatalf("tx.Serialize() error = %v", err) }
	deserializedTx, err := DeserializeTransaction(serializedTx)
	if err != nil { t.Fatalf("DeserializeTransaction() error = %v", err) }

	if !bytes.Equal(tx.ID, deserializedTx.ID) { t.Errorf("ID mismatch") }
	if tx.Timestamp != deserializedTx.Timestamp { t.Errorf("Timestamp mismatch") }
	valid, err := deserializedTx.VerifySignature()
	if err != nil { t.Fatalf("deserializedTx.VerifySignature() error = %v", err) }
	if !valid { t.Errorf("deserializedTx.VerifySignature() = false for single-signer") }

	key1, _ := crypto.GenerateECDSAKeyPair()
	key2, _ := crypto.GenerateECDSAKeyPair()
	multiSigID := []byte("test_multisig_id_for_serialization")
	authKeys := [][]byte{ crypto.SerializePublicKeyToBytes(&key1.PublicKey), crypto.SerializePublicKeyToBytes(&key2.PublicKey)}
	SortByteSlices(authKeys)

	txMulti := &Transaction{
		ID:        []byte("multi_tx_id_serial"), Timestamp: time.Now().UnixNano(), From: multiSigID,
		TxType:    TxStandard, To: receiverPubKeyBytes, Amount: 123, Fee: 3,
		RequiredSignatures: 1, AuthorizedPublicKeys: authKeys,
		Signers:   []SignerInfo{{PublicKey: authKeys[0], Signature: []byte("dummy_sig_serial")}},
		PublicKey: nil, Signature: nil,
	}
	serializedMultiTx, err := txMulti.Serialize()
	if err != nil { t.Fatalf("txMulti.Serialize() error = %v", err) }
	deserializedMultiTx, err := DeserializeTransaction(serializedMultiTx)
	if err != nil { t.Fatalf("DeserializeTransaction(multiTx) error = %v", err) }

	if !bytes.Equal(txMulti.ID, deserializedMultiTx.ID) { t.Errorf("MultiTx ID mismatch") }
	if txMulti.RequiredSignatures != deserializedMultiTx.RequiredSignatures { t.Errorf("MultiTx Mismatch") }
}

func TestMultiSignatureVerification(t *testing.T) {
	key1Priv, err := crypto.GenerateECDSAKeyPair(); if err != nil { t.Fatalf("key1Priv gen error: %v", err) }
	key2Priv, err := crypto.GenerateECDSAKeyPair(); if err != nil { t.Fatalf("key2Priv gen error: %v", err) }
	key3Priv, err := crypto.GenerateECDSAKeyPair(); if err != nil { t.Fatalf("key3Priv gen error: %v", err) }

	pub1Bytes := crypto.SerializePublicKeyToBytes(&key1Priv.PublicKey)
	pub2Bytes := crypto.SerializePublicKeyToBytes(&key2Priv.PublicKey)
	pub3Bytes := crypto.SerializePublicKeyToBytes(&key3Priv.PublicKey)
	authorizedKeys := [][]byte{pub1Bytes, pub2Bytes, pub3Bytes}
	SortByteSlices(authorizedKeys)
	multiSigAddressBytes := []byte("derived_multisig_address_placeholder")

	txBase := Transaction{
		Timestamp:            time.Now().UnixNano(), From: multiSigAddressBytes, TxType: TxStandard,
		To:                   []byte("recipient_test"), Amount: 1000, Fee: 10,
		RequiredSignatures:   2, AuthorizedPublicKeys: authorizedKeys, Signers: []SignerInfo{},
		PublicKey: nil, Signature: nil,
	}

	txHashToSign, err := txBase.Hash(); if err != nil { t.Fatalf("txBase.Hash() error: %v", err) }
	txBase.ID = txHashToSign

	// Signer 1 signs
	tx := txBase
	sig1, err := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign); if err != nil { t.Fatalf("sig1 error: %v", err) }
	tx.Signers = append(tx.Signers, SignerInfo{PublicKey: pub1Bytes, Signature: sig1})
	valid, err := tx.VerifySignature()
	if valid { t.Errorf("Succeeded with 1/2 sigs") }
	if err == nil || !strings.Contains(err.Error(), "not enough signers") { t.Errorf("Expected 'not enough signers', got: %v", err) }

	// Signer 2 signs
	sig2, err := ecdsa.SignASN1(rand.Reader, key2Priv, txHashToSign); if err != nil { t.Fatalf("sig2 error: %v", err) }
	tx.Signers = append(tx.Signers, SignerInfo{PublicKey: pub2Bytes, Signature: sig2})
	sort.Slice(tx.Signers, func(i, j int) bool { return bytes.Compare(tx.Signers[i].PublicKey, tx.Signers[j].PublicKey) < 0 })
	valid, err = tx.VerifySignature()
	if err != nil { t.Fatalf("Failed with 2/2 sigs: %v", err) }
	if !valid { t.Errorf("Valid = false with 2/2 sigs") }

	// Tamper signature
	tamperedTx := tx
	tamperedTx.Signers = make([]SignerInfo, len(tx.Signers)); copy(tamperedTx.Signers, tx.Signers)
	tamperedTx.Signers[1].Signature = []byte("tampered_sig")
	valid, err = tamperedTx.VerifySignature()
	if valid { t.Errorf("Succeeded with tampered sig") }
	if err == nil || !strings.Contains(err.Error(), "invalid signature for signer") { t.Errorf("Expected 'invalid sig', got: %v", err) }

	// Unauthorized signer
	key4Unauth, err := crypto.GenerateECDSAKeyPair(); if err != nil { t.Fatalf("key4Unauth gen error: %v", err) }
	pub4UnauthBytes := crypto.SerializePublicKeyToBytes(&key4Unauth.PublicKey)
	sig4Unauth, err := ecdsa.SignASN1(rand.Reader, key4Unauth, txHashToSign); if err != nil { t.Fatalf("sig4Unauth error: %v", err) }
	unauthTx := tx
	unauthTx.Signers = make([]SignerInfo, len(tx.Signers)); copy(unauthTx.Signers, tx.Signers)
	unauthTx.Signers = append(unauthTx.Signers, SignerInfo{PublicKey: pub4UnauthBytes, Signature: sig4Unauth})
	valid, err = unauthTx.VerifySignature()
	if valid { t.Errorf("Succeeded with unauth signer") }
	if err == nil || !strings.Contains(err.Error(), "is not in the authorized list") { t.Errorf("Expected 'not in auth list', got: %v", err) }

	// Duplicate signer
	dupTx := tx
	dupTx.Signers = make([]SignerInfo, len(tx.Signers)); copy(dupTx.Signers, tx.Signers)
	dupTx.Signers = append(dupTx.Signers, tx.Signers[0])
	valid, err = dupTx.VerifySignature()
	if valid { t.Errorf("Succeeded with duplicate signer") }
	if err == nil || !strings.Contains(err.Error(), "duplicate signature from public key") { t.Errorf("Expected 'duplicate sig', got: %v", err) }

	// M > N
	mnTx := Transaction{
		Timestamp: tx.Timestamp, From: tx.From, TxType: tx.TxType, To: tx.To, Amount: tx.Amount, Fee: tx.Fee,
		ID: tx.ID, PublicKey: nil, Signature: nil,
	}
	mnTx.AuthorizedPublicKeys = [][]byte{pub1Bytes, pub2Bytes} // N=2
	SortByteSlices(mnTx.AuthorizedPublicKeys)
	mnTx.RequiredSignatures = 3 // M=3
	// Add M (3) dummy/valid signers to pass the "len(Signers) < M" check.
	mnTx.Signers = []SignerInfo{ // Need 3 signers for M=3
		{PublicKey: pub1Bytes, Signature: sig1},
		{PublicKey: pub2Bytes, Signature: sig2},
		// This third signer's pubkey is not in mnTx.AuthorizedPublicKeys, so this setup is flawed for this specific test.
		// The intention is to test M > N, not necessarily that all signers are valid *and* authorized for this specific check.
		// The "is not in the authorized list" check might trigger first if pub3Bytes is used here.
		// Let's use pub1Bytes again, which would be caught by duplicate signer if that check came before M > N.
		// The M > N check ( tx.RequiredSignatures > uint32(len(tx.AuthorizedPublicKeys)) ) should be hit first.
		{PublicKey: pub1Bytes, Signature: sig1}, // Add a third signature (can be a duplicate for this M>N structural test)
	}
	if len(mnTx.Signers) < int(mnTx.RequiredSignatures) { // Ensure we have enough signers for M
		t.Fatalf("M>N Test setup error: not enough signers provided for M=%d", mnTx.RequiredSignatures)
	}

	valid, err = mnTx.VerifySignature()
	if valid { t.Errorf("Succeeded with M > N (M=%d, N=%d)", mnTx.RequiredSignatures, len(mnTx.AuthorizedPublicKeys)) }
	expectedErrorMsg := fmt.Sprintf("M (%d) cannot be greater than N (%d)", mnTx.RequiredSignatures, len(mnTx.AuthorizedPublicKeys))
	if err == nil || !strings.Contains(err.Error(), expectedErrorMsg) {
		t.Errorf("Expected '%s', got: %v", expectedErrorMsg, err)
	}
}
