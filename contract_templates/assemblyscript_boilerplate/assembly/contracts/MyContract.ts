// assembly/contracts/MyContract.ts
// This is a boilerplate example of an Empower1 smart contract in AssemblyScript.

// Import host function declarations from the ../env/index.ts file
import {
    host_log_message,
    blockchain_set_storage,
    blockchain_get_storage,
    blockchain_get_caller_address
    // Import other host functions as needed by your contract, for example:
    // blockchain_get_caller_public_key,
    // blockchain_emit_event
} from "../env";

// --- Helper for logging (UTF-8 encoding handled) ---
// It's good practice to create helpers for common operations like logging or string conversions.
function log(message: string): void {
  let messageBytes = String.UTF8.encode(message);
  // @ts-ignore: dataStart is a property on ArrayBuffer in AssemblyScript providing the pointer
  host_log_message(messageBytes.dataStart as i32, messageBytes.byteLength);
}

// --- Helper for string to/from byte array for storage ---
// AssemblyScript strings are UTF-16 internally. Host storage likely expects/returns UTF-8 bytes.
function stringToBytesPtrLen(s: string): StaticArray<i32> {
    let sBytes = String.UTF8.encode(s);
    // @ts-ignore
    return [sBytes.dataStart as i32, sBytes.byteLength];
}

function bytesToString(buffer: ArrayBuffer, len: i32): string {
    if (len == 0) return "";
    return String.UTF8.decode(buffer.slice(0, len));
}

// Buffer for receiving caller address (reusable)
const CALLER_ADDRESS_BUFFER_CAPACITY = 130; // Max hex string for P256 pubkey (04 + 64 + 64)
let callerAddressBuffer = new Uint8Array(CALLER_ADDRESS_BUFFER_CAPACITY);
// @ts-ignore
let callerAddressBufPtr = callerAddressBuffer.dataStart as i32;

function getCaller(): string {
    let len = blockchain_get_caller_address(callerAddressBufPtr, CALLER_ADDRESS_BUFFER_CAPACITY);
    if (len > 0 && len <= CALLER_ADDRESS_BUFFER_CAPACITY) {
        return bytesToString(callerAddressBuffer.buffer, len);
    }
    log("Error: Could not get valid caller address, length: " + len.toString());
    return ""; // Or handle error appropriately
}

// --- Publicly Exported Contract Functions ---

/**
 * Initializes the contract. This function can be called once upon deployment.
 * Stores the deployer's address as the "owner" and an initial value.
 * @param initial_value A string value to store under a predefined key "my_key".
 */
export function init(initial_value: string): void {
    const caller = getCaller();
    log("MyContract.init called by: " + caller + " with initial_value: '" + initial_value + "'");

    let myKeyParams = stringToBytesPtrLen("my_key");
    let initialValueParams = stringToBytesPtrLen(initial_value);
    let ownerKeyParams = stringToBytesPtrLen("owner");
    let ownerValueParams = stringToBytesPtrLen(caller); // Store caller as owner

    let setResultMyKey = blockchain_set_storage(myKeyParams[0], myKeyParams[1], initialValueParams[0], initialValueParams[1]);
    if (setResultMyKey == 0) { // 0 is ErrCodeSuccess
        log("MyContract: Initial value stored successfully for 'my_key'.");
    } else {
        log("MyContract: Failed to store initial value for 'my_key'. Error code: " + setResultMyKey.toString());
    }

    let setResultOwner = blockchain_set_storage(ownerKeyParams[0], ownerKeyParams[1], ownerValueParams[0], ownerValueParams[1]);
    if (setResultOwner == 0) {
        log("MyContract: Owner stored successfully.");
    } else {
        log("MyContract: Failed to store owner. Error code: " + setResultOwner.toString());
    }
}

/**
 * A simple function that takes a name, logs it, retrieves a value from storage,
 * and returns a greeting.
 * @param name The name to greet.
 * @returns A greeting string.
 */
