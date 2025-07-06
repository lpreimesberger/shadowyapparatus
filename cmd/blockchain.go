package cmd

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"sync"
	"time"
)

// Block represents a single block in the blockchain
type Block struct {
	Header BlockHeader    `json:"header"`
	Body   BlockBody      `json:"body"`
}

// BlockHeader contains the block metadata
type BlockHeader struct {
	Version           uint32    `json:"version"`
	PreviousBlockHash string    `json:"previous_block_hash"`
	MerkleRoot        string    `json:"merkle_root"`
	Timestamp         time.Time `json:"timestamp"`
	Height            uint64    `json:"height"`
	Nonce             uint64    `json:"nonce"`
	
	// Proof-of-storage specific fields
	ChallengeSeed     string    `json:"challenge_seed"`
	ProofHash         string    `json:"proof_hash"`
	FarmerAddress     string    `json:"farmer_address"`
}

// BlockBody contains the block transactions and other data
type BlockBody struct {
	Transactions []SignedTransaction `json:"transactions"`
	TxCount      uint32              `json:"tx_count"`
}

// GenesisBlock represents the first block in the chain
type GenesisBlock struct {
	Block
	GenesisTimestamp time.Time `json:"genesis_timestamp"`
	NetworkID        string    `json:"network_id"`
	InitialSupply    uint64    `json:"initial_supply"`
}

// Blockchain manages the chain of blocks
type Blockchain struct {
	config *ShadowConfig
	
	// Chain state
	blocks       map[string]*Block  // hash -> block
	blocksByHeight map[uint64]*Block // height -> block
	tipHash      string             // hash of the latest block
	tipHeight    uint64             // height of the latest block
	
	// Synchronization
	mu sync.RWMutex
	
	// Storage
	dataDir string
}

// BlockchainStats contains blockchain statistics
type BlockchainStats struct {
	TipHeight     uint64    `json:"tip_height"`
	TipHash       string    `json:"tip_hash"`
	TotalBlocks   uint64    `json:"total_blocks"`
	GenesisHash   string    `json:"genesis_hash"`
	LastBlockTime time.Time `json:"last_block_time"`
	
	// Transaction stats
	TotalTransactions uint64 `json:"total_transactions"`
	AvgBlockSize     uint64 `json:"avg_block_size"`
	AvgTxPerBlock    float64 `json:"avg_tx_per_block"`
}

// NewBlockchain creates a new blockchain instance
func NewBlockchain(config *ShadowConfig) (*Blockchain, error) {
	bc := &Blockchain{
		config:         config,
		blocks:         make(map[string]*Block),
		blocksByHeight: make(map[uint64]*Block),
		dataDir:        config.BlockchainDirectory,
	}
	
	// Ensure blockchain directory exists
	if err := os.MkdirAll(bc.dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create blockchain directory: %w", err)
	}
	
	// Load existing blockchain or create genesis
	if err := bc.initialize(); err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
	}
	
	return bc, nil
}

// initialize loads the blockchain from disk or creates genesis block
func (bc *Blockchain) initialize() error {
	// Check if genesis block exists
	genesisPath := filepath.Join(bc.dataDir, "genesis.json")
	if _, err := os.Stat(genesisPath); os.IsNotExist(err) {
		// Create genesis block
		genesis, err := bc.createGenesisBlock()
		if err != nil {
			return fmt.Errorf("failed to create genesis block: %w", err)
		}
		
		// Add genesis to chain
		hash := genesis.Hash()
		bc.blocks[hash] = &genesis.Block
		bc.blocksByHeight[0] = &genesis.Block
		bc.tipHash = hash
		bc.tipHeight = 0
		
		// Save genesis block
		if err := bc.saveGenesisBlock(genesis); err != nil {
			return fmt.Errorf("failed to save genesis block: %w", err)
		}
		
		fmt.Printf("Created genesis block: %s\n", hash)
	} else {
		// Load existing blockchain
		if err := bc.loadBlockchain(); err != nil {
			return fmt.Errorf("failed to load blockchain: %w", err)
		}
		
		fmt.Printf("Loaded blockchain: height=%d, tip=%s\n", bc.tipHeight, bc.tipHash[:16]+"...")
	}
	
	return nil
}

