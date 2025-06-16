# Empower1 AssemblyScript Smart Contract Boilerplate

This boilerplate provides a starting point for developing smart contracts for the Empower1 blockchain using AssemblyScript. It includes an example contract, host function declarations, testing setup with `as-pect`, and build configurations.

## Prerequisites

*   **Node.js and npm:** Required for installing dependencies and running scripts. It's recommended to use a recent LTS version of Node.js. Download from [nodejs.org](https://nodejs.org/).
*   **AssemblyScript Compiler (`asc`):** This will be installed as a dev dependency of this boilerplate project. If you wish to use `asc` globally, you can install it via `npm install -g assemblyscript`.
*   **as-pect CLI (`asp`):** The testing tool, also a dev dependency.

## Project Structure

```
.
├── assembly/                # AssemblyScript source files
│   ├── contracts/           # Your main contract logic
│   │   └── MyContract.ts    # Example contract demonstrating storage, logging, caller access
│   └── env/                 # Host function declarations (imported by contracts)
│       └── index.ts         # Declares all available Empower1 host functions
├── build/                   # Compiled WASM output (debug and release targets)
│   ├── debug/
│   │   └── MyContract.wasm
│   │   └── MyContract.wasm.map
│   └── release/
│       └── MyContract.wasm
├── node_modules/            # Project dependencies (managed by npm)
├── tests/                   # as-pect unit tests
│   └── MyContract.spec.ts   # Example tests for MyContract.ts, demonstrating host mocking
├── as-pect.config.js        # Configuration for the as-pect test runner (global mocks)
├── asconfig.json            # AssemblyScript compiler configuration
├── package.json             # Node.js project manifest & scripts
└── README.md                # This file
```

## Setup & Usage

1.  **Copy Boilerplate:** Copy this entire `assemblyscript_boilerplate` directory to a new location for your project.
2.  **Navigate to your new project directory:**
    ```bash
    cd path/to/your_new_contract_project
    ```
3.  **Install dependencies:**
    ```bash
    npm install
    ```
    This will install `assemblyscript`, `@as-pect/cli`, `@as-pect/core`, and `@assemblyscript/transform-array-buffer` as dev dependencies from `package.json`.

## Available NPM Scripts

(Defined in `package.json`)

*   **`npm run asbuild:debug`**: Compiles `assembly/contracts/MyContract.ts` to WASM in debug mode.
    *   Output: `build/debug/MyContract.wasm` (with source map).
*   **`npm run asbuild:release`**: Compiles `assembly/contracts/MyContract.ts` in release mode.
    *   Output: `build/release/MyContract.wasm` (optimized).
*   **`npm run asbuild`**: Alias for `npm run asbuild:release`.
*   **`npm test`**: Runs the `as-pect` test suite defined in `tests/**/*.spec.ts`.
    *   Uses `as-pect.config.js` for global configuration and mocks.
    *   Provides verbose output.

## Developing Your Smart Contract

1.  **Define Your Contract (`assembly/contracts/`):**
    *   Modify `MyContract.ts` or create new `.ts` files.
    *   Define your contract's state, data structures, and exported functions.
2.  **Host Function Declarations (`assembly/env/index.ts`):**
    *   This file declares all available host functions that your contract can import and use to interact with the Empower1 blockchain (e.g., storage, logging, caller info, events).
    *   Import them in your contract: `import { host_log_message, ... } from "../env";`
    *   Refer to the main Empower1 `docs/smart_contract_dev_guide.md` for the complete API specification of these host functions.
3.  **Compiling:**
    *   Use `npm run asbuild:debug` during development for easier debugging.
    *   Use `npm run asbuild:release` for creating optimized WASM for deployment.
4.  **Testing:**
    *   Write unit tests in the `tests/` directory using `as-pect`.
    *   The `tests/MyContract.spec.ts` provides an example of how to import your contract and use `as-pect`'s `MockVM` to provide mock implementations for host functions. This allows testing your contract logic in isolation.
    *   Run tests with `npm test`.

## Deployment & Interaction with Empower1 Node

*   Once your contract is compiled to WASM (e.g., `build/release/MyContract.wasm`), it can be deployed to an Empower1 node.
*   Refer to the main Empower1 project's `TESTING.md` and `docs/smart_contract_dev_guide.md` for detailed instructions on:
    *   Deploying compiled `.wasm` files (currently via debug HTTP endpoints).
    *   Calling functions on your deployed contract (currently via debug HTTP endpoints or the Python CLI wallet).

## Further Information

*   **AssemblyScript Documentation:** [www.assemblyscript.org](https://www.assemblyscript.org/)
*   **as-pect Documentation:** [as-pect.gitbook.io/as-pect](https://as-pect.gitbook.io/as-pect/)
*   **Empower1 Host API & Smart Contract Guide:** See the `docs/` folder in the main Empower1 project repository.
```
