import time
import hashlib
import json
import base64 # For encoding byte arrays in JSON for Go
from cryptography.hazmat.primitives import hashes as crypto_hashes
from cryptography.hazmat.primitives.asymmetric import ec, utils
from cryptography.hazmat.primitives.serialization import Encoding, PublicFormat

# Assuming wallet.py is in the same directory or PYTHONPATH
from wallet import CURVE, load_private_key, get_public_key_bytes

# Transaction Types (mirroring Go's core.TransactionType)
TX_STANDARD = "standard"
TX_CONTRACT_DEPLOY = "contract_deployment"
TX_CONTRACT_CALL = "contract_call"
# Note: A transaction *becomes* multi-sig by populating multi-sig fields,
# TxType might still be standard, deploy, or call.

class SignerInfo:
    def __init__(self, public_key_hex: str, signature_hex: str):
        self.public_key_hex = public_key_hex # Hex string of the signer's public key
        self.signature_hex = signature_hex  # Hex string of the DER-encoded signature

    def to_dict(self):
        # For saving to pending tx file (human-readable hex)
        return {
            "publicKeyHex": self.public_key_hex,
            "signatureHex": self.signature_hex,
        }

    @classmethod
    def from_dict(cls, data: dict):
        return cls(data['publicKeyHex'], data['signatureHex'])

