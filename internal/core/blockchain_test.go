package core

import (
	"bytes"
	"testing"
	// "time"
)

func TestNewBlockchain(t *testing.T) {
	bc := NewBlockchain()
	if bc == nil {
		t.Fatal("NewBlockchain() returned nil")
	}
	if len(bc.blocks) != 1 { // Should contain only the genesis block
		t.Fatalf("New blockchain should have 1 block (genesis), got %d", len(bc.blocks))
	}

	genesisBlock := bc.blocks[0]
	if genesisBlock.Height != 0 {
		t.Errorf("Genesis block height = %d; want 0", genesisBlock.Height)
	}
	if string(genesisBlock.PrevBlockHash) != "genesis_prev_hash" { // As defined in createGenesisBlock
		t.Errorf("Genesis block prev hash is incorrect")
	}
	if genesisBlock.Hash == nil || len(genesisBlock.Hash) == 0 {
		t.Errorf("Genesis block hash not set")
	}

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
		t.Fatalf("GetLastBlock() for new blockchain error = %v", err)
	}
	if !bytes.Equal(lastBlock.Hash, genesisBlock.Hash) {
		t.Errorf("GetLastBlock() does not return genesis block for new blockchain")
	}
}

func TestBlockchainAddBlock(t *testing.T) {
	bc := NewBlockchain()
	genesisBlock := bc.blocks[0]

	// Create a valid next block
	block1Data := []byte("Block 1 Data")
	block1 := NewBlock(1, genesisBlock.Hash, block1Data)
	block1.ProposerAddress = "proposer1"
	block1.Sign("proposer1", nil) // Placeholder sign
	block1.SetHash()

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
	if err == nil {
		t.Errorf("AddBlock(duplicate_block1) error = nil; want error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("already exists in chain")) {
		t.Errorf("Expected 'already exists' error, got: %v", err)
	}


	// Try to add block with incorrect height
	block2WrongHeightData := []byte("Block 2 Data")
	block2WrongHeight := NewBlock(3, block1.Hash, block2WrongHeightData) // Height should be 2
	block2WrongHeight.ProposerAddress = "proposer2"
	block2WrongHeight.Sign("proposer2", nil)
	block2WrongHeight.SetHash()
	err = bc.AddBlock(block2WrongHeight)
	if err == nil {
		t.Errorf("AddBlock(wrong_height_block) error = nil; want error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid block height")) {
		t.Errorf("Expected 'invalid block height' error, got: %v", err)
	}


	// Try to add block with incorrect previous hash
	block2WrongPrevHashData := []byte("Block 2 Data")
	block2WrongPrevHash := NewBlock(2, []byte("wrong_prev_hash"), block2WrongPrevHashData)
	block2WrongPrevHash.ProposerAddress = "proposer2"
	block2WrongPrevHash.Sign("proposer2", nil)
	block2WrongPrevHash.SetHash()
	err = bc.AddBlock(block2WrongPrevHash)
	if err == nil {
		t.Errorf("AddBlock(wrong_prev_hash_block) error = nil; want error")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid previous block hash")) {
		t.Errorf("Expected 'invalid previous block hash' error, got: %v", err)
	}

	// Add another valid block
	block2Data := []byte("Block 2 Data")
	block2 := NewBlock(2, block1.Hash, block2Data)
	block2.ProposerAddress = "proposer2"
	block2.Sign("proposer2", nil)
	block2.SetHash()
	err = bc.AddBlock(block2)
	if err != nil {
		t.Fatalf("AddBlock(valid_block2) error = %v; want nil", err)
	}
	if bc.ChainHeight() != 2 {
		t.Errorf("ChainHeight() = %d; want 2 after block2", bc.ChainHeight())
	}
}

func TestBlockchainGetters(t *testing.T) {
	bc := NewBlockchain()
	genesisBlock := bc.blocks[0]

	block1Data := []byte("Block 1 Data")
	block1 := NewBlock(1, genesisBlock.Hash, block1Data)
	block1.ProposerAddress = "proposer1"; block1.Sign("proposer1", nil); block1.SetHash()
	bc.AddBlock(block1)

	block2Data := []byte("Block 2 Data")
	block2 := NewBlock(2, block1.Hash, block2Data)
	block2.ProposerAddress = "proposer2"; block2.Sign("proposer2", nil); block2.SetHash()
	bc.AddBlock(block2)

	// GetBlockByHeight
	b, err := bc.GetBlockByHeight(1)
	if err != nil {
		t.Fatalf("GetBlockByHeight(1) error = %v", err)
	}
	if !bytes.Equal(b.Hash, block1.Hash) {
		t.Errorf("GetBlockByHeight(1) returned wrong block")
	}
	_, err = bc.GetBlockByHeight(3) // Non-existent height
	if err == nil {
		t.Errorf("GetBlockByHeight(3) error = nil; want error (out of range)")
	}
	_, err = bc.GetBlockByHeight(-1) // Negative height
	if err == nil {
		t.Errorf("GetBlockByHeight(-1) error = nil; want error (out of range)")
	}


	// GetBlockByHash
	b, err = bc.GetBlockByHash(block2.Hash)
	if err != nil {
		t.Fatalf("GetBlockByHash(block2_hash) error = %v", err)
	}
	if !bytes.Equal(b.Hash, block2.Hash) {
		t.Errorf("GetBlockByHash(block2_hash) returned wrong block")
	}
	_, err = bc.GetBlockByHash([]byte("non_existent_hash"))
	if err == nil {
		t.Errorf("GetBlockByHash(non_existent_hash) error = nil; want error")
	}

	// GetLastBlock
	last, err := bc.GetLastBlock()
	if err != nil {
		t.Fatalf("GetLastBlock() error = %v", err)
	}
	if !bytes.Equal(last.Hash, block2.Hash) {
		t.Errorf("GetLastBlock() returned wrong block")
	}
}

func TestAddBlockToEmptyBlockchain(t *testing.T) {
	// Test adding a non-genesis block to an empty (hypothetical) blockchain
	// This scenario shouldn't happen if NewBlockchain always creates genesis.
	// But AddBlock itself should handle it by expecting height 0.
	bcEmpty := &Blockchain{ // Manually create an empty one for this test
		blocks:     make([]*Block, 0),
		blockIndex: make(map[string]*Block),
	}

	// Try adding a block with height 1 (should fail)
	blockHeight1 := NewBlock(1, []byte("some_prev_hash"), []byte("data"))
	blockHeight1.SetHash()
	err := bcEmpty.AddBlock(blockHeight1)
	if err == nil {
		t.Errorf("Adding block with height 1 to empty chain should fail")
	}
	if !bytes.Contains([]byte(err.Error()), []byte("invalid height for first block")) {
		t.Errorf("Expected 'invalid height for first block' error, got: %v", err)
	}

	// Try adding a block with height 0 (should pass, becoming the first block)
	blockHeight0 := NewBlock(0, []byte("any_prev_hash_for_first"), []byte("first_block_data"))
	blockHeight0.ProposerAddress = "first_proposer"; blockHeight0.Sign("first_proposer", nil);
	blockHeight0.SetHash()
	err = bcEmpty.AddBlock(blockHeight0)
	if err != nil {
		t.Errorf("Adding block with height 0 to empty chain failed: %v", err)
	}
	if bcEmpty.ChainHeight() != 0 {
		t.Errorf("ChainHeight after adding first block (height 0) = %d; want 0", bcEmpty.ChainHeight())
	}
	if len(bcEmpty.blocks) != 1 {
		t.Errorf("Number of blocks after adding first block = %d; want 1", len(bcEmpty.blocks))
	}
}
