package core

import (
	"bytes"
	"crypto/sha256"
	"errors" // Explicitly import errors
	"fmt"
	"log"    // For structured logging
	"os"     // For log output
	"sync"
	// "encoding/hex" // For debugging addresses
)

// --- Custom Errors for State Manager ---
var (
	ErrStateInit              = errors.New("state manager initialization error")
	ErrInsufficientBalance    = errors.New("insufficient balance")
	ErrUTXONotFound           = errors.New("utxo not found")
	ErrUTXOAlreadySpent       = errors.New("utxo already spent")
	ErrInvalidTransactionType = errors.New("invalid transaction type for state update")
	ErrStateCorruption        = errors.New("blockchain state corrupted")
	ErrWealthLevelNotFound    = errors.New("wealth level not found for address") // EmPower1 specific
)

// UTXO represents an unspent transaction output.
// This is a fundamental component for UTXO-based blockchains.
type UTXO struct {
	TxID    []byte // The ID of the transaction that created this output
	Vout    int    // The index of the output in that transaction
	Value   uint64 // The amount of value in this output
	Address []byte // The recipient's public key hash (address)
}

// Account represents the state associated with an address.
// EmPower1: This is crucial for storing AI-assessed wealth levels.
type Account struct {
	Balance    uint64            // Current total balance from UTXOs
	Nonce      uint64            // Transaction nonce (for account-based models or replay protection)
	WealthLevel map[string]string // AI/ML assessed wealth level (e.g., {"category": "affluent", "last_updated": "timestamp"})
	// V2+: ReputationScore float64 // Derived from on-chain behavior
	// V2+: DID            []byte // Decentralized Identifier
}

// State manages the global, synchronized state of the EmPower1 Blockchain.
// For V1, this is an in-memory UTXO set manager with conceptual Account states.
// In a production system, this would be backed by persistent storage (e.g., LevelDB, RocksDB).
type State struct {
	mu           sync.RWMutex                     // Mutex for concurrent access
	utxoSet      map[string]*UTXO                 // UTXO set: maps UTXO ID (TxID:Vout) to UTXO object
	accounts     map[string]*Account              // Account states: maps address (hex) to Account object
	logger       *log.Logger                      // Dedicated logger for the State instance
}

// NewState creates a new State manager.
// Initializes the in-memory state storage.
func NewState() (*State, error) {
	logger := log.New(os.Stdout, "STATE: ", log.Ldate|log.Ltime|log.Lshortfile)
	state := &State{
		utxoSet:  make(map[string]*UTXO),
		accounts: make(map[string]*Account), // Initialize account map
		logger:   logger,
	}
	state.logger.Println("State manager initialized.")
	return state, nil
}

// UpdateStateFromBlock updates the blockchain state based on the transactions within a new, valid block.
// This is the core state transition function, called by the blockchain after a block is added.
// It directly supports "Systematize for Scalability, Synchronize for Synergy".
func (s *State) UpdateStateFromBlock(block *Block) error {
	s.mu.Lock() // Acquire write lock for state modification
	defer s.mu.Unlock()

	s.logger.Printf("STATE: Updating state from block #%d (%x)", block.Height, block.Hash)

	// Process each transaction in the block
	for i, tx := range block.Transactions {
		txIDHex := hex.EncodeToString(tx.ID)
		
		// 1. Process Inputs (Remove Spent UTXOs)
		// For standard transactions, mark inputs as spent.
		if tx.TxType == StandardTx || tx.TxType == TxContractCall || tx.TxType == TxContractDeploy {
			for inputIdx, input := range tx.Inputs {
				utxoKey := fmt.Sprintf("%x:%d", input.TxID, input.Vout)
				if _, exists := s.utxoSet[utxoKey]; !exists {
					// This indicates a double-spend or invalid UTXO reference, a critical error.
					s.logger.Errorf("STATE_ERROR: Block %d, Tx %s: Input %d (%s) UTXO not found in state. Possible double-spend or invalid block.",
						block.Height, txIDHex, inputIdx, utxoKey)
					return fmt.Errorf("%w: input UTXO %s not found for tx %s in block %d", ErrUTXONotFound, utxoKey, txIDHex, block.Height)
				}
				// Conceptually: Verify input signature and pubkey against UTXO.Address
				// This should ideally happen in tx.VerifySignature or a pre-validation, but double-check here.
				
				delete(s.utxoSet, utxoKey) // Mark UTXO as spent
				s.logger.Debugf("STATE: Removed spent UTXO %s for tx %s", utxoKey, txIDHex)
			}
		}

		// 2. Process Outputs (Add New UTXOs)
		for outputIdx, output := range tx.Outputs {
			utxoKey := fmt.Sprintf("%x:%d", tx.ID, outputIdx)
			if _, exists := s.utxoSet[utxoKey]; exists {
				// This indicates a duplicate output being added, a critical error.
				s.logger.Errorf("STATE_ERROR: Block %d, Tx %s: Output %d (%s) already exists in state. Possible tx ID collision or state corruption.",
					block.Height, txIDHex, outputIdx, utxoKey)
				return fmt.Errorf("%w: output UTXO %s already exists for tx %s in block %d", ErrUTXOAlreadySpent, utxoKey, txIDHex, block.Height)
			}
			newUTXO := &UTXO{
				TxID:    tx.ID,
				Vout:    outputIdx,
				Value:   output.Value,
				Address: output.PubKeyHash,
			}
			s.utxoSet[utxoKey] = newUTXO
			s.logger.Debugf("STATE: Added new UTXO %s for tx %s, value %d to %x", utxoKey, txIDHex, output.Value, output.PubKeyHash)
			
			// Update conceptual account balance based on new UTXO
			s.updateAccountBalance(output.PubKeyHash, output.Value)
		}
		
		// 3. EmPower1 Specific: AI/ML Driven State Updates (Wealth Gap Redistribution)
		// This is where the core mission of EmPower1 manifests in state.
		// It assumes AI analysis has occurred and its directives are part of the block's AIAuditLog
		// or directly embedded into stimulus/tax transactions.
		if tx.TxType == StimulusTx || tx.TxType == TaxTx {
			s.logger.Printf("STATE: Processing EmPower1 specific transaction Type: %s (ID: %s)", tx.TxType, txIDHex)
			// Conceptual: Here, the state would update specific fields in accounts based on AI/ML analysis
			// For example, update `Account.WealthLevel` for affected users based on the AI's latest assessment.
			// This would involve looking up user addresses in tx.Inputs/Outputs and then updating their Account structs.
			// This needs a robust connection to the AI/ML module's output for that block/tx.
			// Example: if tx.Metadata contains {"ai_assessed_wealth_category": "affluent", "ai_trigger": "tax"}
			// s.updateAccountWealthLevel(tx.From, tx.Metadata["ai_assessed_wealth_category"])
			// s.logger.Printf("STATE: AI/ML driven state update for transaction %s. Type: %s", txIDHex, tx.TxType)
		}

		// 4. Update Account Nonce (for account-based transactions, conceptual for hybrid)
		// If using a nonce for replay protection or ordering (account-based models)
		// s.updateAccountNonce(tx.From, newNonceValue)

	}
	s.logger.Printf("STATE: State update from block #%d complete. Current UTXOs: %d", block.Height, len(s.utxoSet))
	return nil
}