class Transaction:
    def __init__(self, from_address_hex: str, timestamp: int = None,
                 # Standard tx fields
                 to_address_hex: str = None, amount: int = 0, fee: int = 0,
                 # Contract deploy
                 contract_code_bytes: bytes = None,
                 # Contract call
                 target_contract_address_hex: str = None, function_name: str = None, arguments_bytes: bytes = None,
                 # Single signer (if not multi-sig)
                 public_key_hex: str = None,
                 signature_hex: str = None,
                 # Multi-sig fields
                 required_signatures: int = 0,
                 authorized_public_keys_hex: list[str] = None,
                 signers: list[SignerInfo] = None,
                 tx_type: str = TX_STANDARD,
                 tx_id_hex: str = None
                 ):

        self.id_hex = tx_id_hex
        self.timestamp = timestamp if timestamp is not None else int(time.time() * 1_000_000_000)

        self.from_address_hex = from_address_hex

        self.public_key_hex = public_key_hex
        self.signature_hex = signature_hex

        self.tx_type = tx_type

        self.to_address_hex = to_address_hex
        self.amount = int(amount)
        self.fee = int(fee)

        self.contract_code_bytes = contract_code_bytes
        self.target_contract_address_hex = target_contract_address_hex
        self.function_name = function_name
        self.arguments_bytes = arguments_bytes

        self.required_signatures = int(required_signatures)
        self.authorized_public_keys_hex = sorted(list(set(authorized_public_keys_hex))) if authorized_public_keys_hex else []
        self.signers = signers if signers else []

    def __repr__(self):
        return (f"<Transaction id={self.id_hex} type='{self.tx_type}' from='{self.from_address_hex}' "
                f"to='{self.to_address_hex}' amount={self.amount} "
                f"multisigM={self.required_signatures} N={len(self.authorized_public_keys_hex)} signed={len(self.signers)} >")

    def to_dict_for_file(self) -> dict:
        """Returns a dictionary representation suitable for saving to/loading from a JSON file."""
        return {
            "id_hex": self.id_hex,
            "timestamp": self.timestamp,
            "from_address_hex": self.from_address_hex, # This would be the multi-sig address/ID
            "public_key_hex": self.public_key_hex, # Single-signer's pubkey (empty for multi-sig)
            "signature_hex": self.signature_hex,   # Single-signer's signature (empty for multi-sig)
            "tx_type": self.tx_type,
            "to_address_hex": self.to_address_hex,
            "amount": self.amount,
            "fee": self.fee,
            "contract_code_bytes_b64": base64.b64encode(self.contract_code_bytes).decode('utf-8') if self.contract_code_bytes else None,
            "target_contract_address_hex": self.target_contract_address_hex,
            "function_name": self.function_name,
            "arguments_bytes_b64": base64.b64encode(self.arguments_bytes).decode('utf-8') if self.arguments_bytes else None,
            "required_signatures": self.required_signatures,
            "authorized_public_keys_hex": self.authorized_public_keys_hex, # Stored sorted
            "signers": [signer.to_dict() for signer in self.signers],
        }

    @classmethod
    def from_dict_for_file(cls, data: dict):
        """Creates a Transaction object from a dictionary (e.g., loaded from JSON file)."""
        signers_list = [SignerInfo.from_dict(s_data) for s_data in data.get("signers", [])]

        return cls(
            tx_id_hex=data.get("id_hex"),
            timestamp=data["timestamp"],
            from_address_hex=data["from_address_hex"],
            public_key_hex=data.get("public_key_hex"),
            signature_hex=data.get("signature_hex"),
            tx_type=data.get("tx_type", TX_STANDARD),
            to_address_hex=data.get("to_address_hex"),
            amount=data.get("amount", 0),
            fee=data.get("fee", 0),
            contract_code_bytes=base64.b64decode(data["contract_code_bytes_b64"]) if data.get("contract_code_bytes_b64") else None,
            target_contract_address_hex=data.get("target_contract_address_hex"),
            function_name=data.get("function_name"),
            arguments_bytes=base64.b64decode(data["arguments_bytes_b64"]) if data.get("arguments_bytes_b64") else None,
            required_signatures=data.get("required_signatures", 0),
            authorized_public_keys_hex=data.get("authorized_public_keys_hex", []), # Assumes already sorted if from our own file
            signers=signers_list
        )

    def data_for_hashing(self) -> bytes:
        """
        Prepares the transaction data for hashing.
        This MUST produce a JSON string with alphabetically sorted keys to match Go's TxDataForJSONHashing.
        """
        payload_to_hash = {
            "Fee": self.fee,
            "From": self.from_address_hex,
            "Timestamp": self.timestamp,
            "TxType": self.tx_type,
        }

        # Single-signer public key (only if this is not a multi-sig where From is already the multi-sig ID)
        if self.public_key_hex and self.required_signatures == 0:
             payload_to_hash["PublicKey"] = self.public_key_hex

        # Fields for specific transaction types
        if self.tx_type == TX_STANDARD:
            if self.to_address_hex is not None: payload_to_hash["To"] = self.to_address_hex
            payload_to_hash["Amount"] = self.amount
        elif self.tx_type == TX_CONTRACT_DEPLOY:
            if self.contract_code_bytes is not None:
                payload_to_hash["ContractCode"] = base64.b64encode(self.contract_code_bytes).decode('utf-8')
            # Amount might be 0 for deployment, handled by omitempty in Go or explicit inclusion if needed.
            payload_to_hash["Amount"] = self.amount # Amount can be 0
        elif self.tx_type == TX_CONTRACT_CALL:
            if self.target_contract_address_hex is not None:
                payload_to_hash["TargetContractAddress"] = self.target_contract_address_hex
            if self.function_name is not None: payload_to_hash["FunctionName"] = self.function_name
            if self.arguments_bytes is not None:
                payload_to_hash["Arguments"] = base64.b64encode(self.arguments_bytes).decode('utf-8')
            payload_to_hash["Amount"] = self.amount # Amount can be value sent to contract

        # Multi-signature configuration IS PART OF WHAT'S SIGNED
        if self.required_signatures > 0 and len(self.authorized_public_keys_hex) > 0:
            payload_to_hash["RequiredSignatures"] = self.required_signatures
            # Ensure AuthorizedPublicKeys are sorted for canonical representation
            payload_to_hash["AuthorizedPublicKeys"] = sorted(list(set(self.authorized_public_keys_hex)))

        json_string = json.dumps(payload_to_hash, sort_keys=True, separators=(',', ':'))
        return json_string.encode('utf-8')

    def calculate_hash(self) -> str:
        """Calculates the SHA256 hash of the transaction content (hex string)."""
        data_to_hash = self.data_for_hashing()
        hasher = hashlib.sha256()
        hasher.update(data_to_hash)
        return hasher.hexdigest()

    def sign_single(self, private_key_pem_path: str, password: str = None) -> bool:
        """Signs a standard (non-multi-sig) transaction."""
        if self.required_signatures > 0:
            raise ValueError("Use 'add_signature' for multi-sig configured transactions.")

        signing_key = load_private_key(private_key_pem_path, password)
        self.public_key_hex = get_public_key_bytes(signing_key.public_key()).hex()
        # For a single signer tx, From is their own public key address.
        # This should have been set correctly in the constructor.
        if self.from_address_hex != self.public_key_hex:
             print(f"Warning: from_address_hex '{self.from_address_hex}' does not match signer's public_key_hex '{self.public_key_hex}'. Overwriting from_address_hex.")
             self.from_address_hex = self.public_key_hex

        content_hash_hex = self.calculate_hash()
        self.id_hex = content_hash_hex

        content_hash_bytes = bytes.fromhex(content_hash_hex)
        signature_bytes_der = signing_key.sign(
            content_hash_bytes,
            ec.ECDSA(utils.Prehashed(crypto_hashes.SHA256()))
        )
        self.signature_hex = signature_bytes_der.hex()
        return True

    def add_signature(self, private_key_pem_path: str, password: str = None) -> bool:
        """Adds a signature from one of the authorized signers to a multi-sig transaction."""
        if not (self.required_signatures > 0 and len(self.authorized_public_keys_hex) > 0):
            raise ValueError("Transaction is not configured for multi-signature (M and N keys required).")

        signing_key = load_private_key(private_key_pem_path, password)
        signer_public_key_hex = get_public_key_bytes(signing_key.public_key()).hex()

        if signer_public_key_hex not in self.authorized_public_keys_hex:
            raise ValueError(f"Signer's public key {signer_public_key_hex} is not in the authorized list.")

        for s_info in self.signers:
            if s_info.public_key_hex == signer_public_key_hex:
                return True # Already signed by this key

        content_hash_hex = self.calculate_hash()
        if not self.id_hex:
            self.id_hex = content_hash_hex
        elif self.id_hex != content_hash_hex:
            raise ValueError("Transaction content hash mismatch after initiation. Ensure base transaction data hasn't changed.")

        content_hash_bytes = bytes.fromhex(content_hash_hex)
        signature_bytes_der = signing_key.sign(
            content_hash_bytes,
            ec.ECDSA(utils.Prehashed(crypto_hashes.SHA256()))
        )
        signature_hex = signature_bytes_der.hex()

        self.signers.append(SignerInfo(public_key_hex=signer_public_key_hex, signature_hex=signature_hex))
        # Ensure signers are stored sorted by public key hex for deterministic order if needed later (e.g. for tx comparison)
        self.signers.sort(key=lambda s: s.public_key_hex)
        return True

    def verify_signatures_python(self) -> bool:
        """Verifies signatures for either single or multi-sig transactions (Python side)."""
        content_hash_hex = self.calculate_hash()
        if self.id_hex and self.id_hex != content_hash_hex:
             pass
        content_hash_bytes = bytes.fromhex(content_hash_hex)

        if self.required_signatures > 0: # Multi-sig
            if len(self.signers) < self.required_signatures:
                return False

            valid_unique_signatures = 0
            signed_pub_keys = set()
            # Ensure authorized_public_keys_hex are sorted for verification consistency with how they might be stored or processed.
            # The list itself should already be sorted from __init__ or loading.
            # sorted_auth_keys_for_check = sorted(list(set(self.authorized_public_keys_hex)))


            for signer_info in self.signers:
                # Check if signer is authorized (using the potentially unsorted list from tx obj,
                # as the key itself is what matters, not its position in an arbitrarily sorted list for this check)
                if signer_info.public_key_hex not in self.authorized_public_keys_hex:
                    return False
                if signer_info.public_key_hex in signed_pub_keys:
                    return False # Duplicate signer
                try:
                    pub_key_bytes = bytes.fromhex(signer_info.public_key_hex)
                    pub_key_obj = ec.EllipticCurvePublicKey.from_encoded_point(CURVE, pub_key_bytes)
                    sig_bytes = bytes.fromhex(signer_info.signature_hex)
                    pub_key_obj.verify(
                        sig_bytes, content_hash_bytes, ec.ECDSA(utils.Prehashed(crypto_hashes.SHA256()))
                    )
                    signed_pub_keys.add(signer_info.public_key_hex)
                    valid_unique_signatures += 1
                except Exception:
                    return False

            return valid_unique_signatures >= self.required_signatures

        else: # Single-signer
            if not self.public_key_hex or not self.signature_hex:
                return False
            try:
                pub_key_bytes = bytes.fromhex(self.public_key_hex)
                pub_key_obj = ec.EllipticCurvePublicKey.from_encoded_point(CURVE, pub_key_bytes)
                sig_bytes = bytes.fromhex(self.signature_hex)
                pub_key_obj.verify(
                    sig_bytes, content_hash_bytes, ec.ECDSA(utils.Prehashed(crypto_hashes.SHA256()))
                )
                return True
            except Exception:
                return False

