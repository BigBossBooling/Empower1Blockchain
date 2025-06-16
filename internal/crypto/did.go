package crypto

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"fmt"
	"strings"

	"github.com/multiformats/go-multibase"
	"github.com/multiformats/go-multicodec"
)

const (
	CodecSecp256r1PubKeyUncompressed multicodec.Code = 0x1201
)

func GenerateDIDKeySecp256r1(pubKeyBytes []byte) (string, error) {
	if len(pubKeyBytes) != 65 || pubKeyBytes[0] != 0x04 {
		return "", fmt.Errorf("invalid uncompressed P-256 public key format: expected 65 bytes starting with 0x04, got %d bytes", len(pubKeyBytes))
	}

	// 1. Prepend the multicodec prefix.
	// The multicodec.Header function returns the varint representation of the code.
	codecHeaderBytes := multicodec.Header(CodecSecp256r1PubKeyUncompressed)

	var prefixedPubKeyBuf bytes.Buffer
	prefixedPubKeyBuf.Write(codecHeaderBytes)
	prefixedPubKeyBuf.Write(pubKeyBytes)

	didKeyMultibasePart, err := multibase.Encode(multibase.Base58BTC, prefixedPubKeyBuf.Bytes())
	if err != nil {
		return "", fmt.Errorf("failed to encode public key with Base58BTC: %w", err)
	}

	return "did:key:" + didKeyMultibasePart, nil
}

func GenerateDIDKeyFromECDSAPublicKey(pubKey *ecdsa.PublicKey) (string, error) {
	if pubKey == nil || pubKey.Curve != elliptic.P256() {
		return "", fmt.Errorf("public key must be a P256 ECDSA key")
	}
	uncompressedPubKey := elliptic.Marshal(elliptic.P256(), pubKey.X, pubKey.Y)
	return GenerateDIDKeySecp256r1(uncompressedPubKey)
}

func ParseDIDKeySecp256r1(didKeyString string) ([]byte, error) {
	if !strings.HasPrefix(didKeyString, "did:key:") {
		return nil, fmt.Errorf("invalid did:key string format: missing 'did:key:' prefix")
	}
	multibasePart := strings.TrimPrefix(didKeyString, "did:key:")

	encoding, decodedBytesWithCodec, err := multibase.Decode(multibasePart)
	if err != nil {
		return nil, fmt.Errorf("failed to decode multibase string: %w", err)
	}
	if encoding != multibase.Base58BTC {
		return nil, fmt.Errorf("expected Base58BTC ('z') encoding, got: %s (%c)", multibase.EncodingToStr[encoding], encoding)
	}

	// Use multicodec.Consume to read the code and get the rest of the data.
	codec, remainingBytes, err := multicodec.Consume(decodedBytesWithCodec)
	if err != nil {
		return nil, fmt.Errorf("failed to read/consume multicodec code: %w", err)
	}

	if multicodec.Code(codec) != CodecSecp256r1PubKeyUncompressed { // Cast uint64 to multicodec.Code for comparison
		return nil, fmt.Errorf("unexpected multicodec type: expected %s (0x%x), got %s (0x%x)",
			CodecSecp256r1PubKeyUncompressed.String(), uint64(CodecSecp256r1PubKeyUncompressed),
			multicodec.Code(codec).String(), uint64(codec))
	}

	pubKeyBytes := remainingBytes

	if len(pubKeyBytes) != 65 {
		return nil, fmt.Errorf("decoded public key has incorrect length: expected 65, got %d", len(pubKeyBytes))
	}
	if pubKeyBytes[0] != 0x04 {
		return nil, fmt.Errorf("decoded public key is not in uncompressed format (missing 0x04 prefix)")
	}

	return pubKeyBytes, nil
}
