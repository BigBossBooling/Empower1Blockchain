package engine

import (
	"testing"

	pb "github.com/empower1/blockchain/proto"
	"github.com/stretchr/testify/assert"
)

func TestNewRedistributionEngine(t *testing.T) {
	e := New()
	assert.NotNil(t, e)
	assert.NotNil(t, e.scores)
	assert.Empty(t, e.scores)
}

func TestUpdateScores(t *testing.T) {
	e := New()
	records := []*pb.WealthScoreRecord{
		{UserId: "user-a", Score: 10.0},
		{UserId: "user-b", Score: 20.0},
	}

	e.UpdateScores(records)

	assert.Equal(t, 2, len(e.scores))
	assert.Equal(t, 10.0, e.scores["user-a"].Score)
	assert.Equal(t, 20.0, e.scores["user-b"].Score)

	// Test updating an existing score and adding a new one
	newRecords := []*pb.WealthScoreRecord{
		{UserId: "user-a", Score: 15.0},
		{UserId: "user-c", Score: 30.0},
	}
	e.UpdateScores(newRecords)

	assert.Equal(t, 3, len(e.scores))
	assert.Equal(t, 15.0, e.scores["user-a"].Score)
	assert.Equal(t, 30.0, e.scores["user-c"].Score)
}

func TestApplyTax(t *testing.T) {
	e := New()
	records := []*pb.WealthScoreRecord{
		{UserId: "user-a", Score: 100.0},
	}
	e.UpdateScores(records)

	// Test case 1: User is in the score map
	txWithScore := &pb.Transaction{From: "user-a"}
	taxCheckPerformed := e.ApplyTax(txWithScore)
	assert.True(t, taxCheckPerformed)

	// Test case 2: User is NOT in the score map
	txWithoutScore := &pb.Transaction{From: "user-b"}
	taxCheckPerformed = e.ApplyTax(txWithoutScore)
	assert.False(t, taxCheckPerformed)
}
