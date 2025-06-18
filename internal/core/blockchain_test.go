package core

import (
	"bytes"
	"errors" // For errors.Is
	"testing"
	"time" // Used explicitly for dummy data timestamps

	// "empower1.com/core/internal/crypto" // For real crypto key generation, if needed later
)

// Helper functions for creating dummy transactions for tests.
// This ensures consistent, reproducible test data.
func newDummyTx(txType TxType, data string, inputs, outputs int) Transaction {
	tx := Transaction{
		ID:        sha256.Sum256([]byte(data + time.Now().String())), // Unique ID based on data+time
		Timestamp: time.Now().UnixNano(),
		TxType:    txType,
		Metadata:  map[string]string{"test_data_origin": data},
	}
	// Simulate inputs/outputs - actual structs filled partially for block context
	for i := 0; i < inputs; i++ {
		tx.Inputs = append(tx.Inputs, TxInput{TxID: []byte(fmt.Sprintf("prev_tx_id_%d", i)), Vout: i})
	}
	for i := 0; i < outputs; i++ {
		tx.Outputs = append(tx.Outputs, TxOutput{Value: int64(100 + i)})
	}
	return tx
}

// Helper to create a fully signed and hashed block for testing chain continuity.
func createTestBlock(t *testing.T, height int64, prevHash []byte, transactions []Transaction, proposer string, aiAuditLog []byte) *Block {
	block := NewBlock(height, prevHash, transactions)
	// Ensure proposer address is byte slice
	proposerBytes := []byte(proposer)
	block.ProposerAddress = proposerBytes
	block.AIAuditLog = aiAuditLog // Add the AI Audit Log

	// Sign the block (dummy signature for now)
	err := block.Sign(proposerBytes, []byte("dummy_private_key"))
	if err != nil {
		t.Fatalf("Failed to sign block %d: %v", height, err)
	}
	block.SetHash() // Calculate the hash after signing and all fields are set
	return block
}

// --- Test Cases ---

func TestNewBlockchain(t *testing.T) {
	bc := NewBlockchain() // This call implicitly creates and adds the genesis block
	if bc == nil {
		t.Fatal("NewBlockchain() returned nil")
	}
	if len(bc.blocks) != 1 { // Should contain exactly one block (the genesis block)
		t.Fatalf("New blockchain should have 1 block (genesis), got %d", len(bc.blocks))
	}

	genesisBlock := bc.blocks[0]
	if genesisBlock.Height != 0 {
		t.Errorf("Genesis block height = %d; want 0", genesisBlock.Height)
	}
	// Genesis PrevBlockHash should be all zeros as defined in createGenesisBlock
	if !bytes.Equal(genesisBlock.PrevBlockHash, bytes.Repeat([]byte{0x00}, sha256.Size)) {
		t.Errorf("Genesis block PrevBlockHash is incorrect")
	}
	if genesisBlock.Hash == nil || len(genesisBlock.Hash) == 0 {
		t.Errorf("Genesis block hash not set or is empty")
	}
	// Check that ProposerAddress and Signature are set for genesis
	if len(genesisBlock.ProposerAddress) == 0 {
		t.Errorf("Genesis block ProposerAddress not set")
	}
	if len(genesisBlock.Signature) == 0 {
		t.Errorf("Genesis block Signature not set")
	}
	// AIAuditLog could be set for genesis, depends on specific implementation of createGenesisBlock
	// For now, it's ok if it's nil/empty if createGenesisBlock doesn't set it explicitly.

	// Check index
	_, exists := bc.blockIndex[string(genesisBlock.Hash)]
	if !exists {
		t.Errorf("Genesis block not found in blockIndex by its hash")
	}

	height := bc.ChainHeight()
	if height != 0 {
		t.Errorf("ChainHeight() for new blockchain = %d; want 0", height)
	}

	lastBlock, err := bc.GetLastBlock()
	if err != nil {
		t.Fatalf("GetLastBlock() for new blockchain returned error: %v", err)
	}
	if !bytes.Equal(lastBlock.Hash, genesisBlock.Hash) {
		t.Errorf("GetLastBlock() does not return genesis block for new blockchain")
	}
}

