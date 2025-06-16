// empower1-js/src/index.js - Main SDK Export

const { RPCClient } = require('./rpc');
const {
    generateWallet,
    loadWalletFromPrivateKey,
    getPublicKeyFromPrivateKey,
    getAddressFromPublicKey
} = require('./wallet');
const {
    TX_STANDARD,
    TX_CONTRACT_DEPLOY,
    TX_CONTRACT_CALL,
    prepareDataForHashing,
    calculateTransactionHash,
    signTransaction,
    preparePayloadForSubmit
} = require('./transaction');

// Utility or helper functions that might be useful for consumers
const utils = {
    // Add any general utility functions here if needed in the future
    // For example, hexToBytes, bytesToHex if not relying on Buffer or other libs externally
};

module.exports = {
    // RPC
    RPCClient,

    // Wallet
    Wallet: { // Group wallet functions under a Wallet namespace
        generateWallet,
        loadWalletFromPrivateKey,
        getPublicKeyFromPrivateKey,
        getAddressFromPublicKey
    },

    // Transaction
    Transaction: { // Group transaction functions under a Transaction namespace
        TX_STANDARD,
        TX_CONTRACT_DEPLOY,
        TX_CONTRACT_CALL,
        prepareDataForHashing,
        calculateTransactionHash,
        signTransaction,
        preparePayloadForSubmit
        // TODO: Add helper functions here to construct specific transaction objects
        // e.g., createStandardTransferTx, createContractDeployTx, createContractCallTx
        // These would initialize the txObject with correct tx_type and fields.
    },

    // Utilities (if any)
    utils
};

// Example of how one might add transaction creation helper functions later:
/*
module.exports.Transaction.createStandardTransfer = function(fromAddressHex, toAddressHex, amount, fee, timestamp, publicKeyHex) {
    return {
        timestamp: timestamp || Date.now() * 1000000,
        from_address_hex: fromAddressHex,
        public_key_hex: publicKeyHex || fromAddressHex, // Assuming fromAddressHex is derived from pubkey
        to_address_hex: toAddressHex,
        amount: amount,
        fee: fee || 0,
        tx_type: TX_STANDARD,
        // Initialize other fields to defaults or null
        contract_code_bytes: null,
        target_contract_address_hex: null,
        function_name: null,
        arguments_bytes: null,
        required_signatures: 0,
        authorized_public_keys_hex: [],
        signers: [],
        id_hex: null,
        signature_hex: null,
    };
};
*/
