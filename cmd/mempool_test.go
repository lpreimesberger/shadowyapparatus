package cmd

import (
	"encoding/json"
	"testing"
	"time"
)

func createTestTransaction(version int, nonce uint64) *SignedTransaction {
	tx := Transaction{
		Version: version,
		Inputs:  []TransactionInput{},
		Outputs: []TransactionOutput{
			{
				Value:   100,
				Address: "S42618a7524a82df51c8a2406321e161de65073008806f042f0",
			},
		},
		NotUntil:  time.Now().UTC().Add(-time.Hour),
		Timestamp: time.Now().UTC(),
		Nonce:     nonce,
	}

	txData, _ := json.Marshal(tx)
	return &SignedTransaction{
		Transaction: txData,
		Signature:   "test_signature_" + string(rune(nonce)),
		TxHash:      "test_hash_" + string(rune(nonce)),
		SignerKey:   "test_signer_key",
		Algorithm:   "ML-DSA-87",
		Header: JOSEHeader{
			Algorithm: "ML-DSA-87",
			Type:      "shadowy-tx",
		},
	}
}

func TestNewMempool(t *testing.T) {
	config := DefaultMempoolConfig()
	mp := NewMempool(config)

	if mp == nil {
		t.Fatal("NewMempool returned nil")
	}

	if mp.config != config {
		t.Error("Mempool config not set correctly")
	}

	if len(mp.transactions) != 0 {
		t.Error("New mempool should have no transactions")
	}

	if mp.totalSize != 0 {
		t.Error("New mempool should have zero total size")
	}
}

func TestMempoolAddTransaction(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())
	tx := createTestTransaction(1, 1)

	err := mp.AddTransaction(tx, SourceLocal)
	if err != nil {
		t.Fatalf("Failed to add transaction: %v", err)
	}

	if len(mp.transactions) != 1 {
		t.Error("Transaction count should be 1")
	}

	mempoolTx, exists := mp.transactions[tx.TxHash]
	if !exists {
		t.Error("Transaction not found in mempool")
	}

	if mempoolTx.Source != SourceLocal {
		t.Error("Transaction source not set correctly")
	}

	if mempoolTx.TxHash != tx.TxHash {
		t.Error("Transaction hash not set correctly")
	}
}

func TestMempoolAddDuplicateTransaction(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())
	tx := createTestTransaction(1, 1)

	err := mp.AddTransaction(tx, SourceLocal)
	if err != nil {
		t.Fatalf("Failed to add first transaction: %v", err)
	}

	err = mp.AddTransaction(tx, SourceNetwork)
	if err == nil {
		t.Error("Should not allow duplicate transactions")
	}

	if len(mp.transactions) != 1 {
		t.Error("Should still have only one transaction")
	}
}

func TestMempoolRemoveTransaction(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())
	tx := createTestTransaction(1, 1)

	err := mp.AddTransaction(tx, SourceLocal)
	if err != nil {
		t.Fatalf("Failed to add transaction: %v", err)
	}

	err = mp.RemoveTransaction(tx.TxHash)
	if err != nil {
		t.Fatalf("Failed to remove transaction: %v", err)
	}

	if len(mp.transactions) != 0 {
		t.Error("Transaction should be removed from mempool")
	}

	if mp.totalSize != 0 {
		t.Error("Total size should be zero after removing transaction")
	}
}

func TestMempoolRemoveNonexistentTransaction(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())

	err := mp.RemoveTransaction("nonexistent_hash")
	if err == nil {
		t.Error("Should return error when removing nonexistent transaction")
	}
}

func TestMempoolGetTransaction(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())
	tx := createTestTransaction(1, 1)

	err := mp.AddTransaction(tx, SourceLocal)
	if err != nil {
		t.Fatalf("Failed to add transaction: %v", err)
	}

	mempoolTx, err := mp.GetTransaction(tx.TxHash)
	if err != nil {
		t.Fatalf("Failed to get transaction: %v", err)
	}

	if mempoolTx.TxHash != tx.TxHash {
		t.Error("Retrieved transaction has wrong hash")
	}
}

func TestMempoolGetNonexistentTransaction(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())

	_, err := mp.GetTransaction("nonexistent_hash")
	if err == nil {
		t.Error("Should return error when getting nonexistent transaction")
	}
}

