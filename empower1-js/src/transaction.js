// transaction.js - Transaction creation, hashing, and signing for Empower1

const EC = require('elliptic').ec;
const ec = new EC('p256'); // SECP256R1 curve
const sha256 = require('js-sha256').sha256; // For SHA256 hashing
const { Buffer } = require('buffer'); // For Base64 encoding, especially in browser environments

// Transaction Types (mirroring Go's core.TransactionType)
const TX_STANDARD = "standard";
const TX_CONTRACT_DEPLOY = "contract_deployment";
const TX_CONTRACT_CALL = "contract_call";

/**
 * Prepares the transaction data for hashing according to the canonical JSON format.
 * This MUST match the Go node's `core.TxDataForJSONHashing` struct and its JSON marshalling,
 * including alphabetical sorting of keys and base64 encoding for byte arrays.
 * @param {object} txObject - The transaction object (before signing, or for re-hashing).
 * @returns {string} The canonical JSON string to be hashed.
 */
function prepareDataForHashing(txObject) {
    const dataToHash = {};

    // Order of fields for JSON (alphabetical, matching Go's TxDataForJSONHashing struct tags after sorting)
    // Amount, Arguments, AuthorizedPublicKeys, ContractCode, Fee, From, FunctionName,
    // PublicKey, RequiredSignatures, TargetContractAddress, Timestamp, To, TxType

    // Amount (omitempty in Go if 0, but for hashing, include if value is 0 and type expects it)
    if (txObject.tx_type === TX_STANDARD || typeof txObject.amount === 'number') {
        dataToHash.Amount = txObject.amount || 0;
    }
    if (txObject.tx_type === TX_CONTRACT_CALL && typeof txObject.amount === 'number') { // Value sent with call
        dataToHash.Amount = txObject.amount || 0;
    }


    // Arguments (base64 of []byte) - for contract calls
    if (txObject.arguments_bytes instanceof Uint8Array && txObject.tx_type === TX_CONTRACT_CALL) {
        dataToHash.Arguments = Buffer.from(txObject.arguments_bytes).toString('base64');
    } else if (txObject.arguments_bytes && typeof txObject.arguments_bytes === 'string') { // Already base64
        dataToHash.Arguments = txObject.arguments_bytes;
    }


    // AuthorizedPublicKeys (list of hex strings) - for multi-sig
    if (txObject.required_signatures > 0 && Array.isArray(txObject.authorized_public_keys_hex)) {
        // These must be sorted HEX STRINGS for canonical representation, as done in Python and Go.
        dataToHash.AuthorizedPublicKeys = [...txObject.authorized_public_keys_hex].sort();
    }

    // ContractCode (base64 of []byte) - for deployment
    if (txObject.contract_code_bytes instanceof Uint8Array && txObject.tx_type === TX_CONTRACT_DEPLOY) {
        dataToHash.ContractCode = Buffer.from(txObject.contract_code_bytes).toString('base64');
    } else if (txObject.contract_code_bytes && typeof txObject.contract_code_bytes === 'string') {
        dataToHash.ContractCode = txObject.contract_code_bytes;
    }

    dataToHash.Fee = txObject.fee || 0;
    dataToHash.From = txObject.from_address_hex; // Sender PubKey (single) or MultiSig ID (multi)

    if (txObject.function_name && txObject.tx_type === TX_CONTRACT_CALL) {
        dataToHash.FunctionName = txObject.function_name;
    }

    // PublicKey (single-signer's actual key, hex string)
    // Omitted if this is a multi-sig transaction where From is the multi-sig ID.
    if (txObject.public_key_hex && (txObject.required_signatures === 0 || !txObject.required_signatures)) {
        dataToHash.PublicKey = txObject.public_key_hex;
    }

    if (txObject.required_signatures > 0) {
        dataToHash.RequiredSignatures = txObject.required_signatures;
    }

    if (txObject.target_contract_address_hex && txObject.tx_type === TX_CONTRACT_CALL) {
        dataToHash.TargetContractAddress = txObject.target_contract_address_hex;
    }

    dataToHash.Timestamp = txObject.timestamp;

    if (txObject.to_address_hex && txObject.tx_type === TX_STANDARD) {
        dataToHash.To = txObject.to_address_hex;
    }

    dataToHash.TxType = txObject.tx_type;

    // Create canonical JSON: sort keys, no spaces, UTF-8.
    const sortedKeys = Object.keys(dataToHash).sort();
    const canonicalObject = {};
    for (const key of sortedKeys) {
        canonicalObject[key] = dataToHash[key];
    }
    return JSON.stringify(canonicalObject);
}

