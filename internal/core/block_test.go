package core

import (
	"bytes"
	"encoding/hex" // For logging/displaying hashes
	"testing"
	"time" // Re-added for timestamp checks, if needed
	// Assuming 'internal/crypto' contains actual key generation for testing real crypto later
	// "empower1.com/core/internal/crypto"
)

// --- Helper functions for creating dummy transactions for tests ---
// (Adhering to "Know Your Core, Keep it Clear" for test data setup)
func newDummyTx(txType TxType, data string, inputs, outputs int) Transaction {
	tx := Transaction{
		ID:        sha256.Sum256([]byte(data + time.Now().String())), // Unique ID
		Timestamp: time.Now().UnixNano(),
		TxType:    txType,
		Metadata:  map[string]string{"test_data": data},
	}
	// Simulate inputs/outputs - actual structs not needed for basic block test
	for i := 0; i < inputs; i++ {
		tx.Inputs = append(tx.Inputs, TxInput{TxID: []byte(fmt.Sprintf("prev_tx_%d", i)), Vout: i, PubKey: []byte("sender_pubkey")})
	}
	for i := 0; i < outputs; i++ {
		tx.Outputs = append(tx.Outputs, TxOutput{Value: int64(100 + i), PubKeyHash: []byte(fmt.Sprintf("receiver_pubkey_hash_%d", i))})
	}
	return tx
}

// --- Test Cases ---

func TestNewBlock(t *testing.T) {
	// Creating dummy previous block data for comprehensive test, as PrevBlockHash is now []byte
	prevBlock := NewBlock(0, []byte("genesis_prev_hash"), []byte{newDummyTx(StandardTx, "genesis_tx", 1, 1)}) // Use an actual slice of Transaction

	height := int64(1)
	transactions := []Transaction{
		newDummyTx(StandardTx, "tx1_data", 1, 2),
		newDummyTx(StimulusTx, "stimulus_tx_data", 0, 1), // EmPower1 specific TxType
	}

	block := NewBlock(height, prevBlock.Hash, transactions)

	if block.Height != height {
		t.Errorf("block.Height = %d; want %d", block.Height, height)
	}
	if !bytes.Equal(block.PrevBlockHash, prevBlock.Hash) {
		t.Errorf("block.PrevBlockHash is incorrect: got %x, want %x", block.PrevBlockHash, prevBlock.Hash)
	}
	if len(block.Transactions) != len(transactions) {
		t.Errorf("block.Transactions length = %d; want %d", len(block.Transactions), len(transactions))
	}
	// Verify that transactions are correctly copied/assigned (deep equality might be needed for complex structs)
	if !bytes.Equal(block.Transactions[0].ID, transactions[0].ID) {
		t.Errorf("block.Transactions[0] ID is incorrect")
	}
	if block.Timestamp == 0 {
		t.Errorf("block.Timestamp not set")
	}
	if block.ProposerAddress != nil { // Should be nil initially
		t.Errorf("block.ProposerAddress should be nil initially, got %x", block.ProposerAddress)
	}
	if block.Signature != nil { // Should be nil initially
		t.Errorf("block.Signature should be nil initially")
	}
	if block.Hash != nil { // Should be nil until SetHash is called
		t.Errorf("block.Hash should be nil initially")
	}
	if block.AIAuditLog != nil { // Should be nil until AI data is processed/added
		t.Errorf("block.AIAuditLog should be nil initially")
	}
}

