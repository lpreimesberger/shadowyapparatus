package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TokenState manages the state of all tokens in the system
type TokenState struct {
	// Token registry: tokenID -> metadata
	tokens map[string]*TokenMetadata
	
	// Token balances: tokenID -> (address -> balance)
	balances map[string]map[string]uint64
	
	// Locked Shadow tracking: tokenID -> total locked amount
	lockedShadow map[string]uint64
	
	// Concurrency control
	mu sync.RWMutex
	
	// Storage
	dataDir string
}

// TokenBalance represents a token balance for an address
type TokenBalance struct {
	TokenID   string `json:"token_id"`
	Address   string `json:"address"`
	Balance   uint64 `json:"balance"`
	TokenInfo *TokenMetadata `json:"token_info,omitempty"`
}

// TokenStateSnapshot represents the entire token state at a point in time
type TokenStateSnapshot struct {
	Tokens       map[string]*TokenMetadata       `json:"tokens"`
	Balances     map[string]map[string]uint64    `json:"balances"`
	LockedShadow map[string]uint64               `json:"locked_shadow"`
	Timestamp    time.Time                       `json:"timestamp"`
	BlockHeight  uint64                          `json:"block_height"`
}

// NewTokenState creates a new token state manager
func NewTokenState(dataDir string) (*TokenState, error) {
	if err := os.MkdirAll(dataDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create token data directory: %w", err)
	}
	
	ts := &TokenState{
		tokens:       make(map[string]*TokenMetadata),
		balances:     make(map[string]map[string]uint64),
		lockedShadow: make(map[string]uint64),
		dataDir:      dataDir,
	}
	
	// Load existing state if it exists
	if err := ts.loadState(); err != nil {
		// If loading fails, start with clean state (log the error but don't fail)
		fmt.Printf("Warning: Failed to load token state, starting fresh: %v\n", err)
	}
	
	return ts, nil
}

// CreateToken registers a new token in the system
func (ts *TokenState) CreateToken(tokenID string, metadata *TokenMetadata) error {
	log.Printf("🔍 [TOKEN_STATE] CreateToken called for tokenID: %s", tokenID)
	log.Printf("🔍 [TOKEN_STATE] Attempting to acquire mutex lock...")
	ts.mu.Lock()
	log.Printf("🔍 [TOKEN_STATE] Mutex lock acquired successfully")
	defer func() {
		log.Printf("🔍 [TOKEN_STATE] Releasing mutex lock...")
		ts.mu.Unlock()
		log.Printf("🔍 [TOKEN_STATE] Mutex lock released")
	}()
	
	log.Printf("🔍 [TOKEN_STATE] Checking if token already exists...")
	// Check if token already exists
	if _, exists := ts.tokens[tokenID]; exists {
		log.Printf("❌ [TOKEN_STATE] Token %s already exists", tokenID)
		return fmt.Errorf("token %s already exists", tokenID)
	}
	log.Printf("✅ [TOKEN_STATE] Token does not exist, proceeding...")
	
	log.Printf("🔍 [TOKEN_STATE] Validating metadata...")
	// Validate metadata
	if metadata == nil {
		log.Printf("❌ [TOKEN_STATE] Metadata is nil")
		return fmt.Errorf("token metadata cannot be nil")
	}
	
	if metadata.TotalSupply == 0 {
		log.Printf("❌ [TOKEN_STATE] Total supply is zero")
		return fmt.Errorf("token total supply cannot be zero")
	}
	
	if metadata.LockAmount == 0 {
		log.Printf("❌ [TOKEN_STATE] Lock amount is zero")
		return fmt.Errorf("token lock amount cannot be zero")
	}
	log.Printf("✅ [TOKEN_STATE] Metadata validation passed")
	
	log.Printf("🔍 [TOKEN_STATE] Calculating total locked shadow...")
	// Calculate total Shadow that needs to be locked for this token
	totalLocked := metadata.TotalSupply * metadata.LockAmount
	log.Printf("🔍 [TOKEN_STATE] Total locked: %d", totalLocked)
	
	log.Printf("🔍 [TOKEN_STATE] Registering token in maps...")
	// Register the token
	ts.tokens[tokenID] = metadata
	log.Printf("🔍 [TOKEN_STATE] Token metadata registered")
	
	ts.balances[tokenID] = make(map[string]uint64)
	log.Printf("🔍 [TOKEN_STATE] Balance map created")
	
	ts.lockedShadow[tokenID] = totalLocked
	log.Printf("🔍 [TOKEN_STATE] Locked shadow recorded")
	
	log.Printf("🔍 [TOKEN_STATE] Giving initial supply to creator: %s", metadata.Creator)
	// Give initial supply to creator
	ts.balances[tokenID][metadata.Creator] = metadata.TotalSupply
	log.Printf("🔍 [TOKEN_STATE] Initial supply assigned")
	
	// Save state (create snapshot while we still hold the write lock)
	log.Printf("🔍 [TOKEN_STATE] Creating snapshot for save...")
	snapshot := ts.createSnapshotUnsafe(0) // Create snapshot without acquiring lock
	log.Printf("🔍 [TOKEN_STATE] Calling saveStateWithSnapshot()...")
	if err := ts.saveStateWithSnapshot(snapshot); err != nil {
		log.Printf("❌ [TOKEN_STATE] saveStateWithSnapshot() failed: %v", err)
		// Rollback on save failure
		delete(ts.tokens, tokenID)
		delete(ts.balances, tokenID)
		delete(ts.lockedShadow, tokenID)
		return fmt.Errorf("failed to save token state: %w", err)
	}
	log.Printf("✅ [TOKEN_STATE] saveStateWithSnapshot() completed successfully")
	
	log.Printf("✅ [TOKEN_STATE] CreateToken() completed successfully for tokenID: %s", tokenID)
	return nil
}

