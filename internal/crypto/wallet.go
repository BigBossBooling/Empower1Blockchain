package crypto

import (
	"crypto/ecdsa"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath" // For joining paths safely
	"sync" // For potential future thread-safe caching of loaded keys
	"errors" // For custom error types
)

// Define custom errors for WalletKey management for clearer failure states.
var (
	ErrWalletKeyInit      = errors.New("wallet key initialization error")
	ErrWalletKeyNotFound  = errors.New("wallet key file not found")
	ErrWalletKeyCorrupted = errors.New("wallet key file corrupted or invalid format")
	ErrWalletKeySave      = errors.New("failed to save wallet key")
	ErrWalletKeyLoad      = errors.New("failed to load wallet key")
	ErrPasswordRequired   = errors.New("password required for encrypted key")
	ErrInvalidPassword    = errors.New("invalid password for encrypted key") // If we add decryption
)

// WalletKey holds an ECDSA key pair and the corresponding address.
// This struct provides a convenient and secure wrapper for managing user's cryptographic assets.
// It is designed to be easily extensible for future encrypted storage.
type WalletKey struct {
    mu          sync.RWMutex     // Mutex for potential future thread-safe access or lazy loading
    privateKey  *ecdsa.PrivateKey // The ECDSA private key
    publicKey   *ecdsa.PublicKey  // The ECDSA public key (derived from privateKey)
    address     string            // Hex string of the derived blockchain address
}

// NewWalletKey generates a new WalletKey with a fresh ECDSA key pair (P256 curve).
// This is the primary way to create new user identities for EmPower1.
func NewWalletKey() (*WalletKey, error) {
    privKey, err := GenerateECDSAKeyPair() // Assumes P256 from its definition in this package
    if err != nil {
        return nil, fmt.Errorf("%w: failed to generate ECDSA key pair: %v", ErrWalletKeyInit, err)
    }
    
    pubKey := &privKey.PublicKey
    
    // Serialize public key to bytes for address derivation.
    pubKeyBytes, err := SerializePublicKeyToBytes(pubKey)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to serialize public key for address: %v", ErrWalletKeyInit, err)
    }
    
    // Convert public key bytes to address string (hex-encoded for now).
    // In a real system, this would be a more robust, hashed, checksummed address.
    addr := PublicKeyBytesToAddress(pubKeyBytes)

    return &WalletKey{
        privateKey: privKey,
        publicKey:  pubKey,
        address:    addr,
    }, nil
}

// PrivateKey returns the wallet's ECDSA private key.
// Access to this should be handled with extreme care due to its sensitivity.
func (wk *WalletKey) PrivateKey() *ecdsa.PrivateKey {
    return wk.privateKey
}

// PublicKey returns the wallet's ECDSA public key.
func (wk *WalletKey) PublicKey() *ecdsa.PublicKey {
    return wk.publicKey
}

// Address returns the wallet's blockchain address (hex-encoded public key for V1).
// This is the public identifier for the user on the EmPower1 blockchain.
func (wk *WalletKey) Address() string {
    return wk.address
}

// PublicKeyBytes returns the raw serialized public key bytes.
// This is useful for transaction signing and verification where raw bytes are needed.
func (wk *WalletKey) PublicKeyBytes() []byte {
    // This method will now use the new SerializePublicKeyToBytes which returns error
    // For consistency with method signature, we handle potential error here.
    bytes, err := SerializePublicKeyToBytes(wk.publicKey)
    if err != nil {
        // This should ideally not happen since wk.publicKey is already validated at creation
        // and we control the curve, but defensive programming is good.
        // Log a critical error or panic in production if this occurs, as it indicates a core integrity issue.
        return nil // Or handle panic
    }
    return bytes
}

// --- Key Management (Save/Load) ---
// Adhering to "Sense the Landscape, Secure the Solution" for persistent key storage.

// Save saves the WalletKey (private and public key) to a specified file path in PEM format.
// It allows for optional encryption via a password. This uses the crypto package's PEM functions.
func (wk *WalletKey) Save(filePath string, password string) error {
    // Ensure parent directory exists
    dir := filepath.Dir(filePath)
    if err := os.MkdirAll(dir, 0700); err != nil { // Create directory with owner-only permissions
        return fmt.Errorf("%w: failed to create directory %s: %v", ErrWalletKeySave, dir, err)
    }

    var pemPassword []byte
    if password != "" {
        pemPassword = []byte(password)
    }

    // Serialize private key to PEM
    pemBytes, err := SerializePrivateKeyToPEM(wk.privateKey, pemPassword)
    if err != nil {
        return fmt.Errorf("%w: failed to serialize private key to PEM: %v", ErrWalletKeySave, err)
    }

    // Write to file with restrictive permissions (owner only read/write)
    if err := os.WriteFile(filePath, pemBytes, 0600); err != nil {
        return fmt.Errorf("%w: failed to write wallet key to file %s: %v", ErrWalletKeySave, filePath, err)
    }

    // Optionally, save the public key separately (e.g., to a .pub file) for easy sharing,
    // but the private key file contains both (implicitly through derivation).
    // This implementation focuses on the private key file as the primary secure storage.

    return nil
}

// LoadWalletKey loads a WalletKey from a PEM file.
// It attempts to decrypt the private key using the provided password (if applicable).
func LoadWalletKey(filePath string, password string) (*WalletKey, error) {
    pemBytes, err := os.ReadFile(filePath)
    if err != nil {
        if os.IsNotExist(err) {
            return nil, fmt.Errorf("%w: file not found at %s", ErrWalletKeyNotFound, filePath)
        }
        return nil, fmt.Errorf("%w: failed to read file %s: %v", ErrWalletKeyLoad, filePath, err)
    }

    var pemPassword []byte
    if password != "" {
        pemPassword = []byte(password)
    }

    privKey, err := DeserializePrivateKeyFromPEM(pemBytes, pemPassword)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to deserialize or decrypt private key from PEM: %v", ErrWalletKeyCorrupted, err)
    }

    // Derive public key from the loaded private key for consistency.
    pubKey := &privKey.PublicKey
    pubKeyBytes, err := SerializePublicKeyToBytes(pubKey)
    if err != nil {
        return nil, fmt.Errorf("%w: failed to derive public key bytes from loaded private key: %v", ErrWalletKeyCorrupted, err)
    }
    addr := PublicKeyBytesToAddress(pubKeyBytes)

    return &WalletKey{
        privateKey: privKey,
        publicKey:  pubKey,
        address:    addr,
    }, nil
}