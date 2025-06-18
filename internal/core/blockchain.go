package core

import (
	"bytes"
	"errors" // For specific error types
	"fmt"
	"log"    // For structured logging
	"sync"
	"time"

	// Potentially for crypto if not in Block directly
	// "crypto/rand"
	// "crypto/ed25519"
	// "encoding/hex"
)

// Define custom errors for the Blockchain manager, ensuring clear failure states.
var (
	ErrBlockchainEmpty       = errors.New("blockchain is empty")
	ErrBlockNotFound         = errors.New("block not found")
	ErrInvalidBlockHeight    = errors.New("invalid block height")
	ErrInvalidPrevBlockHash  = errors.New("invalid previous block hash")
	ErrBlockAlreadyExists    = errors.New("block with this hash already exists")
	ErrGenesisBlockMalformed = errors.New("genesis block malformed")
)

// Blockchain represents the chain of blocks.
// It's designed as an in-memory representation for core logic, but production
// would require persistent storage (e.g., database integration).
type Blockchain struct {
	mu         sync.RWMutex        // Mutex for concurrent access to blocks and blockIndex
	blocks     []*Block            // Ordered list of blocks in the chain
	blockIndex map[string]*Block   // For quick lookup of blocks by their hash
	logger     *log.Logger         // Dedicated logger for the blockchain instance
}

// NewBlockchain creates a new Blockchain instance and initializes it with a genesis block.
// This is the genesis point of our digital ecosystem.
func NewBlockchain() *Blockchain {
	// Initialize a logger for the blockchain instance
	logger := log.New(os.Stdout, "BLOCKCHAIN: ", log.Ldate|log.Ltime|log.Lshortfile)

	bc := &Blockchain{
		blocks:     make([]*Block, 0),
		blockIndex: make(map[string]*Block),
		logger:     logger,
	}

	// Create and add the genesis block during initialization, ensuring "Know Your Core, Keep it Clear".
	genesis, err := bc.createGenesisBlock()
	if err != nil {
		// If genesis block creation fails, the blockchain cannot be initialized.
		// This is a critical failure, so we panic or return an error from NewBlockchain.
		panic(fmt.Sprintf("Failed to create genesis block: %v", err))
	}
	
	// AddBlock handles locking and indexing
	if err := bc.AddBlock(genesis); err != nil {
		// This should not happen for a valid, first genesis block, but good for robustness.
		panic(fmt.Sprintf("Failed to add genesis block to blockchain: %v", err))
	}
	bc.logger.Printf("Genesis Block added. Hash: %x. Height: %d\n", genesis.Hash, genesis.Height)
	return bc
}

// createGenesisBlock creates the very first block in the blockchain.
// It's a special block with specific, fixed parameters.
func (bc *Blockchain) createGenesisBlock() (*Block, error) {
	// Genesis block data is typically fixed and known.
	// For EmPower1, it might contain initial distribution or a manifest of rules.
	genesisTransactions := []Transaction{
		// Example: A special initial distribution or rule-setting transaction
		newDummyTx(StandardTx, "EmPower1_Genesis_Distribution_Tx", 0, 0),
	}
	
	genesisBlock := NewBlock(0, bytes.Repeat([]byte{0x00}, sha256.Size), genesisTransactions) // PrevHash is all zeros
	
	// Genesis proposer and signature are often hardcoded or set by a trusted entity.
	// This ensures the chain's origin is undeniable.
	genesisProposerAddress := []byte("EmPower1GenesisProposer") // A fixed, known address
	genesisPrivateKey := []byte("hardcoded_genesis_private_key") // Use actual private key in production

	// Sign the genesis block
	if err := genesisBlock.Sign(genesisProposerAddress, genesisPrivateKey); err != nil {
		return nil, fmt.Errorf("failed to sign genesis block: %w", err)
	}
	
	genesisBlock.SetHash() // Calculate hash after all fields are set

	if genesisBlock.Hash == nil || len(genesisBlock.Hash) != sha256.Size {
		return nil, ErrGenesisBlockMalformed // Ensure hash is correctly set
	}
	
	// Conceptual: Genesis block might also contain an initial AI audit log specific to its creation
	// genesisBlock.AIAuditLog, _ = ai_ml_module.GenerateAIAuditLog(genesisBlock)

	bc.logger.Printf("Genesis Block conceptually created. Hash: %x\n", genesisBlock.Hash)
	return genesisBlock, nil
}

