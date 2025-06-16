# Empower1 Smart Contract Development Guide

Welcome to the Empower1 Smart Contract Development Guide! This document provides everything you need to start writing, testing, and deploying smart contracts on the Empower1 blockchain.

## 1. Introduction

Empower1 utilizes WebAssembly (WASM) as the execution engine for its smart contracts. This allows developers to write contracts in a variety of languages that can compile to WASM, with AssemblyScript being the primary recommended language for its ease of use and strong WASM support.

Smart contracts on Empower1 run in a sandboxed environment and interact with the blockchain's state and functionalities through a well-defined set of "host functions" provided by the node.

**Key Features:**
*   **WASM-based:** Secure, efficient, and portable.
*   **AssemblyScript:** TypeScript-like syntax for easy development.
*   **Host Function API:** Enables rich interaction with blockchain data and services.
*   **Gas Model:** Ensures fair resource usage and prevents abuse.

## 2. Setting up Your AssemblyScript Environment

To develop smart contracts for Empower1 using AssemblyScript, you'll need Node.js and npm.

**1. Install Node.js and npm:**
   If you don't have them, download and install from [nodejs.org](https://nodejs.org/). This will also install npm (Node Package Manager).

**2. Install AssemblyScript Compiler (`asc`):**
   The AssemblyScript compiler can be installed globally or as a project dependency.
   ```bash
   npm install --save-dev assemblyscript
   # For global installation (less common for projects):
   # npm install -g assemblyscript
   ```
   It's recommended to install it as a dev dependency in your contract project (`package.json`).

**3. Install Testing Framework (`as-pect`):**
   `as-pect` is the recommended testing suite for AssemblyScript.
   ```bash
   npm install --save-dev @as-pect/cli @as-pect/core
   ```

**4. Initialize Your Project:**
   For a new contract project, navigate to your project directory and run:
   ```bash
   npm init -y
   npx asinit .  # Initializes a new AssemblyScript project structure
   ```
   This will set up a basic AssemblyScript project with `asconfig.json`, an `assembly/` directory for your `.ts` files, and a `tests/` directory.

## 3. Project Structure (Recommended)

A typical Empower1 AssemblyScript contract project might look like this:

