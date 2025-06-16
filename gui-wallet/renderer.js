// renderer.js

// Attempt to require elliptic.
// In a sandboxed renderer (nodeIntegration: false), this will fail unless
// this script is processed by a bundler (like Webpack or Parcel) that includes 'elliptic',
// or if 'elliptic' itself is browser-compatible and doesn't rely on Node.js built-ins.
// For a quick test without a bundler, nodeIntegration: true would be needed in main.js,
// but this is not recommended for production.
// A more secure approach is to perform crypto operations in main.js via IPC.
let EC;
try {
    EC = require('elliptic').ec;
} catch (e) {
    console.error("'elliptic' module could not be loaded in renderer. This is expected if nodeIntegration is false and no bundler is used. Crypto operations should be moved to main process via IPC for security.", e);
    // Fallback or provide a message to the user if critical
    alert("Critical cryptographic library 'elliptic' failed to load. Wallet functionality will be limited. Ensure Node.js integration or proper bundling if developing.");
}

const ec = EC ? new EC('p256') : null; // Use 'p256' for SECP256R1
let currentKeyPair = null; // Store the current keypair in memory for the session

// UI Elements
const createWalletBtn = document.getElementById('createWalletBtn');
const loadWalletBtn = document.getElementById('loadWalletBtn');
const privateKeyInput = document.getElementById('privateKeyInput');
const currentAddressEl = document.getElementById('currentAddress');
const newPrivateKeyEl = document.getElementById('newPrivateKey');
const newKeySection = document.getElementById('newKeySection');
const statusMessagesEl = document.getElementById('statusMessages');
const errorMessagesEl = document.getElementById('errorMessages');

function clearMessages() {
    statusMessagesEl.textContent = '';
    statusMessagesEl.classList.add('hidden');
    errorMessagesEl.textContent = '';
    errorMessagesEl.classList.add('hidden');
}

function showStatus(message) {
    clearMessages();
    statusMessagesEl.textContent = message;
    statusMessagesEl.classList.remove('hidden');
}

function showError(message) {
    clearMessages();
    errorMessagesEl.textContent = message;
    errorMessagesEl.classList.remove('hidden');
}

function displayWalletInfo(keyPair, isNew = false) {
    const publicKeyHex = keyPair.getPublic(false, 'hex'); // false for uncompressed
    currentAddressEl.textContent = publicKeyHex; // Using uncompressed pubkey hex as address

    if (isNew) {
        const privateKeyHex = keyPair.getPrivate('hex');
        newPrivateKeyEl.textContent = privateKeyHex;
        newKeySection.classList.remove('hidden');
    } else {
        newKeySection.classList.add('hidden');
        newPrivateKeyEl.textContent = '';
    }
}

if (createWalletBtn) {
    createWalletBtn.addEventListener('click', () => {
        clearMessages();
        if (!ec) {
            showError("Elliptic library not available.");
            return;
        }
        try {
            currentKeyPair = ec.genKeyPair();
            displayWalletInfo(currentKeyPair, true);
            showStatus('New wallet created. SAVE YOUR PRIVATE KEY SECURELY!');
            privateKeyInput.value = ''; // Clear input field
        } catch (error) {
            console.error("Error creating wallet:", error);
            showError('Error creating wallet: ' + error.message);
        }
    });
}

if (loadWalletBtn) {
    loadWalletBtn.addEventListener('click', () => {
        clearMessages();
        if (!ec) {
            showError("Elliptic library not available.");
            return;
        }
        const privateKeyHex = privateKeyInput.value.trim();

        // Basic validation for P256 private key hex (should be 64 hex chars)
        if (!privateKeyHex || privateKeyHex.length !== 64 || !/^[0-9a-fA-F]+$/.test(privateKeyHex)) {
            showError('Invalid private key format. Expected 64 hex characters.');
            return;
        }

        try {
            currentKeyPair = ec.keyFromPrivate(privateKeyHex, 'hex');
            // Validate the key by trying to get public key (catches some invalid private keys)
            currentKeyPair.getPublic();

            displayWalletInfo(currentKeyPair, false);
            showStatus('Wallet loaded successfully from private key.');
        } catch (error) {
            console.error("Error loading wallet from private key:", error);
            currentKeyPair = null;
            currentAddressEl.textContent = 'Error loading wallet. Invalid private key?';
            showError('Error loading wallet: ' + error.message);
        }
    });
}

// Initial state
clearMessages();
newKeySection.classList.add('hidden');
