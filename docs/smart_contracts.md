# Empower1 Smart Contracts

This document provides an overview of the smart contract capabilities within the Empower1 blockchain, including the chosen technologies, how to write and deploy contracts, the available host function API, and the gas model.

## 1. Overview

Empower1 enables smart contract functionality using WebAssembly (WASM) as its execution engine. This allows developers to write contracts in languages that compile to WASM, such as AssemblyScript, Rust, or C++. Contracts run in a sandboxed environment and can interact with the blockchain state through a defined set of host functions.

## 2. Chosen Technologies

### 2.1. Virtual Machine (VM): WebAssembly (WASM)
*   **Sandboxing:** Ensures safe execution of untrusted contract code.
*   **Efficiency:** Designed for near-native performance.
*   **Portability & Ecosystem:** Compilable from various languages with growing tooling support. Widely adopted in the blockchain space.

### 2.2. WASM Runtime (Go): Wasmer-Go
*   `Wasmer-Go` (github.com/wasmerio/wasmer-go) is used for integrating WASM execution into the Go-based Empower1 node.
*   It allows instantiation of WASM modules, calling exported functions, and providing host functions from Go to the WASM environment.
*   *Alternative:* `Wasmtime-Go` is another strong candidate.

### 2.3. Smart Contract Language: AssemblyScript
*   AssemblyScript (assemblyscript.org) is the initially recommended language.
*   **TypeScript-like syntax:** Accessible to many developers.
*   **Compiles to WASM:** Specifically designed for WASM.
*   **Static Typing & GC:** Offers strong typing and includes a WASM-suitable garbage collector.

## 3. Writing a Basic Smart Contract (AssemblyScript)

Smart contracts are typically written as classes or modules with exported functions that can be called by transactions or other contracts. They interact with the blockchain via imported host functions.

**Example Structure (`my_contract.ts`):**
```typescript
// Import host functions (declarations)
@external("env", "host_log_message")
declare function host_log_message(ptr: i32, len: i32): void;

@external("env", "blockchain_set_storage")
declare function blockchain_set_storage(keyPtr: i32, keyLen: i32, valPtr: i32, valLen: i32): i32;

@external("env", "blockchain_get_storage")
declare function blockchain_get_storage(keyPtr: i32, keyLen: i32, retBufPtr: i32, retBufLen: i32): i32;

// Helper for logging
function log(message: string): void {
  let SmessageBytes = String.UTF8.encode(message);
  // @ts-ignore: dataStart
  host_log_message(messageBytes.dataStart as i32, messageBytes.byteLength);
}

// Exported function
export function myContractFunction(inputValue: i32): i32 {
  log("myContractFunction called with: " + inputValue.toString());

  // Example: Store something
  let key = "my_key";
  let value = "data_for_" + inputValue.toString();
  let keyBytes = String.UTF8.encode(key);
  let valueBytes = String.UTF8.encode(value);
  // @ts-ignore
  blockchain_set_storage(keyBytes.dataStart as i32, keyBytes.byteLength, valueBytes.dataStart as i32, valueBytes.byteLength);

  return inputValue * 2;
}

// Optional init function
export function init(): void {
  log("MyContract initialized!");
}
```

Refer to `contracts_src/simple_storage.ts` and `contracts_src/did_registry.ts` for more complete examples.

**Compilation:**
Use the AssemblyScript compiler (`asc`):
```bash
asc my_contract.ts -b my_contract.wasm --optimize --runtime stub --exportRuntime
```

## 4. Host Functions API (Go -> WASM)

Smart contracts can call these functions provided by the Empower1 node (Go environment). They are typically imported under the `"env"` namespace. Memory pointers (`*_ptr`) and lengths (`*_len`) refer to the WASM module's linear memory.

*(For detailed signatures and error codes, see `internal/vm/host_functions.go` and `internal/vm/gas.go`)*

### Storage:
*   **`blockchain_set_storage(key_ptr: i32, key_len: i32, value_ptr: i32, value_len: i32) -> i32 (err_code)`**
    *   Sets a key-value pair in the contract's private storage. Returns `0` (ErrCodeSuccess) on success.
*   **`blockchain_get_storage(key_ptr: i32, key_len: i32, ret_buf_ptr: i32, ret_buf_len: i32) -> i32 (actual_value_len)`**
    *   Gets a value by key. Writes value to `ret_buf_ptr`.
    *   Returns actual length of the stored value. If key not found, returns `0`. If `actual_value_len > ret_buf_len`, the buffer was too small and only `ret_buf_len` bytes are copied. The contract must check this.

### Logging & Events:
*   **`host_log_message(message_ptr: i32, message_len: i32): void`**
    *   Logs a message from the contract (primarily for debugging).
