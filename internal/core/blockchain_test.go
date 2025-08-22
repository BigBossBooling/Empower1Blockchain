package core

import (
	"testing"
	"time"

	pb "github.com/empower1/blockchain/proto"
	"github.com/stretchr/testify/assert"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestNewBlockchain(t *testing.T) {
	bc := NewBlockchain()
	assert.NotNil(t, bc)
	assert.Equal(t, uint64(0), bc.Height())
	genesis, err := bc.GetBlockByHeight(0)
	assert.NoError(t, err)
	assert.Equal(t, uint64(0), genesis.Header.Height)
}

func TestAddBlock(t *testing.T) {
	bc := NewBlockchain()
	genesis, _ := bc.GetBlockByHeight(0)

	// Create a valid new block
	header := &pb.BlockHeader{
		Version:       1,
		PrevBlockHash: genesis.Block.Hash,
		Height:        1,
		Timestamp:     timestamppb.New(time.Now()),
	}
	newBlock := NewBlock(header, []*pb.Transaction{})
	newBlock.SetHash()

	err := bc.AddBlock(newBlock)
	assert.NoError(t, err)
	assert.Equal(t, uint64(1), bc.Height())

	retrievedBlock, err := bc.GetBlockByHeight(1)
	assert.NoError(t, err)
	assert.Equal(t, newBlock.Block.Hash, retrievedBlock.Block.Hash)
}

func TestAddBlock_InvalidPrevHash(t *testing.T) {
	bc := NewBlockchain()

	// Create a block with an invalid previous hash
	header := &pb.BlockHeader{
		Version:       1,
		PrevBlockHash: []byte("invalid_hash"), // Deliberately wrong
		Height:        1,
		Timestamp:     timestamppb.New(time.Now()),
	}
	newBlock := NewBlock(header, []*pb.Transaction{})
	newBlock.SetHash()

	err := bc.AddBlock(newBlock)
	assert.Error(t, err)
	assert.Equal(t, "invalid previous block hash", err.Error())
	assert.Equal(t, uint64(0), bc.Height())
}

func TestAddBlock_InvalidHeight(t *testing.T) {
	bc := NewBlockchain()
	genesis, _ := bc.GetBlockByHeight(0)

	// Create a block with an invalid height
	header := &pb.BlockHeader{
		Version:       1,
		PrevBlockHash: genesis.Block.Hash,
		Height:        5, // Deliberately wrong
		Timestamp:     timestamppb.New(time.Now()),
	}
	newBlock := NewBlock(header, []*pb.Transaction{})
	newBlock.SetHash()

	err := bc.AddBlock(newBlock)
	assert.Error(t, err)
	assert.Equal(t, "invalid block height: expected 1, got 5", err.Error())
	assert.Equal(t, uint64(0), bc.Height())
}
