package network

import (
	"testing"
	"time"

	"github.com/empower1/blockchain/internal/consensus"
	"github.com/empower1/blockchain/internal/core"
	pb "github.com/empower1/blockchain/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Helper function to create a new signed block for testing
func createSignedTestBlock(bc *core.Blockchain, pos *consensus.POS) (*core.Block, error) {
	prevBlock, err := bc.GetBlockByHeight(bc.Height())
	if err != nil {
		return nil, err
	}
	proposer := pos.NextProposer()

	header := &pb.BlockHeader{
		Version:         1,
		PrevBlockHash:   prevBlock.Block.Hash,
		Height:          prevBlock.Header.Height + 1,
		Timestamp:       timestamppb.New(time.Now()),
		ProposerAddress: proposer.Address,
	}
	newBlock := core.NewBlock(header, []*pb.Transaction{})
	if err := newBlock.Sign(proposer.PrivateKey()); err != nil {
		return nil, err
	}
	if err := newBlock.SetHash(); err != nil {
		return nil, err
	}
	return newBlock, nil
}

func TestSyncer(t *testing.T) {
	// 1. Setup a single validator set (POS) to be shared by both nodes
	pos := consensus.NewPOS()
	mempool := core.NewMempool()

	// 2. Setup Peer A (the node with the longer chain)
	listenAddr := ":5001"
	bcA := core.NewBlockchain(pos)

	// Add 5 blocks to Peer A's chain
	for i := 0; i < 5; i++ {
		newBlock, err := createSignedTestBlock(bcA, pos)
		assert.NoError(t, err)
		err = bcA.AddBlock(newBlock)
		assert.NoError(t, err)
	}
	assert.Equal(t, uint64(5), bcA.Height())

	serverA := NewServer(listenAddr, bcA, mempool)
	go serverA.Start()
	time.Sleep(100 * time.Millisecond)

	// 3. Setup Peer B (the new node that needs to sync)
	bcB := core.NewBlockchain(pos) // Use the same POS instance
	assert.Equal(t, uint64(0), bcB.Height())

	// 4. Run the Syncer for Peer B
	bootstrapNodes := []string{listenAddr}
	syncer := NewSyncer(bcB, bootstrapNodes)
	err := syncer.Start()
	assert.NoError(t, err)

	// 5. Verification
	assert.Equal(t, bcA.Height(), bcB.Height())

	// Verify the last block hash matches
	lastBlockA, err := bcA.GetBlockByHeight(bcA.Height())
	assert.NoError(t, err)
	lastBlockB, err := bcB.GetBlockByHeight(bcB.Height())
	assert.NoError(t, err)
	assert.Equal(t, lastBlockA.Block.Hash, lastBlockB.Block.Hash)
}
