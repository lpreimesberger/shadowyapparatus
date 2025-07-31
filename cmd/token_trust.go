package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// TokenTrustLevel represents the trust level for a token
type TokenTrustLevel int

const (
	TrustUnknown TokenTrustLevel = iota // Default: requires user acceptance
	TrustBanned                        // Explicitly banned by user
	TrustAccepted                      // Accepted by user
	TrustVerified                      // Verified by tracker (future feature)
)

func (t TokenTrustLevel) String() string {
	switch t {
	case TrustUnknown:
		return "unknown"
	case TrustBanned:
		return "banned"
	case TrustAccepted:
		return "accepted"
	case TrustVerified:
		return "verified"
	default:
		return "unknown"
	}
}

// TokenTrustInfo contains trust information for a token
type TokenTrustInfo struct {
	TokenID        string          `json:"token_id"`
	TrustLevel     TokenTrustLevel `json:"trust_level"`
	UserAccepted   bool            `json:"user_accepted"`
	TrackerScore   float64         `json:"tracker_score"`    // 0.0-1.0, from tracker service
	AcceptedAt     *time.Time      `json:"accepted_at,omitempty"`
	Notes          string          `json:"notes,omitempty"`  // User notes about the token
	
	// Token metadata cache (for display purposes)
	Name           string          `json:"name,omitempty"`
	Ticker         string          `json:"ticker,omitempty"`
	Creator        string          `json:"creator,omitempty"`
	LastUpdated    time.Time       `json:"last_updated"`
}

// TokenTrustManager manages token trust settings for wallets
type TokenTrustManager struct {
	walletName string
	dataDir    string
	
	// Trust settings per token
	trustSettings map[string]*TokenTrustInfo
	
	// Synchronization
	mu sync.RWMutex
}

// NewTokenTrustManager creates a new token trust manager for a wallet
func NewTokenTrustManager(walletName, dataDir string) (*TokenTrustManager, error) {
	trustDir := filepath.Join(dataDir, "token_trust")
	if err := os.MkdirAll(trustDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create token trust directory: %w", err)
	}
	
	ttm := &TokenTrustManager{
		walletName:    walletName,
		dataDir:       trustDir,
		trustSettings: make(map[string]*TokenTrustInfo),
	}
	
	// Load existing trust settings
	if err := ttm.loadTrustSettings(); err != nil {
		fmt.Printf("Warning: Failed to load token trust settings for wallet %s: %v\n", walletName, err)
	}
	
	return ttm, nil
}

// GetTokenTrust returns the trust info for a token
func (ttm *TokenTrustManager) GetTokenTrust(tokenID string) *TokenTrustInfo {
	ttm.mu.RLock()
	defer ttm.mu.RUnlock()
	
	if trust, exists := ttm.trustSettings[tokenID]; exists {
		// Return a copy to prevent external modification
		copy := *trust
		return &copy
	}
	
	// Return default unknown trust for new tokens
	return &TokenTrustInfo{
		TokenID:      tokenID,
		TrustLevel:   TrustUnknown,
		UserAccepted: false,
		TrackerScore: 0.0, // Always 0.0 for now as specified
		LastUpdated:  time.Now().UTC(),
	}
}

// SetTokenTrust sets the trust level for a token
func (ttm *TokenTrustManager) SetTokenTrust(tokenID string, trustLevel TokenTrustLevel, notes string) error {
	ttm.mu.Lock()
	defer ttm.mu.Unlock()
	
	now := time.Now().UTC()
	
	trust, exists := ttm.trustSettings[tokenID]
	if !exists {
		trust = &TokenTrustInfo{
			TokenID:     tokenID,
			TrackerScore: 0.0, // Always 0.0 for now
		}
		ttm.trustSettings[tokenID] = trust
	}
	
	trust.TrustLevel = trustLevel
	trust.UserAccepted = (trustLevel == TrustAccepted || trustLevel == TrustVerified)
	trust.Notes = notes
	trust.LastUpdated = now
	
	if trust.UserAccepted && trust.AcceptedAt == nil {
		trust.AcceptedAt = &now
	}
	
	// Save to disk
	return ttm.saveTrustSettings()
}

// AcceptToken marks a token as accepted by the user
func (ttm *TokenTrustManager) AcceptToken(tokenID, notes string) error {
	return ttm.SetTokenTrust(tokenID, TrustAccepted, notes)
}

// BanToken marks a token as banned by the user
func (ttm *TokenTrustManager) BanToken(tokenID, reason string) error {
	return ttm.SetTokenTrust(tokenID, TrustBanned, reason)
}

// UpdateTokenMetadata updates the cached metadata for a token
func (ttm *TokenTrustManager) UpdateTokenMetadata(tokenID, name, ticker, creator string) error {
	ttm.mu.Lock()
	defer ttm.mu.Unlock()
	
	trust, exists := ttm.trustSettings[tokenID]
	if !exists {
		trust = &TokenTrustInfo{
			TokenID:      tokenID,
			TrustLevel:   TrustUnknown,
			UserAccepted: false,
			TrackerScore: 0.0,
		}
		ttm.trustSettings[tokenID] = trust
	}
	
	trust.Name = name
	trust.Ticker = ticker
	trust.Creator = creator
	trust.LastUpdated = time.Now().UTC()
	
	return ttm.saveTrustSettings()
}

// ListTrustedTokens returns all tokens with trust level >= TrustAccepted
func (ttm *TokenTrustManager) ListTrustedTokens() map[string]*TokenTrustInfo {
	ttm.mu.RLock()
	defer ttm.mu.RUnlock()
	
	trusted := make(map[string]*TokenTrustInfo)
	for tokenID, trust := range ttm.trustSettings {
		if trust.TrustLevel >= TrustAccepted {
			copy := *trust
			trusted[tokenID] = &copy
		}
	}
	
	return trusted
}

