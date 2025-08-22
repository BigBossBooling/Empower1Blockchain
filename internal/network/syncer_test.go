package network

import (
	"testing"
	"time"

	"github.com/empower1/blockchain/internal/core"
	pb "github.com/empower1/blockchain/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Helper function to create a new block for testing
func createTestBlock(prevBlock *core.Block) *core.Block {
	header := &pb.BlockHeader{
		Version:       1,
		PrevBlockHash: prevBlock.Block.Hash,
		Height:        prevBlock.Header.Height + 1,
		Timestamp:     timestamppb.New(time.Now()),
	}
	newBlock := core.NewBlock(header, []*pb.Transaction{})
	newBlock.SetHash()
	return newBlock
}

func TestSyncer(t *testing.T) {
	// 1. Setup Peer A (the node with the longer chain)
	listenAddr := ":5001" // Use a different port to avoid conflicts
	bcA := core.NewBlockchain()

	// Add 5 blocks to Peer A's chain
	currentBlock, err := bcA.GetBlockByHeight(0)
	assert.NoError(t, err)
	for i := 0; i < 5; i++ {
		newBlock := createTestBlock(currentBlock)
		err := bcA.AddBlock(newBlock)
		assert.NoError(t, err)
		currentBlock = newBlock
	}
	assert.Equal(t, uint64(5), bcA.Height())

	serverA := NewServer(listenAddr, bcA)
	go serverA.Start()
	time.Sleep(100 * time.Millisecond) // Give the server a moment to start

	// 2. Setup Peer B (the new node that needs to sync)
	bcB := core.NewBlockchain()
	assert.Equal(t, uint64(0), bcB.Height())

	// 3. Run the Syncer for Peer B
	bootstrapNodes := []string{listenAddr}
	syncer := NewSyncer(bcB, bootstrapNodes)
	err = syncer.Start()
	assert.NoError(t, err)

	// 4. Verification
	assert.Equal(t, bcA.Height(), bcB.Height())

	// Verify the last block hash matches
	lastBlockA, err := bcA.GetBlockByHeight(bcA.Height())
	assert.NoError(t, err)
	lastBlockB, err := bcB.GetBlockByHeight(bcB.Height())
	assert.NoError(t, err)
	assert.Equal(t, lastBlockA.Block.Hash, lastBlockB.Block.Hash)
}
