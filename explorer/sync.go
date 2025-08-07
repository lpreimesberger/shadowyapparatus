package main

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "time"
)

// SyncService handles background synchronization with the Shadowy node
type SyncService struct {
    nodeURL  string
    database *Database
    client   *http.Client
    stopCh   chan struct{}
}

// NewSyncService creates a new sync service
func NewSyncService(nodeURL string, database *Database) *SyncService {
    return &SyncService{
        nodeURL:  nodeURL,
        database: database,
        client: &http.Client{
            Timeout: 30 * time.Second,
        },
        stopCh: make(chan struct{}),
    }
}

// Start begins the background synchronization
func (s *SyncService) Start() {
    log.Printf("🔄 Starting background sync service...")

    // Initial sync
    go s.syncOnce()

    // Periodic sync every minute
    go func() {
        ticker := time.NewTicker(1 * time.Minute)
        defer ticker.Stop()

        for {
            select {
            case <-ticker.C:
                s.syncOnce()
            case <-s.stopCh:
                log.Printf("🛑 Sync service stopped")
                return
            }
        }
    }()
}

// Stop stops the sync service
func (s *SyncService) Stop() {
    close(s.stopCh)
}

// syncOnce performs a single synchronization cycle
func (s *SyncService) syncOnce() {
    log.Printf("🔄 Syncing with Shadowy node...")

    // Get blockchain stats from the node
    stats, err := s.getBlockchainStats()
    if err != nil {
        log.Printf("❌ Failed to get blockchain stats: %v", err)
        return
    }

    // Get our latest height
    localHeight, err := s.database.GetLatestHeight()
    if err != nil {
        log.Printf("❌ Failed to get local height: %v", err)
        return
    }

    log.Printf("📊 Local height: %d, Remote height: %d", localHeight, stats.TipHeight)

    // Sync missing blocks
    if stats.TipHeight > localHeight {
        s.syncBlocks(localHeight+1, stats.TipHeight)
    }

    // Update last sync time
    s.database.SetLastSyncTime(time.Now())

    log.Printf("✅ Sync completed")
}

// BlockchainStats represents the stats from the Shadowy node
type BlockchainStats struct {
    TipHeight uint64 `json:"tip_height"`
    TipHash   string `json:"tip_hash"`
}

