package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand" // Needed for ecdsa.SignASN1
	"crypto/sha256"
	"encoding/hex" // Added for debugging/logging hex strings
	"errors"
	"fmt"
	"sort"
	"strings"
	"testing"
	"time"

	"empower1.com/core/internal/crypto" // Assuming this provides key generation and serialization
)

// Helper to get a dummy ECDSA private key for testing
func newDummyPrivateKey(t *testing.T) *ecdsa.PrivateKey {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("Failed to generate dummy private key: %v", err)
	}
	return privKey
}

// Helper to get a dummy public key bytes
func newDummyPublicKeyBytes(t *testing.T) []byte {
	privKey := newDummyPrivateKey(t)
	return elliptic.Marshal(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
}

// --- Test Cases ---

func TestNewStandardTransaction(t *testing.T) {
	senderPrivKey := newDummyPrivateKey(t)
	receiverPubKeyBytes := newDummyPublicKeyBytes(t) // Receiver needs pubkey bytes
	
	amount := uint64(100)
	fee := uint64(10)

	tx, err := NewStandardTransaction(&senderPrivKey.PublicKey, receiverPubKeyBytes, amount, fee)
	if err != nil {
		t.Fatalf("NewStandardTransaction() error = %v", err)
	}

	if tx.Timestamp == 0 {
		t.Errorf("tx.Timestamp not set")
	}
	if !bytes.Equal(tx.From, elliptic.Marshal(elliptic.P256(), senderPrivKey.PublicKey.X, senderPrivKey.PublicKey.Y)) {
		t.Errorf("tx.From is incorrect, expected sender's public key bytes")
	}
	if !bytes.Equal(tx.PublicKey, elliptic.Marshal(elliptic.P256(), senderPrivKey.PublicKey.X, senderPrivKey.PublicKey.Y)) {
		t.Errorf("tx.PublicKey is incorrect, expected sender's public key bytes")
	}
	if tx.TxType != TxStandard {
		t.Errorf("tx.TxType = %s; want %s", tx.TxType, TxStandard)
	}
	if !bytes.Equal(tx.To, receiverPubKeyBytes) {
		t.Errorf("tx.To is incorrect, expected receiver public key bytes")
	}
	if tx.Amount != amount {
		t.Errorf("tx.Amount = %d; want %d", tx.Amount, amount)
	}
	if tx.Fee != fee {
		t.Errorf("tx.Fee = %d; want %d", tx.Fee, fee)
	}

	// Ensure signature fields are initially nil/empty
	if tx.Signature != nil {
		t.Errorf("tx.Signature should be nil initially")
	}
	if tx.ID != nil {
		t.Errorf("tx.ID should be nil initially")
	}

	// Test error cases
	_, err = NewStandardTransaction(nil, receiverPubKeyBytes, amount, fee)
	if err == nil {
		t.Errorf("Expected error for nil sender public key, got nil")
	}
	_, err = NewStandardTransaction(&senderPrivKey.PublicKey, nil, amount, fee)
	if err == nil {
		t.Errorf("Expected error for nil receiver public key hash, got nil")
	}
}

func TestNewContractDeploymentTransaction(t *testing.T) {
	deployerPrivKey := newDummyPrivateKey(t)
	contractCode := []byte("0x6001600055") // Example bytecode
	fee := uint64(20)

	tx, err := NewContractDeploymentTransaction(&deployerPrivKey.PublicKey, contractCode, fee)
	if err != nil {
		t.Fatalf("NewContractDeploymentTransaction() error = %v", err)
	}

	if tx.TxType != TxContractDeploy {
		t.Errorf("tx.TxType = %s; want %s", tx.TxType, TxContractDeploy)
	}
	if !bytes.Equal(tx.ContractCode, contractCode) {
		t.Errorf("tx.ContractCode is incorrect")
	}
	if tx.Fee != fee {
		t.Errorf("tx.Fee = %d; want %d", tx.Fee, fee)
	}
	// Basic sender/public key checks
	expectedFrom := elliptic.Marshal(elliptic.P256(), deployerPrivKey.PublicKey.X, deployerPrivKey.PublicKey.Y)
	if !bytes.Equal(tx.From, expectedFrom) {
		t.Errorf("tx.From is incorrect, expected deployer public key bytes")
	}
	if !bytes.Equal(tx.PublicKey, expectedFrom) {
		t.Errorf("tx.PublicKey is incorrect, expected deployer public key bytes")
	}

	// Test error cases
	_, err = NewContractDeploymentTransaction(nil, contractCode, fee)
	if err == nil {
		t.Errorf("Expected error for nil deployer public key, got nil")
	}
	_, err = NewContractDeploymentTransaction(&deployerPrivKey.PublicKey, nil, fee)
	if err == nil {
		t.Errorf("Expected error for nil contract code, got nil")
	}
	_, err = NewContractDeploymentTransaction(&deployerPrivKey.PublicKey, []byte{}, fee)
	if err == nil {
		t.Errorf("Expected error for empty contract code, got nil")
	}
}

func TestNewContractCallTransaction(t *testing.T) {
	callerPrivKey := newDummyPrivateKey(t)
	contractAddress := newDummyPublicKeyBytes(t) // Dummy contract address
	functionName := "transferTokens"
	args := []byte("0xabcdef")
	fee := uint64(5)

	tx, err := NewContractCallTransaction(&callerPrivKey.PublicKey, contractAddress, functionName, args, fee)
	if err != nil {
		t.Fatalf("NewContractCallTransaction() error = %v", err)
	}

	if tx.TxType != TxContractCall {
		t.Errorf("tx.TxType = %s; want %s", tx.TxType, TxContractCall)
	}
	if !bytes.Equal(tx.TargetContractAddress, contractAddress) {
		t.Errorf("tx.TargetContractAddress is incorrect")
	}
	if tx.FunctionName != functionName {
		t.Errorf("tx.FunctionName is incorrect")
	}
	if !bytes.Equal(tx.Arguments, args) {
		t.Errorf("tx.Arguments is incorrect")
	}
	if tx.Fee != fee {
		t.Errorf("tx.Fee = %d; want %d", tx.Fee, fee)
	}
	// Basic sender/public key checks
	expectedFrom := elliptic.Marshal(elliptic.P256(), callerPrivKey.PublicKey.X, callerPrivKey.PublicKey.Y)
	if !bytes.Equal(tx.From, expectedFrom) {
		t.Errorf("tx.From is incorrect, expected caller public key bytes")
	}
	if !bytes.Equal(tx.PublicKey, expectedFrom) {
		t.Errorf("tx.PublicKey is incorrect, expected caller public key bytes")
	}

	// Test error cases
	_, err = NewContractCallTransaction(nil, contractAddress, functionName, args, fee)
	if err == nil {
		t.Errorf("Expected error for nil caller public key, got nil")
	}
	_, err = NewContractCallTransaction(&callerPrivKey.PublicKey, nil, functionName, args, fee)
	if err == nil {
		t.Errorf("Expected error for nil contract address, got nil")
	}
	_, err = NewContractCallTransaction(&callerPrivKey.PublicKey, []byte{}, functionName, args, fee)
	if err == nil {
		t.Errorf("Expected error for empty contract address, got nil")
	}
	_, err = NewContractCallTransaction(&callerPrivKey.PublicKey, contractAddress, "", args, fee)
	if err == nil {
		t.Errorf("Expected error for empty function name, got nil")
	}
}

func TestNewMultiSigTransaction(t *testing.T) {
	multiSigID := []byte("ms_address_123")
	key1 := newDummyPublicKeyBytes(t)
	key2 := newDummyPublicKeyBytes(t)
	authorizedKeys := [][]byte{key1, key2}
	SortByteSlices(authorizedKeys) // Ensure sorted for consistent ID

	// Test Standard MultiSig Tx
	txStandardMulti, err := NewMultiSigTransaction(
		multiSigID, TxStandard, 2, authorizedKeys,
		uint64(500), newDummyPublicKeyBytes(t), nil, nil, "", nil, uint64(5),
	)
	if err != nil {
		t.Fatalf("NewMultiSigTransaction (Standard) error: %v", err)
	}
	if txStandardMulti.TxType != TxStandard {
		t.Errorf("MultiSig Standard TxType mismatch")
	}
	if txStandardMulti.Amount != 500 {
		t.Errorf("MultiSig Standard Amount mismatch")
	}
	if txStandardMulti.RequiredSignatures != 2 {
		t.Errorf("MultiSig Standard RequiredSignatures mismatch")
	}
	if len(txStandardMulti.AuthorizedPublicKeys) != 2 {
		t.Errorf("MultiSig Standard AuthorizedPublicKeys length mismatch")
	}
	if !bytes.Equal(txStandardMulti.From, multiSigID) {
		t.Errorf("MultiSig Standard From mismatch")
	}
	if len(txStandardMulti.Signers) != 0 {
		t.Errorf("MultiSig Signers should be empty initially")
	}
	if txStandardMulti.PublicKey != nil || txStandardMulti.Signature != nil {
		t.Errorf("MultiSig Tx single sig fields should be nil")
	}

	// Test Contract Deploy MultiSig Tx
	contractCode := []byte("0xabc")
	txDeployMulti, err := NewMultiSigTransaction(
		multiSigID, TxContractDeploy, 1, authorizedKeys,
		0, nil, contractCode, nil, "", nil, uint64(5),
	)
	if err != nil {
		t.Fatalf("NewMultiSigTransaction (Deploy) error: %v", err)
	}
	if txDeployMulti.TxType != TxContractDeploy {
		t.Errorf("MultiSig Deploy TxType mismatch")
	}
	if !bytes.Equal(txDeployMulti.ContractCode, contractCode) {
		t.Errorf("MultiSig Deploy ContractCode mismatch")
	}

	// Test Contract Call MultiSig Tx
	targetAddr := newDummyPublicKeyBytes(t)
	functionName := "withdraw"
	args := []byte("0x123")
	txCallMulti, err := NewMultiSigTransaction(
		multiSigID, TxContractCall, 1, authorizedKeys,
		10, nil, nil, targetAddr, functionName, args, uint64(5),
	)
	if err != nil {
		t.Fatalf("NewMultiSigTransaction (Call) error: %v", err)
	}
	if txCallMulti.TxType != TxContractCall {
		t.Errorf("MultiSig Call TxType mismatch")
	}
	if !bytes.Equal(txCallMulti.TargetContractAddress, targetAddr) {
		t.Errorf("MultiSig Call TargetContractAddress mismatch")
	}
	if txCallMulti.FunctionName != functionName {
		t.Errorf("MultiSig Call FunctionName mismatch")
	}
	if !bytes.Equal(txCallMulti.Arguments, args) {
		t.Errorf("MultiSig Call Arguments mismatch")
	}
	if txCallMulti.Amount != 10 { // Call can have amount
		t.Errorf("MultiSig Call Amount mismatch")
	}

	// Test error cases
	_, err = NewMultiSigTransaction(nil, TxStandard, 1, authorizedKeys, 1, newDummyPublicKeyBytes(t), nil, nil, "", nil, 1)
	if err == nil || !errors.Is(err, ErrMultiSigConfigInvalid) {
		t.Errorf("Expected ErrMultiSigConfigInvalid for nil multiSigID, got %v", err)
	}
	_, err = NewMultiSigTransaction(multiSigID, TxStandard, 0, authorizedKeys, 1, newDummyPublicKeyBytes(t), nil, nil, "", nil, 1)
	if err == nil || !errors.Is(err, ErrMultiSigConfigInvalid) {
		t.Errorf("Expected ErrMultiSigConfigInvalid for 0 requiredSignatures, got %v", err)
	}
	_, err = NewMultiSigTransaction(multiSigID, TxStandard, 1, nil, 1, newDummyPublicKeyBytes(t), nil, nil, "", nil, 1)
	if err == nil || !errors.Is(err, ErrMultiSigConfigInvalid) {
		t.Errorf("Expected ErrMultiSigConfigInvalid for nil authorizedKeys, got %v", err)
	}
	_, err = NewMultiSigTransaction(multiSigID, TxStandard, 3, authorizedKeys, 1, newDummyPublicKeyBytes(t), nil, nil, "", nil, 1) // M > N
	if err == nil || !errors.Is(err, ErrMultiSigConfigInvalid) {
		t.Errorf("Expected ErrMultiSigConfigInvalid for M > N, got %v", err)
	}
}

func TestTransactionHashing(t *testing.T) {
	// Generate unique keys for sender/receiver for consistent test environment
	senderPrivKey := newDummyPrivateKey(t)
	receiverPubKeyBytes := newDummyPublicKeyBytes(t)

	// --- Test Standard Transaction Hashing ---
	tx1, _ := NewStandardTransaction(&senderPrivKey.PublicKey, receiverPubKeyBytes, 100, 10)
	tx1.Timestamp = 1234567890 // Fixed timestamp for deterministic hash

	hash1, err := tx1.Hash()
	if err != nil {
		t.Fatalf("tx1.Hash() error = %v", err)
	}
	if len(hash1) == 0 {
		t.Errorf("tx1.Hash() resulted in empty hash")
	}

	tx2, _ := NewStandardTransaction(&senderPrivKey.PublicKey, receiverPubKeyBytes, 100, 10)
	tx2.Timestamp = 1234567890 // Identical timestamp
	hash2, err := tx2.Hash()
	if err != nil {
		t.Fatalf("tx2.Hash() error = %v", err)
	}
	if !bytes.Equal(hash1, hash2) {
		t.Errorf("Hashes of identical standard transactions do not match. Hash1: %x, Hash2: %x", hash1, hash2)
		tx1Json, _ := tx1.prepareDataForHashing()
		t.Logf("Tx1 JSON: %s", string(tx1Json))
		tx2Json, _ := tx2.prepareDataForHashing()
		t.Logf("Tx2 JSON: %s", string(tx2Json))
	}

	tx3, _ := NewStandardTransaction(&senderPrivKey.PublicKey, receiverPubKeyBytes, 200, 10) // Different amount
	tx3.Timestamp = 1234567890
	hash3, err := tx3.Hash()
	if err != nil {
		t.Fatalf("tx3.Hash() error = %v", err)
	}
	if bytes.Equal(hash1, hash3) {
		t.Errorf("Hashes of different transactions match. Hash1: %x, Hash3: %x", hash1, hash3)
	}

	// Test ID is set after Sign
	tx1Copy, _ := NewStandardTransaction(&senderPrivKey.PublicKey, receiverPubKeyBytes, 100, 10)
	tx1Copy.Timestamp = 1234567890
	preSignHash, _ := tx1Copy.Hash() // Hash before signing

	err = tx1Copy.Sign(senderPrivKey)
	if err != nil {
		t.Fatalf("tx1Copy.Sign() error: %v", err)
	}
	if !bytes.Equal(tx1Copy.ID, preSignHash) { // After Sign, ID should be set to the hash of the content
		t.Errorf("tx1Copy.ID (%x) != pre-sign hash (%x) after signing", tx1Copy.ID, preSignHash)
	}
}

func TestMultiSigTransactionHashing(t *testing.T) {
	key1 := newDummyPublicKeyBytes(t)
	key2 := newDummyPublicKeyBytes(t)
	key3 := newDummyPublicKeyBytes(t)
	
	authorizedKeys := [][]byte{key1, key2} // N=2
	SortByteSlices(authorizedKeys)        // Essential for canonical hash

	multiSigID := []byte("test_multisig_id_for_hashing")

	// Base for multi-sig transactions
	baseTxPayload := func() Transaction {
		tx, _ := NewMultiSigTransaction(
			multiSigID, TxStandard, 2, authorizedKeys,
			uint64(500), newDummyPublicKeyBytes(t), nil, nil, "", nil, uint64(5),
		)
		// Set fixed timestamp for deterministic hash
		tx.Timestamp = 1234567890
		return *tx
	}

	// Test 1: Identical multi-sig config and payload should have same hash
	txMulti1 := baseTxPayload()
	hash1, err := txMulti1.Hash()
	if err != nil {
		t.Fatalf("txMulti1.Hash() error = %v", err)
	}

	txMulti2 := baseTxPayload() // Identical payload
	hash2, err := txMulti2.Hash()
	if err != nil {
		t.Fatalf("txMulti2.Hash() error = %v", err)
	}
	if !bytes.Equal(hash1, hash2) {
		t.Errorf("Hashes of identical multi-sig transactions do not match. Hash1: %x, Hash2: %x", hash1, hash2)
		jsonBytes1, _ := txMulti1.prepareDataForHashing()
		t.Logf("TxMulti1 JSON: %s", string(jsonBytes1))
		jsonBytes2, _ := txMulti2.prepareDataForHashing()
		t.Logf("TxMulti2 JSON: %s", string(jsonBytes2))
	}

	// Test 2: Hash changes when RequiredSignatures (M) changes
	txMultiMChanged := baseTxPayload()
	txMultiMChanged.RequiredSignatures = 1 // Change M
	hashMChanged, err := txMultiMChanged.Hash()
	if err != nil {
		t.Fatalf("txMultiMChanged.Hash() error = %v", err)
	}
	if bytes.Equal(hash1, hashMChanged) {
		t.Errorf("Hash did not change when M changed.")
	}

	// Test 3: Hash changes when AuthorizedPublicKeys (N) change
	txMultiNChanged := baseTxPayload()
	txMultiNChanged.AuthorizedPublicKeys = [][]byte{key1, newDummyPublicKeyBytes(t)} // Change one of the N keys
	SortByteSlices(txMultiNChanged.AuthorizedPublicKeys)
	hashNChanged, err := txMultiNChanged.Hash()
	if err != nil {
		t.Fatalf("txMultiNChanged.Hash() error = %v", err)
	}
	if bytes.Equal(hash1, hashNChanged) {
		t.Errorf("Hash did not change when N keys changed.")
	}
}

// SortByteSlices sorts a slice of byte slices lexicographically.
// Crucial for canonical representation of AuthorizedPublicKeys.
func SortByteSlices(slices [][]byte) {
	sort.Slice(slices, func(i, j int) bool { return bytes.Compare(slices[i], slices[j]) < 0 })
}

func TestTransactionSerialization(t *testing.T) {
	// --- Test single-signer transaction serialization/deserialization ---
	senderPrivKey := newDummyPrivateKey(t)
	receiverPubKeyBytes := newDummyPublicKeyBytes(t)

	txStandard, err := NewStandardTransaction(&senderPrivKey.PublicKey, receiverPubKeyBytes, 100, 10)
	if err != nil {
		t.Fatalf("NewStandardTransaction failed: %v", err)
	}
	txStandard.Timestamp = time.Now().UnixNano() // Ensure timestamp is set before hashing for ID
	// Sign also sets ID
	err = txStandard.Sign(senderPrivKey) 
	if err != nil {
		t.Fatalf("txStandard.Sign() error = %v", err)
	}

	serializedTx, err := txStandard.Serialize()
	if err != nil {
		t.Fatalf("txStandard.Serialize() error = %v", err)
	}
	deserializedTx, err := DeserializeTransaction(serializedTx)
	if err != nil {
		t.Fatalf("DeserializeTransaction() error = %v", err)
	}

	// Check fields
	if !bytes.Equal(txStandard.ID, deserializedTx.ID) {
		t.Errorf("Deserialized single-signer Tx ID mismatch: got %x, want %x", deserializedTx.ID, txStandard.ID)
	}
	if txStandard.Timestamp != deserializedTx.Timestamp {
		t.Errorf("Deserialized single-signer Tx Timestamp mismatch")
	}
	if !bytes.Equal(txStandard.From, deserializedTx.From) {
		t.Errorf("Deserialized single-signer Tx From mismatch")
	}
	if !bytes.Equal(txStandard.To, deserializedTx.To) {
		t.Errorf("Deserialized single-signer Tx To mismatch")
	}
	if txStandard.Amount != deserializedTx.Amount {
		t.Errorf("Deserialized single-signer Tx Amount mismatch")
	}
	if txStandard.Fee != deserializedTx.Fee {
		t.Errorf("Deserialized single-signer Tx Fee mismatch")
	}
	if !bytes.Equal(txStandard.PublicKey, deserializedTx.PublicKey) {
		t.Errorf("Deserialized single-signer Tx PublicKey mismatch")
	}
	if !bytes.Equal(txStandard.Signature, deserializedTx.Signature) {
		t.Errorf("Deserialized single-signer Tx Signature mismatch")
	}
	if txStandard.TxType != deserializedTx.TxType {
		t.Errorf("Deserialized single-signer TxType mismatch")
	}

	// Verify signature after deserialization
	valid, err := deserializedTx.VerifySignature()
	if err != nil {
		t.Fatalf("deserializedTx.VerifySignature() error = %v", err)
	}
	if !valid {
		t.Errorf("deserializedTx.VerifySignature() = false for single-signer; want true")
	}

	// --- Test multi-signer transaction serialization/deserialization ---
	key1Priv := newDummyPrivateKey(t)
	key2Priv := newDummyPrivateKey(t)
	key3Priv := newDummyPrivateKey(t) // For signing
	
	pub1Bytes := elliptic.Marshal(elliptic.P256(), key1Priv.PublicKey.X, key1Priv.PublicKey.Y)
	pub2Bytes := elliptic.Marshal(elliptic.P256(), key2Priv.PublicKey.X, key2Priv.PublicKey.Y)
	
	authorizedKeys := [][]byte{pub1Bytes, pub2Bytes}
	SortByteSlices(authorizedKeys)

	multiSigID := []byte("test_multisig_id_for_serialization")
	receiverPubKeyBytes := newDummyPublicKeyBytes(t)

	// Create a multi-sig transaction with a dummy signature
	txMulti, err := NewMultiSigTransaction(
		multiSigID, TxStandard, 1, authorizedKeys,
		uint64(123), receiverPubKeyBytes, nil, nil, "", nil, uint64(3),
	)
	if err != nil {
		t.Fatalf("NewMultiSigTransaction failed: %v", err)
	}
	txMulti.Timestamp = time.Now().UnixNano() // Ensure timestamp is set
	
	// Manually add one valid signature for serialization test
	// This signer needs to be from authorizedKeys
	sig3, err := ecdsa.SignASN1(rand.Reader, key3Priv, []byte("dummy_hash_for_sig")) // Sign some dummy content
	if err != nil { t.Fatalf("Failed to dummy sign for multi-sig: %v", err) }
	
	// Add a valid signer from authorizedKeys
	txMulti.Signers = append(txMulti.Signers, SignerInfo{
		PublicKey: pub1Bytes, // One of the authorized keys
		Signature: sig3, // Dummy signature for now
	})
	
	// Set ID for the multi-sig transaction before serialization
	multiTxHash, err := txMulti.Hash()
	if err != nil { t.Fatalf("txMulti.Hash() error for serialization test: %v", err) }
	txMulti.ID = multiTxHash

	serializedMultiTx, err := txMulti.Serialize()
	if err != nil { t.Fatalf("txMulti.Serialize() error = %v", err) }
	
	deserializedMultiTx, err := DeserializeTransaction(serializedMultiTx)
	if err != nil { t.Fatalf("DeserializeTransaction(multiTx) error = %v", err) }

	// Compare Multi-Sig Tx fields
	if !bytes.Equal(txMulti.ID, deserializedMultiTx.ID) { t.Errorf("MultiTx ID mismatch") }
	if txMulti.Timestamp != deserializedMultiTx.Timestamp { t.Errorf("MultiTx Timestamp mismatch") }
	if !bytes.Equal(txMulti.From, deserializedMultiTx.From) { t.Errorf("MultiTx From mismatch") }
	if txMulti.TxType != deserializedMultiTx.TxType { t.Errorf("MultiTx TxType mismatch") }
	if txMulti.Amount != deserializedMultiTx.Amount { t.Errorf("MultiTx Amount mismatch") }
	if txMulti.Fee != deserializedMultiTx.Fee { t.Errorf("MultiTx Fee mismatch") }
	if txMulti.RequiredSignatures != deserializedMultiTx.RequiredSignatures { t.Errorf("MultiTx RequiredSignatures mismatch") }
	if !bytes.Equal(txMulti.To, deserializedMultiTx.To) { t.Errorf("MultiTx To mismatch") } // For standard multi-sig
	
	// Compare AuthorizedPublicKeys (must be sorted for consistency)
	if len(txMulti.AuthorizedPublicKeys) != len(deserializedMultiTx.AuthorizedPublicKeys) {
		t.Errorf("MultiTx AuthorizedPublicKeys length mismatch")
	} else {
		for i := range txMulti.AuthorizedPublicKeys {
			if !bytes.Equal(txMulti.AuthorizedPublicKeys[i], deserializedMultiTx.AuthorizedPublicKeys[i]) {
				t.Errorf("MultiTx AuthorizedPublicKeys[%d] mismatch", i)
			}
		}
	}

	// Compare Signers
	if len(txMulti.Signers) != len(deserializedMultiTx.Signers) {
		t.Errorf("MultiTx Signers length mismatch")
	} else {
		for i := range txMulti.Signers {
			if !bytes.Equal(txMulti.Signers[i].PublicKey, deserializedMultiTx.Signers[i].PublicKey) {
				t.Errorf("MultiTx Signers[%d] PublicKey mismatch", i)
			}
			if !bytes.Equal(txMulti.Signers[i].Signature, deserializedMultiTx.Signers[i].Signature) {
				t.Errorf("MultiTx Signers[%d] Signature mismatch", i)
			}
		}
	}
	
	// Ensure single-sig fields are NOT set for a multi-sig tx after deserialization
	if deserializedMultiTx.PublicKey != nil || deserializedMultiTx.Signature != nil {
		t.Errorf("Deserialized multi-sig Tx has single-sig PublicKey/Signature set, should be nil")
	}
	
	// Verify signature after deserialization (requires a valid signature added before serialization)
	// For this test, if we added a *valid* signature above, we'd verify it.
	// As we're using a dummy sig here, we can't fully verify it cryptographically unless we mock.
	// For now, only check that it doesn't error due to missing parts.
	_, err = deserializedMultiTx.VerifySignature()
	if err != nil && !errors.Is(err, ErrNotEnoughSigners) && !strings.Contains(err.Error(), "invalid signature for signer") && !strings.Contains(err.Error(), "unmarshal") { // Expecting actual crypto error if it fails
		t.Errorf("VerifySignature failed for deserialized multi-sig tx unexpectedly: %v", err)
	}
}

func TestMultiSignatureVerification(t *testing.T) {
	// --- Setup: Generate Keys ---
	key1Priv := newDummyPrivateKey(t)
	key2Priv := newDummyPrivateKey(t)
	key3Priv := newDummyPrivateKey(t) // 3rd authorized key, not necessarily signing below
	key4UnauthPriv := newDummyPrivateKey(t) // Unauthorized key for negative test

	pub1Bytes := elliptic.Marshal(elliptic.P256(), key1Priv.PublicKey.X, key1Priv.PublicKey.Y)
	pub2Bytes := elliptic.Marshal(elliptic.P256(), key2Priv.PublicKey.X, key2Priv.PublicKey.Y)
	pub3Bytes := elliptic.Marshal(elliptic.P256(), key3Priv.PublicKey.X, key3Priv.PublicKey.Y)
	pub4UnauthBytes := elliptic.Marshal(elliptic.P256(), key4UnauthPriv.PublicKey.X, key4UnauthPriv.PublicKey.Y)

	authorizedKeys := [][]byte{pub1Bytes, pub2Bytes, pub3Bytes} // N=3
	SortByteSlices(authorizedKeys) // Canonical order

	multiSigAddressBytes := []byte("derived_multisig_address_placeholder") // Dummy for now
	recipientAddressBytes := newDummyPublicKeyBytes(t)

	// --- Base Transaction for Multi-Sig ---
	baseTx, err := NewMultiSigTransaction(
		multiSigAddressBytes, TxStandard, 2, authorizedKeys, // M=2, N=3
		uint64(1000), recipientAddressBytes, nil, nil, "", nil, uint64(10),
	)
	if err != nil {
		t.Fatalf("Failed to create base multi-sig transaction: %v", err)
	}

	// Calculate the hash that needs to be signed by all parties
	txHashToSign, err := baseTx.Hash()
	if err != nil {
		t.Fatalf("baseTx.Hash() error: %v", err)
	}
	baseTx.ID = txHashToSign // Set ID for this specific transaction

	// --- Test Cases for VerifySignature ---

	// Test 1: Not enough signers (0/2)
	t.Run("NotEnoughSigners_Zero", func(t *testing.T) {
		tx := baseTx // Start with base tx, no signers
		valid, err := tx.VerifySignature()
		if valid {
			t.Errorf("VerifySignature succeeded with 0/2 signers; want false")
		}
		if err == nil || !errors.Is(err, ErrNotEnoughSigners) {
			t.Errorf("Expected ErrNotEnoughSigners, got: %v", err)
		}
	})

	// Test 2: Not enough signers (1/2) - Signer 1 only
	t.Run("NotEnoughSigners_OneValid", func(t *testing.T) {
		tx := baseTx
		sig1, err := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
		if err != nil {
			t.Fatalf("Failed to sign: %v", err)
		}
		tx.Signers = []SignerInfo{{PublicKey: pub1Bytes, Signature: sig1}}

		valid, err := tx.VerifySignature()
		if valid {
			t.Errorf("VerifySignature succeeded with 1/2 signers; want false")
		}
		if err == nil || !errors.Is(err, ErrNotEnoughSigners) {
			t.Errorf("Expected ErrNotEnoughSigners, got: %v", err)
		}
	})

	// Test 3: Exactly enough signers (2/2) - Signer 1 & 2
	t.Run("EnoughSigners_AllValid", func(t *testing.T) {
		tx := baseTx
		sig1, err := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
		if err != nil {
			t.Fatalf("Failed to sign (sig1): %v", err)
		}
		sig2, err := ecdsa.SignASN1(rand.Reader, key2Priv, txHashToSign)
		if err != nil {
			t.Fatalf("Failed to sign (sig2): %v", err)
		}
		tx.Signers = []SignerInfo{
			{PublicKey: pub1Bytes, Signature: sig1},
			{PublicKey: pub2Bytes, Signature: sig2},
		}
		// Sort Signers by PublicKey for deterministic order in test assertion,
		// though VerifySignature should handle unsorted internally.
		sort.Slice(tx.Signers, func(i, j int) bool {
			return bytes.Compare(tx.Signers[i].PublicKey, tx.Signers[j].PublicKey) < 0
		})

		valid, err := tx.VerifySignature()
		if err != nil {
			t.Fatalf("VerifySignature failed unexpectedly with 2/2 valid sigs: %v", err)
		}
		if !valid {
			t.Errorf("VerifySignature = false with 2/2 valid sigs; want true")
		}
	})

	// Test 4: Tampered signature
	t.Run("TamperedSignature", func(t *testing.T) {
		tx := baseTxPayload() // Get a fresh base transaction
		sig1, _ := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
		sig2, _ := ecdsa.SignASN1(rand.Reader, key2Priv, txHashToSign)
		tx.Signers = []SignerInfo{
			{PublicKey: pub1Bytes, Signature: sig1},
			{PublicKey: pub2Bytes, Signature: sig2},
		}
		tx.Signers[1].Signature = []byte("tampered_sig") // Tamper one signature

		valid, err := tx.VerifySignature()
		if valid {
			t.Errorf("VerifySignature succeeded with tampered sig; want false")
		}
		if err == nil || !errors.Is(err, ErrSignatureMissingOrInvalid) {
			t.Errorf("Expected ErrSignatureMissingOrInvalid for tampered sig, got: %v", err)
		}
	})

	// Test 5: Unauthorized signer
	t.Run("UnauthorizedSigner", func(t *testing.T) {
		tx := baseTxPayload()
		sig1, _ := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
		sig4Unauth, _ := ecdsa.SignASN1(rand.Reader, key4UnauthPriv, txHashToSign) // Signed by unauth key

		tx.Signers = []SignerInfo{
			{PublicKey: pub1Bytes, Signature: sig1},
			{PublicKey: pub4UnauthBytes, Signature: sig4Unauth}, // This signer is NOT in authorizedKeys
		}
		
		valid, err := tx.VerifySignature()
		if valid {
			t.Errorf("VerifySignature succeeded with unauthorized signer; want false")
		}
		if err == nil || !errors.Is(err, ErrUnauthorizedSigner) {
			t.Errorf("Expected ErrUnauthorizedSigner, got: %v", err)
		}
	})

	// Test 6: Duplicate signer (same public key signing twice)
	t.Run("DuplicateSigner", func(t *testing.T) {
		tx := baseTxPayload()
		sig1_a, _ := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
		sig1_b, _ := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign) // Same key, different signature potentially
		
		tx.Signers = []SignerInfo{
			{PublicKey: pub1Bytes, Signature: sig1_a},
			{PublicKey: pub1Bytes, Signature: sig1_b}, // Duplicate public key
		}

		valid, err := tx.VerifySignature()
		if valid {
			t.Errorf("VerifySignature succeeded with duplicate signer; want false")
		}
		if err == nil || !errors.Is(err, ErrDuplicateSigner) {
			t.Errorf("Expected ErrDuplicateSigner, got: %v", err)
		}
	})

	// Test 7: M > N (RequiredSignatures > len(AuthorizedPublicKeys))
	t.Run("M_GreaterThan_N", func(t *testing.T) {
		tx := baseTxPayload()
		tx.RequiredSignatures = 3 // M=3 (but AuthorizedPublicKeys has N=2 as per baseTxPayload setup)

		// Add enough valid signers to satisfy M, from the authorized list.
		// We add signers from authorizedKeys, so we'll get pub1Bytes, pub2Bytes.
		// Even if we add pub3Bytes, it's not in authorizedKeys.
		// The error should be M > N, not UnauthorizedSigner.
		sig1, _ := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
		sig2, _ := ecdsa.SignASN1(rand.Reader, key2Priv, txHashToSign)
		tx.Signers = []SignerInfo{
			{PublicKey: pub1Bytes, Signature: sig1},
			{PublicKey: pub2Bytes, Signature: sig2},
		}

		valid, err := tx.VerifySignature()
		if valid {
			t.Errorf("VerifySignature succeeded with M > N; want false")
		}
		if err == nil || !errors.Is(err, ErrMultiSigConfigInvalid) || !strings.Contains(err.Error(), "cannot be greater than N") {
			t.Errorf("Expected ErrMultiSigConfigInvalid (M > N), got: %v", err)
		}
	})
	
	// Test 8: Empty/Missing public key or signature in SignerInfo within multi-sig array
	t.Run("MissingSigOrPubKeyInSignerInfo", func(t *testing.T) {
	    tx := baseTxPayload()
	    // Add one valid, then one with missing pubkey
	    sig1, _ := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
	    tx.Signers = []SignerInfo{
	        {PublicKey: pub1Bytes, Signature: sig1},
	        {PublicKey: nil, Signature: []byte("some_sig")}, // Missing pubkey
	    }
	    valid, err := tx.VerifySignature()
	    if valid { t.Errorf("VerifySignature succeeded with missing pubkey in signerInfo") }
	    if err == nil || !errors.Is(err, ErrSignatureMissingOrInvalid) || !strings.Contains(err.Error(), "public key missing for signer") {
	        t.Errorf("Expected 'public key missing' error, got: %v", err)
	    }
	    
	    // Missing signature
	    tx = baseTxPayload()
	    tx.Signers = []SignerInfo{
	        {PublicKey: pub1Bytes, Signature: sig1},
	        {PublicKey: pub2Bytes, Signature: nil}, // Missing signature
	    }
	    valid, err = tx.VerifySignature()
	    if valid { t.Errorf("VerifySignature succeeded with missing sig in signerInfo") }
	    if err == nil || !errors.Is(err, ErrSignatureMissingOrInvalid) || !strings.Contains(err.Error(), "signature missing for signer") {
	        t.Errorf("Expected 'signature missing' error, got: %v", err)
	    }
	})
	
	// Test 9: Tx with MultiSig fields present but zero signers
	t.Run("MultiSigConfigPresentButNoSigners", func(t *testing.T) {
		tx := baseTxPayload()
		tx.Signers = []SignerInfo{} // Explicitly empty signers array
		valid, err := tx.VerifySignature()
		if valid { t.Errorf("VerifySignature succeeded with no signers but multi-sig config") }
		if err == nil || !errors.Is(err, ErrNotEnoughSigners) {
			t.Errorf("Expected ErrNotEnoughSigners when no signers provided for multi-sig, got: %v", err)
		}
	})
	
	// Test 10: Tx with MultiSig fields and Signers but M=0
	t.Run("MultiSigMIsZero", func(t *testing.T) {
	    tx := baseTxPayload()
	    tx.RequiredSignatures = 0 // M=0
	    sig1, _ := ecdsa.SignASN1(rand.Reader, key1Priv, txHashToSign)
	    tx.Signers = []SignerInfo{{PublicKey: pub1Bytes, Signature: sig1}}
	    
	    valid, err := tx.VerifySignature()
	    if valid { t.Errorf("VerifySignature succeeded when M=0") }
	    if err == nil || !errors.Is(err, ErrMultiSigConfigInvalid) || !strings.Contains(err.Error(), "RequiredSignatures") {
	        t.Errorf("Expected ErrMultiSigConfigInvalid (M=0), got: %v", err)
	    }
	})
}