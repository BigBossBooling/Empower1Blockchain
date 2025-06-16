import requests
import json
from transaction import Transaction # Assuming Transaction class is in transaction.py

def send_transaction(node_url: str, tx: Transaction) -> (bool, dict):
    """
    Sends a signed transaction to the Go node.
    The transaction should be fully signed and have its ID set.
    """
    if not node_url.startswith("http"):
        node_url = f"http://{node_url}" # Default to http if not specified

    endpoint = f"{node_url}/tx/submit" # Standardized endpoint

    if not tx.id or not tx.signature or not tx.public_key:
        raise ValueError("Transaction must be fully signed and ID'd before sending.")

    # Prepare payload. Go's json.Unmarshal expects byte arrays to be base64 encoded.
    payload = {
        "ID": bytes.fromhex(tx.id).decode('latin-1'), # This is still problematic, should be base64
        "Timestamp": tx.timestamp,
        "From": tx.from_address, # Keep as hex string, Go side can decode from hex
        "To": tx.to_address,     # Keep as hex string
        "Amount": tx.amount,
        "Fee": tx.fee,
        "Signature": tx.signature, # Keep as hex string of DER signature
        "PublicKey": tx.public_key # Keep as hex string of public key
    }

    # Corrected payload for Go JSON unmarshalling of []byte fields:
    # Go's core.Transaction has []byte for ID, From, To, Signature, PublicKey.
    # When unmarshalling JSON into a struct with []byte fields, Go expects base64-encoded strings.

    tx_payload_for_go = {
        "ID": bytes.fromhex(tx.id).decode('latin-1'), # Incorrect: Should be base64(bytes.fromhex(tx.id))
                                                     # Or, Go side needs to expect hex and decode.
                                                     # For now, let's assume Go side will handle hex strings for these.
                                                     # This matches what block.Sign() and ProposerAddress use (strings).
                                                     # Let's make Go's Transaction struct use strings for these hex representations.
                                                     # This simplifies things greatly.
                                                     # If Go core.Transaction uses []byte, then Python MUST send base64.

        # Let's re-evaluate. The Go `core.Transaction` has []byte fields.
        # The Python `Transaction` stores these as hex strings.
        # For submission, we should send them in a format Go's JSON decoder for []byte expects: base64.
        # Or, we modify the Go struct to take hex strings and decode them.
        # The latter is simpler for now and consistent with how addresses are handled elsewhere (conceptually).
        # Let's assume Go's Transaction struct is modified to accept hex strings for these byte fields,
        # and does hex.DecodeString internally. This is a major assumption for now.
        #
        # Re-Correction: The Go `core.Transaction` struct has `[]byte` fields.
        # The `json` package in Go, when unmarshalling into a `[]byte` field, expects a base64-encoded string.
        # So, Python *must* send base64 encoded strings for `ID`, `From`, `To`, `Signature`, `PublicKey`.
    }

    import base64
    final_payload = {
        "ID": base64.b64encode(bytes.fromhex(tx.id)).decode('utf-8'),
        "Timestamp": tx.timestamp,
        "From": base64.b64encode(bytes.fromhex(tx.from_address)).decode('utf-8'),
        "To": base64.b64encode(bytes.fromhex(tx.to_address)).decode('utf-8'),
        "Amount": tx.amount,
        "Fee": tx.fee,
        "Signature": base64.b64encode(bytes.fromhex(tx.signature)).decode('utf-8'),
        "PublicKey": base64.b64encode(bytes.fromhex(tx.public_key)).decode('utf-8'),
    }

    try:
        headers = {'Content-Type': 'application/json'}
        response = requests.post(endpoint, data=json.dumps(final_payload), headers=headers, timeout=10)

        if response.status_code == 200 or response.status_code == 202: # Accepted
            print(f"Transaction submitted successfully to {endpoint}")
            try:
                return True, response.json()
            except json.JSONDecodeError:
                return True, {"message": response.text} # Return text if not JSON
        else:
            print(f"Failed to submit transaction to {endpoint}. Status: {response.status_code}")
            try:
                return False, response.json() # Try to get error details if JSON
            except json.JSONDecodeError:
                return False, {"error": response.text, "status_code": response.status_code}

    except requests.exceptions.RequestException as e:
        print(f"Error connecting to node {endpoint}: {e}")
        return False, {"error": str(e)}

if __name__ == '__main__':
    # This is a placeholder test. Needs a running Go node.
    # And wallet.py and transaction.py must be in the same directory or PYTHONPATH

    print("Client tests (requires a running Go node and configured wallet/tx):")

    # 1. Setup a dummy wallet and transaction (normally done via CLI commands)
    from wallet import generate_key_pair, save_private_key, get_public_key_bytes
    import os

    # Create dummy wallet
    sender_priv, sender_pub = generate_key_pair()
    recipient_priv, recipient_pub = generate_key_pair() # Dummy recipient

    wallet_file = "tmp_test_client_wallet.pem"
    save_private_key(sender_priv, wallet_file, "test")

    # Create a transaction instance
    tx_to_send = Transaction(
        from_address_bytes=get_public_key_bytes(sender_pub),
        to_address_bytes=get_public_key_bytes(recipient_pub),
        amount=10,
        fee=1
    )
    tx_to_send.sign(wallet_file, "test") # Signs and sets ID, PublicKey, Signature

    print(f"Transaction to send: {tx_to_send.to_dict()}")

    # 2. Attempt to send (this will fail if node is not running at http://localhost:18080/tx/submit)
    node_url_test = "http://localhost:18080" # Matches debug server in Go node if that's where /tx/submit is
                                        # The Go node main.go currently has debug on :18080, P2P on :8080
                                        # The /tx/submit endpoint is not yet on the debug server.
                                        # It will be on the main application port, e.g., :8080 or :18080 if we choose one.
                                        # Let's assume we'll add /tx/submit to the debug server for now.

    print(f"\nAttempting to send transaction to {node_url_test}...")
    success, response_data = send_transaction(node_url_test, tx_to_send)

    if success:
        print("Test send_transaction SUCCEEDED (mock or actual). Response:")
    else:
        print("Test send_transaction FAILED. Response:")
    print(response_data)

    os.remove(wallet_file)
    print("\nClient test finished. Remember to have a Go node running with the /tx/submit endpoint.")
