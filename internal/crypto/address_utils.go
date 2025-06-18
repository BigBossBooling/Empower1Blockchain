package crypto

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex" // For converting addresses to hex string representation
	"errors"       // For specific error types
	"fmt"
	"hash"         // For common hashing interface
	"regexp"       // For address validation regex
	"strings"      // For string manipulation
	
	"golang.org/x/crypto/ripemd160" // For RIPEMD160 hash (common in address derivation)
)

// --- Custom Errors for Address Utilities ---
var (
	ErrInvalidAddressLength = errors.New("invalid address length")
	ErrInvalidAddressFormat = errors.New("invalid address format")
	ErrAddressChecksum      = errors.New("address checksum mismatch")
	ErrInvalidVersionByte   = errors.New("invalid address version byte")
	ErrPublicKeyHash        = errors.New("public key hash failed")
)

// AddressPrefix defines the prefix for EmPower1 addresses.
// Example: "emp" or "ep1" for human readability.
const (
	EmPower1AddressPrefix = "ep1" // For human-readable addresses, similar to Bitcoin/Substrate
	AddressVersionByte    = 0x00  // Version byte for P256K raw address (conceptual)
	AddressChecksumLength = 4     // Number of bytes for checksum
	// The actual P256 uncompressed public key bytes length is 65.
	// Hashed pubkey might be 20 bytes (RIPEMD160), or 32 (SHA256).
	// Let's assume the address is derived from RIPEMD160(SHA256(PublicKey)).
	PublicKeyHashLength   = 20    // Length of RIPEMD160 hash
	FullAddressLength     = 1 + PublicKeyHashLength + AddressChecksumLength // Version + Hash + Checksum
)


// --- Address Derivation Functions ---
// These functions are fundamental for creating and validating user addresses on EmPower1.

// HashPublicKey hashes a raw public key byte slice (e.g., 65-byte uncompressed P256)
// to derive a shorter, unique identifier, typically used as the core of an address.
// Standard derivation: RIPEMD160(SHA256(PublicKeyBytes))
func HashPublicKey(pubKeyBytes []byte) ([]byte, error) {
	if len(pubKeyBytes) == 0 {
		return nil, fmt.Errorf("%w: public key bytes cannot be empty for hashing", ErrPublicKeyHash)
	}

	// 1. SHA256 Hash
	hasher256 := sha256.New()
	hasher256.Write(pubKeyBytes)
	sha256Hash := hasher256.Sum(nil)

	// 2. RIPEMD160 Hash
	hasher160 := ripemd160.New()
	hasher160.Write(sha256Hash)
	ripemd160Hash := hasher160.Sum(nil)

	if len(ripemd160Hash) != PublicKeyHashLength {
		return nil, fmt.Errorf("%w: derived public key hash has incorrect length: expected %d, got %d", ErrPublicKeyHash, PublicKeyHashLength, len(ripemd160Hash))
	}
	return ripemd160Hash, nil
}

// Checksum generates a 4-byte checksum for address validation.
// This is typically the first 4 bytes of a double SHA256 hash.
func Checksum(payload []byte) []byte {
	firstSHA := sha256.Sum256(payload)
	secondSHA := sha256.Sum256(firstSHA[:])
	return secondSHA[:AddressChecksumLength]
}

// EncodeAddress encodes a public key hash into a full EmPower1 address string.
// This follows a conceptual Base58-like encoding scheme (similar to Bitcoin/Substrate)
// which includes a version byte and a checksum for error detection.
func EncodeAddress(pubKeyHash []byte) (string, error) {
	if len(pubKeyHash) != PublicKeyHashLength {
		return "", fmt.Errorf("%w: public key hash must be %d bytes", ErrInvalidAddressLength, PublicKeyHashLength)
	}

	// 1. Prepend version byte
	payload := append([]byte{AddressVersionByte}, pubKeyHash...)
	// 2. Append checksum
	checksum := Checksum(payload)
	payloadWithChecksum := append(payload, checksum...)

	// 3. Base58 Encode (conceptual: using a simplified encoding for now, real Base58 is complex)
	// In a real implementation, you'd use a specific Base58 encoding library that handles alphabet correctly.
	// For this conceptual example, we'll just hex encode the final bytes.
	// A proper Base58 encoding would be much more robust for human readability.
	encoded := hex.EncodeToString(payloadWithChecksum)

	// 4. Prepend human-readable prefix
	return EmPower1AddressPrefix + "_" + encoded, nil
}

