package mempool

import (

	// For TestMempoolPruneByBlockchain
	"encoding/hex"
	"errors"
	"strings"
	"testing"
	"time"

	"empower1.com/core/core"   // Assuming this path to core package
	"empower1.com/core/crypto" // Assuming this path to crypto package for key generation
)

// Helper function to create a new signed transaction for testing
// This helper now supports specific TxType for EmPower1's unique prioritization.
func newTestSignedTransaction(t *testing.T, amount uint64, txType core.TxType) *core.Transaction {
	// Generate ECDSA key pair using the crypto package
	senderPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate sender key pair: %v", err)
	}
	receiverPrivKey, err := crypto.GenerateECDSAKeyPair()
	if err != nil {
		t.Fatalf("Failed to generate receiver key pair: %v", err)
	}

	senderPubKey := &senderPrivKey.PublicKey
	receiverPubKeyBytes, err := crypto.SerializePublicKeyToBytes(&receiverPrivKey.PublicKey)
	if err != nil {
		t.Fatalf("Failed to serialize receiver public key: %v", err)
	}

	var tx *core.Transaction
	// Use appropriate constructor based on TxType
	switch txType {
	case core.TxStandard:
		tx, err = core.NewStandardTransaction(senderPubKey, receiverPubKeyBytes, amount, 10) // Fixed fee for simplicity
	case core.StimulusTx: // Stimulus txs typically have no sender input, just outputs
		tx, err = core.NewStandardTransaction(senderPubKey, receiverPubKeyBytes, amount, 0) // Stimulus might have zero fee
		tx.TxType = core.StimulusTx                                                         // Explicitly set type if constructor doesn't
	case core.TaxTx: // Tax txs also special
		tx, err = core.NewStandardTransaction(senderPubKey, receiverPubKeyBytes, amount, 0) // Tax might have zero fee
		tx.TxType = core.TaxTx                                                              // Explicitly set type
	default:
		// Fallback for other types if NewTransaction doesn't have a specific constructor yet.
		tx, err = core.NewStandardTransaction(senderPubKey, receiverPubKeyBytes, amount, 5)
		if err != nil {
			t.Fatalf("Failed to create generic transaction: %v", err)
		}
		tx.TxType = txType // Ensure type is set for generic case too.
	}

	if err != nil {
		t.Fatalf("Failed to create new transaction of type %s: %v", txType, err)
	}

	// Sign the transaction
	err = tx.Sign(senderPrivKey)
	if err != nil {
		t.Fatalf("Failed to sign transaction: %v", err)
	}
	return tx
}

// --- Test Suite for Mempool Functionality ---

func TestNewMempool(t *testing.T) {
	mp, err := NewMempool(100)
	if err != nil {
		t.Fatalf("NewMempool(100) returned error: %v", err)
	}
	if mp == nil {
		t.Fatal("NewMempool() returned nil")
	}
	if mp.capacity != 100 {
		t.Errorf("mp.capacity = %d; want %d", mp.capacity, 100)
	}
	if len(mp.transactions) != 0 {
		t.Errorf("New mempool.transactions map is not empty")
	}
	if len(mp.priorityQueue) != 0 {
		t.Errorf("New mempool.priorityQueue is not empty")
	}

	// Test default max size (capacity <= 0)
	mpDefault, err := NewMempool(0)
	if err != nil || mpDefault == nil {
		t.Fatalf("NewMempool(0) returned error or nil: %v", err)
	}
	if mpDefault.capacity != defaultMaxPendingTransactions {
		t.Errorf("Default max transactions not set correctly: got %d, want %d", mpDefault.capacity, defaultMaxPendingTransactions)
	}

	// Test negative capacity (should error)
	_, err = NewMempool(-5)
	if err == nil || !errors.Is(err, ErrMempoolInit) || !strings.Contains(err.Error(), "capacity must be positive") {
		t.Errorf("Expected ErrMempoolInit for negative capacity, got: %v", err)
	}
}

