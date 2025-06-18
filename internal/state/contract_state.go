package state

import (
	"bytes"
	"encoding/hex"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output
	"sync"

	"empower1.com/core/core" // Assuming 'core' is the package alias for empower1.com/core/core
)

// --- Custom Errors for State Manager ---
var (
	ErrStateInit              = errors.New("state manager initialization error")
	ErrInsufficientBalance    = errors.New("insufficient balance") // For UTXO/account balance checks
	ErrUTXONotFound           = errors.New("utxo not found")
	ErrUTXOAlreadySpent       = errors.New("utxo already spent")
	ErrInvalidTransactionType = errors.New("invalid transaction type for state update")
	ErrStateCorruption        = errors.New("blockchain state corrupted")
	ErrWealthLevelNotFound    = errors.New("wealth level not found for address") // EmPower1 specific
	ErrContractStorage        = errors.New("contract storage error")
	ErrContractCode           = errors.New("contract code error")
	ErrInvalidContractAddress = errors.New("invalid contract address")
)

// UTXO represents an unspent transaction output.
// This is a fundamental component for UTXO-based models.
type UTXO struct {
	TxID    []byte // The ID of the transaction that created this output
	Vout    int    // The index of the output in that transaction
	Value   uint64 // The amount of value in this output
	Address []byte // The recipient's public key hash (address)
}

// Account represents the conceptual state associated with an address.
// EmPower1: This is crucial for storing AI-assessed wealth levels and potentially DID linkages.
type Account struct {
	Balance     uint64            // Current total balance (conceptually tracked by summing UTXOs)
	Nonce       uint64            // Transaction nonce (for account-based models or replay protection in multi-sig)
	WealthLevel map[string]string // AI/ML assessed wealth level (e.g., {"category": "affluent", "last_updated": "timestamp"})
	DID         []byte            // V2+: Decentralized Identifier linked to this account
	// V2+: ReputationScore float64 // Derived from on-chain behavior for PoS participation
}

// ContractStorageData represents the persistent storage for a single smart contract.
// Key-value pairs are stored as raw bytes.
type ContractStorageData map[string][]byte

// State manages the global, synchronized state of the EmPower1 Blockchain.
// This includes the UTXO set, conceptual account states, and smart contract storage.
// For V1, this is an in-memory implementation. A production system would use a persistent DB.
type State struct {
	mu           sync.RWMutex                     // Mutex for concurrent access to all state maps
	utxoSet      map[string]*UTXO                 // UTXO set: maps UTXO ID (TxID:Vout) -> UTXO object
	accounts     map[string]*Account              // Account states: maps address (hex string) -> Account object
	contracts    map[string]ContractStorageData   // Contract storage: maps contractAddress (hex) -> ContractStorageData
	contractCode map[string][]byte                // Deployed contract WASM bytecode: maps contractAddress (hex) -> wasmCode
	logger       *log.Logger                      // Dedicated logger for the State instance
}

// NewState creates a new State manager.
// Initializes all in-memory state storage.
func NewState() (*State, error) {
	logger := log.New(os.Stdout, "STATE: ", log.Ldate|log.Ltime|log.Lshortfile)
	state := &State{
		utxoSet:      make(map[string]*UTXO),
		accounts:     make(map[string]*Account),
		contracts:    make(map[string]ContractStorageData),
		contractCode: make(map[string][]byte),
		logger:       logger,
	}
	state.logger.Println("State manager initialized.")
	return state, nil
}

// --- State Update Function (Core State Transition) ---

