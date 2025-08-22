package core

import (
	"crypto/sha256"
	"time"

	pb "github.com/empower1/blockchain/proto"
	"google.golang.org/protobuf/proto"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// Block represents a single block in the EmPower1 blockchain.
// It wraps the protobuf Block message to provide additional methods.
type Block struct {
	*pb.Block
}

// NewBlock creates a new Block.
func NewBlock(header *pb.BlockHeader, transactions []*pb.Transaction) *Block {
	return &Block{
		Block: &pb.Block{
			Header:       header,
			Transactions: transactions,
		},
	}
}

// CalculateHash calculates and returns the SHA-256 hash of the block's header.
// This method ensures the hashing process is deterministic.
func (b *Block) CalculateHash() ([]byte, error) {
	headerBytes, err := proto.Marshal(b.Header)
	if err != nil {
		return nil, err
	}
	hash := sha256.Sum256(headerBytes)
	return hash[:], nil
}

// SetHash calculates the block's hash and sets it on the block's Hash field.
func (b *Block) SetHash() error {
	hash, err := b.CalculateHash()
	if err != nil {
		return err
	}
	b.Block.Hash = hash
	return nil
}

// NewGenesisBlock creates and returns the first block in the chain.
func NewGenesisBlock() *Block {
	header := &pb.BlockHeader{
		Version:          1,
		PrevBlockHash:    make([]byte, 32), // 32 bytes of zeros
		TransactionsHash: make([]byte, 32), // No transactions in genesis
		Timestamp:        timestamppb.New(time.Date(2025, 8, 21, 0, 0, 0, 0, time.UTC)),
		Height:           0,
	}
	block := NewBlock(header, []*pb.Transaction{})
	block.SetHash() // Set the initial hash
	return block
}
