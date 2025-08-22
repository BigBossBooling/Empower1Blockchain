package engine

import (
	"fmt"
	"sync"

	pb "github.com/empower1/blockchain/proto"
)

// RedistributionEngine is responsible for the on-chain logic of the IRE.
// It maintains the current state of wealth scores and applies taxes.
type RedistributionEngine struct {
	lock   sync.RWMutex
	scores map[string]*pb.WealthScoreRecord // maps user_id to their score record
}

// New creates a new RedistributionEngine.
func New() *RedistributionEngine {
	return &RedistributionEngine{
		scores: make(map[string]*pb.WealthScoreRecord),
	}
}

// UpdateScores safely updates the engine's internal score map with new records.
// This method is intended to be called by the Oracle Client.
func (e *RedistributionEngine) UpdateScores(records []*pb.WealthScoreRecord) {
	e.lock.Lock()
	defer e.lock.Unlock()

	for _, record := range records {
		e.scores[record.UserId] = record
	}
	fmt.Printf("Redistribution Engine: Updated %d scores.\n", len(records))
}

// ApplyTax is a placeholder for the logic that checks a transaction
// and applies the redistribution tax if applicable.
// It returns true if a tax check was performed on a user with a score.
func (e *RedistributionEngine) ApplyTax(tx *pb.Transaction) bool {
	e.lock.RLock()
	defer e.lock.RUnlock()

	senderScore, ok := e.scores[tx.From]
	if !ok {
		// User is not opted into the redistribution system. No tax applies.
		return false
	}

	// In a real implementation, we would have a threshold.
	// For now, we just log the check and return true.
	fmt.Printf("Checking tax for user %s with score %f\n", tx.From, senderScore.Score)

	// TODO: Implement the actual tax logic (e.g., if score > threshold, apply 9% tax).
	return true
}
