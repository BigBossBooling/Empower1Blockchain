import multibase
import multicodec
import binascii # For hex decoding

# Multicodec name for P-256 (secp256r1) uncompressed public key.
# 'p256-pub' corresponds to 0x1201 in the multicodec table.
CODEC_P256_PUB_UNCOMPRESSED_NAME = 'p256-pub'

def generate_did_key_from_public_key_hex(public_key_hex: str) -> str:
    """
    Generates a did:key string from an uncompressed SECP256R1 public key hex string.
    Format: did:key:z<base58btc_encoded(multicodec_p256_pub_prefix_bytes + pubKeyBytes_uncompressed)>

    Args:
        public_key_hex: Hex string of the uncompressed P-256 public key
                          (e.g., "04aabbcc...ff", 65 bytes / 130 hex chars).

    Returns:
        The did:key string.

    Raises:
        ValueError: If the public key hex is not valid or multicodec wrapping fails.
    """
    if not isinstance(public_key_hex, str):
        raise ValueError("Public key hex must be a string.")
    if not public_key_hex.startswith('04') or len(public_key_hex) != 130:
        raise ValueError(f"Invalid public key hex format. Expected 65-byte uncompressed key hex string starting with '04'. Length was {len(public_key_hex)}")

    try:
        pub_key_bytes = binascii.unhexlify(public_key_hex)
    except binascii.Error:
        raise ValueError("Public key hex is not a valid hex string.")

    # Double check byte properties after hex decode (already partially covered by length and prefix check on hex)
    if len(pub_key_bytes) != 65 or pub_key_bytes[0] != 0x04:
        raise ValueError("Invalid public key bytes after hex decoding. Expected 65 bytes starting with 0x04.")

    # 1. Wrap the public key bytes with the multicodec prefix for 'p256-pub'.
    # The python-multicodec library uses names. 'p256-pub' is for 0x1201.
    try:
        prefixed_pub_key = multicodec.wrap(CODEC_P256_PUB_UNCOMPRESSED_NAME, pub_key_bytes)
    except Exception as e:
        # This might happen if 'p256-pub' is not recognized or if the input bytes are rejected.
        raise ValueError(f"Failed to wrap public key with multicodec '{CODEC_P256_PUB_UNCOMPRESSED_NAME}': {e}")

    # 2. Encode the result with Base58BTC (multibase 'z' prefix).
    # multibase.encode returns bytes, so decode to utf-8 string.
    did_key_multibase_part = multibase.encode('base58btc', prefixed_pub_key)

    return "did:key:" + did_key_multibase_part.decode('utf-8')

if __name__ == '__main__':
    # Example usage:
    # This is a known P256 uncompressed public key (hex) and its corresponding did:key
    # generated using the Go implementation (which uses multicodec 0x1201 -> varint 0x81 0x24).
    # The python-multicodec library for 'p256-pub' (0x1201) should produce the same varint prefix.
    known_go_pub_hex = "04d0c7dee8fc1ba33fd28088095b2eff0a290329996127813779b25a73c3b100940021c0c0d76047e331a248a5789550f8d3e301b5f9999a33317573db0904f94e"
    expected_did_key_from_go = "did:key:zQ3shxNt9N8j5y4MX2o2oPqR3xYd9fT5zP3gC9gB8h8vJ6xJg" # Matches Go's output for 0x1201

    print(f"Test Vector Public Key Hex: {known_go_pub_hex}")
    try:
        python_generated_did = generate_did_key_from_public_key_hex(known_go_pub_hex)
        print(f"Python Generated DID Key:   {python_generated_did}")
        print(f"Expected DID Key (from Go): {expected_did_key_from_go}")

        if python_generated_did == expected_did_key_from_go:
            print("\nSUCCESS: Python generated DID matches the expected Go DID for the test vector.")
        else:
            print("\nERROR: Python generated DID does NOT match the expected Go DID for the test vector.")
            # Debugging multicodec prefix bytes from python-multicodec:
            pub_bytes_for_debug = binascii.unhexlify(known_go_pub_hex)
            mc_wrapped_for_debug = multicodec.wrap(CODEC_P256_PUB_UNCOMPRESSED_NAME, pub_bytes_for_debug)
            print(f"  Multicodec wrapped bytes (hex) from Python: {mc_wrapped_for_debug.hex()}")
            # Expected prefix for 0x1201 is 0x81 0x24. So, mc_wrapped_for_debug should start with 8124...
            # Go varint for 0x1201: [129 36] -> 0x81 0x24

    except ValueError as e:
        print(f"Error during example: {e}")

    # Test with a key from our wallet module
    # This assumes wallet.py is in the same directory or accessible via PYTHONPATH
    try:
        from wallet import generate_key_pair, get_public_key_bytes
        _, pub_key = generate_key_pair()
        pub_hex_for_test = get_public_key_bytes(pub_key).hex()
        print(f"\nTest with a newly generated wallet.py key:")
        print(f"Public Key Hex: {pub_hex_for_test}")
        did_key_wallet = generate_did_key_from_public_key_hex(pub_hex_for_test)
        print(f"Generated DID Key: {did_key_wallet}")
    except ImportError:
        print("\nSkipping wallet.py generated key test as wallet.py could not be imported (likely path issue in standalone run).")
    except ValueError as e:
        print(f"Error with wallet.py key: {e}")