func TestMempoolAddTransaction(t *testing.T) {
	mp, _ := NewMempool(10) // Capacity 10
	tx1 := newTestSignedTransaction(t, 100, core.TxStandard)

	// Test 1: Add valid transaction
	err := mp.AddTransaction(tx1)
	if err != nil {
		t.Errorf("AddTransaction(valid_tx) error = %v; want nil", err)
	}
	if mp.Size() != 1 { // Use mp.Size()
		t.Errorf("Mempool count = %d; want 1 after adding one tx", mp.Size())
	}
	if !mp.Contains(tx1.ID) {
		t.Errorf("Mempool should contain tx1")
	}

	// Test 2: Add duplicate transaction
	err = mp.AddTransaction(tx1)
	if err == nil || !errors.Is(err, ErrTransactionExists) {
		t.Errorf("Expected ErrTransactionExists for duplicate tx, got %v", err)
	}
	if mp.Size() != 1 { // Count should remain 1
		t.Errorf("Mempool count = %d; want 1 after adding duplicate", mp.Size())
	}

	// Test 3: Add nil transaction
	err = mp.AddTransaction(nil)
	if err == nil || !errors.Is(err, ErrInvalidTransaction) {
		t.Errorf("Expected ErrInvalidTransaction for nil tx, got %v", err)
	}

	// Test 4: Add transaction with invalid signature (simulated)
	txInvalidSig := newTestSignedTransaction(t, 200, core.TxStandard)
	originalSig := txInvalidSig.Signature
	txInvalidSig.Signature = []byte("invalid_signature_bytes") // Tamper signature for test

	err = mp.AddTransaction(txInvalidSig)
	if err == nil || !errors.Is(err, ErrInvalidTransaction) || !strings.Contains(err.Error(), "signature invalid") {
		t.Errorf("Expected ErrInvalidTransaction for invalid signature, got %v", err)
	}
	txInvalidSig.Signature = originalSig // Restore for future tests if needed

	// Test 5: Add transaction with ID/hash mismatch
	txHashMismatch := newTestSignedTransaction(t, 300, core.TxStandard)
	txHashMismatch.ID = []byte("fake_id_that_does_not_match_content") // Tamper ID
	err = mp.AddTransaction(txHashMismatch)
	if err == nil || !errors.Is(err, ErrInvalidTransaction) || !strings.Contains(err.Error(), "ID mismatch") {
		t.Errorf("Expected ErrInvalidTransaction for ID mismatch, got %v", err)
	}
	// Restore ID for future tests if needed
	correctHash, _ := txHashMismatch.Hash()
	txHashMismatch.ID = correctHash

	// Test 6: Fill mempool to max capacity and try to add more
	for i := 0; i < 9; i++ { // Already 1 tx in mempool (tx1)
		tx := newTestSignedTransaction(t, uint64(1000+i), core.TxStandard)
		err := mp.AddTransaction(tx)
		if err != nil {
			t.Fatalf("Failed to add transaction %d to fill mempool: %v", i, err)
		}
	}
	if mp.Size() != 10 {
		t.Errorf("Mempool count = %d; want 10 after filling", mp.Size())
	}

	txOverLimit := newTestSignedTransaction(t, 5000, core.TxStandard)
	err = mp.AddTransaction(txOverLimit)
	if err == nil || !errors.Is(err, ErrMempoolCapacityFull) {
		t.Errorf("Expected ErrMempoolCapacityFull when full, got %v", err)
	}
	if mp.Size() != 10 { // Ensure count remains max
		t.Errorf("Mempool count changed after full add, should remain 10, got %d", mp.Size())
	}
}