// UpdateStateFromBlock applies the transactions from a new, validated block to the blockchain state.
// This is the core state transition function, called by the blockchain after a block is added.
// It rigorously processes transactions to maintain the integrity of the global state.
// It directly supports "Systematize for Scalability, Synchronize for Synergy".
func (s *State) UpdateStateFromBlock(block *Block) error {
	s.mu.Lock() // Acquire write lock for state modification
	defer s.mu.Unlock()

	s.logger.Printf("STATE: Updating state from block #%d (%x). Processing %d transactions.", block.Height, block.Hash, len(block.Transactions))

	for i, tx := range block.Transactions {
		txIDHex := hex.EncodeToString(tx.ID)
		s.logger.Debugf("STATE: Processing Tx %d/%d (%s), Type: %s", i+1, len(block.Transactions), txIDHex, tx.TxType.String())

		// Handle specific transaction types
		switch tx.TxType {
		case StandardTx:
			if err := s.processStandardTx(tx, block.Height); err != nil {
				return fmt.Errorf("%w: failed to process standard tx %s in block %d: %v", ErrStateCorruption, txIDHex, block.Height, err)
			}
		case TxContractDeploy:
			if err := s.processContractDeployTx(tx, block.Height); err != nil {
				return fmt.Errorf("%w: failed to process contract deploy tx %s in block %d: %v", ErrStateCorruption, txIDHex, block.Height, err)
			}
		case TxContractCall:
			if err := s.processContractCallTx(tx, block.Height); err != nil {
				return fmt.Errorf("%w: failed to process contract call tx %s in block %d: %v", ErrStateCorruption, txIDHex, block.Height, err)
			}
		case StimulusTx: // EmPower1 Specific: AI/ML-driven redistribution (stimulus)
			if err := s.processStimulusTx(tx, block.Height); err != nil {
				return fmt.Errorf("%w: failed to process stimulus tx %s in block %d: %v", ErrStateCorruption, txIDHex, block.Height, err)
			}
		case TaxTx: // EmPower1 Specific: AI/ML-driven tax collection
			if err := s.processTaxTx(tx, block.Height); err != nil {
				return fmt.Errorf("%w: failed to process tax tx %s in block %d: %v", ErrStateCorruption, txIDHex, block.Height, err)
			}
		default:
			return fmt.Errorf("%w: unknown transaction type %s in block %d for tx %s", ErrInvalidTransactionType, tx.TxType.String(), block.Height, txIDHex)
		}
	}
	s.logger.Printf("STATE: State update from block #%d complete. UTXOs: %d, Accounts: %d, Contracts: %d", block.Height, len(s.utxoSet), len(s.accounts), len(s.contracts))
	return nil
}

// --- Private Helpers for Transaction Processing (by type) ---
// These methods encapsulate the state transition logic for different transaction types.

func (s *State) processStandardTx(tx *core.Transaction, blockHeight int64) error {
	// 1. Mark Inputs as Spent (Remove Spent UTXOs)
	for inputIdx, input := range tx.Inputs {
		utxoKey := fmt.Sprintf("%x:%d", input.TxID, input.Vout)
		if _, exists := s.utxoSet[utxoKey]; !exists {
			// This is a critical error: indicates double-spend or invalid UTXO reference.
			s.logger.Errorf("STATE_ERROR: Block %d, Tx %x: Input %d (%s) UTXO not found. Possible double-spend.",
				blockHeight, tx.ID, inputIdx, utxoKey)
			return fmt.Errorf("%w: input UTXO %s not found for tx %x in block %d", ErrUTXONotFound, utxoKey, tx.ID, blockHeight)
		}
		delete(s.utxoSet, utxoKey) // Mark UTXO as spent
		s.logger.Debugf("STATE: Removed spent UTXO %s for tx %x", utxoKey, tx.ID)
	}

	// 2. Add New UTXOs from Outputs
	return s.addTxOutputsAsUTXOs(tx, blockHeight)
}

func (s *State) processContractDeployTx(tx *core.Transaction, blockHeight int64) error {
	// 1. Mark deployer's inputs as spent (if deployer paid fee with UTXOs)
	// Currently, tx.Inputs is only for value transfer. Deploy fee could come from 'From' in standard way.
	// For now, assuming contract deploy just has `ContractCode` and `Fee`.
	// If it consumes inputs, those would be processed.

	// 2. Store Contract Code
	contractAddress := tx.TargetContractAddress // For deployment, TargetContractAddress is the contract's new address
	if contractAddress == nil || len(contractAddress) == 0 {
		return fmt.Errorf("%w: contract address missing for deployment tx %x", ErrContractCode, tx.ID)
	}
	if tx.ContractCode == nil || len(tx.ContractCode) == 0 {
		return fmt.Errorf("%w: contract code missing for deployment tx %x", ErrContractCode, tx.ID)
	}
	return s.StoreContractCode(contractAddress, tx.ContractCode)
}

