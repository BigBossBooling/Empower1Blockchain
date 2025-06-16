import os
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.backends import default_backend

CURVE = ec.SECP256R1() # Equivalent to Go's P256

def generate_key_pair():
    """Generates a new ECDSA private/public key pair."""
    private_key = ec.generate_private_key(CURVE, default_backend())
    public_key = private_key.public_key()
    return private_key, public_key

def serialize_private_key_pem(private_key, password=None):
    """Serializes a private key to PEM format, optionally encrypted."""
    encryption_algorithm = serialization.NoEncryption()
    if password:
        encryption_algorithm = serialization.BestAvailableEncryption(password.encode('utf-8'))

    pem = private_key.private_bytes(
        encoding=serialization.Encoding.PEM,
        format=serialization.PrivateFormat.PKCS8,
        encryption_algorithm=encryption_algorithm
    )
    return pem

def deserialize_private_key_pem(pem_data, password=None):
    """Deserializes a private key from PEM format, optionally decrypted."""
    try:
        private_key = serialization.load_pem_private_key(
            pem_data,
            password=password.encode('utf-8') if password else None,
            backend=default_backend()
        )
        if not isinstance(private_key, ec.EllipticCurvePrivateKey):
            raise TypeError("Key is not an Elliptic Curve private key.")
        return private_key
    except Exception as e:
        raise ValueError(f"Failed to load private key: {e}")


def save_private_key(private_key, filepath, password=None):
    """Saves a private key to a file in PEM format."""
    pem_data = serialize_private_key_pem(private_key, password)
    with open(filepath, 'wb') as f:
        f.write(pem_data)
    os.chmod(filepath, 0o600) # Read/Write for owner only
    print(f"Wallet saved to {filepath}")

def load_private_key(filepath, password=None):
    """Loads a private key from a PEM file."""
    if not os.path.exists(filepath):
        raise FileNotFoundError(f"Wallet file not found: {filepath}")
    with open(filepath, 'rb') as f:
        pem_data = f.read()
    return deserialize_private_key_pem(pem_data, password)

def get_public_key_bytes(public_key):
    """
    Serializes the public key to uncompressed bytes.
    This format (0x04 + X + Y) is common for P256/SECP256R1.
    """
    return public_key.public_bytes(
        encoding=serialization.Encoding.X962, # Uncompressed format
        format=serialization.PublicFormat.UncompressedPoint # Prepend 0x04
    )

def public_key_to_address(public_key):
    """
    Derives an address from the public key (hex string of uncompressed public key bytes).
    This should match the Go node's address scheme.
    """
    pub_key_bytes = get_public_key_bytes(public_key)
    return pub_key_bytes.hex()

if __name__ == '__main__':
    # Simple test
    priv, pub = generate_key_pair()
    print(f"Generated Private Key: [object]")
    print(f"Generated Public Key: [object]")

    address = public_key_to_address(pub)
    print(f"Address: {address}")
    print(f"Address length: {len(address)}") # Should be 130 (04 + 64 for X + 64 for Y)

    # Test save and load
    test_password = "testpassword123"
    save_private_key(priv, "test_wallet.pem", password=test_password)
    loaded_priv = load_private_key("test_wallet.pem", password=test_password)

    loaded_pub = loaded_priv.public_key()
    loaded_address = public_key_to_address(loaded_pub)
    print(f"Loaded Address: {loaded_address}")

    assert address == loaded_address
    print("Wallet save/load test successful.")
    os.remove("test_wallet.pem")

    # Test save without password
    save_private_key(priv, "test_wallet_no_pass.pem")
    loaded_priv_no_pass = load_private_key("test_wallet_no_pass.pem")
    assert public_key_to_address(loaded_priv_no_pass.public_key()) == address
    print("Wallet save/load (no password) test successful.")
    os.remove("test_wallet_no_pass.pem")
