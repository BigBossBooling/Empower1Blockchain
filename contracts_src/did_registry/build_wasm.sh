#!/bin/bash
# Script to compile the did_registry.ts to WASM using asc.

OUT_DIR="./out"
mkdir -p $OUT_DIR

SRC_FILE="did_registry.ts"
OUT_FILE="$OUT_DIR/did_registry.wasm"

echo "Compiling $SRC_FILE to $OUT_FILE using npx asc..."

# Use npx to run the AssemblyScript compiler.
# Assumes assemblyscript is listed in package.json (e.g. via npm install assemblyscript --save-dev in this directory)
# or installed globally.
npx asc "$SRC_FILE" \
    -b "$OUT_FILE" \
    --optimize \
    --runtime stub \
    --exportRuntime \
    # --use abort=path/to/abort/if/needed

if [ $? -eq 0 ]; then
  echo "Compilation successful: $OUT_FILE created."
  ls -l "$OUT_FILE"
else
  echo "Compilation failed. Ensure 'assemblyscript' is installed (locally or globally) and in your PATH, or use 'npm install assemblyscript --save-dev' in this directory."
fi
