package crypto

import (
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"math/big"
)

const (
	// The number of bytes in a P256 coordinate.
	p256CoordinateBytes = 32
	// The number of bytes in our signature (R + S)
	SignatureLen = 64
)

// PrivateKey represents an ECDSA private key.
type PrivateKey struct {
	key *ecdsa.PrivateKey
}

// NewPrivateKey creates a new random private key using the P256 curve.
func NewPrivateKey() (*PrivateKey, error) {
	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, err
	}
	return &PrivateKey{key: key}, nil
}

// Sign generates a fixed-size 64-byte signature for the given data hash.
func (k *PrivateKey) Sign(dataHash []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, k.key, dataHash)
	if err != nil {
		return nil, err
	}

	// Pad r and s to 32 bytes each to create a fixed-size 64-byte signature.
	rBytes := r.Bytes()
	sBytes := s.Bytes()
	sig := make([]byte, SignatureLen)
	copy(sig[p256CoordinateBytes-len(rBytes):], rBytes)
	copy(sig[SignatureLen-len(sBytes):], sBytes)

	return sig, nil
}

// PublicKey returns the public key corresponding to this private key.
func (k *PrivateKey) PublicKey() *PublicKey {
	return &PublicKey{key: &k.key.PublicKey}
}

// PublicKey represents an ECDSA public key.
type PublicKey struct {
	key *ecdsa.PublicKey
}

// Verify verifies a 64-byte signature of a data hash.
func (k *PublicKey) Verify(signature []byte, dataHash []byte) bool {
	if len(signature) != SignatureLen {
		return false
	}
	// Split the fixed-size signature back into r and s.
	r := new(big.Int).SetBytes(signature[:p256CoordinateBytes])
	s := new(big.Int).SetBytes(signature[p256CoordinateBytes:])
	return ecdsa.Verify(k.key, dataHash, r, s)
}

// Address generates a blockchain address from the public key.
func (k *PublicKey) Address() string {
	pubKeyBytes := elliptic.Marshal(k.key.Curve, k.key.X, k.key.Y)
	hash := sha256.Sum256(pubKeyBytes)
	return hex.EncodeToString(hash[len(hash)-20:])
}