// TransferToken moves tokens from one address to another
func (ts *TokenState) TransferToken(tokenID string, from, to string, amount uint64) error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	// Check if token exists
	if _, exists := ts.tokens[tokenID]; !exists {
		return fmt.Errorf("token %s does not exist", tokenID)
	}
	
	// Check if from address has sufficient balance
	fromBalance := ts.balances[tokenID][from]
	if fromBalance < amount {
		return fmt.Errorf("insufficient token balance: have %d, need %d", fromBalance, amount)
	}
	
	// Perform the transfer
	ts.balances[tokenID][from] = fromBalance - amount
	ts.balances[tokenID][to] += amount
	
	// Clean up zero balances to save space
	if ts.balances[tokenID][from] == 0 {
		delete(ts.balances[tokenID], from)
	}
	
	// Save state (create snapshot while we still hold the write lock)
	snapshot := ts.createSnapshotUnsafe(0)
	if err := ts.saveStateWithSnapshot(snapshot); err != nil {
		// Rollback on save failure
		ts.balances[tokenID][from] = fromBalance
		ts.balances[tokenID][to] -= amount
		if ts.balances[tokenID][to] == 0 {
			delete(ts.balances[tokenID], to)
		}
		return fmt.Errorf("failed to save token state: %w", err)
	}
	
	return nil
}

// MeltToken burns tokens and returns the locked Shadow amount
func (ts *TokenState) MeltToken(tokenID string, from string, amount uint64) (uint64, error) {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	// Check if token exists
	tokenInfo, exists := ts.tokens[tokenID]
	if !exists {
		return 0, fmt.Errorf("token %s does not exist", tokenID)
	}
	
	// Check if from address has sufficient balance
	fromBalance := ts.balances[tokenID][from]
	if fromBalance < amount {
		return 0, fmt.Errorf("insufficient token balance: have %d, need %d", fromBalance, amount)
	}
	
	// Calculate Shadow to return (amount * lock_amount per token)
	shadowToReturn := amount * tokenInfo.LockAmount
	
	// Burn the tokens (reduce balance and total locked Shadow)
	ts.balances[tokenID][from] = fromBalance - amount
	ts.lockedShadow[tokenID] -= shadowToReturn
	
	// Clean up zero balances
	if ts.balances[tokenID][from] == 0 {
		delete(ts.balances[tokenID], from)
	}
	
	// Save state (create snapshot while we still hold the write lock)
	snapshot := ts.createSnapshotUnsafe(0)
	if err := ts.saveStateWithSnapshot(snapshot); err != nil {
		// Rollback on save failure
		ts.balances[tokenID][from] = fromBalance
		ts.lockedShadow[tokenID] += shadowToReturn
		return 0, fmt.Errorf("failed to save token state: %w", err)
	}
	
	return shadowToReturn, nil
}

