package crypto

import (
	"bytes"
	"crypto/elliptic"
	"strings"
	"testing"

	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multicodec"
)

// --- Helper Functions for Testing ---
// newDummyPrivateKey and GenerateECDSAKeyPair are assumed to be in the 'core' package or a 'test_utils' package
// For this test file context, assuming they are available or mocked.
// If GenerateECDSAKeyPair is needed from internal/crypto:
// import "empower1.com/core/internal/crypto"

// Test suite for GenerateDIDKeySecp256r1 and ParseDIDKeySecp256r1
func TestDIDKeySecp256r1GenerationAndParsing(t *testing.T) {
	// Generate a new P256 key pair for testing
	// Assumes GenerateECDSAKeyPair is accessible, e.g., from core.GenerateECDSAKeyPair
	privKey, err := GenerateECDSAKeyPair() 
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key pair for testing: %v", err)
	}
	pubKey := &privKey.PublicKey
	uncompressedPubKeyBytes := elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y)

	t.Run("GenerateValidDIDKey", func(t *testing.T) {
		didKey, err := GenerateDIDKeySecp256r1(uncompressedPubKeyBytes)
		if err != nil {
			t.Fatalf("GenerateDIDKeySecp256r1() error = %v", err)
		}

		// 1. Check for "did:key:" prefix
		if !strings.HasPrefix(didKey, "did:key:") {
			t.Errorf("DID key '%s' does not start with 'did:key:'", didKey)
		}

		// 2. Check for multibase 'z' prefix (Base58BTC is 'z')
		multibasePart := strings.TrimPrefix(didKey, "did:key:")
		if !strings.HasPrefix(multibasePart, string(multibase.Base58BTC)) { // Use string(multibase.Base58BTC) for 'z'
			t.Errorf("DID key part '%s' does not start with multibase prefix 'z'", multibasePart)
		}

		// 3. Try to parse it back to validate round-trip conversion
		parsedPubKeyBytes, err := ParseDIDKeySecp256r1(didKey)
		if err != nil {
			t.Fatalf("ParseDIDKeySecp256r1() failed for generated DID key '%s': %v", didKey, err)
		}

		// 4. Compare original public key bytes with parsed ones
		if !bytes.Equal(uncompressedPubKeyBytes, parsedPubKeyBytes) {
			t.Errorf("Original public key bytes and parsed public key bytes do not match")
			t.Logf("Original: %x", uncompressedPubKeyBytes)
			t.Logf("Parsed:   %x", parsedPubKeyBytes)
		}

		t.Logf("Generated DID Key: %s", didKey)
	})

	t.Run("GenerateWithInvalidPublicKeyFormat", func(t *testing.T) {
		invalidPubKeyBytes := []byte{0x04, 0x01, 0x02, 0x03} // Too short (65 bytes expected)
		_, err := GenerateDIDKeySecp256r1(invalidPubKeyBytes)
		if err == nil {
			t.Errorf("Expected error for invalid public key format (too short), got nil")
		} else if !errors.Is(err, ErrInvalidPublicKeyFormat) { // Check specific error type
			t.Errorf("Expected ErrInvalidPublicKeyFormat, got: %v", err)
		} else if !strings.Contains(err.Error(), "invalid uncompressed P-256 public key format") {
			t.Errorf("Expected error containing 'invalid uncompressed P-256 public key format', got: %v", err)
		}

		// Test with wrong prefix byte (not 0x04)
		wrongPrefixPubKeyBytes := make([]byte, 65)
		wrongPrefixPubKeyBytes[0] = 0x01 // Wrong prefix
		copy(wrongPrefixPubKeyBytes[1:], uncompressedPubKeyBytes[1:]) // Rest of bytes can be valid
		_, err = GenerateDIDKeySecp256r1(wrongPrefixPubKeyBytes)
		if err == nil {
			t.Errorf("Expected error for wrong public key prefix, got nil")
		} else if !errors.Is(err, ErrInvalidPublicKeyFormat) {
			t.Errorf("Expected ErrInvalidPublicKeyFormat, got: %v", err)
		}
	})
}

func TestGenerateDIDKeyFromECDSAPublicKey(t *testing.T) {
	privKey, _ := GenerateECDSAKeyPair() // Generate a valid key
	pubKey := &privKey.PublicKey
	uncompressedPubKeyBytes := elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y)

	t.Run("ValidECDSAPublicKey", func(t *testing.T) {
		didKey, err := GenerateDIDKeyFromECDSAPublicKey(pubKey)
		if err != nil {
			t.Fatalf("GenerateDIDKeyFromECDSAPublicKey() error = %v", err)
		}
		// Verify it's a valid DID key by parsing it back
		parsedPubKeyBytes, err := ParseDIDKeySecp256r1(didKey)
		if err != nil {
			t.Fatalf("Failed to parse generated DID key from ECDSA pubkey: %v", err)
		}
		if !bytes.Equal(uncompressedPubKeyBytes, parsedPubKeyBytes) {
			t.Errorf("Generated DID key from ECDSA pubkey mismatch")
		}
	})

	t.Run("NilECDSAPublicKey", func(t *testing.T) {
		_, err := GenerateDIDKeyFromECDSAPublicKey(nil)
		if err == nil {
			t.Errorf("Expected error for nil ECDSA public key, got nil")
		} else if !errors.Is(err, ErrInvalidPublicKeyFormat) {
			t.Errorf("Expected ErrInvalidPublicKeyFormat, got: %v", err)
		}
	})

	t.Run("UnsupportedCurveECDSAPublicKey", func(t *testing.T) {
		// Create a key on a different curve (e.g., P384) to test unsupported curve
		privKeyP384, err := ecdsa.GenerateKey(elliptic.P384(), rand.Reader) // Use rand.Reader
		if err != nil { t.Fatalf("Failed to generate P384 key: %v", err) }
		
		_, err = GenerateDIDKeyFromECDSAPublicKey(&privKeyP384.PublicKey)
		if err == nil {
			t.Errorf("Expected error for unsupported curve (P384), got nil")
		} else if !errors.Is(err, ErrUnsupportedCurve) {
			t.Errorf("Expected ErrUnsupportedCurve, got: %v", err)
		}
	})
}

