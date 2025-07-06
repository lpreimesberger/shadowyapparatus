package cmd

import (
	"fmt"
)

// Token constants following Bitcoin's design
const (
	// Maximum supply: 21 million SHADOW tokens
	MaxSupply = uint64(21000000)
	
	// Decimal places (8 like Bitcoin)
	DecimalPlaces = 8
	
	// Satoshi equivalent (smallest unit)
	SatoshisPerShadow = uint64(100000000) // 10^8
	
	// Total satoshis possible
	MaxSatoshis = MaxSupply * SatoshisPerShadow // 2.1 quadrillion
	
	// Initial block reward: 50 SHADOW = 5 billion satoshis
	InitialBlockReward = uint64(50) * SatoshisPerShadow
	
	// Halving interval: every 210,000 blocks (~4 years at 10 min blocks)
	HalvingInterval = uint64(210000)
	
	// Maximum number of halvings (after which reward becomes 0)
	MaxHalvings = 64
	
	// Target block time in seconds (10 minutes)
	TargetBlockTime = 600
	
	// Difficulty adjustment interval (every 2 weeks)
	DifficultyAdjustmentInterval = 2016
)

// Reward calculation errors
var (
	ErrInvalidBlockHeight = fmt.Errorf("invalid block height")
	ErrRewardOverflow     = fmt.Errorf("reward calculation overflow")
)

// BlockReward calculates the mining reward for a given block height
func CalculateBlockReward(height uint64) uint64 {
	// Calculate number of halvings that have occurred
	halvings := height / HalvingInterval
	
	// After maximum halvings, no more rewards
	if halvings >= MaxHalvings {
		return 0
	}
	
	// Calculate reward using bit shifting (equivalent to dividing by 2^halvings)
	// 50 SHADOW = 5,000,000,000 satoshis
	reward := InitialBlockReward >> halvings
	
	return reward
}

// CalculateTransactionFee calculates the fee for a transaction
func CalculateTransactionFee(txSize int, priorityFee uint64) uint64 {
	// Base fee: 1000 satoshis (0.00001 SHADOW)
	baseFee := uint64(1000)
	
	// Size fee: 100 satoshis per KB
	sizeFeePerKB := uint64(100)
	sizeInKB := uint64((txSize + 1023) / 1024) // Round up to nearest KB
	sizeFee := sizeInKB * sizeFeePerKB
	
	// Total fee
	totalFee := baseFee + sizeFee + priorityFee
	
	return totalFee
}

// GetTotalSupplyAtHeight calculates total SHADOW supply at given height
func GetTotalSupplyAtHeight(height uint64) uint64 {
	if height == 0 {
		// Genesis block has 1 SHADOW for bootstrap
		return 1 * SatoshisPerShadow
	}
	
	totalSupply := uint64(1 * SatoshisPerShadow) // Genesis bootstrap
	currentHeight := uint64(1) // Start after genesis
	
	for currentHeight <= height {
		// Calculate how many blocks left in this era
		blocksInEra := uint64(HalvingInterval)
		if currentHeight + HalvingInterval - 1 > height {
			blocksInEra = height - currentHeight + 1
		}
		
		// Add rewards for this era
		rewardPerBlock := CalculateBlockReward(currentHeight)
		totalSupply += blocksInEra * rewardPerBlock
		
		// Move to next era
		currentHeight += blocksInEra
	}
	
	return totalSupply
}

// GetInflationRate calculates the annual inflation rate at given height
func GetInflationRate(height uint64) float64 {
	if height == 0 {
		return 0.0
	}
	
	currentSupply := GetTotalSupplyAtHeight(height)
	if currentSupply == 0 {
		return 0.0
	}
	
	// Calculate blocks per year (assuming 10-minute blocks)
	blocksPerYear := uint64(365 * 24 * 6) // 525,600 blocks/year
	
	// Calculate supply after one year
	futureSupply := GetTotalSupplyAtHeight(height + blocksPerYear)
	
	// Calculate inflation rate
	inflationRate := float64(futureSupply - currentSupply) / float64(currentSupply)
	
	return inflationRate * 100.0 // Return as percentage
}

// FormatSatoshis converts satoshis to human-readable SHADOW amount
func FormatSatoshis(satoshis uint64) string {
	shadow := float64(satoshis) / float64(SatoshisPerShadow)
	return fmt.Sprintf("%.8f SHADOW", shadow)
}

// ParseShadow converts SHADOW amount string to satoshis
func ParseShadow(shadowStr string) (uint64, error) {
	var shadow float64
	n, err := fmt.Sscanf(shadowStr, "%f", &shadow)
	if err != nil || n != 1 {
		return 0, fmt.Errorf("invalid SHADOW amount: %s", shadowStr)
	}
	
	if shadow < 0 {
		return 0, fmt.Errorf("negative SHADOW amount not allowed")
	}
	
	if shadow > float64(MaxSupply) {
		return 0, fmt.Errorf("amount exceeds maximum supply")
	}
	
	satoshis := uint64(shadow * float64(SatoshisPerShadow))
	return satoshis, nil
}

