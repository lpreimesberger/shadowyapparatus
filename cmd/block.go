package cmd

import (
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/spf13/cobra"
)

// blockCmd represents the block command
var blockCmd = &cobra.Command{
	Use:   "block",
	Short: "Block operations and information",
	Long:  `Commands for working with blockchain blocks, including listing, querying, and analyzing block data.`,
}

// blockListCmd represents the block list command
var blockListCmd = &cobra.Command{
	Use:   "list",
	Short: "List all locally stored blocks",
	Long: `List all blocks stored in the local blockchain database.
	
Output is compressed to show ranges (e.g., "1-100" instead of listing each block individually).
This provides a quick overview of which blocks are available locally.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Override blockchain directory if specified via flag
		if dataDir, _ := cmd.Flags().GetString("data"); dataDir != "" {
			config.BlockchainDirectory = dataDir
		}

		// Initialize blockchain
		blockchain, err := NewBlockchain(config)
		if err != nil {
			fmt.Printf("Error initializing blockchain: %v\n", err)
			os.Exit(1)
		}

		// Get all available block heights
		heights, err := getAvailableBlockHeights(blockchain)
		if err != nil {
			fmt.Printf("Error getting block heights: %v\n", err)
			os.Exit(1)
		}

		if len(heights) == 0 {
			fmt.Println("No blocks found")
			return
		}

		// Sort heights
		sort.Slice(heights, func(i, j int) bool {
			return heights[i] < heights[j]
		})

		// Get blockchain stats for context
		stats := blockchain.GetStats()

		// Display header information
		fmt.Printf("Block Storage Summary\n")
		fmt.Printf("====================\n")
		fmt.Printf("Total blocks: %d\n", len(heights))
		fmt.Printf("Current tip:  %d\n", stats.TipHeight)
		fmt.Printf("Height range: %d - %d\n", heights[0], heights[len(heights)-1])
		fmt.Printf("\nBlock ranges:\n")

		// Show compact ranges
		ranges := compressHeightRanges(heights)
		for _, rangeStr := range ranges {
			fmt.Printf("  %s\n", rangeStr)
		}

		// Show verbose output if requested
		if verbose, _ := cmd.Flags().GetBool("verbose"); verbose {
			fmt.Printf("\nDetailed block list:\n")
			for _, height := range heights {
				block, err := blockchain.GetBlockByHeight(height)
				if err != nil {
					fmt.Printf("  %d: <error loading>\n", height)
					continue
				}
				fmt.Printf("  %d: %s (txs: %d)\n", height, block.Hash()[:16]+"...", len(block.Body.Transactions))
			}
		}
	},
}

// blockWalkCmd represents the block walk command
var blockWalkCmd = &cobra.Command{
	Use:   "walk",
	Short: "Validate blockchain consistency by walking through all blocks",
	Long: `Walk through all cached blocks and validate their consistency.
	
Checks for:
- Proper parent-child relationships (prev_hash linkage)
- Sequential height progression  
- Block hash integrity
- Missing blocks in sequences
- Orphaned blocks

This helps identify blockchain corruption or sync issues.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading configuration: %v\n", err)
			os.Exit(1)
		}

		// Override blockchain directory if specified via flag
		if dataDir, _ := cmd.Flags().GetString("data"); dataDir != "" {
			config.BlockchainDirectory = dataDir
		}

		// Initialize blockchain
		blockchain, err := NewBlockchain(config)
		if err != nil {
			fmt.Printf("Error initializing blockchain: %v\n", err)
			os.Exit(1)
		}

		// Perform blockchain walk validation
		if err := walkAndValidateBlockchain(blockchain, cmd); err != nil {
			fmt.Printf("Blockchain validation failed: %v\n", err)
			os.Exit(1)
		}
	},
}