# Example Usage (for testing this file directly)
if __name__ == '__main__':
    from wallet import generate_key_pair, public_key_to_address, get_public_key_bytes, save_private_key
    import os

    # Create dummy wallet files
    s_priv1, s_pub1 = generate_key_pair()
    s_priv2, s_pub2 = generate_key_pair()
    s_priv3, s_pub3 = generate_key_pair()
    r_priv, r_pub = generate_key_pair() # Recipient

    save_private_key(s_priv1, "tmp_signer1.pem", "p1")
    save_private_key(s_priv2, "tmp_signer2.pem", "p2")
    save_private_key(s_priv3, "tmp_signer3.pem", "p3")

    s_pub1_hex = get_public_key_bytes(s_pub1).hex()
    s_pub2_hex = get_public_key_bytes(s_pub2).hex()
    s_pub3_hex = get_public_key_bytes(s_pub3).hex()
    r_pub_hex = get_public_key_bytes(r_pub).hex()

    print("--- Single Signer Transaction Test ---")
    tx_single = Transaction(
        from_address_hex=s_pub1_hex, # Will be re-set by sign_single to signer's pubkey
        to_address_hex=r_pub_hex,
        amount=100,
        fee=1,
        tx_type=TX_STANDARD
    )
    tx_single.sign_single("tmp_signer1.pem", "p1")
    print(f"Single-signer TX ID: {tx_single.id_hex}, Valid (Python): {tx_single.verify_signatures_python()}")
    assert tx_single.verify_signatures_python()

    print("\n--- Multi-Signature Transaction Test (2-of-3) ---")
    auth_keys = [s_pub1_hex, s_pub2_hex, s_pub3_hex]
    # Multi-sig address/ID (example, not strictly enforced by Transaction class itself)
    from multisig import derive_multisig_address
    multi_sig_id = derive_multisig_address(2, auth_keys)
    print(f"Derived Multi-sig ID: {multi_sig_id}")

    tx_multi = Transaction(
        from_address_hex=multi_sig_id,
        to_address_hex=r_pub_hex,
        amount=200,
        fee=2,
        tx_type=TX_STANDARD, # A standard transfer, but authorized by multi-sig
        required_signatures=2,
        authorized_public_keys_hex=auth_keys
    )

    print(f"Initial multi-sig tx (unsigned): {tx_multi.to_dict_for_file()}")
    # Hash before any signatures (should be stable)
    initial_hash = tx_multi.calculate_hash()
    tx_multi.id_hex = initial_hash # Set ID for pending tx
    print(f"Initial hash for signing: {initial_hash}")


    tx_multi.add_signature("tmp_signer1.pem", "p1")
    self_hash_after_sig1 = tx_multi.calculate_hash()
    assert initial_hash == self_hash_after_sig1 # Hash of content should not change by adding a signature
    print(f"After 1st signature (Signer1: {s_pub1_hex[:10]}...), Signers: {len(tx_multi.signers)}")
    self_valid1 = tx_multi.verify_signatures_python()
    print(f"Valid after 1 signature (Python): {self_valid1} (expected False as M=2)")
    assert not self_valid1 # Not enough signatures yet

    tx_multi.add_signature("tmp_signer2.pem", "p2")
    self_hash_after_sig2 = tx_multi.calculate_hash()
    assert initial_hash == self_hash_after_sig2
    print(f"After 2nd signature (Signer2: {s_pub2_hex[:10]}...), Signers: {len(tx_multi.signers)}")
    self_valid2 = tx_multi.verify_signatures_python()
    print(f"Valid after 2 signatures (Python): {self_valid2} (expected True as M=2)")
    assert self_valid2

    # Try adding a signature from an unauthorized key (should fail)
    unauth_priv, _ = generate_key_pair()
    save_private_key(unauth_priv, "tmp_unauth.pem", "unauth")
    try:
        tx_multi.add_signature("tmp_unauth.pem", "unauth")
        print("Error: Added signature from unauthorized key!")
        assert False
    except ValueError as e:
        print(f"Correctly failed to add unauthorized signature: {e}")
    os.remove("tmp_unauth.pem")


    # Test saving and loading pending multi-sig tx
    pending_tx_file = "tmp_pending_multisig.json"
    with open(pending_tx_file, 'w') as f:
        json.dump(tx_multi.to_dict_for_file(), f, indent=2)

    with open(pending_tx_file, 'r') as f:
        loaded_data = json.load(f)

    loaded_tx_multi = Transaction.from_dict_for_file(loaded_data)
    print(f"Loaded multi-sig TX ID: {loaded_tx_multi.id_hex}, Valid (Python): {loaded_tx_multi.verify_signatures_python()}")
    assert loaded_tx_multi.id_hex == initial_hash
    assert loaded_tx_multi.verify_signatures_python()
    os.remove(pending_tx_file)


    print("\nAll transaction.py multi-sig tests passed.")

    # Clean up
    os.remove("tmp_signer1.pem")
    os.remove("tmp_signer2.pem")
    os.remove("tmp_signer3.pem")
