# Manual Integration Testing Guide

This guide provides instructions for manually testing the end-to-end functionality of the Empower1 blockchain node and the CLI wallet.

## Prerequisites

1.  **Go**: Ensure Go is installed (version 1.18+ recommended).
2.  **Python**: Ensure Python is installed (version 3.7+ recommended).
3.  **Project Code**: Clone the Empower1 blockchain repository.

## I. Build the Go Node

1.  Navigate to the project's root directory.
2.  Build the `empower1d` executable:
    ```bash
    go build ./cmd/empower1d
    ```
    This will create an `empower1d` (or `empower1d.exe` on Windows) executable in the root directory.

## II. Set up and Run Multiple Go Nodes

We will run three validator nodes. The current Go node implementation uses a hardcoded list of three validators:
- `validator-1-addr`
- `validator-2-addr`
- `validator-3-addr`

Open three separate terminal windows.

**Node 1 (Validator 1):**
```bash
./empower1d -listen :8001 -validator -validator-addr validator-1-addr -debug-listen :18001
```
*   `-listen :8001`: P2P listening port.
*   `-validator`: Runs this node as a validator.
*   `-validator-addr validator-1-addr`: Specifies its validator identity (must match one from the hardcoded list in `cmd/empower1d/main.go`).
*   `-debug-listen :18001`: Port for HTTP RPC and debug endpoints (like `/tx/submit`, `/info`, `/mempool`).
*   No `-connect` flag, as it's the first node.

**Node 2 (Validator 2):**
```bash
./empower1d -listen :8002 -connect localhost:8001 -validator -validator-addr validator-2-addr -debug-listen :18002
```
*   `-listen :8002`: Different P2P port.
*   `-connect localhost:8001`: Connects to Node 1 for peer discovery.
*   `-validator-addr validator-2-addr`: Its validator identity.
*   `-debug-listen :18002`: Different HTTP port.

**Node 3 (Validator 3):**
```bash
./empower1d -listen :8003 -connect localhost:8001 -validator -validator-addr validator-3-addr -debug-listen :18003
```
*   `-listen :8003`: Different P2P port.
*   `-connect localhost:8001`: Connects to Node 1. (Could also connect to Node 2: `localhost:8002`).
*   `-validator-addr validator-3-addr`: Its validator identity.
*   `-debug-listen :18003`: Different HTTP port.

**Observe Node Logs:**
*   Nodes should start up and log their configurations.
*   Node 2 and Node 3 should log establishing connections to Node 1.
*   Nodes will attempt to propose blocks in turn (every ~10 seconds by default). Look for logs related to block proposal, validation, and addition to the chain.

## III. Set up the Python CLI Wallet

1.  Open a new terminal window.
2.  Navigate to the `cli-wallet` directory within the project:
    ```bash
    cd path/to/empower1/cli-wallet
    ```
3.  Create a Python virtual environment and install dependencies:
    ```bash
    python3 -m venv venv
    source venv/bin/activate  # On Windows: venv\Scripts\activate
    pip install -r requirements.txt
    ```

## IV. Use the CLI Wallet

**1. Generate Wallets:**
Generate a wallet for sending transactions (e.g., `sender_wallet.pem`):
```bash
python main.py generate-wallet --outfile sender_wallet.pem --password senderpass
```
Note the public address displayed.

Generate another wallet to act as a recipient (e.g., `recipient_wallet.pem`):
```bash
python main.py generate-wallet --outfile recipient_wallet.pem
```
Note its public address. Let this be `<recipient_address_hex>`.

**2. Send a Transaction:**
Use the `sender_wallet.pem` to send funds to `<recipient_address_hex>`. Connect to one of the running nodes' HTTP RPC ports (e.g., Node 1 on `:18001`).

```bash
python main.py create-transaction \
    --from-wallet sender_wallet.pem --password senderpass \
    --to <recipient_address_hex_from_recipient_wallet> \
    --amount 100 \
    --fee 1 \
    --node-url http://localhost:18001
```
Replace `<recipient_address_hex_from_recipient_wallet>` with the actual address from `recipient_wallet.pem`.

**3. Observe System Behavior:**

*   **CLI Wallet Output:**
    *   Should show the created transaction details.
    *   Should indicate successful signing and submission.
    *   Should display the node's response (e.g., "Transaction accepted and broadcasted").

*   **Go Node Logs (Node connected to by wallet, e.g., Node 1 on :18001):**
    *   Log message for `RPC: /tx/submit - Received valid transaction ...`.
    *   Log message for adding the transaction to the mempool.
    *   Log message for broadcasting `MsgNewTransaction` to P2P peers.

*   **Other Go Node Logs (Nodes 2 and 3):**
    *   Log message for receiving `MsgNewTransaction` from peers.
    *   Log message for adding the transaction to their local mempools.

*   **Mempool Endpoint:**
    Before a block is mined containing the transaction, you can check the mempool of any node:
    ```bash
    curl http://localhost:18001/mempool
    curl http://localhost:18002/mempool
    # etc.
    ```
    The submitted transaction should be listed.

*   **Block Proposal and Mining:**
    *   Eventually, one of the validator nodes (whose turn it is) will propose a new block.
    *   The logs should show it fetching transactions from its mempool.
    *   The logs should show the block being created, validated, and added to the chain.
    *   Other nodes should receive this block via P2P (`MsgNewBlockProposal`), validate it, and add it to their chains.
    *   Logs on all nodes should show transactions being removed from the mempool after being included in a block.

*   **Mempool After Mining:**
    Check the mempool again. The transaction should no longer be present.
    ```bash
    curl http://localhost:18001/mempool
    ```

*   **Node Info Endpoint:**
    Check the `/info` endpoint on any node. The `current_height` should have increased.
    ```bash
    curl http://localhost:18001/info
    ```

## V. Further Tests (Optional)

*   Send multiple transactions from different wallets or to different recipients.
*   Stop one of the validator nodes and observe if the others continue to produce blocks.
*   Restart a node and see if it reconnects and syncs (note: full block syncing is not deeply implemented yet, but it should be aware of peers).
*   Try sending a transaction with an invalid signature (difficult to do with the current CLI, but a theoretical test).
*   Check wallet address display: `python main.py get-address --infile sender_wallet.pem --password senderpass`

This manual testing procedure helps verify the core functionality of transaction processing, P2P communication, and basic PoS consensus.`TESTING.md` created successfully. It provides a clear, step-by-step guide for:
*   Building the Go node.
*   Running a small network of three validator nodes with distinct ports and configurations.
*   Setting up the Python CLI wallet.
*   Using the CLI wallet to generate wallets and send a transaction.
*   Observing expected behavior in node logs and via HTTP RPC endpoints (`/mempool`, `/info`).

Next, I'll update the main `README.md` file.
