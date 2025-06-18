package consensus

import (
	"bytes"
	"empower1.com/core/core" // Corrected import path, assuming 'core' is the package alias for empower1.com/core/core
	"encoding/gob"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output
	"time"
)

// Define custom errors specific to ValidationService for clearer failure states.
var (
	ErrValidationServiceInit = errors.New("validation service initialization error")
	ErrNilBlock              = errors.New("cannot validate a nil block")
	ErrBlockIntegrity        = errors.New("block integrity check failed")
	ErrChainContinuity       = errors.New("chain continuity check failed")
	ErrTimeProtocol          = errors.New("time protocol violation")
	ErrProposerMismatch      = errors.New("proposer mismatch")
	ErrSignatureVerification = errors.New("signature verification failed")
	ErrBlockHashMismatch     = errors.New("block hash mismatch")
	ErrTransactionIntegrity  = errors.New("transaction integrity check failed")
)

// ValidationService is responsible for validating blocks according to PoS consensus rules.
// It acts as the primary gatekeeper for blocks entering the blockchain.
type ValidationService struct {
	consensusState *ConsensusState
	blockchain     *core.Blockchain // Direct dependency on the core blockchain instance
	logger         *log.Logger      // Dedicated logger for the ValidationService
}

// NewValidationService creates a new ValidationService instance.
// It takes dependencies (ConsensusState, Blockchain) to perform its checks.
func NewValidationService(cs *ConsensusState, bc *core.Blockchain) (*ValidationService, error) {
	if cs == nil {
		return nil, fmt.Errorf("%w: ConsensusState cannot be nil", ErrValidationServiceInit)
	}
	if bc == nil {
		return nil, fmt.Errorf("%w: Blockchain cannot be nil", ErrValidationServiceInit)
	}

	logger := log.New(os.Stdout, "VALIDATION: ", log.Ldate|log.Ltime|log.Lshortfile)
	
	vs := &ValidationService{
		consensusState: cs,
		blockchain:     bc,
		logger:         logger,
	}
	vs.logger.Println("ValidationService initialized.")
	return vs, nil
}

