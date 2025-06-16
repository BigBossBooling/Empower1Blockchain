# Empower1 Smart Contracts: Overview

Empower1 enables on-chain programmable logic through WebAssembly (WASM) based smart contracts. This document provides a high-level overview of the smart contract system.

## Core Concepts

*   **Virtual Machine (VM):** Empower1 uses WebAssembly (WASM) as the runtime environment for smart contracts. This provides a secure, sandboxed, and high-performance execution engine.
*   **Smart Contract Language:** AssemblyScript (a language with TypeScript-like syntax that compiles to WASM) is the primary recommended language for developing smart contracts on Empower1. Other languages that compile to WASM may also be usable.
*   **Host Functions:** Smart contracts interact with the Empower1 blockchain (e.g., to access storage, get block information, or identify the caller) through a defined API of "host functions" provided by the node.
*   **Gas Model:** Contract execution and resource usage (like storage) are metered by a gas mechanism to prevent abuse and manage network resources. Transactions invoking contracts must supply a gas limit.
*   **Deployment & Interaction:** Contracts are deployed on the blockchain as WASM bytecode. Users can then interact with deployed contracts by sending transactions that specify the contract address, function to call, and arguments.

## Key Documentation Resources

For detailed information on developing, deploying, and interacting with smart contracts on Empower1, please refer to the following documents:

1.  **[Smart Contract Development Guide](./smart_contract_dev_guide.md):**
    *   Comprehensive guide covering environment setup, writing AssemblyScript contracts, available host functions, compilation, testing with `as-pect`, deployment, calling functions, the gas model, and security best practices.
    *   **This is the primary guide for smart contract developers.**

2.  **[Smart Contract Boilerplate](../contract_templates/assemblyscript_boilerplate/README.md):**
    *   A ready-to-use boilerplate project to kickstart AssemblyScript smart contract development for Empower1.

3.  **Example Smart Contracts:**
    *   **Simple Storage:** Located in `contracts_src/simple_storage/simple_storage.ts`. Demonstrates basic state manipulation (set/get) and logging.
    *   **DID Registry:** Located in `contracts_src/did_registry/did_registry.ts`. A more complex example showcasing DID document registration, caller authentication using host functions, and event emission.

4.  **[Decentralized Identifiers (DIDs) Framework](./did_framework.md):**
    *   Explains how DIDs (specifically `did:key`) are implemented and how the `DIDRegistry` smart contract functions within this framework.

5.  **[TESTING.md](../TESTING.md):**
    *   Contains sections with step-by-step instructions for deploying and interacting with the example smart contracts using debug endpoints and the CLI wallet.

## Current Status & Future Work

*   The current implementation supports deploying WASM contracts and calling their functions via debug RPC endpoints.
*   A foundational set of host functions is available, focusing on storage, logging, and caller identification.
*   The gas model accounts for host function execution and a base fee for WASM instantiation.
*   Future work includes implementing dedicated transaction types for contract deployment and calls, full instruction-level gas metering, expanding the host function API, and enhancing the developer tooling.

This overview provides a starting point. For any development work, the **[Smart Contract Development Guide](./smart_contract_dev_guide.md)** should be consulted.
