package cmd

import (
	"container/heap"
	"encoding/json"
	"fmt"
	"sync"
	"time"
)

const (
	// Mempool size limits
	DefaultMaxMempoolSize     = 100 * 1024 * 1024 // 100MB
	DefaultMaxTransactions    = 10000             // Maximum number of transactions
	DefaultTxExpiryTime      = 24 * time.Hour     // Transaction expiry time
	DefaultMinFee            = 1                  // Minimum fee per transaction
	
	// Priority weights
	FeeWeight       = 1.0   // Weight for transaction fee in priority calculation
	TimeWeight      = 0.1   // Weight for time in priority calculation
	SizeWeight      = -0.01 // Weight for size (negative = smaller is better)
)

// TransactionSource indicates where a transaction originated
type TransactionSource int

const (
	SourceLocal TransactionSource = iota
	SourceNetwork
	SourceAPI
)

func (ts TransactionSource) String() string {
	switch ts {
	case SourceLocal:
		return "local"
	case SourceNetwork:
		return "network"
	case SourceAPI:
		return "api"
	default:
		return "unknown"
	}
}

// MempoolTransaction wraps a transaction with metadata
type MempoolTransaction struct {
	// Core transaction data
	Transaction *SignedTransaction `json:"transaction"`
	TxHash      string            `json:"tx_hash"`
	
	// Metadata
	Source       TransactionSource `json:"source"`
	ReceivedAt   time.Time        `json:"received_at"`
	Size         int              `json:"size"`         // Size in bytes
	Fee          uint64           `json:"fee"`          // Transaction fee
	Priority     float64          `json:"priority"`     // Calculated priority
	
	// Validation status
	IsValidated  bool   `json:"is_validated"`
	ValidationError string `json:"validation_error,omitempty"`
	
	// Processing status
	BroadcastCount int       `json:"broadcast_count"`
	LastBroadcast  time.Time `json:"last_broadcast"`
}

// TransactionPriorityQueue implements a priority queue for transactions
type TransactionPriorityQueue []*MempoolTransaction

func (pq TransactionPriorityQueue) Len() int { return len(pq) }

func (pq TransactionPriorityQueue) Less(i, j int) bool {
	// Higher priority comes first (max heap)
	return pq[i].Priority > pq[j].Priority
}

func (pq TransactionPriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
}

func (pq *TransactionPriorityQueue) Push(x interface{}) {
	*pq = append(*pq, x.(*MempoolTransaction))
}

func (pq *TransactionPriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	*pq = old[0 : n-1]
	return item
}

// MempoolConfig contains configuration for the mempool
type MempoolConfig struct {
	MaxMempoolSize    int64         `json:"max_mempool_size"`
	MaxTransactions   int           `json:"max_transactions"`
	TxExpiryTime     time.Duration `json:"tx_expiry_time"`
	MinFee           uint64        `json:"min_fee"`
	EnableValidation bool          `json:"enable_validation"`
	EnableBroadcast  bool          `json:"enable_broadcast"`
}

// DefaultMempoolConfig returns the default mempool configuration
func DefaultMempoolConfig() *MempoolConfig {
	return &MempoolConfig{
		MaxMempoolSize:    DefaultMaxMempoolSize,
		MaxTransactions:   DefaultMaxTransactions,
		TxExpiryTime:     DefaultTxExpiryTime,
		MinFee:           DefaultMinFee,
		EnableValidation: true,
		EnableBroadcast:  false, // Disabled by default for testing
	}
}

// MempoolStats contains statistics about the mempool
type MempoolStats struct {
	TransactionCount   int           `json:"transaction_count"`
	TotalSize         int64         `json:"total_size"`
	AverageFee        uint64        `json:"average_fee"`
	OldestTransaction time.Time     `json:"oldest_transaction"`
	NewestTransaction time.Time     `json:"newest_transaction"`
	SourceBreakdown   map[string]int `json:"source_breakdown"`
	ValidationStats   ValidationStats `json:"validation_stats"`
}

// ValidationStats tracks validation-related statistics
type ValidationStats struct {
	ValidTransactions   int `json:"valid_transactions"`
	InvalidTransactions int `json:"invalid_transactions"`
	PendingValidation   int `json:"pending_validation"`
}

// Mempool represents the transaction memory pool
type Mempool struct {
	// Configuration
	config *MempoolConfig
	
	// Data storage
	transactions map[string]*MempoolTransaction // hash -> transaction
	priorityQueue TransactionPriorityQueue      // Priority-ordered transactions
	
	// Indexing for fast lookups
	txBySender    map[string][]*MempoolTransaction // sender address -> transactions
	txByReceiver  map[string][]*MempoolTransaction // receiver address -> transactions
	txBySource    map[TransactionSource][]*MempoolTransaction // source -> transactions
	
	// State tracking
	totalSize     int64                // Total size in bytes
	stats         MempoolStats         // Current statistics
	
	// Concurrency control
	mu sync.RWMutex
	
	// Validators
	validators []TransactionValidator
}