// GetTokenInfo returns metadata for a token
func (ts *TokenState) GetTokenInfo(tokenID string) (*TokenMetadata, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	tokenInfo, exists := ts.tokens[tokenID]
	if !exists {
		return nil, fmt.Errorf("token %s does not exist", tokenID)
	}
	
	// Return a copy to prevent external modification
	copy := *tokenInfo
	return &copy, nil
}

// GetTokenBalance returns the balance of a specific token for an address
func (ts *TokenState) GetTokenBalance(tokenID, address string) (uint64, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	// Check if token exists
	if _, exists := ts.tokens[tokenID]; !exists {
		return 0, fmt.Errorf("token %s does not exist", tokenID)
	}
	
	return ts.balances[tokenID][address], nil
}

// GetAllTokenBalances returns all token balances for an address
func (ts *TokenState) GetAllTokenBalances(address string) ([]TokenBalance, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	var balances []TokenBalance
	
	for tokenID, tokenBalances := range ts.balances {
		if balance, exists := tokenBalances[address]; exists && balance > 0 {
			tokenInfo := ts.tokens[tokenID]
			balances = append(balances, TokenBalance{
				TokenID:   tokenID,
				Address:   address,
				Balance:   balance,
				TokenInfo: tokenInfo,
			})
		}
	}
	
	return balances, nil
}

// GetTokenHolders returns all addresses that hold a specific token
func (ts *TokenState) GetTokenHolders(tokenID string) (map[string]uint64, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	// Check if token exists
	if _, exists := ts.tokens[tokenID]; !exists {
		return nil, fmt.Errorf("token %s does not exist", tokenID)
	}
	
	// Return a copy of the balances map
	holders := make(map[string]uint64)
	for address, balance := range ts.balances[tokenID] {
		holders[address] = balance
	}
	
	return holders, nil
}

// GetTotalSupply returns the current circulating supply of a token
func (ts *TokenState) GetTotalSupply(tokenID string) (uint64, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	// Check if token exists
	if _, exists := ts.tokens[tokenID]; !exists {
		return 0, fmt.Errorf("token %s does not exist", tokenID)
	}
	
	// Sum all balances to get circulating supply
	total := uint64(0)
	for _, balance := range ts.balances[tokenID] {
		total += balance
	}
	
	return total, nil
}

// GetLockedShadow returns the total amount of Shadow locked for a token
func (ts *TokenState) GetLockedShadow(tokenID string) (uint64, error) {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	// Check if token exists
	if _, exists := ts.tokens[tokenID]; !exists {
		return 0, fmt.Errorf("token %s does not exist", tokenID)
	}
	
	return ts.lockedShadow[tokenID], nil
}

// ListAllTokens returns metadata for all registered tokens
func (ts *TokenState) ListAllTokens() map[string]*TokenMetadata {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	// Return copies to prevent external modification
	result := make(map[string]*TokenMetadata)
	for tokenID, metadata := range ts.tokens {
		copy := *metadata
		result[tokenID] = &copy
	}
	
	return result
}

// GetSnapshot returns a complete snapshot of the token state
func (ts *TokenState) GetSnapshot(blockHeight uint64) *TokenStateSnapshot {
	log.Printf("🔍 [TOKEN_STATE] GetSnapshot() called with blockHeight: %d", blockHeight)
	
	log.Printf("🔍 [TOKEN_STATE] Acquiring read lock...")
	ts.mu.RLock()
	log.Printf("🔍 [TOKEN_STATE] Read lock acquired")
	defer func() {
		log.Printf("🔍 [TOKEN_STATE] Releasing read lock...")
		ts.mu.RUnlock()
		log.Printf("🔍 [TOKEN_STATE] Read lock released")
	}()
	
	log.Printf("🔍 [TOKEN_STATE] Creating deep copy of tokens map (%d entries)...", len(ts.tokens))
	// Create deep copies of all data
	tokens := make(map[string]*TokenMetadata)
	for tokenID, metadata := range ts.tokens {
		copy := *metadata
		tokens[tokenID] = &copy
	}
	log.Printf("🔍 [TOKEN_STATE] Tokens map copied successfully")
	
	log.Printf("🔍 [TOKEN_STATE] Creating deep copy of balances map (%d entries)...", len(ts.balances))
	balances := make(map[string]map[string]uint64)
	for tokenID, tokenBalances := range ts.balances {
		balances[tokenID] = make(map[string]uint64)
		for address, balance := range tokenBalances {
			balances[tokenID][address] = balance
		}
	}
	log.Printf("🔍 [TOKEN_STATE] Balances map copied successfully")
	
	log.Printf("🔍 [TOKEN_STATE] Creating deep copy of locked shadow map (%d entries)...", len(ts.lockedShadow))
	lockedShadow := make(map[string]uint64)
	for tokenID, locked := range ts.lockedShadow {
		lockedShadow[tokenID] = locked
	}
	log.Printf("🔍 [TOKEN_STATE] Locked shadow map copied successfully")
	
	log.Printf("🔍 [TOKEN_STATE] Creating snapshot object...")
	snapshot := &TokenStateSnapshot{
		Tokens:       tokens,
		Balances:     balances,
		LockedShadow: lockedShadow,
		Timestamp:    time.Now().UTC(),
		BlockHeight:  blockHeight,
	}
	log.Printf("✅ [TOKEN_STATE] GetSnapshot() completed successfully")
	
	return snapshot
}

