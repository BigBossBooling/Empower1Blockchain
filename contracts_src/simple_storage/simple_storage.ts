// simple_storage.ts - An AssemblyScript smart contract for simple key-value storage.

// --- Host Function Imports ---
// These functions are provided by the blockchain environment (Go host).
// The "env" namespace is a common convention.

// Logs a message using the host's logging facility.
// Parameters: pointer to string message in WASM memory, length of message.
@external("env", "host_log_message")
declare function host_log_message(message_ptr: i32, message_len: i32): void;

// Sets a value in the contract's persistent storage.
// Parameters: key_ptr, key_len, value_ptr, value_len.
// Returns: 0 on success, non-zero on error (e.g., out of gas).
@external("env", "blockchain_set_storage")
declare function blockchain_set_storage(key_ptr: i32, key_len: i32, value_ptr: i32, value_len: i32): i32;

// Gets a value from the contract's persistent storage.
// Parameters: key_ptr, key_len (for the key to lookup),
//             ret_buf_ptr (pointer to a buffer in WASM memory to write the value),
//             ret_buf_len (length of the provided buffer).
// Returns: Actual length of the value. If this is > ret_buf_len, the buffer was too small.
//          If key not found, returns 0. If error, returns a negative error code.
//          (Note: The Go host_functions.go currently returns specific error codes like ErrCodeBufferTooSmall (4)
//           as a positive i32 value for the length field in some error cases, or 0 if not found.
//           This contract will need to interpret the return value carefully.
//           Let's assume for now:
//           - Positive value: actual length of data (data copied if it fit, or indicates required size if > ret_buf_len)
//           - 0: key not found
//           - Negative: other error (e.g. -ErrCodeInvalidMemoryAccess if host returns negative codes)
//           The Go side currently returns actual_value_len (which can be 0 for not found) or an error code like ErrCodeBufferTooSmall.
//           The return signature in host_functions.go for BlockchainGetStorage is `actual_value_len (i32)`.
//           Let's assume 0 means not found, >0 is actual length. If actual_length > ret_buf_len, buffer was too small.
@external("env", "blockchain_get_storage")
declare function blockchain_get_storage(key_ptr: i32, key_len: i32, ret_buf_ptr: i32, ret_buf_len: i32): i32;


// --- Helper function to log messages from the contract ---
function log(message: string): void {
  let messageBytes = String.UTF8.encode(message);
  // @ts-ignore: dataStart is a property on ArrayBuffer in AssemblyScript
  host_log_message(messageBytes.dataStart as i32, messageBytes.byteLength);
}

// --- Public-Facing Contract Functions ---

// Sets a string value for a given string key.
export function set(key: string, value: string): void {
  log("set called with key: '" + key + "', value: '" + value + "'");

  let keyBytes = String.UTF8.encode(key);
  let valueBytes = String.UTF8.encode(value);

  // @ts-ignore
  let keyPtr = keyBytes.dataStart as i32;
  let keyLen = keyBytes.byteLength;
  // @ts-ignore
  let valuePtr = valueBytes.dataStart as i32;
  let valueLen = valueBytes.byteLength;

  let result = blockchain_set_storage(keyPtr, keyLen, valuePtr, valueLen);
  if (result == 0) { // 0 is ErrCodeSuccess from host
    log("Storage set successfully for key: " + key);
  } else {
    log("Failed to set storage for key: " + key + ". Error code: " + result.toString());
  }
}