func TestMempoolGetPendingTransactions(t *testing.T) {
	mp, _ := NewMempool(10)
	txsStandard := make([]*core.Transaction, 3)
	txsStimulus := make([]*core.Transaction, 2)
	txsTax := make([]*core.Transaction, 1)

	// Add transactions with varied types and fees to test priority queue
	// Standard: fee 10
	// Stimulus: fee 0 (but high priority)
	// Tax: fee 0 (but lower priority than stimulus)
	txsStandard[0] = newTestSignedTransaction(t, 100, core.TxStandard)
	txsStandard[0].Fee = 10
	txsStandard[1] = newTestSignedTransaction(t, 101, core.TxStandard)
	txsStandard[1].Fee = 5
	txsStandard[2] = newTestSignedTransaction(t, 102, core.TxStandard)
	txsStandard[2].Fee = 15 // Highest fee standard

	txsStimulus[0] = newTestSignedTransaction(t, 200, core.StimulusTx)
	txsStimulus[0].Fee = 0 // Stimulus should prioritize regardless of fee
	txsStimulus[1] = newTestSignedTransaction(t, 201, core.StimulusTx)
	txsStimulus[1].Fee = 0

	txsTax[0] = newTestSignedTransaction(t, 300, core.TaxTx)
	txsTax[0].Fee = 0 // Tax tx

	// Add in a non-sorted order to verify priority queueing
	mp.AddTransaction(txsStandard[0])
	mp.AddTransaction(txsStimulus[0])
	mp.AddTransaction(txsTax[0])
	mp.AddTransaction(txsStandard[1])
	mp.AddTransaction(txsStimulus[1])
	mp.AddTransaction(txsStandard[2]) // Added last, but has highest standard fee

	if mp.Size() != 6 {
		t.Fatalf("Mempool count mismatch before retrieval: got %d, want 6", mp.Size())
	}

	pending := mp.GetPendingTransactions(mp.Size()) // Get all
	if len(pending) != 6 {
		t.Errorf("len(GetPendingTransactions) = %d; want 6", len(pending))
	}

	// Verify priority order (Stimulus -> Highest Fee Standard -> Lower Fee Standard -> Tax -> Oldest for same type/fee)
	// Expected Order: Stimulus0, Stimulus1, Standard2(fee15), Standard0(fee10), Standard1(fee5), Tax0(fee0)
	// (Assuming original timestamp influences for same fee/type)

	expectedOrderIDs := make([]string, 0, 6)
	expectedOrderIDs = append(expectedOrderIDs, hex.EncodeToString(txsStimulus[0].ID))
	expectedOrderIDs = append(expectedOrderIDs, hex.EncodeToString(txsStimulus[1].ID)) // Assuming timestamps make this consistent
	expectedOrderIDs = append(expectedOrderIDs, hex.EncodeToString(txsStandard[2].ID))
	expectedOrderIDs = append(expectedOrderIDs, hex.EncodeToString(txsStandard[0].ID))
	expectedOrderIDs = append(expectedOrderIDs, hex.EncodeToString(txsStandard[1].ID))
	expectedOrderIDs = append(expectedOrderIDs, hex.EncodeToString(txsTax[0].ID))

	for i, tx := range pending {
		if hex.EncodeToString(tx.ID) != expectedOrderIDs[i] {
			t.Errorf("Priority order mismatch at index %d.\nExpected: %s\nGot:      %s", i, expectedOrderIDs[i], hex.EncodeToString(tx.ID))
			t.Logf("Full Expected Order: %v", expectedOrderIDs)
			t.Logf("Full Actual Order:   %v", func() []string {
				ids := make([]string, len(pending))
				for k, v := range pending {
					ids[k] = hex.EncodeToString(v.ID)
				}
				return ids
			}())
			break
		}
	}

	// Test capacity limit
	pendingLimited := mp.GetPendingTransactions(3)
	if len(pendingLimited) != 3 {
		t.Errorf("len(GetPendingTransactions(3)) = %d; want 3", len(pendingLimited))
	}
	if hex.EncodeToString(pendingLimited[0].ID) != expectedOrderIDs[0] {
		t.Errorf("Limited retrieval order mismatch")
	}

	// Test getting more than available (should return all available)
	pendingAll := mp.GetPendingTransactions(100)
	if len(pendingAll) != 6 {
		t.Errorf("len(GetPendingTransactions(100)) = %d; want 6 (all available)", len(pendingAll))
	}

	// Test with maxCount 0 or negative (should return all) - based on current implementation logic
	pendingZero := mp.GetPendingTransactions(0)
	if len(pendingZero) != 6 {
		t.Errorf("len(GetPendingTransactions(0)) = %d; want 6", len(pendingZero))
	}
	pendingNegative := mp.GetPendingTransactions(-1)
	if len(pendingNegative) != 6 {
		t.Errorf("len(GetPendingTransactions(-1)) = %d; want 6", len(pendingNegative))
	}
}