```
my-empower1-contract/
├── assembly/                # AssemblyScript source files
│   ├── contracts/           # Your main contract logic
│   │   └── MyContract.ts
│   └── env/                 # Host function declarations (optional, can be in contract file)
│       └── index.ts         # (e.g., declare all host functions here)
├── build/                   # Compiled WASM files (debug and release)
├── node_modules/            # Project dependencies
├── tests/                   # as-pect tests
│   ├── MyContract.spec.ts
│   └── setup.js             # (Optional: as-pect global setup)
├── asconfig.json            # AssemblyScript compiler configuration
├── package.json             # Node.js project manifest
└── package-lock.json
└── README.md
```
Our [Boilerplate Project](#7-smart-contract-boilerplate) provides a ready-to-use version of this structure.

## 4. Writing Contracts (AssemblyScript Basics)

AssemblyScript's syntax is very similar to TypeScript.

### 4.1. Data Types
AssemblyScript supports various static types like `i8, u8, i16, u16, i32, u32, i64, u64, f32, f64, bool, string`, as well as complex types like classes, arrays, maps, etc. For blockchain interactions, you'll often deal with `string`, integer types, and byte arrays (`Uint8Array` or `ArrayBuffer`).

### 4.2. Importing Host Functions
Your contract will need to declare the host functions it intends to use. These are provided by the Empower1 node at runtime.
```typescript
// Example: In your contract file or a shared 'env.ts'
// The "env" string is the module name the host expects.

@external("env", "host_log_message")
declare function host_log_message(message_ptr: i32, message_len: i32): void;

@external("env", "blockchain_set_storage")
declare function blockchain_set_storage(key_ptr: i32, key_len: i32, value_ptr: i32, value_len: i32): i32;
// ... declare other host functions you need
```
Pointers (`ptr`) and lengths (`len`) refer to locations and sizes in the WASM module's linear memory.

### 4.3. String Handling
AssemblyScript strings are UTF-16. Host functions often expect UTF-8 byte arrays for strings.
```typescript
function log(message: string): void {
  let messageBytes = String.UTF8.encode(message); // Encode to UTF-8 ArrayBuffer
  // @ts-ignore: dataStart is a valid property on ArrayBuffer providing the pointer
  host_log_message(messageBytes.dataStart as i32, messageBytes.byteLength);
}
```

### 4.4. Persistent Storage
Use `blockchain_set_storage` and `blockchain_get_storage` for key-value persistence. Keys and values are typically strings or byte arrays.
```typescript
// Storing a string value
let myKey = "user_name";
let myValue = "Alice";
let keyBytes = String.UTF8.encode(myKey);
let valueBytes = String.UTF8.encode(myValue);
// @ts-ignore
let Kptr = keyBytes.dataStart as i32;
// @ts-ignore
let Vptr = valueBytes.dataStart as i32;
let success_code = blockchain_set_storage(Kptr, keyBytes.byteLength, Vptr, valueBytes.byteLength);
if (success_code != 0) { log("Failed to store 'user_name'"); }

// Retrieving a string value
let readBuffer = new Uint8Array(128); // Buffer to receive the value
// @ts-ignore
let Rptr = readBuffer.dataStart as i32;
let actualLen = blockchain_get_storage(Kptr, keyBytes.byteLength, Rptr, readBuffer.byteLength);

if (actualLen > 0 && actualLen <= readBuffer.byteLength) {
  let retrievedValue = String.UTF8.decode(readBuffer.buffer.slice(0, actualLen));
  log("Retrieved value: " + retrievedValue);
} else if (actualLen == 0) {
  log("Key not found.");
} else if (actualLen > readBuffer.byteLength) {
  log("Buffer too small to retrieve value. Required: " + actualLen.toString());
}
```

### 4.5. Exported Functions
Functions that you want to be callable from transactions or other contracts must be exported:
```typescript
export function myPublicFunction(arg1: string): string {
  // ... logic ...
  return "result";
}
```
An `init()` function can be exported and is often called once upon deployment.

### 4.6. Error Handling
Host functions return error codes (typically `0` for success). Your contract should check these. You can use `abort()` for unrecoverable errors in the contract, or return specific error codes from your public functions.

## 5. Available Host Functions (API)

The Empower1 node provides the following host functions, callable from the `"env"` module in your AssemblyScript contract. (Error codes are defined in `internal/vm/host_functions.go`).

*   **`host_log_message(message_ptr: i32, message_len: i32): void`**
    *   Logs a UTF-8 message from WASM memory. Useful for debugging.
    *   Gas: Base cost + per byte.

*   **`blockchain_set_storage(key_ptr: i32, key_len: i32, value_ptr: i32, value_len: i32): i32`**
    *   Stores `value` associated with `key` in the contract's private state.
    *   Both key and value are UTF-8 byte arrays from WASM memory.
    *   Returns `0` (ErrCodeSuccess) on success, or an error code.
    *   Gas: Base cost + per byte of key and value.

*   **`blockchain_get_storage(key_ptr: i32, key_len: i32, ret_buf_ptr: i32, ret_buf_len: i32): i32`**
    *   Retrieves value for `key`. Value is written to `ret_buf_ptr`.
    *   Returns actual length of the stored value.
    *   If key not found, returns `0`.
    *   If `ret_buf_len` is too small, only `ret_buf_len` bytes are copied, but the full actual length is still returned (contract must check `actual_len > ret_buf_len`).
    *   Returns negative error code for memory access issues.
    *   Gas: Base cost + per byte of key and value retrieved.

*   **`blockchain_get_caller_public_key(ret_buf_ptr: i32, ret_buf_len: i32) -> i32`**
    *   Writes the raw uncompressed public key (65 bytes for P256) of the transaction signer into `ret_buf_ptr`.
    *   Returns actual length written (e.g., 65) or an error code (e.g., if buffer too small, or if caller is not identifiable as a single public key).
    *   Gas: Base cost.

*   **`blockchain_generate_did_key(pubkey_ptr: i32, pubkey_len: i32, ret_buf_ptr: i32, ret_buf_len: i32) -> i32`**
    *   Reads raw public key bytes from WASM memory.
    *   Generates the full `did:key:z...` string using the node's standard scheme.
    *   Writes this DID string to `ret_buf_ptr`. Returns its length or error code.
    *   Gas: Base cost (includes crypto operations).

*   **`blockchain_emit_event(topic_ptr: i32, topic_len: i32, data_ptr: i32, data_len: i32): void`**
    *   Emits an event with a topic and data (UTF-8 strings/byte arrays from WASM). Currently logs to node console.
    *   Gas: Base cost + per byte of topic and data.

*   **Stubbed/Conceptual Host Functions (Functionality limited or not yet fully implemented):**
    *   `blockchain_get_caller_address(ret_buf_ptr: i32, ret_len: i32) -> i32`: Returns hex address of caller.
    *   `blockchain_get_balance(address_ptr: i32, address_len: i32) -> u64`: Returns balance of an account.
    *   `blockchain_send_funds(to_address_ptr: i32, to_address_len: i32, amount: u64) -> i32 (err_code)`: Transfers funds.
    *   `blockchain_get_block_timestamp() -> u64`: Timestamp of current block.
    *   `blockchain_get_block_height() -> u64`: Height of current block.

## 6. Compiling Contracts

Use the AssemblyScript compiler (`asc`):
```bash
# For a release build (optimized, smaller):
npx asc assembly/contracts/MyContract.ts -b build/release/MyContract.wasm --optimize --runtime stub --exportRuntime

# For a debug build (source maps, debug symbols):
npx asc assembly/contracts/MyContract.ts -b build/debug/MyContract.wasm --sourceMap --debug --runtime stub --exportRuntime
```
*   `assembly/contracts/MyContract.ts`: Path to your main contract file.
*   `-b build/release/MyContract.wasm`: Output WASM binary file.
*   `--optimize`: Apply optimizations.
*   `--runtime stub`: Use a minimal runtime suitable for blockchain environments where the host provides core functionalities.
*   `--exportRuntime`: Exports necessary runtime helper functions (like `__new`, `__pin`, `__unpin`, `__collect`) if your contract uses dynamic memory allocation for types like `string`, `Array`, `Map` passed to/from host or complex exports.

**`asconfig.json`:**
It's highly recommended to use an `asconfig.json` file at the root of your AssemblyScript project to manage compiler options:
```json
{
  "targets": {
    "debug": {
      "outFile": "build/debug/contract.wasm",
      "sourceMap": true,
      "debug": true
    },
    "release": {
      "outFile": "build/release/contract.wasm",
      "sourceMap": false,
      "optimizeLevel": 3,    // Aggressive optimizations
      "shrinkLevel": 0,      // Default shrink level
      "converge": false,     // Helps with code size
      "noAssert": true       // Remove assertion checks in release
    }
  },
  "options": {
    "bindings": "esm",       // Or "raw" / "legacy"
    "initialMemory": 1,      // Initial memory pages (1 page = 64KB)
    "maximumMemory": 10,     // Optional: max memory pages
    "runtime": "stub",       // Essential for most blockchain environments
    "exportRuntime": true    // Often needed for string/array marshalling with host
  }
}
```
With `asconfig.json`, you can compile using:
```bash
npx asc --target debug  # For debug build
npx asc --target release # For release build
```

## 7. Testing Contracts (`as-pect`)

`as-pect` is a popular testing framework for AssemblyScript.

**1. Setup:**
   Install `as-pect` as shown in section 2. Your `package.json` should have a test script:
   ```json
   "scripts": {
     "test": "asp --verbose"
   }
   ```
**2. Writing Tests (`tests/MyContract.spec.ts`):**
   ```typescript
   // In tests/MyContract.spec.ts
   import { MyContract, someFunction } from "../assembly/contracts/MyContract"; // Path to your compiled module or source

   // Mocking Host Functions for testing:
   // as-pect allows various ways to mock. A common way is to provide a mock object
   // to the TestContext or use `mockFunction`.

   let mockStorage = new Map<string, string>();
   let lastLoggedMessage = "";

   // Example of simple global mocks (more advanced mocking with as-pect is preferred)
   function mock_host_log_message(message: string): void { // Note: AS contract expects ptr/len
     lastLoggedMessage = message;
     // console.log("MOCK_LOG: " + message);
   }
   function mock_blockchain_set_storage(key: string, value: string): i32 {
     mockStorage.set(key, value);
     return 0; // Success
   }
   function mock_blockchain_get_storage(key: string): string | null {
     return mockStorage.get(key) || null;
   }

   describe("MyContract", () => {
     beforeEach(() => {
       mockStorage.clear();
       lastLoggedMessage = "";
       // For as-pect, you'd typically use its mocking utilities to replace imports.
       // e.g. replaceImports({ env: { host_log_message: mock_host_log_message, ... } });
       // This example is conceptual for how mocking works.
     });

     it("should initialize correctly", () => {
       // Assuming MyContract.init calls log and set_storage
       // You'd need to set up mocks that your compiled WASM can link to.
       // This usually involves as-pect's `instantiate` with mock imports.
       // const instance = instantiate<typeof MyContract>(fs.readFileSync("build/debug/MyContract.wasm"), {
       //   env: { /* mocked host functions */ }
       // });
       // instance.exports.init("test_init_val");
       // expect(lastLoggedMessage).toBe("...");
       // expect(mockStorage.get("some_key")).toBe("test_init_val");
       expect(true).toBe(true); // Placeholder
     });
   });
   ```
   Refer to the [as-pect documentation](https://as-pect.gitbook.io/) for detailed usage on mocking imports and writing tests.

**3. Running Tests:**
   ```bash
   npm test
   ```

## 8. Deployment & Interaction

### Deployment
*   **Current Method:** Use the debug HTTP endpoint `/debug/deploy-contract` on the Empower1 node.
    *   **Payload:** `{"wasm_file_path": "path/to/your/contract.wasm", "deployer_address": "hex_deployer_pubkey"}`
    *   See `TESTING.md` for `curl` examples.
*   **Future:** A dedicated `TxContractDeploy` transaction type will be processed through consensus.

### Calling Contract Functions
*   **Current Method (Debug):** Use the debug HTTP endpoint `/debug/call-contract`.
    *   **Payload:** `{"contract_address": "hex_addr", "function_name": "func_name", "arguments_json": "[\"arg1\", 20]", "gas_limit": 1000000}`
    *   `arguments_json`: A JSON string representing an array of arguments. The Go handler for this debug endpoint has simplified argument marshalling, especially for strings. Proper ABI encoding/decoding will be needed for robust calls.
    *   See `TESTING.md` for `curl` and Python CLI examples.
*   **Future:** A dedicated `TxContractCall` transaction type will be processed through consensus.

## 9. Gas Model (Recap)
*   Contract execution consumes gas. Provide `GasLimit` in call transactions.
*   Gas is charged for:
    *   A base fee for WASM execution (placeholder).
    *   Host function calls (base cost + data-dependent cost).
*   Execution halts with `ErrOutOfGas` if limit is exceeded.
*   See Section 5 of `docs/smart_contracts.md` (this document, previously `architecture.md`) for more details.

## 10. Security Best Practices (Basic)

*   **Input Validation:** Validate all inputs to public functions.
*   **State Management:** Be mindful of who can change state and how.
*   **Error Handling:** Check return codes from host functions. Use `abort()` or return errors for contract failures.
*   **Reentrancy:** (If applicable to your chain's model) Be cautious if contracts can call other contracts or themselves within a single transaction. Empower1's current host API doesn't directly facilitate complex reentrancy.
*   **Integer Overflows/Underflows:** Use AssemblyScript's safe types or perform checks if doing arithmetic that might overflow/underflow (e.g., `u64.addWrapped`).
*   **Gas Limits:** Be mindful of gas costs of operations to prevent functions from running out of gas unexpectedly.

## 11. Example Contracts

*   **`contracts_src/simple_contract/simple_contract.ts`**: A very basic contract demonstrating exported functions and a simple host call (`host_log_message`).
*   **`contracts_src/simple_storage/simple_storage.ts`**: Demonstrates using storage host functions (`blockchain_set_storage`, `blockchain_get_storage`) and logging.
*   **`contracts_src/did_registry/did_registry.ts`**: A more complex example showing use of multiple host functions for a DID registration system, including caller authentication logic.

This guide will be updated as the Empower1 smart contract platform evolves.

## 12. Smart Contract Boilerplate

To help you get started quickly with AssemblyScript development for Empower1, a boilerplate project is provided in the `contract_templates/assemblyscript_boilerplate/` directory of the main Empower1 repository.

This boilerplate includes:
*   A recommended project structure.
*   An example `MyContract.ts` demonstrating basic features.
*   A comprehensive `assembly/env/index.ts` file declaring all available host functions.
*   Pre-configured `asconfig.json` for compiler settings.
*   A `package.json` with scripts for building and testing (`as-pect`).
*   An example test suite in `tests/MyContract.spec.ts` showing host function mocking.
*   A dedicated `README.md` within the boilerplate directory explaining how to use it.

**To use the boilerplate:**
1.  Copy the `contract_templates/assemblyscript_boilerplate/` directory to a new location for your project.
2.  Navigate into your new project directory.
3.  Run `npm install` to set up dependencies.
4.  Follow the instructions in the boilerplate's `README.md` to start building and testing your smart contract.
