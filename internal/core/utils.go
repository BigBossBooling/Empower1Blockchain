package core

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic" // For P256 curve
	"crypto/rand"
	"crypto/sha256" // Potentially used for address derivation if more complex than raw pubkey
	"encoding/binary"
	"encoding/hex" // Used for hex encoding of addresses/keys for display/comparison
	"errors"
	"fmt"
	"sort"
)

// --- General Utility Functions ---

// encodeInt64 converts an int64 to a byte slice using binary.BigEndian encoding.
// This is crucial for consistent byte representation of numeric values across the network,
// ensuring cryptographic integrity and deterministic hashing.
func encodeInt64(num int64) []byte {
	buf := new(bytes.Buffer)
	// binary.Write should not return an error for fixed-size types like int64
	err := binary.Write(buf, binary.BigEndian, num)
	if err != nil {
		// Log and panic only if absolutely unrecoverable, or return error for handling upstream
		// In a production blockchain, this would likely be an unrecoverable serialization error.
		panic(fmt.Sprintf("CORE_UTIL_PANIC: failed to encode int64: %v", err)) // Provide more context for panic
	}
	return buf.Bytes()
}

// decodeInt64 decodes a byte slice into an int64 using binary.BigEndian encoding.
// This is the inverse of encodeInt64, crucial for deserializing numeric values from blocks/transactions.
func decodeInt64(data []byte) (int64, error) {
	var num int64
	buf := bytes.NewReader(data)
	err := binary.Read(buf, binary.BigEndian, &num)
	if err != nil {
		return 0, fmt.Errorf("failed to decode int64: %w", err)
	}
	return num, nil
}

// SortByteSlices sorts a slice of byte slices lexicographically.
// This is critical for achieving canonical representation in data structures (e.g., for multi-signature schemes' authorized public keys).
// Canonicalization ensures that identical data always produces the same hash, regardless of original order.
func SortByteSlices(slices [][]byte) {
	sort.Slice(slices, func(i, j int) bool { return bytes.Compare(slices[i], slices[j]) < 0 })
}

// --- Cryptographic & Address Utility Functions ---
// These functions are fundamental for key management and address derivation in EmPower1.

// GenerateKeyPairECDSA generates a new ECDSA private/public key pair using the P256 elliptic curve.
// This is the standard curve used in many modern blockchain and security applications.
func GenerateKeyPairECDSA() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("failed to generate ECDSA key pair: %w", err)
	}
	return privKey, nil
}

// PublicKeyToAddress derives a simplified address from an ECDSA public key.
// IMPORTANT: In a real EmPower1 production blockchain, this would involve
// a more robust hashing scheme (e.g., SHA256 then RIPEMD160) and adding a version byte
// and checksum to create a human-readable, error-checked address string.
// For now, it uses the raw marshaled public key bytes as the address.
func PublicKeyToAddress(pubKey *ecdsa.PublicKey) []byte {
	// Marshaling the public key to bytes gives a compressed or uncompressed point representation.
	// This serves as a unique identifier for now.
	return elliptic.Marshal(pubKey.Curve, pubKey.X, pubKey.Y)
}

// AddressFromPubKeyBytes (conceptual) reconstructs an ECDSA PublicKey from raw marshaled public key bytes.
// This is the inverse of PublicKeyToAddress for this simplified address scheme.
func AddressFromPubKeyBytes(addrBytes []byte) (*ecdsa.PublicKey, error) {
	x, y := elliptic.Unmarshal(elliptic.P256(), addrBytes)
	if x == nil || y == nil { // Unmarshal returns nil X,Y if data is not a valid point on the curve
		return nil, errors.New("failed to unmarshal public key from address bytes or invalid point")
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// PublicKeyBytesToHexString converts raw public key bytes to a hexadecimal string.
// Useful for display and canonical JSON representation.
func PublicKeyBytesToHexString(pubKeyBytes []byte) string {
	return hex.EncodeToString(pubKeyBytes)
}

// HexStringToPublicKeyBytes converts a hexadecimal string back to raw public key bytes.
// Useful for deserialization from JSON or configuration.
func HexStringToPublicKeyBytes(hexString string) ([]byte, error) {
	data, err := hex.DecodeString(hexString)
	if err != nil {
		return nil, fmt.Errorf("failed to decode hex string to public key bytes: %w", err)
	}
	return data, nil
}