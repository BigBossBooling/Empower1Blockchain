package core

import (
	"testing"

	pb "github.com/empower1/blockchain/proto"
	"github.com/stretchr/testify/assert"
)

func TestNewMempool(t *testing.T) {
	p := NewMempool()
	assert.NotNil(t, p)
	assert.NotNil(t, p.transactions)
	assert.Empty(t, p.transactions)
}

func TestMempool_Add(t *testing.T) {
	p := NewMempool()
	tx := &pb.Transaction{From: "a", To: "b", Value: 1}

	added, err := p.Add(tx)
	assert.NoError(t, err)
	assert.True(t, added)

	hash, _ := calculateTxHash(tx)
	assert.True(t, p.Contains(hash))

	// Test adding the same transaction again
	added, err = p.Add(tx)
	assert.NoError(t, err)
	assert.False(t, added)
	assert.Equal(t, 1, len(p.transactions))
}

func TestMempool_GetPending(t *testing.T) {
	p := NewMempool()
	tx1 := &pb.Transaction{From: "a", To: "b", Value: 1}
	tx2 := &pb.Transaction{From: "c", To: "d", Value: 2}

	p.Add(tx1)
	p.Add(tx2)

	pending := p.GetPending()
	assert.Equal(t, 2, len(pending))

	// Check if both transactions are present (order is not guaranteed in maps)
	foundTx1 := false
	foundTx2 := false
	for _, tx := range pending {
		if tx.Value == 1 {
			foundTx1 = true
		}
		if tx.Value == 2 {
			foundTx2 = true
		}
	}
	assert.True(t, foundTx1)
	assert.True(t, foundTx2)
}

func TestMempool_Clear(t *testing.T) {
	p := NewMempool()
	tx := &pb.Transaction{From: "a", To: "b", Value: 1}
	p.Add(tx)

	assert.Equal(t, 1, len(p.transactions))

	p.Clear()
	assert.Empty(t, p.transactions)
}
