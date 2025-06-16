// did_registry.ts - DID Registry Smart Contract

// --- Host Function Imports ---
// These functions are provided by the blockchain environment (Go host).

@external("env", "host_log_message")
declare function host_log_message(message_ptr: i32, message_len: i32): void;

@external("env", "blockchain_set_storage")
declare function blockchain_set_storage(key_ptr: i32, key_len: i32, value_ptr: i32, value_len: i32): i32;

@external("env", "blockchain_get_storage")
declare function blockchain_get_storage(key_ptr: i32, key_len: i32, ret_buf_ptr: i32, ret_buf_len: i32): i32;

// Gets the raw public key of the transaction signer.
// Writes to ret_buf_ptr, returns actual length or error code.
@external("env", "blockchain_get_caller_public_key")
declare function blockchain_get_caller_public_key(ret_buf_ptr: i32, ret_buf_len: i32): i32;

// Generates a did:key string from raw public key bytes.
// pubkey_ptr/len points to raw public key. ret_buf_ptr/len is for the output did:key string.
// Returns actual length of did:key string or error code.
@external("env", "blockchain_generate_did_key")
declare function blockchain_generate_did_key(pubkey_ptr: i32, pubkey_len: i32, ret_buf_ptr: i32, ret_buf_len: i32): i32;

@external("env", "blockchain_emit_event")
declare function blockchain_emit_event(topic_ptr: i32, topic_len: i32, data_ptr: i32, data_len: i32): void;


// --- Helper Functions ---
function log(message: string): void {
  let messageBytes = String.UTF8.encode(message);
  // @ts-ignore: dataStart is a property on ArrayBuffer in AssemblyScript
  host_log_message(messageBytes.dataStart as i32, messageBytes.byteLength);
}

// Pre-allocate buffers for host function calls that write back data
// These are module-level static allocations.
const CALLER_PUBKEY_BUF_CAPACITY = 70; // Enough for 65-byte uncompressed P256 pubkey + buffer
let callerPubKeyBuffer = new Uint8Array(CALLER_PUBKEY_BUF_CAPACITY);
// @ts-ignore
let callerPubKeyBufPtr = callerPubKeyBuffer.dataStart as i32;

const DID_STRING_BUF_CAPACITY = 128; // did:key strings can be around 50-60 chars for P256, plus did:key: prefix
let didStringBuffer = new Uint8Array(DID_STRING_BUF_CAPACITY);
// @ts-ignore
let didStringBufPtr = didStringBuffer.dataStart as i32;


// --- Contract Logic ---

export function init(): void {
  log("DIDRegistry contract initialized.");
}

