package cmd

import (
	"fmt"
	"log"
	"sync"
	"time"
)

// SyndicateMember represents a member of a syndicate
type SyndicateMember struct {
	Address          string        `json:"address"`
	NFTTokenID       string        `json:"nft_token_id"`       // The membership NFT ID
	ReportedCapacity uint64        `json:"reported_capacity"`  // Self-reported storage capacity
	AdjustedCapacity uint64        `json:"adjusted_capacity"`  // Capacity adjusted based on proof density
	JoinTime         time.Time     `json:"join_time"`
	ExpirationTime   time.Time     `json:"expiration_time"`
	RenewalCount     uint32        `json:"renewal_count"`
	UniqueProofs     uint64        `json:"unique_proofs"`      // Proofs submitted in current gross (144 blocks)
	LastProofReset   uint64        `json:"last_proof_reset"`   // Block height when proofs were last reset
}

// SyndicateStats represents statistics for a syndicate
type SyndicateStats struct {
	Syndicate       SyndicateType     `json:"syndicate"`
	Members         []SyndicateMember `json:"members"`
	TotalCapacity   uint64            `json:"total_capacity"`    // Sum of all adjusted capacities
	BlocksWon       uint64            `json:"blocks_won"`        // Blocks won in current fortnight
	LastBlockWin    uint64            `json:"last_block_win"`    // Block height of last win
	WinPercentage   float64           `json:"win_percentage"`    // Percentage of blocks won in fortnight
	FortnightStart  uint64            `json:"fortnight_start"`   // Starting block of current fortnight window
}

// SyndicateManager handles all syndicate operations and tracking
type SyndicateManager struct {
	// Syndicate data
	syndicates    map[SyndicateType]*SyndicateStats
	
	// Block tracking for fortnight windows (2016 blocks)
	blockHistory  []BlockWinner // Last 2016 blocks with winner info
	currentBlock  uint64
	
	// Concurrency control
	mu sync.RWMutex
}

// BlockWinner represents who won a specific block
type BlockWinner struct {
	BlockHeight uint64        `json:"block_height"`
	Winner      SyndicateType `json:"winner"`      // Which syndicate won (or -1 for solo)
	MinerAddr   string        `json:"miner_addr"`  // Individual miner address
	Timestamp   time.Time     `json:"timestamp"`
}

// NewSyndicateManager creates a new syndicate manager
func NewSyndicateManager() *SyndicateManager {
	sm := &SyndicateManager{
		syndicates:   make(map[SyndicateType]*SyndicateStats),
		blockHistory: make([]BlockWinner, 0, 2016), // Fortnight capacity
		currentBlock: 0,
	}
	
	// Initialize all four syndicates
	for syndicate := SyndicateSeiryu; syndicate <= SyndicateGenbu; syndicate++ {
		sm.syndicates[syndicate] = &SyndicateStats{
			Syndicate:      syndicate,
			Members:        make([]SyndicateMember, 0),
			TotalCapacity:  0,
			BlocksWon:      0,
			LastBlockWin:   0,
			WinPercentage:  0.0,
			FortnightStart: 0,
		}
	}
	
	log.Printf("🐉 [SYNDICATE_MANAGER] Initialized Four Guardians syndicate system")
	log.Printf("🐉 [SYNDICATE_MANAGER] Seiryu (Azure Dragon), Byakko (White Tiger), Suzaku (Vermillion Bird), Genbu (Black Tortoise)")
	
	return sm
}

// AddMember adds a new member to a syndicate based on their NFT
func (sm *SyndicateManager) AddMember(nftTokenID string, syndicateData *SyndicateData) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	syndicate := syndicateData.Syndicate
	stats, exists := sm.syndicates[syndicate]
	if !exists {
		return fmt.Errorf("syndicate %s does not exist", syndicate.String())
	}
	
	// Check if member already exists (shouldn't happen due to NFT uniqueness)
	for _, member := range stats.Members {
		if member.Address == syndicateData.MinerAddress {
			return fmt.Errorf("miner %s already has active membership in %s", 
				syndicateData.MinerAddress, syndicate.String())
		}
	}
	
	// Create new member
	member := SyndicateMember{
		Address:          syndicateData.MinerAddress,
		NFTTokenID:       nftTokenID,
		ReportedCapacity: syndicateData.ReportedCapacity,
		AdjustedCapacity: syndicateData.ReportedCapacity, // Initially same as reported
		JoinTime:         time.Unix(syndicateData.JoinTime, 0),
		ExpirationTime:   time.Unix(syndicateData.ExpirationTime, 0),
		RenewalCount:     syndicateData.RenewalCount,
		UniqueProofs:     0,
		LastProofReset:   sm.currentBlock,
	}
	
	// Add member to syndicate
	stats.Members = append(stats.Members, member)
	stats.TotalCapacity += member.AdjustedCapacity
	
	log.Printf("🐉 [SYNDICATE_MANAGER] %s joined %s (capacity: %d bytes)", 
		member.Address, syndicate.Description(), member.ReportedCapacity)
	
	return nil
}

