package cmd

import (
    "crypto/sha256"
    "encoding/binary"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "os"
    "path/filepath"
    "sort"
    "sync"
    "time"
)

// Global variables for blockchain bootstrap configuration
var (
    // AllowFork when true, allows creating new testnet genesis blocks instead of bootstrapping
    AllowFork = false
)

// Block represents a single block in the blockchain
type Block struct {
    Header BlockHeader `json:"header"`
    Body   BlockBody   `json:"body"`
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
    ChallengeSeed string `json:"challenge_seed"`
    ProofHash     string `json:"proof_hash"`
    FarmerAddress string `json:"farmer_address"`
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
// BlockBroadcaster interface for broadcasting new blocks
type BlockBroadcaster interface {
    BroadcastBlock(block *Block)
}

type Blockchain struct {
    config *ShadowConfig

    // Chain state
    blocks         map[string]*Block // hash -> block
    blocksByHeight map[uint64]*Block // height -> block
    tipHash        string            // hash of the latest block
    tipHeight      uint64            // height of the latest block

    // Token system
    tokenState    *TokenState
    tokenExecutor *TokenExecutor

    // Syndicate system
    syndicateManager *SyndicateManager

    // Synchronization
    mu sync.RWMutex

    // Sync state tracking (for detecting stuck sync)
    lastHeightChangeTime time.Time
    stuckSyncAttempts    int
    maxStuckAttempts     int

    // Storage
    dataDir string

    // Network broadcasting
    broadcaster BlockBroadcaster
}

// BlockchainStats contains blockchain statistics
type BlockchainStats struct {
    TipHeight     uint64    `json:"tip_height"`
    TipHash       string    `json:"tip_hash"`
    TotalBlocks   uint64    `json:"total_blocks"`
    GenesisHash   string    `json:"genesis_hash"`
    LastBlockTime time.Time `json:"last_block_time"`

    // Transaction stats
    TotalTransactions uint64  `json:"total_transactions"`
    AvgBlockSize      uint64  `json:"avg_block_size"`
    AvgTxPerBlock     float64 `json:"avg_tx_per_block"`
}

// NewBlockchain creates a new blockchain instance
func NewBlockchain(config *ShadowConfig) (*Blockchain, error) {
    bc := &Blockchain{
        config:         config,
        blocks:         make(map[string]*Block),
        blocksByHeight: make(map[uint64]*Block),
        dataDir:        config.BlockchainDirectory,
        lastHeightChangeTime: time.Now(),
        stuckSyncAttempts:    0,
        maxStuckAttempts:     3, // Reset after 3 failed attempts
    }

    // Ensure blockchain directory exists
    if err := os.MkdirAll(bc.dataDir, 0755); err != nil {
        return nil, fmt.Errorf("failed to create blockchain directory: %w", err)
    }

    // Initialize token system
    tokenDataDir := filepath.Join(bc.dataDir, "tokens")
    tokenState, err := NewTokenState(tokenDataDir)
    if err != nil {
        return nil, fmt.Errorf("failed to initialize token state: %w", err)
    }
    bc.tokenState = tokenState

    // Initialize syndicate system
    bc.syndicateManager = NewSyndicateManager()
    log.Printf("🐉 [BLOCKCHAIN] Syndicate system initialized")

    // Initialize token executor with syndicate manager
    bc.tokenExecutor = NewTokenExecutor(tokenState, bc.syndicateManager)

    // Load existing blockchain or create genesis
    if err := bc.initialize(); err != nil {
        return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
    }

    return bc, nil
}

// SetBroadcaster sets the block broadcaster for network propagation
func (bc *Blockchain) SetBroadcaster(broadcaster BlockBroadcaster) {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    bc.broadcaster = broadcaster
}


// initialize loads the blockchain from disk or creates genesis block
func (bc *Blockchain) initialize() error {
    // Check if genesis block exists
    genesisPath := filepath.Join(bc.dataDir, "genesis.json")
    if _, err := os.Stat(genesisPath); os.IsNotExist(err) {
        var genesis *GenesisBlock
        var err error

        if AllowFork {
            // Create new testnet genesis block
            fmt.Printf("🔱 Creating new testnet genesis block (--fork mode)\n")
            genesis, err = bc.createGenesisBlock()
            if err != nil {
                return fmt.Errorf("failed to create genesis block: %w", err)
            }
        } else {
            // No genesis found and fork not allowed
            fmt.Printf("❌ No local genesis.json found in %s/genesis.json\n", bc.dataDir)
            fmt.Printf("💡 Options:\n")
            fmt.Printf("   1. Use --fork flag to create a new testnet\n")
            fmt.Printf("   2. Use ./shadowy-tendermint tendermint bootstrap to setup from bootstrap package\n")
            fmt.Printf("   3. Copy genesis.json from an existing node to %s/genesis.json\n", bc.dataDir)
            return fmt.Errorf("no genesis block found and --fork not enabled")
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
        Version: 1,
        Inputs:  []TransactionInput{},
        Outputs: []TransactionOutput{
            {
                Value:   1 * SatoshisPerShadow,                                 // 1 SHADOW bootstrap (100,000,000 satoshis)
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

    startTime := time.Now()
    hash := block.Hash()

    log.Printf("⛓️  [BLOCKCHAIN] Adding block to chain...")
    log.Printf("📋 [BLOCKCHAIN] Block details:")
    log.Printf("   🏷️  Hash: %s", hash)
    log.Printf("   📏 Height: %d", block.Header.Height)
    log.Printf("   🔗 Previous: %s", block.Header.PreviousBlockHash)
    log.Printf("   📦 Transactions: %d", len(block.Body.Transactions))
    farmerAddr := block.Header.FarmerAddress
    if len(farmerAddr) > 16 {
        farmerAddr = farmerAddr[:16] + "..."
    }
    log.Printf("   👨‍🌾 Farmer: %s", farmerAddr)
    log.Printf("   🕐 Timestamp: %s", block.Header.Timestamp.Format("15:04:05"))

    // Validate block
    log.Printf("🔍 [BLOCKCHAIN] Validating block...")
    validationStart := time.Now()
    if err := bc.validateBlock(block); err != nil {
        log.Printf("❌ [BLOCKCHAIN] Block validation FAILED: %v", err)
        return fmt.Errorf("invalid block: %w", err)
    }
    validationDuration := time.Since(validationStart)
    log.Printf("✅ [BLOCKCHAIN] Block validation PASSED in %v", validationDuration)

    // Execute token operations in all transactions
    log.Printf("🪙 [BLOCKCHAIN] Processing token operations...")
    tokenExecStart := time.Now()
    totalShadowLocked := uint64(0)
    totalShadowReleased := uint64(0)
    tokenOpsProcessed := 0

    log.Printf("🔍 [BLOCKCHAIN] Starting validation of %d transactions", len(block.Body.Transactions))
    for i, signedTx := range block.Body.Transactions {
        log.Printf("🔍 [BLOCKCHAIN] Processing transaction %d/%d", i+1, len(block.Body.Transactions))

        // Parse the transaction
        var tx Transaction
        if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
            log.Printf("❌ [BLOCKCHAIN] Failed to parse transaction %d: %v", i, err)
            return fmt.Errorf("failed to parse transaction %d: %w", i, err)
        }
        
        // Debug: Log raw transaction JSON to see if L-address is preserved
        if len(tx.Outputs) > 0 && len(tx.Outputs[0].Address) == 41 && tx.Outputs[0].Address[0] == 'L' {
            log.Printf("🔍 [BLOCKCHAIN] Raw transaction JSON: %s", string(signedTx.Transaction))
        }

        log.Printf("🔍 [BLOCKCHAIN] Transaction %d has %d token operations", i, len(tx.TokenOps))

        // Execute token operations if any exist
        if len(tx.TokenOps) > 0 {
            log.Printf("🔍 [BLOCKCHAIN] Executing token operations for transaction %d", i)
            for j, op := range tx.TokenOps {
                log.Printf("🔍 [BLOCKCHAIN] Token op %d: type=%d, tokenID=%s", j, op.Type, op.TokenID)
            }

            result, err := bc.tokenExecutor.ExecuteTokenOperations(&tx)
            if err != nil {
                log.Printf("❌ [BLOCKCHAIN] Token execution failed for transaction %d: %v", i, err)
                return fmt.Errorf("token execution failed for transaction %d: %w", i, err)
            }
            log.Printf("✅ [BLOCKCHAIN] Token execution succeeded for transaction %d", i)

            totalShadowLocked += result.ShadowLocked
            totalShadowReleased += result.ShadowReleased
            tokenOpsProcessed += len(result.Operations)

            log.Printf("🪙 [BLOCKCHAIN] Transaction %d: %d token operations executed", i, len(result.Operations))
        }
        
        // Check for SHADOW transfers to L-addresses (liquidity provision)
        log.Printf("🔍 [BLOCKCHAIN] Checking transaction %d outputs for L-address SHADOW transfers", i)
        for j, output := range tx.Outputs {
            log.Printf("🔍 [BLOCKCHAIN] Output %d: Address=%s, Value=%d", j, output.Address, output.Value)
            if len(output.Address) == 41 && output.Address[0] == 'L' {
                log.Printf("🏊 [BLOCKCHAIN] Found SHADOW transfer to L-address %s, value: %d", output.Address, output.Value)
                // Handle SHADOW liquidity provision  
                err := bc.handleShadowLiquidityProvision(&signedTx, &tx, output.Address, output.Value, i)
                if err != nil {
                    log.Printf("❌ [BLOCKCHAIN] SHADOW liquidity provision failed: %v", err)
                    // Don't fail the block, just log the error
                } else {
                    log.Printf("✅ [BLOCKCHAIN] SHADOW liquidity provision completed for transaction %d", i)
                }
            }
        }
        
        log.Printf("🔍 [BLOCKCHAIN] Finished processing transaction %d", i)
    }

    tokenExecDuration := time.Since(tokenExecStart)
    if tokenOpsProcessed > 0 {
        log.Printf("✅ [BLOCKCHAIN] Token processing COMPLETED in %v", tokenExecDuration)
        log.Printf("   🪙 Operations: %d", tokenOpsProcessed)
        log.Printf("   🔒 Shadow locked: %d", totalShadowLocked)
        log.Printf("   🔓 Shadow released: %d", totalShadowReleased)
    } else {
        log.Printf("⚪ [BLOCKCHAIN] No token operations to process")
    }

    // Track syndicate performance for this block
    if bc.syndicateManager != nil {
        log.Printf("🐉 [BLOCKCHAIN] Tracking syndicate performance for block...")
        farmerAddress := block.Header.FarmerAddress

        // Determine which syndicate won this block (if any)
        winnerSyndicate := bc.determineSyndicateWinner(farmerAddress)

        // Update syndicate manager with block win information
        bc.syndicateManager.UpdateBlockWin(block.Header.Height, winnerSyndicate, farmerAddress)

        if winnerSyndicate >= SyndicateSeiryu && winnerSyndicate <= SyndicateGenbu {
            log.Printf("🐉 [BLOCKCHAIN] Block won by %s (farmer: %s)", winnerSyndicate.Description(), farmerAddress)
        } else {
            log.Printf("🐉 [BLOCKCHAIN] Block won by solo miner (farmer: %s)", farmerAddress)
        }
    }

    // Check if this is a new tip
    isNewTip := block.Header.Height > bc.tipHeight
    prevTipHeight := bc.tipHeight
    prevTipHash := bc.tipHash

    // Add to chain
    log.Printf("💾 [BLOCKCHAIN] Storing block in memory...")
    bc.blocks[hash] = block
    bc.blocksByHeight[block.Header.Height] = block

    // Update tip if this is the new highest block
    if isNewTip {
        bc.tipHash = hash
        bc.tipHeight = block.Header.Height
        log.Printf("🎯 [BLOCKCHAIN] New blockchain tip!")
        log.Printf("   📏 Height: %d -> %d (+%d)", prevTipHeight, bc.tipHeight, bc.tipHeight-prevTipHeight)
        log.Printf("   🔗 Tip Hash: %s -> %s", prevTipHash[:16]+"...", bc.tipHash[:16]+"...")
    } else {
        log.Printf("🔀 [BLOCKCHAIN] Block added to side chain (height %d, current tip: %d)",
            block.Header.Height, bc.tipHeight)
    }

    // Persist block
    log.Printf("💿 [BLOCKCHAIN] Persisting block to disk...")
    persistStart := time.Now()
    if err := bc.saveBlock(block); err != nil {
        log.Printf("❌ [BLOCKCHAIN] Block persistence FAILED: %v", err)
        return fmt.Errorf("failed to save block: %w", err)
    }
    persistDuration := time.Since(persistStart)
    log.Printf("✅ [BLOCKCHAIN] Block persisted to disk in %v", persistDuration)

    // Calculate blockchain statistics
    totalBlocks := len(bc.blocks)
    totalDuration := time.Since(startTime)

    log.Printf("📊 [BLOCKCHAIN] Blockchain updated successfully!")
    log.Printf("   ⚡ Total time: %v", totalDuration)
    log.Printf("   📚 Total blocks: %d", totalBlocks)
    log.Printf("   📏 Chain height: %d", bc.tipHeight)
    log.Printf("   🏷️  Chain tip: %s", bc.tipHash[:32]+"...")

    // Broadcast block to consensus peers if we have a broadcaster
    if bc.broadcaster != nil && isNewTip {
        log.Printf("📡 [BLOCKCHAIN] Broadcasting new block to network peers...")
        bc.broadcaster.BroadcastBlock(block)
    }

    return nil
}

// handleShadowLiquidityProvision processes SHADOW transfers to L-addresses
func (bc *Blockchain) handleShadowLiquidityProvision(signedTx *SignedTransaction, tx *Transaction, lAddress string, shadowAmount uint64, txIndex int) error {
    log.Printf("🏊 [BLOCKCHAIN] Processing SHADOW liquidity provision to %s", lAddress)
    
    // Find the sender address by looking at transaction outputs for change address pattern
    // Simple heuristic: if there are multiple outputs, the non-L-address output is likely change to sender
    var senderAddress string
    
    log.Printf("🔍 [BLOCKCHAIN] Analyzing %d outputs to find sender", len(tx.Outputs))
    for i, output := range tx.Outputs {
        log.Printf("🔍 [BLOCKCHAIN] Output %d: Address=%s, Value=%d", i, output.Address, output.Value)
        
        // If this output is NOT the L-address, it's likely change back to sender
        if output.Address != lAddress && len(output.Address) == 51 && output.Address[0] == 'S' {
            senderAddress = output.Address
            log.Printf("✅ [BLOCKCHAIN] Identified sender as: %s (change output)", senderAddress)
            break
        }
    }
    
    // If no change output found, try to extract sender from signature
    if senderAddress == "" {
        log.Printf("🔍 [BLOCKCHAIN] No change output found, attempting to extract sender from signature")
        
        // Try to derive sender address from the transaction signature
        extractedSender, err := bc.extractSenderFromSignature(signedTx)
        if err != nil {
            log.Printf("❌ [BLOCKCHAIN] Failed to extract sender from signature: %v", err)
            log.Printf("⚠️ [BLOCKCHAIN] No sender identified - liquidity provision skipped")
            return nil
        }
        
        senderAddress = extractedSender
        log.Printf("✅ [BLOCKCHAIN] Extracted sender from signature: %s", senderAddress)
    }
    
    // Use the token executor to handle liquidity provision with "SHADOW" as tokenID
    if bc.tokenExecutor != nil {
        log.Printf("🏊 [BLOCKCHAIN] Calling liquidity provision handler: provider=%s, lAddress=%s, amount=%d", 
            senderAddress, lAddress, shadowAmount)
        err := bc.tokenExecutor.handleLiquidityProvision(lAddress, senderAddress, "SHADOW", shadowAmount)
        if err != nil {
            log.Printf("❌ [BLOCKCHAIN] SHADOW liquidity provision failed: %v", err)
            return fmt.Errorf("failed to handle SHADOW liquidity provision: %w", err)
        }
        log.Printf("✅ [BLOCKCHAIN] SHADOW liquidity provision completed successfully")
    } else {
        log.Printf("⚠️ [BLOCKCHAIN] No token executor available for liquidity provision")
    }
    
    log.Printf("🔍 [BLOCKCHAIN] handleShadowLiquidityProvision returning successfully")
    return nil
}

// extractSenderFromSignature attempts to derive the sender address from transaction signature
func (bc *Blockchain) extractSenderFromSignature(signedTx *SignedTransaction) (string, error) {
    log.Printf("🔍 [BLOCKCHAIN] Attempting to extract sender from transaction signature")
    
    // Check if we have signer key information
    if signedTx.SignerKey == "" {
        return "", fmt.Errorf("no signer key available in transaction")
    }
    
    log.Printf("🔍 [BLOCKCHAIN] Found signer key: %s", signedTx.SignerKey)
    
    // Derive address from public key
    senderAddress, err := bc.deriveAddressFromPublicKey(signedTx.SignerKey)
    if err != nil {
        return "", fmt.Errorf("failed to derive address from public key: %w", err)
    }
    
    log.Printf("✅ [BLOCKCHAIN] Derived sender address from signature: %s", senderAddress)
    return senderAddress, nil
}

// deriveAddressFromPublicKey converts a public key to a Shadowy address
func (bc *Blockchain) deriveAddressFromPublicKey(publicKeyHex string) (string, error) {
    log.Printf("🔍 [BLOCKCHAIN] Deriving address from public key: %s", publicKeyHex)
    
    // Decode the hex public key
    pubKeyBytes, err := hex.DecodeString(publicKeyHex)
    if err != nil {
        return "", fmt.Errorf("failed to decode public key hex: %w", err)
    }
    
    // Use the same logic as DeriveAddress() in wallet.go
    // Call the existing function directly
    address := DeriveAddress(pubKeyBytes)
    
    log.Printf("✅ [BLOCKCHAIN] Derived address: %s", address)
    return address, nil
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
        if block.Header.Height != prevBlock.Header.Height+1 {
            return fmt.Errorf("invalid height: expected %d, got %d",
                prevBlock.Header.Height+1, block.Header.Height)
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

    // Validate token operations in all transactions
    log.Printf("🔍 [BLOCKCHAIN] Starting token operation validation for %d transactions", len(block.Body.Transactions))
    for i, signedTx := range block.Body.Transactions {
        log.Printf("🔍 [BLOCKCHAIN] Validating token operations in transaction %d", i)

        // Parse the transaction
        var tx Transaction
        if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
            log.Printf("❌ [BLOCKCHAIN] Failed to parse transaction %d for validation: %v", i, err)
            return fmt.Errorf("failed to parse transaction %d: %w", i, err)
        }

        log.Printf("🔍 [BLOCKCHAIN] Transaction %d has %d token operations", i, len(tx.TokenOps))

        // Validate basic token operation structure
        if err := tx.ValidateTokenOperations(); err != nil {
            log.Printf("❌ [BLOCKCHAIN] Transaction %d has invalid token operation structure: %v", i, err)
            return fmt.Errorf("transaction %d has invalid token operations: %w", i, err)
        }

        // Validate token operations can be executed (check state consistency)
        if len(tx.TokenOps) > 0 {
            log.Printf("🔍 [BLOCKCHAIN] Validating token operation execution for transaction %d", i)
            if err := bc.tokenExecutor.ValidateTokenOperationExecution(&tx); err != nil {
                log.Printf("❌ [BLOCKCHAIN] Transaction %d token operations cannot be executed: %v", i, err)
                return fmt.Errorf("transaction %d token operations cannot be executed: %w", i, err)
            }
            log.Printf("✅ [BLOCKCHAIN] Token operation validation passed for transaction %d", i)
        }
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

    // Validate chain integrity and trim invalid blocks
    if err := bc.validateAndTrimChain(); err != nil {
        return fmt.Errorf("failed to validate blockchain: %w", err)
    }

    return nil
}

// validateAndTrimChain validates the blockchain integrity and trims invalid blocks
func (bc *Blockchain) validateAndTrimChain() error {
    log.Printf("🔍 [BLOCKCHAIN] Validating chain integrity...")

    if bc.tipHeight == 0 {
        log.Printf("✅ [BLOCKCHAIN] Only genesis block present, validation complete")
        return nil
    }

    var lastValidHeight uint64 = 0
    var lastValidHash string = ""

    // Get genesis block as starting point
    genesisBlock, exists := bc.blocksByHeight[0]
    if !exists {
        return fmt.Errorf("genesis block not found")
    }
    lastValidHash = genesisBlock.Hash()
    lastValidHeight = 0

    log.Printf("📊 [BLOCKCHAIN] Validating chain from height 0 to %d", bc.tipHeight)

    // Walk through each block sequentially
    for height := uint64(1); height <= bc.tipHeight; height++ {
        currentBlock, exists := bc.blocksByHeight[height]
        if !exists {
            log.Printf("⚠️  [BLOCKCHAIN] Block at height %d missing, trimming chain", height)
            break
        }

        currentHash := currentBlock.Hash()
        expectedParent := lastValidHash
        actualParent := currentBlock.Header.PreviousBlockHash

        if actualParent != expectedParent {
            log.Printf("❌ [BLOCKCHAIN] Invalid parent-child relationship at height %d", height)
            log.Printf("   Block hash: %s", currentHash[:16]+"...")
            log.Printf("   Expected parent: %s", expectedParent[:16]+"...")
            log.Printf("   Actual parent:   %s", actualParent[:16]+"...")
            log.Printf("✂️  [BLOCKCHAIN] Trimming invalid blocks from height %d onwards", height)
            break
        }

        // Block is valid
        lastValidHeight = height
        lastValidHash = currentHash
        // too noisy
        //log.Printf("✅ [BLOCKCHAIN] Block %d valid (parent: %s)", height, expectedParent[:16]+"...")
    }

    // If we found invalid blocks, trim them
    if lastValidHeight < bc.tipHeight {
        log.Printf("🔧 [BLOCKCHAIN] Trimming blockchain from height %d to %d", lastValidHeight+1, bc.tipHeight)

        if err := bc.TrimBlocksFromHeight(lastValidHeight + 1); err != nil {
            return fmt.Errorf("failed to trim invalid blocks: %w", err)
        }

        log.Printf("✅ [BLOCKCHAIN] Chain validation complete, trimmed to height %d", lastValidHeight)
        
        // Check if we should trigger nuclear reset due to stuck sync
        if shouldReset := bc.RecordSyncAttempt(uint64(lastValidHeight)); shouldReset {
            log.Printf("🙀 [BLOCKCHAIN] Triggering nuclear reset due to repeated sync failures!")
            if err := bc.NuclearReset(); err != nil {
                log.Printf("❌ [BLOCKCHAIN] Nuclear reset failed: %v", err)
                return fmt.Errorf("nuclear reset failed: %w", err)
            }
        }
    } else {
        log.Printf("✅ [BLOCKCHAIN] Chain validation complete, all %d blocks valid", bc.tipHeight+1)
        // Record successful sync progress
        bc.RecordSyncAttempt(bc.tipHeight)
    }

    return nil
}

// DeleteBlock removes a block from both memory and disk storage
func (bc *Blockchain) DeleteBlock(block *Block) error {
    bc.mu.Lock()
    defer bc.mu.Unlock()

    hash := block.Hash()
    height := block.Header.Height

    // Remove from memory
    delete(bc.blocks, hash)
    delete(bc.blocksByHeight, height)

    // Remove from disk
    blockPath := filepath.Join(bc.dataDir, "blocks", hash+".json")
    if err := os.Remove(blockPath); err != nil && !os.IsNotExist(err) {
        return fmt.Errorf("failed to delete block file %s: %w", blockPath, err)
    }

    log.Printf("🗑️ [BLOCKCHAIN] Deleted block at height %d, hash %s", height, hash[:16]+"...")
    return nil
}

// TrimBlocksFromHeight removes all blocks from the specified height onwards
func (bc *Blockchain) TrimBlocksFromHeight(fromHeight uint64) error {
    bc.mu.Lock()
    defer bc.mu.Unlock()

    log.Printf("✂️ [BLOCKCHAIN] Trimming blocks from height %d onwards", fromHeight)

    var blocksToDelete []*Block

    // Find all blocks to delete
    for height := fromHeight; height <= bc.tipHeight; height++ {
        if block, exists := bc.blocksByHeight[height]; exists {
            blocksToDelete = append(blocksToDelete, block)
        }
    }

    // Delete each block
    for _, block := range blocksToDelete {
        hash := block.Hash()
        height := block.Header.Height

        // Remove from memory
        delete(bc.blocks, hash)
        delete(bc.blocksByHeight, height)

        // Remove from disk
        blockPath := filepath.Join(bc.dataDir, "blocks", hash+".json")
        if err := os.Remove(blockPath); err != nil && !os.IsNotExist(err) {
            log.Printf("Warning: failed to delete block file %s: %v", blockPath, err)
        }
    }

    // Update tip to the block before the trimmed range
    if fromHeight > 0 {
        newTipHeight := fromHeight - 1
        if newTipBlock, exists := bc.blocksByHeight[newTipHeight]; exists {
            bc.tipHeight = newTipHeight
            bc.tipHash = newTipBlock.Hash()
            log.Printf("📍 [BLOCKCHAIN] Updated tip to height %d, hash %s", bc.tipHeight, bc.tipHash[:16]+"...")
        }
    } else {
        // Trimming from genesis - this shouldn't happen in normal operation
        bc.tipHeight = 0
        bc.tipHash = ""
        log.Printf("⚠️ [BLOCKCHAIN] Warning: trimmed to genesis")
    }

    log.Printf("✂️ [BLOCKCHAIN] Trimmed %d blocks, new tip height: %d", len(blocksToDelete), bc.tipHeight)

    // Reset token state if we trimmed back to early blocks
    // Any trim operation could affect token state, so reset to be safe
    if bc.tokenState != nil {
        log.Printf("🔄 [BLOCKCHAIN] Resetting token state after block trimming...")
        if err := bc.tokenState.ResetToGenesis(); err != nil {
            log.Printf("⚠️ [BLOCKCHAIN] Failed to reset token state: %v", err)
            // Don't fail the trim operation, but log the warning
        }

        // Rebuild token state from remaining blocks
        log.Printf("🔄 [BLOCKCHAIN] Rebuilding token state from remaining blocks...")
        if err := bc.rebuildTokenState(); err != nil {
            log.Printf("⚠️ [BLOCKCHAIN] Failed to rebuild token state: %v", err)
        }
    }

    return nil
}

// rebuildTokenState rebuilds the token state by replaying all blocks from genesis
func (bc *Blockchain) rebuildTokenState() error {
    if bc.tokenState == nil || bc.tokenExecutor == nil {
        return fmt.Errorf("token state or executor not initialized")
    }

    log.Printf("🏗️ [BLOCKCHAIN] Rebuilding token state from %d blocks...", bc.tipHeight+1)

    // Process blocks in order from height 1 to current tip
    for height := uint64(1); height <= bc.tipHeight; height++ {
        block, exists := bc.blocksByHeight[height]
        if !exists {
            log.Printf("⚠️ [BLOCKCHAIN] Missing block at height %d during token state rebuild", height)
            continue
        }

        log.Printf("🔄 [BLOCKCHAIN] Replaying token operations from block %d", height)

        // Process each transaction in the block
        for txIndex, signedTx := range block.Body.Transactions {
            // Parse the transaction
            var tx Transaction
            if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
                log.Printf("⚠️ [BLOCKCHAIN] Failed to parse transaction %d in block %d: %v", txIndex, height, err)
                continue
            }

            // Execute token operations if any
            if len(tx.TokenOps) > 0 {
                log.Printf("🪙 [BLOCKCHAIN] Executing %d token operations from block %d, tx %d", len(tx.TokenOps), height, txIndex)
                result, err := bc.tokenExecutor.ExecuteTokenOperations(&tx)
                if err != nil {
                    log.Printf("⚠️ [BLOCKCHAIN] Failed to execute token operations in block %d, tx %d: %v", height, txIndex, err)
                    // Continue with other transactions even if one fails
                    continue
                }

                if !result.Success {
                    log.Printf("⚠️ [BLOCKCHAIN] Token operations failed in block %d, tx %d: %s", height, txIndex, result.Error)
                }
            }
        }
    }

    log.Printf("✅ [BLOCKCHAIN] Token state rebuild complete")
    return nil
}

// ReorganizeChain performs a chain reorganization to a longer chain
func (bc *Blockchain) ReorganizeChain(newBlocks []*Block, newTipHeight uint64) error {
    bc.mu.Lock()
    defer bc.mu.Unlock()

    if len(newBlocks) == 0 {
        return fmt.Errorf("no blocks provided for reorganization")
    }

    log.Printf("🔄 [BLOCKCHAIN] Starting chain reorganization...")
    log.Printf("   📊 Current tip: height %d, hash %s", bc.tipHeight, bc.tipHash[:16]+"...")
    log.Printf("   📈 New chain: %d blocks, target height %d", len(newBlocks), newTipHeight)

    // Find common ancestor
    commonAncestor, err := bc.findCommonAncestor(newBlocks)
    if err != nil {
        return fmt.Errorf("failed to find common ancestor: %w", err)
    }

    if commonAncestor == nil {
        return fmt.Errorf("no common ancestor found - chains are incompatible")
    }

    log.Printf("   🔗 Common ancestor: height %d, hash %s",
        commonAncestor.Header.Height, commonAncestor.Hash()[:16]+"...")

    // Roll back to common ancestor
    rollbackHeight := commonAncestor.Header.Height
    blocksToRemove := bc.tipHeight - rollbackHeight

    if blocksToRemove > 0 {
        log.Printf("⏪ [BLOCKCHAIN] Rolling back %d blocks to common ancestor", blocksToRemove)
        if err := bc.rollbackToHeight(rollbackHeight); err != nil {
            return fmt.Errorf("failed to rollback to common ancestor: %w", err)
        }
    }

    // Add new blocks from common ancestor forward
    log.Printf("⏭️  [BLOCKCHAIN] Adding %d new blocks from reorganized chain", len(newBlocks))
    successfullyAdded := 0

    for i, block := range newBlocks {
        // Skip blocks we already have (up to common ancestor)
        if block.Header.Height <= rollbackHeight {
            log.Printf("   ↩️  Skipping block %d (already have up to height %d)",
                block.Header.Height, rollbackHeight)
            continue
        }

        log.Printf("   ➕ Adding block %d/%d: height %d",
            i+1, len(newBlocks), block.Header.Height)

        // Add block without broadcasting (we're syncing, not creating new blocks)
        if err := bc.addBlockWithoutBroadcast(block); err != nil {
            log.Printf("❌ [BLOCKCHAIN] Failed to add block %d during reorganization: %v",
                block.Header.Height, err)
            return fmt.Errorf("failed to add block %d during reorganization: %w",
                block.Header.Height, err)
        }
        successfullyAdded++
    }

    log.Printf("✅ [BLOCKCHAIN] Chain reorganization complete!")
    log.Printf("   📊 Successfully added %d blocks", successfullyAdded)
    log.Printf("   📏 New tip: height %d, hash %s", bc.tipHeight, bc.tipHash[:16]+"...")
    log.Printf("   🔄 Reorganization gained %d blocks", bc.tipHeight-(rollbackHeight+blocksToRemove))

    return nil
}

// findCommonAncestor finds the common ancestor between current chain and new blocks
func (bc *Blockchain) findCommonAncestor(newBlocks []*Block) (*Block, error) {
    // Sort new blocks by height for easier processing
    blocksByHeight := make(map[uint64]*Block)
    for _, block := range newBlocks {
        blocksByHeight[block.Header.Height] = block
    }

    // Start from our current tip and work backwards
    currentHeight := bc.tipHeight

    for currentHeight > 0 {
        // Check if we have this height in the current chain
        currentBlock, exists := bc.blocksByHeight[currentHeight]
        if !exists {
            currentHeight--
            continue
        }

        // Check if the new chain has a block at this height
        newBlock, exists := blocksByHeight[currentHeight]
        if !exists {
            currentHeight--
            continue
        }

        // If hashes match, this is our common ancestor
        if currentBlock.Hash() == newBlock.Hash() {
            log.Printf("🔍 [BLOCKCHAIN] Found common ancestor at height %d", currentHeight)
            return currentBlock, nil
        }

        currentHeight--
    }

    // Check genesis block
    if genesisBlock, exists := bc.blocksByHeight[0]; exists {
        if newGenesisBlock, exists := blocksByHeight[0]; exists {
            if genesisBlock.Hash() == newGenesisBlock.Hash() {
                log.Printf("🔍 [BLOCKCHAIN] Common ancestor is genesis block")
                return genesisBlock, nil
            }
        }
    }

    return nil, fmt.Errorf("no common ancestor found")
}

// rollbackToHeight rolls back the blockchain to a specific height
func (bc *Blockchain) rollbackToHeight(targetHeight uint64) error {
    if targetHeight >= bc.tipHeight {
        return nil // Nothing to rollback
    }

    log.Printf("⏪ [BLOCKCHAIN] Rolling back from height %d to %d", bc.tipHeight, targetHeight)

    // Remove blocks and their files
    blocksRemoved := 0
    for height := bc.tipHeight; height > targetHeight; height-- {
        // Remove from memory
        if block, exists := bc.blocksByHeight[height]; exists {
            hash := block.Hash()
            delete(bc.blocks, hash)
            delete(bc.blocksByHeight, height)

            // Remove block file
            blockPath := filepath.Join(bc.dataDir, "blocks", hash+".json")
            if err := os.Remove(blockPath); err != nil {
                log.Printf("Warning: failed to remove block file %s: %v", blockPath, err)
            }

            blocksRemoved++
        }
    }

    // Update tip to target height
    if newTipBlock, exists := bc.blocksByHeight[targetHeight]; exists {
        bc.tipHash = newTipBlock.Hash()
        bc.tipHeight = targetHeight
        log.Printf("✅ [BLOCKCHAIN] Rolled back %d blocks, new tip: height %d",
            blocksRemoved, bc.tipHeight)
    } else {
        return fmt.Errorf("failed to find block at target height %d after rollback", targetHeight)
    }

    return nil
}

// addBlockWithoutBroadcast adds a block without broadcasting to peers (for sync)
func (bc *Blockchain) addBlockWithoutBroadcast(block *Block) error {
    hash := block.Hash()

    // Validate block
    if err := bc.validateBlock(block); err != nil {
        return fmt.Errorf("invalid block: %w", err)
    }

    // Add to chain
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

// GetTokenState returns the token state manager
func (bc *Blockchain) GetTokenState() *TokenState {
    return bc.tokenState
}

// GetTokenExecutor returns the token executor
func (bc *Blockchain) GetTokenExecutor() *TokenExecutor {
    return bc.tokenExecutor
}

// GetSyndicateManager returns the syndicate manager
func (bc *Blockchain) GetSyndicateManager() *SyndicateManager {
    return bc.syndicateManager
}

// determineSyndicateWinner determines which syndicate a farmer belongs to based on their active NFTs
func (bc *Blockchain) determineSyndicateWinner(farmerAddress string) SyndicateType {
    // Default to -1 (solo miner) if no syndicate membership found
    const soloMiner SyndicateType = -1

    // Get all token balances for the farmer
    balances, err := bc.tokenState.GetAllTokenBalances(farmerAddress)
    if err != nil {
        log.Printf("⚠️ [BLOCKCHAIN] Failed to get farmer token balances: %v", err)
        return soloMiner
    }

    // Look for active syndicate membership NFTs
    for _, balance := range balances {
        if balance.Balance > 0 && balance.TokenInfo != nil && balance.TokenInfo.Syndicate != nil {
            syndicateData := balance.TokenInfo.Syndicate

            // Check if this syndicate NFT is still active (not expired)
            currentTime := time.Now().Unix()
            if syndicateData.ExpirationTime > currentTime {
                log.Printf("🐉 [BLOCKCHAIN] Found active syndicate membership: %s for %s",
                    syndicateData.Syndicate.String(), farmerAddress)
                return syndicateData.Syndicate
            } else {
                log.Printf("🐉 [BLOCKCHAIN] Found expired syndicate membership: %s for %s",
                    syndicateData.Syndicate.String(), farmerAddress)
            }
        }
    }

    // No active syndicate membership found
    return soloMiner
}

// IsStuckInSync checks if the blockchain is stuck in a sync loop
func (bc *Blockchain) IsStuckInSync() bool {
    bc.mu.RLock()
    defer bc.mu.RUnlock()
    
    // Consider stuck if same height for more than 5 minutes
    stuckThreshold := 5 * time.Minute
    return time.Since(bc.lastHeightChangeTime) > stuckThreshold
}

// RecordSyncAttempt records a sync attempt and checks if we should reset
func (bc *Blockchain) RecordSyncAttempt(newHeight uint64) bool {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    if newHeight > bc.tipHeight {
        // Progress made - reset stuck counter
        bc.tipHeight = newHeight
        bc.lastHeightChangeTime = time.Now()
        bc.stuckSyncAttempts = 0
        return false
    } else {
        // No progress - increment stuck counter
        bc.stuckSyncAttempts++
        log.Printf("🐱 [BLOCKCHAIN] Sync attempt %d/%d failed (still at height %d)", 
            bc.stuckSyncAttempts, bc.maxStuckAttempts, bc.tipHeight)
        
        if bc.stuckSyncAttempts >= bc.maxStuckAttempts {
            log.Printf("🙀 [BLOCKCHAIN] Maximum stuck attempts reached! Time for nuclear reset...")
            return true // Should reset
        }
    }
    return false
}

// NuclearReset completely wipes the blockchain and starts from genesis
// This is the "cat knocking stuff off the counter" solution
func (bc *Blockchain) NuclearReset() error {
    bc.mu.Lock()
    defer bc.mu.Unlock()
    
    log.Printf("☢️  [BLOCKCHAIN] NUCLEAR RESET INITIATED - Wiping all blockchain data!")
    log.Printf("🙀 [BLOCKCHAIN] *Cat knocking everything off the counter*")
    
    // Clear in-memory state
    bc.blocks = make(map[string]*Block)
    bc.blocksByHeight = make(map[uint64]*Block)
    bc.tipHash = ""
    bc.tipHeight = 0
    bc.lastHeightChangeTime = time.Now()
    bc.stuckSyncAttempts = 0
    
    // Remove all blockchain files
    if err := os.RemoveAll(bc.dataDir); err != nil {
        return fmt.Errorf("failed to remove blockchain directory: %w", err)
    }
    
    // Recreate directory
    if err := os.MkdirAll(bc.dataDir, 0755); err != nil {
        return fmt.Errorf("failed to recreate blockchain directory: %w", err)
    }
    
    // Reinitialize token system
    tokenDataDir := filepath.Join(bc.dataDir, "tokens")
    if err := os.MkdirAll(tokenDataDir, 0755); err != nil {
        return fmt.Errorf("failed to create token directory: %w", err)
    }
    
    // Reset token state
    if bc.tokenState != nil {
        newTokenState, err := NewTokenState(tokenDataDir)
        if err != nil {
            return fmt.Errorf("failed to reset token state: %w", err)
        }
        bc.tokenState = newTokenState
    }
    
    // Reset token executor
    if bc.tokenExecutor != nil {
        bc.tokenExecutor = NewTokenExecutor(bc.tokenState, bc.syndicateManager)
    }
    
    // Reset syndicate system
    if bc.syndicateManager != nil {
        bc.syndicateManager = NewSyndicateManager()
    }
    
    log.Printf("☢️  [BLOCKCHAIN] Nuclear reset complete! Starting fresh from genesis.")
    log.Printf("🐱 [BLOCKCHAIN] Counter is now clear - ready to sync from peers!")
    
    return nil
}
