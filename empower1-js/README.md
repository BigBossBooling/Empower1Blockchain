# Empower1-JS SDK

A JavaScript SDK for interacting with the Empower1 Blockchain node.

## Features (Planned & In-Progress)

*   **Wallet Management:**
    *   Generate new P256/SECP256R1 key pairs.
    *   Load wallets from private key hex strings.
    *   Derive public keys and addresses from private keys.
*   **RPC Communication:**
    *   Submit transactions to the node.
    *   Query node information (e.g., status, mempool).
    *   Call smart contract view functions (via debug endpoints initially).
*   **Transaction Crafting & Signing:**
    *   Create various transaction types (standard, contract deploy, contract call).
    *   Implement canonical JSON hashing compatible with the Go node.
    *   Sign transactions using ECDSA with P256 curve (DER-encoded signatures).
*   **DID Utilities:** (To be aligned with main project DID features)
    *   Generate `did:key` strings from public keys.

## Setup

1.  **Clone the repository** or ensure this `empower1-js` directory is part of the main Empower1 project.
2.  **Navigate to the `empower1-js` directory:**
    ```bash
    cd path/to/empower1/empower1-js
    ```
3.  **Install Dependencies:**
    ```bash
    npm install
    ```
    This will install `axios`, `elliptic`, `js-sha256`, and any other dependencies listed in `package.json`.

## Usage (Conceptual - As SDK develops)

```javascript
// Example (Node.js environment)
const empower1 = require('empower1-js'); // Or import { RPCClient, Wallet, Transaction } from 'empower1-js';

async function main() {
    // Initialize RPC Client
    const rpc = new empower1.RPCClient('http://localhost:18001'); // Assuming node's debug/RPC port

    // Get Node Info
    try {
        const nodeInfo = await rpc.getNodeInfo();
        console.log("Node Info:", nodeInfo);
    } catch (e) {
        console.error("Error getting node info:", e.message);
    }

    // Wallet Operations
    const myWallet = empower1.Wallet.generateWallet();
    console.log("New Wallet:", myWallet);

    const loadedWallet = empower1.Wallet.loadWalletFromPrivateKey(myWallet.privateKey);
    console.log("Loaded Wallet Address:", loadedWallet.address);

    // Create & Sign Transaction (Simplified example)
    // const txCreator = new empower1.TransactionCreator(myWallet); // Hypothetical helper
    // const rawTx = txCreator.createStandardTransfer(loadedWallet.address, someOtherAddress, 100, 10);
    // const signedTx = await txCreator.signTransaction(rawTx);

    // const txPayload = empower1.Transaction.preparePayloadForSubmit(signedTx);
    // const txResult = await rpc.submitTransaction(txPayload);
    // console.log("Transaction submission result:", txResult);
}

// main().catch(console.error);
```

## Development

*   **Dependencies:**
    *   `axios`: For making HTTP requests to the Empower1 node.
    *   `elliptic`: For ECDSA cryptographic operations (SECP256R1/P256).
    *   `js-sha256`: For SHA256 hashing if Node.js `crypto` or browser `crypto.subtle` isn't used directly.
*   **Structure:**
    *   `src/index.js`: Main module export.
    *   `src/rpc.js`: RPC client logic.
    *   `src/wallet.js`: Wallet and key management.
    *   `src/transaction.js`: Transaction creation, hashing, and signing.
*   **Building:** Currently plain JavaScript. Future enhancements might include TypeScript and a bundler (Rollup/Webpack).

## Testing
Unit tests will be added (e.g., using Jest or Mocha) in the `tests/` directory.
```bash
# npm test
```
(The `test` script in `package.json` would need to be updated to run the chosen test framework).