// Gets a string value for a given string key.
// Returns the value, or null if the key is not found or an error occurs.
export function get(key: string): string | null {
  log("get called with key: '" + key + "'");

  let keyBytes = String.UTF8.encode(key);
  // @ts-ignore
  let keyPtr = keyBytes.dataStart as i32;
  let keyLen = keyBytes.byteLength;

  // Allocate a buffer in WASM memory to receive the value.
  // Start with a reasonable size. If it's too small, we might need a way to resize or re-call.
  // For this example, we'll use a fixed-size buffer. A more robust contract
  // might call a "get_storage_size" host function first if available, or handle reallocation.
  let initialBufferCapacity = 256; // bytes
  let valueBuffer = new Uint8Array(initialBufferCapacity); // StaticArray uses fixed size at compile time
                                                       // Using Uint8Array which is dynamic but we use fixed size here.
  // @ts-ignore
  let retBufPtr = valueBuffer.dataStart as i32;
  let retBufLen = valueBuffer.byteLength; // This is initialBufferCapacity

  log("Calling blockchain_get_storage with buffer capacity: " + retBufLen.toString());

  let actualValueLen = blockchain_get_storage(keyPtr, keyLen, retBufPtr, retBufLen);
  log("blockchain_get_storage returned actual_value_len: " + actualValueLen.toString());


  if (actualValueLen == 0) { // Assuming 0 means key not found (as per current Go host func for nil value)
    log("Key not found: " + key);
    return null;
  } else if (actualValueLen < 0) { // Assuming negative values are error codes from host
    log("Error retrieving value for key: " + key + ". Error code: " + actualValueLen.toString());
    return null;
  // } else if (actualValueLen > retBufLen) {
  //   // This case means the buffer was too small.
  //   // The host_functions.go's BlockchainGetStorage copies min(actual, buffer) and returns actual.
  //   // So, if actualValueLen > retBufLen, the copied data is incomplete.
  //   // A robust contract would detect this and potentially try again with a larger buffer
  //   // or have a mechanism to query size first.
  //   // For this example, we'll treat it as an error or partial read.
  //   log("Buffer too small for key: " + key + ". Required: " + actualValueLen.toString() + ", Have: " + retBufLen.toString());
  //   // We could try to decode the partial data, but it's likely not useful.
  //   // For simplicity, return null. Or, the contract could allocate `actualValueLen` and call again.
  //   // Let's try to return the part that was copied, up to retBufLen.
  //   // However, the current Go host function copies min(actual, retBufLen) bytes,
  //   // and returns actualValueLen. So if actualValueLen > retBufLen, we know it's truncated.
  //   // The data in valueBuffer up to retBufLen *is* valid, but it's only part of the full value.
  //   // This is a design choice on how the contract handles it.
  //   // For this example, we'll return the (potentially truncated) string.
  //   // A better contract would have a loop or pre-size query.
  //    let partialValueBytes = valueBuffer.slice(0, retBufLen); // It's already truncated to retBufLen by host if too large
  //    log("Value for key '" + key + "' was truncated. Returning partial data.");
  //    return String.UTF8.decode(partialValueBytes.buffer);

  // Revised logic based on host function returning actual length:
  } else if (actualValueLen > retBufLen) {
    log("Buffer too small for key: " + key + ". Required: " + actualValueLen.toString() + ", Have: " + retBufLen.toString() + ". Value may be truncated.");
    // The host function copies min(actualValueLen, retBufLen) bytes.
    // So, data in valueBuffer up to retBufLen is valid, but it's incomplete.
    // For this simple example, we'll decode what we got.
    let partialValueBytes = valueBuffer.slice(0, retBufLen);
    return String.UTF8.decode(partialValueBytes.buffer); // This will be a truncated string
  }

  // If actualValueLen <= retBufLen, the entire value was copied.
  let valueBytes = valueBuffer.slice(0, actualValueLen);
  let resultValue = String.UTF8.decode(valueBytes.buffer);
  log("Value retrieved for key '" + key + "': '" + resultValue + "'");
  return resultValue;
}

// init function (optional constructor)
// AssemblyScript doesn't have explicit constructors in the same way as some languages.
// An `init` or `constructor` function can be exported and called by the deployer.
export function init(): void {
  log("simple_storage contract initialized!");
  set("initial_key", "initial_value_set_during_init");
}
