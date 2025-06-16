package consensus

import (
	// For now, using string for address. Later, this might be a more complex type e.g. crypto.PublicKey
	// "crypto/ed25519"
)

// Validator represents a node that participates in the consensus mechanism.
type Validator struct {
	Address string // Public key identifier of the validator
	Stake   int64  // Amount of stake the validator has (simplified)
	IsActive bool   // Whether the validator is currently active in the set
	// Add more fields as needed, e.g., Jailed, UnbondingHeight, etc.
}

// NewValidator creates a new Validator.
// For now, address is a simple string. Later, it should be derived from a public key.
func NewValidator(address string, stake int64) *Validator {
	return &Validator{
		Address:  address,
		Stake:    stake,
		IsActive: true, // Assume active by default on creation
	}
}

// Equals checks if two validators are the same (based on Address).
func (v *Validator) Equals(other *Validator) bool {
	if other == nil {
		return false
	}
	return v.Address == other.Address
}