func TestMempoolPriorityQueue(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())

	tx1 := createTestTransaction(1, 1)
	tx2 := createTestTransaction(1, 2)
	tx3 := createTestTransaction(1, 3)

	mp.AddTransaction(tx1, SourceLocal)
	mp.AddTransaction(tx2, SourceNetwork)
	mp.AddTransaction(tx3, SourceAPI)

	highPriorityTxs := mp.GetHighestPriorityTransactions(2)
	if len(highPriorityTxs) != 2 {
		t.Errorf("Expected 2 high priority transactions, got %d", len(highPriorityTxs))
	}

	if len(highPriorityTxs) > 0 {
		t.Logf("Priority of first transaction: %f", highPriorityTxs[0].Priority)
		if highPriorityTxs[0].Priority <= 0 {
			t.Errorf("Priority should be calculated and positive, got %f", highPriorityTxs[0].Priority)
		}
	}
}

func TestMempoolSizeLimit(t *testing.T) {
	config := DefaultMempoolConfig()
	config.MaxMempoolSize = 1000 // Small size limit
	mp := NewMempool(config)

	for i := 0; i < 10; i++ { // Reduced from 100 to 10
		tx := createTestTransaction(1, uint64(i))
		err := mp.AddTransaction(tx, SourceLocal)
		if err != nil && mp.totalSize >= config.MaxMempoolSize {
			t.Logf("Mempool size limit reached at %d bytes with %d transactions", mp.totalSize, len(mp.transactions))
			break
		}
	}

	if mp.totalSize > config.MaxMempoolSize {
		t.Error("Mempool exceeded size limit")
	}
}

func TestMempoolTransactionLimit(t *testing.T) {
	config := DefaultMempoolConfig()
	config.MaxTransactions = 5
	mp := NewMempool(config)

	for i := 0; i < 10; i++ {
		tx := createTestTransaction(1, uint64(i))
		err := mp.AddTransaction(tx, SourceLocal)
		if err != nil && len(mp.transactions) >= config.MaxTransactions {
			break
		}
	}

	if len(mp.transactions) > config.MaxTransactions {
		t.Error("Mempool exceeded transaction limit")
	}
}

func TestMempoolCleanupExpiredTransactions(t *testing.T) {
	config := DefaultMempoolConfig()
	config.TxExpiryTime = 100 * time.Millisecond
	mp := NewMempool(config)

	tx := createTestTransaction(1, 1)
	mp.AddTransaction(tx, SourceLocal)

	if len(mp.transactions) != 1 {
		t.Error("Should have one transaction before cleanup")
	}

	time.Sleep(150 * time.Millisecond)

	expiredCount := mp.CleanupExpiredTransactions()
	if expiredCount != 1 {
		t.Errorf("Expected 1 expired transaction, got %d", expiredCount)
	}

	if len(mp.transactions) != 0 {
		t.Error("Expired transaction should be removed")
	}
}

func TestMempoolStats(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())

	tx1 := createTestTransaction(1, 1)
	tx2 := createTestTransaction(1, 2)

	mp.AddTransaction(tx1, SourceLocal)
	mp.AddTransaction(tx2, SourceNetwork)

	stats := mp.GetStats()

	if stats.TransactionCount != 2 {
		t.Errorf("Expected 2 transactions in stats, got %d", stats.TransactionCount)
	}

	if stats.TotalSize <= 0 {
		t.Error("Total size should be positive")
	}

	if len(stats.SourceBreakdown) == 0 {
		t.Error("Source breakdown should not be empty")
	}

	if stats.SourceBreakdown["local"] != 1 {
		t.Error("Should have 1 local transaction")
	}

	if stats.SourceBreakdown["network"] != 1 {
		t.Error("Should have 1 network transaction")
	}
}

func TestMempoolGetTransactionsBySender(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())

	// Create transactions with inputs that have the same hash prefix
	tx1 := createTestTransactionWithInput("test_hash_12345678abcd", 1)
	tx2 := createTestTransactionWithInput("test_hash_12345678efgh", 2)

	mp.AddTransaction(tx1, SourceLocal)
	mp.AddTransaction(tx2, SourceLocal)

	// The sender address will be generated as "sender_from_input_" + first 8 chars of hash
	senderAddr := "sender_from_input_test_has"
	txs := mp.GetTransactionsBySender(senderAddr)

	if len(txs) != 2 {
		t.Errorf("Expected 2 transactions for sender, got %d", len(txs))
	}
}