// TransactionValidator interface for transaction validation
type TransactionValidator interface {
	ValidateTransaction(tx *SignedTransaction) error
	Name() string
}

// NewMempool creates a new mempool with the given configuration
func NewMempool(config *MempoolConfig) *Mempool {
	if config == nil {
		config = DefaultMempoolConfig()
	}
	
	mp := &Mempool{
		config:        config,
		transactions:  make(map[string]*MempoolTransaction),
		priorityQueue: make(TransactionPriorityQueue, 0),
		txBySender:    make(map[string][]*MempoolTransaction),
		txByReceiver:  make(map[string][]*MempoolTransaction),
		txBySource:    make(map[TransactionSource][]*MempoolTransaction),
		validators:    make([]TransactionValidator, 0),
	}
	
	// Initialize priority queue
	heap.Init(&mp.priorityQueue)
	
	// Add default validators if validation is enabled
	if config.EnableValidation {
		mp.AddValidator(&BasicTransactionValidator{})
		mp.AddValidator(&SignatureValidator{})
		mp.AddValidator(&TemporalValidator{})
		mp.AddValidator(&FeeValidator{MinFee: config.MinFee})
	}
	
	return mp
}

// AddValidator adds a transaction validator to the mempool
func (mp *Mempool) AddValidator(validator TransactionValidator) {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	
	mp.validators = append(mp.validators, validator)
}

// AddTransaction adds a transaction to the mempool
func (mp *Mempool) AddTransaction(tx *SignedTransaction, source TransactionSource) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	
	// Check if transaction already exists
	if _, exists := mp.transactions[tx.TxHash]; exists {
		return fmt.Errorf("transaction %s already exists in mempool", tx.TxHash)
	}
	
	// Parse the underlying transaction for analysis
	var parsedTx Transaction
	if err := json.Unmarshal(tx.Transaction, &parsedTx); err != nil {
		return fmt.Errorf("failed to parse transaction: %w", err)
	}
	
	// Calculate transaction size
	txData, _ := json.Marshal(tx)
	txSize := len(txData)
	
	// Check mempool size limits
	if mp.totalSize+int64(txSize) > mp.config.MaxMempoolSize {
		// Try to evict some transactions first
		if err := mp.evictTransactions(int64(txSize)); err != nil {
			return fmt.Errorf("mempool full and cannot evict enough transactions: %w", err)
		}
	}
	
	if len(mp.transactions) >= mp.config.MaxTransactions {
		return fmt.Errorf("mempool has reached maximum transaction count (%d)", mp.config.MaxTransactions)
	}
	
	// Create mempool transaction
	mempoolTx := &MempoolTransaction{
		Transaction:    tx,
		TxHash:         tx.TxHash,
		Source:         source,
		ReceivedAt:     time.Now().UTC(),
		Size:           txSize,
		Fee:            mp.calculateFee(&parsedTx),
		BroadcastCount: 0,
	}
	
	// Validate transaction if validation is enabled
	if mp.config.EnableValidation {
		if err := mp.validateTransaction(tx); err != nil {
			mempoolTx.IsValidated = true
			mempoolTx.ValidationError = err.Error()
			// Still add invalid transactions for analysis, but mark them
		} else {
			mempoolTx.IsValidated = true
		}
	}
	
	// Calculate priority
	mempoolTx.Priority = mp.calculatePriority(mempoolTx)
	
	// Add to storage
	mp.transactions[tx.TxHash] = mempoolTx
	mp.totalSize += int64(txSize)
	
	// Add to indices
	mp.addToIndices(mempoolTx, &parsedTx)
	
	// Add to priority queue
	heap.Push(&mp.priorityQueue, mempoolTx)
	
	// Update statistics
	mp.updateStats()
	
	return nil
}

// RemoveTransaction removes a transaction from the mempool
func (mp *Mempool) RemoveTransaction(txHash string) error {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	
	return mp.removeTransactionInternal(txHash)
}

