import unittest
import time
import hashlib
import json
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.hazmat.primitives import hashes as crypto_hashes
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

import os
import sys
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from transaction import Transaction
from wallet import generate_key_pair, get_public_key_bytes, save_private_key, CURVE # CURVE needed for verify

# Dummy key data for consistent testing where needed
# In a real test suite, might load these from fixed test files or generate once
TEST_SENDER_PRIV_KEY, TEST_SENDER_PUB_KEY = generate_key_pair()
TEST_SENDER_PUB_KEY_BYTES = get_public_key_bytes(TEST_SENDER_PUB_KEY)

TEST_RECIPIENT_PRIV_KEY, TEST_RECIPIENT_PUB_KEY = generate_key_pair()
TEST_RECIPIENT_PUB_KEY_BYTES = get_public_key_bytes(TEST_RECIPIENT_PUB_KEY)

TEST_WALLET_FILE = "tmp_test_tx_wallet.pem"
TEST_PASSWORD = "tx_test_password"


class TestTransaction(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        # Save a temporary wallet file for signing tests
        save_private_key(TEST_SENDER_PRIV_KEY, TEST_WALLET_FILE, TEST_PASSWORD)

    @classmethod
    def tearDownClass(cls):
        if os.path.exists(TEST_WALLET_FILE):
            os.remove(TEST_WALLET_FILE)

    def test_transaction_creation(self):
        timestamp = int(time.time() * 1_000_000_000)
        tx = Transaction(
            from_address_bytes=TEST_SENDER_PUB_KEY_BYTES,
            to_address_bytes=TEST_RECIPIENT_PUB_KEY_BYTES,
            amount=100,
            fee=10,
            timestamp=timestamp,
            public_key_bytes=TEST_SENDER_PUB_KEY_BYTES
        )
        self.assertIsNone(tx.id) # ID not set until signing
        self.assertEqual(tx.timestamp, timestamp)
        self.assertEqual(tx.from_address, TEST_SENDER_PUB_KEY_BYTES.hex())
        self.assertEqual(tx.to_address, TEST_RECIPIENT_PUB_KEY_BYTES.hex())
        self.assertEqual(tx.amount, 100)
        self.assertEqual(tx.fee, 10)
        self.assertEqual(tx.public_key, TEST_SENDER_PUB_KEY_BYTES.hex())
        self.assertIsNone(tx.signature)

    def test_data_for_hashing_format(self):
        fixed_timestamp = 1678886400_000000000 # Example: 2023-03-15 12:00:00 UTC in ns
        tx = Transaction(
            from_address_bytes=TEST_SENDER_PUB_KEY_BYTES,
            to_address_bytes=TEST_RECIPIENT_PUB_KEY_BYTES,
            amount=50,
            fee=5,
            timestamp=fixed_timestamp,
            public_key_bytes=TEST_SENDER_PUB_KEY_BYTES
        )

        # Expected JSON structure based on Python's transaction.py data_for_hashing()
        # Keys must be sorted alphabetically for canonical JSON
        expected_payload_dict = {
            "Amount": 50,
            "Fee": 5,
            "From": TEST_SENDER_PUB_KEY_BYTES.hex(),
            "PublicKey": TEST_SENDER_PUB_KEY_BYTES.hex(),
            "Timestamp": fixed_timestamp,
            "To": TEST_RECIPIENT_PUB_KEY_BYTES.hex(),
        }
        expected_json_string = json.dumps(expected_payload_dict, sort_keys=True, separators=(',', ':'))

        data_bytes = tx.data_for_hashing()
        self.assertEqual(data_bytes.decode('utf-8'), expected_json_string)

    def test_calculate_hash(self):
        tx = Transaction(
            from_address_bytes=TEST_SENDER_PUB_KEY_BYTES,
            to_address_bytes=TEST_RECIPIENT_PUB_KEY_BYTES,
            amount=200,
            fee=20,
            public_key_bytes=TEST_SENDER_PUB_KEY_BYTES
        )
        tx.timestamp = 12345 # Fixed timestamp for consistent hash

        data_to_hash = tx.data_for_hashing()
        expected_hash_obj = hashlib.sha256()
        expected_hash_obj.update(data_to_hash)
        expected_hash_hex = expected_hash_obj.hexdigest()

        calculated_hash_hex = tx.calculate_hash()
        self.assertEqual(calculated_hash_hex, expected_hash_hex)
        self.assertEqual(len(calculated_hash_hex), 64) # SHA256 hex string length

    def test_sign_transaction(self):
        tx = Transaction(
            from_address_bytes=TEST_SENDER_PUB_KEY_BYTES, # This will be overwritten by signer's pubkey
            to_address_bytes=TEST_RECIPIENT_PUB_KEY_BYTES,
            amount=1000,
            fee=100
        )

        # Sign the transaction using the setUpClass wallet file
        tx.sign(TEST_WALLET_FILE, TEST_PASSWORD)

        self.assertIsNotNone(tx.id)
        self.assertIsNotNone(tx.signature)
        self.assertIsNotNone(tx.public_key)

        # Verify that from_address and public_key are updated to the signer's
        self.assertEqual(tx.from_address, TEST_SENDER_PUB_KEY_BYTES.hex())
        self.assertEqual(tx.public_key, TEST_SENDER_PUB_KEY_BYTES.hex())

        # ID should be the hex digest of the hash of the content
        content_hash_hex = tx.calculate_hash() # Recalculate to ensure it's based on current (post-pubkey update) state
        self.assertEqual(tx.id, content_hash_hex)

        # Signature verification (Python side)
        self.assertTrue(tx.verify_signature_python())

    def test_sign_transaction_auto_sets_public_key(self):
        # Create transaction without initially setting public_key_bytes
        tx = Transaction(
            from_address_bytes=TEST_SENDER_PUB_KEY_BYTES, # This will be overwritten
            to_address_bytes=TEST_RECIPIENT_PUB_KEY_BYTES,
            amount=500
        )
        self.assertIsNone(tx.public_key) # Should be None initially

        tx.sign(TEST_WALLET_FILE, TEST_PASSWORD)

        self.assertIsNotNone(tx.public_key)
        self.assertEqual(tx.public_key, TEST_SENDER_PUB_KEY_BYTES.hex())
        self.assertTrue(tx.verify_signature_python())

    def test_verify_tampered_transaction(self):
        tx = Transaction(
            from_address_bytes=TEST_SENDER_PUB_KEY_BYTES,
            to_address_bytes=TEST_RECIPIENT_PUB_KEY_BYTES,
            amount=100,
            public_key_bytes=TEST_SENDER_PUB_KEY_BYTES
        )
        tx.sign(TEST_WALLET_FILE, TEST_PASSWORD)
        self.assertTrue(tx.verify_signature_python()) # Original should be valid

        # Tamper amount AFTER signing
        tx.amount = 200
        self.assertFalse(tx.verify_signature_python())

        # Restore amount, tamper 'to_address'
        tx.amount = 100
        tx.to_address = TEST_SENDER_PUB_KEY_BYTES.hex() # Send to self (different from original)
        self.assertFalse(tx.verify_signature_python())


if __name__ == '__main__':
    unittest.main()
