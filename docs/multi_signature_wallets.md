# Multi-Signature Wallets in Empower1

This document outlines the native M-of-N multi-signature scheme for the Empower1 blockchain. This scheme allows multiple parties to collectively control funds or authorize actions, requiring a threshold (M) of signatures from a defined set (N) of authorized public keys.

## 1. Overview

A multi-signature (multi-sig) wallet requires M signatures from a total of N authorized participants to approve a transaction. This is useful for:
-   **Enhanced Security:** Requiring multiple parties to sign reduces the risk of single-point-of-failure or unauthorized access.
-   **Shared Control:** Distributing control of assets among multiple stakeholders (e.g., a company's board, partners).
-   **Escrow Services:** A trusted third party can be one of the signers in an M-of-N setup.

For this implementation, we will adopt a scheme where the multi-signature configuration (M value and N public keys) is included directly within each transaction originating from a multi-sig arrangement.

## 2. Multi-Signature Identifier (Address)

A multi-signature entity needs a unique identifier, akin to an address. This identifier can be derived deterministically from its configuration.

*   **Derivation Method:**
    1.  Collect all N `AuthorizedPublicKeys` (as byte arrays, typically uncompressed SECP256R1 public keys).
    2.  Sort these public key byte arrays lexicographically (to ensure order-independence).
    3.  Concatenate the sorted public keys.
    4.  Prepend the M value (e.g., as a 1-byte or 2-byte representation).
    5.  Hash the resulting byte string (e.g., using SHA256).
    6.  The resulting hash (or a truncated/encoded version of it) serves as the multi-sig identifier/address.

    *Example: `MultiSigAddress = SHA256(M_value + sorted(PubKey1) + sorted(PubKey2) + ... + sorted(PubKeyN))`*

    For on-chain representation and usage in the `From` field of a transaction, this byte array can be hex-encoded.

## 3. Transaction Structure

The `Transaction` struct in `internal/core/transaction.go` will be modified to support multi-sig operations.

*   **Key Fields for Multi-Sig:**
    *   `TxType`: Set to a new type, e.g., `TxMultiSig` (or reuse `TxStandard` but populate multi-sig fields). For now, we will assume a standard transaction can *be* a multi-sig transaction if multi-sig fields are populated. The `From` field would then represent the multi-sig identifier.
    *   `From`: This field will hold the Multi-Signature Identifier (Address) derived as described above.
    *   `RequiredSignatures (M)`: A `uint32` indicating how many signatures are required (the M in M-of-N).
    *   `AuthorizedPublicKeys (N_keys)`: A list `[][]byte` containing all N public keys authorized to sign for this multi-sig setup. These keys must be sorted lexicographically before deriving the multi-sig address to ensure consistency.
    *   `Signers`: A list `[]SignerInfo` where `SignerInfo` is a struct:
        ```go
        type SignerInfo struct {
            PublicKey []byte // The public key of the actual signer
            Signature []byte // The signature provided by this signer
        }
        ```
        This list will store the collected signatures.
    *   The existing single-signer fields (`PublicKey`, `Signature` on the main `Transaction` struct) would be unused or empty for a multi-sig transaction.

*   **Data to be Signed:**
    Each signer in the `Signers` list signs a hash of the core transaction payload *and* the multi-sig configuration. This ensures that the intent of the transaction and the authority under which it's being made are both signed.
    The data for hashing would include:
    *   `Timestamp`
    *   `To` (recipient address for standard transfers)
    *   `Amount`
    *   `Fee`
    *   `TxType` (if explicit types are used beyond standard)
    *   `RequiredSignatures (M)`
    *   `AuthorizedPublicKeys (N_keys)` (sorted list of all N public keys)
    *   `Nonce` or sequence number (if added for replay protection for the multi-sig entity)
    *   Any other relevant payload fields like `FunctionName`, `Arguments` for contract calls, etc.

    Essentially, everything *except* the `Signers` list itself is hashed.

## 4. Validation Rules (Go Node)

When a node receives a transaction claiming to be from a multi-sig entity (e.g., identified by its `From` address being a multi-sig identifier and/or `RequiredSignatures > 0`):

1.  **Verify Multi-Sig Identifier (Address):** The `From` field should correctly correspond to the hash derived from `RequiredSignatures` and `AuthorizedPublicKeys` present in the transaction.
2.  **Sufficient Signatures:** `len(Signers)` must be greater than or equal to `RequiredSignatures (M)`.
3.  **Authorized Signers:** Each `PublicKey` in the `Signers` list must be present in the `AuthorizedPublicKeys (N_keys)` list.
4.  **Valid Signatures:** For each `SignerInfo` in `Signers`:
    *   Its `Signature` must be a valid signature over the common transaction hash (derived from payload + M + N_keys).
    *   The signature must verify against the `SignerInfo.PublicKey`.
5.  **No Duplicate Signers:** The `PublicKey` for each entry in `Signers` must be unique. A single authorized party cannot provide multiple signatures for the same transaction.
6.  **Sufficient Unique Authorized Signers:** The number of unique, valid signatures from authorized public keys must meet the M threshold.

## 5. CLI Wallet Workflow (File-Based)

The `cli-wallet` will facilitate a multi-step, offline process for creating and signing multi-sig transactions.

*   **Configuration (`multisig.py`):**
    *   Users can define an M-of-N setup by specifying M and a list of N public key hex strings.
    *   The tool will generate and display the corresponding multi-sig identifier/address.
    *   This configuration (M, N keys, multi-sig address) is saved to a local JSON file (e.g., `multisig_config.json`).
*   **Initiation (`main.py multisig-initiate-tx`):**
    *   Takes the `multisig_config.json`, recipient, amount, etc., as input.
    *   Creates a transaction structure including:
        *   The payload (recipient, amount).
        *   The M value and N public keys (from the config file).
        *   An empty list for `Signers`.
    *   Saves this "pending" transaction to a new JSON file (e.g., `pending_tx.json`). This file can then be passed to the required number of co-signers.
*   **Signing (`main.py multisig-sign-tx`):**
    *   Takes the `pending_tx.json` file and the path to a signer's wallet PEM file.
    *   Loads the pending transaction.
    *   Loads the signer's private key.
    *   Calculates the hash of the transaction payload + multi-sig config.
    *   Signs this hash.
    *   Adds a `SignerInfo` (signer's public key + signature) to the `Signers` list in the transaction data.
    *   Saves the updated transaction data back to a file (either overwriting or creating a new one, e.g., `tx_signed_by_1.json`). This file is then passed to the next signer.
*   **Broadcasting (`main.py multisig-broadcast-tx`):**
    *   Takes the (now sufficiently) signed transaction JSON file.
    *   Optionally performs local verification (checks if M signatures from authorized keys are present).
    *   Submits the complete transaction to the Empower1 node via the `/tx/submit` RPC endpoint.

This file-based workflow allows co-signers to sign independently without needing direct online communication between their wallets for the signing process.

## 6. CLI Wallet Usage Examples

The `cli-wallet/main.py` script provides commands to manage and use multi-signature configurations and transactions.

**Prerequisites:** Ensure the CLI wallet is set up as described in `cli-wallet/README.md`.

**1. Create a Multi-Signature Configuration:**

Suppose you want a 2-of-3 multi-signature setup. You have three wallet PEM files: `signer1.pem`, `signer2.pem`, `signer3.pem`.
Passwords are `p1`, `p2`, `p3` respectively (use empty string or appropriate handling for no password).

```bash
# In cli-wallet directory
python main.py multisig create-config \
    -m 2 \
    --signer-wallet signer1.pem --signer-password p1 \
    --signer-wallet signer2.pem --signer-password p2 \
    --signer-wallet signer3.pem --signer-password p3 \
    --outfile our_multisig_config.json
```
This will:
- Load each signer's wallet to get their public key.
- Derive the multi-sig address/identifier.
- Save the configuration (M=2, N=3, list of sorted authorized public keys, multi-sig address) to `our_multisig_config.json`.
- Print the multi-sig address to the console. Note this address.

**2. Initiate a Multi-Signature Transaction:**

Using the configuration created above, initiate a transaction. Let the multi-sig entity send 500 units to a recipient address `<recipient_address_hex>`.

```bash
python main.py multisig initiate-tx \
    --config our_multisig_config.json \
    --to <recipient_address_hex> \
    --amount 500 \
    --fee 5 \
    --tx-type standard \
    --outfile pending_payment.json
```
This creates `pending_payment.json` containing the transaction details, M, N keys, and an empty list for signatures. The transaction ID (hash of content + M/N config) is also calculated and stored.

**3. Collect Signatures:**

Share `pending_payment.json` with the authorized signers. Each required signer will use their wallet to add their signature.

*   **Signer 1 signs:**
    ```bash
    python main.py multisig sign-tx \
        --pending-tx pending_payment.json \
        --wallet signer1.pem --password p1 \
        --outfile pending_payment_signed1.json
        # Or overwrite: --outfile pending_payment.json
    ```
    This updates the transaction file with Signer 1's signature.

*   **Signer 2 signs (using the output from Signer 1):**
    ```bash
    python main.py multisig sign-tx \
        --pending-tx pending_payment_signed1.json \
        --wallet signer2.pem --password p2 \
        --outfile pending_payment_signed2.json
    ```
    Now `pending_payment_signed2.json` contains two signatures.

**4. Broadcast the Transaction:**

Once enough signatures are collected (M=2 in this case), anyone can broadcast the transaction.

```bash
python main.py multisig broadcast-tx \
    --signed-tx pending_payment_signed2.json \
    --node-url http://localhost:18001
    # (Adjust node URL as needed)
```
The node will receive the transaction, validate all signatures and configurations, and if valid, add it to the mempool.

This workflow allows for secure, offline collection of multiple signatures before a transaction is authorized and broadcasted.