func TestBlockchainAddBlock(t *testing.T) {
	bc := NewBlockchain()
	genesisBlock := bc.blocks[0]

	// Create a valid next block
	tx1_1 := newDummyTx(StandardTx, "tx1_1_data", 1, 1)
	tx1_2 := newDummyTx(StimulusTx, "tx1_2_data_stimulus", 0, 1) // EmPower1 specific TxType
	block1 := createTestBlock(t, 1, genesisBlock.Hash, []Transaction{tx1_1, tx1_2}, "proposer1_pubkey", []byte("audit_log_b1"))

	err := bc.AddBlock(block1)
	if err != nil {
		t.Fatalf("AddBlock(valid_block1) error = %v; want nil", err)
	}
	if len(bc.blocks) != 2 {
		t.Errorf("Blockchain length = %d; want 2 after adding block1", len(bc.blocks))
	}
	if bc.ChainHeight() != 1 {
		t.Errorf("ChainHeight() = %d; want 1", bc.ChainHeight())
	}
	if _, exists := bc.blockIndex[string(block1.Hash)]; !exists {
		t.Errorf("Block1 not found in blockIndex after adding")
	}

	// Try to add the same block again (duplicate hash)
	err = bc.AddBlock(block1)
	if err == nil || !errors.Is(err, ErrBlockAlreadyExists) { // Use errors.Is for custom errors
		t.Errorf("Expected ErrBlockAlreadyExists for duplicate block, got %v", err)
	}
	if len(bc.blocks) != 2 { // Ensure no block was actually added
		t.Errorf("Blockchain length changed after duplicate add, should remain 2, got %d", len(bc.blocks))
	}

	// Try to add block with incorrect height
	block2WrongHeight := createTestBlock(t, 3, block1.Hash, []Transaction{newDummyTx(StandardTx, "tx2_wrong_height", 1, 1)}, "proposer2_pubkey", nil) // Height should be 2
	err = bc.AddBlock(block2WrongHeight)
	if err == nil || !errors.Is(err, ErrInvalidBlockHeight) {
		t.Errorf("Expected ErrInvalidBlockHeight, got %v", err)
	}

	// Try to add block with incorrect previous hash
	block2WrongPrevHash := createTestBlock(t, 2, []byte("wrong_prev_hash"), []Transaction{newDummyTx(StandardTx, "tx2_wrong_prev", 1, 1)}, "proposer2_pubkey", nil)
	err = bc.AddBlock(block2WrongPrevHash)
	if err == nil || !errors.Is(err, ErrInvalidPrevBlockHash) {
		t.Errorf("Expected ErrInvalidPrevBlockHash, got %v", err)
	}
    
    // --- Test signature/validation failures ---
    // Block with valid height/prev_hash, but invalid signature (dummy impl)
    blockWithInvalidSig := NewBlock(2, block1.Hash, []Transaction{newDummyTx(StandardTx, "invalid_sig_tx", 1, 1)})
    blockWithInvalidSig.ProposerAddress = []byte("proposer_invalid_sig")
    // Manually set a clearly invalid signature format for the dummy verifier
    blockWithInvalidSig.Signature = []byte("TRULY_INVALID_SIG") 
    blockWithInvalidSig.SetHash() // Hash needs to be set for the block itself
    err = bc.AddBlock(blockWithInvalidSig)
    if err == nil || !errors.Is(err, ErrInvalidSignature) {
        t.Errorf("Expected ErrInvalidSignature for invalid signature, got %v", err)
    }

    // Block with valid height/prev_hash, but failing ValidateTransactions (e.g., empty TX ID)
    txInvalidInternal := newDummyTx(StandardTx, "tx_empty_id", 1, 1)
    txInvalidInternal.ID = []byte{} // Simulate invalid internal transaction state
    blockWithTxValidationError := createTestBlock(t, 2, block1.Hash, []Transaction{txInvalidInternal}, "proposer_tx_error", nil)
    err = bc.AddBlock(blockWithTxValidationError)
    if err == nil { // Specific error will come from ValidateTransactions
        t.Errorf("Expected transaction validation error, got nil")
    }
    // We expect the error message from ValidateTransactions, e.g., "transaction 0 has no ID"
    if err != nil && !bytes.Contains([]byte(err.Error()), []byte("transaction 0 has no ID")) {
        t.Errorf("Expected error containing 'transaction 0 has no ID', got: %v", err)
    }

	// Add another valid block
	tx2_1 := newDummyTx(StandardTx, "tx2_1_data", 1, 1)
	block2 := createTestBlock(t, 2, block1.Hash, []Transaction{tx2_1}, "proposer2_pubkey", []byte("audit_log_b2"))
	err = bc.AddBlock(block2)
	if err != nil {
		t.Fatalf("AddBlock(valid_block2) error = %v; want nil", err)
	}
	if bc.ChainHeight() != 2 {
		t.Errorf("ChainHeight() = %d; want 2 after block2", bc.ChainHeight())
	}
	if _, exists := bc.blockIndex[string(block2.Hash)]; !exists {
		t.Errorf("Block2 not found in blockIndex after adding")
	}
}

