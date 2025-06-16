package core

import (
	"bytes" // Added import for bytes.Equal
	"fmt"
	"sync"
	"time"
)

// Blockchain represents the chain of blocks.
// For now, it's a simple in-memory list.
type Blockchain struct {
	mu         sync.RWMutex
	blocks     []*Block
	blockIndex map[string]*Block // For quick lookup by hash
}

// NewBlockchain creates a new Blockchain instance and initializes it with a genesis block.
func NewBlockchain() *Blockchain {
	bc := &Blockchain{
		blocks:     make([]*Block, 0),
		blockIndex: make(map[string]*Block),
	}
	genesis := bc.createGenesisBlock()
	bc.AddBlock(genesis) // AddBlock handles locking and indexing
	return bc
}

func (bc *Blockchain) createGenesisBlock() *Block {
	genesisBlock := NewBlock(0, []byte("genesis_prev_hash"), []byte("Genesis Block Data"))
	// For genesis, proposer and signature might be nil or special values
	genesisBlock.ProposerAddress = "genesis_proposer"
	genesisBlock.Signature = []byte("genesis_signature")
	genesisBlock.Timestamp = time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC).UnixNano() // Fixed timestamp
	genesisBlock.SetHash() // Calculate hash after all fields are set
	fmt.Printf("Genesis Block created. Hash: %x\n", genesisBlock.Hash)
	return genesisBlock
}

// AddBlock appends a block to the blockchain.
// It assumes the block has already been validated by the consensus engine.
func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()

	// Check for duplicate block hash first, as this is a cheap check.
	if _, exists := bc.blockIndex[string(block.Hash)]; exists {
		return fmt.Errorf("block with hash %x already exists in chain", block.Hash)
	}

	if len(bc.blocks) > 0 {
		lastBlock := bc.blocks[len(bc.blocks)-1]
		if block.Height != lastBlock.Height+1 {
			return fmt.Errorf("invalid block height: expected %d, got %d (last block height %d)", lastBlock.Height+1, block.Height, lastBlock.Height)
		}
		if !bytes.Equal(block.PrevBlockHash, lastBlock.Hash) { // Use bytes.Equal for []byte comparison
			return fmt.Errorf("invalid previous block hash: expected %x, got %x", lastBlock.Hash, block.PrevBlockHash)
		}
	} else { // This is the first block (genesis)
		if block.Height != 0 {
			return fmt.Errorf("invalid height for first block: expected 0, got %d", block.Height)
		}
		// For genesis, PrevBlockHash might be a specific value or empty, not necessarily matching a "lastBlock.Hash"
		// This is usually handled by the fact that it's the first block, so the above `if len(bc.blocks) > 0` handles it.
	}

	bc.blocks = append(bc.blocks, block)
	bc.blockIndex[string(block.Hash)] = block
	// fmt.Printf("Block #%d (%x) added to blockchain. PrevHash: %x. Proposer: %s\n", block.Height, block.Hash, block.PrevBlockHash, block.ProposerAddress)
	return nil
}

// GetBlockByHeight returns the block at a given height.
func (bc *Blockchain) GetBlockByHeight(height int64) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if height < 0 || height >= int64(len(bc.blocks)) {
		return nil, fmt.Errorf("block height %d out of range", height)
	}
	return bc.blocks[height], nil
}

// GetBlockByHash returns the block with the given hash.
func (bc *Blockchain) GetBlockByHash(hash []byte) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	block, exists := bc.blockIndex[string(hash)]
	if !exists {
		return nil, fmt.Errorf("block with hash %x not found", hash)
	}
	return block, nil
}

// GetLastBlock returns the latest block in the chain.
func (bc *Blockchain) GetLastBlock() (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if len(bc.blocks) == 0 {
		return nil, fmt.Errorf("blockchain is empty")
	}
	return bc.blocks[len(bc.blocks)-1], nil
}

// ChainHeight returns the current height of the blockchain (height of the last block).
func (bc *Blockchain) ChainHeight() int64 {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	if len(bc.blocks) == 0 {
		return -1 // Or handle as error, but -1 indicates no blocks yet (pre-genesis)
	}
	return bc.blocks[len(bc.blocks)-1].Height
}