func (s *State) processContractCallTx(tx *core.Transaction, blockHeight int64) error {
	// 1. Mark caller's inputs as spent (if caller paid fee with UTXOs or sent value)
	// Similar to StandardTx, process inputs if value/fee is transferred.

	// 2. Execute Contract Logic (Conceptual)
	// This is where the contract's code would be executed within a VM.
	contractAddress := tx.TargetContractAddress
	functionName := tx.FunctionName
	args := tx.Arguments
	
	// Conceptual: load contract code, initialize VM, execute function
	// contractCode, err := s.GetContractCode(contractAddress)
	// if err != nil { return fmt.Errorf("%w: contract %x not deployed: %v", ErrContractStorage, contractAddress, err) }
	// vm := NewContractVM(contractCode, s) // Pass state manager to VM for storage access
	// callResult, err := vm.Execute(functionName, args, tx.From, tx.Amount) // Pass sender, value
	// if err != nil { return fmt.Errorf("%w: contract call failed for %x/%s: %v", ErrContractStorage, contractAddress, functionName, err) }

	// 3. Process Contract Execution Side Effects (Conceptual)
	// This includes:
	// - Updates to contract's own storage (via vm.SetStorage/GetStorage)
	// - New UTXOs generated by contract (e.g., token transfers)
	// - Logs emitted by contract execution
	// - Updates to account nonces if an account-based model for contracts
	
	// For now, assume contract execution might update contract's storage directly via SetContractStorage
	// and add new UTXOs from Outputs (if contract emitted them).
	return s.addTxOutputsAsUTXOs(tx, blockHeight) // Add any outputs from contract call (e.g., token transfers)
}

func (s *State) processStimulusTx(tx *core.Transaction, blockHeight int64) error {
	// EmPower1 Specific: AI/ML-driven redistribution (stimulus payment)
	// Stimulus transactions typically have no inputs (minted implicitly by consensus for rewards).
	// They only have outputs, sending value to targeted less-privileged users.
	if len(tx.Inputs) > 0 {
		return fmt.Errorf("%w: stimulus transaction %s should have no inputs, but found %d", ErrInvalidTransaction, tx.ID, len(tx.Inputs))
	}
	// Add new UTXOs created by this stimulus (these implicitly increase total supply)
	// Validation of *why* this stimulus was generated (e.g., AI/ML logic) would happen in Consensus/Proposer.
	return s.addTxOutputsAsUTXOs(tx, blockHeight)
}

func (s *State) processTaxTx(tx *core.Transaction, blockHeight int64) error {
	// EmPower1 Specific: AI/ML-driven tax collection
	// Tax transactions conceptually collect value from specific users.
	// This would involve consuming UTXOs (inputs) from the taxed party.
	if len(tx.Inputs) == 0 {
		return fmt.Errorf("%w: tax transaction %s should have inputs, but found none", ErrInvalidTransaction, tx.ID)
	}
	// Process inputs (consume UTXOs from taxed party)
	for inputIdx, input := range tx.Inputs {
		utxoKey := fmt.Sprintf("%x:%d", input.TxID, input.Vout)
		if _, exists := s.utxoSet[utxoKey]; !exists {
			s.logger.Errorf("STATE_ERROR: Block %d, Tx %x: Tax input %d (%s) UTXO not found.",
				blockHeight, tx.ID, inputIdx, utxoKey)
			return fmt.Errorf("%w: tax input UTXO %s not found for tx %x in block %d", ErrUTXONotFound, utxoKey, tx.ID, blockHeight)
		}
		delete(s.utxoSet, utxoKey) // Mark UTXO as spent
		s.logger.Debugf("STATE: Removed taxed UTXO %s for tx %x", utxoKey, tx.ID)
	}

	// Process outputs (value goes to a treasury address or burned)
	// A TaxTx might have outputs to a central treasury account.
	return s.addTxOutputsAsUTXOs(tx, blockHeight) // Add outputs to treasury/burn address
}

// addTxOutputsAsUTXOs is a helper to add all outputs of a transaction as new UTXOs.
func (s *State) addTxOutputsAsUTXOs(tx *core.Transaction, blockHeight int64) error {
	for outputIdx, output := range tx.Outputs {
		utxoKey := fmt.Sprintf("%x:%d", tx.ID, outputIdx)
		if _, exists := s.utxoSet[utxoKey]; exists {
			s.logger.Errorf("STATE_ERROR: Block %d, Tx %x: Output %d (%s) already exists. Possible tx ID collision or state corruption.",
				blockHeight, tx.ID, outputIdx, utxoKey)
			return fmt.Errorf("%w: output UTXO %s already exists for tx %x in block %d", ErrUTXOAlreadySpent, utxoKey, tx.ID, blockHeight)
		}
		newUTXO := &UTXO{
			TxID:    tx.ID,
			Vout:    outputIdx,
			Value:   output.Value,
			Address: output.PubKeyHash, // The recipient's address
		}
		s.utxoSet[utxoKey] = newUTXO
		s.logger.Debugf("STATE: Added new UTXO %s for tx %x, value %d to %x", utxoKey, tx.ID, output.Value, output.PubKeyHash)
		
		// Update conceptual account balance based on new UTXO
		s.updateAccountBalance(output.PubKeyHash, output.Value)
		// EmPower1: If this output is for a user, conceptual update of wealth level could be here
		// s.updateAccountWealthLevel(output.PubKeyHash, "category_from_AI") // Conceptual AI input
	}
	return nil
}


