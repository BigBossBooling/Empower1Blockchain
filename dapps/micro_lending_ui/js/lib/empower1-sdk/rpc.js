// rpc.js - RPC client for interacting with the Empower1 node

const axios = require('axios');

class RPCClient {
    constructor(nodeUrl) {
        if (!nodeUrl) {
            throw new Error("Node URL must be provided to RPCClient constructor.");
        }
        // Ensure nodeUrl doesn't have a trailing slash for consistency
        this.nodeUrl = nodeUrl.endsWith('/') ? nodeUrl.slice(0, -1) : nodeUrl;
    }

    /**
     * Generic method to send a POST request.
     * @param {string} endpoint - The API endpoint (e.g., "/tx/submit").
     * @param {object} payload - The JSON payload to send.
     * @returns {Promise<object>} The JSON response from the node.
     */
    async _postRequest(endpoint, payload) {
        const url = `${this.nodeUrl}${endpoint}`;
        try {
            const response = await axios.post(url, payload, {
                headers: { 'Content-Type': 'application/json' }
            });
            return response.data; // axios automatically parses JSON response
        } catch (error) {
            if (error.response) {
                // The request was made and the server responded with a status code
                // that falls out of the range of 2xx
                console.error(`Error POST ${url}: Status ${error.response.status}`, error.response.data);
                throw new Error(`Node request failed: ${error.response.status} - ${JSON.stringify(error.response.data) || error.message}`);
            } else if (error.request) {
                // The request was made but no response was received
                console.error(`Error POST ${url}: No response received`, error.request);
                throw new Error(`Node request failed: No response from server at ${url}`);
            } else {
                // Something happened in setting up the request that triggered an Error
                console.error(`Error POST ${url}: Request setup error`, error.message);
                throw new Error(`Node request failed: ${error.message}`);
            }
        }
    }

    /**
     * Generic method to send a GET request.
     * @param {string} endpoint - The API endpoint (e.g., "/info").
     * @param {object} params - Optional query parameters as an object.
     * @returns {Promise<object>} The JSON response from the node.
     */
    async _getRequest(endpoint, params = {}) {
        const url = `${this.nodeUrl}${endpoint}`;
        try {
            const response = await axios.get(url, { params });
            return response.data;
        } catch (error) {
            if (error.response) {
                console.error(`Error GET ${url}: Status ${error.response.status}`, error.response.data);
                throw new Error(`Node request failed: ${error.response.status} - ${JSON.stringify(error.response.data) || error.message}`);
            } else if (error.request) {
                console.error(`Error GET ${url}: No response received`, error.request);
                throw new Error(`Node request failed: No response from server at ${url}`);
            } else {
                console.error(`Error GET ${url}: Request setup error`, error.message);
                throw new Error(`Node request failed: ${error.message}`);
            }
        }
    }

    /**
     * Submits a signed transaction to the node.
     * @param {object} signedTxPayload - The transaction payload, ready for submission.
     *                                   (Assumes byte arrays are already base64 encoded by the caller).
     * @returns {Promise<object>} The node's response.
     */
    async submitTransaction(signedTxPayload) {
        return this._postRequest("/tx/submit", signedTxPayload);
    }

    /**
     * Gets general information from the node.
     * @returns {Promise<object>} The node's information.
     */
    async getNodeInfo() {
        return this._getRequest("/info");
    }

    /**
     * Gets current mempool information/contents from the node.
     * @returns {Promise<object>} The mempool data.
     */
    async getMempoolInfo() {
        // Assuming /mempool endpoint returns JSON. If it's plain text, this needs adjustment.
        return this._getRequest("/mempool");
    }

    /**
     * Calls a smart contract function via the debug endpoint.
     * @param {object} contractCallPayload - Payload for the /debug/call-contract endpoint.
     *        Example: { contract_address: "hex", function_name: "name", arguments_json: "[]", gas_limit: 1000000 }
     * @returns {Promise<object>} The result of the contract call.
     */
    async callContract(contractCallPayload) {
        return this._postRequest("/debug/call-contract", contractCallPayload);
    }

    /**
     * Gets the balance for a given address. (Placeholder - requires node endpoint)
     * @param {string} address - The address to query.
     * @returns {Promise<object>} The balance information.
     */
    async getBalance(address) {
        // This endpoint does not exist yet on the Go node.
        console.warn("getBalance: This endpoint ('/balance') is not yet implemented on the Go node.");
        // return this._getRequest("/balance", { address: address });
        return Promise.resolve({ address: address, balance: "0", note: "Endpoint not implemented" });
    }
}

module.exports = { RPCClient };

// Basic test
if (typeof require !== 'undefined' && require.main === module) {
    (async () => {
        // Replace with your actual node URL if different
        const nodeUrl = "http://localhost:18001"; // Default debug port for first node
        const client = new RPCClient(nodeUrl);

        try {
            console.log(`Attempting to connect to node at: ${nodeUrl}`);

            const info = await client.getNodeInfo();
            console.log("\nNode Info:", info);

            const mempoolInfo = await client.getMempoolInfo();
            console.log("\nMempool Info/Contents:", mempoolInfo);

            // Example for callContract (assumes simple_storage.wasm is deployed and has an 'init' or view function)
            // This requires a deployed contract address.
            // const deployedContractAddress = "your_deployed_simple_storage_contract_address_hex";
            // if (deployedContractAddress) {
            //     const callResult = await client.callContract({
            //         contract_address: deployedContractAddress,
            //         function_name: "get", // Assuming 'get' function
            //         arguments_json: JSON.stringify(["some_key"]), // Arguments as a JSON string array
            //         gas_limit: 1000000
            //     });
            //     console.log("\nCall Contract ('get' some_key) Result:", callResult);
            // }

            const balance = await client.getBalance("some_dummy_address_for_balance_test");
            console.log("\nGet Balance (Placeholder):", balance);

        } catch (error) {
            console.error("\nError during RPC client test:", error.message);
        }
    })();
}
