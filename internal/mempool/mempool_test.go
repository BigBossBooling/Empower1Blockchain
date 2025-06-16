package mempool

import (
	"empower1.com/core/internal/core"
	"empower1.com/core/internal/crypto"
	"testing"
	"time"
	"bytes"
	"encoding/gob" // Added import for TestMempoolPruneByBlockchain
	"encoding/hex"
)

// Helper function to create a new signed transaction for testing
func newTestSignedTransaction(t *testing.T, amount uint64) *core.Transaction {
	senderPrivKey, _ := crypto.GenerateECDSAKeyPair()
	receiverPrivKey, _ := crypto.GenerateECDSAKeyPair()
	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKey := &receiverPrivKey.PublicKey

	tx, err := core.NewTransaction(senderPubKey, receiverPubKey, amount, 1)
	if err != nil {
		t.Fatalf("Failed to create new transaction: %v", err)
	}
	err = tx.Sign(senderPrivKey)
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}
	return tx
}

func TestNewMempool(t *testing.T) {
	mp := NewMempool(100)
	if mp == nil {
		t.Fatal("NewMempool() returned nil")
	}
	if mp.maxTransactions != 100 {
		t.Errorf("mp.maxTransactions = %d; want %d", mp.maxTransactions, 100)
	}
	if len(mp.pending) != 0 {
		t.Errorf("New mempool is not empty")
	}

	mpDefault := NewMempool(0) // Test default max size
	if mpDefault.maxTransactions != defaultMaxPendingTransactions {
		t.Errorf("Default max transactions not set correctly")
	}
}

func TestMempoolAddTransaction(t *testing.T) {
	mp := NewMempool(10)
	tx1 := newTestSignedTransaction(t, 100)

	// Add valid transaction
	err := mp.AddTransaction(tx1)
	if err != nil {
		t.Errorf("AddTransaction(valid_tx) error = %v; want nil", err)
	}
	if mp.Count() != 1 {
		t.Errorf("Mempool count = %d; want 1 after adding one tx", mp.Count())
	}

	// Add duplicate transaction
	err = mp.AddTransaction(tx1)
	if err == nil {
		t.Errorf("AddTransaction(duplicate_tx) error = nil; want error")
	}
	if mp.Count() != 1 { // Count should remain 1
		t.Errorf("Mempool count = %d; want 1 after adding duplicate", mp.Count())
	}

	// Add nil transaction
	err = mp.AddTransaction(nil)
	if err == nil {
		t.Errorf("AddTransaction(nil) error = nil; want error")
	}

	// Test adding transaction with invalid signature
	txInvalidSig := newTestSignedTransaction(t, 200)
	txInvalidSig.Signature = []byte("invalid_signature_bytes") // Tamper signature
	err = mp.AddTransaction(txInvalidSig)
	if err == nil {
		t.Errorf("AddTransaction(invalid_sig_tx) error = nil; want error")
	}
	// Verify it contains "invalid signature"
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("invalid signature")) {
		t.Errorf("Expected error containing 'invalid signature', got: %v", err)
	}


	// Fill mempool to max capacity
	for i := 0; i < 9; i++ { // Already 1 tx in mempool
		tx := newTestSignedTransaction(t, uint64(i+1))
		// ensure unique IDs if tx creation is too fast
		time.Sleep(time.Millisecond)
		err := mp.AddTransaction(tx)
		if err != nil {
			t.Fatalf("Failed to add transaction %d to fill mempool: %v", i, err)
		}
	}
	if mp.Count() != 10 {
		t.Errorf("Mempool count = %d; want 10 after filling", mp.Count())
	}

	// Try to add one more transaction (should fail as mempool is full)
	txOverLimit := newTestSignedTransaction(t, 500)
	err = mp.AddTransaction(txOverLimit)
	if err == nil {
		t.Errorf("AddTransaction(over_limit_tx) error = nil; want error (mempool full)")
	}
	if err != nil && !bytes.Contains([]byte(err.Error()), []byte("mempool is full")) {
		t.Errorf("Expected error containing 'mempool is full', got: %v", err)
	}
}

func TestMempoolGetTransaction(t *testing.T) {
	mp := NewMempool(10)
	tx1 := newTestSignedTransaction(t, 100)
	mp.AddTransaction(tx1)

	// Get existing transaction
	retrievedTx, exists := mp.GetTransaction(hex.EncodeToString(tx1.ID))
	if !exists {
		t.Errorf("GetTransaction(existing_tx_id) exists = false; want true")
	}
	if !bytes.Equal(retrievedTx.ID, tx1.ID) {
		t.Errorf("Retrieved transaction ID does not match")
	}

	// Get non-existing transaction
	_, exists = mp.GetTransaction(hex.EncodeToString([]byte("non_existing_id")))
	if exists {
		t.Errorf("GetTransaction(non_existing_tx_id) exists = true; want false")
	}
}

