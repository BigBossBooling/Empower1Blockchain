package consensus

import (
	"bytes"
	"errors"
	"fmt"
	"log" // Using standard log package for better control
	"os"   // For log output
	"time"

	// Ensure this import path matches your core package structure
	"empower1.com/core/core" 

	// Placeholder for actual crypto library, e.g., Ed25519 or ECDSA utils
	// "empower1.com/core/internal/crypto" // Assuming this has key types/unmarshalling
)

// Define custom errors specific to the ProposerService for clearer failure states.
var (
	ErrProposerNotConfigured = errors.New("proposer service not configured")
	ErrMempoolUnavailable    = errors.New("mempool retriever not configured")
	ErrTransactionSerialization = errors.New("failed to serialize transactions for block data")
	ErrBlockSigningFailed    = errors.New("failed to sign block proposal")
	ErrInvalidBlockTimestamp = errors.New("invalid block timestamp")
)

// ProposerService is responsible for creating new block proposals when it's the node's turn.
// It gathers transactions, constructs the block, signs it, and calculates its hash.
type ProposerService struct {
	validatorAddress []byte // Public key bytes of the validator (immutable for this instance)
	privateKey       []byte // Corresponding private key bytes (highly sensitive)
	mempool          MempoolRetriever
	logger           *log.Logger // Dedicated logger for proposer service
}

// MempoolRetriever defines the interface for fetching transactions from a mempool.
// This adheres to the dependency inversion principle, making ProposerService testable and modular.
type MempoolRetriever interface {
	GetPendingTransactions(maxCount int) []*core.Transaction
	// Note: Removal of transactions from mempool should happen after block finalization/commitment
	// by a separate service that observes chain state. Proposer only reads.
}

// NewProposerService creates a new ProposerService instance.
// It requires the validator's address (public key bytes) and corresponding private key bytes.
// In a real application, privateKey would be loaded securely (e.g., from a KMS or encrypted storage).
func NewProposerService(validatorAddress []byte, privateKey []byte, mp MempoolRetriever) (*ProposerService, error) {
	if len(validatorAddress) == 0 {
		return nil, fmt.Errorf("%w: validator address cannot be empty", ErrProposerNotConfigured)
	}
	if len(privateKey) == 0 {
		return nil, fmt.Errorf("%w: private key cannot be empty", ErrProposerNotConfigured)
	}
	if mp == nil {
		return nil, fmt.Errorf("%w: mempool retriever cannot be nil", ErrMempoolUnavailable)
	}

	logger := log.New(os.Stdout, "PROPOSER: ", log.Ldate|log.Ltime|log.Lshortfile) // Initialize logger
	
	ps := &ProposerService{
		validatorAddress: validatorAddress,
		privateKey:       privateKey,
		mempool:          mp,
		logger:           logger,
	}
	ps.logger.Printf("ProposerService initialized for validator: %x", ps.validatorAddress)
	return ps, nil
}

const (
	maxTxsPerBlock    = 100 // Max transactions to include in a block, aligning with "Systematize for Scalability"
	maxFutureTimeOffset = 5 * time.Second // Max allowable future timestamp for a block
	minTimeBetweenBlocks = 1 * time.Nanosecond // Smallest allowed time diff for sequential blocks
)

// CreateProposalBlock creates a new block proposal.
// It orchestrates transaction gathering, block structuring, signing, and hashing.
// This function is central to the PoS consensus "Kinetic System".
func (ps *ProposerService) CreateProposalBlock(height int64, prevBlockHash []byte, prevBlockTimestamp int64) (*core.Block, error) {
	if ps.validatorAddress == nil || len(ps.validatorAddress) == 0 || ps.privateKey == nil || len(ps.privateKey) == 0 {
		return nil, ErrProposerNotConfigured
	}
	if ps.mempool == nil {
		return nil, ErrMempoolUnavailable
	}

	ps.logger.Printf("Creating proposal for block #%d. PrevHash: %x. PrevTimestamp: %d", height, prevBlockHash, prevBlockTimestamp)

	// 1. Gather transactions from the mempool (Input: current blockchain state, Mempool)
	// This ensures our block is useful and contains relevant pending transactions.
	pendingTxs := ps.mempool.GetPendingTransactions(maxTxsPerBlock)
	
	// 2. Validate transactions (Conceptual: AI/ML pre-validation for EmPower1)
	// This is where AI/ML logic might perform a quick pre-check on transactions
	// before they are even included in a proposed block, enhancing "Sense the Landscape".
	for i, tx := range pendingTxs {
		// Example: Call a conceptual AI/ML module's pre-validation
		// if !ai_ml_module.PreValidateTxForInclusion(tx) {
		// 	ps.logger.Printf("PROPOSER_WARN: Transaction %x rejected by AI pre-validation for block #%d", tx.ID, height)
		// 	pendingTxs = append(pendingTxs[:i], pendingTxs[i+1:]...) // Remove problematic tx
		// }
	}

	// 3. Create the block structure
	block := core.NewBlock(height, prevBlockHash, pendingTxs)

	// 4. Set block timestamp (Crucial for liveness and ordering)
	// Timestamp must be greater than previous block's but not too far in the future.
	currentTime := time.Now().UnixNano()
	if height > 0 && currentTime <= prevBlockTimestamp {
		// If current time is not strictly greater than previous, increment previous timestamp by min allowed.
		// This ensures strict monotonic increase of timestamps for non-genesis blocks.
		block.Timestamp = prevBlockTimestamp + minTimeBetweenBlocks
		ps.logger.Printf("PROPOSER_WARN: Current time %d not greater than prev block timestamp %d. Setting block #%d timestamp to %d (prev + min_diff)", currentTime, prevBlockTimestamp, height, block.Timestamp)
	} else {
		block.Timestamp = currentTime
	}
	// Basic check: prevent proposals with excessively future timestamps.
	if block.Timestamp > time.Now().Add(maxFutureTimeOffset).UnixNano() {
		return nil, fmt.Errorf("%w: block timestamp too far in the future", ErrInvalidBlockTimestamp)
	}
	
	// 5. Sign the block's header
	// The block's ProposerAddress is set here before signing.
	err := block.Sign(ps.validatorAddress, ps.privateKey)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to sign block proposal for block %d: %v", ErrBlockSigningFailed, height, err)
	}

	// 6. Calculate and set the final hash
	// The hash is calculated AFTER signing and all header/finalization fields are set.
	block.SetHash()

	ps.logger.Printf("ProposerService: Created proposal block #%d (%x) by %x with %d txs. PrevHash: %x\n", 
		block.Height, block.Hash, ps.validatorAddress, len(pendingTxs), block.PrevBlockHash)
	return block, nil
}

// Note: The actual removal of transactions from the mempool after they are included in a block
// should be handled by the component that confirms block finality and updates the blockchain state
// (e.g., the `Chain` or `Consensus` manager that commits blocks). The ProposerService only reads from the mempool.