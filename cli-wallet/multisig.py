import json
import hashlib
import os

# Add parent directory to sys.path to allow imports from wallet module
import sys
sys.path.append(os.path.abspath(os.path.join(os.path.dirname(__file__), '.'))) # Appends cli-wallet
from wallet import public_key_to_address, load_private_key, get_public_key_bytes

def derive_multisig_address(m_required: int, authorized_pubkey_hex_list: list[str]) -> str:
    """
    Derives a multi-sig identifier/address.
    Method: SHA256(M_value_byte + sorted_concatenated_pubkey_bytes) -> hex_string
    """
    if not (1 <= m_required <= len(authorized_pubkey_hex_list)):
        raise ValueError("M must be between 1 and N (number of authorized keys)")
    if len(authorized_pubkey_hex_list) == 0:
        raise ValueError("Authorized public key list cannot be empty.")

    # Sort hex public keys lexicographically to ensure consistent order
    sorted_pubkey_hex_list = sorted(list(set(authorized_pubkey_hex_list))) # Ensure unique and sorted

    if len(sorted_pubkey_hex_list) != len(authorized_pubkey_hex_list):
        # This implies duplicates were provided in the input list
        raise ValueError("Duplicate public keys found in authorized list.")

    hasher = hashlib.sha256()

    # Add M value (e.g., as a single byte, assuming M is reasonably small, e.g., < 256)
    if m_required > 255: # Simple check
        raise ValueError("M value too large for single byte representation in this example.")
    hasher.update(bytes([m_required]))

    # Add sorted public keys (as bytes)
    for pubkey_hex in sorted_pubkey_hex_list:
        try:
            pubkey_bytes = bytes.fromhex(pubkey_hex)
            if not pubkey_hex.startswith('04') or len(pubkey_bytes) != 65: # Basic check for P256 uncompressed
                 raise ValueError(f"Invalid public key format or length for '{pubkey_hex}'. Expected uncompressed P256 hex.")
            hasher.update(pubkey_bytes)
        except ValueError as e:
            raise ValueError(f"Invalid hex string for public key '{pubkey_hex}': {e}")

    return hasher.hexdigest()

def create_multisig_config(filepath: str, m_required: int, authorized_wallet_files: list[str], passwords: list[str | None]):
    """
    Creates a multi-sig configuration file.
    - m_required: Number of required signatures.
    - authorized_wallet_files: List of paths to PEM wallet files of authorized signers.
    - passwords: List of passwords for the corresponding wallet files (None if not encrypted).
    """
    if len(authorized_wallet_files) != len(passwords):
        raise ValueError("Number of wallet files and passwords must match.")
    if m_required <= 0 or m_required > len(authorized_wallet_files):
        raise ValueError("M (required signatures) must be > 0 and <= N (number of authorized keys).")

    authorized_pubkey_hex_list = []
    for i, wallet_file in enumerate(authorized_wallet_files):
        try:
            priv_key = load_private_key(wallet_file, passwords[i])
            pub_key = priv_key.public_key()
            # Use the same address derivation as for single wallets for consistency in format
            # This address is the hex of uncompressed P256 public key bytes (0x04 + X + Y)
            address_hex = public_key_to_address(pub_key)
            authorized_pubkey_hex_list.append(address_hex)
        except Exception as e:
            raise ValueError(f"Error loading wallet {wallet_file}: {e}")

    # Ensure uniqueness of public keys before deriving address
    if len(set(authorized_pubkey_hex_list)) != len(authorized_pubkey_hex_list):
        raise ValueError("Duplicate public keys detected among authorized signers.")

    # Sort the hex public keys before deriving the address to ensure canonical derivation
    # The derive_multisig_address function also sorts internally, but good to be explicit.
    # The list stored in the config should also be sorted for consistency.
    sorted_authorized_pubkey_hex_list = sorted(authorized_pubkey_hex_list)

    multisig_address = derive_multisig_address(m_required, sorted_authorized_pubkey_hex_list)

    config_data = {
        "m_required": m_required,
        "authorized_public_keys_hex": sorted_authorized_pubkey_hex_list, # Store sorted list
        "multisig_address_hex": multisig_address,
        "n_total_keys": len(sorted_authorized_pubkey_hex_list)
    }

    with open(filepath, 'w') as f:
        json.dump(config_data, f, indent=4)

    print(f"Multi-sig configuration saved to: {filepath}")
    print(f"  M (Required Signatures): {m_required}")
    print(f"  N (Total Authorized Keys): {len(sorted_authorized_pubkey_hex_list)}")
    print(f"  Multi-sig Address (Identifier): {multisig_address}")
    for i, pk_hex in enumerate(sorted_authorized_pubkey_hex_list):
        print(f"    Authorized Key {i+1}: {pk_hex}")

def load_multisig_config(filepath: str) -> dict:
    """Loads a multi-sig configuration file."""
    if not os.path.exists(filepath):
        raise FileNotFoundError(f"Multi-sig config file not found: {filepath}")
    with open(filepath, 'r') as f:
        config_data = json.load(f)

    # Basic validation
    if not all(k in config_data for k in ["m_required", "authorized_public_keys_hex", "multisig_address_hex", "n_total_keys"]):
        raise ValueError("Invalid or incomplete multi-sig config file.")
    if config_data["m_required"] <= 0 or config_data["m_required"] > config_data["n_total_keys"]:
        raise ValueError("Invalid M or N values in config.")
    if len(config_data["authorized_public_keys_hex"]) != config_data["n_total_keys"]:
        raise ValueError("Mismatch between n_total_keys and length of authorized_public_keys_hex list.")

    # Verify the address derivation matches the stored one (consistency check)
    # This ensures the authorized_public_keys_hex are sorted in the config as expected.
    re_derived_address = derive_multisig_address(config_data["m_required"], config_data["authorized_public_keys_hex"])
    if re_derived_address != config_data["multisig_address_hex"]:
        raise ValueError("Multi-sig address in config does not match re-derivation. Public keys might be unsorted or M changed.")

    return config_data


if __name__ == '__main__':
    # Example Usage / Simple Test
    print("Multi-sig example:")
    # Create dummy wallet files for testing
    wallet_files = []
    passwords = []
    num_keys = 3
    for i in range(num_keys):
        priv, _ = generate_key_pair()
        fname = f"tmp_msig_signer_{i+1}.pem"
        pwd = f"pass{i+1}"
        save_private_key(priv, fname, pwd)
        wallet_files.append(fname)
        passwords.append(pwd)

    config_file = "tmp_multisig_config.json"
    m_val = 2

    try:
        print(f"\nCreating {m_val}-of-{num_keys} multi-sig config...")
        create_multisig_config(config_file, m_val, wallet_files, passwords)

        print(f"\nLoading multi-sig config from {config_file}...")
        loaded_cfg = load_multisig_config(config_file)
        print("Config loaded successfully:")
        print(json.dumps(loaded_cfg, indent=2))

        # Test address derivation consistency
        addr_from_loaded = derive_multisig_address(loaded_cfg['m_required'], loaded_cfg['authorized_public_keys_hex'])
        assert addr_from_loaded == loaded_cfg['multisig_address_hex']
        print("\nAddress derivation consistency check passed.")

    except Exception as e:
        print(f"Error in multi-sig example: {e}")
    finally:
        # Clean up dummy wallet files and config file
        for fname in wallet_files:
            if os.path.exists(fname):
                os.remove(fname)
        if os.path.exists(config_file):
            os.remove(config_file)
        print("\nCleaned up temporary files.")
