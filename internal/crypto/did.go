package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"errors" // Explicitly import errors
	"fmt"
	"strings"

	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multicodec"
)

// --- Custom Errors for Crypto Package ---
var (
	ErrInvalidPublicKeyFormat = errors.New("invalid public key format")
	ErrUnsupportedCurve       = errors.New("unsupported elliptic curve")
	ErrDIDKeyFormat           = errors.New("invalid did:key string format")
	ErrMultibaseDecode        = errors.New("failed to decode multibase string")
	ErrUnexpectedEncoding     = errors.New("unexpected multibase encoding")
	ErrMulticodecRead         = errors.New("failed to read multicodec code")
	ErrUnexpectedMulticodec   = errors.New("unexpected multicodec type")
	ErrPubKeyLengthMismatch   = errors.New("public key length mismatch after decoding")
)

// CodecSecp256r1PubKeyUncompressed defines the multicodec for uncompressed P-256 public keys.
// This ensures standard, interoperable encoding for did:key identifiers.
const CodecSecp256r1PubKeyUncompressed multicodec.Code = 0x1201

// GenerateDIDKeySecp256r1 generates a 'did:key' identifier for an uncompressed P-256 public key.
// This is a core function for the Decentralized Identity (DID) System of EmPower1.
// Adheres to "Sense the Landscape, Secure the Solution".
func GenerateDIDKeySecp256r1(pubKeyBytes []byte) (string, error) {
	// Validate input public key format: uncompressed P-256 keys are 65 bytes (0x04 || X || Y)
	if len(pubKeyBytes) != 65 || pubKeyBytes[0] != 0x04 {
		return "", fmt.Errorf("%w: expected 65 bytes starting with 0x04 for uncompressed P-256, got %d bytes", ErrInvalidPublicKeyFormat, len(pubKeyBytes))
	}

	// 1. Prepend the multicodec prefix (varint representation of the code).
	// This makes the encoded data self-describing for the public key type.
	codecHeaderBytes := multicodec.Header(CodecSecp256r1PubKeyUncompressed)

	var prefixedPubKeyBuf bytes.Buffer
	prefixedPubKeyBuf.Write(codecHeaderBytes)
	prefixedPubKeyBuf.Write(pubKeyBytes)

	// 2. Encode the multicodec-prefixed bytes using multibase (Base58BTC is standard for did:key).
	// The 'z' prefix in Base58BTC indicates Base58BTC encoding.
	didKeyMultibasePart, err := multibase.Encode(multibase.Base58BTC, prefixedPubKeyBuf.Bytes())
	if err != nil {
		return "", fmt.Errorf("%w: failed to encode public key with Base58BTC: %v", ErrMultibaseDecode, err)
	}

	// 3. Prepend the 'did:key:' prefix to form the final DID.
	return "did:key:" + didKeyMultibasePart, nil
}

// GenerateDIDKeyFromECDSAPublicKey generates a 'did:key' identifier from an ECDSA public key.
// It ensures the key is P256 and converts it to uncompressed bytes before calling GenerateDIDKeySecp256r1.
func GenerateDIDKeyFromECDSAPublicKey(pubKey *ecdsa.PublicKey) (string, error) {
	if pubKey == nil {
		return "", fmt.Errorf("%w: public key cannot be nil", ErrInvalidPublicKeyFormat)
	}
	// Ensure the curve is P256, as specified by the multicodec.
	if pubKey.Curve != elliptic.P256() {
		return "", fmt.Errorf("%w: public key must use P256 curve, got %s", ErrUnsupportedCurve, pubKey.Curve.Params().Name)
	}
	// Marshal the public key to its uncompressed byte representation (65 bytes, starts with 0x04).
	uncompressedPubKeyBytes := elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y)
	
	// Delegate to the byte-based generation function.
	return GenerateDIDKeySecp256r1(uncompressedPubKeyBytes)
}

// ParseDIDKeySecp256r1 parses a 'did:key' identifier back into an uncompressed P-256 public key byte slice.
// This is crucial for verifying the identity associated with a DID.
func ParseDIDKeySecp256r1(didKeyString string) ([]byte, error) {
	// 1. Validate 'did:key:' prefix.
	if !strings.HasPrefix(didKeyString, "did:key:") {
		return nil, ErrDIDKeyFormat
	}
	multibasePart := strings.TrimPrefix(didKeyString, "did:key:")

	// 2. Decode the multibase part.
	encoding, decodedBytesWithCodec, err := multibase.Decode(multibasePart)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMultibaseDecode, err)
	}
	if encoding != multibase.Base58BTC {
		return nil, fmt.Errorf("%w: expected Base58BTC ('z') encoding, got %s ('%c')", ErrUnexpectedEncoding, multibase.EncodingToStr[encoding], encoding)
	}

	// 3. Consume the multicodec prefix to get the raw public key bytes.
	codec, remainingBytes, err := multicodec.Consume(decodedBytesWithCodec)
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrMulticodecRead, err)
	}

	// 4. Validate the multicodec type.
	if multicodec.Code(codec) != CodecSecp256r1PubKeyUncompressed {
		return nil, fmt.Errorf("%w: expected %s (0x%x), got %s (0x%x)",
			ErrUnexpectedMulticodec, CodecSecp256r1PubKeyUncompressed.String(), uint64(CodecSecp256r1PubKeyUncompressed),
			multicodec.Code(codec).String(), uint64(codec))
	}

	pubKeyBytes := remainingBytes

	// 5. Final validation of the public key byte slice.
	if len(pubKeyBytes) != 65 {
		return nil, fmt.Errorf("%w: expected 65 bytes, got %d", ErrPubKeyLengthMismatch, len(pubKeyBytes))
	}
	if pubKeyBytes[0] != 0x04 { // 0x04 indicates uncompressed point
		return nil, fmt.Errorf("%w: decoded public key is not in uncompressed P-256 format (missing 0x04 prefix)", ErrInvalidPublicKeyFormat)
	}

	return pubKeyBytes, nil
}