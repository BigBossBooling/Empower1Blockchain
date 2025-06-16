// A simple AssemblyScript contract for testing WASM execution.

// Export a function to be called from the host (Go).
export function add(a: i32, b: i32): i32 {
  return a + b;
}

// Host function for logging (imported from the Go environment).
// The names "env" and "host_log_message" must match what the Go host provides.
@external("env", "host_log_message")
declare function host_log_message(message_ptr: i32, message_len: i32): void;

// A function that calls the imported host_log_message function.
export function logTest(message: string): void {
  // Convert the AssemblyScript string to a UTF-8 byte array in WASM memory.
  // AssemblyScript strings are UTF-16 internally. For host calls expecting C-style strings
  // or byte arrays, explicit conversion is often needed.

  let messageBytes = String.UTF8.encode(message);
  // Get a pointer to the start of the byte array in memory.
  // Pass this pointer and the length to the host function.
  // The `dataStart` property of an ArrayBuffer gives the pointer to the underlying memory.
  // For StaticArray<u8> or Array<u8>, one might need to be more careful or use `messageBytes.dataStart`.
  // For `String.UTF8.encode`, it returns an ArrayBuffer.

  // @ts-ignore: dataStart is a valid property on ArrayBuffer for AS
  let message_ptr = messageBytes.dataStart;
  let message_len = messageBytes.byteLength;

  host_log_message(message_ptr as i32, message_len as i32);
}

// Another example: A function that uses memory directly for strings (less safe, more C-like)
// This shows how to prepare memory for the host if the host expects to read a null-terminated string.
// For this prototype, we'll stick to passing ptr/len for byte arrays.

// Example of allocating memory and returning a pointer (if contract needs to return complex data)
// export function getMessage(): i32 {
//   let message = "Hello from WASM!";
//   let messageArr = String.UTF8.encode(message);
//   // In a real scenario, you might need to ensure this memory is not garbage collected
//   // if the pointer is to be long-lived, or use specific allocation functions.
//   // For simple returns, the host would copy it out immediately.
//   // @ts-ignore
//   return messageArr.dataStart as i32;
// }

// Function to demonstrate potential issues or more complex logic
export function greet(name: string): string {
  let logMsg = "greet function called with: " + name;
  logTest(logMsg); // Call host function to log this message
  return "Hello, " + name + "!";
}
