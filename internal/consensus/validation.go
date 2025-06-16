package consensus

import (
	"bytes"
	"empower1.com/core/internal/core"
	"encoding/gob"
	"fmt"
	// "log" // For debugging
	"time"
)

// ValidationService is responsible for validating blocks according to PoS consensus rules.
type ValidationService struct {
	consensusState *ConsensusState
	blockchain     *core.Blockchain
}

// NewValidationService creates a new ValidationService.
func NewValidationService(cs *ConsensusState, bc *core.Blockchain) *ValidationService {
	return &ValidationService{
		consensusState: cs,
		blockchain:     bc,
	}
}

// ValidateBlock performs various checks on a block to ensure it conforms to consensus rules.
func (vs *ValidationService) ValidateBlock(block *core.Block) error {
	if block == nil {
		return fmt.Errorf("cannot validate a nil block")
	}

	// 1. Basic structural validation (already partially in Block methods, but can be expanded)
	if block.Hash == nil || len(block.Hash) == 0 {
		return fmt.Errorf("block hash is missing")
	}
	if block.ProposerAddress == "" {
		return fmt.Errorf("block proposer address is missing")
	}
	if block.Signature == nil || len(block.Signature) == 0 {
		return fmt.Errorf("block signature is missing")
	}

	// 2. Check against the current blockchain state
	lastBlock, err := vs.blockchain.GetLastBlock()
	if err != nil {
		// If it's the genesis block being validated (height 0)
		if block.Height == 0 {
			// Genesis block might have different validation rules or be assumed valid if it matches known genesis
			// For now, we assume genesis from NewBlockchain is correct.
			// If this is a received genesis block, it needs to match the expected one.
			// This simple validation service assumes the local blockchain's genesis is the source of truth.
			// So, if we have no last block, and this isn't height 0, it's an error.
			return fmt.Errorf("blockchain is empty, cannot get last block to validate block #%d: %w", block.Height, err)
		}
		// Otherwise, if it's not genesis, it's an error.
		return fmt.Errorf("failed to get last block for validation: %w", err)
	}

	// For non-genesis blocks
	if block.Height > 0 {
		if block.Height != lastBlock.Height+1 {
			return fmt.Errorf("invalid block height: expected %d, got %d (last block %d)", lastBlock.Height+1, block.Height, lastBlock.Height)
		}
		if string(block.PrevBlockHash) != string(lastBlock.Hash) {
			return fmt.Errorf("invalid previous block hash: expected %x, got %x", lastBlock.Hash, block.PrevBlockHash)
		}
		if block.Timestamp <= lastBlock.Timestamp {
			return fmt.Errorf("block timestamp (%d) must be after previous block timestamp (%d)", block.Timestamp, lastBlock.Timestamp)
		}
	}


	// 3. Check block timestamp relative to current node time (clock drift tolerance)
	// Allow some drift, e.g., block timestamp not too far in the future or past.
	// This is simplified. Real networks have more sophisticated time synchronization.
	maxDriftFuture := 10 * time.Second // Max 10s in the future
	maxDriftPast := 1 * time.Minute    // Max 1 min in the past (generous for test nets)

	currentTime := time.Now()
	blockTime := time.Unix(0, block.Timestamp)

	if blockTime.After(currentTime.Add(maxDriftFuture)) {
		return fmt.Errorf("block timestamp %s is too far in the future (current time %s)", blockTime, currentTime)
	}
	// For non-genesis blocks, check it's not too old relative to now
	// Genesis block timestamp can be anything in the past.
	if block.Height > 0 && blockTime.Before(currentTime.Add(-maxDriftPast)) {
		// This check might be too strict if blocks are received after a long network delay.
		// More important is that it's after the previous block.
		// log.Printf("Warning: Block timestamp %s is significantly in the past (current time %s)", blockTime, currentTime)
	}


	// 4. Verify proposer legitimacy
	expectedProposer, err := vs.consensusState.GetProposerForHeight(block.Height)
	if err != nil {
		return fmt.Errorf("failed to determine expected proposer for height %d: %w", block.Height, err)
	}
	if expectedProposer == nil { // Should be caught by the error above, but defensive check
		return fmt.Errorf("no proposer scheduled for height %d", block.Height)
	}
	if block.ProposerAddress != expectedProposer.Address {
		// SLOGAN: Slashing condition - proposed by wrong validator
		// For now, just log. In a real system, this would be evidence for slashing.
		fmt.Printf("SLASHING_EVENT (LOG): Block #%d proposed by %s, expected %s\n", block.Height, block.ProposerAddress, expectedProposer.Address)
		return fmt.Errorf("block proposed by wrong validator: expected %s, got %s", expectedProposer.Address, block.ProposerAddress)
	}

	// 5. Verify block signature
	// This uses the placeholder VerifySignature method in block.go
	validSignature, err := block.VerifySignature()
	if err != nil {
		// SLOGAN: Slashing condition - signature verification error
		fmt.Printf("SLASHING_EVENT (LOG): Block #%d from %s - signature verification error: %v\n", block.Height, block.ProposerAddress, err)
		return fmt.Errorf("error verifying block signature: %w", err)
	}
	if !validSignature {
		// SLOGAN: Slashing condition - invalid signature
		fmt.Printf("SLASHING_EVENT (LOG): Block #%d from %s - invalid signature\n", block.Height, block.ProposerAddress)
		return fmt.Errorf("invalid block signature")
	}

	// 6. Verify block hash (ensure it matches the content)
	// The received hash should match a freshly computed hash.
	originalHash := block.Hash
	block.SetHash() // Recalculate hash based on received content
	if string(originalHash) != string(block.Hash) {
		// SLOGAN: Slashing condition - block content does not match its hash (tampering)
		fmt.Printf("SLASHING_EVENT (LOG): Block #%d from %s - content does not match hash. Received %x, Calculated %x\n", block.Height, block.ProposerAddress, originalHash, block.Hash)
		block.Hash = originalHash // Restore original hash for context if needed elsewhere
		return fmt.Errorf("block content does not match its hash: received %x, calculated %x", originalHash, block.Hash)
	}
	// Restore original hash if it was modified, though for an invalid block, it might not matter.
	// block.Hash = originalHash

	// 7. Validate transactions within the block
	if err := vs.validateBlockTransactions(block.Data, block.Height); err != nil {
		// Specific slashing logs for transaction validation errors are inside validateBlockTransactions
		return fmt.Errorf("block #%d contains invalid transactions: %w", block.Height, err)
	}


	// All checks passed
	// fmt.Printf("ValidationService: Block #%d (%x) by %s validated successfully.\n", block.Height, block.Hash, block.ProposerAddress)
	return nil
}

