package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/sha256" // For address derivation if not raw pubkey
	"crypto/x509"
	"encoding/hex"
	"encoding/pem"
	"errors" // Explicitly import errors
	"fmt"
	"os"
	"strings" // Added for DID parsing

	// Removed multiformats imports if did:key moved to utils.go, or kept if this package is solely for crypto primitives.
	// Assuming did:key generation/parsing lives here as it's crypto-specific.
	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multicodec"
)

// --- Custom Error Definitions for the Crypto Package ---
var (
	ErrInvalidKeyFormat         = errors.New("invalid key format")
	ErrUnsupportedCurve         = errors.New("unsupported elliptic curve")
	ErrKeyGeneration            = errors.New("key generation failed")
	ErrKeySerialization         = errors.New("key serialization failed")
	ErrKeyDeserialization       = errors.New("key deserialization failed")
	ErrPEMEncoding              = errors.New("pem encoding error")
	ErrPEMDecoding              = errors.New("pem decoding error")
	ErrPEMEncrypted             = errors.New("pem block is encrypted and decryption not supported by this method") // Explicit
	ErrUnsupportedPEMType       = errors.New("unsupported pem block type")
	ErrInvalidDIDKeyFormat      = errors.New("invalid did:key string format")
	ErrMultibaseDecodeFailed    = errors.New("failed to decode multibase string")
	ErrUnexpectedMultibaseEnc   = errors.New("unexpected multibase encoding")
	ErrMulticodecReadFailed     = errors.New("failed to read multicodec code")
	ErrUnexpectedMulticodecType = errors.New("unexpected multicodec type")
	ErrPubKeyLengthMismatch     = errors.New("public key length mismatch")
)

// --- Constants for DID Key Generation ---
const (
	CodecSecp256r1PubKeyUncompressed multicodec.Code = 0x1201
	P256UncompressedPubKeyLength     = 65 // 0x04 prefix + 32-byte X + 32-byte Y
)

// --- Key Generation ---

// GenerateECDSAKeyPair generates a new ECDSA private and public key pair using the P256 elliptic curve.
func GenerateECDSAKeyPair() (*ecdsa.PrivateKey, error) {
	privKey, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to generate ECDSA key pair: %v", ErrKeyGeneration, err)
	}
	return privKey, nil
}

// --- Public Key Serialization/Deserialization (Raw Bytes) ---

// SerializePublicKeyToBytes marshals an ECDSA public key to its uncompressed byte representation (65 bytes).
func SerializePublicKeyToBytes(pubKey *ecdsa.PublicKey) ([]byte, error) {
	if pubKey == nil {
		return nil, fmt.Errorf("%w: public key is nil", ErrKeySerialization)
	}
	if pubKey.Curve != elliptic.P256() { // Ensure consistent curve for marshalling
		return nil, fmt.Errorf("%w: public key curve must be P256 for serialization, got %s", ErrUnsupportedCurve, pubKey.Curve.Params().Name)
	}
	return elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y), nil
}

// DeserializePublicKeyFromBytes unmarshals an uncompressed P-256 public key byte slice (65 bytes) to an *ecdsa.PublicKey.
func DeserializePublicKeyFromBytes(pubKeyBytes []byte) (*ecdsa.PublicKey, error) {
	if len(pubKeyBytes) != P256UncompressedPubKeyLength { // Check length
		return nil, fmt.Errorf("%w: public key bytes must be %d bytes for P256 uncompressed, got %d", ErrInvalidKeyFormat, P256UncompressedPubKeyLength, len(pubKeyBytes))
	}
	// Check for uncompressed format prefix
	if pubKeyBytes[0] != 0x04 { 
		return nil, fmt.Errorf("%w: public key bytes must be in uncompressed format (start with 0x04)", ErrInvalidKeyFormat)
	}

	x, y := elliptic.Unmarshal(elliptic.P256(), pubKeyBytes)
	if x == nil || y == nil { // Unmarshal returns nil X,Y if it fails to parse a valid point
		return nil, fmt.Errorf("%w: failed to unmarshal public key bytes to elliptic curve point", ErrKeyDeserialization)
	}
	return &ecdsa.PublicKey{Curve: elliptic.P256(), X: x, Y: y}, nil
}

