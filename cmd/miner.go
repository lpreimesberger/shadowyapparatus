package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"sort"
	"sync"
	"time"
)

// Miner represents the block mining service
type Miner struct {
	config     *ShadowConfig
	blockchain *Blockchain
	mempool    *Mempool
	farming    *FarmingService
	
	// Mining state
	isRunning     bool
	minerAddress  string
	mu            sync.RWMutex
	
	// Context and synchronization
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
	
	// Mining statistics
	stats MiningStats
	statsMutex sync.RWMutex
}

// MiningStats contains mining performance statistics
type MiningStats struct {
	StartTime        time.Time `json:"start_time"`
	BlocksMined      uint64    `json:"blocks_mined"`
	TotalRewards     uint64    `json:"total_rewards_satoshi"`
	LastBlockTime    time.Time `json:"last_block_time"`
	AverageBlockTime time.Duration `json:"average_block_time"`
	
	// Challenge statistics
	ChallengesAttempted uint64 `json:"challenges_attempted"`
	ValidProofs         uint64 `json:"valid_proofs"`
	ProofSuccessRate    float64 `json:"proof_success_rate"`
	
	// Transaction processing
	TransactionsProcessed uint64 `json:"transactions_processed"`
	FeesCollected        uint64 `json:"fees_collected_satoshi"`
}

// ProofOfStorage represents a proof-of-storage solution
type ProofOfStorage struct {
	Challenge     []byte    `json:"challenge"`
	Solution      []byte    `json:"solution"`
	PlotFile      string    `json:"plot_file"`
	Offset        int64     `json:"offset"`
	Quality       uint64    `json:"quality"`
	PrivateKey    string    `json:"private_key"`
	Signature     string    `json:"signature"`
	Timestamp     time.Time `json:"timestamp"`
}

// NewMiner creates a new mining service
func NewMiner(config *ShadowConfig, blockchain *Blockchain, mempool *Mempool, farming *FarmingService, minerAddress string) *Miner {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &Miner{
		config:       config,
		blockchain:   blockchain,
		mempool:      mempool,
		farming:      farming,
		minerAddress: minerAddress,
		ctx:          ctx,
		cancel:       cancel,
		stats: MiningStats{
			StartTime: time.Now().UTC(),
		},
	}
}

// Start begins the mining process
func (m *Miner) Start() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if m.isRunning {
		return fmt.Errorf("miner is already running")
	}
	
	log.Printf("Starting miner with address: %s", m.minerAddress)
	
	// Start mining loop
	m.wg.Add(1)
	go m.miningLoop()
	
	m.isRunning = true
	log.Printf("Miner started successfully")
	
	return nil
}

// Stop stops the mining process
func (m *Miner) Stop() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !m.isRunning {
		return nil
	}
	
	log.Printf("Stopping miner...")
	
	m.cancel()
	m.wg.Wait()
	
	m.isRunning = false
	log.Printf("Miner stopped")
	
	return nil
}

// IsRunning returns whether the miner is currently active
func (m *Miner) IsRunning() bool {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.isRunning
}

// GetStats returns current mining statistics
func (m *Miner) GetStats() MiningStats {
	m.statsMutex.RLock()
	defer m.statsMutex.RUnlock()
	return m.stats
}

// miningLoop is the main mining process
func (m *Miner) miningLoop() {
	defer m.wg.Done()
	
	log.Printf("Mining loop started")
	
	// Target block time (10 minutes)
	targetInterval := time.Duration(TargetBlockTime) * time.Second
	ticker := time.NewTicker(targetInterval)
	defer ticker.Stop()
	
	for {
		select {
		case <-m.ctx.Done():
			log.Printf("Mining loop stopping")
			return
			
		case <-ticker.C:
			if err := m.attemptBlockGeneration(); err != nil {
				log.Printf("Block generation failed: %v", err)
			}
		}
	}
}

