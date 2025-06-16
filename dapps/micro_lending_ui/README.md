# Empower1 Micro-Lending DApp - Basic UI

This is a basic HTML/CSS/JavaScript user interface for interacting with the `MicroLendingPool` smart contract deployed on an Empower1 blockchain node. It utilizes the `empower1-js` SDK for wallet operations, transaction creation, and communication with the node.

## Features

*   Connect to a specified Empower1 node and view basic node information.
*   Load a user wallet by pasting a private key (hexadecimal format).
*   Display the current user's wallet address.
*   Set the address of the deployed `MicroLendingPool` smart contract.
*   Interact with the smart contract:
    *   Deposit funds into the pool.
    *   Withdraw deposited funds.
    *   Request a loan.
    *   Repay a loan.
    *   View total pool balance.
    *   View the current user's deposited balance and loan status (conceptual, as direct string return from WASM for complex objects is simplified).
*   Display status messages and errors.

## Prerequisites

*   A running Empower1 blockchain node with the `MicroLendingPool` smart contract deployed. You will need the HTTP RPC URL of this node and the deployed contract's address.
*   A modern web browser (if running outside Electron initially) or Electron for the intended setup.
*   The `empower1-js` SDK files (located in `js/lib/empower1-sdk/`).

## Setup and Running

**1. SDK Placement:**
   This DApp UI assumes the `empower1-js` SDK files (`wallet.js`, `transaction.js`, `rpc.js`, `index.js`) have been copied into the `dapps/micro_lending_ui/js/lib/empower1-sdk/` directory. This is a temporary measure due to the lack of a build/bundling process for the SDK in this phase.

**2. Open `index.html`:**
   *   **Directly in a browser (for basic testing, with caveats):**
        *   Open the `index.html` file in a web browser.
        *   **Important Note on `require`:** The SDK files use `require()` (CommonJS modules). Browsers do not support this natively. For the DApp to function in a browser without a build step (like Webpack/Browserify), the SDK scripts would need to be modified to expose their functionalities globally (e.g., by assigning to `window.MySDKComponent = ...`). The current `app.js` attempts to use them as if they are globally available after their script tags run, which is a simplification. Some browsers might also have issues with `elliptic` or `js-sha256` without these being bundled or specifically browser-compatible versions.
   *   **With Electron (Recommended way to run as intended for future development):**
        1.  Ensure you have Electron installed globally or locally in a project that can run this DApp.
        2.  Conceptually, you would point Electron to a main process file that loads this `index.html`. (This DApp itself isn't a full Electron project yet, but designed to be part of one).
        3.  If you have the main `gui-wallet` Electron project set up (from Phase 3.1 Part 1), you could potentially adapt its `main.js` to load this DApp's `index.html` for testing, or run a separate minimal Electron instance.
        4.  **For this DApp to work as intended with `require` statements within the SDK files, it would typically need to be part of an Electron project where `nodeIntegration` is carefully managed (ideally `false` with a preload script exposing SDK functions, or `true` for simpler cases but less security).** The `app.js` comments highlight these considerations.

**3. Using the DApp UI:**

*   **Connect to Node:**
    *   Enter the HTTP RPC URL of your running Empower1 node (e.g., `http://localhost:18001`).
    *   Click "Connect to Node & Get Info". Node information should appear if successful.
*   **Load Wallet:**
    *   Paste your 64-character hexadecimal private key into the input field.
    *   Click "Load Wallet". Your address will be displayed.
*   **Set Contract Address:**
    *   Paste the hexadecimal address of the deployed `MicroLendingPool` smart contract.
*   **Refresh Pool Data:**
    *   Click "Refresh Pool Data" to attempt to fetch `poolBalance` and `userInfo` from the contract. Note that displaying complex returned data (like the JSON string from `getUserInfo`) is simplified in this UI version and might show raw pointers or require the Go debug endpoint to return more processed data.
*   **Perform Actions:**
    *   Use the "Deposit Funds," "Withdraw Funds," "Request Loan," and "Repay Loan" sections. Enter an amount and click the respective button.
    *   These actions will:
        1.  Create a transaction object using the loaded wallet and contract details.
        2.  Sign the transaction.
        3.  Submit it to the node via the `/tx/submit` endpoint.
    *   Observe status messages and any errors.
    *   Check the Empower1 node's console output for contract logs and transaction processing messages.

## Development Notes

*   **SDK Dependency:** This UI relies heavily on the `empower1-js` SDK being correctly placed and functional. The SDK's `require` statements for `elliptic`, `axios`, `js-sha256`, and `buffer` mean it's primarily designed for a Node.js-like environment or one provided by Electron's main/preload processes, or a bundled browser environment.
*   **Error Handling:** Basic error messages are displayed.
*   **Argument Marshalling for Contract Calls:** Passing arguments (especially strings) to smart contract functions from JavaScript to WASM requires careful handling of memory and types. `app.js` currently prepares arguments as a JSON string encoded to UTF-8 bytes for `arguments_bytes` in the transaction. The Go node's debug endpoint for contract calls (`/debug/call-contract`) and the `VMService` would need to correctly interpret this for the specific WASM function being called. For functions expecting direct numeric types (like `u64` for amounts in the lending pool), these might pass through more easily if the JS SDK and Go VM handle them.
*   **Read-only Calls:** Read-only contract functions like `getPoolBalance` and `getUserInfo` are currently called using the node's `/debug/call-contract` endpoint via `rpcClient.callContract()`. This is a simplification for this UI; a production DApp might use a different mechanism or still send transactions for queries if required by the chain.

This UI serves as a basic demonstration and testing tool for the `MicroLendingPool` contract and the `empower1-js` SDK.
```
