package core

import (
	"bytes"
	"testing"
	// "time" // Removed unused import
	"empower1.com/core/internal/crypto" // For key generation to simulate proposer
	"encoding/hex"
)

func TestNewBlock(t *testing.T) {
	prevBlock := NewBlock(0, []byte("genesis_prev"), []byte("genesis_data"))
	prevBlock.ProposerAddress = "genesis_proposer"
	prevBlock.Sign("genesis_proposer", nil) // Placeholder sign
	prevBlock.SetHash()

	height := int64(1)
	data := []byte("block data")

	block := NewBlock(height, prevBlock.Hash, data)

	if block.Height != height {
		t.Errorf("block.Height = %d; want %d", block.Height, height)
	}
	if !bytes.Equal(block.PrevBlockHash, prevBlock.Hash) {
		t.Errorf("block.PrevBlockHash is incorrect")
	}
	if !bytes.Equal(block.Data, data) {
		t.Errorf("block.Data is incorrect")
	}
	if block.Timestamp == 0 {
		t.Errorf("block.Timestamp not set")
	}
	if block.ProposerAddress != "" { // Should be empty until signed
		t.Errorf("block.ProposerAddress should be empty initially, got %s", block.ProposerAddress)
	}
	if block.Signature != nil { // Should be nil until signed
		t.Errorf("block.Signature should be nil initially")
	}
	if block.Hash != nil { // Should be nil until SetHash is called
		t.Errorf("block.Hash should be nil initially")
	}
}

func TestBlockSignAndVerifySignature(t *testing.T) {
	block := NewBlock(1, []byte("prev_hash"), []byte("some data"))

	// Using a dummy proposer address and key for this placeholder test
	// In a real scenario, this would use actual validator keys from consensus
	proposerKey, _ := crypto.GenerateECDSAKeyPair() // Not used by current placeholder Sign
	proposerAddress := crypto.PublicKeyBytesToAddress(crypto.SerializePublicKeyToBytes(&proposerKey.PublicKey))

	// Test placeholder Sign method
	err := block.Sign(proposerAddress, nil) // Private key not used by placeholder
	if err != nil {
		t.Fatalf("block.Sign() error = %v", err)
	}
	if block.ProposerAddress != proposerAddress {
		t.Errorf("block.ProposerAddress = %s; want %s", block.ProposerAddress, proposerAddress)
	}
	expectedSignature := []byte("signed-by-" + proposerAddress)
	if !bytes.Equal(block.Signature, expectedSignature) {
		t.Errorf("block.Signature is incorrect for placeholder signing")
	}

	// Test placeholder VerifySignature method
	valid, err := block.VerifySignature()
	if err != nil {
		t.Fatalf("block.VerifySignature() error = %v", err)
	}
	if !valid {
		t.Errorf("block.VerifySignature() = false; want true for placeholder signature")
	}

	// Tamper with ProposerAddress and expect verification to fail
	originalProposer := block.ProposerAddress
	block.ProposerAddress = "tampered_proposer"
	valid, _ = block.VerifySignature()
	if valid {
		t.Errorf("block.VerifySignature() = true after tampering ProposerAddress; want false")
	}
	block.ProposerAddress = originalProposer // Restore

	// Tamper with Signature and expect verification to fail
	originalSignature := block.Signature
	block.Signature = []byte("tampered_signature")
	valid, _ = block.VerifySignature()
	if valid {
		t.Errorf("block.VerifySignature() = true after tampering Signature; want false")
	}
	block.Signature = originalSignature // Restore

	// Test with empty ProposerAddress or Signature (should fail)
	block.ProposerAddress = ""
	_, err = block.VerifySignature()
	if err == nil {
		t.Errorf("Expected error for VerifySignature with empty ProposerAddress")
	}
	block.ProposerAddress = originalProposer // Restore

	block.Signature = nil
	_, err = block.VerifySignature()
	if err == nil {
		t.Errorf("Expected error for VerifySignature with nil Signature")
	}
	block.Signature = originalSignature // Restore
}

func TestBlockSetHash(t *testing.T) {
	block := NewBlock(1, []byte("prev_hash"), []byte("some data"))
	block.ProposerAddress = "test_proposer" // ProposerAddress is part of HeaderBytes
	block.Timestamp = 1234567890 // Fixed timestamp

	// Calculate hash first time
	block.SetHash()
	hash1 := make([]byte, len(block.Hash))
	copy(hash1, block.Hash)

	if len(hash1) == 0 {
		t.Errorf("block.SetHash() resulted in empty hash")
	}

	// Call SetHash again, should be idempotent if fields haven't changed
	block.SetHash()
	if !bytes.Equal(hash1, block.Hash) {
		t.Errorf("block.SetHash() is not idempotent. Hash1: %x, Hash2: %x", hash1, block.Hash)
	}

	// Change a field that's part of HeaderBytes and expect hash to change
	block.Data = []byte("different data")
	block.SetHash()
	hash2 := block.Hash
	if bytes.Equal(hash1, hash2) {
		t.Errorf("Block hash did not change after data modification. Hash1: %x, Hash2: %x", hash1, hash2)
	}

	t.Logf("TestBlockSetHash: Hash1: %s", hex.EncodeToString(hash1))
	t.Logf("TestBlockSetHash: Hash2 (after data change): %s", hex.EncodeToString(hash2))
}

func TestBlockHeaderBytes(t *testing.T) {
	block := NewBlock(1, []byte("prev_hash"), []byte("some data"))
	block.ProposerAddress = "test_proposer"
	block.Timestamp = 1234567890

	headerBytes1 := block.HeaderBytes()
	if len(headerBytes1) == 0 {
		t.Errorf("block.HeaderBytes() returned empty slice")
	}

	// Change data, expect HeaderBytes to change
	block.Data = []byte("new data")
	headerBytes2 := block.HeaderBytes()
	if bytes.Equal(headerBytes1, headerBytes2) {
		t.Errorf("HeaderBytes did not change after data modification")
	}

	// Change ProposerAddress, expect HeaderBytes to change
	block.ProposerAddress = "another_proposer"
	headerBytes3 := block.HeaderBytes()
	if bytes.Equal(headerBytes2, headerBytes3) {
		t.Errorf("HeaderBytes did not change after ProposerAddress modification")
	}
}
