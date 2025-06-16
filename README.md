# Empower1 Blockchain

Empower1 is a learning project to build a simplified blockchain node from scratch. It demonstrates core concepts including peer-to-peer networking, proof-of-stake consensus, transaction handling (including multi-signature), smart contracts (via WASM), Decentralized Identifiers (DIDs), and a command-line wallet for interaction.

**Phase 1: Foundational Blockchain (Completed)**
*   Basic node structure, P2P communication (connection management, peer discovery).
*   Proof-of-Stake (PoS) consensus: static validator set, round-robin proposer selection, block proposal and validation.
*   Core data structures: Blocks and basic (single-signer) transactions.
*   Transaction lifecycle: Creation, signing (Go/Python crypto compatibility for ECDSA P-256 with DER signatures), mempool, inclusion in blocks.
*   Python CLI wallet: Key generation, basic transaction sending.
*   Cross-language hashing: Canonical JSON representation for transaction data to ensure consistent hashes between Go and Python.

**Phase 2: Empowering Functionality - Smart Intelligence (Completed)**
*   **Smart Contracts:**
    *   WASM-based execution using Wasmer-Go runtime.
    *   AssemblyScript as the initial smart contract language.
    *   Host Function API enabling contracts to interact with blockchain state (storage, logging, caller info).
    *   Basic gas model (gas tank, consumption in host functions, placeholder for instruction-level metering).
    *   Example contracts: `simple_contract` (basic execution) and `simple_storage` (stateful).
    *   Debug RPC endpoints for deploying and calling contracts.
    *   Documentation: [Smart Contracts Overview](docs/smart_contracts.md)
*   **Multi-Signature Wallets:**
    *   Native M-of-N multi-signature scheme.
    *   Multi-sig configuration (M, N public keys) included in transactions.
    *   Validation logic for multi-sig transactions in the Go node.
    *   Python CLI wallet support for: creating multi-sig configurations, initiating transactions, offline signature collection (file-based), and broadcasting.
    *   Documentation: [Multi-Signature Wallets](docs/multi_signature_wallets.md)
*   **Decentralized Identifiers (DIDs):**
    *   Implementation of the `did:key` method using SECP256R1 public keys (multicodec `0x1201` for uncompressed P256 public key, then Base58BTC encoded with `z` prefix).
    *   A `DIDRegistry` smart contract (written in AssemblyScript) allows anchoring DID document URIs and hashes on-chain, associated with a `did:key`.
    *   Authentication for registry operations is performed within the smart contract by verifying that the transaction signer's derived `did:key` matches the DID being managed. This uses new host functions (`blockchain_get_caller_public_key`, `blockchain_generate_did_key`).
    *   The Python CLI wallet now includes commands to generate `did:key`s from wallets, and to register and query DID document information via the `DIDRegistry` smart contract.
    *   Documentation: [Decentralized Identifiers (DIDs)](docs/did_framework.md)
*   **AI/ML Conceptual Framework:**
    *   A conceptual exploration of using K-Means clustering for anonymized "wealth" assessment based on potential blockchain activity patterns.
    *   Includes a Python script for offline analysis on mock, anonymized data.
    *   Documentation: [AI/ML Wealth Assessment Concept](docs/ai_ml_wealth_assessment.md)
*   **Developer Experience:**
    *   Comprehensive Smart Contract Development Guide: [Smart Contract Dev Guide](docs/smart_contract_dev_guide.md)
    *   AssemblyScript Boilerplate: A template project to kickstart smart contract development, located in `contract_templates/assemblyscript_boilerplate/`.
*   **Testing & Refinements:**
    *   Expanded Go unit tests (core, crypto, mempool, multi-sig transactions, DID crypto).
    *   Expanded Python CLI wallet unit tests (wallet, transactions, multi-sig, DID utils).
    *   Comprehensive manual testing guide (`TESTING.md`).

## Project Structure

*   `cmd/empower1d/`: Main application code for the Empower1 daemon (node).
*   `internal/core/`: Core blockchain logic (blocks, transactions).
*   `internal/crypto/`: Cryptographic utilities (ECDSA key generation, address handling).
*   `internal/mempool/`: Transaction pool for pending transactions.
*   `internal/p2p/`: Peer-to-peer networking for node communication.
*   `internal/consensus/`: Proof-of-Stake consensus logic (validator management, block proposal/validation).
*   `cli-wallet/`: Python-based command-line wallet for interacting with the Empower1 node.
*   `TESTING.md`: Comprehensive guide for manual integration testing, including multi-node setup and smart contract interactions.
*   `docs/`: Contains detailed design documents for various features.

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
Unit tests for the Python CLI wallet can be run from the project root (ensure dependencies from `cli-wallet/requirements.txt` are installed, preferably in a virtual environment):
```bash
python -m unittest discover -s ./cli-wallet/tests -p "test_*.py"
```

## Development Notes & Simplifications (Current State)

*   **Hashing Compatibility**: Transaction content hashing for signatures uses a canonical JSON representation (fields sorted alphabetically: Amount, Fee, From, PublicKey, Timestamp, To) hashed with SHA256. Both Go and Python sides adhere to this.
*   **Signatures**: ECDSA with SECP256R1 (P256) curve is used. Signatures are DER-encoded. Go node's `transaction.VerifySignature` uses `ecdsa.VerifyASN1` for compatibility.
*   **Validator Set**: Currently hardcoded in `cmd/empower1d/main.go`.
*   **Block Signing**: Placeholder string-based signing for blocks, not cryptographic.
*   **State Management**: In-memory data structures for blockchain, mempool, contract storage, and deployed contract code. No persistent database storage yet.
*   **Smart Contract Argument Marshalling**: Passing complex types (especially strings from Go to AssemblyScript function parameters) for contract calls via debug endpoints is simplified; robust ABI and memory management (e.g., using AS `__new`/`__pin`) is needed for general use.
*   **Gas Metering**: Currently basic (host function calls + flat fee for WASM execution). Instruction-level WASM metering is a future enhancement.
*   **Error Handling**: Implemented at various levels, but can always be made more granular and robust.
*   **Network Security**: Basic P2P communication; no advanced security features like encryption or authenticated P2P messages.
*   **Transaction Nonces:** Not yet implemented for replay protection for individual accounts or multi-sig entities.

## Future Work (Beyond Phase 2)
*   **Persistent Storage:** Integrate a database (e.g., BadgerDB, LevelDB) for all blockchain state.
*   **Full Account Model:** Implement comprehensive account state, balance tracking, and nonce management.
*   **Dynamic Validator Sets & Staking:** Introduce mechanisms for validators to join/leave and stake tokens.
*   **Cryptographic Block Signing:** Replace placeholder block signing with actual cryptographic signatures from validators.
*   **Advanced Smart Contract Features:**
    *   Robust ABI for argument passing and return values between Go and WASM.
    *   More complete host function API (e.g., inter-contract calls, access to more blockchain primitives).
    *   Event indexing and querying.
    *   Smart contract upgradeability.
*   **Enhanced P2P Networking:** Improve peer discovery, message broadcasting, and network resilience.
*   **Comprehensive Fee Model:** Implement `GasPrice` and a more nuanced fee structure.
*   **Light Clients & Wallets:** Develop more sophisticated wallet solutions and light client support.
*   **Formal Governance Mechanisms.**