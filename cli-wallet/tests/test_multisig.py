import unittest
import os
import json
import hashlib

import sys
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '..')))

from wallet import generate_key_pair, save_private_key, load_private_key, public_key_to_address, get_public_key_bytes
from multisig import derive_multisig_address, create_multisig_config, load_multisig_config
from transaction import Transaction, SignerInfo, TX_STANDARD # Assuming Transaction class is updated

class TestMultiSig(unittest.TestCase):

    def setUp(self):
        self.wallet_files = []
        self.passwords = []
        self.pub_keys_hex = []
        self.num_keys = 3
        for i in range(self.num_keys):
            priv, pub = generate_key_pair()
            fname = f"tmp_ms_test_signer_{i+1}.pem"
            pwd = f"ms_pass{i+1}"
            save_private_key(priv, fname, pwd)
            self.wallet_files.append(fname)
            self.passwords.append(pwd)
            self.pub_keys_hex.append(public_key_to_address(pub)) # Uses the 0x04+X+Y hex format

        self.config_file = "tmp_ms_test_config.json"
        self.pending_tx_file = "tmp_ms_pending_tx.json"

    def tearDown(self):
        for fname in self.wallet_files:
            if os.path.exists(fname):
                os.remove(fname)
        if os.path.exists(self.config_file):
            os.remove(self.config_file)
        if os.path.exists(self.pending_tx_file):
            os.remove(self.pending_tx_file)

    def test_derive_multisig_address(self):
        m = 2
        # Ensure keys are sorted for derivation if manual
        sorted_keys = sorted(self.pub_keys_hex)
        addr1 = derive_multisig_address(m, sorted_keys)
        self.assertIsNotNone(addr1)
        self.assertEqual(len(addr1), 64) # SHA256 hex digest

        # Test idempotency (same keys, same M = same address)
        addr2 = derive_multisig_address(m, sorted_keys)
        self.assertEqual(addr1, addr2)

        # Test different M yields different address
        addr_m3 = derive_multisig_address(3, sorted_keys)
        self.assertNotEqual(addr1, addr_m3)

        # Test different key set yields different address
        _, temp_pub = generate_key_pair()
        temp_pub_hex = public_key_to_address(temp_pub)
        addr_diff_keys = derive_multisig_address(m, sorted(self.pub_keys_hex[:2] + [temp_pub_hex]))
        self.assertNotEqual(addr1, addr_diff_keys)

        # Test that input order of pubkeys doesn't matter due to internal sorting
        addr_unsorted_input = derive_multisig_address(m, [self.pub_keys_hex[1], self.pub_keys_hex[0], self.pub_keys_hex[2]])
        self.assertEqual(addr1, addr_unsorted_input)

        with self.assertRaises(ValueError, msg="M must be between 1 and N"):
            derive_multisig_address(0, self.pub_keys_hex)
        with self.assertRaises(ValueError, msg="M must be between 1 and N"):
            derive_multisig_address(4, self.pub_keys_hex) # M > N
        with self.assertRaises(ValueError, msg="Duplicate public keys"):
            derive_multisig_address(2, [self.pub_keys_hex[0], self.pub_keys_hex[0], self.pub_keys_hex[1]])


    def test_create_and_load_multisig_config(self):
        m_val = 2
        create_multisig_config(self.config_file, m_val, self.wallet_files, self.passwords)
        self.assertTrue(os.path.exists(self.config_file))

        config = load_multisig_config(self.config_file)
        self.assertEqual(config["m_required"], m_val)
        self.assertEqual(config["n_total_keys"], self.num_keys)
        self.assertEqual(len(config["authorized_public_keys_hex"]), self.num_keys)
        # Check if keys in config are sorted (create_multisig_config sorts them)
        self.assertEqual(config["authorized_public_keys_hex"], sorted(self.pub_keys_hex))

        expected_addr = derive_multisig_address(m_val, sorted(self.pub_keys_hex))
        self.assertEqual(config["multisig_address_hex"], expected_addr)

    def test_multisig_transaction_workflow(self):
        # 1. Create Config
        m_val = 2
        create_multisig_config(self.config_file, m_val, self.wallet_files, self.passwords)
        config = load_multisig_config(self.config_file)

        # 2. Initiate Transaction
        recipient_priv, recipient_pub = generate_key_pair()
        recipient_addr_hex = public_key_to_address(recipient_pub)

        tx_to_init = Transaction(
            from_address_hex=config["multisig_address_hex"],
            to_address_hex=recipient_addr_hex,
            amount=500,
            fee=5,
            tx_type=TX_STANDARD,
            required_signatures=config["m_required"],
            authorized_public_keys_hex=config["authorized_public_keys_hex"]
        )
        # ID should be set based on initial content + M/N config
        tx_to_init.id_hex = tx_to_init.calculate_hash()

        with open(self.pending_tx_file, 'w') as f:
            json.dump(tx_to_init.to_dict_for_file(), f, indent=2)

        self.assertTrue(os.path.exists(self.pending_tx_file))

        # 3. Load pending tx and add first signature
        with open(self.pending_tx_file, 'r') as f:
            tx_data_loaded1 = json.load(f)
        tx_signing1 = Transaction.from_dict_for_file(tx_data_loaded1)

        # Sign with first wallet
        tx_signing1.add_signature(self.wallet_files[0], self.passwords[0])
        self.assertEqual(len(tx_signing1.signers), 1)
        self.assertEqual(tx_signing1.signers[0].public_key_hex, self.pub_keys_hex[0]) # Assuming wallet_files[0] corresponds to pub_keys_hex[0] if sorted

        # Verify (Python side) - should be false as M=2
        self.assertFalse(tx_signing1.verify_signatures_python())

        # Save after first signature
        with open(self.pending_tx_file, 'w') as f:
            json.dump(tx_signing1.to_dict_for_file(), f, indent=2)

        # 4. Load pending tx and add second signature
        with open(self.pending_tx_file, 'r') as f:
            tx_data_loaded2 = json.load(f)
        tx_signing2 = Transaction.from_dict_for_file(tx_data_loaded2)

        # Sign with second wallet
        tx_signing2.add_signature(self.wallet_files[1], self.passwords[1])
        self.assertEqual(len(tx_signing2.signers), 2)

        # Verify (Python side) - should be true now
        self.assertTrue(tx_signing2.verify_signatures_python())

        # Check that ID (hash of core content) hasn't changed
        self.assertEqual(tx_signing2.id_hex, tx_to_init.id_hex)

        # Check that adding same signature again doesn't increase count
        tx_signing2.add_signature(self.wallet_files[1], self.passwords[1])
        self.assertEqual(len(tx_signing2.signers), 2)

        # Check that signing with unauthorized key fails
        unauth_wallet_file = "tmp_unauth_ms_wallet.pem"
        unauth_priv, _ = generate_key_pair()
        save_private_key(unauth_priv, unauth_wallet_file, "unauth_pass")
        with self.assertRaises(ValueError):
            tx_signing2.add_signature(unauth_wallet_file, "unauth_pass")
        if os.path.exists(unauth_wallet_file):
            os.remove(unauth_wallet_file)

if __name__ == '__main__':
    unittest.main()