// saveState persists the current token state to disk
func (ts *TokenState) saveState() error {
	log.Printf("🔍 [TOKEN_STATE] saveState() called")
	
	log.Printf("🔍 [TOKEN_STATE] Creating snapshot...")
	snapshot := ts.GetSnapshot(0) // Block height will be updated by caller
	log.Printf("🔍 [TOKEN_STATE] Snapshot created successfully")
	
	return ts.saveStateWithSnapshot(snapshot)
}

// saveStateWithSnapshot persists a given snapshot to disk
func (ts *TokenState) saveStateWithSnapshot(snapshot *TokenStateSnapshot) error {
	log.Printf("🔍 [TOKEN_STATE] saveStateWithSnapshot() called")
	
	log.Printf("🔍 [TOKEN_STATE] Marshalling snapshot to JSON...")
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		log.Printf("❌ [TOKEN_STATE] Failed to marshal snapshot: %v", err)
		return fmt.Errorf("failed to marshal token state: %w", err)
	}
	log.Printf("🔍 [TOKEN_STATE] Snapshot marshalled successfully, size: %d bytes", len(data))
	
	log.Printf("🔍 [TOKEN_STATE] Building file path...")
	statePath := filepath.Join(ts.dataDir, "token_state.json")
	log.Printf("🔍 [TOKEN_STATE] File path: %s", statePath)
	
	log.Printf("🔍 [TOKEN_STATE] Writing file to disk...")
	if err := os.WriteFile(statePath, data, 0644); err != nil {
		log.Printf("❌ [TOKEN_STATE] Failed to write file: %v", err)
		return fmt.Errorf("failed to write token state file: %w", err)
	}
	log.Printf("✅ [TOKEN_STATE] File written successfully")
	
	log.Printf("✅ [TOKEN_STATE] saveStateWithSnapshot() completed successfully")
	return nil
}

// createSnapshotUnsafe creates a snapshot without acquiring locks (caller must hold appropriate lock)
func (ts *TokenState) createSnapshotUnsafe(blockHeight uint64) *TokenStateSnapshot {
	log.Printf("🔍 [TOKEN_STATE] createSnapshotUnsafe() called with blockHeight: %d", blockHeight)
	
	log.Printf("🔍 [TOKEN_STATE] Creating deep copy of tokens map (%d entries)...", len(ts.tokens))
	// Create deep copies of all data
	tokens := make(map[string]*TokenMetadata)
	for tokenID, metadata := range ts.tokens {
		copy := *metadata
		tokens[tokenID] = &copy
	}
	log.Printf("🔍 [TOKEN_STATE] Tokens map copied successfully")
	
	log.Printf("🔍 [TOKEN_STATE] Creating deep copy of balances map (%d entries)...", len(ts.balances))
	balances := make(map[string]map[string]uint64)
	for tokenID, tokenBalances := range ts.balances {
		balances[tokenID] = make(map[string]uint64)
		for address, balance := range tokenBalances {
			balances[tokenID][address] = balance
		}
	}
	log.Printf("🔍 [TOKEN_STATE] Balances map copied successfully")
	
	log.Printf("🔍 [TOKEN_STATE] Creating deep copy of locked shadow map (%d entries)...", len(ts.lockedShadow))
	lockedShadow := make(map[string]uint64)
	for tokenID, locked := range ts.lockedShadow {
		lockedShadow[tokenID] = locked
	}
	log.Printf("🔍 [TOKEN_STATE] Locked shadow map copied successfully")
	
	log.Printf("🔍 [TOKEN_STATE] Creating snapshot object...")
	snapshot := &TokenStateSnapshot{
		Tokens:       tokens,
		Balances:     balances,
		LockedShadow: lockedShadow,
		Timestamp:    time.Now().UTC(),
		BlockHeight:  blockHeight,
	}
	log.Printf("✅ [TOKEN_STATE] createSnapshotUnsafe() completed successfully")
	
	return snapshot
}

