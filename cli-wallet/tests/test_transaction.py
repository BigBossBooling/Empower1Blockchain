import unittest
import time
import hashlib
import json
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.hazmat.primitives import hashes as crypto_hashes
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat
import base64

import os
import sys
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from transaction import Transaction, SignerInfo, TX_STANDARD, TX_CONTRACT_DEPLOY, TX_CONTRACT_CALL
from wallet import generate_key_pair, get_public_key_bytes, save_private_key, CURVE

TEST_SENDER_PRIV_KEY, TEST_SENDER_PUB_KEY = generate_key_pair()
TEST_SENDER_PUB_KEY_BYTES = get_public_key_bytes(TEST_SENDER_PUB_KEY)
TEST_SENDER_PUB_KEY_HEX = TEST_SENDER_PUB_KEY_BYTES.hex()

TEST_RECIPIENT_PRIV_KEY, TEST_RECIPIENT_PUB_KEY = generate_key_pair()
TEST_RECIPIENT_PUB_KEY_BYTES = get_public_key_bytes(TEST_RECIPIENT_PUB_KEY)
TEST_RECIPIENT_PUB_KEY_HEX = TEST_RECIPIENT_PUB_KEY_BYTES.hex()

TEST_WALLET_FILE = "tmp_test_tx_wallet.pem" # Used for single signer tests
TEST_PASSWORD = "tx_test_password"


