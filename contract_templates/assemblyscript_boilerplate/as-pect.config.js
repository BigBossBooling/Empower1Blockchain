// as-pect.config.js
// This is a basic configuration for as-pect.
// For more advanced configurations, see: https://as-pect.gitbook.io/as-pect/cli/config-file

module.exports = {
  /**
   * A set of globs defining the entry files for the tests.
   * This will usually be `assembly/__tests__/**/*.spec.ts` or similar.
   * For this boilerplate, tests are in `tests/` at the root of the boilerplate.
   */
  entries: ["./tests/**/*.spec.ts"],

  /**
   * A set of globs defining the files to be covered by code coverage.
   * This will usually be `assembly/**/*.ts` excluding test files.
   */
  coverage: ["./assembly/contracts/**/*.ts"],

  /**
   * A set of globs defining the files that are not compiled or watched.
   * (e.g., node_modules, build files)
   */
  disclude: [/node_modules/, /build/],

  /**
   * Command line options passed to the AssemblyScript compiler.
   * Ensure this aligns with your main asconfig.json for consistency, especially runtime.
   * as-pect will compile your tests and contract sources.
   */
  // ascOptions: ["--runtime", "stub", "--exportRuntime"], // Redundant if asconfig is used by asp

  /**
   * Reporter options.
   */
  // reportOpts: { ... }

  /**
   * Enable code coverage.
   */
  // coverage: [], // Add files to be covered here

  /**
   * Specify if the summary should be printed.
   */
  // summary: true,

  /**
   * Specify if the detailed table should be printed.
   */
  // detailed: true,

  /**
   * Mocks for host functions.
   * This is a global way to provide mocks. You can also mock per test suite/test.
   * The object structure should be: { "envModuleName": { "functionNameInEnv": mockImplementation, ... }, ... }
   * The mockImplementation should match the signature expected by your WASM module.
   *
   * For functions expecting (ptr, len) for strings and returning void or simple types:
   * - Input strings: You'll need a way to get strings from WASM memory if your mock needs to read them.
   *   as-pect provides utilities like `asp.getString(ptr, len)` within tests.
   * - Output strings (if host func writes to WASM): Mocks might simulate writing to a JS buffer
   *   and return ptr/len, or as-pect's `asp.setString` can be used.
   */
  mocks: {
    "env": { // Matches the "env" module name used in @external in AssemblyScript
      host_log_message: function(message_ptr, message_len) {
        // In a test, you might want to capture this.
        // For now, just a console log. `asp.getString` would be used inside an actual test.
        // This global mock won't have access to the `asp` instance for getString directly.
        console.log(`MOCK_HOST: host_log_message called (ptr: ${message_ptr}, len: ${message_len}) - Content would need memory access to read.`);
      },
      blockchain_set_storage: function(key_ptr, key_len, value_ptr, value_len) {
        console.log(`MOCK_HOST: blockchain_set_storage called (key_ptr: ${key_ptr}, val_ptr: ${value_ptr})`);
        return 0; // Return 0 for success (ErrCodeSuccess)
      },
      blockchain_get_storage: function(key_ptr, key_len, ret_buf_ptr, ret_buf_len) {
        console.log(`MOCK_HOST: blockchain_get_storage called (key_ptr: ${key_ptr}, ret_buf_ptr: ${ret_buf_ptr})`);
        // To simulate "key not found", return 0.
        // To simulate value found, you'd write to ret_buf_ptr (if you had memory access)
        // and return the actual length.
        // This basic mock just logs and returns 0 (key not found).
        return 0;
      },
      // Add other host functions used by MyContract.ts here with basic mock implementations
      // blockchain_get_caller_public_key: (ptr, len) => { /* ... */ return 0; },
      // blockchain_generate_did_key: (ptr1,len1,ptr2,len2) => { /* ... */ return 0; },
      // blockchain_emit_event: (ptr1,len1,ptr2,len2) => { /* ... */ },
    }
  },

  /**
   * Specify if the tests should run in a browser.
   */
  // browser: false,
};
