package consensus

import (
	"bytes"
	"empower1.com/core/internal/core"
	"encoding/gob"
	"fmt"
	// "log" // For debugging
	"time"
	// "crypto/rand" // For real key generation and signing
	// "golang.org/x/crypto/ed25519" // For real Ed25519 keys
)

// ProposerService is responsible for creating new block proposals when it's the node's turn.
type ProposerService struct {
	validatorAddress string
	privateKey       []byte // Placeholder for actual private key
	mempool          MempoolRetriever
}

// MempoolRetriever defines the interface for fetching transactions from a mempool.
// This avoids a direct dependency on the mempool package, adhering to dependency inversion.
type MempoolRetriever interface {
	GetPendingTransactions(maxCount int) []*core.Transaction
	// RemoveTransactions might be called elsewhere after block is committed
}


// NewProposerService creates a new ProposerService.
// In a real app, privateKey would be loaded securely.
func NewProposerService(validatorAddress string, privateKey []byte, mp MempoolRetriever) *ProposerService {
	return &ProposerService{
		validatorAddress: validatorAddress,
		privateKey:       privateKey, // This is highly simplified.
		mempool:          mp,
	}
}

const maxTxsPerBlock = 100 // Max transactions to include in a block

// CreateProposalBlock creates a new block proposal.
// It gathers transactions from the mempool, signs the block, and sets the hash.
func (ps *ProposerService) CreateProposalBlock(height int64, prevBlockHash []byte, prevBlockTimestamp int64) (*core.Block, error) {
	if ps.validatorAddress == "" {
		return nil, fmt.Errorf("proposer service not configured with a validator address")
	}
	if ps.mempool == nil {
		return nil, fmt.Errorf("proposer service not configured with a mempool")
	}

	// 1. Gather transactions from the mempool
	pendingTxs := ps.mempool.GetPendingTransactions(maxTxsPerBlock)
	var blockData []byte
	var err error

	if len(pendingTxs) > 0 {
		// Serialize transactions for Block.Data
		// Using gob encoding for the slice of transactions
		var txDataBuf bytes.Buffer
		encoder := gob.NewEncoder(&txDataBuf)
		if err = encoder.Encode(pendingTxs); err != nil {
			return nil, fmt.Errorf("failed to serialize transactions for block data: %w", err)
		}
		blockData = txDataBuf.Bytes()
		// log.Printf("ProposerService: Including %d transactions in block #%d\n", len(pendingTxs), height)
	} else {
		blockData = []byte{} // Empty block if no transactions
		// log.Printf("ProposerService: No transactions in mempool for block #%d\n", height)
	}

	// 2. Create the block structure
	block := core.NewBlock(height, prevBlockHash, blockData)

	// Ensure block timestamp is greater than previous block's timestamp
	// And not too far in the future (basic validation)
	currentTime := time.Now().UnixNano()
	if currentTime <= prevBlockTimestamp && height > 0 {
		block.Timestamp = prevBlockTimestamp + 1
	} else {
		block.Timestamp = currentTime
	}
	if block.Timestamp > time.Now().Add(10*time.Second).UnixNano() {
		block.Timestamp = time.Now().UnixNano()
	}

	// 3. Sign the block
	err = block.Sign(ps.validatorAddress, ps.privateKey)
	if err != nil {
		return nil, fmt.Errorf("failed to sign block proposal: %w", err)
	}

	// 4. Calculate and set the final hash
	block.SetHash()

	// log.Printf("ProposerService: Created proposal block #%d by %s with %d txs. Hash: %x\n", block.Height, ps.validatorAddress, len(pendingTxs), block.Hash)
	return block, nil
}

// Note: The actual removal of transactions from the mempool after they are included in a block
// should be handled by the component that confirms block finality and updates the blockchain state.
// The ProposerService only reads from the mempool.