func TestBlockSignAndVerifySignature(t *testing.T) {
	transactions := []Transaction{newDummyTx(StandardTx, "sign_test_tx", 1, 1)}
	block := NewBlock(1, []byte("prev_hash_sign"), transactions)

	// In a real scenario, this would use actual validator keys
	// Dummy proposer address (e.g., a hashed public key)
	proposerAddressBytes := []byte("em_validator_alpha") 
	privateKeyBytes := []byte("dummy_private_key_for_test") // Not used by current placeholder

	// Test placeholder Sign method
	err := block.Sign(proposerAddressBytes, privateKeyBytes) 
	if err != nil {
		t.Fatalf("block.Sign() error = %v", err)
	}
	if !bytes.Equal(block.ProposerAddress, proposerAddressBytes) {
		t.Errorf("block.ProposerAddress = %x; want %x", block.ProposerAddress, proposerAddressBytes)
	}
	if !bytes.HasPrefix(block.Signature, []byte("empower1-signed-by-")) { // Check our specific dummy prefix
		t.Errorf("block.Signature is incorrect for placeholder signing: got %s", string(block.Signature))
	}

	// Test placeholder VerifySignature method (should pass for valid dummy sig)
	valid, err := block.VerifySignature()
	if err != nil {
		t.Fatalf("block.VerifySignature() error = %v", err)
	}
	if !valid {
		t.Errorf("block.VerifySignature() = false; want true for placeholder signature")
	}

	// --- Test error conditions ---
	// Test with empty ProposerAddress to Sign
	block2 := NewBlock(2, []byte("hash2"), transactions)
	err = block2.Sign(nil, privateKeyBytes)
	if err == nil || !errors.Is(err, ErrMissingProposer) { // Use errors.Is for custom errors
		t.Errorf("Expected ErrMissingProposer for Sign with nil ProposerAddress, got %v", err)
	}
	err = block2.Sign([]byte{}, privateKeyBytes)
	if err == nil || !errors.Is(err, ErrMissingProposer) {
		t.Errorf("Expected ErrMissingProposer for Sign with empty ProposerAddress, got %v", err)
	}
	
	// Test with empty PrivateKeyBytes to Sign
	err = block2.Sign(proposerAddressBytes, nil)
	if err == nil {
		t.Errorf("Expected error for Sign with nil PrivateKeyBytes")
	}
	err = block2.Sign(proposerAddressBytes, []byte{})
	if err == nil {
		t.Errorf("Expected error for Sign with empty PrivateKeyBytes")
	}


	// Tamper with ProposerAddress and expect verification to fail
	originalProposer := block.ProposerAddress
	block.ProposerAddress = []byte("tampered_proposer")
	valid, err = block.VerifySignature()
	if err == nil || !errors.Is(err, ErrInvalidSignature) { // Verify specific error for tampered signature
		t.Errorf("Expected ErrInvalidSignature after tampering ProposerAddress, got %v (valid=%t)", err, valid)
	}
	if valid {
		t.Errorf("block.VerifySignature() = true after tampering ProposerAddress; want false")
	}
	block.ProposerAddress = originalProposer // Restore

	// Tamper with Signature and expect verification to fail
	originalSignature := block.Signature
	block.Signature = []byte("tampered_signature")
	valid, err = block.VerifySignature()
	if err == nil || !errors.Is(err, ErrInvalidSignature) { // Verify specific error
		t.Errorf("Expected ErrInvalidSignature after tampering Signature, got %v (valid=%t)", err, valid)
	}
	if valid {
		t.Errorf("block.VerifySignature() = true after tampering Signature; want false")
	}
	block.Signature = originalSignature // Restore

	// Test with empty ProposerAddress for VerifySignature (should fail)
	block.ProposerAddress = nil // Use nil for missing
	_, err = block.VerifySignature()
	if err == nil || !errors.Is(err, ErrMissingProposer) {
		t.Errorf("Expected ErrMissingProposer for VerifySignature with nil ProposerAddress, got %v", err)
	}
	block.ProposerAddress = originalProposer // Restore

	// Test with nil Signature for VerifySignature (should fail)
	block.Signature = nil
	_, err = block.VerifySignature()
	if err == nil || !errors.Is(err, ErrMissingSignature) {
		t.Errorf("Expected ErrMissingSignature for VerifySignature with nil Signature, got %v", err)
	}
	block.Signature = originalSignature // Restore
}

