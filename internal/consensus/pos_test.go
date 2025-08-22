package consensus

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPOS_NextProposer_StakeWeighted(t *testing.T) {
	pos := NewPOS()
	assert.NotNil(t, pos)

	// The stakes are 100, 50, 25. Total stake is 175.
	// The proposer list should have 175 entries.
	assert.Equal(t, 175, len(pos.proposerList))

	counts := make(map[string]int)
	totalCalls := 175 * 2 // Run through the cycle twice to be sure

	for i := 0; i < totalCalls; i++ {
		proposer := pos.NextProposer()
		counts[proposer.Address]++
	}

	// Verify the distribution matches the stake weights
	assert.Equal(t, 3, len(counts)) // Should be 3 unique validators

	// The counts should be exactly double the stake, since we ran the cycle twice
	for _, validator := range pos.validators {
		expectedCount := int(validator.Stake * 2)
		assert.Equal(t, expectedCount, counts[validator.Address])
	}
}