// RewardSchedule represents a single era in the reward schedule
type RewardSchedule struct {
	Era           int     `json:"era"`
	StartBlock    uint64  `json:"start_block"`
	EndBlock      uint64  `json:"end_block"`
	RewardShadow  float64 `json:"reward_shadow"`
	RewardSatoshi uint64  `json:"reward_satoshi"`
	BlocksInEra   uint64  `json:"blocks_in_era"`
	TotalReward   uint64  `json:"total_reward_satoshi"`
	Years         float64 `json:"approximate_years"`
}

// GetRewardSchedule returns the complete reward schedule
func GetRewardSchedule() []RewardSchedule {
	var schedule []RewardSchedule
	
	for era := 0; era < 10; era++ { // Show first 10 eras
		startBlock := uint64(era) * HalvingInterval
		endBlock := startBlock + HalvingInterval - 1
		
		rewardSatoshi := CalculateBlockReward(startBlock)
		if rewardSatoshi == 0 {
			break
		}
		
		rewardShadow := float64(rewardSatoshi) / float64(SatoshisPerShadow)
		totalReward := HalvingInterval * rewardSatoshi
		years := float64(HalvingInterval) * TargetBlockTime / (365.25 * 24 * 3600)
		
		schedule = append(schedule, RewardSchedule{
			Era:           era + 1,
			StartBlock:    startBlock,
			EndBlock:      endBlock,
			RewardShadow:  rewardShadow,
			RewardSatoshi: rewardSatoshi,
			BlocksInEra:   HalvingInterval,
			TotalReward:   totalReward,
			Years:         years,
		})
	}
	
	return schedule
}

// NetworkStats provides current network statistics
type NetworkStats struct {
	CurrentHeight     uint64  `json:"current_height"`
	CurrentReward     uint64  `json:"current_reward_satoshi"`
	CurrentRewardShadow float64 `json:"current_reward_shadow"`
	TotalSupply       uint64  `json:"total_supply_satoshi"`
	TotalSupplyShadow float64 `json:"total_supply_shadow"`
	InflationRate     float64 `json:"inflation_rate_percent"`
	NextHalving       uint64  `json:"blocks_until_halving"`
	PercentMined      float64 `json:"percent_mined"`
}

// GetNetworkStats calculates current network statistics
func GetNetworkStats(currentHeight uint64) NetworkStats {
	currentReward := CalculateBlockReward(currentHeight)
	totalSupply := GetTotalSupplyAtHeight(currentHeight)
	inflationRate := GetInflationRate(currentHeight)
	
	// Calculate blocks until next halving
	nextHalvingHeight := ((currentHeight / HalvingInterval) + 1) * HalvingInterval
	blocksUntilHalving := nextHalvingHeight - currentHeight
	
	// Calculate percent of total supply mined
	percentMined := float64(totalSupply) / float64(MaxSatoshis) * 100.0
	
	return NetworkStats{
		CurrentHeight:       currentHeight,
		CurrentReward:       currentReward,
		CurrentRewardShadow: float64(currentReward) / float64(SatoshisPerShadow),
		TotalSupply:         totalSupply,
		TotalSupplyShadow:   float64(totalSupply) / float64(SatoshisPerShadow),
		InflationRate:       inflationRate,
		NextHalving:         blocksUntilHalving,
		PercentMined:        percentMined,
	}
}

// ValidateReward checks if a block reward is correct for given height
func ValidateReward(height uint64, claimedReward uint64) error {
	expectedReward := CalculateBlockReward(height)
	if claimedReward != expectedReward {
		return fmt.Errorf("invalid block reward: expected %d, got %d", 
			expectedReward, claimedReward)
	}
	return nil
}

// EstimateHalvingDate estimates when the next halving will occur
func EstimateHalvingDate(currentHeight uint64, avgBlockTime float64) (uint64, float64) {
	nextHalvingHeight := ((currentHeight / HalvingInterval) + 1) * HalvingInterval
	blocksRemaining := nextHalvingHeight - currentHeight
	
	// Estimate time in seconds
	timeRemaining := float64(blocksRemaining) * avgBlockTime
	
	return nextHalvingHeight, timeRemaining
}

// GetHalvingHistory returns historical halving events
func GetHalvingHistory() []map[string]interface{} {
	var history []map[string]interface{}
	
	for i := 0; i < 6; i++ { // First 6 halvings
		height := uint64(i) * HalvingInterval
		rewardBefore := CalculateBlockReward(height - 1)
		rewardAfter := CalculateBlockReward(height)
		
		if i == 0 {
			rewardBefore = InitialBlockReward
		}
		
		halving := map[string]interface{}{
			"halving_number":   i + 1,
			"block_height":     height,
			"reward_before":    float64(rewardBefore) / float64(SatoshisPerShadow),
			"reward_after":     float64(rewardAfter) / float64(SatoshisPerShadow),
			"reduction_percent": 50.0,
			"estimated_year":   2025 + (i * 4), // Rough estimate
		}
		
		history = append(history, halving)
	}
	
	return history
}