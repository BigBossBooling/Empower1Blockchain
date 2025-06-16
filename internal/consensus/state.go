package consensus

import (
	"empower1.com/core/internal/core"
	"fmt"
	"sort"
	"sync"
)

// ConsensusState holds the current state relevant to the consensus process.
type ConsensusState struct {
	mu              sync.RWMutex
	currentHeight   int64
	validatorSet    []*Validator
	proposerSchedule map[int64]*Validator // Maps height to the selected proposer for that height
	// In a more complex system, this might include:
	// - Current round/step
	// - Votes received for the current block proposal
	// - Information about locked blocks or quorums
}

// NewConsensusState creates a new ConsensusState.
func NewConsensusState() *ConsensusState {
	return &ConsensusState{
		currentHeight:   0, // Assuming 0 for genesis, will be updated with new blocks
		validatorSet:    make([]*Validator, 0),
		proposerSchedule: make(map[int64]*Validator),
	}
}

// LoadInitialValidators loads the initial set of validators.
// For now, this is a hardcoded list. Later, it could come from a config file or genesis state.
func (cs *ConsensusState) LoadInitialValidators(validators []*Validator) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	cs.validatorSet = make([]*Validator, len(validators))
	copy(cs.validatorSet, validators)
	// Sort validators by address for deterministic proposer selection (if not weighted by stake)
	sort.Slice(cs.validatorSet, func(i, j int) bool {
		return cs.validatorSet[i].Address < cs.validatorSet[j].Address
	})
	cs.recalculateProposerSchedule(cs.currentHeight + 1, 10) // Pre-calculate for next 10 heights
	fmt.Printf("ConsensusState: Initial validator set loaded. Count: %d\n", len(cs.validatorSet))
}

// GetValidatorSet returns a copy of the current validator set.
func (cs *ConsensusState) GetValidatorSet() []*Validator {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	setCopy := make([]*Validator, len(cs.validatorSet))
	copy(setCopy, cs.validatorSet)
	return setCopy
}

// UpdateHeight updates the current block height.
// This should be called when a new block is successfully added to the chain.
func (cs *ConsensusState) UpdateHeight(newHeight int64) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if newHeight > cs.currentHeight {
		cs.currentHeight = newHeight
		// Potentially recalculate proposer schedule or other height-dependent state
		// For simplicity, we might pre-calculate or do it on demand.
		// Pre-calculate for next 10 heights if current one is met
		if cs.proposerSchedule[newHeight] != nil && cs.proposerSchedule[newHeight+1] == nil {
			cs.recalculateProposerSchedule(newHeight+1, 10)
		}
	}
}

// CurrentHeight returns the current block height known by the consensus state.
func (cs *ConsensusState) CurrentHeight() int64 {
	cs.mu.RLock()
	defer cs.mu.RUnlock()
	return cs.currentHeight
}

// recalculateProposerSchedule determines proposers for future heights (round-robin for now).
// This is an internal method and should be called with the main lock held.
func (cs *ConsensusState) recalculateProposerSchedule(startHeight int64, numHeights int) {
	if len(cs.validatorSet) == 0 {
		fmt.Println("ConsensusState: No validators available to schedule proposers.")
		return
	}
	// Simple round-robin selection based on the sorted validator set
	for i := 0; i < numHeights; i++ {
		height := startHeight + int64(i)
		if cs.proposerSchedule[height] == nil { // Only if not already scheduled
			proposerIndex := (height - 1) % int64(len(cs.validatorSet)) // -1 because height 1 is first block
			cs.proposerSchedule[height] = cs.validatorSet[proposerIndex]
			// fmt.Printf("ConsensusState: Proposer for height %d scheduled: %s\n", height, cs.validatorSet[proposerIndex].Address)
		}
	}
}

// GetProposerForHeight returns the designated proposer for a given block height.
// It recalculates the schedule if the requested height is beyond the current schedule.
func (cs *ConsensusState) GetProposerForHeight(height int64) (*Validator, error) {
	cs.mu.Lock() // Lock for potential recalculation
	defer cs.mu.Unlock()

	if len(cs.validatorSet) == 0 {
		return nil, fmt.Errorf("no validators in set to determine proposer")
	}

	proposer, scheduled := cs.proposerSchedule[height]
	if !scheduled {
		// If not scheduled, calculate up to this height + a buffer
		// This handles cases where we jump ahead or need a proposer for a future height not yet cached.
		// The startHeight for recalculation should be the next height for which a proposer isn't scheduled.
		// For simplicity, we'll try to schedule from current known height up to requested height + buffer.
		// A more robust way would find the lowest height > currentHeight that lacks a proposer.
		recalcStartHeight := cs.currentHeight + 1
		if height > recalcStartHeight { // Only if the requested height is in the future
			 cs.recalculateProposerSchedule(recalcStartHeight, int(height-recalcStartHeight+10))
		} else { // If requesting current or past height (should ideally be scheduled)
			 cs.recalculateProposerSchedule(height, 10) // Ensure at least this and a few more are scheduled
		}


		proposer, scheduled = cs.proposerSchedule[height]
		if !scheduled {
			// This should ideally not happen if recalculateProposerSchedule works correctly with validators
			return nil, fmt.Errorf("failed to determine proposer for height %d even after trying to schedule", height)
		}
	}
	return proposer, nil
}

// SetCurrentBlock updates the consensus state based on the latest accepted block.
// This would be called by the blockchain when a block is successfully added.
func (cs *ConsensusState) SetCurrentBlock(block *core.Block) {
	cs.mu.Lock()
	defer cs.mu.Unlock()
	if block.Height > cs.currentHeight {
		cs.currentHeight = block.Height
		// If the new block's height means we need to schedule more proposers, do it.
		// Check if the next height is already scheduled. If not, schedule more.
		if cs.proposerSchedule[block.Height+1] == nil && len(cs.validatorSet) > 0 {
			cs.recalculateProposerSchedule(block.Height+1, 10) // Schedule for the next 10 heights
		}
	}
	// In a more complex system, we might update validator stakes, active sets, etc., based on block data.
}
