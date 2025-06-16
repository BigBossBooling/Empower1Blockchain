// tests/MyContract.spec.ts
import { init, Greet, storeValue, getValue } from '../assembly/contracts/MyContract';
import { MockVM, VM } from "as-pect"; // Import MockVM for easier mocking

// Mock the host environment state that our contract interacts with.
// This allows us to simulate blockchain storage and other host features during tests.
const mockStorage = new Map<string, string>();
let lastLoggedMessage: string = "";
let currentCallerAddress: string = "default_test_caller_address"; // Default caller for tests

describe("MyContract", () => {
  // Before each test, reset the mock storage, logs, and default caller.
  // Also, set up the mocks for the host functions that MyContract.ts imports.
  beforeEach(() => {
    VM.reset(); // Resets the AssemblyScript VM state provided by as-pect
    mockStorage.clear();
    lastLoggedMessage = "";
    currentCallerAddress = "default_test_caller_address"; // Reset to default

    // Use MockVM to provide mock implementations for the host functions.
    // These mocks will be called by the WASM module during tests.
    MockVM.setFunction("env", "host_log_message", (message_ptr: i32, message_len: i32): void => {
      // In a real test, you'd use VM.getString(message_ptr, message_len) to read from WASM memory.
      // For simplicity, we'll just note it was called, or if the message is simple, log it.
      // This mock is simplified; a full mock would read the string from WASM memory.
      lastLoggedMessage = `Log: ptr=${message_ptr}, len=${message_len}`; // Placeholder
      // To actually get the string: lastLoggedMessage = VM.getString(message_ptr, message_len);
      // console.log("HOST_MOCK_LOG: " + VM.getString(message_ptr, message_len));
    });

    MockVM.setFunction("env", "blockchain_set_storage", (key_ptr: i32, key_len: i32, value_ptr: i32, value_len: i32): i32 => {
      const key = VM.getString(key_ptr, key_len);
      const value = VM.getString(value_ptr, value_len);
      mockStorage.set(key, value);
      // console.log(`MOCK_SET_STORAGE: ${key} = ${value}`);
      return 0; // Return 0 for success (ErrCodeSuccess)
    });

    MockVM.setFunction("env", "blockchain_get_storage", (key_ptr: i32, key_len: i32, ret_buf_ptr: i32, ret_buf_len: i32): i32 => {
      const key = VM.getString(key_ptr, key_len);
      const value = mockStorage.get(key);
      if (value) {
        const valueBytes = String.UTF8.encode(value);
        if (valueBytes.byteLength <= ret_buf_len) {
          VM.memory.set(valueBytes, ret_buf_ptr); // Write value to the WASM buffer
          return valueBytes.byteLength; // Return actual length written
        }
        return valueBytes.byteLength; // Indicate needed buffer size
      }
      return 0; // Key not found
    });

    MockVM.setFunction("env", "blockchain_get_caller_address", (ret_buf_ptr: i32, ret_buf_len: i32): i32 => {
      const callerBytes = String.UTF8.encode(currentCallerAddress);
      if (callerBytes.byteLength <= ret_buf_len) {
        VM.memory.set(callerBytes, ret_buf_ptr);
        return callerBytes.byteLength;
      }
      return callerBytes.byteLength; // Indicate needed size
    });
  });

  it("should initialize and store owner and initial value", () => {
    currentCallerAddress = "owner_on_init_addr"; // Set the desired caller for this test
    // No need to update MockVM for blockchain_get_caller_address if it reads currentCallerAddress closure var.
    // However, if MockVM.setFunction creates a new closure each time, then yes.
    // For as-pect, mocks are typically set up to access such test-scoped variables.
    // Let's refine the mock setup in beforeEach to ensure it uses the current `currentCallerAddress`.
    MockVM.setFunction("env", "blockchain_get_caller_address", (ret_buf_ptr: i32, ret_buf_len: i32): i32 => {
        const callerBytes = String.UTF8.encode(currentCallerAddress);
        if (callerBytes.byteLength <= ret_buf_len) {
            VM.memory.set(callerBytes, ret_buf_ptr);
            return callerBytes.byteLength;
        }
        return callerBytes.byteLength;
    });


    init("FirstValue");
    expect(mockStorage.get("my_key")).toBe("FirstValue", "'my_key' should be 'FirstValue' after init.");
    expect(mockStorage.get("owner")).toBe("owner_on_init_addr", "Owner should be set to the caller during init.");
    // Check logs (simplified, as actual log content capture from host_log_message mock is complex here)
    // expect(lastLoggedMessage).toContain("MyContract.init called by: owner_on_init_addr");
  });

  it("Greet should return a greeting including the stored value", () => {
    mockStorage.set("my_key", "Stored World"); // Pre-set storage for this test
    const greeting = Greet("User");
    expect(greeting).toBe("Hello, User! Stored value for 'my_key' is: 'Stored World'");
  });

  it("Greet should handle missing 'my_key'", () => {
    const greeting = Greet("User");
    expect(greeting).toBe("Hello, User! Stored value for 'my_key' is: 'Key 'my_key' not found.'");
  });

  it("storeValue should update storage if called by owner", () => {
    // 1. Initialize to set an owner
    currentCallerAddress = "the_owner";
    MockVM.setFunction("env", "blockchain_get_caller_address", (ret_buf_ptr: i32, ret_buf_len: i32): i32 => {
        const callerBytes = String.UTF8.encode(currentCallerAddress); // Use updated currentCallerAddress
        if (callerBytes.byteLength <= ret_buf_len) { VM.memory.set(callerBytes, ret_buf_ptr); return callerBytes.byteLength; }
        return callerBytes.byteLength;
    });
    init("initial");
    expect(mockStorage.get("owner")).toBe("the_owner");

    // 2. Call storeValue as "the_owner"
    storeValue("anotherKey", "newValue");
    expect(mockStorage.get("anotherKey")).toBe("newValue");
  });

  it("storeValue should be denied if not called by owner", () => {
    // 1. Initialize to set an owner
    currentCallerAddress = "the_real_owner";
     MockVM.setFunction("env", "blockchain_get_caller_address", (ret_buf_ptr: i32, ret_buf_len: i32): i32 => {
        const callerBytes = String.UTF8.encode(currentCallerAddress);
        if (callerBytes.byteLength <= ret_buf_len) { VM.memory.set(callerBytes, ret_buf_ptr); return callerBytes.byteLength; }
        return callerBytes.byteLength;
    });
    init("initial_val");
    expect(mockStorage.get("owner")).toBe("the_real_owner");

    // 2. Attempt to call storeValue as "impostor"
    currentCallerAddress = "impostor_caller"; // Change current caller for this part of the test
     MockVM.setFunction("env", "blockchain_get_caller_address", (ret_buf_ptr: i32, ret_buf_len: i32): i32 => {
        const callerBytes = String.UTF8.encode(currentCallerAddress);
        if (callerBytes.byteLength <= ret_buf_len) { VM.memory.set(callerBytes, ret_buf_ptr); return callerBytes.byteLength; }
        return callerBytes.byteLength;
    });

    storeValue("anotherKey", "hackedValue");
    expect(mockStorage.get("anotherKey")).toBeNull("Value should not have been stored by non-owner.");
    // Check logs to confirm denial message (conceptual, depends on log capture)
    // expect(lastLoggedMessage).toContain("MyContract.storeValue denied: caller impostor_caller is not owner the_real_owner");
  });

  it("getValue should retrieve a previously stored value", () => {
    mockStorage.set("getKeyTest", "RetrievedThis");
    const val = getValue("getKeyTest");
    expect(val).toBe("RetrievedThis");
  });

  it("getValue should return null for a non-existent key", () => {
    const val = getValue("keyThatDoesNotExist");
    expect(val).toBeNull();
  });
});