// DecodeAddress decodes an EmPower1 address string back into its public key hash and validates it.
func DecodeAddress(address string) ([]byte, error) {
	if !strings.HasPrefix(address, EmPower1AddressPrefix+"_") {
		return nil, fmt.Errorf("%w: address does not start with '%s_'", ErrInvalidAddressFormat, EmPower1AddressPrefix)
	}
	hexPart := strings.TrimPrefix(address, EmPower1AddressPrefix+"_")

	payloadWithChecksum, err := hex.DecodeString(hexPart)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode hex part of address: %v", ErrInvalidAddressFormat, err)
	}

	if len(payloadWithChecksum) != FullAddressLength {
		return nil, fmt.Errorf("%w: address has incorrect decoded length: expected %d, got %d", ErrInvalidAddressLength, FullAddressLength, len(payloadWithChecksum))
	}

	// 1. Extract components
	versionByte := payloadWithChecksum[0]
	pubKeyHash := payloadWithChecksum[1 : 1+PublicKeyHashLength]
	checksum := payloadWithChecksum[1+PublicKeyHashLength:]

	// 2. Validate version byte
	if versionByte != AddressVersionByte {
		return nil, fmt.Errorf("%w: expected version byte 0x%x, got 0x%x", ErrInvalidVersionByte, AddressVersionByte, versionByte)
	}

	// 3. Verify checksum
	expectedChecksum := Checksum(payloadWithChecksum[:FullAddressLength-AddressChecksumLength]) // Payload without checksum
	if !bytes.Equal(checksum, expectedChecksum) {
		return nil, fmt.Errorf("%w: checksum mismatch", ErrAddressChecksum)
	}

	return pubKeyHash, nil
}

// IsValidAddress checks if a given string is a valid EmPower1 address.
func IsValidAddress(address string) bool {
	_, err := DecodeAddress(address)
	return err == nil
}


// --- Multi-Signature Address Derivation (Conceptual) ---
// This would be used to derive a deterministic multi-sig address from M, N, and the list of N public keys.
// The derived address ensures the 'From' field of a multi-sig transaction is canonical.
// Standard derivation: Hash(M || N || sorted_public_keys...)
func DeriveMultiSigAddress(requiredSignatures uint32, authorizedPublicKeys [][]byte) ([]byte, error) {
	if requiredSignatures == 0 || len(authorizedPublicKeys) == 0 {
		return nil, fmt.Errorf("%w: invalid multi-sig configuration for address derivation", ErrInvalidAddress)
	}
	if requiredSignatures > uint32(len(authorizedPublicKeys)) {
		return nil, fmt.Errorf("%w: M (%d) cannot be greater than N (%d) for multi-sig address derivation", ErrInvalidAddress, requiredSignatures, len(authorizedPublicKeys))
	}

	// Sort public keys canonically before hashing, crucial for deterministic address derivation.
	SortByteSlices(authorizedPublicKeys) 

	var buf bytes.Buffer
	// Include M (required signatures) and N (total authorized keys) in the hash.
	binary.Write(&buf, binary.BigEndian, requiredSignatures)
	binary.Write(&buf, binary.BigEndian, uint32(len(authorizedPublicKeys))) // N value

	for _, pk := range authorizedPublicKeys {
		buf.Write(pk)
	}

	// Hash the combined data to get the multi-sig identifier/address.
	multiSigHash := sha256.Sum256(buf.Bytes())
	return multiSigHash[:], nil // Return the raw hash as the conceptual multi-sig ID/address
}