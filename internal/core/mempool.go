package core

import (
	"crypto/sha256"
	"encoding/hex"
	"sync"

	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/protobuf/proto"
)

// Mempool is a thread-safe, in-memory store for pending transactions.
type Mempool struct {
	lock         sync.RWMutex
	transactions map[string]*pb.Transaction
}

// NewMempool creates a new Mempool.
func NewMempool() *Mempool {
	return &Mempool{
		transactions: make(map[string]*pb.Transaction),
	}
}

// Add adds a transaction to the mempool.
// It returns true if the transaction was added, false if it already exists.
func (p *Mempool) Add(tx *pb.Transaction) (bool, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	hash, err := calculateTxHash(tx)
	if err != nil {
		return false, err
	}

	if _, ok := p.transactions[hash]; ok {
		return false, nil // Transaction already exists
	}

	p.transactions[hash] = tx
	return true, nil
}

// Contains checks if the mempool contains a transaction with the given hash.
func (p *Mempool) Contains(hash string) bool {
	p.lock.RLock()
	defer p.lock.RUnlock()

	_, ok := p.transactions[hash]
	return ok
}

// GetPending returns a slice of all transactions currently in the mempool.
func (p *Mempool) GetPending() []*pb.Transaction {
	p.lock.RLock()
	defer p.lock.RUnlock()

	pending := make([]*pb.Transaction, 0, len(p.transactions))
	for _, tx := range p.transactions {
		pending = append(pending, tx)
	}
	return pending
}

// Clear removes all transactions from the mempool.
func (p *Mempool) Clear() {
	p.lock.Lock()
	defer p.lock.Unlock()

	p.transactions = make(map[string]*pb.Transaction)
}

// calculateTxHash is a helper to hash a transaction.
// In a real implementation, this would be part of a Transaction wrapper struct
// and would likely not hash the entire transaction, but specific fields.
func calculateTxHash(tx *pb.Transaction) (string, error) {
	txBytes, err := proto.Marshal(tx)
	if err != nil {
		return "", err
	}
	hash := sha256.Sum256(txBytes)
	return hex.EncodeToString(hash[:]), nil
}