// createGenesisBlock creates the first block in the chain
func (bc *Blockchain) createGenesisBlock() (*GenesisBlock, error) {
	now := time.Now().UTC()
	
	// Create genesis transaction (minimal bootstrap - 1 SHADOW only)
	genesisTx := &Transaction{
		Version:   1,
		Inputs:    []TransactionInput{},
		Outputs: []TransactionOutput{
			{
				Value:   1 * SatoshisPerShadow, // 1 SHADOW bootstrap (100,000,000 satoshis)
				Address: "S42618a7524a82df51c8a2406321e161de65073008806f042f0", // Genesis address
			},
		},
		Timestamp: now,
		NotUntil:  now,
		Nonce:     0,
	}
	
	// Sign genesis transaction (self-signed for bootstrap)
	genesisHash, err := genesisTx.Hash()
	if err != nil {
		return nil, fmt.Errorf("failed to hash genesis transaction: %w", err)
	}
	
	// Marshal transaction to json.RawMessage
	txData, err := json.Marshal(genesisTx)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal genesis transaction: %w", err)
	}
	
	signedGenesisTx := &SignedTransaction{
		Transaction: json.RawMessage(txData),
		Signature:   "genesis_signature",
		TxHash:      genesisHash,
		SignerKey:   "genesis_signer",
		Algorithm:   "genesis",
		Header: JOSEHeader{
			Algorithm: "genesis",
			Type:      "JWT",
		},
	}
	
	// Create genesis block
	header := BlockHeader{
		Version:           1,
		PreviousBlockHash: "0000000000000000000000000000000000000000000000000000000000000000",
		Timestamp:         now,
		Height:            0,
		Nonce:             0,
		ChallengeSeed:     "genesis_challenge",
		ProofHash:         "genesis_proof",
		FarmerAddress:     "genesis_farmer",
	}
	
	body := BlockBody{
		Transactions: []SignedTransaction{*signedGenesisTx},
		TxCount:      1,
	}
	
	// Calculate merkle root
	header.MerkleRoot = calculateMerkleRoot(body.Transactions)
	
	genesis := &GenesisBlock{
		Block: Block{
			Header: header,
			Body:   body,
		},
		GenesisTimestamp: now,
		NetworkID:        "shadowy-mainnet",
		InitialSupply:    1 * SatoshisPerShadow, // 1 SHADOW bootstrap
	}
	
	return genesis, nil
}

// Hash calculates the hash of a block
func (b *Block) Hash() string {
	// Serialize header for hashing
	headerBytes := b.serializeHeader()
	hash := sha256.Sum256(headerBytes)
	return hex.EncodeToString(hash[:])
}

// serializeHeader serializes the block header for hashing
func (b *Block) serializeHeader() []byte {
	var buf []byte
	
	// Version (4 bytes)
	versionBytes := make([]byte, 4)
	binary.LittleEndian.PutUint32(versionBytes, b.Header.Version)
	buf = append(buf, versionBytes...)
	
	// Previous block hash (32 bytes)
	prevHashBytes, _ := hex.DecodeString(b.Header.PreviousBlockHash)
	if len(prevHashBytes) != 32 {
		prevHashBytes = make([]byte, 32) // Zero hash for genesis
	}
	buf = append(buf, prevHashBytes...)
	
	// Merkle root (32 bytes)
	merkleBytes, _ := hex.DecodeString(b.Header.MerkleRoot)
	if len(merkleBytes) != 32 {
		merkleBytes = make([]byte, 32)
	}
	buf = append(buf, merkleBytes...)
	
	// Timestamp (8 bytes)
	timestampBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(timestampBytes, uint64(b.Header.Timestamp.Unix()))
	buf = append(buf, timestampBytes...)
	
	// Height (8 bytes)
	heightBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(heightBytes, b.Header.Height)
	buf = append(buf, heightBytes...)
	
	// Nonce (8 bytes)
	nonceBytes := make([]byte, 8)
	binary.LittleEndian.PutUint64(nonceBytes, b.Header.Nonce)
	buf = append(buf, nonceBytes...)
	
	// Challenge seed and proof (for simplicity, just append as strings)
	buf = append(buf, []byte(b.Header.ChallengeSeed)...)
	buf = append(buf, []byte(b.Header.ProofHash)...)
	
	return buf
}

