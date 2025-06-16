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
    --node-url http://localhost:18080
```
*(Note: The Go node's HTTP server for RPC (including /tx/submit) is started on the `--debug-listen` address, which defaults to `:18080` in the current `cmd/empower1d/main.go` configuration.)*


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
