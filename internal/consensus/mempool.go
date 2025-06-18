package mempool

import (
	"bytes"
	"errors" // For specific error types
	"fmt"
	"log"    // For structured logging
	"os"     // For log output
	"sort"
	"sync"
	"time"

	"empower1.com/core/core" // Assuming 'core' is the package alias for empower1.com/core/core
)

// Define custom errors for Mempool for clearer failure states.
var (
	ErrMempoolInit         = errors.New("mempool initialization error")
	ErrTransactionExists   = errors.New("transaction already exists in mempool")
	ErrInvalidTransaction  = errors.New("invalid transaction to add to mempool")
	ErrMempoolCapacityFull = errors.New("mempool capacity is full")
	ErrTransactionNotFound = errors.New("transaction not found in mempool")
)

// Mempool manages pending (unconfirmed) transactions.
// It's a critical component for network throughput and transaction propagation.
type Mempool struct {
	mu           sync.RWMutex                  // Mutex for concurrent access
	transactions map[string]*core.Transaction  // Store transactions by their hex ID for quick lookup
	order        []string                      // Maintain order of transaction IDs (e.g., by arrival or fee)
	capacity     int                           // Maximum number of transactions the mempool can hold
	logger       *log.Logger                   // Dedicated logger for the Mempool instance
}

// NewMempool creates a new Mempool instance.
// It initializes the in-memory storage for transactions.
func NewMempool(capacity int) (*Mempool, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: mempool capacity must be positive", ErrMempoolInit)
	}
	logger := log.New(os.Stdout, "MEMPOOL: ", log.Ldate|log.Ltime|log.Lshortfile)
	mp := &Mempool{
		transactions: make(map[string]*core.Transaction),
		order:        make([]string, 0),
		capacity:     capacity,
		logger:       logger,
	}
	mp.logger.Printf("Mempool initialized with capacity: %d", capacity)
	return mp, nil
}

// AddTransaction adds a transaction to the mempool.
// It performs basic validation and manages capacity.
// Adheres to "Sense the Landscape, Secure the Solution".
func (mp *Mempool) AddTransaction(tx *core.Transaction) error {
	mp.mu.Lock() // Acquire write lock
	defer mp.mu.Unlock()

	if tx == nil || tx.ID == nil || len(tx.ID) == 0 {
		return ErrInvalidTransaction // Basic check for malformed transaction
	}
	txIDHex := hex.EncodeToString(tx.ID)

	if _, exists := mp.transactions[txIDHex]; exists {
		// Log and return if transaction already exists (prevents duplicates)
		mp.logger.Debugf("MEMPOOL: Transaction %s already exists, ignoring.", txIDHex)
		return ErrTransactionExists
	}

	// Basic structural validation (e.g., verify signature, hash matches content)
	// Full transaction validation (e.g., double-spend, balance checks) happens when proposed for block.
	isValid, err := tx.VerifySignature()
	if err != nil || !isValid {
		mp.logger.Printf("MEMPOOL_WARN: Rejected invalid signature for transaction %s: %v", txIDHex, err)
		return fmt.Errorf("%w: signature invalid or missing for %s", ErrInvalidTransaction, txIDHex)
	}
	txHashCalculated, err := tx.Hash() // Recompute hash to ensure ID matches content
	if err != nil || !bytes.Equal(tx.ID, txHashCalculated) {
		mp.logger.Printf("MEMPOOL_WARN: Rejected transaction %s due to ID/hash mismatch", txIDHex)
		return fmt.Errorf("%w: ID mismatch for %s", ErrInvalidTransaction, txIDHex)
	}

	// EmPower1 Specific: AI/ML pre-screening (conceptual)
	// This is where AI/ML logic for anti-fraud or anomaly detection could pre-screen transactions.
	// aiVerdict, aiErr := ai_ml_module.PreScreenTransaction(tx)
	// if aiErr != nil || !aiVerdict.IsApproved {
	//    mp.logger.Printf("MEMPOOL_WARN: Rejected transaction %s by AI/ML pre-screening: %s", txIDHex, aiVerdict.FlagReason)
	//    return fmt.Errorf("%w: AI/ML pre-screening rejected transaction %s", ErrInvalidTransaction, txIDHex)
	// }


	// Manage capacity - Eviction policy (e.g., FIFO, lowest fee, oldest, random)
	if len(mp.transactions) >= mp.capacity {
		// For simplicity, here we'll just reject. A real mempool would evict.
		mp.logger.Warnf("MEMPOOL_WARN: Mempool full (%d/%d), cannot add transaction %s", len(mp.transactions), mp.capacity, txIDHex)
		return ErrMempoolCapacityFull
	}

	// Add to map and ordered list
	mp.transactions[txIDHex] = tx
	mp.order = append(mp.order, txIDHex) // Simple FIFO for 'order' list

	mp.logger.Printf("MEMPOOL: Added transaction %s (Type: %s, From: %x)", txIDHex, tx.TxType, tx.From)
	return nil
}

