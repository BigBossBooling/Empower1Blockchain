package mempool

import (
	"bytes"
	"encoding/gob" // For serializing/deserializing transactions from block.Data
	"encoding/hex"
	"errors" // For custom error types
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
	ErrTxDeserialization   = errors.New("failed to deserialize transaction from block data")
)

const defaultMaxPendingTransactions = 1000 // Default max transactions to hold

// Mempool manages pending (unconfirmed) transactions.
// It's a critical component for network throughput and transaction propagation.
// EmPower1: Transactions are prioritized based on specific criteria like type, fee, and AI/ML insights.
type Mempool struct {
	mu           sync.RWMutex                  // Mutex for concurrent access
	transactions map[string]*core.Transaction  // Store transactions by their hex ID for quick lookup
	priorityQueue []string                     // Maintain ordered list of transaction IDs based on priority/selection rules
	capacity     int                           // Maximum number of transactions the mempool can hold
	logger       *log.Logger                   // Dedicated logger for the Mempool instance
}

// NewMempool creates a new Mempool instance.
// Initializes the in-memory storage for transactions.
func NewMempool(capacity int) (*Mempool, error) {
	if capacity <= 0 {
		return nil, fmt.Errorf("%w: mempool capacity must be positive", ErrMempoolInit)
	}
	logger := log.New(os.Stdout, "MEMPOOL: ", log.Ldate|log.Ltime|log.Lshortfile)
	mp := &Mempool{
		transactions:  make(map[string]*core.Transaction),
		priorityQueue: make([]string, 0), // Use this for ordered selection
		capacity:      capacity,
		logger:        logger,
	}
	mp.logger.Printf("Mempool initialized with capacity: %d", capacity)
	return mp, nil
}

// AddTransaction adds a transaction to the mempool after basic validation.
// It also places the transaction into the priority queue based on a ranking heuristic.
// Adheres to "Sense the Landscape, Secure the Solution" and "Systematize for Scalability".
func (mp *Mempool) AddTransaction(tx *core.Transaction) error {
	mp.mu.Lock() // Acquire write lock
	defer mp.mu.Unlock()

	if tx == nil || tx.ID == nil || len(tx.ID) == 0 {
		mp.logger.Warnf("MEMPOOL_WARN: Attempted to add nil or malformed transaction.")
		return ErrInvalidTransaction
	}
	txIDHex := hex.EncodeToString(tx.ID)

	if _, exists := mp.transactions[txIDHex]; exists {
		mp.logger.Debugf("MEMPOOL: Transaction %s already exists, ignoring.", txIDHex)
		return ErrTransactionExists
	}

	// 1. Basic structural validation (verify signature, hash matches content)
	// These checks are crucial for "Integrity" and preventing "Garbage In".
	isValid, err := tx.VerifySignature()
	if err != nil || !isValid {
		mp.logger.Warnf("MEMPOOL_WARN: Rejected invalid signature for transaction %s: %v", txIDHex, err)
		return fmt.Errorf("%w: signature invalid or missing for %s", ErrInvalidTransaction, txIDHex)
	}
	txHashCalculated, err := tx.Hash() // Recompute hash to ensure ID matches content
	if err != nil || !bytes.Equal(tx.ID, txHashCalculated) {
		mp.logger.Warnf("MEMPOOL_WARN: Rejected transaction %s due to ID/hash mismatch: %v", txIDHex, err)
		return fmt.Errorf("%w: ID mismatch for %s", ErrInvalidTransaction, txIDHex)
	}

	// 2. EmPower1 Specific: AI/ML pre-screening (conceptual hook for advanced validation)
	// This is where AI/ML logic for anti-fraud, anomaly detection, or initial wealth assessment
	// could pre-screen transactions before they consume full mempool resources.
	// aiVerdict, aiErr := ai_ml_module.PreScreenTransaction(tx) // Assuming ai_ml_module exists
	// if aiErr != nil || !aiVerdict.IsApproved {
	//    mp.logger.Warnf("MEMPOOL_WARN: Rejected transaction %s by AI/ML pre-screening: %s", txIDHex, aiVerdict.FlagReason)
	//    return fmt.Errorf("%w: AI/ML pre-screening rejected transaction %s", ErrInvalidTransaction, txIDHex)
	// }

	// 3. Manage capacity - Eviction policy if mempool is full
	if len(mp.transactions) >= mp.capacity {
		// Implement a proper eviction strategy for "Systematize for Scalability".
		// Example: remove the lowest priority/fee tx, or oldest. For now, we'll just reject.
		mp.logger.Warnf("MEMPOOL_WARN: Mempool full (%d/%d), cannot add transaction %s. Capacity management needed.", len(mp.transactions), mp.capacity, txIDHex)
		return ErrMempoolCapacityFull
	}

	// 4. Add to map and priority queue
	mp.transactions[txIDHex] = tx
	
	// EmPower1 Specific: Add to priority queue based on sorting heuristic
	mp.insertIntoPriorityQueue(txIDHex, tx) 

	mp.logger.Printf("MEMPOOL: Added transaction %s (Type: %s, From: %s). Current size: %d", txIDHex, tx.TxType, hex.EncodeToString(tx.From), len(mp.transactions))
	return nil
}