export function registerDIDDocument(did_to_register: string, document_hash: string, document_location_uri: string): void {
  log("registerDIDDocument called for DID: " + did_to_register + ", hash: " + document_hash + ", uri: " + document_location_uri);

  // 1. Get Caller's Public Key from the host
  let callerPubKeyLen = blockchain_get_caller_public_key(callerPubKeyBufPtr, CALLER_PUBKEY_BUF_CAPACITY);

  if (callerPubKeyLen <= 0) { // Check for error or empty key
    log("Error: Failed to get valid caller public key. Host returned length: " + callerPubKeyLen.toString());
    return;
  }
  if (callerPubKeyLen > CALLER_PUBKEY_BUF_CAPACITY) {
      log("Error: Buffer too small for caller public key. Required: " + callerPubKeyLen.toString() + ", Capacity: " + CALLER_PUBKEY_BUF_CAPACITY.toString());
      // Potentially, could try to reallocate and call again if AS supported dynamic buffer growth easily for this pattern.
      return;
  }
  // Use the sub-array of the buffer that contains the actual public key
  let callerPubKeyBytes = callerPubKeyBuffer.subarray(0, callerPubKeyLen);

  // 2. Generate did:key from Caller's Public Key using host function
  // @ts-ignore: dataStart on subarray might be tricky, but AS handles it for typed arrays.
  // It's safer to pass the original buffer's dataStart (callerPubKeyBufPtr) if the host function expects that,
  // and the host function uses the returned callerPubKeyLen.
  // Let's assume blockchain_generate_did_key takes the pointer to the start of actual key data.
  // For a subarray, dataStart is relative to the subarray, not the original buffer.
  // So, we need to pass the pointer from the original buffer `callerPubKeyBufPtr`.

  let derivedDidLen = blockchain_generate_did_key(callerPubKeyBufPtr, callerPubKeyLen, didStringBufPtr, DID_STRING_BUF_CAPACITY);

  if (derivedDidLen <= 0) { // Check for error or empty DID
    log("Error: Failed to generate DID:key from caller's public key. Host returned length: " + derivedDidLen.toString());
    return;
  }
   if (derivedDidLen > DID_STRING_BUF_CAPACITY) {
      log("Error: Buffer too small for derived DID string. Required: " + derivedDidLen.toString() + ", Capacity: " + DID_STRING_BUF_CAPACITY.toString());
      return;
  }
  let derivedDidFromCaller = String.UTF8.decode(didStringBuffer.buffer.slice(0, derivedDidLen));
  log("DID derived by host from caller's pubkey: " + derivedDidFromCaller);

  // 3. Authenticate: Compare derived DID with the one provided for registration
  if (did_to_register != derivedDidFromCaller) {
    log("Authentication failed: Provided DID '" + did_to_register + "' does not match DID derived from transaction signer ('" + derivedDidFromCaller + "'). Registration aborted.");
    return;
  }
  log("Authentication successful for DID: " + did_to_register);

  // 4. Store the document info
  let docInfoJson = "{\"document_hash\":\"" + document_hash + "\",\"document_location_uri\":\"" + document_location_uri + "\"}";

  let didStorageKeyBytes = String.UTF8.encode(did_to_register); // Use the validated did_to_register as key
  let docInfoStorageBytes = String.UTF8.encode(docInfoJson);

  // @ts-ignore
  let didStorageKeyPtr = didStorageKeyBytes.dataStart as i32;
  let didStorageKeyLen = didStorageKeyBytes.byteLength;
  // @ts-ignore
  let docInfoStoragePtr = docInfoStorageBytes.dataStart as i32;
  let docInfoStorageLen = docInfoStorageBytes.byteLength;

  let setResult = blockchain_set_storage(didStorageKeyPtr, didStorageKeyLen, docInfoStoragePtr, docInfoStorageLen);

  if (setResult == 0) { // Success (assuming 0 is success code from host, e.g. ErrCodeSuccess)
    log("DID Document info registered successfully for: " + did_to_register);
    // Emit event
    let eventTopic = "DIDDocumentRegistered";
    let eventData = "{\"did\":\"" + did_to_register + "\",\"hash\":\"" + document_hash + "\",\"uri\":\"" + document_location_uri + "\"}";

    let topicBytes = String.UTF8.encode(eventTopic);
    let dataBytes = String.UTF8.encode(eventData);
    // @ts-ignore
    blockchain_emit_event(topicBytes.dataStart as i32, topicBytes.byteLength, dataBytes.dataStart as i32, dataBytes.byteLength);
  } else {
    log("Failed to register DID Document info. Storage error code from host: " + setResult.toString());
  }
}

export function getDIDDocumentInfo(did_string: string): string | null {
  log("getDIDDocumentInfo called for DID: " + did_string);
  let didBytes = String.UTF8.encode(did_string);
  // @ts-ignore
  let didPtr = didBytes.dataStart as i32;
  let didLen = didBytes.byteLength;

  let bufferCapacity = 512;
  let valueBuffer = new Uint8Array(bufferCapacity);
  // @ts-ignore
  let retBufPtr = valueBuffer.dataStart as i32;

  let actualLen = blockchain_get_storage(didPtr, didLen, retBufPtr, valueBuffer.byteLength);

  if (actualLen <= 0) {
    log("DID info not found or error for: " + did_string + ". Host returned code/len: " + actualLen.toString());
    return null;
  }
  if (actualLen > valueBuffer.byteLength) {
    log("Buffer too small for DID info: " + did_string + ". Required: " + actualLen.toString() + ", Capacity: " + valueBuffer.byteLength.toString());
    return null;
  }

  let resultJson = String.UTF8.decode(valueBuffer.buffer.slice(0, actualLen));
  log("Retrieved DID info for " + did_string + ": " + resultJson);
  return resultJson;
}

function _parseJsonField(jsonString: string, field: string): string | null {
  // This is a very naive and fragile JSON parser.
  // It's better to use a proper JSON library in AssemblyScript if available and complex parsing is needed.
  // For this specific structure "{\"field\":\"value\"...}", it might work.
  let keyToFind = "\"" + field + "\":\"";
  let startIndex = jsonString.indexOf(keyToFind);
  if (startIndex == -1) {
    return null;
  }
  startIndex += keyToFind.length;
  let endIndex = jsonString.indexOf("\"", startIndex);
  if (endIndex == -1) {
    return null;
  }
  return jsonString.substring(startIndex, endIndex);
}

export function getDIDDocumentHash(did_string: string): string | null {
  let docInfoJson = getDIDDocumentInfo(did_string);
  if (docInfoJson === null) return null;
  return _parseJsonField(docInfoJson, "document_hash");
}

export function getDIDDocumentURI(did_string: string): string | null {
  let docInfoJson = getDIDDocumentInfo(did_string);
  if (docInfoJson === null) return null;
  return _parseJsonField(docInfoJson, "document_location_uri");
}