*   **`blockchain_emit_event(topic_ptr: i32, topic_len: i32, data_ptr: i32, data_len: i32): void`**
    *   Emits an event with a topic and data. Currently logs to the node console.

### Caller & Context Information:
*   **`blockchain_get_caller_public_key(ret_buf_ptr: i32, ret_buf_len: i32) -> i32 (actual_key_len)`**
    *   Writes the raw uncompressed public key (65 bytes for P256) of the transaction signer (caller) into `ret_buf_ptr`.
    *   Returns actual length written or error code.
*   **`blockchain_generate_did_key(pubkey_ptr: i32, pubkey_len: i32, ret_buf_ptr: i32, ret_buf_len: i32) -> i32 (actual_did_len)`**
    *   Takes raw public key bytes from WASM memory.
    *   Generates the `did:key:z...` string.
    *   Writes the DID string to `ret_buf_ptr`. Returns its length or error code.
*   **`blockchain_get_caller_address(ret_buf_ptr: i32, ret_len: i32) -> i32 (actual_len)`** (Less used if `_get_caller_public_key` is preferred)
    *   Writes the hex-encoded string address of the caller to `ret_buf_ptr`.
*   **`blockchain_get_block_timestamp() -> u64`** (Stubbed)
*   **`blockchain_get_block_height() -> u64`** (Stubbed, not yet exposed in `vm.go`)

### Financial:
*   **`blockchain_get_balance(address_ptr: i32, address_len: i32) -> u64`** (Stubbed)
*   **`blockchain_send_funds(to_address_ptr: i32, to_address_len: i32, amount: u64) -> i32 (err_code)`** (Stubbed)

## 5. Gas Model

Execution of smart contracts consumes "gas" to manage resource usage.

*   **Gas Costs:**
    *   **Host Function Calls:** Each host function has a base gas cost, and some charge additional gas based on data size (e.g., bytes stored/read, log message length). Examples (from `internal/vm/host_functions.go`):
        *   `blockchain_log_message`: Base 10 gas + 1 gas/byte.
        *   `blockchain_set_storage`: Base 100 gas + 1 gas/byte (key+value).
        *   `blockchain_get_storage`: Base 50 gas + 1 gas/byte (key+value retrieved).
    *   **WASM Execution:** A flat `baseWasmExecutionCost` (e.g., 100 gas) is currently deducted per `ExecuteContract` call as a placeholder for instruction-level metering.
*   **Metering:**
    *   Gas is tracked via a `GasTank` in `internal/vm/gas.go`.
    *   Host functions consume gas from this tank.
    *   If `gasLimit` (provided by the calling transaction) is exceeded, execution halts with an `ErrOutOfGas`.
    *   Fine-grained WASM instruction-level gas metering using Wasmer-Go's middleware is a future enhancement.
*   **Gas in Transactions:** Contract call transactions must specify a `GasLimit`. (Future: `GasPrice` for fee markets).

## 6. Contract Deployment

*   **Transaction Type:** A transaction with `TxType` set to `core.TxContractDeploy` and `ContractCode` field containing the WASM bytecode.
*   **Process (Current - Simplified via Debug Endpoint):**
    1.  User submits WASM bytecode via `/debug/deploy-contract` endpoint (see `TESTING.md`).
    2.  Node generates a contract address (currently placeholder logic: `state.GenerateNewContractAddress`).
    3.  Node stores the WASM code in an in-memory store (`state.DeployedContractsStore`) associated with the new address.
*   **Future:** Deployment will be a standard transaction type processed through consensus, incurring gas, and potentially running an `init` function.

## 7. Calling Contract Functions

*   **Transaction Type:** A transaction with `TxType` set to `core.TxContractCall`, specifying `TargetContractAddress`, `FunctionName`, and serialized `Arguments`.
*   **Process (Current - Simplified via Debug Endpoint):**
    1.  User submits call details via `/debug/call-contract` (see `TESTING.md`).
    2.  Node retrieves WASM code for the `TargetContractAddress`.
    3.  `VMService.ExecuteContract` is called:
        *   The caller's public key (for `blockchain_get_caller_public_key`) is passed to the VM environment.
        *   Gas limit is enforced.
        *   The specified function is executed.
        *   Return values and state changes (via host functions modifying `state.GlobalContractStore`) are handled.
    *   **Argument Marshalling:** Passing complex arguments (like strings or arrays) from Go to WASM exported functions requires careful marshalling (allocating memory in WASM via an exported `__new` function, writing data, passing pointers). This is simplified in current debug endpoints, and robust marshalling is future work.
*   **Future:** Calls will be standard transactions, gas will be fully accounted for, and state changes will be part of the blockchain's atomic state transition.

## 8. Contract Code Storage (Recap)
*   Deployed WASM bytecode is stored by the node (currently in an in-memory map: `state.DeployedContractsStore`).
*   Retrieved by contract address when a call is made.
```