// calculateMerkleRoot calculates the merkle root of transactions
func calculateMerkleRoot(transactions []SignedTransaction) string {
	if len(transactions) == 0 {
		return "0000000000000000000000000000000000000000000000000000000000000000"
	}
	
	// Get transaction hashes
	var hashes []string
	for _, tx := range transactions {
		hashes = append(hashes, tx.TxHash)
	}
	
	// Build merkle tree
	for len(hashes) > 1 {
		var nextLevel []string
		
		// Process pairs
		for i := 0; i < len(hashes); i += 2 {
			var combined string
			if i+1 < len(hashes) {
				combined = hashes[i] + hashes[i+1]
			} else {
				combined = hashes[i] + hashes[i] // Duplicate odd hash
			}
			
			hash := sha256.Sum256([]byte(combined))
			nextLevel = append(nextLevel, hex.EncodeToString(hash[:]))
		}
		
		hashes = nextLevel
	}
	
	return hashes[0]
}

// AddBlock adds a new block to the blockchain
func (bc *Blockchain) AddBlock(block *Block) error {
	bc.mu.Lock()
	defer bc.mu.Unlock()
	
	// Validate block
	if err := bc.validateBlock(block); err != nil {
		return fmt.Errorf("invalid block: %w", err)
	}
	
	// Add to chain
	hash := block.Hash()
	bc.blocks[hash] = block
	bc.blocksByHeight[block.Header.Height] = block
	
	// Update tip if this is the new highest block
	if block.Header.Height > bc.tipHeight {
		bc.tipHash = hash
		bc.tipHeight = block.Header.Height
	}
	
	// Persist block
	if err := bc.saveBlock(block); err != nil {
		return fmt.Errorf("failed to save block: %w", err)
	}
	
	return nil
}

// validateBlock validates a block before adding to chain
func (bc *Blockchain) validateBlock(block *Block) error {
	// Check if previous block exists (except for genesis)
	if block.Header.Height > 0 {
		if _, exists := bc.blocks[block.Header.PreviousBlockHash]; !exists {
			return fmt.Errorf("previous block not found: %s", block.Header.PreviousBlockHash)
		}
		
		// Check height consistency
		prevBlock := bc.blocks[block.Header.PreviousBlockHash]
		if block.Header.Height != prevBlock.Header.Height + 1 {
			return fmt.Errorf("invalid height: expected %d, got %d", 
				prevBlock.Header.Height + 1, block.Header.Height)
		}
	}
	
	// Validate merkle root
	expectedMerkleRoot := calculateMerkleRoot(block.Body.Transactions)
	if block.Header.MerkleRoot != expectedMerkleRoot {
		return fmt.Errorf("invalid merkle root: expected %s, got %s", 
			expectedMerkleRoot, block.Header.MerkleRoot)
	}
	
	// Validate transaction count
	if uint32(len(block.Body.Transactions)) != block.Body.TxCount {
		return fmt.Errorf("transaction count mismatch: expected %d, got %d", 
			len(block.Body.Transactions), block.Body.TxCount)
	}
	
	// TODO: Add more validation (proof-of-storage validation, etc.)
	
	return nil
}

// GetBlock retrieves a block by hash
func (bc *Blockchain) GetBlock(hash string) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	block, exists := bc.blocks[hash]
	if !exists {
		return nil, fmt.Errorf("block not found: %s", hash)
	}
	
	return block, nil
}

// GetBlockByHeight retrieves a block by height
func (bc *Blockchain) GetBlockByHeight(height uint64) (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	block, exists := bc.blocksByHeight[height]
	if !exists {
		return nil, fmt.Errorf("block not found at height: %d", height)
	}
	
	return block, nil
}

// GetTip returns the current tip of the blockchain
func (bc *Blockchain) GetTip() (*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	if bc.tipHash == "" {
		return nil, fmt.Errorf("no tip block found")
	}
	
	return bc.blocks[bc.tipHash], nil
}