/**
 * Calculates the SHA256 hash of the transaction data.
 * @param {object} txObject - The transaction object.
 * @returns {string} The SHA256 hash hex string.
 */
function calculateTransactionHash(txObject) {
    const canonicalJson = prepareDataForHashing(txObject);
    const hash = sha256.create();
    hash.update(canonicalJson);
    return hash.hex();
}

/**
 * Signs a transaction object.
 * @param {object} txObject - The transaction object to sign (must have all fields for hashing set).
 * @param {string} privateKeyHex - The private key hex of the signer.
 * @returns {object} A new transaction object with id_hex, public_key_hex (of signer), and signature_hex populated.
 *                   For multi-sig, it adds a SignerInfo object to txObject.signers.
 */
function signTransaction(txObject, privateKeyHex) {
    if (!txObject || !privateKeyHex) {
        throw new Error("Transaction object and private key hex are required for signing.");
    }

    const keyPair = ec.keyFromPrivate(privateKeyHex, 'hex');
    const signerPublicKeyHex = keyPair.getPublic(false, 'hex');

    const txToSign = { ...txObject }; // Work on a copy

    if (txToSign.required_signatures > 0 && Array.isArray(txToSign.authorized_public_keys_hex)) {
        // Multi-signature signing
        if (!txToSign.authorized_public_keys_hex.includes(signerPublicKeyHex)) {
            throw new Error(`Signer ${signerPublicKeyHex} is not in the authorized list.`);
        }
        if (!txToSign.signers) {
            txToSign.signers = [];
        }
        // Check if already signed by this key
        if (txToSign.signers.some(s => s.publicKeyHex === signerPublicKeyHex)) {
            console.warn(`Transaction already signed by ${signerPublicKeyHex}.`);
            return txToSign; // Or throw error, or just return existing
        }

        // For multi-sig, 'From' is the multi-sig ID. 'PublicKey' field (main tx) is not used.
        txToSign.public_key_hex = null;
        txToSign.signature_hex = null;

    } else {
        // Single-signer transaction
        txToSign.from_address_hex = signerPublicKeyHex; // From is the signer's address
        txToSign.public_key_hex = signerPublicKeyHex;   // PublicKey is also the signer's
        txToSign.required_signatures = 0;
        txToSign.authorized_public_keys_hex = [];
        txToSign.signers = [];
    }

    const txHashHex = calculateTransactionHash(txToSign);
    txToSign.id_hex = txHashHex; // Set ID before individual signing for multi-sig consistency (ID is of payload+config)

    const signature = keyPair.sign(Buffer.from(txHashHex, 'hex'), { canonical: true }); // DER encoded
    const signatureHex = signature.toDER('hex');

    if (txToSign.required_signatures > 0) {
        txToSign.signers.push({
            publicKeyHex: signerPublicKeyHex,
            signatureHex: signatureHex
        });
        // Sort signers for canonical representation if needed (e.g. before final broadcast)
        txToSign.signers.sort((a, b) => a.publicKeyHex.localeCompare(b.publicKeyHex));
    } else {
        txToSign.signature_hex = signatureHex;
    }

    return txToSign;
}


/**
 * Prepares the final JSON payload for submitting a transaction to the Go node's /tx/submit.
 * Ensures all byte arrays are Base64 encoded.
 * @param {object} signedTxObject - The signed transaction object.
 * @returns {object} The payload ready for JSON stringification and sending.
 */
