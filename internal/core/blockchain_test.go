package core

import (
	"testing"
	"time"

	"github.com/empower1/blockchain/internal/consensus"
	pb "github.com/empower1/blockchain/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewBlockchain(t *testing.T) {
	pos := consensus.NewPOS()
	bc := NewBlockchain(pos)
	assert.NotNil(t, bc)
	assert.Equal(t, uint64(0), bc.Height())
	genesis, err := bc.GetBlockByHeight(0)
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), genesis.Header.Height)
}

func TestAddBlock(t *testing.T) {
	pos := consensus.NewPOS()
	bc := NewBlockchain(pos)
	genesis, err := bc.GetBlockByHeight(0)
	assert.NoError(t, err)

	// Create a valid new block
	proposer := pos.NextProposer()
	header := &pb.BlockHeader{
		Version:         1,
		PrevBlockHash:   genesis.Block.Hash,
		Height:          1,
		Timestamp:       timestamppb.New(time.Now()),
		ProposerAddress: proposer.Address,
	}
	newBlock := NewBlock(header, []*pb.Transaction{})
	err = newBlock.Sign(proposer.PrivateKey())
	assert.NoError(t, err)
	newBlock.SetHash()

	err = bc.AddBlock(newBlock)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), bc.Height())

	retrievedBlock, err := bc.GetBlockByHeight(1)
	assert.NoError(t, err)
	assert.Equal(t, newBlock.Block.Hash, retrievedBlock.Block.Hash)
}

func TestAddBlock_InvalidPrevHash(t *testing.T) {
	pos := consensus.NewPOS()
	bc := NewBlockchain(pos)

	// Create a block with an invalid previous hash
	proposer := pos.NextProposer()
	header := &pb.BlockHeader{
		Version:         1,
		PrevBlockHash:   []byte("invalid_hash"), // Deliberately wrong
		Height:          1,
		Timestamp:       timestamppb.New(time.Now()),
		ProposerAddress: proposer.Address,
	}
	newBlock := NewBlock(header, []*pb.Transaction{})
	err := newBlock.Sign(proposer.PrivateKey())
	assert.NoError(t, err)
	newBlock.SetHash()

	err = bc.AddBlock(newBlock)
	assert.Error(t, err)
	assert.Equal(t, "invalid previous block hash", err.Error())
	assert.Equal(t, uint64(0), bc.Height())
}

func TestAddBlock_InvalidHeight(t *testing.T) {
	pos := consensus.NewPOS()
	bc := NewBlockchain(pos)
	genesis, err := bc.GetBlockByHeight(0)
	assert.NoError(t, err)

	// Create a block with an invalid height
	proposer := pos.NextProposer()
	header := &pb.BlockHeader{
		Version:         1,
		PrevBlockHash:   genesis.Block.Hash,
		Height:          5, // Deliberately wrong
		Timestamp:       timestamppb.New(time.Now()),
		ProposerAddress: proposer.Address,
	}
	newBlock := NewBlock(header, []*pb.Transaction{})
	err = newBlock.Sign(proposer.PrivateKey())
	assert.NoError(t, err)
	newBlock.SetHash()

	err = bc.AddBlock(newBlock)
	assert.Error(t, err)
	assert.Equal(t, "invalid block height: expected 1, got 5", err.Error())
	assert.Equal(t, uint64(0), bc.Height())
}

func TestAddBlock_InvalidSignature(t *testing.T) {
	pos := consensus.NewPOS()
	bc := NewBlockchain(pos)
	genesis, err := bc.GetBlockByHeight(0)
	assert.NoError(t, err)

	proposer := pos.NextProposer()

	// To ensure we get a different validator, we advance the proposer index
	// past all the slots for the first validator (stake 100).
	for i := 0; i < 100; i++ {
		pos.NextProposer()
	}
	wrongProposer := pos.NextProposer()
	assert.NotEqual(t, proposer.Address, wrongProposer.Address)

	header := &pb.BlockHeader{
		Version:         1,
		PrevBlockHash:   genesis.Block.Hash,
		Height:          1,
		Timestamp:       timestamppb.New(time.Now()),
		ProposerAddress: proposer.Address, // Header lists the correct proposer
	}
	newBlock := NewBlock(header, []*pb.Transaction{})

	// But the block is signed by the WRONG key
	err = newBlock.Sign(wrongProposer.PrivateKey())
	assert.NoError(t, err)
	newBlock.SetHash()

	err = bc.AddBlock(newBlock)
	assert.Error(t, err)
	assert.Equal(t, "invalid block signature", err.Error())
}
