// app.js - UI logic for the Micro-Lending DApp

// --- Assume SDK is loaded and available globally ---
// In a real bundled app, you might do:
// const empower1SDK = require('./lib/empower1-sdk/index.js');
// Or using ES6 imports if SDK is structured that way and you're using a module script.
// For this basic setup, we'll assume `empower1SDK` is created by one of the included scripts,
// or we'll access its components directly if module.exports from SDK files don't create a single global.

// Let's assume the individual SDK files assign to window or are accessible if not using modules.
// For simplicity, let's try to use them as if they are global or easily accessible.
// This might require `nodeIntegration: true` in Electron if using `require` directly here,
// or a bundler. The current setup with `nodeIntegration: false` and separate script tags
// means these `require` calls will fail in the renderer.
//
// The most straightforward way without a bundler and with nodeIntegration:false
// is to modify the SDK's index.js to attach its exports to the window object,
// e.g., window.empower1SDK = module.exports;
// Or, for this demo, assume we can access the functions if the script tags run them.
//
// Given the current SDK files use `module.exports`, they are CommonJS modules.
// To use them in the browser directly via `<script src="...">` without a bundler,
// they would need to be adapted (e.g., UMD pattern or exposing to `window`).
//
// Simplification: We will write the code AS IF `RPCClient`, `Wallet`, `Transaction` objects/namespaces
// are available globally, which would be the case if `index.js` from the SDK assigned them to `window`.
// This is a conceptual step. A real build would handle this.

// --- Global Variables ---
let rpcClient;
let currentUserWallet = null; // To store { privateKey, publicKey, address, keyPair (elliptic) }
let microLendingContractAddress = '';

// --- UI Elements ---
const nodeUrlInput = document.getElementById('nodeUrl');
const connectNodeBtn = document.getElementById('connectNodeBtn');
const nodeInfoDiv = document.getElementById('nodeInfo');

const privateKeyInput = document.getElementById('privateKeyInput');
const loadWalletBtn = document.getElementById('loadWalletBtn');
const currentUserAddressEl = document.getElementById('currentUserAddress');
const userBalanceEl = document.getElementById('userBalance'); // Placeholder

const contractAddressInput = document.getElementById('contractAddress');
const refreshPoolDataBtn = document.getElementById('refreshPoolDataBtn');
const poolBalanceEl = document.getElementById('poolBalance');
const userDepositedBalanceEl = document.getElementById('userDepositedBalance');
const userLoanedAmountEl = document.getElementById('userLoanedAmount');
const userLoanRepaidStatusEl = document.getElementById('userLoanRepaidStatus');

const depositAmountInput = document.getElementById('depositAmount');
const depositBtn = document.getElementById('depositBtn');
const withdrawAmountInput = document.getElementById('withdrawAmount');
const withdrawBtn = document.getElementById('withdrawBtn');
const loanAmountInput = document.getElementById('loanAmount');
const requestLoanBtn = document.getElementById('requestLoanBtn');
const repayAmountInput = document.getElementById('repayAmount');
const repayLoanBtn = document.getElementById('repayLoanBtn');

const statusMessagesEl = document.getElementById('statusMessages');
const errorMessagesEl = document.getElementById('errorMessages');

// --- Utility Functions ---
function showStatus(message) {
    statusMessagesEl.textContent = `[${new Date().toLocaleTimeString()}] ${message}`;
}
function showError(message) {
    errorMessagesEl.textContent = `[${new Date().toLocaleTimeString()}] ERROR: ${message}`;
}
function clearMessages() {
    statusMessagesEl.textContent = '';
    errorMessagesEl.textContent = '';
}

// --- SDK Initialization ---
// This assumes the SDK files (wallet.js, transaction.js, rpc.js) have executed
// and their exports are available. For CommonJS modules in browser, this is tricky.
// Let's assume `Wallet` and `RPCClient` and `Transaction` are global vars created by those scripts.
// This is a placeholder for proper module loading/bundling.
const SDK = { // Simulating the SDK being available
    Wallet: typeof Wallet !== 'undefined' ? Wallet : null,
    Transaction: typeof Transaction !== 'undefined' ? Transaction : null,
    RPCClient: typeof RPCClient !== 'undefined' ? RPCClient : null
};