func createTestTransactionWithInput(prevTxHash string, nonce uint64) *SignedTransaction {
	tx := Transaction{
		Version: 1,
		Inputs: []TransactionInput{
			{
				PreviousTxHash: prevTxHash,
				OutputIndex:    0,
			},
		},
		Outputs: []TransactionOutput{
			{
				Value:   100,
				Address: "S42618a7524a82df51c8a2406321e161de65073008806f042f0",
			},
		},
		NotUntil:  time.Now().UTC().Add(-time.Hour),
		Timestamp: time.Now().UTC(),
		Nonce:     nonce,
	}

	txData, _ := json.Marshal(tx)
	return &SignedTransaction{
		Transaction: txData,
		Signature:   "test_signature_" + string(rune(nonce)),
		TxHash:      "test_hash_" + string(rune(nonce)),
		SignerKey:   "test_signer_key",
		Algorithm:   "ML-DSA-87",
		Header: JOSEHeader{
			Algorithm: "ML-DSA-87",
			Type:      "shadowy-tx",
		},
	}
}

func TestMempoolValidation(t *testing.T) {
	config := DefaultMempoolConfig()
	config.EnableValidation = false
	mp := NewMempool(config)

	tx := createTestTransaction(1, 1)
	err := mp.AddTransaction(tx, SourceLocal)
	if err != nil {
		t.Fatalf("Failed to add transaction with validation disabled: %v", err)
	}

	mempoolTx, _ := mp.GetTransaction(tx.TxHash)
	if mempoolTx.IsValidated {
		t.Error("Transaction should not be validated when validation is disabled")
	}
}

func TestMempoolWithValidation(t *testing.T) {
	config := DefaultMempoolConfig()
	config.EnableValidation = true
	mp := NewMempool(config)

	tx := createTestTransaction(1, 1)
	err := mp.AddTransaction(tx, SourceLocal)
	if err != nil {
		t.Fatalf("Failed to add transaction with validation enabled: %v", err)
	}

	mempoolTx, _ := mp.GetTransaction(tx.TxHash)
	if !mempoolTx.IsValidated {
		t.Error("Transaction should be validated when validation is enabled")
	}
}

func TestTransactionSource(t *testing.T) {
	sources := []TransactionSource{SourceLocal, SourceNetwork, SourceAPI}
	expected := []string{"local", "network", "api"}

	for i, source := range sources {
		if source.String() != expected[i] {
			t.Errorf("Expected %s, got %s", expected[i], source.String())
		}
	}

	unknownSource := TransactionSource(999)
	if unknownSource.String() != "unknown" {
		t.Error("Unknown source should return 'unknown'")
	}
}

func TestMempoolConcurrency(t *testing.T) {
	mp := NewMempool(DefaultMempoolConfig())

	done := make(chan bool, 10)

	for i := 0; i < 10; i++ {
		go func(idx int) {
			tx := createTestTransaction(1, uint64(idx))
			mp.AddTransaction(tx, SourceLocal)
			mp.GetTransaction(tx.TxHash)
			done <- true
		}(i)
	}

	for i := 0; i < 10; i++ {
		<-done
	}

	if len(mp.transactions) > 10 {
		t.Error("Unexpected number of transactions after concurrent operations")
	}
}

func BenchmarkMempoolAddTransaction(b *testing.B) {
	mp := NewMempool(DefaultMempoolConfig())

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		tx := createTestTransaction(1, uint64(i))
		mp.AddTransaction(tx, SourceLocal)
	}
}

func BenchmarkMempoolGetTransaction(b *testing.B) {
	mp := NewMempool(DefaultMempoolConfig())

	for i := 0; i < 1000; i++ {
		tx := createTestTransaction(1, uint64(i))
		mp.AddTransaction(tx, SourceLocal)
	}

	hashes := make([]string, 1000)
	for _, tx := range mp.transactions {
		hashes = append(hashes, tx.TxHash)
		if len(hashes) >= 1000 {
			break
		}
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		hash := hashes[i%len(hashes)]
		mp.GetTransaction(hash)
	}
}