function preparePayloadForSubmit(signedTxObject) {
    const payload = {
        ID: signedTxObject.id_hex ? Buffer.from(signedTxObject.id_hex, 'hex').toString('base64') : null,
        Timestamp: signedTxObject.timestamp,
        From: Buffer.from(signedTxObject.from_address_hex, 'hex').toString('base64'),
        PublicKey: signedTxObject.public_key_hex ? Buffer.from(signedTxObject.public_key_hex, 'hex').toString('base64') : null,
        Signature: signedTxObject.signature_hex ? Buffer.from(signedTxObject.signature_hex, 'hex').toString('base64') : null,
        TxType: signedTxObject.tx_type,
        To: signedTxObject.to_address_hex ? Buffer.from(signedTxObject.to_address_hex, 'hex').toString('base64') : null,
        Amount: signedTxObject.amount,
        Fee: signedTxObject.fee,
        ContractCode: signedTxObject.contract_code_bytes instanceof Uint8Array ? Buffer.from(signedTxObject.contract_code_bytes).toString('base64') : (typeof signedTxObject.contract_code_bytes === 'string' ? signedTxObject.contract_code_bytes : null),
        TargetContractAddress: signedTxObject.target_contract_address_hex ? Buffer.from(signedTxObject.target_contract_address_hex, 'hex').toString('base64') : null,
        FunctionName: signedTxObject.function_name || "", // Ensure empty string not null
        Arguments: signedTxObject.arguments_bytes instanceof Uint8Array ? Buffer.from(signedTxObject.arguments_bytes).toString('base64') : (typeof signedTxObject.arguments_bytes === 'string' ? signedTxObject.arguments_bytes : null),
        RequiredSignatures: signedTxObject.required_signatures || 0,
        AuthorizedPublicKeys: signedTxObject.authorized_public_keys_hex
            ? signedTxObject.authorized_public_keys_hex.map(pkHex => Buffer.from(pkHex, 'hex').toString('base64'))
            : [],
        Signers: signedTxObject.signers
            ? signedTxObject.signers.map(s => ({
                PublicKey: Buffer.from(s.publicKeyHex, 'hex').toString('base64'),
                Signature: Buffer.from(s.signatureHex, 'hex').toString('base64')
            }))
            : [],
    };
    // Remove null fields to match Go's omitempty behavior more closely if needed
    return Object.fromEntries(Object.entries(payload).filter(([_, v]) => v !== null));
}


module.exports = {
    TX_STANDARD,
    TX_CONTRACT_DEPLOY,
    TX_CONTRACT_CALL,
    prepareDataForHashing,
    calculateTransactionHash,
    signTransaction,
    preparePayloadForSubmit
};

// Basic test
if (typeof require !== 'undefined' && require.main === module) {
    const { generateWallet } = require('./wallet'); // Assuming wallet.js is in the same directory

    const wallet1 = generateWallet();
    const wallet2 = generateWallet(); // Recipient

    const txObj = {
        timestamp: Date.now() * 1000000, // Nanoseconds
        from_address_hex: wallet1.address, // For single signer, From and PublicKey are from wallet1
        public_key_hex: wallet1.publicKey,
        to_address_hex: wallet2.address,
        amount: 100,
        fee: 10,
        tx_type: TX_STANDARD,
        // No contract or multi-sig fields for this simple test
    };

    console.log("Original Tx Object for Hashing:", txObj);
    const canonicalJson = prepareDataForHashing(txObj);
    console.log("Canonical JSON for Hashing:", canonicalJson);
    const hashHex = calculateTransactionHash(txObj);
    console.log("Calculated Hash (ID):", hashHex);

    const signedTx = signTransaction(txObj, wallet1.privateKey);
    console.log("\nSigned Transaction Object:", signedTx);

    // Verify signature (conceptual, elliptic doesn't have simple verify on keypair)
    const keyPairForVerify = ec.keyFromPublic(signedTx.public_key_hex, 'hex');
    const isValid = keyPairForVerify.verify(Buffer.from(signedTx.id_hex, 'hex'), Buffer.from(signedTx.signature_hex, 'hex'));
    console.log("Is signature valid (local JS check)?", isValid);
    if (!isValid) {
        console.error("Local signature verification FAILED");
    } else {
        console.log("Local signature verification SUCCEEDED");
    }

    const payloadForGo = preparePayloadForSubmit(signedTx);
    console.log("\nPayload for Go Node (/tx/submit):", JSON.stringify(payloadForGo, null, 2));
}
