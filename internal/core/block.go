package core

import (
	"crypto/sha256"
	"time"

	"github.com/empower1/blockchain/internal/crypto"
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

// Sign signs the block with the given private key.
// It calculates the hash of the header (without the signature), signs it,
// and sets the signature on the header.
func (b *Block) Sign(privKey *crypto.PrivateKey) error {
	hash, err := b.CalculateHash()
	if err != nil {
		return err
	}

	sig, err := privKey.Sign(hash)
	if err != nil {
		return err
	}

	b.Header.Signature = sig
	return nil
}

// VerifySignature verifies the block's signature against the given public key.
func (b *Block) VerifySignature(pubKey *crypto.PublicKey) (bool, error) {
	if b.Header.Signature == nil {
		return false, nil
	}

	hash, err := b.CalculateHash()
	if err != nil {
		return false, err
	}

	return pubKey.Verify(b.Header.Signature, hash), nil
}

// CalculateHash calculates and returns the SHA-256 hash of the block's header.
// It temporarily removes the signature from the header before hashing to ensure
// the hash is of the content that was actually signed.
func (b *Block) CalculateHash() ([]byte, error) {
	// Create a copy of the header to avoid modifying the original
	headerCopy := proto.Clone(b.Header).(*pb.BlockHeader)
	headerCopy.Signature = nil // The signature must not be part of the hash

	headerBytes, err := proto.Marshal(headerCopy)
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