// --- Address Derivation and Conversion ---
// This assumes a simple raw public key byte address.
// In a real blockchain, addresses are typically hashed (e.g., SHA256 + RIPEMD160) for brevity and checksum.

// PublicKeyBytesToAddress converts raw public key bytes to a hexadecimal string address.
// This is typically for display or comparison.
func PublicKeyBytesToAddress(pubKeyBytes []byte) string {
	return hex.EncodeToString(pubKeyBytes)
}

// AddressToPublicKeyBytes converts a hex string address back to raw public key bytes.
func AddressToPublicKeyBytes(address string) ([]byte, error) {
	data, err := hex.DecodeString(address)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to decode hex address string: %v", ErrInvalidAddressFormat, err)
	}
	return data, nil
}

// --- PEM Encoding/Decoding (for storing/loading keys from files) ---
// Adheres to "Sense the Landscape, Secure the Solution" for key management.

// SerializePrivateKeyToPEM converts an ECDSA private key to PEM format (PKCS#8 unencrypted or SEC1).
// This function aims for standard Go library compatibility, which best supports unencrypted keys.
// NOTE: This implementation does NOT support password-encrypted PEMs currently.
func SerializePrivateKeyToPEM(privKey *ecdsa.PrivateKey, password []byte) ([]byte, error) {
	if privKey == nil {
		return nil, fmt.Errorf("%w: private key is nil", ErrKeySerialization)
	}
	if len(password) > 0 {
		return nil, fmt.Errorf("%w: password-encrypted PEM serialization not supported by this method", ErrPEMEncrypted)
	}

	// Prefer PKCS#8 for broader compatibility where possible, otherwise SEC1.
	// x509.MarshalPKCS8PrivateKey is generally preferred.
	derBytes, err := x509.MarshalPKCS8PrivateKey(privKey)
	if err != nil {
		// Fallback to SEC1 if PKCS8 fails, though it generally shouldn't for ECDSA.
		derBytes, err = x509.MarshalECPrivateKey(privKey) // SEC1 format
		if err != nil {
			return nil, fmt.Errorf("%w: failed to marshal private key to DER (PKCS8/SEC1): %v", ErrKeySerialization, err)
		}
		pemBlock := &pem.Block{Type: "EC PRIVATE KEY", Bytes: derBytes} // SEC1 format
		return pem.EncodeToMemory(pemBlock), nil
	}
	// PKCS#8 unencrypted format
	pemBlock := &pem.Block{Type: "PRIVATE KEY", Bytes: derBytes} 
	return pem.EncodeToMemory(pemBlock), nil
}

// DeserializePrivateKeyFromPEM converts PEM formatted bytes back to an ECDSA private key.
// This function supports standard unencrypted PKCS#8 ("PRIVATE KEY") and SEC1 ("EC PRIVATE KEY").
// NOTE: This implementation does NOT support password-encrypted PEMs.
func DeserializePrivateKeyFromPEM(pemBytes []byte, password []byte) (*ecdsa.PrivateKey, error) {
	block, rest := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("%w: failed to decode PEM block", ErrPEMDecoding)
	}
	if len(rest) > 0 { // Check if there's any data left after decoding one block
		return nil, fmt.Errorf("%w: unexpected trailing data after PEM block", ErrPEMDecoding)
	}

	if x509.IsEncryptedPEMBlock(block) {
		return nil, fmt.Errorf("%w: pem block is encrypted", ErrPEMEncrypted)
	}
	if len(password) > 0 { // If password was provided but block isn't flagged as encrypted
		// This suggests a potential misuse or a non-standard encryption that x509 won't handle.
		// Log a warning or error if strict. For now, we error.
		return nil, fmt.Errorf("%w: password provided but PEM block is not marked as encrypted (or encrypted format not supported)", ErrPEMEncrypted)
	}

	var privKey interface{}
	var err error

	switch block.Type {
	case "EC PRIVATE KEY": // SEC1 format
		privKey, err = x509.ParseECPrivateKey(block.Bytes)
	case "PRIVATE KEY": // PKCS#8 unencrypted
		privKey, err = x509.ParsePKCS8PrivateKey(block.Bytes)
	default:
		return nil, fmt.Errorf("%w: unsupported PEM block type for private key: %s", ErrUnsupportedPEMType, block.Type)
	}

	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse private key DER bytes: %v", ErrKeyDeserialization, err)
	}

	ecdsaPrivKey, ok := privKey.(*ecdsa.PrivateKey)
	if !ok {
		return nil, fmt.Errorf("%w: key parsed is not an ECDSA private key", ErrKeyDeserialization)
	}
	return ecdsaPrivKey, nil
}