func TestParseDIDKeySecp256r1(t *testing.T) {
	// Generate a valid did:key string to use in tests
	privKey, _ := GenerateECDSAKeyPair()
	uncompressedPubKeyBytes := elliptic.Marshal(elliptic.P256(), privKey.PublicKey.X, privKey.PublicKey.Y)
	validDIDKey, _ := GenerateDIDKeySecp256r1(uncompressedPubKeyBytes)

	t.Logf("Testing ParseDIDKeySecp256r1 with generated valid DID: %s", validDIDKey)

	tests := []struct {
		name          string
		didKeyStr     string
		expectError   bool
		expectedError error // Specific error type to check
		errorContains string // Substring to check in error message
		expectedKey   []byte // nil if expecting error or don't care about specific key
	}{
		{
			name:        "Valid DID Key",
			didKeyStr:   validDIDKey,
			expectError: false,
			expectedKey: uncompressedPubKeyBytes,
		},
		{
			name:        "Invalid did:key prefix",
			didKeyStr:   "foo:key:zQ3s",
			expectError: true,
			expectedError: ErrDIDKeyFormat,
			errorContains: "missing 'did:key:' prefix",
		},
		{
			name:        "Invalid multibase encoding prefix",
			didKeyStr:   "did:key:bQ3sY...", // 'b' is base32, not 'z' for Base58BTC
			expectError: true,
			expectedError: ErrUnexpectedEncoding,
			errorContains: "expected Base58BTC ('z') encoding",
		},
		{
			name:        "Malformed multibase part (not valid base58)",
			didKeyStr:   "did:key:zQ_!", // Contains invalid base58 chars
			expectError: true,
			expectedError: ErrMultibaseDecode,
			errorContains: "failed to decode multibase string",
		},
		{
			name:        "Invalid multicodec - too short after decode",
			didKeyStr:   "did:key:z123", // Decodes to very few bytes, not enough for multicodec header + key
			expectError: true,
			expectedError: ErrMulticodecRead,
			errorContains: "failed to read multicodec code",
		},
		{
			name:        "Unexpected multicodec type",
			// This requires constructing a valid multibase string with a different codec.
			// Example for 0x01 (raw binary): encode(Base58BTC, multicodec.Header(0x01) + pubKeyBytes[:10])
			didKeyStr: func() string {
				// Create a dummy key and encode it with a different codec (e.g., 0x01 for raw binary)
				// This simulates a different valid multicodec being found.
				var buf bytes.Buffer
				buf.Write(multicodec.Header(0x01)) // Use a different valid multicodec
				buf.Write(uncompressedPubKeyBytes[1:6]) // Just part of the key for a valid parse
				s, _ := multibase.Encode(multibase.Base58BTC, buf.Bytes())
				return "did:key:" + s
			}(),
			expectError: true,
			expectedError: ErrUnexpectedMulticodec,
			errorContains: "unexpected multicodec type",
		},
		{
			name:        "Public key length mismatch (too short)",
			didKeyStr: func() string {
				// Encode a shorter public key
				var buf bytes.Buffer
				buf.Write(multicodec.Header(CodecSecp256r1PubKeyUncompressed))
				buf.Write(uncompressedPubKeyBytes[:10]) // Only 10 bytes of key
				s, _ := multibase.Encode(multibase.Base58BTC, buf.Bytes())
				return "did:key:" + s
			}(),
			expectError: true,
			expectedError: ErrPubKeyLengthMismatch,
			errorContains: "public key length mismatch",
		},
		{
			name:        "Public key missing 0x04 prefix (not uncompressed)",
			didKeyStr: func() string {
				// Encode a pubkey with 0x03 prefix (compressed, not what we expect)
				compressedPubKey := make([]byte, 33)
				compressedPubKey[0] = 0x03 // Simulate compressed prefix
				copy(compressedPubKey[1:], uncompressedPubKeyBytes[1:33]) // Fill with some bytes
				var buf bytes.Buffer
				buf.Write(multicodec.Header(CodecSecp256r1PubKeyUncompressed))
				buf.Write(compressedPubKey) // Use the compressed key here
				s, _ := multibase.Encode(multibase.Base58BTC, buf.Bytes())
				return "did:key:" + s
			}(),
			expectError: true,
			expectedError: ErrInvalidPublicKeyFormat,
			errorContains: "not in uncompressed P-256 format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedKey, err := ParseDIDKeySecp256r1(tt.didKeyStr)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if tt.expectedError != nil && !errors.Is(err, tt.expectedError) {
					t.Errorf("Expected error type %v, got: %v", tt.expectedError, err)
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if tt.expectedKey != nil && !bytes.Equal(parsedKey, tt.expectedKey) {
					t.Errorf("Parsed key mismatch.\nExpected: %x\nGot:      %x", tt.expectedKey, parsedKey)
				}
			}
		})
	}
}