// AddBlock appends a block to the blockchain after basic validation.
// It assumes the block has already undergone full consensus validation by the PoS engine.
// This function is critical for maintaining chain integrity.
func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mu.Lock() // Ensure thread-safe access
	defer bc.mu.Unlock()

	// 1. Check for duplicate block hash early (efficient preliminary check).
	if _, exists := bc.blockIndex[string(block.Hash)]; exists {
		bc.logger.Printf("BLOCK_ADD_ERROR: Block with hash %x already exists. Height: %d", block.Hash, block.Height)
		return ErrBlockAlreadyExists
	}

	// 2. Validate block structure and cryptographic integrity.
	// These checks are fundamental for "Sense the Landscape, Secure the Solution".
	if block.Hash == nil || len(block.Hash) != sha256.Size {
		return fmt.Errorf("block hash is nil or invalid length: %x", block.Hash)
	}
	// Verify the block's own hash is correctly calculated (optional, as SetHash should ensure this)
	// tempBlock := *block
	// tempBlock.Hash = nil // Clear hash for recalculation
	// tempBlock.SetHash()
	// if !bytes.Equal(block.Hash, tempBlock.Hash) {
	// 	return fmt.Errorf("block's calculated hash (%x) does not match provided hash (%x)", tempBlock.Hash, block.Hash)
	// }

	// Verify the block's signature using its proposer address.
	isValidSignature, err := block.VerifySignature()
	if err != nil {
		return fmt.Errorf("signature verification failed for block %d (%x): %w", block.Height, block.Hash, err)
	}
	if !isValidSignature {
		return ErrInvalidSignature
	}

	// 3. Validate chain continuity for non-genesis blocks.
	if len(bc.blocks) > 0 { // For any block after genesis
		lastBlock := bc.blocks[len(bc.blocks)-1]
		
		// Ensure height is sequential.
		if block.Height != lastBlock.Height+1 {
			bc.logger.Printf("BLOCK_ADD_ERROR: Invalid block height. Expected %d, got %d (last %d). Block hash: %x", lastBlock.Height+1, block.Height, lastBlock.Height, block.Hash)
			return ErrInvalidBlockHeight
		}
		// Ensure previous block hash matches the last block in the chain.
		if !bytes.Equal(block.PrevBlockHash, lastBlock.Hash) {
			bc.logger.Printf("BLOCK_ADD_ERROR: Invalid previous block hash. Expected %x, got %x. Block hash: %x", lastBlock.Hash, block.PrevBlockHash, block.Hash)
			return ErrInvalidPrevBlockHash
		}
		// Validate transactions within the block (e.g., basic format, IDs, AI flags)
		if err := block.ValidateTransactions(); err != nil { // This is where AI/ML flagging comes in
			bc.logger.Printf("BLOCK_ADD_ERROR: Transaction validation failed for block %d (%x): %v", block.Height, block.Hash, err)
			return fmt.Errorf("transaction validation failed: %w", err)
		}

	} else { // This is explicitly for the genesis block
		if block.Height != 0 {
			return ErrInvalidBlockHeight // Genesis must be height 0
		}
		// For genesis, PrevBlockHash should be a specific zero hash or genesis hash,
		// already handled by createGenesisBlock. No need to check against lastBlock.
	}

	// 4. Add the block to the chain and index. This is the "Iterate Intelligently" step.
	bc.blocks = append(bc.blocks, block)
	bc.blockIndex[string(block.Hash)] = block
	
	bc.logger.Printf("BLOCK: Block #%d (%x) added to blockchain. PrevHash: %x. Proposer: %x\n", block.Height, block.Hash, block.PrevBlockHash, block.ProposerAddress)
	return nil
}

// GetBlockByHeight returns the block at a given height.
func (bc *Blockchain) GetBlockByHeight(height int64) (*Block, error) {
	bc.mu.RLock() // Read-lock for safe concurrent access
	defer bc.mu.RUnlock()