// attemptBlockGeneration tries to generate a new block
func (m *Miner) attemptBlockGeneration() error {
	log.Printf("Attempting to generate new block...")
	
	// Get current blockchain tip
	currentTip, err := m.blockchain.GetTip()
	if err != nil {
		return fmt.Errorf("failed to get blockchain tip: %w", err)
	}
	
	// Create block challenge from current state
	challenge := m.generateChallenge(currentTip)
	
	// Attempt to solve proof-of-storage challenge
	proof, err := m.solveStorageChallenge(challenge)
	if err != nil {
		log.Printf("Failed to solve storage challenge: %v", err)
		m.updateChallengeStats(false)
		return err
	}
	
	log.Printf("Found valid proof-of-storage solution!")
	m.updateChallengeStats(true)
	
	// Collect transactions from mempool
	transactions := m.collectTransactions()
	log.Printf("Collected %d transactions from mempool", len(transactions))
	
	// Calculate total fees
	totalFees := m.calculateTotalFees(transactions)
	
	// Create coinbase transaction (block reward + fees)
	coinbase, err := m.createCoinbaseTransaction(currentTip.Header.Height+1, totalFees)
	if err != nil {
		return fmt.Errorf("failed to create coinbase transaction: %w", err)
	}
	
	// Prepend coinbase to transactions
	allTransactions := append([]SignedTransaction{*coinbase}, transactions...)
	
	// Create new block
	newBlock, err := m.createBlock(currentTip, allTransactions, proof)
	if err != nil {
		return fmt.Errorf("failed to create block: %w", err)
	}
	
	// Add block to blockchain
	if err := m.blockchain.AddBlock(newBlock); err != nil {
		return fmt.Errorf("failed to add block to blockchain: %w", err)
	}
	
	// Remove processed transactions from mempool
	m.removeProcessedTransactions(transactions)
	
	// Update mining statistics
	m.updateMiningStats(newBlock, totalFees, len(transactions))
	
	log.Printf("✅ Successfully mined block %d with hash %s", 
		newBlock.Header.Height, newBlock.Hash()[:16]+"...")
	log.Printf("   Reward: %.8f SHADOW, Fees: %.8f SHADOW, Transactions: %d",
		float64(CalculateBlockReward(newBlock.Header.Height))/float64(SatoshisPerShadow),
		float64(totalFees)/float64(SatoshisPerShadow),
		len(transactions))
	
	return nil
}

// generateChallenge creates a proof-of-storage challenge
func (m *Miner) generateChallenge(currentTip *Block) []byte {
	// Create challenge based on previous block hash + timestamp
	challengeData := fmt.Sprintf("%s:%d:%s", 
		currentTip.Hash(), 
		time.Now().Unix(), 
		m.minerAddress)
	
	hash := sha256.Sum256([]byte(challengeData))
	return hash[:]
}

// solveStorageChallenge attempts to find a valid proof-of-storage solution
func (m *Miner) solveStorageChallenge(challenge []byte) (*ProofOfStorage, error) {
	if m.farming == nil || !m.farming.IsRunning() {
		return nil, fmt.Errorf("farming service not available")
	}
	
	// Convert challenge to hex for farming service
	challengeHex := hex.EncodeToString(challenge)
	
	// Create storage challenge
	storageChallenge := &StorageChallenge{
		ID:        fmt.Sprintf("mining_%d", time.Now().UnixNano()),
		Challenge: []byte(challengeHex),
		Timestamp: time.Now().UTC(),
		Difficulty: 1,
	}
	
	// Submit to farming service and get proof
	storageProof := m.farming.SubmitChallenge(storageChallenge)
	if storageProof.Error != "" {
		return nil, fmt.Errorf("storage proof error: %s", storageProof.Error)
	}
	
	if !storageProof.Valid {
		return nil, fmt.Errorf("invalid storage proof")
	}
	
	// Calculate proof quality (lower is better)
	quality := m.calculateProofQuality(challenge, storageProof)
	
	proof := &ProofOfStorage{
		Challenge:  challenge,
		Solution:   []byte(storageProof.Signature),
		PlotFile:   storageProof.PlotFile,
		Offset:     storageProof.Offset,
		Quality:    quality,
		PrivateKey: storageProof.PrivateKey,
		Signature:  storageProof.Signature,
		Timestamp:  time.Now().UTC(),
	}
	
	return proof, nil
}

