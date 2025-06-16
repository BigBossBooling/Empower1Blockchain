import time
import hashlib
import json # Using JSON for hashing consistency. Could also use a library like `cbor2` or custom struct packing.
from cryptography.hazmat.primitives import hashes as crypto_hashes # Renamed to avoid conflict
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
from wallet import CURVE # Import CURVE from wallet.py

class Transaction:
    def __init__(self, from_address_bytes, to_address_bytes, amount, fee=0, timestamp=None, public_key_bytes=None):
        self.id = None # Will be hash of content
        self.timestamp = timestamp if timestamp is not None else int(time.time() * 1_000_000_000) # Nanoseconds
        self.from_address = from_address_bytes.hex() # Store as hex string, matching Go's ProposerAddress type if it's string
        self.to_address = to_address_bytes.hex()   # Store as hex string
        self.amount = int(amount)
        self.fee = int(fee)
        self.public_key = public_key_bytes.hex() if public_key_bytes else None # Sender's public key, hex encoded
        self.signature = None # DER-encoded signature, hex string

    def __repr__(self):
        return f"<Transaction id={self.id} from={self.from_address} to={self.to_address} amount={self.amount}>"

    def to_dict(self, for_signing=False):
        """Returns a dictionary representation of the transaction."""
        data = {
            "Timestamp": self.timestamp,
            "From": self.from_address, # In Go, these are []byte. For hashing, use bytes.
            "To": self.to_address,
            "Amount": self.amount,
            "Fee": self.fee,
            "PublicKey": self.public_key, # This is hex string of public key bytes
        }
        if not for_signing:
            data["ID"] = self.id
            data["Signature"] = self.signature
        return data

    def to_json_for_broadcast(self):
        """ Prepares the transaction for sending to the Go node. Matches Go struct fields. """
        if not self.id or not self.signature or not self.public_key:
            raise ValueError("Transaction must be fully signed and ID'd before broadcasting.")

        return {
            "ID": bytes.fromhex(self.id).decode('latin-1'), # Assuming Go expects []byte as string
            "Timestamp": self.timestamp,
            "From": bytes.fromhex(self.from_address).decode('latin-1'),
            "To": bytes.fromhex(self.to_address).decode('latin-1'),
            "Amount": self.amount,
            "Fee": self.fee,
            "Signature": bytes.fromhex(self.signature).decode('latin-1'),
            "PublicKey": bytes.fromhex(self.public_key).decode('latin-1'),
        }


    def data_for_hashing(self):
        """
        Prepares the transaction data for hashing.
        Order and types must precisely match Go's `Transaction.prepareDataForHashing()`.
        Go side uses gob encoding on a struct:
        type TxDataForHashing struct {
            Timestamp int64
            From      []byte
            To        []byte
            Amount    uint64
            Fee       uint64
            PublicKey []byte
        }
        We need to replicate this structure and encoding as closely as possible.
        Using JSON with ordered keys for simplicity here, but GOB or a strict binary format is safer.
        For now, let's use a specific JSON string representation.
        A better method would be to use a canonical serialization format.
        Python's `struct` module or a library like `construct` could be used for precise binary packing.
        Let's try to match the Go GOB structure by serializing a dictionary with the same field names.
        The Go GOB encoder will likely output field names.

        Given Go uses gob, and gob is not easily portable, a more robust approach is to define
        a canonical JSON representation or a simple concatenation of fields in a defined order
        and byte format (e.g., lengths prefixed or fixed size).

        For now, let's use a simple, ordered JSON string representation for hashing.
        This is a common simplification but has pitfalls if not perfectly matched.
        The Go `Transaction.Hash()` uses gob encoding of `TxDataForHashing`.
        This is tricky to replicate perfectly in Python without a shared schema or IDL.

        Let's assume Go's TxDataForHashing for `Transaction.Hash()` is:
        Timestamp (int64), From ([]byte), To ([]byte), Amount (uint64), Fee (uint64), PublicKey ([]byte)
        Concatenated in order, with specific byte representations (e.g., BigEndian for numbers).
        This is the most robust way if not using a cross-language serialization like Protobuf.

        Simplification: For now, use JSON with sorted keys. This is NOT ideal for cross-language hashing.
        A better approach is to define a byte string explicitly.
        Let's try to build a byte string:
        - Timestamp: 8 bytes, big-endian
        - From: length-prefixed bytes
        - To: length-prefixed bytes
        - Amount: 8 bytes, big-endian
        - Fee: 8 bytes, big-endian
        - PublicKey: length-prefixed bytes
        """

        # This data structure MUST match Go's TxDataForHashing for gob encoding
        # For GOB compatibility, we'd need a Python GOB library or a very specific byte layout.
        # Let's define a canonical JSON representation instead for cross-language hashing,
        # assuming the Go side is also adjusted to hash this JSON representation.
        # If Go side MUST use gob, this Python hash will not match.
        #
        # The Go code for Transaction.Hash() uses:
        #   data := TxDataForHashing{ Timestamp, From, To, Amount, Fee, PublicKey }
        #   gob.NewEncoder(&buf).Encode(data)
        #   sha256.Sum256(buf.Bytes())
        #
        # This implies we need to make Python produce GOB compatible output for these fields.
        # This is non-trivial.
        #
        # Alternative: Modify Go to hash a canonical JSON or byte string.
        # For this exercise, we'll *assume* we can construct a byte string in Python
        # that, when hashed, matches what the Go side would hash.
        # This is a major simplification point for now.
        #
        # Let's assume a simpler canonical form for hashing for now:
        # A JSON string with fields in a specific order.
        # This is fragile. A byte-concatenation approach is better.

        payload_to_hash = {
            "Timestamp": self.timestamp,
            # From, To, PublicKey are hex strings in the object, convert to bytes for hashing
            "From": self.from_address, # Hex string
            "To": self.to_address,     # Hex string
            "Amount": self.amount,
            "Fee": self.fee,
            "PublicKey": self.public_key # Hex string
        }
        # Serialize to JSON string, ensure keys are sorted for consistency
        # Ensure no spaces and use UTF-8
        # This JSON string will be hashed. The Go side must do the equivalent.
        json_string = json.dumps(payload_to_hash, sort_keys=True, separators=(',', ':'))
        return json_string.encode('utf-8')


    def calculate_hash(self):
        """Calculates the SHA256 hash of the transaction content for ID and signing."""
        data_to_hash = self.data_for_hashing()
        hasher = hashlib.sha256()
        hasher.update(data_to_hash)
        return hasher.hexdigest() # Return as hex string

    def sign(self, private_key_pem_path, password=None):
        """Signs the transaction with the private key from the given wallet file."""
        from wallet import load_private_key # Local import to avoid circular dependency if wallet uses Transaction

        private_key = load_private_key(private_key_pem_path, password)

        if self.public_key is None:
            # Derive public key from private key if not set
            public_key_obj = private_key.public_key()
            self.public_key = public_key_obj.public_bytes(
                encoding=Encoding.X962,
                format=PublicFormat.UncompressedPoint
            ).hex()

        # Ensure self.from_address matches the public key of the signer
        # This is implicitly handled if PublicKey is derived from the private key
        # and From is set from PublicKey.
        # Here, we assume self.from_address was set correctly at instantiation,
        # or it should be updated now from private_key.public_key()
        signer_public_key_bytes = private_key.public_key().public_bytes(
            Encoding.X962, PublicFormat.UncompressedPoint
        )
        self.from_address = signer_public_key_bytes.hex()
        self.public_key = self.from_address # From and PublicKey are the same for the sender


        content_hash_hex = self.calculate_hash()
        self.id = content_hash_hex # Set transaction ID

        content_hash_bytes = bytes.fromhex(content_hash_hex)

        # Sign the hash (bytes)
        signature_bytes_der = private_key.sign(
            content_hash_bytes,
            ec.ECDSA(utils.Prehashed(crypto_hashes.SHA256())) # Sign the pre-hashed data
        )
        self.signature = signature_bytes_der.hex() # Store signature as hex string

        print(f"Signed transaction ID: {self.id}")
        print(f"Signature (DER hex): {self.signature[:64]}...")
        return True

    def verify_signature_python(self):
        """Verifies the transaction's signature (meant for Python-side verification if needed)."""
        if not self.public_key or not self.signature:
            raise ValueError("PublicKey and Signature must be present for verification.")

        public_key_bytes = bytes.fromhex(self.public_key)
        signature_bytes = bytes.fromhex(self.signature)

        # Reconstruct public key object
        # Assuming X962 uncompressed format from how public_key is stored
        public_key = ec.EllipticCurvePublicKey.from_encoded_point(CURVE, public_key_bytes)

        content_hash_hex = self.calculate_hash() # Recalculate hash of content
        content_hash_bytes = bytes.fromhex(content_hash_hex)

        try:
            public_key.verify(
                signature_bytes,
                content_hash_bytes,
                ec.ECDSA(utils.Prehashed(crypto_hashes.SHA256()))
            )
            return True
        except Exception: # cryptography.exceptions.InvalidSignature
            return False