class TestTransaction(unittest.TestCase):

    @classmethod
    def setUpClass(cls):
        save_private_key(TEST_SENDER_PRIV_KEY, TEST_WALLET_FILE, TEST_PASSWORD)

    @classmethod
    def tearDownClass(cls):
        if os.path.exists(TEST_WALLET_FILE):
            os.remove(TEST_WALLET_FILE)

    def test_transaction_creation_standard(self):
        timestamp = int(time.time() * 1_000_000_000)
        tx = Transaction(
            from_address_hex=TEST_SENDER_PUB_KEY_HEX,
            to_address_hex=TEST_RECIPIENT_PUB_KEY_HEX,
            amount=100,
            fee=10,
            timestamp=timestamp,
            public_key_hex=TEST_SENDER_PUB_KEY_HEX,
            tx_type=TX_STANDARD
        )
        self.assertIsNone(tx.id_hex)
        self.assertEqual(tx.timestamp, timestamp)
        self.assertEqual(tx.from_address_hex, TEST_SENDER_PUB_KEY_HEX)
        self.assertEqual(tx.to_address_hex, TEST_RECIPIENT_PUB_KEY_HEX)
        self.assertEqual(tx.amount, 100)
        self.assertEqual(tx.fee, 10)
        self.assertEqual(tx.public_key_hex, TEST_SENDER_PUB_KEY_HEX)
        self.assertIsNone(tx.signature_hex)
        self.assertEqual(tx.tx_type, TX_STANDARD)
        self.assertEqual(tx.required_signatures, 0)
        self.assertEqual(len(tx.authorized_public_keys_hex), 0)
        self.assertEqual(len(tx.signers), 0)


    def test_data_for_hashing_format_standard_tx(self):
        fixed_timestamp = 1678886400_000000000
        tx = Transaction(
            from_address_hex=TEST_SENDER_PUB_KEY_HEX,
            to_address_hex=TEST_RECIPIENT_PUB_KEY_HEX,
            amount=50,
            fee=5,
            timestamp=fixed_timestamp,
            public_key_hex=TEST_SENDER_PUB_KEY_HEX,
            tx_type=TX_STANDARD
        )

        expected_payload_dict = {
            "Amount": 50,
            "Fee": 5,
            "From": TEST_SENDER_PUB_KEY_HEX,
            "PublicKey": TEST_SENDER_PUB_KEY_HEX,
            "Timestamp": fixed_timestamp,
            "To": TEST_RECIPIENT_PUB_KEY_HEX,
            "TxType": TX_STANDARD,
        }
        # Fields not relevant for TX_STANDARD single-signer (like Arguments, ContractCode, multi-sig fields)
        # should be omitted by data_for_hashing due to conditional logic and `omitempty` in Go's JSON struct.
        # Python's data_for_hashing explicitly adds fields based on type.

        expected_json_string = json.dumps(expected_payload_dict, sort_keys=True, separators=(',', ':'))

        data_bytes = tx.data_for_hashing()
        self.assertEqual(data_bytes.decode('utf-8'), expected_json_string)

    def test_calculate_hash(self):
        tx = Transaction(
            from_address_hex=TEST_SENDER_PUB_KEY_HEX,
            to_address_hex=TEST_RECIPIENT_PUB_KEY_HEX,
            amount=200, fee=20, public_key_hex=TEST_SENDER_PUB_KEY_HEX, tx_type=TX_STANDARD
        )
        tx.timestamp = 12345

        data_to_hash = tx.data_for_hashing()
        expected_hash_obj = hashlib.sha256()
        expected_hash_obj.update(data_to_hash)
        expected_hash_hex = expected_hash_obj.hexdigest()

        calculated_hash_hex = tx.calculate_hash()
        self.assertEqual(calculated_hash_hex, expected_hash_hex)

    def test_sign_single_transaction(self):
        tx = Transaction(
            from_address_hex=TEST_SENDER_PUB_KEY_HEX,
            to_address_hex=TEST_RECIPIENT_PUB_KEY_HEX,
            amount=1000, fee=100, tx_type=TX_STANDARD
        )

        tx.sign_single(TEST_WALLET_FILE, TEST_PASSWORD)

        self.assertIsNotNone(tx.id_hex)
        self.assertIsNotNone(tx.signature_hex)
        self.assertIsNotNone(tx.public_key_hex)
        self.assertEqual(tx.from_address_hex, TEST_SENDER_PUB_KEY_HEX)
        self.assertEqual(tx.public_key_hex, TEST_SENDER_PUB_KEY_HEX)
        content_hash_hex = tx.calculate_hash()
        self.assertEqual(tx.id_hex, content_hash_hex)
        self.assertTrue(tx.verify_signatures_python())

    def test_sign_single_transaction_auto_sets_public_key_and_from(self):
        tx = Transaction(
            from_address_hex="dummy_from_initially_will_be_overwritten",
            to_address_hex=TEST_RECIPIENT_PUB_KEY_HEX,
            amount=500, tx_type=TX_STANDARD
        )
        self.assertIsNone(tx.public_key_hex)

        tx.sign_single(TEST_WALLET_FILE, TEST_PASSWORD)

        self.assertIsNotNone(tx.public_key_hex)
        self.assertEqual(tx.public_key_hex, TEST_SENDER_PUB_KEY_HEX)
        self.assertEqual(tx.from_address_hex, TEST_SENDER_PUB_KEY_HEX)
        self.assertTrue(tx.verify_signatures_python())

    def test_verify_tampered_single_sig_transaction(self): # Renamed for clarity
        tx = Transaction(
            from_address_hex=TEST_SENDER_PUB_KEY_HEX,
            to_address_hex=TEST_RECIPIENT_PUB_KEY_HEX,
            amount=100, public_key_hex=TEST_SENDER_PUB_KEY_HEX, tx_type=TX_STANDARD
        )
        tx.sign_single(TEST_WALLET_FILE, TEST_PASSWORD)
        self.assertTrue(tx.verify_signatures_python())

        tx.amount = 200 # Tamper
        self.assertFalse(tx.verify_signatures_python())

        tx.amount = 100 # Restore
        self.assertTrue(tx.verify_signatures_python())
        tx.to_address_hex = TEST_SENDER_PUB_KEY_HEX # Tamper again
        self.assertFalse(tx.verify_signatures_python())

if __name__ == '__main__':
    unittest.main()
