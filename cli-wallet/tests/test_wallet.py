import unittest
import os
import tempfile
from cryptography.hazmat.primitives import serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.hazmat.primitives.hashes import SHA256
from cryptography.exceptions import InvalidSignature

# Assuming wallet.py is in the parent directory relative to tests/
# Adjust the path if your structure is different or if using a proper package structure
import sys
sys.path.insert(0, os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

import wallet

class TestWallet(unittest.TestCase):

    def setUp(self):
        # Create a temporary directory for wallet files
        self.test_dir = tempfile.TemporaryDirectory()
        self.wallet_file_path = os.path.join(self.test_dir.name, "test_wallet.pem")
        self.password = "testpassword123"

    def tearDown(self):
        # Clean up the temporary directory
        self.test_dir.cleanup()

    def test_generate_key_pair(self):
        priv_key, pub_key = wallet.generate_key_pair()
        self.assertIsNotNone(priv_key)
        self.assertIsInstance(priv_key, ec.EllipticCurvePrivateKey)
        self.assertIsNotNone(pub_key)
        self.assertIsInstance(pub_key, ec.EllipticCurvePublicKey)
        self.assertEqual(priv_key.curve.name, wallet.CURVE.name)
        self.assertEqual(pub_key.curve.name, wallet.CURVE.name)

    def test_serialize_deserialize_private_key_no_password(self):
        priv_key, _ = wallet.generate_key_pair()
        pem_data = wallet.serialize_private_key_pem(priv_key, None)
        self.assertIsNotNone(pem_data)

        deserialized_priv_key = wallet.deserialize_private_key_pem(pem_data, None)
        self.assertIsNotNone(deserialized_priv_key)
        self.assertEqual(
            priv_key.private_numbers().private_value,
            deserialized_priv_key.private_numbers().private_value
        )

    def test_serialize_deserialize_private_key_with_password(self):
        priv_key, _ = wallet.generate_key_pair()
        pem_data = wallet.serialize_private_key_pem(priv_key, self.password)
        self.assertIsNotNone(pem_data)

        deserialized_priv_key = wallet.deserialize_private_key_pem(pem_data, self.password)
        self.assertIsNotNone(deserialized_priv_key)
        self.assertEqual(
            priv_key.private_numbers().private_value,
            deserialized_priv_key.private_numbers().private_value
        )

        with self.assertRaisesRegex(ValueError, "Failed to load private key"): # Python's cryptography typically raises ValueError on bad decrypt
            wallet.deserialize_private_key_pem(pem_data, "wrongpassword")

    def test_save_load_private_key_no_password(self):
        priv_key_orig, pub_key_orig = wallet.generate_key_pair()
        wallet.save_private_key(priv_key_orig, self.wallet_file_path, None)

        loaded_priv_key = wallet.load_private_key(self.wallet_file_path, None)
        self.assertIsNotNone(loaded_priv_key)
        self.assertEqual(
            priv_key_orig.private_numbers().private_value,
            loaded_priv_key.private_numbers().private_value
        )
        self.assertEqual(
            wallet.public_key_to_address(pub_key_orig),
            wallet.public_key_to_address(loaded_priv_key.public_key())
        )

    def test_save_load_private_key_with_password(self):
        priv_key_orig, pub_key_orig = wallet.generate_key_pair()
        wallet.save_private_key(priv_key_orig, self.wallet_file_path, self.password)

        loaded_priv_key = wallet.load_private_key(self.wallet_file_path, self.password)
        self.assertIsNotNone(loaded_priv_key)
        self.assertEqual(
            priv_key_orig.private_numbers().private_value,
            loaded_priv_key.private_numbers().private_value
        )
        self.assertEqual(
            wallet.public_key_to_address(pub_key_orig),
            wallet.public_key_to_address(loaded_priv_key.public_key())
        )

        with self.assertRaisesRegex(ValueError, "Failed to load private key"): # Python's cryptography typically raises ValueError on bad decrypt
            wallet.load_private_key(self.wallet_file_path, "wrongpassword")

    def test_get_public_key_bytes_and_address_derivation(self):
        _, pub_key = wallet.generate_key_pair()

        pub_key_bytes = wallet.get_public_key_bytes(pub_key)
        self.assertIsNotNone(pub_key_bytes)
        self.assertEqual(len(pub_key_bytes), 65) # Uncompressed P256: 0x04 + 32B X + 32B Y
        self.assertEqual(pub_key_bytes[0], 0x04)

        address = wallet.public_key_to_address(pub_key)
        self.assertIsNotNone(address)
        self.assertEqual(len(address), 130) # Hex representation of 65 bytes

        # Ensure address is valid hex
        try:
            bytes.fromhex(address)
        except ValueError:
            self.fail("Address is not a valid hex string")

    def test_load_non_existent_wallet(self):
        with self.assertRaises(FileNotFoundError):
            wallet.load_private_key("non_existent_wallet.pem")

if __name__ == '__main__':
    unittest.main()
