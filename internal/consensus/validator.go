package consensus

import (
	"bytes" // Used for bytes.Equal for []byte comparison
	"errors" // For custom errors
	"fmt"    // For error messages
	// "crypto/ed25519" // For actual public key types, in a future iteration
)

// Validator-specific errors
var (
	ErrInvalidValidatorAddress = errors.New("validator address cannot be empty")
	ErrInvalidValidatorStake   = errors.New("validator stake must be positive")
)

// Validator represents a node that participates in the Proof-of-Stake consensus mechanism.
// For EmPower1, this includes core attributes like Address and Stake, and is extensible
// for future reputation and activity-based weighting.
type Validator struct {
	Address  []byte // Public key bytes or unique cryptographically derived identifier of the validator
	Stake    uint64 // Amount of PTCN tokens staked by the validator (using uint64 for clarity and consistency with amounts)
	IsActive bool   // Whether the validator is currently active in the consensus set (can be toggled by governance/slashing)
	// V2+ Enhancements for EmPower1's unique consensus:
	// ReputationScore float64 // Derived from on-chain behavior, uptime, AI audit flags (0.0 to 1.0)
	// ActivityScore   float64 // Derived from participation in network tasks, dApp usage (e.g., in EmPower1's ecosystem)
	// LastProposedHeight int64 // For round-robin fairness, or last active contribution
	// JailedUntil     int64 // Timestamp if validator is temporarily jailed
	// UnbondingHeight int64 // Height at which stake will be fully unbonded
	// V2+: NodeID      string // Unique network ID, separate from cryptographic address
}

// NewValidator creates a new Validator instance.
// This constructor ensures basic validity, adhering to "Know Your Core, Keep it Clear".
func NewValidator(address []byte, stake uint64) (*Validator, error) {
    if len(address) == 0 {
        return nil, ErrInvalidValidatorAddress
    }
    // In a real blockchain, you'd validate address format (e.g., checksum, length).
    
    if stake == 0 { // Stake must be positive to participate in PoS
        return nil, ErrInvalidValidatorStake
    }

    return &Validator{
        Address:  address,
        Stake:    stake,
        IsActive: true, // Assume active by default on creation, can be changed later
    }, nil
}

// Equals checks if two Validator instances represent the same entity, based on their Address.
// This is critical for managing validator sets and preventing duplicates.
func (v *Validator) Equals(other *Validator) bool {
    if v == nil || other == nil { // A validator cannot be nil for comparison
        return false
    }
    // Use bytes.Equal for byte slice comparison, which is correct for addresses.
    return bytes.Equal(v.Address, other.Address)
}

// String provides a human-readable string representation of the Validator.
// Useful for logging and debugging, aligning with "Sense the Landscape".
func (v *Validator) String() string {
    status := "Active"
    if !v.IsActive {
        status = "Inactive"
    }
    return fmt.Sprintf("Validator{Address: %x, Stake: %d, Status: %s}", v.Address, v.Stake, status)
}

// --- Conceptual V2+ Methods for EmPower1's Enhanced PoS ---
/*
// UpdateReputation conceptually updates the validator's reputation score.
// This would be called by the consensus engine or a governance pallet.
func (v *Validator) UpdateReputation(scoreChange float64) {
    v.ReputationScore += scoreChange
    if v.ReputationScore > 1.0 { v.ReputationScore = 1.0 }
    if v.ReputationScore < 0.0 { v.ReputationScore = 0.0 }
    // Emit an event for reputation update
}

// UpdateActivityScore conceptually updates the validator's activity score.
func (v *Validator) UpdateActivityScore(activityDelta float64) {
    v.ActivityScore += activityDelta
    // Add decay or max limits
}

// TotalWeight calculates the validator's effective weight in consensus.
// This is where EmPower1's unique hybrid PoS logic resides.
func (v *Validator) TotalWeight() uint64 {
    // Example: (Stake + (Stake * ReputationScore * ActivityScore)) or other complex formula
    // This is the core algorithm for weighted proposer selection/voting.
    weight := v.Stake 
    // if v.ReputationScore > 0 && v.ActivityScore > 0 {
    //    weight += uint64(float64(v.Stake) * v.ReputationScore * v.ActivityScore)
    // }
    return weight
}

// IsEligible checks if a validator is currently eligible to propose/validate.
// Considers active status, jailed status, minimum stake etc.
func (v *Validator) IsEligible(currentHeight int64) bool {
    // Example:
    // return v.IsActive && v.Stake >= minValidatorStake && v.JailedUntil <= currentHeight
    return v.IsActive // Basic for now
}
*/