// RemoveMember removes a member from their syndicate (when NFT expires or is melted)
func (sm *SyndicateManager) RemoveMember(nftTokenID string) error {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	// Find and remove member from any syndicate
	for syndicate, stats := range sm.syndicates {
		for i, member := range stats.Members {
			if member.NFTTokenID == nftTokenID {
				// Remove member from slice
				stats.Members = append(stats.Members[:i], stats.Members[i+1:]...)
				stats.TotalCapacity -= member.AdjustedCapacity
				
				log.Printf("🐉 [SYNDICATE_MANAGER] %s left %s (NFT expired/melted)", 
					member.Address, syndicate.Description())
				return nil
			}
		}
	}
	
	return fmt.Errorf("member with NFT %s not found in any syndicate", nftTokenID)
}

// GetSyndicateStats returns current statistics for a syndicate
func (sm *SyndicateManager) GetSyndicateStats(syndicate SyndicateType) (*SyndicateStats, error) {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	stats, exists := sm.syndicates[syndicate]
	if !exists {
		return nil, fmt.Errorf("syndicate %s does not exist", syndicate.String())
	}
	
	// Return a copy to prevent external modification
	statsCopy := *stats
	statsCopy.Members = make([]SyndicateMember, len(stats.Members))
	copy(statsCopy.Members, stats.Members)
	
	return &statsCopy, nil
}

// GetAllSyndicateStats returns statistics for all syndicates
func (sm *SyndicateManager) GetAllSyndicateStats() map[SyndicateType]*SyndicateStats {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	result := make(map[SyndicateType]*SyndicateStats)
	
	for syndicate, stats := range sm.syndicates {
		// Return copies to prevent external modification
		statsCopy := *stats
		statsCopy.Members = make([]SyndicateMember, len(stats.Members))
		copy(statsCopy.Members, stats.Members)
		result[syndicate] = &statsCopy
	}
	
	return result
}

// GetLowestCapacitySyndicate returns the syndicate with the lowest total capacity
// This is used for automatic assignment of new miners
func (sm *SyndicateManager) GetLowestCapacitySyndicate() SyndicateType {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	lowestSyndicate := SyndicateSeiryu
	lowestCapacity := sm.syndicates[SyndicateSeiryu].TotalCapacity
	
	for syndicate := SyndicateByakko; syndicate <= SyndicateGenbu; syndicate++ {
		if sm.syndicates[syndicate].TotalCapacity < lowestCapacity {
			lowestCapacity = sm.syndicates[syndicate].TotalCapacity
			lowestSyndicate = syndicate
		}
	}
	
	return lowestSyndicate
}

// UpdateBlockWin records a block win for tracking dominance
func (sm *SyndicateManager) UpdateBlockWin(blockHeight uint64, winner SyndicateType, minerAddr string) {
	sm.mu.Lock()
	defer sm.mu.Unlock()
	
	sm.currentBlock = blockHeight
	
	// Add to block history
	blockWin := BlockWinner{
		BlockHeight: blockHeight,
		Winner:      winner,
		MinerAddr:   minerAddr,
		Timestamp:   time.Now(),
	}
	
	// Maintain rolling window of 2016 blocks (fortnight)
	if len(sm.blockHistory) >= 2016 {
		sm.blockHistory = sm.blockHistory[1:] // Remove oldest
	}
	sm.blockHistory = append(sm.blockHistory, blockWin)
	
	// Update syndicate stats
	if winner >= SyndicateSeiryu && winner <= SyndicateGenbu {
		sm.syndicates[winner].BlocksWon++
		sm.syndicates[winner].LastBlockWin = blockHeight
		
		// Recalculate win percentages for all syndicates
		sm.recalculateWinPercentages()
		
		log.Printf("🐉 [SYNDICATE_MANAGER] Block %d won by %s (miner: %s)", 
			blockHeight, winner.Description(), minerAddr)
	}
}

// recalculateWinPercentages updates win percentages based on current fortnight window
func (sm *SyndicateManager) recalculateWinPercentages() {
	totalBlocks := uint64(len(sm.blockHistory))
	if totalBlocks == 0 {
		return
	}
	
	// Count wins for each syndicate in current window
	wins := make(map[SyndicateType]uint64)
	for _, block := range sm.blockHistory {
		if block.Winner >= SyndicateSeiryu && block.Winner <= SyndicateGenbu {
			wins[block.Winner]++
		}
	}
	
	// Update percentages
	for syndicate := SyndicateSeiryu; syndicate <= SyndicateGenbu; syndicate++ {
		sm.syndicates[syndicate].BlocksWon = wins[syndicate]
		sm.syndicates[syndicate].WinPercentage = float64(wins[syndicate]) / float64(totalBlocks) * 100.0
	}
}

// CheckDominanceThreshold returns true if a syndicate has won more than 35% of recent blocks
func (sm *SyndicateManager) CheckDominanceThreshold(syndicate SyndicateType) bool {
	sm.mu.RLock()
	defer sm.mu.RUnlock()
	
	stats, exists := sm.syndicates[syndicate]
	if !exists {
		return false
	}
	
	return stats.WinPercentage > 35.0
}

// GetMemberBySyndicate returns all members of a specific syndicate
func (sm *SyndicateManager) GetMembersBySyndicate(syndicate SyndicateType) ([]SyndicateMember, error) {
	stats, err := sm.GetSyndicateStats(syndicate)
	if err != nil {
		return nil, err
	}
	
	return stats.Members, nil
}