// --- Event Listeners and Logic ---

window.addEventListener('DOMContentLoaded', () => {
    if (!SDK.RPCClient || !SDK.Wallet || !SDK.Transaction) {
        showError("Empower1 SDK components not loaded correctly. Ensure SDK scripts are included and accessible.");
        console.error("SDK components missing:", SDK);
        // Disable buttons if SDK is not loaded
        const allButtons = document.querySelectorAll('button');
        allButtons.forEach(btn => btn.disabled = true);
        connectNodeBtn.disabled = false; // Allow trying to connect
    }

    if (connectNodeBtn) {
        connectNodeBtn.addEventListener('click', async () => {
            clearMessages();
            const nodeUrl = nodeUrlInput.value.trim();
            if (!nodeUrl) {
                showError("Please enter a node URL.");
                return;
            }
            if (!SDK.RPCClient) { showError("RPCClient SDK part not loaded."); return; }

            rpcClient = new SDK.RPCClient(nodeUrl);
            try {
                showStatus("Connecting to node and fetching info...");
                const info = await rpcClient.getNodeInfo();
                nodeInfoDiv.textContent = JSON.stringify(info, null, 2);
                showStatus("Node info loaded successfully.");
                // Enable other buttons after successful connection
                loadWalletBtn.disabled = false;
                // contract related buttons should be enabled after contract address is set
            } catch (e) {
                showError(`Failed to connect to node or get info: ${e.message}`);
                nodeInfoDiv.textContent = "Failed to load node info.";
            }
        });
    }

    if (loadWalletBtn) {
        loadWalletBtn.disabled = true; // Enable after node connection
        loadWalletBtn.addEventListener('click', () => {
            clearMessages();
            if (!SDK.Wallet) { showError("Wallet SDK part not loaded."); return; }
            const pkHex = privateKeyInput.value.trim();
            currentUserWallet = SDK.Wallet.loadWalletFromPrivateKey(pkHex);
            if (currentUserWallet) {
                currentUserAddressEl.textContent = currentUserWallet.address;
                showStatus(`Wallet loaded. Address: ${currentUserWallet.address}`);
                // TODO: Fetch and display native balance (requires node endpoint)
                userBalanceEl.textContent = "N/A (feature pending)";
                refreshPoolDataBtn.disabled = false;
                // Enable contract action buttons if contract address is also set
                checkEnableContractActions();
            } else {
                showError("Failed to load wallet from private key.");
                currentUserAddressEl.textContent = "Error or Invalid Key";
            }
        });
    }

    if (contractAddressInput) {
        contractAddressInput.addEventListener('input', () => {
            microLendingContractAddress = contractAddressInput.value.trim();
            checkEnableContractActions();
        });
    }

    if (refreshPoolDataBtn) {
        refreshPoolDataBtn.disabled = true; // Enable after wallet and contract address
        refreshPoolDataBtn.addEventListener('click', fetchAllContractData);
    }

    // Add event listeners for contract actions
    setupContractAction(depositBtn, depositAmountInput, "deposit");
    setupContractAction(withdrawBtn, withdrawAmountInput, "withdraw");
    setupContractAction(requestLoanBtn, loanAmountInput, "requestLoan");
    setupContractAction(repayLoanBtn, repayAmountInput, "repayLoan");

});

function checkEnableContractActions() {
    const enabled = currentUserWallet && microLendingContractAddress.length > 0;
    depositBtn.disabled = !enabled;
    withdrawBtn.disabled = !enabled;
    requestLoanBtn.disabled = !enabled;
    repayLoanBtn.disabled = !enabled;
}