// ListBannedTokens returns all banned tokens
func (ttm *TokenTrustManager) ListBannedTokens() map[string]*TokenTrustInfo {
	ttm.mu.RLock()
	defer ttm.mu.RUnlock()
	
	banned := make(map[string]*TokenTrustInfo)
	for tokenID, trust := range ttm.trustSettings {
		if trust.TrustLevel == TrustBanned {
			copy := *trust
			banned[tokenID] = &copy
		}
	}
	
	return banned
}

// ListUnknownTokens returns all tokens with unknown trust level
func (ttm *TokenTrustManager) ListUnknownTokens() map[string]*TokenTrustInfo {
	ttm.mu.RLock()
	defer ttm.mu.RUnlock()
	
	unknown := make(map[string]*TokenTrustInfo)
	for tokenID, trust := range ttm.trustSettings {
		if trust.TrustLevel == TrustUnknown {
			copy := *trust
			unknown[tokenID] = &copy
		}
	}
	
	return unknown
}

// IsTokenTrusted returns true if the token is accepted or verified
func (ttm *TokenTrustManager) IsTokenTrusted(tokenID string) bool {
	trust := ttm.GetTokenTrust(tokenID)
	return trust.TrustLevel >= TrustAccepted
}

// IsTokenBanned returns true if the token is banned
func (ttm *TokenTrustManager) IsTokenBanned(tokenID string) bool {
	trust := ttm.GetTokenTrust(tokenID)
	return trust.TrustLevel == TrustBanned
}

// GetTrustSummary returns a summary of trust settings
func (ttm *TokenTrustManager) GetTrustSummary() map[string]int {
	ttm.mu.RLock()
	defer ttm.mu.RUnlock()
	
	summary := map[string]int{
		"unknown":  0,
		"banned":   0,
		"accepted": 0,
		"verified": 0,
		"total":    len(ttm.trustSettings),
	}
	
	for _, trust := range ttm.trustSettings {
		summary[trust.TrustLevel.String()]++
	}
	
	return summary
}

// saveTrustSettings saves trust settings to disk
func (ttm *TokenTrustManager) saveTrustSettings() error {
	trustFile := filepath.Join(ttm.dataDir, fmt.Sprintf("%s_trust.json", ttm.walletName))
	
	data, err := json.MarshalIndent(ttm.trustSettings, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal trust settings: %w", err)
	}
	
	if err := os.WriteFile(trustFile, data, 0644); err != nil {
		return fmt.Errorf("failed to write trust settings file: %w", err)
	}
	
	return nil
}

// loadTrustSettings loads trust settings from disk
func (ttm *TokenTrustManager) loadTrustSettings() error {
	trustFile := filepath.Join(ttm.dataDir, fmt.Sprintf("%s_trust.json", ttm.walletName))
	
	data, err := os.ReadFile(trustFile)
	if err != nil {
		if os.IsNotExist(err) {
			// No existing trust file, start fresh
			return nil
		}
		return fmt.Errorf("failed to read trust settings file: %w", err)
	}
	
	if err := json.Unmarshal(data, &ttm.trustSettings); err != nil {
		return fmt.Errorf("failed to unmarshal trust settings: %w", err)
	}
	
	// Initialize maps if nil
	if ttm.trustSettings == nil {
		ttm.trustSettings = make(map[string]*TokenTrustInfo)
	}
	
	fmt.Printf("Loaded token trust settings for wallet %s: %d tokens\n", 
		ttm.walletName, len(ttm.trustSettings))
	
	return nil
}

// ProcessUnknownTokenWarning displays a warning for unknown tokens and prompts user
func (ttm *TokenTrustManager) ProcessUnknownTokenWarning(tokenID, name, ticker, creator string) (bool, error) {
	trust := ttm.GetTokenTrust(tokenID)
	
	// If already processed, return the current state
	if trust.TrustLevel != TrustUnknown {
		return trust.UserAccepted, nil
	}
	
	// Update metadata cache
	if err := ttm.UpdateTokenMetadata(tokenID, name, ticker, creator); err != nil {
		fmt.Printf("Warning: Failed to update token metadata: %v\n", err)
	}
	
	// Display warning
	fmt.Printf("\n⚠️  UNKNOWN TOKEN DETECTED ⚠️\n")
	fmt.Printf("================================\n")
	fmt.Printf("Token ID: %s\n", tokenID)
	fmt.Printf("Name:     %s\n", name)
	fmt.Printf("Ticker:   %s\n", ticker)
	fmt.Printf("Creator:  %s\n", creator)
	fmt.Printf("Trust Score: %.1f/10.0 (from tracker)\n", trust.TrackerScore*10)
	fmt.Printf("\n🛡️  This token is UNKNOWN and may be unsafe!\n")
	fmt.Printf("Only accept tokens from trusted sources.\n")
	fmt.Printf("\nOptions:\n")
	fmt.Printf("  [a] Accept this token (mark as trusted)\n")
	fmt.Printf("  [b] Ban this token (hide from wallet)\n")
	fmt.Printf("  [i] Ignore for now (keep as unknown)\n")
	
	// For now, we'll return false (not accepted) since this is non-interactive
	// In a real implementation, this would prompt the user
	fmt.Printf("\n⚠️  Token marked as UNKNOWN - use 'wallet tokens accept <token_id>' to trust it\n")
	
	return false, nil
}