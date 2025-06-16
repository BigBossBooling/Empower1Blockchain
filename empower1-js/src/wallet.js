// wallet.js - Wallet generation and key management functionality

const EC = require('elliptic').ec;
const ec = new EC('p256'); // SECP256R1 curve, same as Go node and Python CLI wallet

/**
 * Generates a new P256/SECP256R1 key pair.
 * @returns {object} Object containing privateKey (hex), publicKey (hex uncompressed), and address (hex uncompressed public key).
 */
function generateWallet() {
    const keyPair = ec.genKeyPair();
    const privateKeyHex = keyPair.getPrivate('hex');
    // Get uncompressed public key (starts with 04)
    const publicKeyHex = keyPair.getPublic(false, 'hex');

    // For Empower1, the address is currently the uncompressed public key hex.
    const address = publicKeyHex;

    return {
        privateKey: privateKeyHex,
        publicKey: publicKeyHex,
        address: address,
    };
}

/**
 * Loads a wallet (key pair and address) from a private key hex string.
 * @param {string} privateKeyHex - The private key in hexadecimal format.
 * @returns {object|null} Object containing privateKey, publicKey, and address, or null if private key is invalid.
 */
function loadWalletFromPrivateKey(privateKeyHex) {
    if (!privateKeyHex || typeof privateKeyHex !== 'string' || privateKeyHex.length !== 64 || !/^[0-9a-fA-F]+$/.test(privateKeyHex)) {
        console.error("Invalid private key hex format. Expected 64 hex characters.");
        return null;
    }
    try {
        const keyPair = ec.keyFromPrivate(privateKeyHex, 'hex');
        const publicKeyHex = keyPair.getPublic(false, 'hex'); // false for uncompressed
        const address = publicKeyHex; // Address is uncompressed pubkey hex

        return {
            privateKey: privateKeyHex, // The input private key
            publicKey: publicKeyHex,
            address: address,
            keyPair: keyPair // Store the elliptic keypair object for further use (e.g. signing)
        };
    } catch (error) {
        console.error("Error loading wallet from private key:", error.message);
        return null;
    }
}

/**
 * Derives the public key (hex, uncompressed) from a private key hex string.
 * @param {string} privateKeyHex - The private key in hexadecimal format.
 * @returns {string|null} The uncompressed public key hex string, or null if private key is invalid.
 */
function getPublicKeyFromPrivateKey(privateKeyHex) {
    const wallet = loadWalletFromPrivateKey(privateKeyHex);
    return wallet ? wallet.publicKey : null;
}

/**
 * Derives the address from a public key hex string.
 * For Empower1, the address is currently the uncompressed public key hex itself.
 * @param {string} publicKeyHex - The uncompressed public key in hexadecimal format.
 * @returns {string} The address.
 */
function getAddressFromPublicKey(publicKeyHex) {
    // Assuming publicKeyHex is already the uncompressed public key format starting with '04'
    if (!publicKeyHex || typeof publicKeyHex !== 'string' || !publicKeyHex.startsWith('04') || publicKeyHex.length !== 130) {
        console.error("Invalid public key hex format for address derivation.");
        return null;
    }
    return publicKeyHex;
}

module.exports = {
    generateWallet,
    loadWalletFromPrivateKey,
    getPublicKeyFromPrivateKey,
    getAddressFromPublicKey,
    ec // Export ec instance if needed by other modules like transaction.js for key objects
};

// Basic test
if (typeof require !== 'undefined' && require.main === module) {
    const wallet1 = generateWallet();
    console.log("Generated Wallet 1:", wallet1);

    if (wallet1) {
        const wallet2 = loadWalletFromPrivateKey(wallet1.privateKey);
        console.log("Loaded Wallet 2 from Wallet 1's private key:", wallet2);
        if (wallet2 && wallet1.address === wallet2.address) {
            console.log("Wallet generation and loading test: SUCCESS");
        } else {
            console.log("Wallet generation and loading test: FAILED");
        }

        const pubFromPriv = getPublicKeyFromPrivateKey(wallet1.privateKey);
        console.log("Public key from private key:", pubFromPriv);
        if (pubFromPriv === wallet1.publicKey) {
            console.log("getPublicKeyFromPrivateKey test: SUCCESS");
        } else {
            console.log("getPublicKeyFromPrivateKey test: FAILED");
        }

        const addrFromPub = getAddressFromPublicKey(wallet1.publicKey);
        console.log("Address from public key:", addrFromPub);
        if (addrFromPub === wallet1.address) {
            console.log("getAddressFromPublicKey test: SUCCESS");
        } else {
            console.log("getAddressFromPublicKey test: FAILED");
        }
    }
}
