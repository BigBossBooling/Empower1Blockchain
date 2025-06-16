package crypto

import (
	"bytes" // Added for bytes.Equal
	"crypto/elliptic"
	"testing"
	"strings"
)

func TestGenerateDIDKeySecp256r1(t *testing.T) {
	// Generate a new P256 key pair for testing
	privKey, err := GenerateECDSAKeyPair() // Assumes P256 is default
	if err != nil {
		t.Fatalf("Failed to generate ECDSA key pair: %v", err)
	}
	pubKey := &privKey.PublicKey
	uncompressedPubKeyBytes := elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y)

	didKey, err := GenerateDIDKeySecp256r1(uncompressedPubKeyBytes)
	if err != nil {
		t.Fatalf("GenerateDIDKeySecp256r1() error = %v", err)
	}

	// 1. Check for "did:key:" prefix
	if !strings.HasPrefix(didKey, "did:key:") {
		t.Errorf("DID key '%s' does not start with 'did:key:'", didKey)
	}

	// 2. Check for multibase 'z' prefix (Base58BTC)
	multibasePart := strings.TrimPrefix(didKey, "did:key:")
	if !strings.HasPrefix(multibasePart, "z") {
		t.Errorf("DID key part '%s' does not start with multibase prefix 'z'", multibasePart)
	}

	// 3. Try to parse it back (basic validation of structure)
	parsedPubKeyBytes, err := ParseDIDKeySecp256r1(didKey)
	if err != nil {
		t.Fatalf("ParseDIDKeySecp256r1() failed for generated DID key '%s': %v", didKey, err)
	}

	// 4. Compare original public key bytes with parsed ones
	if !bytes.Equal(uncompressedPubKeyBytes, parsedPubKeyBytes) { // Use bytes.Equal
		t.Errorf("Original public key bytes and parsed public key bytes do not match")
		t.Logf("Original: %x", uncompressedPubKeyBytes)
		t.Logf("Parsed:   %x", parsedPubKeyBytes)
	}

	// Test with invalid public key format
	invalidPubKeyBytes := []byte{0x04, 0x01, 0x02, 0x03} // Too short
	_, err = GenerateDIDKeySecp256r1(invalidPubKeyBytes)
	if err == nil {
		t.Errorf("GenerateDIDKeySecp256r1() expected error for invalid public key, got nil")
	} else if !strings.Contains(err.Error(), "invalid uncompressed P-256 public key format") {
		t.Errorf("GenerateDIDKeySecp256r1() wrong error for invalid pubkey: %v", err)
	}

	t.Logf("Generated DID Key: %s", didKey)
}

func TestParseDIDKeySecp256r1(t *testing.T) {
	// Example valid did:key for a P256 key (structure, not a specific known key)
	// This is a conceptual example of what a did:key might look like.
	// The actual value comes from base58btc encoding of (0x1201 + 65-byte-pubkey)
	// A known test vector would be better if available.
	// For now, we'll generate one and use it.

	privKey, _ := GenerateECDSAKeyPair()
	pubKey := &privKey.PublicKey
	uncompressedPubKeyBytes := elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y)
	validDIDKey, _ := GenerateDIDKeySecp256r1(uncompressedPubKeyBytes)
	t.Logf("Testing with generated valid DID: %s", validDIDKey)


	tests := []struct {
		name        string
		didKeyStr   string
		expectError bool
		errorContains string
		expectedKey []byte // nil if expecting error or don't care about specific key
	}{
		{
			name:        "Valid DID Key",
			didKeyStr:   validDIDKey,
			expectError: false,
			expectedKey: uncompressedPubKeyBytes,
		},
		{
			name:        "Invalid prefix",
			didKeyStr:   "did:foo:zQ3s...",
			expectError: true,
			errorContains: "invalid did:key string format",
		},
		{
			name:        "Invalid multibase prefix",
			didKeyStr:   "did:key:bQ3s...", // 'b' is base32, not 'z' for base58btc
			expectError: true,
			errorContains: "expected Base58BTC ('z') encoding",
		},
		{
			name:        "Invalid multicodec (too short after decode)",
			didKeyStr:   "did:key:z123", // Base58 decodes to very few bytes
			expectError: true,
			errorContains: "failed to read multicodec header", // or could be multibase decode error
		},
		// To test wrong multicodec value, one would need a valid base58 string
		// that decodes to bytes containing a different valid multicodec prefix.
		// e.g., did:key:z<base58(0xOTHER_CODE + key_bytes)>
		// This is harder to construct without specific examples.
		// For now, the GenerateDIDKeySecp256r1 test implicitly tests the correct codec is used.
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedKey, err := ParseDIDKeySecp256r1(tt.didKeyStr)
			if tt.expectError {
				if err == nil {
					t.Errorf("Expected error, got nil")
				} else if tt.errorContains != "" && !strings.Contains(err.Error(), tt.errorContains) {
					t.Errorf("Expected error containing '%s', got: %v", tt.errorContains, err)
				}
			} else {
				if err != nil {
					t.Errorf("Expected no error, got: %v", err)
				}
				if tt.expectedKey != nil && !bytes.Equal(parsedKey, tt.expectedKey) { // Use bytes.Equal
					t.Errorf("Parsed key mismatch.\nExpected: %x\nGot:      %x", tt.expectedKey, parsedKey)
				}
			}
		})
	}
}

// bytesEqual is a helper for comparing byte slices, as bytes.Equal is not available here directly.
// Oh, wait, `bytes` package should be available in test files.
// bytesEqual helper removed, using standard bytes.Equal now.
