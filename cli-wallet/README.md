# Empower1 CLI Wallet

This CLI wallet allows interaction with an Empower1 blockchain node. It can be used to:
- Generate new wallet key pairs (ECDSA SECP256R1/P256).
- Save and load wallet keys from PEM files (optionally encrypted).
- Display wallet public addresses.
- Create, sign, and send transactions to an Empower1 node.

## Prerequisites

- Python 3.7+
- An Empower1 node running and accessible via its RPC URL.

## Installation

1.  **Clone the repository** (if you haven't already) or ensure this `cli-wallet` directory is available.
2.  **Navigate to the `cli-wallet` directory**:
    ```bash
    cd path/to/empower1/cli-wallet
    ```
3.  **Install dependencies**:
    It's recommended to use a virtual environment.
    ```bash
    python3 -m venv venv
    source venv/bin/activate  # On Windows: venv\Scripts\activate
    pip install -r requirements.txt
    ```

## Usage

The main script is `main.py`. Use the `--help` option to see available commands and their options.

```bash
python main.py --help
```

### Commands

**1. Generate a new wallet:**
```bash
python main.py generate-wallet --outfile my_wallet.pem
```
With password encryption:
```bash
python main.py generate-wallet --outfile my_secure_wallet.pem --password yoursecretpassword
```

**2. Get public address from a wallet file:**
```bash
python main.py get-address --infile my_wallet.pem
```
If encrypted:
```bash
python main.py get-address --infile my_secure_wallet.pem --password yoursecretpassword
```

**3. Create and send a transaction:**
Make sure you have a wallet file (e.g., `my_wallet.pem`) and know the recipient's address and the node URL.
The recipient address should be the hex string of their uncompressed public key (starting with `04...`).

```bash
python main.py create-transaction \
    --from-wallet my_wallet.pem \
    --to <recipient_public_key_hex> \
    --amount <amount_to_send> \
    --node-url http://localhost:18080
    # (Adjust --node-url to your node's HTTP RPC address, e.g. http://localhost:8080 if /tx/submit is on main port)
```
If the sender's wallet is encrypted:
```bash
python main.py create-transaction \
    --from-wallet my_secure_wallet.pem --password yoursenderpassword \
    --to <recipient_public_key_hex> \
    --amount <amount_to_send> \
    --fee 1 \
    --node-url http://localhost:18001
    # (Adjust node_url to your node's HTTP RPC address, e.g., http://localhost:18001 if using default debug port for node 1)
```

**4. DID (Decentralized Identifier) Commands:**

*   **Generate a `did:key` for a wallet:**
    ```bash
    python main.py did generate --wallet my_wallet.pem --password mypassword
    ```
    This will output the `did:key:...` string (e.g., `did:key:zQ3s...`) associated with the wallet's public key, constructed using multicodec `0x1201` and base58btc.

*   **Register a DID Document (via `DIDRegistry` smart contract):**
    First, ensure the `DIDRegistry` smart contract is deployed (see main project `TESTING.md` for instructions on deploying contracts like `did_registry.wasm`). Let its on-chain address be `<did_registry_contract_address>`.
    You must use the wallet that corresponds to the DID you are registering.
    ```bash
    # Example:
    # Assume 'did_owner_wallet.pem' generated '<owner_did_key_string>'
    python main.py did register-document \
        --wallet did_owner_wallet.pem --password didownerpass \
        --did "<owner_did_key_string>" \
        --doc-hash "your_document_sha256_hash" \
        --doc-uri "ipfs://your_document_cid" \
        --contract-address "<did_registry_contract_address>" \
        --node-url http://localhost:18001 \
        --fee 10
    ```
    This command:
    1. Verifies that the `--did` string corresponds to the public key in `--wallet`.
    2. Creates a contract call transaction targeting the `registerDIDDocument` function of the `DIDRegistry` contract.
    3. The transaction is signed by the private key in `did_owner_wallet.pem`.
    4. Submits the transaction to the node via the `/tx/submit` endpoint. The smart contract will then perform its own authentication by comparing the `did_string` with a `did:key` generated from the caller's public key (obtained via a host function).

*   **Get DID Document Information (from `DIDRegistry` smart contract):**
    ```bash
    python main.py did get-info \
        --did "<target_did_key_string>" \
        --contract-address "<did_registry_contract_address>" \
        --node-url http://localhost:18001
    ```
    This command calls the `getDIDDocumentInfo` function on the `DIDRegistry` contract.
    *Currently, this CLI command uses the Go node's `/debug/call-contract` endpoint for simplicity, as `getDIDDocumentInfo` is a read-only query. This means it doesn't require a signing wallet or send a full on-chain transaction for this query.*
    It will print the JSON string `"{ "document_hash": "...", "document_location_uri": "..." }"` or `null`.

*(Note: The Go node's HTTP server for RPC and debug endpoints (like `/tx/submit`, `/debug/call-contract`) is typically run on the port specified by the `-debug-listen` flag, e.g., `:18001`, `:18002`.)*

## Development Notes

- **Key Generation**: Uses ECDSA with the SECP256R1 curve (same as Go's P256).
- **Address Format**: The address is the hexadecimal representation of the uncompressed public key (prefixed with `04`).
- **Transaction Hashing**: For signing, the transaction data (Timestamp, From, To, Amount, Fee, PublicKey) is serialized into a canonical JSON string (keys sorted alphabetically) and then hashed with SHA256. The Go node does the equivalent to verify.
- **Signatures**: Signatures are DER-encoded. The Go node is configured to verify DER-encoded signatures for transactions.
- **Transaction Submission**: Transactions are POSTed as JSON to the `/tx/submit` endpoint on the Go node. Byte array fields in the JSON (ID, From, To, Signature, PublicKey) are sent as Base64-encoded strings.

## TODO

- Add `get-balance` command (requires corresponding node endpoint).
- More robust error handling and user feedback.
- Support for different wallet file formats or hardware wallets.
- Configuration file for node URL, default wallet, etc.
- More sophisticated fee estimation.
- Better handling of transaction nonces if replay protection becomes necessary beyond timestamp/ID.
