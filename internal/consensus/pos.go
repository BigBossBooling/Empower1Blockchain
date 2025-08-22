package consensus

import "sync"

// Validator represents a node that is eligible to propose and validate blocks.
type Validator struct {
	Address string
	// In a real implementation, this would contain public keys, stake amount, etc.
}

// POS is a placeholder for the Proof-of-Stake consensus engine.
// For now, it implements a simple round-robin proposer selection.
type POS struct {
	lock          sync.RWMutex
	validators    []*Validator
	proposerIndex int
}

// NewPOS creates a new PoS engine with a hardcoded list of validators.
func NewPOS() *POS {
	// In a real system, validators would be managed dynamically.
	validators := []*Validator{
		{Address: "validator-1-address"},
		{Address: "validator-2-address"},
		{Address: "validator-3-address"},
	}

	return &POS{
		validators:    validators,
		proposerIndex: 0,
	}
}

// NextProposer selects the next block proposer in a deterministic round-robin fashion.
func (p *POS) NextProposer() string {
	p.lock.Lock()
	defer p.lock.Unlock()

	proposer := p.validators[p.proposerIndex]
	p.proposerIndex = (p.proposerIndex + 1) % len(p.validators)
	return proposer.Address
}