// calculateProofQuality determines the quality of a proof-of-storage solution
func (m *Miner) calculateProofQuality(challenge []byte, proof *StorageProof) uint64 {
	// Combine challenge with proof signature
	combined := append(challenge, []byte(proof.Signature)...)
	hash := sha256.Sum256(combined)
	
	// Convert first 8 bytes to uint64 (lower values = better quality)
	quality := uint64(hash[0])<<56 | uint64(hash[1])<<48 | uint64(hash[2])<<40 | uint64(hash[3])<<32 |
		uint64(hash[4])<<24 | uint64(hash[5])<<16 | uint64(hash[6])<<8 | uint64(hash[7])
	
	return quality
}

// collectTransactions gathers transactions from the mempool for inclusion in a block
func (m *Miner) collectTransactions() []SignedTransaction {
	// Get highest priority transactions
	maxTxsPerBlock := 1000 // Limit transactions per block
	
	mempoolTxs := m.mempool.GetHighestPriorityTransactions(maxTxsPerBlock)
	
	// Filter valid transactions and convert to SignedTransaction
	var validTxs []SignedTransaction
	for _, mempoolTx := range mempoolTxs {
		// Basic validation (could add more sophisticated checks)
		if mempoolTx.TxHash != "" && mempoolTx.Transaction != nil {
			validTxs = append(validTxs, *mempoolTx.Transaction)
		}
	}
	
	// Sort by fee rate (highest first)
	sort.Slice(validTxs, func(i, j int) bool {
		feeI := m.estimateTransactionFee(&validTxs[i])
		feeJ := m.estimateTransactionFee(&validTxs[j])
		return feeI > feeJ
	})
	
	return validTxs
}

// estimateTransactionFee estimates the fee for a transaction
func (m *Miner) estimateTransactionFee(tx *SignedTransaction) uint64 {
	// Estimate transaction size (rough approximation)
	txData, _ := json.Marshal(tx)
	txSize := len(txData)
	
	// Calculate fee based on size
	return CalculateTransactionFee(txSize, 0)
}

// calculateTotalFees sums up all transaction fees in a block
func (m *Miner) calculateTotalFees(transactions []SignedTransaction) uint64 {
	var totalFees uint64
	
	for _, tx := range transactions {
		fee := m.estimateTransactionFee(&tx)
		totalFees += fee
	}
	
	return totalFees
}

// createCoinbaseTransaction creates the coinbase transaction for block rewards
func (m *Miner) createCoinbaseTransaction(height uint64, fees uint64) (*SignedTransaction, error) {
	// Calculate block reward
	blockReward := CalculateBlockReward(height)
	totalReward := blockReward + fees
	
	// Create coinbase transaction
	coinbaseTx := &Transaction{
		Version: 1,
		Inputs:  []TransactionInput{}, // Coinbase has no inputs
		Outputs: []TransactionOutput{
			{
				Value:   totalReward,
				Address: m.minerAddress,
			},
		},
		Timestamp: time.Now().UTC(),
		NotUntil:  time.Now().UTC(),
		Nonce:     height, // Use height as nonce for uniqueness
	}
	
	// Hash the transaction
	txHash, err := coinbaseTx.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to hash coinbase transaction: %w", err)
	}
	
	// Marshal transaction to json.RawMessage
	txData, err := json.Marshal(coinbaseTx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal coinbase transaction: %w", err)
	}
	
	// Create signed transaction (self-signed for coinbase)
	signedCoinbase := &SignedTransaction{
		Transaction: json.RawMessage(txData),
		Signature:   fmt.Sprintf("coinbase_signature_%d", height),
		TxHash:      txHash,
		SignerKey:   m.minerAddress,
		Algorithm:   "coinbase",
		Header: JOSEHeader{
			Algorithm: "coinbase",
			Type:      "JWT",
		},
	}
	
	return signedCoinbase, nil
}