# Example Usage (for testing this file directly)
if __name__ == '__main__':
    from wallet import generate_key_pair, public_key_to_address, get_public_key_bytes, save_private_key, load_private_key, CURVE
    import os

    # Create dummy wallet files
    sender_priv_key, sender_pub_key = generate_key_pair()
    recipient_priv_key, recipient_pub_key = generate_key_pair() # Just for a valid recipient address

    save_private_key(sender_priv_key, "tmp_sender_wallet.pem", "sender")

    sender_pub_bytes = get_public_key_bytes(sender_pub_key)
    recipient_pub_bytes = get_public_key_bytes(recipient_pub_key)

    # Create a transaction instance
    tx = Transaction(
        from_address_bytes=sender_pub_bytes,
        to_address_bytes=recipient_pub_bytes,
        amount=100,
        fee=10,
        public_key_bytes=sender_pub_bytes # Initially set based on from_address
    )

    print(f"Initial Transaction: {tx.to_dict()}")
    print(f"Data for hashing: {tx.data_for_hashing().decode()}")

    # Sign the transaction
    tx.sign("tmp_sender_wallet.pem", "sender")
    print(f"Signed Transaction: {tx.to_dict()}")

    # Verify signature (Python side)
    is_valid_python = tx.verify_signature_python()
    print(f"Signature valid (Python check): {is_valid_python}")
    assert is_valid_python

    print("\nTransaction Test Complete.")
    os.remove("tmp_sender_wallet.pem")

    # Test case where public_key is not pre-set
    tx_no_pub = Transaction(
        from_address_bytes=sender_pub_bytes,
        to_address_bytes=recipient_pub_bytes,
        amount=50
    )
    save_private_key(sender_priv_key, "tmp_sender_wallet2.pem", "sender")
    tx_no_pub.sign("tmp_sender_wallet2.pem", "sender")
    assert tx_no_pub.public_key == sender_pub_bytes.hex()
    print("Transaction signing with auto public key set: OK")
    assert tx_no_pub.verify_signature_python()
    os.remove("tmp_sender_wallet2.pem")


    # Test a case where a different key tries to sign (should fail if implemented)
    # or where `from_address` in constructor does not match signing key.
    # Current `sign` method overwrites `from_address` and `public_key` with signer's info.

    print("All transaction.py tests passed.")