// loadState loads token state from disk
func (ts *TokenState) loadState() error {
	statePath := filepath.Join(ts.dataDir, "token_state.json")
	
	data, err := os.ReadFile(statePath)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing state file, start fresh
			return nil
		}
		return fmt.Errorf("failed to read token state file: %w", err)
	}
	
	var snapshot TokenStateSnapshot
	if err := json.Unmarshal(data, &snapshot); err != nil {
		return fmt.Errorf("failed to unmarshal token state: %w", err)
	}
	
	// Restore state from snapshot
	ts.tokens = snapshot.Tokens
	ts.balances = snapshot.Balances
	ts.lockedShadow = snapshot.LockedShadow
	
	// Initialize maps if they're nil
	if ts.tokens == nil {
		ts.tokens = make(map[string]*TokenMetadata)
	}
	if ts.balances == nil {
		ts.balances = make(map[string]map[string]uint64)
	}
	if ts.lockedShadow == nil {
		ts.lockedShadow = make(map[string]uint64)
	}
	
	fmt.Printf("Loaded token state: %d tokens, %d token types with balances\n", 
		len(ts.tokens), len(ts.balances))
	
	return nil
}

// GetAllTokens returns all token metadata
func (ts *TokenState) GetAllTokens() map[string]*TokenMetadata {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	// Return a copy to prevent external modification
	result := make(map[string]*TokenMetadata)
	for tokenID, metadata := range ts.tokens {
		copy := *metadata
		result[tokenID] = &copy
	}
	
	return result
}

// GetTokenBalances returns all balances for a specific token
func (ts *TokenState) GetTokenBalances(tokenID string) map[string]uint64 {
	ts.mu.RLock()
	defer ts.mu.RUnlock()
	
	// Check if token exists
	if _, exists := ts.tokens[tokenID]; !exists {
		return make(map[string]uint64)
	}
	
	// Return a copy to prevent external modification
	result := make(map[string]uint64)
	if balances, exists := ts.balances[tokenID]; exists {
		for address, balance := range balances {
			result[address] = balance
		}
	}
	
	return result
}

// ResetToGenesis resets the token state to genesis (empty) state
func (ts *TokenState) ResetToGenesis() error {
	ts.mu.Lock()
	defer ts.mu.Unlock()
	
	log.Printf("🔄 [TOKEN_STATE] Resetting token state to genesis...")
	
	// Clear all in-memory state
	ts.tokens = make(map[string]*TokenMetadata)
	ts.balances = make(map[string]map[string]uint64)
	ts.lockedShadow = make(map[string]uint64)
	
	// Remove entire token data directory and recreate it clean
	log.Printf("🗑️ [TOKEN_STATE] Removing entire token data directory: %s", ts.dataDir)
	if err := os.RemoveAll(ts.dataDir); err != nil {
		log.Printf("⚠️ [TOKEN_STATE] Failed to remove token data directory: %v", err)
	}
	
	// Recreate the data directory
	if err := os.MkdirAll(ts.dataDir, 0755); err != nil {
		return fmt.Errorf("failed to recreate token data directory: %w", err)
	}
	log.Printf("📁 [TOKEN_STATE] Recreated clean token data directory")
	
	// Save the empty state to disk (creates new clean file)
	if err := ts.saveState(); err != nil {
		return fmt.Errorf("failed to save reset token state: %w", err)
	}
	
	log.Printf("✅ [TOKEN_STATE] Token state reset to genesis complete")
	return nil
}