func TestBlockchainGetters(t *testing.T) {
	bc := NewBlockchain()
	genesisBlock := bc.blocks[0]

	block1 := createTestBlock(t, 1, genesisBlock.Hash, []Transaction{newDummyTx(StandardTx, "g_tx1", 1, 1)}, "proposer_g1", nil)
	bc.AddBlock(block1)

	block2 := createTestBlock(t, 2, block1.Hash, []Transaction{newDummyTx(StandardTx, "g_tx2", 1, 1)}, "proposer_g2", nil)
	bc.AddBlock(block2)

	// GetBlockByHeight
	b, err := bc.GetBlockByHeight(1)
	if err != nil {
		t.Fatalf("GetBlockByHeight(1) error = %v", err)
	}
	if !bytes.Equal(b.Hash, block1.Hash) {
		t.Errorf("GetBlockByHeight(1) returned wrong block. Got %x, want %x", b.Hash, block1.Hash)
	}
	// Test error for out of range height
	_, err = bc.GetBlockByHeight(3) // Non-existent height
	if err == nil || !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("Expected ErrBlockNotFound for GetBlockByHeight(3), got %v", err)
	}
	_, err = bc.GetBlockByHeight(-1) // Negative height
	if err == nil || !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("Expected ErrBlockNotFound for GetBlockByHeight(-1), got %v", err)
	}

	// GetBlockByHash
	b, err = bc.GetBlockByHash(block2.Hash)
	if err != nil {
		t.Fatalf("GetBlockByHash(block2_hash) error = %v", err)
	}
	if !bytes.Equal(b.Hash, block2.Hash) {
		t.Errorf("GetBlockByHash(block2_hash) returned wrong block. Got %x, want %x", b.Hash, block2.Hash)
	}
	_, err = bc.GetBlockByHash([]byte("non_existent_hash"))
	if err == nil || !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("Expected ErrBlockNotFound for non-existent hash, got %v", err)
	}

	// GetLastBlock
	last, err := bc.GetLastBlock()
	if err != nil {
		t.Fatalf("GetLastBlock() error = %v", err)
	}
	if !bytes.Equal(last.Hash, block2.Hash) {
		t.Errorf("GetLastBlock() returned wrong block. Got %x, want %x", last.Hash, block2.Hash)
	}
}

func TestBlockchainEmptyState(t *testing.T) {
	bcEmpty := &Blockchain{ // Manually create an empty one for specific empty state tests
		blocks:     make([]*Block, 0),
		blockIndex: make(map[string]*Block),
		logger:     log.New(os.Stdout, "EMPTY_BC_TEST: ", log.Ldate|log.Ltime|log.Lshortfile),
	}

	// ChainHeight
	if height := bcEmpty.ChainHeight(); height != -1 {
		t.Errorf("Empty chain ChainHeight() = %d; want -1", height)
	}

	// GetLastBlock
	_, err := bcEmpty.GetLastBlock()
	if err == nil || !errors.Is(err, ErrBlockchainEmpty) {
		t.Errorf("Expected ErrBlockchainEmpty for empty chain GetLastBlock, got %v", err)
	}

	// GetBlockByHeight (empty chain)
	_, err = bcEmpty.GetBlockByHeight(0)
	if err == nil || !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("Expected ErrBlockNotFound for empty chain GetBlockByHeight(0), got %v", err)
	}

	// GetBlockByHash (empty chain)
	_, err = bcEmpty.GetBlockByHash([]byte("any_hash"))
	if err == nil || !errors.Is(err, ErrBlockNotFound) {
		t.Errorf("Expected ErrBlockNotFound for empty chain GetBlockByHash, got %v", err)
	}
}

func TestAddBlockToEmptyBlockchainExplicitly(t *testing.T) {
	// This tests the AddBlock logic when the chain is conceptually empty (before NewBlockchain does its magic).
	bcTest := &Blockchain{
		blocks:     make([]*Block, 0),
		blockIndex: make(map[string]*Block),
		logger:     log.New(os.Stdout, "ADD_TO_EMPTY_TEST: ", log.Ldate|log.Ltime|log.Lshortfile),
	}

	// Try adding a block with height 1 (should fail, only height 0 allowed for first)
	blockHeight1 := createTestBlock(t, 1, []byte("some_prev_hash"), []Transaction{newDummyTx(StandardTx, "tx_h1", 1, 1)}, "proposer_h1", nil)
	err := bcTest.AddBlock(blockHeight1)
	if err == nil || !errors.Is(err, ErrInvalidBlockHeight) {
		t.Errorf("Expected ErrInvalidBlockHeight for adding height 1 block to empty chain, got %v", err)
	}
	if len(bcTest.blocks) != 0 {
		t.Errorf("Blockchain not empty after failed add, len %d", len(bcTest.blocks))
	}

	// Try adding a block with height 0 (should pass, becoming the first block)
	genesisTestBlock := createTestBlock(t, 0, bytes.Repeat([]byte{0x00}, sha256.Size), []Transaction{newDummyTx(StandardTx, "genesis_test_tx", 1, 1)}, "test_genesis_proposer", []byte("test_audit_log"))
	err = bcTest.AddBlock(genesisTestBlock)
	if err != nil {
		t.Fatalf("Adding height 0 block to empty chain failed: %v", err)
	}
	if bcTest.ChainHeight() != 0 {
		t.Errorf("ChainHeight after adding first block (height 0) = %d; want 0", bcTest.ChainHeight())
	}
	if len(bcTest.blocks) != 1 {
		t.Errorf("Number of blocks after adding first block = %d; want 1", len(bcTest.blocks))
	}
	if !bytes.Equal(bcTest.blocks[0].Hash, genesisTestBlock.Hash) {
		t.Errorf("Added genesis block hash mismatch")
	}
}