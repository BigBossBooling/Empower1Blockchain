package mempool

import (
	"bytes" // Added import
	"empower1.com/core/internal/core"
	"encoding/gob" // Added import
	"encoding/hex"
	"fmt"
	"sync"
)

const defaultMaxPendingTransactions = 1000 // Default max transactions in mempool

// Mempool holds transactions that are waiting to be included in a block.
type Mempool struct {
	mu              sync.RWMutex
	pending         map[string]*core.Transaction // Key: Transaction ID (hex string)
	maxTransactions int
	// TODO: Add eviction policies if maxTransactions is reached (e.g., by fee, by age)
}

// NewMempool creates a new Mempool.
func NewMempool(maxTransactions int) *Mempool {
	if maxTransactions <= 0 {
		maxTransactions = defaultMaxPendingTransactions
	}
	return &Mempool{
		pending:         make(map[string]*core.Transaction),
		maxTransactions: maxTransactions,
	}
}

// AddTransaction adds a transaction to the mempool after basic validation.
func (mp *Mempool) AddTransaction(tx *core.Transaction) error {
	if tx == nil {
		return fmt.Errorf("cannot add nil transaction to mempool")
	}
	if tx.ID == nil || len(tx.ID) == 0 {
		// Calculate ID if not already set (should be set post-signing)
		id, err := tx.Hash()
		if err != nil {
			return fmt.Errorf("failed to calculate transaction hash for ID: %w", err)
		}
		tx.ID = id // Ensure ID is set
	}
	txIDHex := hex.EncodeToString(tx.ID)

	mp.mu.Lock()
	defer mp.mu.Unlock()

	// Check if mempool is full
	if len(mp.pending) >= mp.maxTransactions {
		// TODO: Implement eviction strategy (e.g., remove oldest or lowest fee)
		return fmt.Errorf("mempool is full, max transactions: %d", mp.maxTransactions)
	}

	// Check if transaction already exists
	if _, exists := mp.pending[txIDHex]; exists {
		return fmt.Errorf("transaction %s already exists in mempool", txIDHex)
	}

	// Basic validation: verify signature before adding
	// This assumes tx.PublicKey is correctly populated.
	valid, err := tx.VerifySignature()
	if err != nil {
		return fmt.Errorf("failed to verify transaction %s signature: %w", txIDHex, err)
	}
	if !valid {
		return fmt.Errorf("transaction %s has an invalid signature", txIDHex)
	}

	// TODO: Add more validation:
	// - Check for sufficient balance (requires access to ledger state)
	// - Check for double spending against blockchain state + mempool state
	// - Nonce validation if applicable

	mp.pending[txIDHex] = tx
	// log.Printf("Mempool: Added transaction %s. Pending: %d\n", txIDHex, len(mp.pending))
	return nil
}

// GetTransaction retrieves a transaction by its ID (hex string).
func (mp *Mempool) GetTransaction(txIDHex string) (*core.Transaction, bool) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	tx, exists := mp.pending[txIDHex]
	return tx, exists
}

// GetPendingTransactions returns a slice of transactions to be included in a block.
// It can select up to maxCount transactions.
// For now, it returns transactions in unspecified order. Later, order by fee or age.
func (mp *Mempool) GetPendingTransactions(maxCount int) []*core.Transaction {
	mp.mu.RLock() // Changed to RLock as we are only reading. If we modify (e.g. mark as selected), need Lock.
	defer mp.mu.RUnlock()

	if maxCount <= 0 || maxCount > len(mp.pending) {
		maxCount = len(mp.pending)
	}

	txs := make([]*core.Transaction, 0, maxCount)
	count := 0
	for _, tx := range mp.pending { // Order is not guaranteed due to map iteration
		if count >= maxCount {
			break
		}
		txs = append(txs, tx)
		count++
	}
	return txs
}

// RemoveTransactions removes a list of transactions from the mempool, typically after they've been mined.
func (mp *Mempool) RemoveTransactions(txs []*core.Transaction) {
	if len(txs) == 0 {
		return
	}
	mp.mu.Lock()
	defer mp.mu.Unlock()

	removedCount := 0
	for _, tx := range txs {
		if tx == nil || tx.ID == nil {
			continue
		}
		txIDHex := hex.EncodeToString(tx.ID)
		if _, exists := mp.pending[txIDHex]; exists {
			delete(mp.pending, txIDHex)
			removedCount++
		}
	}
	// if removedCount > 0 {
	// 	log.Printf("Mempool: Removed %d transactions. Pending: %d\n", removedCount, len(mp.pending))
	// }
}

// PruneByBlockchain removes transactions that are already included in the provided blocks.
// This is useful if a node re-syncs or receives blocks out of order.
func (mp *Mempool) PruneByBlockchain(blocks []*core.Block) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	blockTxs := make(map[string]bool) // Map of tx IDs in the blocks

	for _, block := range blocks {
		if len(block.Data) > 0 { // Assuming block.Data contains serialized transactions
			// This requires a way to deserialize transactions from block.Data
			// For now, let's assume block.Data is a gob-encoded slice of transactions
			var transactionsInBlock []*core.Transaction
			decoder := gob.NewDecoder(bytes.NewReader(block.Data))
			if err := decoder.Decode(&transactionsInBlock); err == nil {
				for _, tx := range transactionsInBlock {
					if tx != nil && tx.ID != nil {
						blockTxs[hex.EncodeToString(tx.ID)] = true
					}
				}
			} // else: log error or handle different data formats
		}
	}

	if len(blockTxs) == 0 {
		return
	}

	removedCount := 0
	for txIDHex := range mp.pending {
		if blockTxs[txIDHex] {
			delete(mp.pending, txIDHex)
			removedCount++
		}
	}
	// if removedCount > 0 {
	// 	log.Printf("Mempool: Pruned %d transactions already in blockchain. Pending: %d\n", removedCount, len(mp.pending))
	// }
}


// Count returns the number of pending transactions in the mempool.
func (mp *Mempool) Count() int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return len(mp.pending)
}
