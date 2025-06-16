package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"
)

// WalletKey holds an ECDSA key pair and the corresponding address.
type WalletKey struct {
	privateKey *ecdsa.PrivateKey
	publicKey  *ecdsa.PublicKey
	address    string // Hex string of serialized public key
}

// NewWalletKey generates a new WalletKey with a fresh ECDSA key pair.
func NewWalletKey() (*WalletKey, error) {
	privKey, err := GenerateECDSAKeyPair() // Uses P256 by default from keys.go
	if err != nil {
		return nil, err
	}
	pubKey := &privKey.PublicKey
	pubKeyBytes := SerializePublicKeyToBytes(pubKey)
	addr := hex.EncodeToString(pubKeyBytes)

	return &WalletKey{
		privateKey: privKey,
		publicKey:  pubKey,
		address:    addr,
	}, nil
}

// PrivateKey returns the wallet's private key.
func (wk *WalletKey) PrivateKey() *ecdsa.PrivateKey {
	return wk.privateKey
}

// PublicKey returns the wallet's public key.
func (wk *WalletKey) PublicKey() *ecdsa.PublicKey {
	return wk.publicKey
}

// Address returns the wallet's address (hex-encoded public key).
func (wk *WalletKey) Address() string {
	return wk.address
}

// PublicKeyBytes returns the serialized public key bytes.
func (wk *WalletKey) PublicKeyBytes() []byte {
	return SerializePublicKeyToBytes(wk.publicKey)
}

// TODO: Add functions to load/save WalletKey from/to encrypted files.
// For example:
// func LoadWalletKey(filePath string, password string) (*WalletKey, error)
// func (wk *WalletKey) Save(filePath string, password string) error
