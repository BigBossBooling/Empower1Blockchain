package core

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/empower1/blockchain/internal/consensus"
)

// Blockchain represents the blockchain structure, holding all blocks.
type Blockchain struct {
	lock   sync.RWMutex
	blocks []*Block
	pos    *consensus.POS // The consensus engine
}

// NewBlockchain creates a new blockchain, initialized with the genesis block.
func NewBlockchain(pos *consensus.POS) *Blockchain {
	bc := &Blockchain{
		blocks: []*Block{},
		pos:    pos,
	}
	bc.blocks = append(bc.blocks, NewGenesisBlock())
	return bc
}

// AddBlock adds a new block to the blockchain after validating it.
func (bc *Blockchain) AddBlock(b *Block) error {
	bc.lock.Lock()
	defer bc.lock.Unlock()

	if err := bc.validateBlock_nolock(b); err != nil {
		return err
	}

	bc.blocks = append(bc.blocks, b)
	return nil
}

// Height returns the current height of the blockchain (number of blocks).
func (bc *Blockchain) Height() uint64 {
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	return bc.height_nolock()
}

// height_nolock is the internal, non-locking version of Height.
func (bc *Blockchain) height_nolock() uint64 {
	return uint64(len(bc.blocks) - 1)
}

// GetBlockByHeight returns the block at a given height.
func (bc *Blockchain) GetBlockByHeight(height uint64) (*Block, error) {
	bc.lock.RLock()
	defer bc.lock.RUnlock()
	return bc.getBlockByHeight_nolock(height)
}

// getBlockByHeight_nolock is the internal, non-locking version of GetBlockByHeight.
func (bc *Blockchain) getBlockByHeight_nolock(height uint64) (*Block, error) {
	if height > bc.height_nolock() {
		return nil, fmt.Errorf("height %d is too high", height)
	}
	return bc.blocks[height], nil
}

// validateBlock_nolock performs a series of checks to ensure a block is valid before adding it.
func (bc *Blockchain) validateBlock_nolock(b *Block) error {
	// Get the current head of the chain
	currentBlock, err := bc.getBlockByHeight_nolock(bc.height_nolock())
	if err != nil {
		return err
	}

	// Check if the new block's height is correct
	if b.Header.Height != currentBlock.Header.Height+1 {
		return fmt.Errorf("invalid block height: expected %d, got %d", currentBlock.Header.Height+1, b.Header.Height)
	}

	// Check if the previous block hash matches the current block's hash
	if !bytes.Equal(b.Header.PrevBlockHash, currentBlock.Block.Hash) {
		return fmt.Errorf("invalid previous block hash")
	}

	// Re-calculate and validate the new block's hash
	hash, err := b.CalculateHash()
	if err != nil {
		return err
	}
	if !bytes.Equal(hash, b.Block.Hash) {
		return fmt.Errorf("invalid block hash: calculated hash does not match block's hash")
	}

	// Verify the block's signature
	proposerAddress := b.Header.ProposerAddress
	proposer := bc.pos.GetValidator(proposerAddress)
	if proposer == nil {
		return fmt.Errorf("validator not found for proposer address: %s", proposerAddress)
	}

	valid, err := b.VerifySignature(proposer.PublicKey)
	if err != nil {
		return fmt.Errorf("error verifying signature: %v", err)
	}
	if !valid {
		return fmt.Errorf("invalid block signature")
	}

	return nil
}