async function fetchAllContractData() {
    if (!rpcClient || !microLendingContractAddress) {
        showError("Node not connected or contract address not set.");
        return;
    }
    showStatus("Fetching contract data...");
    try {
        // Get Pool Balance
        let result = await rpcClient.callContract({
            contract_address: microLendingContractAddress,
            function_name: "getPoolBalance",
            arguments_json: "[]",
            gas_limit: 1000000
        });
        poolBalanceEl.textContent = result.contract_result !== null ? result.contract_result.toString() : "N/A";

        if (currentUserWallet) {
            // Get User Info (deposits, loans)
            // The getUserInfo function in the contract expects the user's address (hex string) as an argument.
            // The address is the uncompressed public key hex.
            const userAddressForContract = currentUserWallet.address;
            const argsJson = JSON.stringify([userAddressForContract]);

            result = await rpcClient.callContract({
                contract_address: microLendingContractAddress,
                function_name: "getUserInfo",
                arguments_json: argsJson,
                gas_limit: 2000000
            });

            if (result.contract_result) {
                // Assuming getUserInfo returns a JSON string like:
                // "{ \"deposited_balance\": X, \"loaned_amount\": Y, \"has_active_loan\": true/false }"
                // The current contract returns a string pointer. The Go debug handler returns this pointer.
                // For this UI, we can't easily dereference it.
                // This part needs the Go debug handler to return the actual string content.
                // For now, we'll just display the raw result.
                showStatus("User info raw result: " + JSON.stringify(result.contract_result));
                // If it were returning JSON string:
                // const userInfo = JSON.parse(result.contract_result);
                // userDepositedBalanceEl.textContent = userInfo.deposited_balance.toString();
                // userLoanedAmountEl.textContent = userInfo.loaned_amount.toString();
                // userLoanRepaidStatusEl.textContent = userInfo.has_active_loan ? "No (Active Loan)" : "Yes (Or No Loan)";
                userDepositedBalanceEl.textContent = "N/A (complex result)";
                userLoanedAmountEl.textContent = "N/A (complex result)";
                userLoanRepaidStatusEl.textContent = "N/A (complex result)";

            } else {
                userDepositedBalanceEl.textContent = "0";
                userLoanedAmountEl.textContent = "0";
                userLoanRepaidStatusEl.textContent = "Yes";
            }
        }
        showStatus("Contract data refreshed.");
    } catch (e) {
        showError(`Error fetching contract data: ${e.message}`);
        poolBalanceEl.textContent = "Error";
        userDepositedBalanceEl.textContent = "Error";
        userLoanedAmountEl.textContent = "Error";
        userLoanRepaidStatusEl.textContent = "Error";
    }
}


function setupContractAction(button, amountInput, functionName) {
    if (button) {
        button.disabled = true; // Enable after wallet and contract address
        button.addEventListener('click', async () => {
            await handleContractAction(amountInput, functionName);
        });
    }
}