func TestMempoolGetPendingTransactions(t *testing.T) {
	mp := NewMempool(10)
	txs := make([]*core.Transaction, 5)
	for i := 0; i < 5; i++ {
		txs[i] = newTestSignedTransaction(t, uint64(100+i))
		mp.AddTransaction(txs[i])
		time.Sleep(time.Millisecond) // Ensure unique timestamps if tx creation is very fast
	}

	// Get all 5 transactions
	pending := mp.GetPendingTransactions(5)
	if len(pending) != 5 {
		t.Errorf("len(GetPendingTransactions(5)) = %d; want 5", len(pending))
	}

	// Get fewer transactions (e.g., 3)
	pending = mp.GetPendingTransactions(3)
	if len(pending) != 3 {
		t.Errorf("len(GetPendingTransactions(3)) = %d; want 3", len(pending))
	}

	// Get more than available (should return all available)
	pending = mp.GetPendingTransactions(10)
	if len(pending) != 5 {
		t.Errorf("len(GetPendingTransactions(10)) = %d; want 5 (all available)", len(pending))
	}

	// Get with maxCount 0 or negative (should return all)
	// Current implementation might take maxCount as literal. Test based on current behavior.
	// The GetPendingTransactions function caps maxCount if it's > len(mp.pending)
	// or if it's <=0, it sets it to len(mp.pending).
	// So asking for 0 should give all, asking for negative should give all.
	pending = mp.GetPendingTransactions(0)
	if len(pending) != 5 {
		t.Errorf("len(GetPendingTransactions(0)) = %d; want 5", len(pending))
	}
	pending = mp.GetPendingTransactions(-1)
	if len(pending) != 5 {
		t.Errorf("len(GetPendingTransactions(-1)) = %d; want 5", len(pending))
	}


	// Check if returned transactions are correct (by ID)
	pendingAll := mp.GetPendingTransactions(5)
	foundCount := 0
	for _, originalTx := range txs {
		found := false
		for _, pTx := range pendingAll {
			if bytes.Equal(originalTx.ID, pTx.ID) {
				found = true
				break
			}
		}
		if found {
			foundCount++
		}
	}
	if foundCount != 5 {
		t.Errorf("Not all original transactions were found in pending list")
	}
}

func TestMempoolRemoveTransactions(t *testing.T) {
	mp := NewMempool(10)
	txsToKeep := make([]*core.Transaction, 2)
	txsToRemove := make([]*core.Transaction, 3)

	for i := 0; i < 2; i++ {
		txsToKeep[i] = newTestSignedTransaction(t, uint64(10+i))
		mp.AddTransaction(txsToKeep[i])
		time.Sleep(time.Millisecond)
	}
	for i := 0; i < 3; i++ {
		txsToRemove[i] = newTestSignedTransaction(t, uint64(20+i))
		mp.AddTransaction(txsToRemove[i])
		time.Sleep(time.Millisecond)
	}

	if mp.Count() != 5 {
		t.Fatalf("Mempool count before removal = %d; want 5", mp.Count())
	}

	mp.RemoveTransactions(txsToRemove)
	if mp.Count() != 2 {
		t.Errorf("Mempool count after removal = %d; want 2", mp.Count())
	}

	// Check that the correct transactions were removed
	for _, removedTx := range txsToRemove {
		_, exists := mp.GetTransaction(hex.EncodeToString(removedTx.ID))
		if exists {
			t.Errorf("Transaction %s was not removed", hex.EncodeToString(removedTx.ID))
		}
	}
	for _, keptTx := range txsToKeep {
		_, exists := mp.GetTransaction(hex.EncodeToString(keptTx.ID))
		if !exists {
			t.Errorf("Transaction %s was removed but should have been kept", hex.EncodeToString(keptTx.ID))
		}
	}

	// Test removing non-existent transactions (should not error, count should not change)
	nonExistentTx := newTestSignedTransaction(t, 999)
	mp.RemoveTransactions([]*core.Transaction{nonExistentTx})
	if mp.Count() != 2 {
		t.Errorf("Mempool count changed after trying to remove non-existent tx; got %d, want 2", mp.Count())
	}
}

// TestPruneByBlockchain is conceptual as it depends on block data format.
// For now, it tests if the mempool attempts to decode block data.
// A more thorough test would mock blocks with known transactions.
func TestMempoolPruneByBlockchain(t *testing.T) {
	mp := NewMempool(10)
	txInBlock := newTestSignedTransaction(t, 100)
	txNotInBlock := newTestSignedTransaction(t, 200)

	mp.AddTransaction(txInBlock)
	mp.AddTransaction(txNotInBlock)

	if mp.Count() != 2 {
		t.Fatalf("Mempool count before prune = %d; want 2", mp.Count())
	}

	// Create a dummy block that would contain txInBlock
	// The current PruneByBlockchain decodes []*core.Transaction from block.Data using gob
	var blockDataBytes bytes.Buffer
	encoder := gob.NewEncoder(&blockDataBytes)
	err := encoder.Encode([]*core.Transaction{txInBlock})
	if err != nil {
		t.Fatalf("Failed to gob encode transactions for block data: %v", err)
	}

	block := &core.Block{Data: blockDataBytes.Bytes()} // Only Data field is relevant for this test

	mp.PruneByBlockchain([]*core.Block{block})

	if mp.Count() != 1 {
		t.Errorf("Mempool count after prune = %d; want 1", mp.Count())
	}

	_, exists := mp.GetTransaction(hex.EncodeToString(txInBlock.ID))
	if exists {
		t.Errorf("Transaction %s (in block) was not pruned from mempool", hex.EncodeToString(txInBlock.ID))
	}
	_, exists = mp.GetTransaction(hex.EncodeToString(txNotInBlock.ID))
	if !exists {
		t.Errorf("Transaction %s (not in block) was pruned from mempool", hex.EncodeToString(txNotInBlock.ID))
	}
}