func TestMempoolRemoveTransactions(t *testing.T) {
	mp, _ := NewMempool(10)
	txsToAdd := make([]*core.Transaction, 5)
	for i := 0; i < 5; i++ {
		txsToAdd[i] = newTestSignedTransaction(t, uint64(10+i), core.TxStandard)
		mp.AddTransaction(txsToAdd[i])
		time.Sleep(time.Millisecond) // Ensure unique timestamps
	}

	txsToRemove := []*core.Transaction{txsToAdd[0], txsToAdd[2]} // Remove two specific transactions
	txIDsToRemove := make([][]byte, len(txsToRemove))
	for i, tx := range txsToRemove {
		txIDsToRemove[i] = tx.ID
	}

	if mp.Size() != 5 {
		t.Fatalf("Mempool count before removal = %d; want 5", mp.Size())
	}

	mp.RemoveTransactions(txIDsToRemove)
	if mp.Size() != 3 {
		t.Errorf("Mempool count after removal = %d; want 3", mp.Size())
	}

	// Check that the correct transactions were removed
	for _, removedTx := range txsToRemove {
		if mp.Contains(removedTx.ID) {
			t.Errorf("Transaction %s was not removed", hex.EncodeToString(removedTx.ID))
		}
	}
	// Check that the correct transactions were kept
	for _, keptTx := range []*core.Transaction{txsToAdd[1], txsToAdd[3], txsToAdd[4]} {
		if !mp.Contains(keptTx.ID) {
			t.Errorf("Transaction %s was removed but should have been kept", hex.EncodeToString(keptTx.ID))
		}
	}

	// Test removing non-existent transactions (should not error, count should not change)
	nonExistentTx := newTestSignedTransaction(t, 999, core.TxStandard)
	mp.RemoveTransactions([][]byte{nonExistentTx.ID})
	if mp.Size() != 3 {
		t.Errorf("Mempool count changed after trying to remove non-existent tx; got %d, want 3", mp.Size())
	}
}

func TestMempoolPruneByBlockchain(t *testing.T) {
	mp, _ := NewMempool(10)
	txInBlock1 := newTestSignedTransaction(t, 100, core.TxStandard)
	txInBlock2 := newTestSignedTransaction(t, 101, core.StimulusTx)
	txNotInBlock := newTestSignedTransaction(t, 200, core.TxStandard)

	mp.AddTransaction(txInBlock1)
	mp.AddTransaction(txInBlock2)
	mp.AddTransaction(txNotInBlock)

	if mp.Size() != 3 {
		t.Fatalf("Mempool count before prune = %d; want 3", mp.Size())
	}

	// Create a dummy block that would contain txInBlock1 and txInBlock2
	// Block struct now directly holds []Transaction
	block := &core.Block{
		Transactions: []*core.Transaction{txInBlock1, txInBlock2},
		// Other fields like Height, PrevBlockHash, Hash, etc., are not strictly needed for PruneByBlockchain
		// but would be present in a real block.
	}

	mp.PruneByBlockchain([]*core.Block{block})

	if mp.Size() != 1 {
		t.Errorf("Mempool count after prune = %d; want 1", mp.Size())
	}

	if mp.Contains(txInBlock1.ID) {
		t.Errorf("Transaction %s (in block) was not pruned", hex.EncodeToString(txInBlock1.ID))
	}
	if mp.Contains(txInBlock2.ID) {
		t.Errorf("Transaction %s (in block) was not pruned", hex.EncodeToString(txInBlock2.ID))
	}
	if !mp.Contains(txNotInBlock.ID) {
		t.Errorf("Transaction %s (not in block) was pruned but should have been kept", hex.EncodeToString(txNotInBlock.ID))
	}
}