// updateAccountBalance conceptually updates an account's balance based on UTXO changes.
// In a pure UTXO model, balances are always calculated by summing UTXOs.
// This is a hybrid/conceptual step for convenience for EmPower1's account model.
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
	// This simple sum is conceptual. Actual UTXO sum is derived from walking the UTXOSet.
	// For account-based model, this would be direct addition/subtraction.
	account.Balance += valueChange 
	// For deductions, you'd need to subtract. This method is oversimplified.
	// In a true UTXO system, balance is recalculated by summing UTXOs belonging to the address.
}

// GetBalance returns the current confirmed balance for a given address.
// In a pure UTXO model, this sums up all UTXOs belonging to the address.
func (s *State) GetBalance(address []byte) (uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrHex := hex.EncodeToString(address)
	totalBalance := uint64(0)
	found := false
	
	// Sum all UTXOs belonging to this address
	for _, utxo := range s.utxoSet {
		if bytes.Equal(utxo.Address, address) {
			totalBalance += utxo.Value
			found = true
		}
	}
	
	if !found {
		return 0, ErrInsufficientBalance // Or a more specific "address not found" error
	}
	return totalBalance, nil
}

// FindSpendableOutputs finds and returns a list of UTXOs that can be spent by an address to cover an amount.
// This is crucial for creating new transactions.
func (s *State) FindSpendableOutputs(address []byte, amount uint64) ([]UTXO, uint64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	foundOutputs := []UTXO{}
	accumulated := uint64(0)

	// Iterate through UTXOs and select those belonging to the address until amount is met.
	for _, utxo := range s.utxoSet {
		if bytes.Equal(utxo.Address, address) {
			foundOutputs = append(foundOutputs, *utxo) // Append a copy of the UTXO
			accumulated += utxo.Value
			if accumulated >= amount {
				break
			}
		}
	}

	if accumulated < amount {
		return nil, 0, ErrInsufficientBalance
	}
	return foundOutputs, accumulated, nil
}

// GetWealthLevel retrieves the conceptual AI/ML assessed wealth level for an address.
// EmPower1 specific, leveraging AI/ML integration.
func (s *State) GetWealthLevel(address []byte) (map[string]string, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	addrHex := hex.EncodeToString(address)
	account, exists := s.accounts[addrHex]
	if !exists || account.WealthLevel == nil || len(account.WealthLevel) == 0 {
		return nil, ErrWealthLevelNotFound
	}
	// Return a copy to prevent external modification
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
		account = &Account{
			Balance:     0,
			Nonce:       0,
			WealthLevel: make(map[string]string),
		}
		s.accounts[addrHex] = account
	}
	// Deep copy the map
	account.WealthLevel = make(map[string]string)
	for k, v := range level {
		account.WealthLevel[k] = v
	}
	s.logger.Printf("STATE: Updated wealth level for %x: %v", address, level)
	return nil
}