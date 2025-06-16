#!/bin/bash
# Script to compile the simple_storage.ts to WASM using asc.
# Uses npx to run the locally installed AssemblyScript compiler.

# Output directory for the compiled WASM file (relative to this script's location)
OUT_DIR="./out"
mkdir -p $OUT_DIR

# Source file
SRC_FILE="simple_storage.ts"

# Output WASM file path
OUT_FILE="$OUT_DIR/simple_storage.wasm"

echo "Compiling $SRC_FILE to $OUT_FILE using npx asc..."

# Use npx to run the AssemblyScript compiler.
# This assumes assemblyscript is listed in package.json (or installed locally via npm install assemblyscript --save-dev).
npx asc "$SRC_FILE" \
    -b "$OUT_FILE" \
    --optimize \
    --runtime stub \
    --exportRuntime \
    # Add --use abort=path_to_abort_implementation if needed, e.g., if your host doesn't provide 'abort'
    # For simple_storage, it might not be strictly necessary if no runtime errors are expected to abort.

# Check if compilation was successful
if [ $? -eq 0 ]; then
  echo "Compilation successful: $OUT_FILE created."
  ls -l "$OUT_FILE"
else
  echo "Compilation failed. Ensure 'assemblyscript' is installed locally (e.g., npm install assemblyscript --save-dev in this directory)."
fi