// insertIntoPriorityQueue places a transaction ID into the ordered list based on a scoring function.
// This is critical for EmPower1's unique transaction prioritization (equity, fee, type).
func (mp *Mempool) insertIntoPriorityQueue(txIDHex string, tx *core.Transaction) {
	// Score the transaction (Conceptual for V2+)
	// score := calculateTransactionScore(tx) // Function would consider tx.Fee, tx.TxType, AI/ML insights, age.

	// For V1 simplicity, just append and re-sort. A real PQ would be a heap or skip list.
	mp.priorityQueue = append(mp.priorityQueue, txIDHex)
	
	// Sort the entire queue (INEFFICIENT for large N, but simple for V1/demonstration)
	// In production, this would be a min/max heap or a skip list for O(logN) insertion/retrieval.
	sort.Slice(mp.priorityQueue, func(i, j int) bool {
		txI := mp.transactions[mp.priorityQueue[i]]
		txJ := mp.transactions[mp.priorityQueue[j]]
		
		// EmPower1 Priority Logic (conceptual, aligns with "Know Your Core, Keep it Clear"):
		// 1. StimulusTx always highest priority
		if txI.TxType == core.StimulusTx && txJ.TxType != core.StimulusTx {
			return true // txI (Stimulus) comes before txJ
		}
		if txJ.TxType == core.StimulusTx && txI.TxType != core.StimulusTx {
			return false // txJ (Stimulus) comes before txI
		}
		// 2. Then by Fee (higher fee first)
		if txI.Fee != txJ.Fee {
			return txI.Fee > txJ.Fee // Higher fee first
		}
		// 3. Then by Age (older first for fairness/anti-starvation)
		return txI.Timestamp < txJ.Timestamp // Older first
		// 4. V2+: AI/ML social impact score
	})
	mp.logger.Debugf("MEMPOOL: Transaction %s inserted into priority queue.", txIDHex)
}


// GetPendingTransactions retrieves a list of pending transactions, ordered by priority.
// This implements the MempoolRetriever interface for the ProposerService.
// EmPower1: This selection logic is critical for equitable block construction.
func (mp *Mempool) GetPendingTransactions(maxCount int) []*core.Transaction {
	mp.mu.RLock() // Acquire read lock
	defer mp.mu.RUnlock()

	if len(mp.transactions) == 0 {
		return []*core.Transaction{}
	}

	selectedTxs := make([]*core.Transaction, 0, maxCount)
	count := 0
	
	// Iterate through the priorityQueue (which is already sorted)
	for _, txIDHex := range mp.priorityQueue {
		if tx, found := mp.transactions[txIDHex]; found { // Ensure it's still in the map (not removed concurrently)
			selectedTxs = append(selectedTxs, tx)
			count++
			if count >= maxCount {
				break
			}
		}
	}
	
	mp.logger.Debugf("MEMPOOL: Retrieved %d pending transactions for proposer.", len(selectedTxs))
	return selectedTxs
}