// removeTransactionInternal removes a transaction without acquiring the lock (internal use only)
func (mp *Mempool) removeTransactionInternal(txHash string) error {
	mempoolTx, exists := mp.transactions[txHash]
	if !exists {
		return fmt.Errorf("transaction %s not found in mempool", txHash)
	}
	
	// Parse the underlying transaction for cleanup
	var parsedTx Transaction
	if err := json.Unmarshal(mempoolTx.Transaction.Transaction, &parsedTx); err != nil {
		return fmt.Errorf("failed to parse transaction for removal: %w", err)
	}
	
	// Remove from storage
	delete(mp.transactions, txHash)
	mp.totalSize -= int64(mempoolTx.Size)
	
	// Remove from indices
	mp.removeFromIndices(mempoolTx, &parsedTx)
	
	// Note: We don't remove from priority queue here for performance
	// The queue will be cleaned up during the next pop operation
	
	// Update statistics
	mp.updateStats()
	
	return nil
}

// GetTransaction retrieves a transaction from the mempool
func (mp *Mempool) GetTransaction(txHash string) (*MempoolTransaction, error) {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	mempoolTx, exists := mp.transactions[txHash]
	if !exists {
		return nil, fmt.Errorf("transaction %s not found in mempool", txHash)
	}
	
	return mempoolTx, nil
}

// GetTransactionsBySender returns all transactions from a specific sender
func (mp *Mempool) GetTransactionsBySender(senderAddress string) []*MempoolTransaction {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	transactions := mp.txBySender[senderAddress]
	result := make([]*MempoolTransaction, len(transactions))
	copy(result, transactions)
	return result
}

// GetHighestPriorityTransactions returns the N highest priority transactions
func (mp *Mempool) GetHighestPriorityTransactions(count int) []*MempoolTransaction {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	result := make([]*MempoolTransaction, 0, count)
	tempQueue := make(TransactionPriorityQueue, len(mp.priorityQueue))
	copy(tempQueue, mp.priorityQueue)
	
	for i := 0; i < count && len(tempQueue) > 0; i++ {
		tx := heap.Pop(&tempQueue).(*MempoolTransaction)
		
		// Check if transaction still exists (not removed)
		if _, exists := mp.transactions[tx.TxHash]; exists {
			result = append(result, tx)
		}
	}
	
	return result
}

// GetStats returns current mempool statistics
func (mp *Mempool) GetStats() MempoolStats {
	mp.mu.RLock()
	defer mp.mu.RUnlock()
	
	return mp.stats
}

// CleanupExpiredTransactions removes expired transactions
func (mp *Mempool) CleanupExpiredTransactions() int {
	mp.mu.Lock()
	defer mp.mu.Unlock()
	
	expiredCount := 0
	cutoffTime := time.Now().UTC().Add(-mp.config.TxExpiryTime)
	
	for txHash, mempoolTx := range mp.transactions {
		if mempoolTx.ReceivedAt.Before(cutoffTime) {
			// Parse transaction for cleanup
			var parsedTx Transaction
			if err := json.Unmarshal(mempoolTx.Transaction.Transaction, &parsedTx); err != nil {
				continue // Skip if we can't parse
			}
			
			// Remove from storage
			delete(mp.transactions, txHash)
			mp.totalSize -= int64(mempoolTx.Size)
			
			// Remove from indices
			mp.removeFromIndices(mempoolTx, &parsedTx)
			
			expiredCount++
		}
	}
	
	if expiredCount > 0 {
		mp.updateStats()
	}
	
	return expiredCount
}

// Helper methods

func (mp *Mempool) calculateFee(tx *Transaction) uint64 {
	// Simple fee calculation - in a real implementation this would be more sophisticated
	// For now, return a base fee plus fee per output
	return mp.config.MinFee + uint64(len(tx.Outputs))
}

func (mp *Mempool) calculatePriority(mempoolTx *MempoolTransaction) float64 {
	// Priority calculation based on fee, time, and size
	timeFactor := float64(time.Since(mempoolTx.ReceivedAt).Seconds())
	feeFactor := float64(mempoolTx.Fee)
	sizeFactor := float64(mempoolTx.Size)
	
	priority := feeFactor*FeeWeight + timeFactor*TimeWeight + sizeFactor*SizeWeight
	
	// Boost priority for local transactions
	if mempoolTx.Source == SourceLocal {
		priority *= 1.5
	}
	
	// Ensure minimum priority of 0.1 to avoid negative priorities
	if priority < 0.1 {
		priority = 0.1
	}
	
	return priority
}

func (mp *Mempool) validateTransaction(tx *SignedTransaction) error {
	for _, validator := range mp.validators {
		if err := validator.ValidateTransaction(tx); err != nil {
			return fmt.Errorf("validation failed (%s): %w", validator.Name(), err)
		}
	}
	return nil
}