// --- Account Management Helpers ---

// updateAccountBalance conceptually updates an account's balance based on UTXO changes.
// In a pure UTXO model, balances are always calculated by summing UTXOs.
// This is a hybrid/conceptual step for convenience for EmPower1's account model to store metadata.
func (s *State) updateAccountBalance(address []byte, valueChange uint64) {
	addrHex := hex.EncodeToString(address)
	account, exists := s.accounts[addrHex]
	if !exists {
		account = &Account{
			Balance:    0,
			Nonce:      0,
			WealthLevel: make(map[string]string), // Initialize empty
		}
		s.accounts[addrHex] = account
	}
	// This simple sum is conceptual for a hybrid model. For deductions, you'd need separate logic
	// to ensure integrity (e.g., from consumed inputs). This method only adds.
	account.Balance += valueChange 
}

// updateAccountNonce is a conceptual function for account-based models.
// Used for replay protection/ordering of transactions from a specific account.
func (s *State) updateAccountNonce(address []byte, newNonceValue uint64) {
	addrHex := hex.EncodeToString(address)
	account, exists := s.accounts[addrHex]
	if !exists {
		account = &Account{
			Balance:     0,
			Nonce:       0,
			WealthLevel: make(map[string]string),
		}
		s.accounts[addrHex] = account
	}
	account.Nonce = newNonceValue
	s.logger.Debugf("STATE: Updated nonce for %x to %d", address, newNonceValue)
}

// --- Public State Query Functions ---

// GetBalance returns the current confirmed balance for a given address.
// In a pure UTXO model, this sums up all UTXOs belonging to the address.
func (s *State) GetBalance(address []byte) (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrHex := hex.EncodeToString(address)
	totalBalance := uint64(0)
	found := false
	
	// Sum all UTXOs belonging to this address for the balance.
	for _, utxo := range s.utxoSet {
		if bytes.Equal(utxo.Address, address) {
			totalBalance += utxo.Value
			found = true
		}
	}
	
	if !found { // Return error if address has no UTXOs at all.
		return 0, fmt.Errorf("%w: address %s not found in UTXO set or has zero balance", ErrInsufficientBalance, addrHex)
	}
	return totalBalance, nil
}

// FindSpendableOutputs finds and returns a list of UTXOs that can be spent by an address to cover an amount.
// This is crucial for creating new transactions (e.g., standard transfers, tax payments).
// It returns the selected UTXOs, their total value, and an error if insufficient.
func (s *State) FindSpendableOutputs(address []byte, amount uint64) ([]UTXO, uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	foundOutputs := []UTXO{}
	accumulated := uint64(0)

	// Iterate through UTXOs and select those belonging to the address until amount is met.
	// A real implementation would optimize selection (e.g., choose fewest UTXOs, oldest, etc.).
	for _, utxo := range s.utxoSet {
		if bytes.Equal(utxo.Address, address) {
			foundOutputs = append(foundOutputs, *utxo) // Append a copy of the UTXO
			accumulated += utxo.Value
			if accumulated >= amount {
				break // Found enough
			}
		}
	}

	if accumulated < amount {
		return nil, 0, fmt.Errorf("%w: address %x has %d, but needs %d", ErrInsufficientBalance, address, accumulated, amount)
	}
	return foundOutputs, accumulated, nil
}

// GetWealthLevel retrieves the conceptual AI/ML assessed wealth level for an address.
// EmPower1 specific, leveraging AI/ML integration for transparency and targeted redistribution.
func (s *State) GetWealthLevel(address []byte) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrHex := hex.EncodeToString(address)
	account, exists := s.accounts[addrHex]
	if !exists || account.WealthLevel == nil || len(account.WealthLevel) == 0 {
		return nil, fmt.Errorf("%w: wealth level not found for address %s", ErrWealthLevelNotFound, addrHex)
	}
	// Return a copy to prevent external modification of the stored map.
	wealthCopy := make(map[string]string)
	for k, v := range account.WealthLevel {
		wealthCopy[k] = v
	}
	return wealthCopy, nil
}