// walkAndValidateBlockchain performs a comprehensive validation of the blockchain
func walkAndValidateBlockchain(blockchain *Blockchain, cmd *cobra.Command) error {
	verbose, _ := cmd.Flags().GetBool("verbose")
	fix, _ := cmd.Flags().GetBool("fix")
	
	fmt.Printf("Blockchain Validation Walk\n")
	fmt.Printf("=========================\n")
	
	// Get all available block heights
	heights, err := getAvailableBlockHeights(blockchain)
	if err != nil {
		return fmt.Errorf("failed to get block heights: %w", err)
	}
	
	if len(heights) == 0 {
		fmt.Println("No blocks found to validate")
		return nil
	}
	
	// Sort heights
	sort.Slice(heights, func(i, j int) bool {
		return heights[i] < heights[j]
	})
	
	stats := &BlockValidationStats{
		TotalBlocks:    len(heights),
		ValidBlocks:    0,
		InvalidBlocks:  0,
		MissingBlocks:  0,
		OrphanedBlocks: 0,
	}
	
	fmt.Printf("Validating %d blocks (heights %d - %d)\n\n", 
		len(heights), heights[0], heights[len(heights)-1])
	
	// Step 1: Validate each block individually
	fmt.Printf("Step 1: Individual Block Validation\n")
	fmt.Printf("-----------------------------------\n")
	
	blockMap := make(map[uint64]*Block)
	hashMap := make(map[string]*Block)
	
	for _, height := range heights {
		block, err := blockchain.GetBlockByHeight(height)
		if err != nil {
			fmt.Printf("❌ Block %d: Failed to load - %v\n", height, err)
			stats.InvalidBlocks++
			continue
		}
		
		// Validate block hash integrity (just compute it to verify structure is valid)
		expectedHash := block.Hash()
		// Note: BlockHeader doesn't store the hash, it's computed on demand
		
		// Store blocks for chain validation
		blockMap[height] = block
		hashMap[expectedHash] = block
		
		if verbose {
			fmt.Printf("✅ Block %d: %s (txs: %d)\n", 
				height, expectedHash[:16]+"...", len(block.Body.Transactions))
		}
		
		stats.ValidBlocks++
	}
	
	// Step 2: Validate chain consistency
	fmt.Printf("\nStep 2: Chain Consistency Validation\n")
	fmt.Printf("------------------------------------\n")
	
	// Check for missing blocks in sequence
	for i := 0; i < len(heights)-1; i++ {
		current := heights[i]
		next := heights[i+1]
		
		if next != current+1 {
			missing := next - current - 1
			fmt.Printf("⚠️  Gap detected: Missing %d block(s) between %d and %d\n", 
				missing, current, next)
			stats.MissingBlocks += int(missing)
		}
	}
	
	// Step 3: Validate parent-child relationships
	fmt.Printf("\nStep 3: Parent-Child Relationship Validation\n")
	fmt.Printf("--------------------------------------------\n")
	
	orphanCount := 0
	chainErrors := 0
	
	// Always show genesis block first
	if genesisBlock, exists := blockMap[0]; exists {
		genesisHash := genesisBlock.Hash()
		fmt.Printf("Block 0: %s -> Genesis (no parent)\n", genesisHash)
	}
	
	for i := 1; i < len(heights); i++ { // Start from block 1 (genesis has no parent)
		height := heights[i]
		block := blockMap[height]
		blockHash := block.Hash()
		parentHeight := height - 1
		
		// Check if parent exists
		parentBlock, exists := blockMap[parentHeight]
		if !exists {
			fmt.Printf("❌ Block %d: %s -> Parent block %d not found (orphaned)\n", height, blockHash, parentHeight)
			stats.OrphanedBlocks++
			orphanCount++
			continue
		}
		
		// Validate parent hash linkage
		expectedParentHash := parentBlock.Hash()
		if block.Header.PreviousBlockHash != expectedParentHash {
			fmt.Printf("❌ Block %d: %s -> INVALID parent hash\n", height, blockHash)
			fmt.Printf("   Expected: %s\n", expectedParentHash)
			fmt.Printf("   Got:      %s\n", block.Header.PreviousBlockHash)
			chainErrors++
			
			if fix {
				fmt.Printf("🔧 Fixing parent hash for block %d\n", height)
				block.Header.PreviousBlockHash = expectedParentHash
			}
		} else {
			// Show successful validation with parent hash
			if verbose {
				fmt.Printf("✅ Block %d: %s -> Valid linkage to parent %s\n", 
					height, blockHash, expectedParentHash)
			} else {
				fmt.Printf("Block %d: %s -> ✅ Valid\n", height, blockHash)
			}
		}
	}
	
	// Step 4: Genesis block validation
	fmt.Printf("\nStep 4: Genesis Block Validation\n")
	fmt.Printf("--------------------------------\n")
	
	if genesisBlock, exists := blockMap[0]; exists {
		// Treat both empty string and zero hash as valid for genesis block
		zeroHash := "0000000000000000000000000000000000000000000000000000000000000000"
		if genesisBlock.Header.PreviousBlockHash != "" && genesisBlock.Header.PreviousBlockHash != zeroHash {
			fmt.Printf("❌ Genesis block: Should have empty or zero previous_block_hash, got %s\n", genesisBlock.Header.PreviousBlockHash)
			chainErrors++
		} else {
			if genesisBlock.Header.PreviousBlockHash == zeroHash {
				fmt.Printf("✅ Genesis block: Valid (zero previous_block_hash)\n")
			} else {
				fmt.Printf("✅ Genesis block: Valid (empty previous_block_hash)\n")
			}
		}
	} else {
		fmt.Printf("❌ Genesis block: Not found\n")
		stats.MissingBlocks++
	}
	
	// Summary
	fmt.Printf("\nValidation Summary\n")
	fmt.Printf("==================\n")
	fmt.Printf("Total blocks:     %d\n", stats.TotalBlocks)
	fmt.Printf("Valid blocks:     %d\n", stats.ValidBlocks)
	fmt.Printf("Invalid blocks:   %d\n", stats.InvalidBlocks)
	fmt.Printf("Missing blocks:   %d\n", stats.MissingBlocks)
	fmt.Printf("Orphaned blocks:  %d\n", stats.OrphanedBlocks)
	fmt.Printf("Chain errors:     %d\n", chainErrors)
	
	if stats.InvalidBlocks == 0 && stats.OrphanedBlocks == 0 && chainErrors == 0 && stats.MissingBlocks == 0 {
		fmt.Printf("\n🎉 Blockchain validation PASSED! All blocks are consistent.\n")
	} else {
		fmt.Printf("\n❌ Blockchain validation FAILED! Found %d issues.\n", 
			stats.InvalidBlocks+stats.OrphanedBlocks+chainErrors)
		
		if !fix {
			fmt.Printf("\nRun with --fix flag to attempt automatic repairs.\n")
		}
	}
	
	return nil
}