func TestBlockSetHash(t *testing.T) {
	transactions := []Transaction{newDummyTx(StandardTx, "hash_test_tx", 1, 1)}
	block := NewBlock(1, []byte("prev_hash_sethash"), transactions)
	block.ProposerAddress = []byte("test_proposer") 
	block.Timestamp = 1234567890 
	block.AIAuditLog = []byte("dummy_ai_log") // EmPower1 specific

	// Calculate hash first time
	block.SetHash()
	hash1 := make([]byte, len(block.Hash))
	copy(hash1, block.Hash)

	if len(hash1) == 0 {
		t.Errorf("block.SetHash() resulted in empty hash")
	}

	// Call SetHash again, should be idempotent if fields haven't changed
	block.SetHash()
	if !bytes.Equal(hash1, block.Hash) {
		t.Errorf("block.SetHash() is not idempotent. Hash1: %x, Hash2: %x", hash1, block.Hash)
	}

	// Change a field that's part of HeaderForSigning and expect hash to change
	block.Transactions = []Transaction{newDummyTx(StimulusTx, "different_tx_data", 0, 1)} // Change transactions
	block.SetHash()
	hash2 := block.Hash
	if bytes.Equal(hash1, hash2) {
		t.Errorf("Block hash did not change after transactions modification. Hash1: %x, Hash2: %x", hash1, hash2)
	}

	// Change ProposerAddress, expect hash to change
	block.ProposerAddress = []byte("another_proposer_hash")
	block.SetHash()
	hash3 := block.Hash
	if bytes.Equal(hash2, hash3) {
		t.Errorf("Block hash did not change after ProposerAddress modification. Hash2: %x, Hash3: %x", hash2, hash3)
	}

	// Change AIAuditLog, expect hash to change
	block.AIAuditLog = []byte("new_ai_log")
	block.SetHash()
	hash4 := block.Hash
	if bytes.Equal(hash3, hash4) {
		t.Errorf("Block hash did not change after AIAuditLog modification. Hash3: %x, Hash4: %x", hash3, hash4)
	}

	t.Logf("TestBlockSetHash: Hash1: %s", hex.EncodeToString(hash1))
	t.Logf("TestBlockSetHash: Hash2 (after tx change): %s", hex.EncodeToString(hash2))
	t.Logf("TestBlockSetHash: Hash3 (after proposer change): %s", hex.EncodeToString(hash3))
	t.Logf("TestBlockSetHash: Hash4 (after AI log change): %s", hex.EncodeToString(hash4))
}

func TestBlockHeaderForSigning(t *testing.T) {
	transactions := []Transaction{
		newDummyTx(StandardTx, "header_tx1", 1, 1),
		newDummyTx(TaxTx, "header_tx2", 1, 1), // EmPower1 specific TxType
	}
	block := NewBlock(1, []byte("prev_hash_header"), transactions)
	block.ProposerAddress = []byte("test_proposer_header")
	block.Timestamp = 9876543210
	block.AIAuditLog = []byte("audit_log_header")

	headerBytes1 := block.HeaderForSigning()
	if len(headerBytes1) == 0 {
		t.Errorf("block.HeaderForSigning() returned empty slice")
	}

	// Change transactions, expect HeaderForSigning to change
	block.Transactions = []Transaction{newDummyTx(StimulusTx, "new_header_tx", 0, 1)}
	headerBytes2 := block.HeaderForSigning()
	if bytes.Equal(headerBytes1, headerBytes2) {
		t.Errorf("HeaderForSigning did not change after transactions modification")
	}

	// Change ProposerAddress, expect HeaderForSigning to change
	block.ProposerAddress = []byte("another_proposer_header")
	headerBytes3 := block.HeaderForSigning()
	if bytes.Equal(headerBytes2, headerBytes3) {
		t.Errorf("HeaderForSigning did not change after ProposerAddress modification")
	}

	// Change AIAuditLog, expect HeaderForSigning to change
	block.AIAuditLog = []byte("new_audit_log_header")
	headerBytes4 := block.HeaderForSigning()
	if bytes.Equal(headerBytes3, headerBytes4) {
		t.Errorf("HeaderForSigning did not change after AIAuditLog modification")
	}
}

func TestBlockValidateTransactions(t *testing.T) {
    // Test case 1: Valid transactions (basic check)
    block1 := NewBlock(1, []byte("prev_hash_valid"), []Transaction{
        newDummyTx(StandardTx, "valid_tx1", 1, 1),
        newDummyTx(StimulusTx, "valid_tx2", 0, 1),
    })
    err := block1.ValidateTransactions()
    if err != nil {
        t.Errorf("Expected no error for valid transactions, got %v", err)
    }

    // Test case 2: Transaction with empty ID (should fail)
    txWithEmptyID := newDummyTx(StandardTx, "empty_id_tx", 1, 1)
    txWithEmptyID.ID = []byte{} // Tamper with ID
    block2 := NewBlock(2, []byte("prev_hash_invalid"), []Transaction{txWithEmptyID})
    err = block2.ValidateTransactions()
    if err == nil {
        t.Errorf("Expected error for transaction with empty ID, got none")
    }
    expectedErr := "transaction 0 has no ID"
    if err != nil && err.Error() != expectedErr {
        t.Errorf("Expected error message '%s', got '%s'", expectedErr, err.Error())
    }

    // Test case 3: Conceptual AI/ML flagging (requires actual AI integration)
    // This test case cannot be fully implemented without the actual ai_ml_module.
    // It would involve:
    //   - Mocking ai_ml_module.AnalyzeTransactions to return a specific verdict.
    //   - Creating a transaction that the mock would flag.
    //   - Asserting that ValidateTransactions returns the expected error/flag.
    // For now, this remains a conceptual test.
}