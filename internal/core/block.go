package core

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"fmt" // Added fmt import
	"time"
)

// Block represents a block in the blockchain.
type Block struct {
	Height        int64
	Timestamp     int64
	PrevBlockHash []byte
	Data          []byte
	Hash          []byte
	ProposerAddress string // Address of the validator who proposed this block
	Signature     []byte // Signature of the block proposer
}

// NewBlock creates a new block.
// ProposerAddress and Signature will be set by the consensus mechanism.
func NewBlock(height int64, prevBlockHash []byte, data []byte) *Block {
	block := &Block{
		Height:        height,
		Timestamp:     time.Now().UnixNano(),
		PrevBlockHash: prevBlockHash,
		Data:          data,
		// ProposerAddress and Signature are set later by the proposer
	}
	// Hash is calculated after all fields, including proposer and signature, are set.
	// For now, we might calculate it without them, and then recalculate,
	// or the consensus logic will be responsible for the final hash calculation.
	// Let's assume SetHash will be called by the consensus layer after filling proposer details.
	return block
}

// HeaderBytes returns the byte representation of the block's headers used for hashing and signing.
// It excludes the Hash and Signature itself.
func (b *Block) HeaderBytes() []byte {
	return bytes.Join(
		[][]byte{
			encodeInt64(b.Height),
			encodeInt64(b.Timestamp),
			b.PrevBlockHash,
			[]byte(b.ProposerAddress),
			b.Data, // Data is part of the header for hashing in this simple model
		},
		[]byte{},
	)
}

// SetHash calculates and sets the hash of the block using HeaderBytes.
func (b *Block) SetHash() {
	blockHeaders := b.HeaderBytes()
	hash := sha256.Sum256(blockHeaders)
	b.Hash = hash[:]
}

// Sign generates a signature for the block using a private key.
// Placeholder: This requires actual cryptographic key handling.
// For now, it will just set a dummy signature.
func (b *Block) Sign(proposerAddress string, privateKeyBytes []byte) error {
	b.ProposerAddress = proposerAddress
	// In a real implementation:
	// privKey, err := crypto.UnmarshalEd25519PrivateKey(privateKeyBytes)
	// if err != nil { return err }
	// sig, err := privKey.Sign(rand.Reader, b.HeaderBytes(), crypto.Hash(0))
	// if err != nil { return err }
	// b.Signature = sig

	// Placeholder signature
	b.Signature = []byte("signed-by-" + b.ProposerAddress)
	return nil
}

// VerifySignature checks if the block's signature is valid for the ProposerAddress.
// Placeholder: This requires actual cryptographic key handling.
func (b *Block) VerifySignature() (bool, error) {
	if b.ProposerAddress == "" {
		return false, fmt.Errorf("block has no proposer address")
	}
	if len(b.Signature) == 0 {
		return false, fmt.Errorf("block has no signature")
	}
	// In a real implementation:
	// pubKeyBytes, err := hex.DecodeString(b.ProposerAddress) // Assuming address is hex pubkey
	// if err != nil { return false, fmt.Errorf("invalid proposer address format: %w", err) }
	// pubKey, err := crypto.UnmarshalEd25519PublicKey(pubKeyBytes)
	// if err != nil { return false, fmt.Errorf("could not unmarshal public key: %w", err) }
	// valid := pubKey.Verify(b.HeaderBytes(), b.Signature)
	// return valid, nil

	// Placeholder verification
	expectedSignature := []byte("signed-by-" + b.ProposerAddress)
	return bytes.Equal(b.Signature, expectedSignature), nil
}


func encodeInt64(num int64) []byte {
	buf := new(bytes.Buffer)
	err := binary.Write(buf, binary.BigEndian, num)
	if err != nil {
		// This should not happen in practice with int64
		panic(err)
	}
	return buf.Bytes()
}

// Convenience function for decoding int64, useful for completeness
// func decodeInt64(data []byte) (int64, error) {
// 	var num int64
// 	buf := bytes.NewReader(data)
// 	err := binary.Read(buf, binary.BigEndian, &num)
// 	return num, err
// }