// GetPendingTransactions retrieves a list of pending transactions, typically for block proposers.
// This implements the MempoolRetriever interface.
// EmPower1: This selection logic is critical for equity and AI/ML influence.
func (mp *Mempool) GetPendingTransactions(maxCount int) []*core.Transaction {
	mp.mu.RLock() // Acquire read lock
	defer mp.mu.RUnlock()

	if len(mp.transactions) == 0 {
		return []*core.Transaction{}
	}

	// 1. Transaction Selection Strategy (Key for EmPower1's mission)
	// For V1: Simple selection based on internal order.
	// V2+: Prioritize based on:
	//   - Transaction Type (e.g., StimulusTx might get higher priority over StandardTx)
	//   - Transaction Fee (higher fees get priority)
	//   - AI/ML assessed Social Impact Score (Conceptual: tx.Metadata["social_score"])
	//   - Age (older transactions get priority to prevent starvation)
	// This selection process is part of "Systematize for Scalability, Synchronize for Synergy".

	selectedTxs := make([]*core.Transaction, 0, maxCount)
	count := 0
	
	// A more sophisticated selection would build a priority queue or sort based on criteria.
	// For now, iterate through `order` and select.
	for _, txIDHex := range mp.order {
		if tx, found := mp.transactions[txIDHex]; found {
			selectedTxs = append(selectedTxs, tx)
			count++
			if count >= maxCount {
				break
			}
		}
	}
	
	mp.logger.Debugf("MEMPOOL: Retrieved %d pending transactions (max %d).", len(selectedTxs), maxCount)
	return selectedTxs
}

// RemoveTransactions removes a list of transactions from the mempool.
// This is typically called after transactions are confirmed in a finalized block.
func (mp *Mempool) RemoveTransactions(txIDs [][]byte) {
	mp.mu.Lock() // Acquire write lock
	defer mp.mu.Unlock()

	for _, id := range txIDs {
		txIDHex := hex.EncodeToString(id)
		if _, exists := mp.transactions[txIDHex]; exists {
			delete(mp.transactions, txIDHex)
			// Remove from `order` list - inefficient for large lists, but fine for V1.
			// A production mempool would use a more efficient data structure (e.g., linked list, SkipList).
			for i, orderedID := range mp.order {
				if orderedID == txIDHex {
					mp.order = append(mp.order[:i], mp.order[i+1:]...)
					break
				}
			}
			mp.logger.Debugf("MEMPOOL: Removed transaction %s (confirmed).", txIDHex)
		} else {
			mp.logger.Warnf("MEMPOOL_WARN: Attempted to remove non-existent transaction %s from mempool.", txIDHex)
		}
	}
}

// Size returns the current number of transactions in the mempool.
func (mp *Mempool) Size() int {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	return len(mp.transactions)
}

// Contains checks if a transaction with the given ID exists in the mempool.
func (mp *Mempool) Contains(txID []byte) bool {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	_, exists := mp.transactions[hex.EncodeToString(txID)]
	return exists
}

// GetTransactionByID retrieves a transaction from the mempool by its ID.
func (mp *Mempool) GetTransactionByID(txID []byte) (*core.Transaction, error) {
    mp.mu.RLock()
    defer mp.mu.RUnlock()
    tx, exists := mp.transactions[hex.EncodeToString(txID)]
    if !exists {
        return nil, ErrTransactionNotFound
    }
    return tx, nil
}