// RemoveTransactions removes a list of transactions from the mempool.
// This is typically called by the consensus engine after transactions are confirmed in a finalized block.
// Adheres to "Systematize for Scalability".
func (mp *Mempool) RemoveTransactions(txIDs [][]byte) {
	mp.mu.Lock() // Acquire write lock
	defer mp.mu.Unlock()

	removedCount := 0
	newPriorityQueue := make([]string, 0, len(mp.priorityQueue)) // Build a new queue without removed items

	// Create a map for quick lookup of IDs to be removed
	txIDsToRemove := make(map[string]struct{})
	for _, id := range txIDs {
		txIDsToRemove[hex.EncodeToString(id)] = struct{}{}
	}

	for _, txIDHex := range mp.priorityQueue { // Iterate old order
		if _, shouldRemove := txIDsToRemove[txIDHex]; shouldRemove {
			delete(mp.transactions, txIDHex) // Remove from map
			removedCount++
		} else {
			newPriorityQueue = append(newPriorityQueue, txIDHex) // Keep in new queue
		}
	}
	mp.priorityQueue = newPriorityQueue // Update the ordered list

	if removedCount > 0 {
		mp.logger.Printf("MEMPOOL: Removed %d confirmed transactions. Current pending: %d\n", removedCount, len(mp.transactions))
	}
}

// PruneByBlockchain removes transactions that are already included in the provided blocks.
// This is useful if a node re-syncs or receives blocks out of order from the network.
func (mp *Mempool) PruneByBlockchain(blocks []*core.Block) {
	mp.mu.Lock()
	defer mp.mu.Unlock()

	blockTxs := make(map[string]struct{}) // Map of tx IDs in the blocks

	for _, block := range blocks {
		// Assuming block.Transactions is accessible directly or can be deserialized from block.Data
		// (Moved from previous block.Data deserialization in Block.ValidateTransactions)
		// Here, we should use the actual block.Transactions field directly, if Block struct is updated.
		// If block.Data still holds gob-encoded transactions, deserialize here.
		if len(block.Transactions) > 0 { // Assuming Block struct now holds []Transaction directly
			for _, tx := range block.Transactions {
				if tx != nil && tx.ID != nil {
					blockTxs[hex.EncodeToString(tx.ID)] = struct{}{}
				}
			}
		} else if len(block.Data) > 0 { // Fallback if block.Transactions is not populated directly in Block (older versions)
			var transactionsInBlock []*core.Transaction
			decoder := gob.NewDecoder(bytes.NewReader(block.Data))
			if err := decoder.Decode(&transactionsInBlock); err == nil {
				for _, tx := range transactionsInBlock {
					if tx != nil && tx.ID != nil {
						blockTxs[hex.EncodeToString(tx.ID)] = struct{}{}
					}
				}
			} else {
				mp.logger.Errorf("MEMPOOL_ERROR: Failed to deserialize transactions from block %x data during pruning: %v", block.Hash, err)
				// Consider this a serious error in production, indicating malformed blocks or data.
				continue 
			}
		}
	}

	if len(blockTxs) == 0 {
		return
	}

	removedCount := 0
	newPriorityQueue := make([]string, 0, len(mp.priorityQueue))

	for txIDHex := range mp.transactions { // Iterate map for efficiency
		if _, inBlock := blockTxs[txIDHex]; inBlock {
			delete(mp.transactions, txIDHex)
			removedCount++
		} else {
			// Only add to newPriorityQueue if it was NOT removed AND it was in the old priorityQueue
			// This relies on mp.priorityQueue's elements being unique.
			newPriorityQueue = append(newPriorityQueue, txIDHex) 
		}
	}
	mp.priorityQueue = newPriorityQueue // Update the ordered list

	if removedCount > 0 {
		mp.logger.Printf("MEMPOOL: Pruned %d transactions already in blockchain. Current pending: %d\n", removedCount, len(mp.transactions))
	}
}


// Size returns the current number of pending transactions in the mempool.
func (mp *Mempool) Size() int {