// getBlockchainStats fetches blockchain statistics from the node
func (s *SyncService) getBlockchainStats() (*BlockchainStats, error) {
    theURL := fmt.Sprintf("%s/api/v1/blockchain", s.nodeURL)
    resp, err := s.client.Get(theURL)
    if err != nil {
        return nil, fmt.Errorf("failed to fetch stats: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("%s returned status %d", theURL, resp.StatusCode)
    }

    var stats BlockchainStats
    if err := json.NewDecoder(resp.Body).Decode(&stats); err != nil {
        return nil, fmt.Errorf("failed to decode stats: %w", err)
    }

    return &stats, nil
}

// syncBlocks syncs blocks from startHeight to endHeight
func (s *SyncService) syncBlocks(startHeight, endHeight uint64) {
    log.Printf("📥 Syncing blocks %d to %d", startHeight, endHeight)

    // Sync in batches to avoid overwhelming the node
    batchSize := uint64(10)

    for height := startHeight; height <= endHeight; height += batchSize {
        endBatch := height + batchSize - 1
        if endBatch > endHeight {
            endBatch = endHeight
        }

        if err := s.syncBlockBatch(height, endBatch); err != nil {
            log.Printf("❌ Failed to sync batch %d-%d: %v", height, endBatch, err)
            continue
        }

        log.Printf("✅ Synced blocks %d-%d", height, endBatch)

        // Small delay to be nice to the node
        time.Sleep(100 * time.Millisecond)
    }
}

// syncBlockBatch syncs a batch of blocks
func (s *SyncService) syncBlockBatch(startHeight, endHeight uint64) error {
    for height := startHeight; height <= endHeight; height++ {
        if err := s.syncBlock(height); err != nil {
            return fmt.Errorf("failed to sync block %d: %w", height, err)
        }
    }
    return nil
}

// syncBlock syncs a single block
func (s *SyncService) syncBlock(height uint64) error {
    // Get block from node
    resp, err := s.client.Get(fmt.Sprintf("%s/api/v1/blockchain/block/height/%d", s.nodeURL, height))
    if err != nil {
        return fmt.Errorf("failed to fetch block: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("node returned status %d", resp.StatusCode)
    }

    var block Block
    if err := json.NewDecoder(resp.Body).Decode(&block); err != nil {
        return fmt.Errorf("failed to decode block: %w", err)
    }

    // Calculate block hash
    blockHash := s.calculateBlockHash(&block)

    // Store in database
    if err := s.database.StoreBlock(blockHash, &block); err != nil {
        return fmt.Errorf("failed to store block: %w", err)
    }
    
    // Extract and store individual transactions
    if err := s.extractAndStoreTransactions(blockHash, &block); err != nil {
        log.Printf("❌ Failed to extract transactions from block %d: %v", block.Header.Height, err)
        // Don't fail the entire sync for transaction parsing errors
    }

    return nil
}

// calculateBlockHash calculates the hash of a block
// This is a simplified hash calculation - you may need to adjust based on Shadowy's actual hashing
func (s *SyncService) calculateBlockHash(block *Block) string {
    // Create a simple hash from block data
    // In production, this should match Shadowy's exact block hashing algorithm
    data, _ := json.Marshal(block.Header)
    hash := sha256.Sum256(data)
    return hex.EncodeToString(hash[:])
}

// GetNetworkStats returns current network statistics
func (s *SyncService) GetNetworkStats() (*NetworkStats, error) {
    localHeight, err := s.database.GetLatestHeight()
    if err != nil {
        return nil, err
    }

    totalBlocks, err := s.database.GetBlockCount()
    if err != nil {
        return nil, err
    }

    lastSync, err := s.database.GetLastSyncTime()
    if err != nil {
        return nil, err
    }

    syncStatus := "active"
    if time.Since(lastSync) > 5*time.Minute {
        syncStatus = "stale"
    }

    return &NetworkStats{
        Height:      localHeight,
        TotalBlocks: totalBlocks,
        LastSync:    lastSync,
        SyncStatus:  syncStatus,
        NodeURL:     s.nodeURL,
    }, nil
}

// extractAndStoreTransactions parses and stores individual transactions from a block
func (s *SyncService) extractAndStoreTransactions(blockHash string, block *Block) error {
    log.Printf("📦 Block %d: Processing %d transactions", block.Header.Height, len(block.Body.Transactions))
    for _, signedTx := range block.Body.Transactions {
        // Parse the raw transaction
        var tx Transaction
        if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
            log.Printf("❌ Failed to parse transaction %s: %v", signedTx.TxHash, err)
            continue
        }
        
        // Process regular transaction outputs
        for _, output := range tx.Outputs {
            if output.Address != "" {
                walletTx := &WalletTransaction{
                    TxHash:      signedTx.TxHash,
                    BlockHash:   blockHash,
                    BlockHeight: block.Header.Height,
                    Timestamp:   tx.Timestamp,
                    Type:        "received",
                    Amount:      output.Value,
                    Fee:         0, // We'll calculate this below
                    FromAddress: "", // We'll try to determine this from inputs
                    ToAddress:   output.Address,
                }
                
                // Try to determine from address from inputs
                if len(tx.Inputs) > 0 && tx.Inputs[0].ScriptSig != "" {
                    // For now, extract from script sig if possible
                    // This is simplified - real implementation would need to parse scripts properly
                    walletTx.FromAddress = "unknown" // Placeholder
                }
                
                // Store the transaction
                if err := s.database.StoreTransaction(walletTx); err != nil {
                    log.Printf("❌ Failed to store transaction %s: %v", signedTx.TxHash, err)
                }
            }
        }
        
        // Process token operations
        log.Printf("🔍 Block %d: Processing %d token operations", block.Header.Height, len(tx.TokenOps))
        for i, tokenOp := range tx.TokenOps {
            log.Printf("🪙 Token Op %d: Type=%d (%s), TokenID=%.8s, Amount=%d, From=%s, To=%s", 
                i, int(tokenOp.Type), tokenOp.Type.String(), tokenOp.TokenID, tokenOp.Amount, tokenOp.From, tokenOp.To)
            
            if tokenOp.To != "" || tokenOp.From != "" {
                walletTx := &WalletTransaction{
                    TxHash:      signedTx.TxHash,
                    BlockHash:   blockHash,
                    BlockHeight: block.Header.Height,
                    Timestamp:   tx.Timestamp,
                    Type:        "token_" + tokenOp.Type.String(),
                    Amount:      0, // Token ops don't change main balance
                    Fee:         0,
                    FromAddress: tokenOp.From,
                    ToAddress:   tokenOp.To,
                    TokenSymbol: tokenOp.TokenID,
                    TokenAmount: tokenOp.Amount,
                }
                
                if err := s.database.StoreTransaction(walletTx); err != nil {
                    log.Printf("❌ Failed to store token transaction %s: %v", signedTx.TxHash, err)
                }
                
                // Process token-specific operations
                if err := s.processTokenOperation(blockHash, block, signedTx.TxHash, &tokenOp, tx.Timestamp); err != nil {
                    log.Printf("❌ Failed to process token operation %s: %v", signedTx.TxHash, err)
                }
            }
        }
    }
    
    // Store mining reward for the farmer
    if block.Header.FarmerAddress != "" {
        // Mining reward transaction
        rewardTx := &WalletTransaction{
            TxHash:      fmt.Sprintf("mining_%s", blockHash),
            BlockHash:   blockHash,
            BlockHeight: block.Header.Height,
            Timestamp:   block.Header.Timestamp,
            Type:        "mining_reward",
            Amount:      1000000, // Placeholder - should be actual block reward
            Fee:         0,
            FromAddress: "",
            ToAddress:   block.Header.FarmerAddress,
        }
        
        if err := s.database.StoreTransaction(rewardTx); err != nil {
            log.Printf("❌ Failed to store mining reward %s: %v", rewardTx.TxHash, err)
        }
    }
    
    return nil
}

// processTokenOperation handles token-specific operations and updates token records
func (s *SyncService) processTokenOperation(blockHash string, block *Block, txHash string, tokenOp *TokenOperation, timestamp time.Time) error {
    tokenID := tokenOp.TokenID
    
    // Store token transaction
    tokenTx := &TokenTransaction{
        TxHash:      txHash,
        BlockHash:   blockHash,
        BlockHeight: block.Header.Height,
        Timestamp:   timestamp,
        Type:        tokenOp.Type.String(),
        Amount:      tokenOp.Amount,
        FromAddress: tokenOp.From,
        ToAddress:   tokenOp.To,
    }
    
    if err := s.database.StoreTokenTransaction(tokenID, tokenTx); err != nil {
        return fmt.Errorf("failed to store token transaction: %w", err)
    }
    
    // Handle different token operation types
    switch tokenOp.Type {
    case TOKEN_CREATE:
        // Create new token record using metadata if available
        name := "Token " + tokenID[:8] // Default name
        ticker := "TKN" + tokenID[:4]  // Default ticker
        decimals := uint8(6)           // Default decimals
        meltValue := uint64(1000000)   // Default lock amount
        
        if tokenOp.Metadata != nil {
            name = tokenOp.Metadata.Name
            ticker = tokenOp.Metadata.Ticker
            decimals = tokenOp.Metadata.Decimals
            meltValue = tokenOp.Metadata.LockAmount
        }
        
        token := &TokenInfo{
            TokenID:       tokenID,
            Name:          name,
            Ticker:        ticker,
            TotalSupply:   tokenOp.Amount,
            Decimals:      decimals,
            Creator:       tokenOp.To,
            CreationTime:  timestamp,
            CreationBlock: block.Header.Height,
            
            // Statistics (will be updated as transactions occur)
            HolderCount:       1,
            TransferCount:     0,
            LastActivity:      timestamp,
            TotalMelted:       0,
            CirculatingSupply: tokenOp.Amount,
            MeltValue:         meltValue,
        }
        
        if err := s.database.StoreToken(token); err != nil {
            return fmt.Errorf("failed to store new token: %w", err)
        }
        
        log.Printf("✅ Created token: %s (%s) - ID: %.8s", token.Name, token.Ticker, token.TokenID)
        
        // Create initial holder record
        if err := s.database.UpdateTokenHolder(tokenID, tokenOp.To, tokenOp.Amount); err != nil {
            return fmt.Errorf("failed to create initial token holder: %w", err)
        }
        
    case TOKEN_TRANSFER:
        // Update holder balances
        if tokenOp.From != "" {
            // Get current balance and subtract
            fromBalance, err := s.getTokenBalance(tokenID, tokenOp.From)
            if err != nil {
                fromBalance = 0 // Assume 0 if not found
            }
            newFromBalance := fromBalance - tokenOp.Amount
            if newFromBalance < 0 {
                newFromBalance = 0
            }
            
            if err := s.database.UpdateTokenHolder(tokenID, tokenOp.From, uint64(newFromBalance)); err != nil {
                return fmt.Errorf("failed to update from holder balance: %w", err)
            }
        }
        
        if tokenOp.To != "" {
            // Get current balance and add
            toBalance, err := s.getTokenBalance(tokenID, tokenOp.To)
            if err != nil {
                toBalance = 0 // Assume 0 if not found
            }
            newToBalance := toBalance + tokenOp.Amount
            
            if err := s.database.UpdateTokenHolder(tokenID, tokenOp.To, newToBalance); err != nil {
                return fmt.Errorf("failed to update to holder balance: %w", err)
            }
        }
        
        // Update token statistics
        if err := s.updateTokenStats(tokenID, timestamp, "transfer"); err != nil {
            log.Printf("❌ Failed to update token stats: %v", err)
        }
        
    case TOKEN_MELT:
        // Reduce circulating supply
        if tokenOp.From != "" {
            // Get current balance and subtract
            fromBalance, err := s.getTokenBalance(tokenID, tokenOp.From)
            if err != nil {
                fromBalance = 0
            }
            newFromBalance := fromBalance - tokenOp.Amount
            if newFromBalance < 0 {
                newFromBalance = 0
            }
            
            if err := s.database.UpdateTokenHolder(tokenID, tokenOp.From, uint64(newFromBalance)); err != nil {
                return fmt.Errorf("failed to update holder balance for melt: %w", err)
            }
        }
        
        // Update token statistics
        if err := s.updateTokenStats(tokenID, timestamp, "melt"); err != nil {
            log.Printf("❌ Failed to update token stats: %v", err)
        }
        
    case POOL_CREATE:
        // Create new liquidity pool
        if err := s.processPoolCreation(blockHash, block, txHash, tokenOp, timestamp); err != nil {
            return fmt.Errorf("failed to process pool creation: %w", err)
        }
    }
    
    return nil
}

// getTokenBalance retrieves current token balance for an address
func (s *SyncService) getTokenBalance(tokenID, address string) (uint64, error) {
    holders, err := s.database.GetTokenHolders(tokenID, 1000) // Get more holders to find this one
    if err != nil {
        return 0, err
    }
    
    for _, holder := range holders {
        if holder.Address == address {
            return holder.Balance, nil
        }
    }
    
    return 0, nil // Not found = 0 balance
}

// updateTokenStats updates token statistics
func (s *SyncService) updateTokenStats(tokenID string, timestamp time.Time, opType string) error {
    token, err := s.database.GetToken(tokenID)
    if err != nil {
        return err
    }
    
    // Update last activity
    token.LastActivity = timestamp
    
    // Update operation-specific stats
    switch opType {
    case "transfer":
        token.TransferCount++
    case "melt":
        // Would need to track total melted amount
    }
    
    // Recalculate holder count (simplified)
    holders, err := s.database.GetTokenHolders(tokenID, 1000)
    if err == nil {
        token.HolderCount = len(holders)
    }
    
    return s.database.StoreToken(token)
}

// processPoolCreation creates a new liquidity pool from a POOL_CREATE operation
func (s *SyncService) processPoolCreation(blockHash string, block *Block, txHash string, tokenOp *TokenOperation, timestamp time.Time) error {
    poolID := tokenOp.TokenID
    
    // Extract pool metadata from token operation
    var tokenA, tokenB string
    var tokenASymbol, tokenBSymbol string = "SHADOW", "SHADOW" // Default symbols
    var reserveA, reserveB uint64 = 0, 0
    var totalLiquidity uint64 = tokenOp.Amount
    
    // Parse pool metadata if available
    if tokenOp.Metadata != nil {
        // For pool creation, metadata might contain pool parameters
        // This is simplified - actual implementation would need to parse pool-specific metadata
        tokenA = tokenOp.Metadata.Creator // Using creator field for tokenA ID
        tokenASymbol = tokenOp.Metadata.Ticker
        
        // Token B could be SHADOW (empty) or another token
        if tokenOp.Metadata.URI != "" {
            tokenB = tokenOp.Metadata.URI // Using URI field for tokenB ID (if not SHADOW pair)
            // Would need to look up tokenB symbol from database
            if tokenBInfo, err := s.database.GetToken(tokenB); err == nil {
                tokenBSymbol = tokenBInfo.Ticker
            } else {
                tokenBSymbol = "TKN" + tokenB[:4] // Fallback
            }
        }
        
        // Initial reserves from lock amount
        reserveA = tokenOp.Metadata.LockAmount
        reserveB = tokenOp.Amount
    } else {
        // Default pool setup for SHADOW pair
        tokenA = poolID[:32] // First 32 chars as tokenA
        tokenASymbol = "TKN" + poolID[:4]
        tokenB = "" // Empty means SHADOW pair
        tokenBSymbol = "SHADOW"
        reserveA = tokenOp.Amount
        reserveB = 1000000 // Default SHADOW reserve
    }
    
    // Calculate initial TVL (simplified - in SHADOW units)
    tvl := reserveB + (reserveA / 1000) // Very basic conversion
    
    pool := &LiquidityPool{
        PoolID:         poolID,
        TokenA:         tokenA,
        TokenB:         tokenB,
        TokenASymbol:   tokenASymbol,
        TokenBSymbol:   tokenBSymbol,
        ReserveA:       reserveA,
        ReserveB:       reserveB,
        TotalLiquidity: totalLiquidity,
        Creator:        tokenOp.To,
        CreationTime:   timestamp,
        CreationBlock:  block.Header.Height,
        
        // Initial statistics
        TradeCount:   0,
        VolumeA:      0,
        VolumeB:      0,
        LastActivity: timestamp,
        APR:          0.0,
        TVL:          tvl,
    }
    
    if err := s.database.StorePool(pool); err != nil {
        return fmt.Errorf("failed to store new pool: %w", err)
    }
    
    log.Printf("✅ Created liquidity pool: %s/%s - ID: %.8s", pool.TokenASymbol, pool.TokenBSymbol, pool.PoolID)
    
    // Store pool creation transaction
    poolTx := &PoolTransaction{
        TxHash:      txHash,
        BlockHash:   blockHash,
        BlockHeight: block.Header.Height,
        Timestamp:   timestamp,
        Type:        "create",
        AmountA:     reserveA,
        AmountB:     reserveB,
        Address:     tokenOp.To,
        LPTokens:    totalLiquidity,
    }
    
    if err := s.database.StorePoolTransaction(poolID, poolTx); err != nil {
        return fmt.Errorf("failed to store pool creation transaction: %w", err)
    }
    
    return nil
}