// GetStats returns blockchain statistics
func (bc *Blockchain) GetStats() BlockchainStats {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	stats := BlockchainStats{
		TipHeight:   bc.tipHeight,
		TipHash:     bc.tipHash,
		TotalBlocks: uint64(len(bc.blocks)),
	}
	
	// Get genesis hash
	if genesisBlock, exists := bc.blocksByHeight[0]; exists {
		stats.GenesisHash = genesisBlock.Hash()
	}
	
	// Get last block time
	if tipBlock, exists := bc.blocks[bc.tipHash]; exists {
		stats.LastBlockTime = tipBlock.Header.Timestamp
	}
	
	// Calculate transaction statistics
	var totalTxs uint64
	var totalSize uint64
	
	for _, block := range bc.blocks {
		totalTxs += uint64(block.Body.TxCount)
		// Approximate block size (JSON serialization)
		if data, err := json.Marshal(block); err == nil {
			totalSize += uint64(len(data))
		}
	}
	
	stats.TotalTransactions = totalTxs
	if len(bc.blocks) > 0 {
		stats.AvgBlockSize = totalSize / uint64(len(bc.blocks))
		stats.AvgTxPerBlock = float64(totalTxs) / float64(len(bc.blocks))
	}
	
	return stats
}

// GetRecentBlocks returns the most recent blocks
func (bc *Blockchain) GetRecentBlocks(count int) ([]*Block, error) {
	bc.mu.RLock()
	defer bc.mu.RUnlock()
	
	var blocks []*Block
	
	// Get heights in descending order
	var heights []uint64
	for height := range bc.blocksByHeight {
		heights = append(heights, height)
	}
	sort.Slice(heights, func(i, j int) bool {
		return heights[i] > heights[j]
	})
	
	// Get blocks up to count
	for i, height := range heights {
		if i >= count {
			break
		}
		if block, exists := bc.blocksByHeight[height]; exists {
			blocks = append(blocks, block)
		}
	}
	
	return blocks, nil
}

// Storage functions

func (bc *Blockchain) saveGenesisBlock(genesis *GenesisBlock) error {
	genesisPath := filepath.Join(bc.dataDir, "genesis.json")
	data, err := json.MarshalIndent(genesis, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal genesis block: %w", err)
	}
	
	return os.WriteFile(genesisPath, data, 0644)
}

func (bc *Blockchain) saveBlock(block *Block) error {
	hash := block.Hash()
	blockPath := filepath.Join(bc.dataDir, "blocks", hash+".json")
	
	// Ensure blocks directory exists
	if err := os.MkdirAll(filepath.Dir(blockPath), 0755); err != nil {
		return fmt.Errorf("failed to create blocks directory: %w", err)
	}
	
	data, err := json.MarshalIndent(block, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal block: %w", err)
	}
	
	return os.WriteFile(blockPath, data, 0644)
}

func (bc *Blockchain) loadBlockchain() error {
	// Load genesis block
	genesisPath := filepath.Join(bc.dataDir, "genesis.json")
	genesisData, err := os.ReadFile(genesisPath)
	if err != nil {
		return fmt.Errorf("failed to read genesis block: %w", err)
	}
	
	var genesis GenesisBlock
	if err := json.Unmarshal(genesisData, &genesis); err != nil {
		return fmt.Errorf("failed to parse genesis block: %w", err)
	}
	
	// Add genesis to chain
	genesisHash := genesis.Hash()
	bc.blocks[genesisHash] = &genesis.Block
	bc.blocksByHeight[0] = &genesis.Block
	bc.tipHash = genesisHash
	bc.tipHeight = 0
	
	// Load all other blocks
	blocksDir := filepath.Join(bc.dataDir, "blocks")
	if _, err := os.Stat(blocksDir); !os.IsNotExist(err) {
		entries, err := os.ReadDir(blocksDir)
		if err != nil {
			return fmt.Errorf("failed to read blocks directory: %w", err)
		}
		
		for _, entry := range entries {
			if !entry.IsDir() && filepath.Ext(entry.Name()) == ".json" {
				blockPath := filepath.Join(blocksDir, entry.Name())
				blockData, err := os.ReadFile(blockPath)
				if err != nil {
					fmt.Printf("Warning: failed to read block file %s: %v\n", entry.Name(), err)
					continue
				}
				
				var block Block
				if err := json.Unmarshal(blockData, &block); err != nil {
					fmt.Printf("Warning: failed to parse block file %s: %v\n", entry.Name(), err)
					continue
				}
				
				hash := block.Hash()
				bc.blocks[hash] = &block
				bc.blocksByHeight[block.Header.Height] = &block
				
				// Update tip if this is higher
				if block.Header.Height > bc.tipHeight {
					bc.tipHash = hash
					bc.tipHeight = block.Header.Height
				}
			}
		}
	}
	
	return nil
}