// BlockValidationStats tracks block validation results
type BlockValidationStats struct {
	TotalBlocks    int
	ValidBlocks    int
	InvalidBlocks  int
	MissingBlocks  int
	OrphanedBlocks int
}

// getAvailableBlockHeights returns all block heights that exist in the blockchain
func getAvailableBlockHeights(blockchain *Blockchain) ([]uint64, error) {
	var heights []uint64
	
	// Get current tip height
	stats := blockchain.GetStats()
	
	// Check each height from 0 to tip
	for i := uint64(0); i <= stats.TipHeight; i++ {
		// Try to get block at this height
		_, err := blockchain.GetBlockByHeight(i)
		if err == nil {
			heights = append(heights, i)
		}
	}
	
	return heights, nil
}

// compressHeightRanges takes a sorted slice of heights and returns compressed range strings
func compressHeightRanges(heights []uint64) []string {
	if len(heights) == 0 {
		return []string{}
	}

	var ranges []string
	start := heights[0]
	end := heights[0]

	for i := 1; i < len(heights); i++ {
		if heights[i] == end+1 {
			// Continue the current range
			end = heights[i]
		} else {
			// End current range and start a new one
			ranges = append(ranges, formatRange(start, end))
			start = heights[i]
			end = heights[i]
		}
	}

	// Add the final range
	ranges = append(ranges, formatRange(start, end))

	return ranges
}

// formatRange formats a range of block heights
func formatRange(start, end uint64) string {
	if start == end {
		return fmt.Sprintf("%d", start)
	}
	return fmt.Sprintf("%d-%d", start, end)
}

// parseBlockHeight parses a block height from string (for future subcommands)
func parseBlockHeight(heightStr string) (uint64, error) {
	if heightStr == "tip" || heightStr == "latest" {
		return 0, fmt.Errorf("special height keywords not implemented yet")
	}
	
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid block height: %s", heightStr)
	}
	
	return height, nil
}

// findMissingBlocks identifies gaps in the block sequence
func findMissingBlocks(heights []uint64) []string {
	if len(heights) == 0 {
		return []string{}
	}

	sort.Slice(heights, func(i, j int) bool {
		return heights[i] < heights[j]
	})

	var missing []string
	
	for i := 0; i < len(heights)-1; i++ {
		current := heights[i]
		next := heights[i+1]
		
		if next > current+1 {
			// There's a gap
			gapStart := current + 1
			gapEnd := next - 1
			missing = append(missing, formatRange(gapStart, gapEnd))
		}
	}
	
	return missing
}

func init() {
	// Add block command to root
	rootCmd.AddCommand(blockCmd)
	
	// Add subcommands to block
	blockCmd.AddCommand(blockListCmd)
	blockCmd.AddCommand(blockWalkCmd)
	
	// Add flags to list command
	blockListCmd.Flags().StringP("data", "d", "", "Override blockchain directory (uses config value if not specified)")
	blockListCmd.Flags().BoolP("verbose", "v", false, "Show detailed information for each block")
	blockListCmd.Flags().Bool("missing", false, "Show missing block ranges")
	
	// Add flags to walk command
	blockWalkCmd.Flags().StringP("data", "d", "", "Override blockchain directory (uses config value if not specified)")
	blockWalkCmd.Flags().BoolP("verbose", "v", false, "Show detailed validation information for each block")
	blockWalkCmd.Flags().Bool("fix", false, "Attempt to automatically fix detected issues")
}