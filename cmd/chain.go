package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

func id2text(thisHash string) string {
	if thisHash == "2bb0b9cd9ba0a755c3a7a1364aa2536c487c780c0ca8c8a6ae3a9402d9e9271d" {
		return "testnet0"
	} else {
		return fmt.Sprintf("?? %s ??", thisHash)
	}
}

// chainCmd represents the chain command
var chainCmd = &cobra.Command{
	Use:   "chain",
	Short: "Show chain information",
	Long:  `Display chain information including chain ID derived from genesis block hash.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration (same as node command)
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

		// Get blockchain stats
		stats := blockchain.GetStats()

		// Generate chain ID from genesis hash
		chainID := stats.GenesisHash
		if chainID == "" {
			// Fallback: try to get genesis block directly
			if genesisBlock, err := blockchain.GetBlockByHeight(0); err == nil {
				chainID = genesisBlock.Hash()
			} else {
				chainID = "unknown"
			}
		}

		// Get additional chain information
		height := stats.TipHeight
		tipHash := stats.TipHash

		// Genesis block path
		genesisPath := filepath.Join(config.BlockchainDirectory, "genesis.json")

		// Display information
		fmt.Printf("Chain Information\n")
		fmt.Printf("=================\n")
		fmt.Printf("Chain ID:       %s\n", id2text(chainID))
		fmt.Printf("Height:         %d\n", height)
		fmt.Printf("Tip Hash:       %s\n", tipHash)
		fmt.Printf("Genesis Hash:   %s\n", stats.GenesisHash)
		fmt.Printf("Genesis Path:   %s\n", genesisPath)
		fmt.Printf("Blockchain Dir: %s\n", config.BlockchainDirectory)

		if chainID != "unknown" && len(chainID) > 16 {
			fmt.Printf("Chain ID (short): %s...\n", chainID[:16])
		}
	},
}

func init() {
	chainCmd.Flags().StringP("data", "d", "", "Override blockchain directory (uses config value if not specified)")
}
