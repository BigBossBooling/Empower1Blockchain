import unittest
import os
from cryptography.hazmat.primitives.asymmetric import ec

# Add cli-wallet directory to Python path to allow imports from wallet module
import sys
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from wallet import (
    generate_key_pair,
    serialize_private_key_pem,
    deserialize_private_key_pem,
    save_private_key,
    load_private_key,
    get_public_key_bytes,
    public_key_to_address,
    CURVE
)

class TestWallet(unittest.TestCase):

    def setUp(self):
        self.test_wallet_file = "test_wallet_for_unittest.pem"
        self.test_password = "unittestpassword"

    def tearDown(self):
        if os.path.exists(self.test_wallet_file):
            os.remove(self.test_wallet_file)

    def test_generate_key_pair(self):
        priv_key, pub_key = generate_key_pair()
        self.assertIsNotNone(priv_key)
        self.assertIsNotNone(pub_key)
        self.assertIsInstance(priv_key, ec.EllipticCurvePrivateKey)
        self.assertIsInstance(pub_key, ec.EllipticCurvePublicKey)
        self.assertEqual(priv_key.curve.name, CURVE.name)
        self.assertEqual(pub_key.curve.name, CURVE.name)

    def test_private_key_serialization_deserialization_no_password(self):
        priv_key, _ = generate_key_pair()
        pem_data = serialize_private_key_pem(priv_key)
        self.assertIsNotNone(pem_data)

        deserialized_priv_key = deserialize_private_key_pem(pem_data)
        self.assertIsNotNone(deserialized_priv_key)
        # Compare private key values (D attribute for EC keys)
        self.assertEqual(
            priv_key.private_numbers().private_value,
            deserialized_priv_key.private_numbers().private_value
        )
        self.assertEqual(
            priv_key.public_key().public_numbers().x,
            deserialized_priv_key.public_key().public_numbers().x
        )

    def test_private_key_serialization_deserialization_with_password(self):
        priv_key, _ = generate_key_pair()
        pem_data = serialize_private_key_pem(priv_key, self.test_password)
        self.assertIsNotNone(pem_data)

        deserialized_priv_key = deserialize_private_key_pem(pem_data, self.test_password)
        self.assertIsNotNone(deserialized_priv_key)
        self.assertEqual(
            priv_key.private_numbers().private_value,
            deserialized_priv_key.private_numbers().private_value
        )

        # Test with wrong password
        with self.assertRaises(ValueError): # Expecting a ValueError for decryption failure
            deserialize_private_key_pem(pem_data, "wrongpassword")

    def test_save_load_private_key_no_password(self):
        priv_key, pub_key_orig = generate_key_pair()
        save_private_key(priv_key, self.test_wallet_file)

        loaded_priv_key = load_private_key(self.test_wallet_file)
        self.assertIsNotNone(loaded_priv_key)
        self.assertEqual(
            priv_key.private_numbers().private_value,
            loaded_priv_key.private_numbers().private_value
        )
        # Check if original public key matches loaded one
        self.assertEqual(
            public_key_to_address(pub_key_orig),
            public_key_to_address(loaded_priv_key.public_key())
        )


    def test_save_load_private_key_with_password(self):
        priv_key, pub_key_orig = generate_key_pair()
        save_private_key(priv_key, self.test_wallet_file, self.test_password)

        loaded_priv_key = load_private_key(self.test_wallet_file, self.test_password)
        self.assertIsNotNone(loaded_priv_key)
        self.assertEqual(
            priv_key.private_numbers().private_value,
            loaded_priv_key.private_numbers().private_value
        )
        self.assertEqual(
            public_key_to_address(pub_key_orig),
            public_key_to_address(loaded_priv_key.public_key())
        )

        # Test loading with wrong password
        with self.assertRaises(ValueError):
            load_private_key(self.test_wallet_file, "wrongpassword")

    def test_get_public_key_bytes_and_address_derivation(self):
        _, pub_key = generate_key_pair()

        pub_key_bytes = get_public_key_bytes(pub_key)
        self.assertIsNotNone(pub_key_bytes)
        # Uncompressed P256 public key should start with 0x04 and be 1 (04) + 32 (x) + 32 (y) = 65 bytes long
        self.assertEqual(len(pub_key_bytes), 65)
        self.assertEqual(pub_key_bytes[0], 0x04)

        address = public_key_to_address(pub_key)
        self.assertIsNotNone(address)
        self.assertEqual(len(address), 130) # 65 bytes * 2 (hex)
        self.assertTrue(all(c in "0123456789abcdef" for c in address))

        # Test consistency: generate address again from same key bytes
        address_again = pub_key_bytes.hex()
        self.assertEqual(address, address_again)

    def test_load_non_existent_wallet(self):
        with self.assertRaises(FileNotFoundError):
            load_private_key("non_existent_wallet.pem")

if __name__ == '__main__':
    unittest.main()