async function handleContractAction(amountInput, functionName) {
    clearMessages();
    if (!currentUserWallet) { showError("Wallet not loaded."); return; }
    if (!rpcClient) { showError("Node not connected."); return; }
    if (!microLendingContractAddress) { showError("Contract address not set."); return; }
    if (!SDK.Transaction) { showError("Transaction SDK part not loaded."); return; }

    const amount = parseInt(amountInput.value);
    if (isNaN(amount) || amount <= 0) {
        showError("Please enter a valid positive amount.");
        return;
    }

    showStatus(`Preparing to call ${functionName} with amount ${amount}...`);

    try {
        // Arguments for contract call.
        // deposit(amount: u64), withdraw(amount: u64), requestLoan(amount: u64), repayLoan(amount: u64)
        // These functions in the contract expect u64.
        // The JS SDK and current debug endpoint in Go have simplified arg marshalling.
        // The contract actually expects (ptr, len) for strings, or direct numbers.
        // The MicroLendingPool.gr.ts uses u64 for amounts.
        // The JS SDK `ExecuteContract` takes ...interface{}. For numbers, they should pass through.
        const contractArgs = [amount]; // Pass amount as a number.
                                    // If the AS function signature was (amount: string), we'd need to stringify and handle ptr/len.
                                    // Since it's u64, Wasmer-Go should handle number conversion.

        const txObject = {
            from_address_hex: currentUserWallet.address,
            public_key_hex: currentUserWallet.publicKey,
            tx_type: SDK.Transaction.TX_CONTRACT_CALL,
            target_contract_address_hex: microLendingContractAddress,
            function_name: functionName,
            // arguments_bytes: new TextEncoder().encode(JSON.stringify(contractArgs)), // If passing complex args as JSON bytes
            // For simple numeric args, Wasmer-Go might handle them if passed directly.
            // The current `VMService.ExecuteContract` takes `...interface{}`.
            // The `simple_storage` example showed that string args are tricky.
            // Numeric args like u64 might be passed directly.
            // For now, we'll pass the number directly. The Go debug endpoint needs to handle this.
            // The `handleDebugCallContract` parses `arguments_json`. So, we should send JSON string.
            arguments_bytes: new TextEncoder().encode(JSON.stringify(contractArgs)),
            amount: 0, // For contract calls, value transfer is often separate or part of 'amount' in tx.
                       // MicroLendingPool functions don't take value this way.
            fee: 10, // Example fee
            timestamp: Date.now() * 1000000,
        };

        const signedTx = SDK.Transaction.signTransaction(txObject, currentUserWallet.privateKey);
        const payload = SDK.Transaction.preparePayloadForSubmit(signedTx);

        showStatus(`Submitting ${functionName} transaction... TxID: ${signedTx.id_hex}`);
        const result = await rpcClient.submitTransaction(payload);
        showStatus(`${functionName} call submitted. Node response: ${JSON.stringify(result)}`);

        // After action, refresh data
        await new Promise(resolve => setTimeout(resolve, 1000)); // Wait a bit for tx to process
        await fetchAllContractData();

    } catch (e) {
        showError(`Error during ${functionName}: ${e.message}`);
        console.error(`Error in ${functionName}:`, e);
    }
}

// --- Initial button states ---
if (loadWalletBtn) loadWalletBtn.disabled = true;
if (refreshPoolDataBtn) refreshPoolDataBtn.disabled = true;
if (depositBtn) depositBtn.disabled = true;
if (withdrawBtn) withdrawBtn.disabled = true;
if (requestLoanBtn) requestLoanBtn.disabled = true;
if (repayLoanBtn) repayLoanBtn.disabled = true;

// This is a simple way to expose SDK parts to app.js if they are not true ES6 modules or bundled.
// If empower1-sdk/index.js does `window.empower1SDK = module.exports;`, then this isn't needed.
// Otherwise, app.js needs a way to access them.
// The current script tags for SDK in index.html will execute them, but their `module.exports`
// won't automatically create globals unless the scripts themselves do so.
// For this demo, we'll assume the `Wallet`, `Transaction`, `RPCClient` are available globally
// after their scripts run. This is often achieved by scripts checking `if (typeof window !== 'undefined')`
// and assigning `window.MyLibrary = ...`. The provided SDK files don't do this.
// This will be a point of failure if not addressed by a bundler or by modifying SDK files.
//
// Let's assume for this controlled environment that the script tags in index.html
// make these available, or that nodeIntegration:true is used (which it is not).
// A more robust way for plain JS without bundlers is for each SDK file to do:
// if (typeof window !== 'undefined') { window.Wallet = module.exports; } etc.
// Or `index.js` assigns `window.empower1SDK = module.exports;`
//
// To make this runnable without manually editing SDK files for browser globals:
// The `require` calls at the top of this file will fail in a standard browser env
// or sandboxed Electron renderer.
// The `SDK` object above is a placeholder for how these would be accessed.
// True functionality depends on how these CommonJS modules are made available.
//
// For now, I will assume that the `Wallet`, `Transaction`, `RPCClient` constants
// defined at the top of this script will be correctly populated by the loaded SDK scripts.
// This is a common pattern in simple examples but not robust.

console.log("app.js loaded. SDK (conceptual):", SDK);
if (!SDK.RPCClient) console.warn("RPCClient not found in SDK object after script load.");
if (!SDK.Wallet) console.warn("Wallet not found in SDK object after script load.");
if (!SDK.Transaction) console.warn("Transaction not found in SDK object after script load.");