func (mp *Mempool) addToIndices(mempoolTx *MempoolTransaction, parsedTx *Transaction) {
	// Index by sender (from inputs)
	for _, input := range parsedTx.Inputs {
		// In a real implementation, we'd resolve the sender from the input
		// For now, we'll use a placeholder
		senderAddr := "sender_from_input_" + input.PreviousTxHash[:8]
		mp.txBySender[senderAddr] = append(mp.txBySender[senderAddr], mempoolTx)
	}
	
	// Index by receiver (from outputs)
	for _, output := range parsedTx.Outputs {
		mp.txByReceiver[output.Address] = append(mp.txByReceiver[output.Address], mempoolTx)
	}
	
	// Index by source
	mp.txBySource[mempoolTx.Source] = append(mp.txBySource[mempoolTx.Source], mempoolTx)
}

func (mp *Mempool) removeFromIndices(mempoolTx *MempoolTransaction, parsedTx *Transaction) {
	// Remove from sender index
	for _, input := range parsedTx.Inputs {
		senderAddr := "sender_from_input_" + input.PreviousTxHash[:8]
		if txs, exists := mp.txBySender[senderAddr]; exists {
			mp.txBySender[senderAddr] = mp.removeFromSlice(txs, mempoolTx)
		}
	}
	
	// Remove from receiver index
	for _, output := range parsedTx.Outputs {
		if txs, exists := mp.txByReceiver[output.Address]; exists {
			mp.txByReceiver[output.Address] = mp.removeFromSlice(txs, mempoolTx)
		}
	}
	
	// Remove from source index
	if txs, exists := mp.txBySource[mempoolTx.Source]; exists {
		mp.txBySource[mempoolTx.Source] = mp.removeFromSlice(txs, mempoolTx)
	}
}

func (mp *Mempool) removeFromSlice(slice []*MempoolTransaction, target *MempoolTransaction) []*MempoolTransaction {
	for i, tx := range slice {
		if tx.TxHash == target.TxHash {
			return append(slice[:i], slice[i+1:]...)
		}
	}
	return slice
}

func (mp *Mempool) evictTransactions(neededSpace int64) error {
	// Simple eviction: remove lowest priority transactions until we have enough space
	evictedSpace := int64(0)
	
	// Create a reverse priority queue (lowest priority first)
	tempQueue := make(TransactionPriorityQueue, len(mp.priorityQueue))
	copy(tempQueue, mp.priorityQueue)
	
	for evictedSpace < neededSpace && len(tempQueue) > 0 {
		// Find the lowest priority transaction
		minIdx := 0
		for i := 1; i < len(tempQueue); i++ {
			if tempQueue[i].Priority < tempQueue[minIdx].Priority {
				minIdx = i
			}
		}
		
		// Remove lowest priority transaction
		evictTx := tempQueue[minIdx]
		tempQueue = append(tempQueue[:minIdx], tempQueue[minIdx+1:]...)
		
		if _, exists := mp.transactions[evictTx.TxHash]; exists {
			// Remove transaction without acquiring lock (we already hold it)
			mp.removeTransactionInternal(evictTx.TxHash)
			evictedSpace += int64(evictTx.Size)
		}
	}
	
	if evictedSpace < neededSpace {
		return fmt.Errorf("could not evict enough transactions: needed %d, evicted %d", neededSpace, evictedSpace)
	}
	
	return nil
}

func (mp *Mempool) updateStats() {
	// Update statistics (called with lock held)
	validCount := 0
	invalidCount := 0
	totalFee := uint64(0)
	var oldest, newest time.Time
	sourceBreakdown := make(map[string]int)
	
	first := true
	for _, mempoolTx := range mp.transactions {
		if mempoolTx.IsValidated && mempoolTx.ValidationError == "" {
			validCount++
		} else if mempoolTx.IsValidated {
			invalidCount++
		}
		
		totalFee += mempoolTx.Fee
		
		if first {
			oldest = mempoolTx.ReceivedAt
			newest = mempoolTx.ReceivedAt
			first = false
		} else {
			if mempoolTx.ReceivedAt.Before(oldest) {
				oldest = mempoolTx.ReceivedAt
			}
			if mempoolTx.ReceivedAt.After(newest) {
				newest = mempoolTx.ReceivedAt
			}
		}
		
		sourceBreakdown[mempoolTx.Source.String()]++
	}
	
	avgFee := uint64(0)
	if len(mp.transactions) > 0 {
		avgFee = totalFee / uint64(len(mp.transactions))
	}
	
	mp.stats = MempoolStats{
		TransactionCount:   len(mp.transactions),
		TotalSize:         mp.totalSize,
		AverageFee:        avgFee,
		OldestTransaction: oldest,
		NewestTransaction: newest,
		SourceBreakdown:   sourceBreakdown,
		ValidationStats: ValidationStats{
			ValidTransactions:   validCount,
			InvalidTransactions: invalidCount,
			PendingValidation:   len(mp.transactions) - validCount - invalidCount,
		},
	}
}