// SerializePublicKeyToPEM converts an ECDSA public key to PEM format (PKIX).
func SerializePublicKeyToPEM(pubKey *ecdsa.PublicKey) ([]byte, error) {
	if pubKey == nil {
		return nil, fmt.Errorf("%w: public key is nil", ErrKeySerialization)
	}
	derBytes, err := x509.MarshalPKIXPublicKey(pubKey)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to marshal public key to DER (PKIX): %v", ErrKeySerialization, err)
	}
	pemBlock := &pem.Block{
		Type:  "PUBLIC KEY",
		Bytes: derBytes,
	}
	return pem.EncodeToMemory(pemBlock), nil
}

// DeserializePublicKeyFromPEM converts PEM formatted bytes (PKIX) back to an ECDSA public key.
func DeserializePublicKeyFromPEM(pemBytes []byte) (*ecdsa.PublicKey, error) {
	block, rest := pem.Decode(pemBytes)
	if block == nil {
		return nil, fmt.Errorf("%w: failed to decode PEM block", ErrPEMDecoding)
	}
	if len(rest) > 0 { // Check for extra data after PEM block
		return nil, fmt.Errorf("%w: unexpected trailing data after PEM block", ErrPEMDecoding)
	}
	if block.Type != "PUBLIC KEY" {
		return nil, fmt.Errorf("%w: expected PEM block type 'PUBLIC KEY', got '%s'", ErrUnsupportedPEMType, block.Type)
	}

	pub, err := x509.ParsePKIXPublicKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("%w: failed to parse DER encoded public key (PKIX): %v", ErrKeyDeserialization, err)
	}
	ecdsaPubKey, ok := pub.(*ecdsa.PublicKey)
	if !ok {
		return nil, fmt.Errorf("%w: key parsed from PEM is not an ECDSA public key", ErrKeyDeserialization)
	}
	return ecdsaPubKey, nil
}

// SavePrivateKeyPEM saves a private key to a file in PEM format (unencrypted PKCS#8 or SEC1).
func SavePrivateKeyPEM(privKey *ecdsa.PrivateKey, filePath string, password []byte) error {
	pemBytes, err := SerializePrivateKeyToPEM(privKey, password)
	if err != nil {
		return fmt.Errorf("failed to serialize private key to PEM: %w", err)
	}
	// Use 0600 for owner-only read/write permissions for private keys.
	return os.WriteFile(filePath, pemBytes, 0600)
}

// LoadPrivateKeyPEM loads a private key from a PEM file (unencrypted PKCS#8 or SEC1).
func LoadPrivateKeyPEM(filePath string, password []byte) (*ecdsa.PrivateKey, error) {
	pemBytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("private key file not found at '%s': %w", filePath, err)
		}
		return nil, fmt.Errorf("failed to read private key file '%s': %w", filePath, err)
	}
	return DeserializePrivateKeyFromPEM(pemBytes, password)
}

// SavePublicKeyPEM saves a public key to a file in PEM format (PKIX).
func SavePublicKeyPEM(pubKey *ecdsa.PublicKey, filePath string) error {
	pemBytes, err := SerializePublicKeyToPEM(pubKey)
	if err != nil {
		return fmt.Errorf("failed to serialize public key to PEM: %w", err)
	}
	// Use 0644 for owner read/write, group/other read permissions for public keys.
	return os.WriteFile(filePath, pemBytes, 0644)
}

// LoadPublicKeyPEM loads a public key from a PEM file (PKIX).
func LoadPublicKeyPEM(filePath string) (*ecdsa.PublicKey, error) {
	pemBytes, err := os.ReadFile(filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, fmt.Errorf("public key file not found at '%s': %w", filePath, err)
		}
		return nil, fmt.Errorf("failed to read public key file '%s': %w", filePath, err)
	}
	return DeserializePublicKeyFromPEM(pemBytes)
}


// --- DID Key Generation/Parsing (Moved from utils.go if centralizing here) ---
// If this package is responsible for all crypto-related aspects, including DID key formatting.