export function Greet(name: string): string {
    log("MyContract.Greet called with name: '" + name + "'");

    let keyParams = stringToBytesPtrLen("my_key");

    let valueBuffer = new Uint8Array(128); // Buffer to receive the stored value
    // @ts-ignore
    let valueBufPtr = valueBuffer.dataStart as i32;

    let actualLen = blockchain_get_storage(keyParams[0], keyParams[1], valueBufPtr, valueBuffer.byteLength);
    let storedValueStr: string = "[Error retrieving stored value]";

    if (actualLen == 0) { // Key not found
        storedValueStr = "Key 'my_key' not found.";
    } else if (actualLen > 0 && actualLen <= valueBuffer.byteLength) { // Value found and fits
        storedValueStr = bytesToString(valueBuffer.buffer, actualLen);
    } else if (actualLen > valueBuffer.byteLength) { // Buffer too small
        storedValueStr = "Buffer too small (value truncated). Required: " + actualLen.toString();
    } else { // Negative actualLen likely an error code from host
        storedValueStr = "Error code from storage: " + actualLen.toString();
    }

    return "Hello, " + name + "! Stored value for 'my_key' is: '" + storedValueStr + "'";
}

/**
 * Stores a string value under a given string key, only if called by the owner.
 * @param key The key.
 * @param value The value.
 */
export function storeValue(key: string, value: string): void {
    const caller = getCaller();
    if (caller == "") {
        log("MyContract.storeValue denied: could not identify caller.");
        return;
    }

    let ownerKeyParams = stringToBytesPtrLen("owner");
    let ownerValueBuffer = new Uint8Array(CALLER_ADDRESS_BUFFER_CAPACITY); // Buffer for owner address
    // @ts-ignore
    let ownerValueBufPtr = ownerValueBuffer.dataStart as i32;

    let ownerActualLen = blockchain_get_storage(ownerKeyParams[0], ownerKeyParams[1], ownerValueBufPtr, ownerValueBuffer.byteLength);
    let ownerAddress: string | null = null;

    if (ownerActualLen > 0 && ownerActualLen <= ownerValueBuffer.byteLength) {
        ownerAddress = bytesToString(ownerValueBuffer.buffer, ownerActualLen);
    }

    if (ownerAddress && ownerAddress == caller) {
        log("MyContract.storeValue (by owner " + caller + "): key='" + key + "', value='" + value + "'");
        let keyParams = stringToBytesPtrLen(key);
        let valueParams = stringToBytesPtrLen(value);
        let setResult = blockchain_set_storage(keyParams[0], keyParams[1], valueParams[0], valueParams[1]);
        if (setResult != 0) {
            log("MyContract.storeValue: Error setting storage. Code: " + setResult.toString());
        }
    } else {
        log("MyContract.storeValue denied: caller '" + caller + "' is not owner '" + (ownerAddress || "null") + "'.");
    }
}

/**
 * Retrieves a string value for a given string key.
 * @param key The key.
 * @returns The stored string value, or null if the key is not found or an error occurs.
 */
export function getValue(key: string): string | null {
    log("MyContract.getValue called for key: '" + key + "'");

    let keyParams = stringToBytesPtrLen(key);
    let valueBuffer = new Uint8Array(256); // Adjust buffer size as needed
    // @ts-ignore
    let valueBufPtr = valueBuffer.dataStart as i32;

    let actualLen = blockchain_get_storage(keyParams[0], keyParams[1], valueBufPtr, valueBuffer.byteLength);

    if (actualLen == 0) { // Key not found
        log("MyContract: Key '" + key + "' not found.");
        return null;
    } else if (actualLen > 0 && actualLen <= valueBuffer.byteLength) { // Value found and fits
        return bytesToString(valueBuffer.buffer, actualLen);
    } else if (actualLen > valueBuffer.byteLength) { // Buffer too small
        log("MyContract: Buffer too small for key '" + key + "'. Required: " + actualLen.toString() + ", Buffer: " + valueBuffer.byteLength.toString() + ". Value is truncated.");
        // For this example, return what was copied (truncated)
        return bytesToString(valueBuffer.buffer, valueBuffer.byteLength);
    } else { // Negative value indicates other host error
         log("MyContract: Error retrieving value for key '" + key + "'. Host error code: " + actualLen.toString());
        return null;
    }
}
