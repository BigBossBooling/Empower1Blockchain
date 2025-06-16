# Empower1 Blockchain

Empower1 is a learning project to build a simplified blockchain node from scratch, demonstrating core concepts like peer-to-peer networking, proof-of-stake consensus, transaction handling, and a command-line wallet.

**Phase 1 (Completed):** Basic node structure, P2P communication, PoS consensus (proposer selection, block validation), transaction creation & signing (compatible Go/Python crypto), mempool, and a Python CLI wallet for interaction.

## Project Structure

*   `cmd/empower1d/`: Main application code for the Empower1 daemon (node).
*   `internal/core/`: Core blockchain logic (blocks, transactions).
*   `internal/crypto/`: Cryptographic utilities (ECDSA key generation, address handling).
*   `internal/mempool/`: Transaction pool for pending transactions.
*   `internal/p2p/`: Peer-to-peer networking for node communication.
*   `internal/consensus/`: Proof-of-Stake consensus logic (validator management, block proposal/validation).
*   `cli-wallet/`: Python-based command-line wallet for interacting with the Empower1 node.
*   `TESTING.md`: Guide for manual integration testing.

## Getting Started

### Prerequisites
*   Go (version 1.18+ recommended)
*   Python (version 3.7+ recommended for the CLI wallet)

### Building the Go Node

1.  Navigate to the project root directory.
2.  Build the `empower1d` executable:
    ```bash
    go build ./cmd/empower1d
    ```
    This creates an `empower1d` executable in the root directory.

### Running a Single Node

To run a single Empower1 node (e.g., for development or as the first node in a network):
```bash
./empower1d -listen :8001 -validator -validator-addr validator-1-addr -debug-listen :18001
```
*   `-listen :8001`: P2P listening port.
*   `-validator`: Runs this node as a validator.
*   `-validator-addr validator-1-addr`: Specifies its validator identity (must match one from the hardcoded list in `cmd/empower1d/main.go` for the PoS consensus). The current hardcoded validator addresses are `validator-1-addr`, `validator-2-addr`, `validator-3-addr`.
*   `-debug-listen :18001`: Port for HTTP RPC and debug endpoints.

### CLI Wallet

A Python-based CLI wallet is available in the `cli-wallet/` directory. It allows you to:
*   Generate wallets.
*   Create and sign transactions.
*   Submit transactions to a running Empower1 node.

For setup and usage instructions, see the [CLI Wallet README](cli-wallet/README.md).

### HTTP RPC Endpoints (Go Node)

The Go node exposes several HTTP RPC endpoints on the port specified by `-debug-listen` (default `:18080` or as configured, e.g., `:18001` in the example above):

*   `POST /tx/submit`: Submit a new transaction. Expects a JSON payload with transaction details (see `SubmitTxPayload` in `cmd/empower1d/main.go`; byte arrays like ID, From, To, Signature, PublicKey should be Base64-encoded strings).
*   `GET /info`: View basic node status information (height, validator status, mempool size, peers).
*   `GET /mempool`: View transactions currently in the mempool.
*   `GET /create-test-tx`: (Debug) Creates, signs, and broadcasts a test transaction from the node's own wallet.

## Testing

For detailed instructions on manual integration testing, including running multiple nodes and using the CLI wallet to send transactions, please refer to [TESTING.md](TESTING.md).

Unit tests for Go packages can be run using standard Go tooling from the project root:
```bash
go test ./...
```
Unit tests for the Python CLI wallet can be run from the project root:
```bash
python -m unittest discover -s ./cli-wallet/tests -p "test_*.py"
```
(Ensure you have installed Python dependencies from `cli-wallet/requirements.txt`, preferably in a virtual environment.)

## Development Notes & Simplifications

*   **Hashing Compatibility**: Transaction content hashing for signatures uses a canonical JSON representation (fields sorted alphabetically: Amount, Fee, From, PublicKey, Timestamp, To) hashed with SHA256. Both Go and Python sides adhere to this.
*   **Signatures**: ECDSA with SECP256R1 (P256) curve is used. Signatures are DER-encoded. Go node's `transaction.VerifySignature` uses `ecdsa.VerifyASN1` for compatibility.
*   **Validator Set**: Currently hardcoded in `cmd/empower1d/main.go`.
*   **Block Signing**: Placeholder string-based signing for blocks, not cryptographic.
*   **State Management**: In-memory data structures for blockchain, mempool, etc. No persistent storage yet.
*   **Error Handling**: Basic error handling; can be made more robust.
*   **Network Security**: No encryption or advanced P2P security features implemented.

## Future Work (Potential Phase 2)
*   Persistent storage (e.g., BadgerDB, LevelDB).
*   Account state and balance tracking.
*   More sophisticated validator management (staking, dynamic set).
*   Cryptographic block signing.
*   Smart contract capabilities (e.g., via a VM like EVM or WASM).
*   Improved network robustness and peer discovery.
*   More comprehensive testing and metrics.