// CodecSecp256r1PubKeyUncompressed defines the multicodec for uncompressed P-256 public keys.
const CodecSecp256r1PubKeyUncompressed multicodec.Code = 0x1201 // Redefine if not already in this file


// GenerateDIDKeySecp256r1 generates a 'did:key' identifier for an uncompressed P-256 public key.
func GenerateDIDKeySecp256r1(pubKeyBytes []byte) (string, error) {
	if len(pubKeyBytes) != P256UncompressedPubKeyLength || pubKeyBytes[0] != 0x04 {
		return "", fmt.Errorf("%w: expected %d bytes starting with 0x04 for uncompressed P-256, got %d bytes", ErrInvalidKeyFormat, P256UncompressedPubKeyLength, len(pubKeyBytes))
	}

	codecHeaderBytes := multicodec.Header(CodecSecp256r1PubKeyUncompressed)
	var prefixedPubKeyBuf bytes.Buffer
	prefixedPubKeyBuf.Write(codecHeaderBytes)
	prefixedPubKeyBuf.Write(pubKeyBytes)

	didKeyMultibasePart, err := multibase.Encode(multibase.Base58BTC, prefixedPubKeyBuf.Bytes())
	if err != nil {
		return "", fmt.Errorf("%w: failed to encode public key with Base58BTC: %v", ErrMultibaseDecodeFailed, err)
	}

	return "did:key:" + didKeyMultibasePart, nil
}

// GenerateDIDKeyFromECDSAPublicKey generates a 'did:key' identifier from an ECDSA public key.
func GenerateDIDKeyFromECDSAPublicKey(pubKey *ecdsa.PublicKey) (string, error) {
	if pubKey == nil {
		return "", fmt.Errorf("%w: public key cannot be nil", ErrInvalidKeyFormat)
	}
	if pubKey.Curve != elliptic.P256() {
		return "", fmt.Errorf("%w: public key curve must be P256, got %s", ErrUnsupportedCurve, pubKey.Curve.Params().Name)
	}
	uncompressedPubKeyBytes, err := SerializePublicKeyToBytes(pubKey) // Use internal serialization
	if err != nil {
		return "", fmt.Errorf("%w: failed to serialize public key to bytes for DID generation: %v", ErrKeySerialization, err)
	}
	return GenerateDIDKeySecp256r1(uncompressedPubKeyBytes)
}

// ParseDIDKeySecp256r1 parses a 'did:key' identifier back into an uncompressed P-256 public key byte slice.
func ParseDIDKeySecp256r1(didKeyString string) ([]byte, error) {
	if !strings.HasPrefix(didKeyString, "did:key:") {
		return nil, ErrInvalidDIDKeyFormat
	}
	multibasePart := strings.TrimPrefix(didKeyString, "did:key:")

	encoding, decodedBytesWithCodec, err := multibase.Decode(multibasePart)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMultibaseDecodeFailed, err)
	}
	if encoding != multibase.Base58BTC {
		return nil, fmt.Errorf("%w: expected Base58BTC ('z') encoding, got %s ('%c')", ErrUnexpectedEncoding, multibase.EncodingToStr[encoding], encoding)
	}

	codec, remainingBytes, err := multicodec.Consume(decodedBytesWithCodec)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMulticodecReadFailed, err)
	}

	if multicodec.Code(codec) != CodecSecp256r1PubKeyUncompressed {
		return nil, fmt.Errorf("%w: expected %s (0x%x), got %s (0x%x)",
			ErrUnexpectedMulticodecType, CodecSecp256r1PubKeyUncompressed.String(), uint64(CodecSecp256r1PubKeyUncompressed),
			multicodec.Code(codec).String(), uint64(codec))
	}

	pubKeyBytes := remainingBytes

	if len(pubKeyBytes) != P256UncompressedPubKeyLength {
		return nil, fmt.Errorf("%w: expected %d bytes, got %d", ErrPubKeyLengthMismatch, P256UncompressedPubKeyLength, len(pubKeyBytes))
	}
	if pubKeyBytes[0] != 0x04 {
		return nil, fmt.Errorf("%w: decoded public key is not in uncompressed P-256 format (missing 0x04 prefix)", ErrInvalidKeyFormat)
	}

	return pubKeyBytes, nil
}