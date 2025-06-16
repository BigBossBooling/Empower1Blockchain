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

## VI. Smart Contract Testing (Deploy & Call `simple_storage.wasm`)

This section describes how to test deploying and interacting with the `simple_storage.wasm` contract. These steps assume you have a node running (e.g., Node 1 from Section II, with HTTP RPC on `:18001`).

**1. New Go Node Endpoints for Smart Contracts (Debug/Test)**

The Go node (`cmd/empower1d/main.go`) will be updated with the following new debug endpoints:

*   `POST /debug/deploy-contract`:
    *   Expects a JSON payload: `{"wasm_file_path": "path/to/your.wasm", "deployer_address": "hex_encoded_pubkey_of_deployer"}`
    *   Reads the WASM file, creates a deployment transaction, and processes it directly (stores code, generates contract address).
    *   Returns JSON: `{"message": "Contract deployed", "contract_address": "new_hex_address", "tx_id": "deployment_tx_id"}`
*   `POST /debug/call-contract`:
    *   Expects JSON: `{"contract_address": "hex_address", "function_name": "func_name", "arguments": [], "gas_limit": 1000000}`
        *   `arguments`: For this test, it will be a list of simple values that the Go handler will try to pass to the WASM function. For `simple_storage.wasm` functions:
            *   `set(key: string, value: string)`: `arguments` might be `["myKey", "myValue"]` (Go handler will need to prepare these as ptr/len for WASM).
            *   `get(key: string)`: `arguments` might be `["myKey"]`.
            *   `log(message: string)`: `arguments` might be `["Log this!"]`.
            *   `init()`: `arguments` would be empty `[]`.
    *   Executes the contract function via `VMService.ExecuteContract`.
    *   Returns JSON: `{"message": "Contract called", "result": <result_from_vm>, "gas_consumed": <gas>, "logs": ["log_output_if_any"]}` (logs might appear in node console instead of response for simplicity).

**2. Compile `simple_storage.wasm` (if not already done)**
    ```bash
    # (Assuming assemblyscript is installed, e.g., via npm install -g assemblyscript)
    # cd contracts_src/simple_storage
    # ./build_wasm.sh
    # cd ../..
    ```
    The pre-compiled `simple_storage.wasm` is already checked into `contracts_src/simple_storage/out/`.

**3. Deploy `simple_storage.wasm`**

Use `curl` (or any HTTP client) to send a deployment request to one of your running nodes.
You'll need a "deployer address". For this test, you can use the address of a wallet generated by the Python CLI (e.g., from `sender_wallet.pem`).

Get deployer address:
```bash
# In cli-wallet directory:
# python main.py get-address --infile sender_wallet.pem --password senderpass
# Copy the Public Address output. Let this be <deployer_address_hex>.
```

