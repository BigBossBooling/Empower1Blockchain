# Empower1 GUI Wallet (Electron) - Phase 3.1 Part 1

This is a basic GUI wallet for the Empower1 Blockchain, built using Electron and JavaScript. This initial version focuses on local wallet generation and loading from a private key string.

## Features (Phase 3.1 Part 1)

*   **Wallet Generation:** Create a new SECP256R1 (P256) key pair. The private key and public key (address) are displayed.
*   **Wallet Loading:** Load an existing wallet by providing its private key in hexadecimal format.
*   **Address Display:** Shows the current wallet's address (derived from the uncompressed public key, hex encoded).

**Note:** This version does *not* yet interact with a running Empower1 node (no transaction sending, balance checking, etc.). It also does not yet support loading/saving encrypted wallet files from/to disk (only in-memory and loading from raw private key hex).

## Technology Stack

*   **Electron:** For building the cross-platform desktop application.
*   **HTML/CSS/JavaScript:** For the user interface and renderer process logic.
*   **`elliptic` library:** For ECDSA cryptographic operations (key generation, public key derivation using the `p256` curve).

## Project Structure

*   `main.js`: The Electron main process script. Handles window creation and app lifecycle.
*   `index.html`: The main HTML file for the GUI.
*   `renderer.js`: JavaScript for the renderer process (UI logic, interaction with `elliptic`).
*   `preload.js`: Preload script for Electron (currently minimal, can be used for secure IPC later).
*   `package.json`: Node.js project configuration, including dependencies.

## Setup and Running

**1. Prerequisites:**
    *   Node.js and npm: Download and install from [nodejs.org](https://nodejs.org/).
    *   Git (optional, if cloning the repository).

**2. Get the Code:**
    *   Ensure the `gui-wallet` directory and its contents are present in your project.

**3. Install Dependencies:**
    *   Navigate to the `gui-wallet` directory in your terminal:
        ```bash
        cd path/to/your_project/gui-wallet
        ```
    *   Install the necessary npm packages:
        ```bash
        npm install
        ```
        This will install `electron` (from `devDependencies`) and `elliptic` (from `dependencies`) as defined in `package.json`.

**4. Run the Application:**
    *   From the `gui-wallet` directory, run:
        ```bash
        npm start
        ```
        This command executes `electron .` as defined in the `scripts` section of `package.json`.

## Usage

*   **Create New Wallet:**
    *   Click the "Create New Wallet" button.
    *   The application will generate a new P256 key pair.
    *   Your new **Address (Uncompressed Public Key Hex)** and **Private Key (Hex)** will be displayed.
    *   **VERY IMPORTANT:** Securely save the displayed Private Key. It is shown only once. If you lose it, you will lose access to any funds or identity associated with this key.
*   **Load Wallet from Private Key:**
    *   Enter a 64-character hexadecimal private key into the "Load from Private Key (Hex)" input field.
    *   Click the "Load Wallet" button.
    *   If the private key is valid, the corresponding Address will be displayed. The "Newly Generated Wallet Details" section will be hidden.
    *   If the private key is invalid, an error message will be shown.

## Development Notes & Security Considerations

*   **`require('elliptic')` in Renderer:**
    *   The current `renderer.js` uses `require('elliptic')`. By default, Electron's newer versions have `nodeIntegration: false` and `contextIsolation: true` for security. In such a setup, `require()` is not available directly in the renderer process for Node.js modules.
    *   **To run this specific version as-is without code changes for IPC:** You might temporarily set `nodeIntegration: true` in `gui-wallet/main.js` within `webPreferences`. However, this is **not recommended for production or more advanced applications** due to security risks.
    *   **Secure Approach (Future Work):** For a production-quality application, cryptographic operations (like those using `elliptic`) should be performed in the main process or a secure preload script, with results passed to the renderer process via Electron's Inter-Process Communication (IPC) mechanisms (`contextBridge` and `ipcRenderer`/`ipcMain`). This keeps the renderer process, which displays web content, sandboxed from direct Node.js API access.
*   **Private Key Handling:** Displaying private keys directly in the UI and requiring users to paste them is for simplicity in this initial version. Real wallets use more secure methods for key storage (encrypted files, hardware wallets) and handling. Saving to an encrypted file is a planned next step.
*   **No File-Based Wallet Loading/Saving Yet:** This version does not implement loading from/saving to PEM or other encrypted file formats.

This basic GUI provides the foundation for future wallet functionalities.