// validateBlockTransactions deserializes and validates transactions within a block.
func (vs *ValidationService) validateBlockTransactions(blockData []byte, blockHeight int64) error {
	if len(blockData) == 0 {
		// log.Printf("ValidationService: Block #%d has no transaction data to validate.", blockHeight)
		return nil // No transactions to validate
	}

	var txsInBlock []*core.Transaction
	decoder := gob.NewDecoder(bytes.NewReader(blockData))
	if err := decoder.Decode(&txsInBlock); err != nil {
		// This could be a slashing condition: malformed transaction data in block
		fmt.Printf("SLASHING_EVENT (LOG): Block #%d contains malformed transaction data: %v\n", blockHeight, err)
		return fmt.Errorf("failed to deserialize transactions from block data: %w", err)
	}

	if len(txsInBlock) == 0 && len(blockData) > 0 {
		// This case means blockData was not empty but didn't decode to any transactions.
		// This could be an encoding issue or non-tx data.
		// Depending on rules, this might be an error. For now, log a warning.
		// log.Printf("ValidationService: Warning - Block #%d data was non-empty but yielded zero transactions upon deserialization.", blockHeight)
		return nil // Or return an error if empty but non-nil data is disallowed.
	}

	// log.Printf("ValidationService: Validating %d transactions in block #%d.\n", len(txsInBlock), blockHeight)

	for i, tx := range txsInBlock {
		if tx == nil {
			return fmt.Errorf("nil transaction found in block #%d at index %d", blockHeight, i)
		}
		// 1. Verify transaction signature
		valid, err := tx.VerifySignature()
		if err != nil {
			// SLOGAN: Slashing condition - transaction in block has signature verification error
			fmt.Printf("SLASHING_EVENT (LOG): Transaction %x in block #%d failed signature verification: %v\n", tx.ID, blockHeight, err)
			return fmt.Errorf("transaction %x in block #%d failed signature verification: %w", tx.ID, blockHeight, err)
		}
		if !valid {
			// SLOGAN: Slashing condition - transaction in block has invalid signature
			fmt.Printf("SLASHING_EVENT (LOG): Transaction %x in block #%d has invalid signature\n", tx.ID, blockHeight)
			return fmt.Errorf("transaction %x in block #%d has invalid signature", tx.ID, blockHeight)
		}

		// 2. Ensure transaction ID matches its content hash
		currentHash, err := tx.Hash()
		if err != nil {
			return fmt.Errorf("failed to calculate hash for tx %x in block #%d: %w", tx.ID, blockHeight, err)
		}
		if !bytes.Equal(tx.ID, currentHash) {
			// SLOGAN: Slashing condition - transaction ID mismatch (tampering or malformed)
			fmt.Printf("SLASHING_EVENT (LOG): Transaction %x in block #%d has ID that does not match its content hash. ID: %x, Calculated: %x\n", tx.ID, blockHeight, tx.ID, currentHash)
			return fmt.Errorf("transaction %x in block #%d ID does not match its content hash", tx.ID, blockHeight)
		}


		// TODO: Add more transaction validation rules:
		// - Check for sufficient sender balance (requires ledger access)
		// - Check for double spending against current block + parent blocks' state
		// - Validate nonce if applicable
		// - Check transaction fees, limits, etc.
		// log.Printf("ValidationService: Transaction %x in block #%d validated successfully.\n", tx.ID, blockHeight)
	}
	return nil
}


// Conceptual: If a validator proposes an invalid block that is detected by ValidateBlock,
// this information should be used as input for a slashing mechanism.
// For this step, we just log a "SLASHING_EVENT (LOG)".
// True slashing would involve submitting evidence to a smart contract or special transaction
// to penalize the validator (e.g., reduce their stake).
// Example slashing conditions logged above:
// - Wrong proposer
// - Invalid signature
// - Block content doesn't match hash
// - Malformed transaction data in block
// - Invalid transaction signature in block
// - Transaction ID mismatch in block
// - Double spending (would require transaction validation)
// - Proposing multiple different blocks for the same height (equivocation) - harder to detect here, often needs more context.