Deploy the contract:
```bash
curl -X POST -H "Content-Type: application/json" \
    -d '{"wasm_file_path": "contracts_src/simple_storage/out/simple_storage.wasm", "deployer_address": "<deployer_address_hex>"}' \
    http://localhost:18001/debug/deploy-contract
```
*   Replace `<deployer_address_hex>` with the actual hex address.
*   Observe the node logs for deployment messages.
*   The response should include the new `contract_address`. Note this down (let's call it `<deployed_contract_address>`).

**4. Call the `init` function (optional, but good practice if defined)**
The `simple_storage.wasm` has an `init` function that sets an initial key.
```bash
curl -X POST -H "Content-Type: application/json" \
    -d '{"contract_address": "<deployed_contract_address>", "function_name": "init", "arguments": [], "gas_limit": 1000000}' \
    http://localhost:18001/debug/call-contract
```
*   Check node logs for "simple_storage contract initialized!" and "Storage set successfully for key: initial_key".

**5. Call `set` function**
```bash
curl -X POST -H "Content-Type: application/json" \
    -d '{"contract_address": "<deployed_contract_address>", "function_name": "set", "arguments_json": "[\"greeting\", \"Hello Empower1\"]", "gas_limit": 1000000}' \
    http://localhost:18001/debug/call-contract
```
*(Note: `arguments_json` is used here to pass a JSON string representing an array of strings. The Go handler for `/debug/call-contract` will need to parse this and then correctly prepare these string arguments for the WASM `set(key: string, value: string)` function, likely by writing them to WASM memory and passing pointers/lengths.)*

*   Observe node logs for:
    *   `CONTRACT_LOG (Addr: <deployed_contract_address_bytes>): set called with key: 'greeting', value: 'Hello Empower1'`
    *   `CONTRACT_LOG (Addr: <deployed_contract_address_bytes>): Storage set successfully for key: greeting`

**6. Call `get` function**
```bash
curl -X POST -H "Content-Type: application/json" \
    -d '{"contract_address": "<deployed_contract_address>", "function_name": "get", "arguments_json": "[\"greeting\"]", "gas_limit": 1000000}' \
    http://localhost:18001/debug/call-contract
```
*   The JSON response should contain the result from the contract.
    ```json
    {
        "message": "Contract called",
        "result": "<pointer_to_string_in_wasm_or_actual_string_if_host_reads_it>",
        "gas_consumed": "<some_amount>",
        "logs": [ /* ... logs from contract execution via host_log_message ... */ ]
    }
    ```
    (The exact format of "result" for a string returned from WASM needs careful handling on the Go side when sending the JSON response. It might be a pointer, or the Go host function wrapper might read the string from WASM memory and include it directly.)
*   Observe node logs for:
    *   `CONTRACT_LOG (Addr: <deployed_contract_address_bytes>): get called with key: 'greeting'`
    *   `CONTRACT_LOG (Addr: <deployed_contract_address_bytes>): Calling blockchain_get_storage with buffer capacity: 256`
    *   `CONTRACT_LOG (Addr: <deployed_contract_address_bytes>): blockchain_get_storage returned actual_value_len: <length_of_HelloEmpower1>`
    *   `CONTRACT_LOG (Addr: <deployed_contract_address_bytes>): Value retrieved for key 'greeting': 'Hello Empower1'`

**7. Call `get` for the key set by `init`**
```bash
curl -X POST -H "Content-Type: application/json" \
    -d '{"contract_address": "<deployed_contract_address>", "function_name": "get", "arguments_json": "[\"initial_key\"]", "gas_limit": 1000000}' \
    http://localhost:18001/debug/call-contract
```
*   Check response and logs for "initial_value_set_during_init".

This provides a basic flow for testing contract deployment and interaction via new debug endpoints. Proper argument encoding/decoding between JSON, Go, and WASM memory for string types will be the main challenge in implementing the Go-side handlers.

## VII. Smart Contract Testing Details

This section expands on testing specific smart contracts deployed to the node. Ensure you have a node running (e.g., Node 1 from Section II, HTTP RPC on `:18001`) and the Python CLI wallet is set up.

### A. Testing `simple_storage.wasm`

The `simple_storage.wasm` contract allows setting and getting a key-value pair (both strings).

**1. Deploy `simple_storage.wasm`:**
   Use the `/debug/deploy-contract` endpoint. You'll need a deployer address (hex public key). You can get one from a CLI-generated wallet:
   ```bash
   # In cli-wallet directory:
   # python main.py get-address --infile sender_wallet.pem
   # (assuming sender_wallet.pem exists, or generate one)
   # Let <deployer_address_hex> be the output.
   ```
   Deploy:
   ```bash
   curl -X POST -H "Content-Type: application/json" \
       -d '{"wasm_file_path": "contracts_src/simple_storage/out/simple_storage.wasm", "deployer_address": "<deployer_address_hex>"}' \
       http://localhost:18001/debug/deploy-contract
   ```
   Note the returned `<simple_storage_contract_address>`.

**2. Call `init()` function:**
   The `simple_storage` contract has an `init()` function that sets an initial key.
   ```bash
   curl -X POST -H "Content-Type: application/json" \
       -d '{"contract_address": "<simple_storage_contract_address>", "function_name": "init", "arguments_json": "[]", "gas_limit": 1000000}' \
       http://localhost:18001/debug/call-contract
   ```
   Check node logs for: `CONTRACT_LOG (Addr: ...): simple_storage contract initialized!` and `CONTRACT_LOG (Addr: ...): Storage set successfully for key: initial_key`.

**3. Call `set(key: string, value: string)`:**
   ```bash
   curl -X POST -H "Content-Type: application/json" \
       -d '{"contract_address": "<simple_storage_contract_address>", "function_name": "set", "arguments_json": "[\"greeting\", \"Hello Empower1\"]", "gas_limit": 1000000}' \
       http://localhost:18001/debug/call-contract
   ```
   *   **Argument Handling Note:** The Go debug endpoint `handleDebugCallContract` currently has simplified string argument marshalling. It passes Go strings to the VM, which may not be correctly interpreted by AssemblyScript `string` parameters without a proper ABI or memory management functions exported by the WASM module (like `__new`, `__pin`). This call might show errors or unexpected behavior in the contract logs for the arguments themselves, but the host function calls from within `set` should still be attempted.
   *   Observe node logs for contract logs related to `set` and `blockchain_set_storage`.

**4. Call `get(key: string)`:**
   *   **Get "initial_key":**
       ```bash
       curl -X POST -H "Content-Type: application/json" \
           -d '{"contract_address": "<simple_storage_contract_address>", "function_name": "get", "arguments_json": "[\"initial_key\"]", "gas_limit": 1000000}' \
           http://localhost:18001/debug/call-contract
       ```
       Observe node logs for contract logs showing the retrieved value "initial_value_set_during_init". The JSON response's `contract_result` field will likely be an `int32` pointer to the string in WASM memory. The actual string value will be visible in the contract logs via `host_log_message`.

   *   **Get "greeting":**
       ```bash
       curl -X POST -H "Content-Type: application/json" \
           -d '{"contract_address": "<simple_storage_contract_address>", "function_name": "get", "arguments_json": "[\"greeting\"]", "gas_limit": 1000000}' \
           http://localhost:18001/debug/call-contract
       ```
       Observe logs for "Hello Empower1".

### B. Testing `did_registry.wasm`

The `did_registry.wasm` contract allows registering and querying DID document information.

**1. Deploy `did_registry.wasm`:**
   Similar to `simple_storage`, use a deployer address.
   ```bash
   curl -X POST -H "Content-Type: application/json" \
       -d '{"wasm_file_path": "contracts_src/did_registry/out/did_registry.wasm", "deployer_address": "<deployer_address_hex>"}' \
       http://localhost:18001/debug/deploy-contract
   ```
   Note the returned `<did_registry_contract_address>`.

**2. Call `init()` function (Optional but good practice):**
   Use the `/debug/call-contract` endpoint to call the `init` function of your deployed `DIDRegistry` contract.
   ```bash
   curl -X POST -H "Content-Type: application/json" \
       -d '{"contract_address": "<did_registry_contract_address>", "function_name": "init", "arguments_json": "[]", "gas_limit": 1000000}' \
       http://localhost:18001/debug/call-contract
   ```
   Check node logs for: `CONTRACT_LOG (Addr: ...): DIDRegistry contract initialized.`

**3. Prepare DID and Wallet (Python CLI Wallet):**
   *   **Generate a wallet** that will own the DID. If you already have one (e.g., `my_wallet.pem` with password `mypassword`), you can use that.
       ```bash
       # In cli-wallet directory (ensure venv is active)
       python main.py generate-wallet --outfile did_owner_wallet.pem --password didownerpass
       ```
   *   **Generate the `did:key` string** for this wallet:
       ```bash
       python main.py did generate --wallet did_owner_wallet.pem --password didownerpass
       ```
       This will print the `did:key` string (e.g., `did:key:zQ3s...`). Copy this string and let's call it `<owner_did_key_string>`.

**4. Register a DID Document (using Python CLI Wallet):**
   Now, use the `did register-document` command to call the `registerDIDDocument` function on your deployed smart contract.
   ```bash
   # In cli-wallet directory
   python main.py did register-document \
       --wallet did_owner_wallet.pem --password didownerpass \
       --did "<owner_did_key_string>" \
       --doc-hash "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899" \
       --doc-uri "ipfs://QmYourDocumentCID" \
       --contract-address "<did_registry_contract_address>" \
       --node-url http://localhost:18001
   ```
   *   Replace `<owner_did_key_string>` with the DID generated in the previous step.
   *   Replace `<did_registry_contract_address>` with the address from step B.1.
   *   **Observe Node Logs:**
        *   You should see logs related to the `/tx/submit` endpoint receiving the contract call transaction.
        *   Then, logs from the `VMService` executing the contract.
        *   Crucially, look for logs from the `DIDRegistry` contract itself (via `host_log_message`):
            *   "registerDIDDocument called for DID: <owner_did_key_string>..."
            *   "Caller address from host: <hex_public_key_of_did_owner_wallet>"
            *   "DID derived by host from caller's pubkey: <owner_did_key_string>" (this should match the `--did` argument)
            *   "Authentication successful for DID: <owner_did_key_string>"
            *   "DID Document info registered successfully..."
        *   Look for the `CONTRACT_EVENT` log: `Topic: 'DIDDocumentRegistered', Data: '{ "did": "<owner_did_key_string>", ... }'`

**5. Get DID Document Info (using Python CLI Wallet):**
   Use the `did get-info` command to query the information you just registered.
   ```bash
   # In cli-wallet directory
   python main.py did get-info \
       --did "<owner_did_key_string>" \
       --contract-address "<did_registry_contract_address>" \
       --node-url http://localhost:18001
   ```
   *   **CLI Output:** The command should print the JSON string containing the document hash and URI that you registered:
     `"{ "document_hash": "aabbccddeeff00112233445566778899aabbccddeeff00112233445566778899", "document_location_uri": "ipfs://QmYourDocumentCID" }"`.
   *   **Observe Node Logs:** You'll see logs related to the `/debug/call-contract` endpoint being used by the CLI for this query, and contract logs from `getDIDDocumentInfo`.

**6. Test `getDIDDocumentHash` and `getDIDDocumentURI` (Optional - via curl):**
   You can also test the individual getter functions using the `/debug/call-contract` endpoint directly with `curl` as described previously in this file (Section VII.A), by changing `function_name` and `arguments_json`. The contract logs will show the retrieved hash or URI.

This provides a thorough test of the DID registration and resolution flow using the smart contract and CLI wallet.

## VIII. Multi-Signature Transaction Testing (CLI Wallet & Node)

This section outlines manual steps to test the M-of-N multi-signature functionality. It assumes you have at least one Go node running (e.g., Node 1 from Section II, with HTTP RPC on `:18001`) and the Python CLI wallet is set up (Section III).

**1. Generate Individual Signer Wallets:**

If you haven't already, generate a few individual wallets that will act as co-signers. For a 2-of-3 multi-sig, you'll need three:
```bash
# In cli-wallet directory
python main.py generate-wallet --outfile signerA.pem --password passA
python main.py generate-wallet --outfile signerB.pem --password passB
python main.py generate-wallet --outfile signerC.pem --password passC
```
Note down their public addresses. Let's say they are:
*   `signerA_address_hex`
*   `signerB_address_hex`
*   `signerC_address_hex`

Also, create a recipient wallet (or use an existing address):
```bash
python main.py generate-wallet --outfile recipientMulti.pem
# Note its address: <recipient_for_multisig_hex>
```

**2. Create Multi-Signature Configuration:**

Create a 2-of-3 multi-sig configuration using the public keys of `signerA`, `signerB`, and `signerC`.
The `multisig create-config` command needs the wallet *files* to extract public keys.
```bash
python main.py multisig create-config -m 2 \
    --signer-wallet signerA.pem --signer-password passA \
    --signer-wallet signerB.pem --signer-password passB \
    --signer-wallet signerC.pem --signer-password passC \
    --outfile 2of3_config.json
```
*   This will create `2of3_config.json`.
*   Note the `multisig_address_hex` (Multi-Sig Identifier) printed by the command. This is the "sender" address for the multi-sig transaction.

**3. Initiate the Multi-Signature Transaction:**

The multi-sig entity (identified by `multisig_address_hex`) will send 75 units to `<recipient_for_multisig_hex>`.
```bash
python main.py multisig initiate-tx \
    --config 2of3_config.json \
    --to <recipient_for_multisig_hex> \
    --amount 75 \
    --fee 2 \
    --outfile pending_multisig_transfer.json
```
*   This creates `pending_multisig_transfer.json`. Inspect its contents. It should include the transaction details, M, N keys, and an empty `signers` list. The `id_hex` (transaction hash to be signed) should be present.

**4. Sign the Transaction (Collect Signatures):**

*   **Signer A signs:**
    ```bash
    python main.py multisig sign-tx \
        --pending-tx pending_multisig_transfer.json \
        --wallet signerA.pem --password passA \
        --outfile pending_multisig_transfer_signed_A.json
    ```
    Inspect `pending_multisig_transfer_signed_A.json`. It should now have one entry in the `signers` list.

*   **Signer B signs (using the output from Signer A):**
    ```bash
    python main.py multisig sign-tx \
        --pending-tx pending_multisig_transfer_signed_A.json \
        --wallet signerB.pem --password passB \
        --outfile pending_multisig_transfer_signed_AB.json
    ```
    Inspect `pending_multisig_transfer_signed_AB.json`. It should now have two signatures in the `signers` list (from signerA and signerB).

**5. Broadcast the Multi-Signature Transaction:**

Now that 2 out of 3 required signatures are collected, broadcast the transaction.
```bash
python main.py multisig broadcast-tx \
    --signed-tx pending_multisig_transfer_signed_AB.json \
    --node-url http://localhost:18001
    # Adjust node URL if necessary
```

**6. Observe System Behavior:**

*   **CLI Wallet Output:**
    *   `multisig-broadcast-tx` should report successful submission.
*   **Go Node Logs (e.g., Node on :18001):**
    *   Log for `RPC: /tx/submit - Received valid transaction ... (Type: standard)`. The type might still be "standard" if the multi-sig fields are additive to a standard tx structure.
    *   Log messages indicating successful multi-signature validation.
    *   Transaction added to the mempool.
    *   P2P broadcast of the transaction.
*   **Other Go Nodes:**
    *   Receive the multi-sig transaction via P2P.
    *   Validate it (including multi-sig verification).
    *   Add to their mempools.
*   **Mempool & Block Progression:**
    *   The transaction should appear in the `/mempool` endpoint of nodes.
    *   Eventually, a validator should propose a block including this multi-sig transaction.
    *   The transaction should be removed from mempools after being mined.
    *   Block height should increase on the `/info` endpoint.

**Further Multi-Sig Test Cases (Conceptual):**
*   Attempt to broadcast with insufficient signatures (e.g., only `pending_multisig_transfer_signed_A.json`). The CLI might warn, and the node should reject it.
*   Attempt to sign with an unauthorized wallet. The `multisig-sign-tx` command should fail.
*   Attempt to add the same signature twice. `add_signature` in Python should be idempotent or prevent it.
*   Create a config with M > N (e.g., 3-of-2). `multisig create-config` should fail.
*   Tamper with `pending_multisig_transfer.json` (e.g., change amount) after the first signature and then try to add a second signature. The second signer's client (if it re-calculates the hash) or the node should detect a signature mismatch if the hash signed by the first signer doesn't match the hash of the (tampered) current content. (Current Python `add_signature` re-calculates hash and compares with stored `id_hex`).
*   Building the Go node.
*   Running a small network of three validator nodes with distinct ports and configurations.
*   Setting up the Python CLI wallet.
*   Using the CLI wallet to generate wallets and send a transaction.
*   Observing expected behavior in node logs and via HTTP RPC endpoints (`/mempool`, `/info`).

Next, I'll update the main `README.md` file.