// ValidateBlock performs a comprehensive set of checks on a block to ensure it conforms to consensus rules.
// This is the core function for maintaining blockchain integrity and security.
// It directly supports "Sense the Landscape, Secure the Solution" for EmPower1.
func (vs *ValidationService) ValidateBlock(block *core.Block) error {
	if block == nil {
		return ErrNilBlock
	}
	
	vs.logger.Printf("VALIDATING: Block #%d (%x) proposed by %x. PrevHash: %x", 
		block.Height, block.Hash, block.ProposerAddress, block.PrevBlockHash)

	// --- 1. Basic Structural & Cryptographic Integrity Checks (First Line of Defense) ---
	// These are fundamental checks that can prevent processing malformed blocks.
	if block.Hash == nil || len(block.Hash) != sha256.Size {
		return fmt.Errorf("%w: block hash is missing or has invalid length (%d bytes, expected %d)", ErrBlockIntegrity, len(block.Hash), sha256.Size)
	}
	if block.ProposerAddress == nil || len(block.ProposerAddress) == 0 {
		return fmt.Errorf("%w: block proposer address is missing", ErrBlockIntegrity)
	}
	if block.Signature == nil || len(block.Signature) == 0 {
		return fmt.Errorf("%w: block signature is missing", ErrBlockIntegrity)
	}

	// Re-verify block hash (ensure it matches the content it claims to hash)
	// This is a critical check against block tampering.
	originalHash := block.Hash
	tempBlock := *block // Create a copy to recalculate hash without modifying original
	tempBlock.Hash = nil // Clear current hash for recalculation
	tempBlock.SetHash() // Recalculate hash based on received content
	
	if !bytes.Equal(originalHash, tempBlock.Hash) {
		vs.logger.Printf("SLASHING_EVENT (LOG): Block #%d proposed by %x - Content hash mismatch. Received %x, Calculated %x\n", 
			block.Height, block.ProposerAddress, originalHash, tempBlock.Hash)
		return fmt.Errorf("%w: block content (%x) does not match its claimed hash (%x)", ErrBlockHashMismatch, tempBlock.Hash, originalHash)
	}
	// Restore original hash to the block (as we validated it, not changed it)
	block.Hash = originalHash


	// --- 2. Chain Continuity & Time Protocol Checks (Ensuring Law of Constant Progression) ---
	if block.Height > 0 { // For all blocks after genesis
		lastBlock, err := vs.blockchain.GetLastBlock()
		if err != nil {
			// This indicates a severe issue with the local blockchain state if not height 0.
			if errors.Is(err, core.ErrBlockchainEmpty) { // If our chain is unexpectedly empty
				return fmt.Errorf("%w: local blockchain is empty, cannot validate non-genesis block #%d", ErrChainContinuity, block.Height)
			}
			return fmt.Errorf("%w: failed to get last block for chain continuity check: %v", ErrChainContinuity, err)
		}

		if block.Height != lastBlock.Height+1 {
			return fmt.Errorf("%w: invalid block height. Expected %d, got %d (last block %d)", ErrChainContinuity, lastBlock.Height+1, block.Height, lastBlock.Height)
		}
		if !bytes.Equal(block.PrevBlockHash, lastBlock.Hash) {
			return fmt.Errorf("%w: invalid previous block hash. Expected %x, got %x", ErrChainContinuity, lastBlock.Hash, block.PrevBlockHash)
		}
		
		// Block timestamp must be strictly after previous block's timestamp
		if block.Timestamp <= lastBlock.Timestamp {
			return fmt.Errorf("%w: block timestamp (%d) must be strictly after previous block timestamp (%d)", ErrTimeProtocol, block.Timestamp, lastBlock.Timestamp)
		}
	} else { // Specifically for genesis block (Height == 0)
		if vs.blockchain.ChainHeight() != -1 && vs.blockchain.ChainHeight() != 0 {
			// This means we already have a blockchain, so a new genesis shouldn't be added.
			return fmt.Errorf("%w: blockchain already initialized, cannot accept new genesis block at height 0", ErrChainContinuity)
		}
		// For genesis, PrevBlockHash should be the designated genesis hash (all zeros)
		if !bytes.Equal(block.PrevBlockHash, bytes.Repeat([]byte{0x00}, sha256.Size)) {
			return fmt.Errorf("%w: genesis block PrevBlockHash is incorrect", ErrChainContinuity)
		}
		// Genesis timestamp can be anything in the past, but typical fixed.
	}
	
	// Check block timestamp relative to current node time (clock drift tolerance)
	// This helps prevent blocks from the far past/future due to clock skew or attacks.
	maxDriftFuture := 10 * time.Second // Max 10s in the future
	maxDriftPast := 5 * time.Minute    // Max 5 min in the past (more lenient for network sync)

	currentTime := time.Now()
	blockTime := time.Unix(0, block.Timestamp)

	if blockTime.After(currentTime.Add(maxDriftFuture)) {
		return fmt.Errorf("%w: block timestamp %s is too far in the future (current time %s)", ErrTimeProtocol, blockTime, currentTime)
	}
	// For non-genesis blocks, ensure it's not excessively old relative to now, assuming active network
	if block.Height > 0 && blockTime.Before(currentTime.Add(-maxDriftPast)) {
		vs.logger.Printf("VALIDATION_WARN: Block #%d timestamp %s is significantly old (current time %s). Possible network latency or old proposal.\n", block.Height, blockTime, currentTime)
		// Depending on network rules, this might be a warning or a soft rejection.
		// For now, it's a warning, but could be an error in stricter production.
	}


	// --- 3. Proposer Legitimacy & Signature Verification ---
	// This ensures the block was proposed by the correct validator and is cryptographically signed.
	expectedProposer, err := vs.consensusState.GetProposerForHeight(block.Height)
	if err != nil {
		return fmt.Errorf("%w: failed to determine expected proposer for height %d: %v", ErrProposerMismatch, block.Height, err)
	}
	// Verify proposer address matches the expected one for this height.
	if !bytes.Equal(block.ProposerAddress, expectedProposer.Address) {
		vs.logger.Printf("SLASHING_EVENT (LOG): Block #%d proposed by %x, expected %x. (Wrong proposer address)\n", block.Height, block.ProposerAddress, expectedProposer.Address)
		return fmt.Errorf("%w: block proposed by wrong validator: expected %x, got %x", ErrProposerMismatch, expectedProposer.Address, block.ProposerAddress)
	}

	// Verify block signature using the block's content