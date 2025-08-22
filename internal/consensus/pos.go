package consensus

import (
	"log"
	"sync"

	"github.com/empower1/blockchain/internal/crypto"
)

// Validator represents a node that is eligible to propose and validate blocks.
type Validator struct {
	Address         string
	PublicKey       *crypto.PublicKey
	Stake           uint64
	ReputationScore float64
	privateKey      *crypto.PrivateKey // Kept here for simplicity in this simulation
}

// PrivateKey returns the validator's private key.
// In a real system, this would be handled by a secure wallet.
func (v *Validator) PrivateKey() *crypto.PrivateKey {
	return v.privateKey
}

// POS is the Proof-of-Stake consensus engine.
// It implements a deterministic, stake-weighted proposer selection.
type POS struct {
	lock          sync.RWMutex
	validators    []*Validator
	proposerList  []*Validator // An expanded list based on stake for weighted selection
	proposerIndex int
}

// NewPOS creates a new PoS engine with a hardcoded list of validators.
func NewPOS() *POS {
	// In a real system, validators would be managed dynamically.
	// For now, we generate keys for our hardcoded validators.
	validators := make([]*Validator, 3)
	stakes := []uint64{100, 50, 25}

	for i := 0; i < 3; i++ {
		privKey, err := crypto.NewPrivateKey()
		if err != nil {
			log.Fatalf("Failed to create private key for validator %d: %v", i, err)
		}
		pubKey := privKey.PublicKey()
		validators[i] = &Validator{
			Address:         pubKey.Address(),
			PublicKey:       pubKey,
			Stake:           stakes[i],
			ReputationScore: 1.0,
			privateKey:      privKey,
		}
		log.Printf("Created validator: %s with stake %d", validators[i].Address, validators[i].Stake)
	}

	// Create the stake-weighted proposer list
	var proposerList []*Validator
	for _, v := range validators {
		for i := uint64(0); i < v.Stake; i++ {
			proposerList = append(proposerList, v)
		}
	}

	return &POS{
		validators:   validators,
		proposerList: proposerList,
	}
}

// NextProposer selects the next block proposer using a deterministic,
// stake-weighted round-robin algorithm.
func (p *POS) NextProposer() *Validator {
	p.lock.Lock()
	defer p.lock.Unlock()

	// The list can be empty if all validators have 0 stake.
	if len(p.proposerList) == 0 {
		return nil
	}

	proposer := p.proposerList[p.proposerIndex]
	p.proposerIndex = (p.proposerIndex + 1) % len(p.proposerList)
	return proposer
}

// GetValidator returns the validator with the given address.
func (p *POS) GetValidator(address string) *Validator {
	p.lock.RLock()
	defer p.lock.RUnlock()

	for _, v := range p.validators {
		if v.Address == address {
			return v
		}
	}
	return nil
}