// UpdateWealthLevel is a conceptual function that would be called by the AI/ML module
// or consensus logic to update an account's wealth level in state.
// This simulates the direct integration of AI/ML insights into the blockchain state.
func (s *State) UpdateWealthLevel(address []byte, level map[string]string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	addrHex := hex.EncodeToString(address)
	account, exists := s.accounts[addrHex]
	if !exists {
		// If account doesn't exist, create a new one with initial state.
		account = &Account{
			Balance:     0, // Will be updated by UTXO processing
			Nonce:       0,
			WealthLevel: make(map[string]string),
		}
		s.accounts[addrHex] = account
	}
	// Deep copy the incoming level map to prevent external modification.
	account.WealthLevel = make(map[string]string)
	for k, v := range level {
		account.WealthLevel[k] = v
	}
	s.logger.Printf("STATE: Updated wealth level for %s: %v", addrHex, level)
	return nil
}


// --- Contract Code & Data Storage ---

// StoreContractCode stores the WASM bytecode for a deployed contract.
// contractAddress is the unique identifier for the contract.
func (s *State) StoreContractCode(contractAddress []byte, code []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	addressHex := hex.EncodeToString(contractAddress)
	if _, exists := s.contractCode[addressHex]; exists {
		return fmt.Errorf("%w: contract code for address %s already exists", ErrContractCode, addressHex)
	}
	// Store a copy to prevent external modification of the stored slice.
	codeCopy := make([]byte, len(code))
	copy(codeCopy, code)
	s.contractCode[addressHex] = codeCopy
	s.logger.Printf("STATE: Stored code for contract %s (%d bytes)", addressHex, len(codeCopy))
	return nil
}

// GetContractCode retrieves the WASM bytecode for a given contract address.
func (s *State) GetContractCode(contractAddress []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addressHex := hex.EncodeToString(contractAddress)
	code, exists := s.contractCode[addressHex]
	if !exists {
		return nil, fmt.Errorf("%w: no contract code found for address %s", ErrContractCode, addressHex)
	}
	// Return a copy to prevent external modification.
	codeCopy := make([]byte, len(code))
	copy(codeCopy, code)
	return codeCopy, nil
}


// GetContractStorage retrieves a value from a contract's storage.
// contractAddress: The address of the contract.
// key: The key (as []byte) within that contract's storage.
func (s *State) GetContractStorage(contractAddress []byte, key []byte) ([]byte, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addressHex := hex.EncodeToString(contractAddress)
	keyHex := hex.EncodeToString(key)

	contractData, ok := s.contracts[addressHex]
	if !ok {
		// If contract has no storage yet, key cannot exist. Return nil value, nil error.
		return nil, nil 
	}

	value, ok := contractData[keyHex]
	if !ok {
		// Key not found within this contract's storage. Return nil value, nil error.
		return nil, nil 
	}

	// Return a copy to prevent external modification.
	valueCopy := make([]byte, len(value))
	copy(valueCopy, value)
	s.logger.Debugf("STATE: GetContractStorage: Contract %s, Key %s, Value %x (len %d)", addressHex, keyHex, valueCopy, len(valueCopy))
	return valueCopy, nil
}

// SetContractStorage sets a value in a contract's storage.
// contractAddress: The address of the contract.
// key: The key (as []byte) within that contract's storage.
// value: The value to set. If value is nil, the key is conceptually deleted.
func (s *State) SetContractStorage(contractAddress []byte, key []byte, value []byte) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	addressHex := hex.EncodeToString(contractAddress)
	keyHex := hex.EncodeToString(key)

	contractData, ok := s.contracts[addressHex]
	if !ok {
		// If contract storage doesn't exist, create it.
		contractData = make(ContractStorageData)
		s.contracts[addressHex] = contractData
		s.logger.Debugf("STATE: Created new storage for contract %s", addressHex)
	}

	if value == nil { // Standard practice: setting a key to nil deletes it
		delete(contractData, keyHex)
		s.logger.Printf("STATE: SetContractStorage: Deleted key %s for contract %s", keyHex, addressHex)
	} else {
		// Store a copy of the value.
		valueCopy := make([]byte, len(value))
		copy(valueCopy, value)
		contractData[keyHex] = valueCopy
		s.logger.Printf("STATE: SetContractStorage: Contract %s, Key %s, Value %x (len %d)", addressHex, keyHex, valueCopy, len(valueCopy))
	}
	return nil
}