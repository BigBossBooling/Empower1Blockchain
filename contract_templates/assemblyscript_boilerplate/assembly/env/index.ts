// assembly/env/index.ts
// Host function declarations for Empower1 smart contracts.

// --- Logging & Events ---
@external("env", "host_log_message")
export declare function host_log_message(message_ptr: i32, message_len: i32): void;

@external("env", "blockchain_emit_event")
export declare function blockchain_emit_event(topic_ptr: i32, topic_len: i32, data_ptr: i32, data_len: i32): void;

// --- State Storage ---
@external("env", "blockchain_set_storage")
export declare function blockchain_set_storage(key_ptr: i32, key_len: i32, value_ptr: i32, value_len: i32): i32; // Returns error_code

@external("env", "blockchain_get_storage")
export declare function blockchain_get_storage(key_ptr: i32, key_len: i32, ret_buf_ptr: i32, ret_buf_len: i32): i32; // Returns actual_value_len or error_code

// --- Caller Information ---
@external("env", "blockchain_get_caller_address") // Returns hex string of public key
export declare function blockchain_get_caller_address(ret_buf_ptr: i32, ret_buf_len: i32): i32;

@external("env", "blockchain_get_caller_public_key") // Returns raw public key bytes
export declare function blockchain_get_caller_public_key(ret_buf_ptr: i32, ret_buf_len: i32): i32;

// --- DID Related ---
@external("env", "blockchain_generate_did_key") // Takes raw pubkey bytes, returns did:key string
export declare function blockchain_generate_did_key(pubkey_ptr: i32, pubkey_len: i32, ret_buf_ptr: i32, ret_buf_len: i32): i32;

// --- Blockchain Information ---
@external("env", "blockchain_get_block_timestamp")
export declare function blockchain_get_block_timestamp(): u64;

// @external("env", "blockchain_get_block_height") // Example if added
// export declare function blockchain_get_block_height(): u64;

// --- Financial Operations (Placeholders/Stubs in current Go implementation) ---
// @external("env", "blockchain_get_balance")
// export declare function blockchain_get_balance(address_ptr: i32, address_len: i32): u64;
// @external("env", "blockchain_send_funds")
// export declare function blockchain_send_funds(to_address_ptr: i32, to_address_len: i32, amount: u64): i32;


// --- Important Notes for using these in contracts ---
// Your contract code will need to handle UTF-8 encoding/decoding when passing strings
// to/from these host functions, typically using:
//   `String.UTF8.encode(myString)` -> returns ArrayBuffer
//   `String.UTF8.decode(buffer.slice(0, length))`
// And then get the pointer and length from the ArrayBuffer for ptr/len arguments:
//   `// @ts-ignore: dataStart is a property`
//   `let ptr = myBuffer.dataStart as i32;`
//   `let len = myBuffer.byteLength;`
// For functions that write data back to a buffer (e.g., blockchain_get_storage),
// your contract must provide a Uint8Array buffer and pass its dataStart and byteLength.
// The return value often indicates actual length written or an error code.
```