// createBlock constructs a new block with the given transactions and proof
func (m *Miner) createBlock(previousBlock *Block, transactions []SignedTransaction, proof *ProofOfStorage) (*Block, error) {
	now := time.Now().UTC()
	
	// Create block header
	header := BlockHeader{
		Version:           1,
		PreviousBlockHash: previousBlock.Hash(),
		Timestamp:         now,
		Height:            previousBlock.Header.Height + 1,
		Nonce:             0, // Could implement nonce for additional randomness
		
		// Proof-of-storage fields
		ChallengeSeed: hex.EncodeToString(proof.Challenge),
		ProofHash:     hex.EncodeToString(proof.Solution),
		FarmerAddress: m.minerAddress,
	}
	
	// Create block body
	body := BlockBody{
		Transactions: transactions,
		TxCount:      uint32(len(transactions)),
	}
	
	// Calculate merkle root
	header.MerkleRoot = calculateMerkleRoot(transactions)
	
	// Create block
	block := &Block{
		Header: header,
		Body:   body,
	}
	
	return block, nil
}

// removeProcessedTransactions removes mined transactions from the mempool
func (m *Miner) removeProcessedTransactions(transactions []SignedTransaction) {
	for _, tx := range transactions {
		if err := m.mempool.RemoveTransaction(tx.TxHash); err != nil {
			log.Printf("Warning: failed to remove transaction %s from mempool: %v", tx.TxHash, err)
		}
	}
}

// updateMiningStats updates mining performance statistics
func (m *Miner) updateMiningStats(block *Block, fees uint64, txCount int) {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()
	
	blockReward := CalculateBlockReward(block.Header.Height)
	
	m.stats.BlocksMined++
	m.stats.TotalRewards += blockReward + fees
	m.stats.LastBlockTime = block.Header.Timestamp
	m.stats.TransactionsProcessed += uint64(txCount)
	m.stats.FeesCollected += fees
	
	// Calculate average block time
	if m.stats.BlocksMined > 1 {
		totalTime := time.Since(m.stats.StartTime)
		m.stats.AverageBlockTime = totalTime / time.Duration(m.stats.BlocksMined)
	}
}

// updateChallengeStats updates proof-of-storage challenge statistics
func (m *Miner) updateChallengeStats(success bool) {
	m.statsMutex.Lock()
	defer m.statsMutex.Unlock()
	
	m.stats.ChallengesAttempted++
	if success {
		m.stats.ValidProofs++
	}
	
	if m.stats.ChallengesAttempted > 0 {
		m.stats.ProofSuccessRate = float64(m.stats.ValidProofs) / float64(m.stats.ChallengesAttempted) * 100.0
	}
}

// SetMiningAddress changes the address that receives block rewards
func (m *Miner) SetMiningAddress(address string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	
	if !IsValidAddress(address) {
		return fmt.Errorf("invalid mining address: %s", address)
	}
	
	oldAddress := m.minerAddress
	m.minerAddress = address
	
	log.Printf("Mining address changed from %s to %s", oldAddress, address)
	return nil
}

// GetMiningAddress returns the current mining address
func (m *Miner) GetMiningAddress() string {
	m.mu.RLock()
	defer m.mu.RUnlock()
	return m.minerAddress
}

// ForceBlockGeneration manually triggers block generation (useful for testing)
func (m *Miner) ForceBlockGeneration() error {
	if !m.IsRunning() {
		return fmt.Errorf("miner is not running")
	}
	
	log.Printf("Forcing block generation...")
	return m.attemptBlockGeneration()
}

// GetEstimatedNextBlock estimates when the next block will be generated
func (m *Miner) GetEstimatedNextBlock() time.Time {
	m.statsMutex.RLock()
	defer m.statsMutex.RUnlock()
	
	if m.stats.LastBlockTime.IsZero() {
		return time.Now().Add(time.Duration(TargetBlockTime) * time.Second)
	}
	
	// Use average block time or target time, whichever is more conservative
	avgTime := m.stats.AverageBlockTime
	if avgTime == 0 || avgTime > time.Duration(TargetBlockTime)*time.Second*2 {
		avgTime = time.Duration(TargetBlockTime) * time.Second
	}
	
	return m.stats.LastBlockTime.Add(avgTime)
}