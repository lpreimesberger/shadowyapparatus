package cmd

import (
	"bufio"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var resetCmd = &cobra.Command{
	Use:   "reset",
	Short: "Reset blockchain node to fresh state with new genesis block",
	Long: `Reset completely removes all blockchain data and creates a fresh genesis block.
This will delete:
- All blockchain data (blocks, genesis)
- Plot database indexes (can be rebuilt)
- Log files
- Cached data

WARNING: This is destructive and cannot be undone!
Wallet files are preserved by default.`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration to get data directories
		config, err := loadConfig()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}

		// Show what will be deleted
		fmt.Println("🔄 BLOCKCHAIN RESET")
		fmt.Println("==================")
		fmt.Printf("This will delete ALL blockchain data from:\n")
		fmt.Printf("  • Blockchain directory: %s\n", config.BlockchainDirectory)
		fmt.Printf("  • Plot database: %s/plot-lookup\n", config.ScratchDirectory)
		fmt.Printf("  • Log files: %s\n", config.LoggingDirectory)
		fmt.Printf("  • Scratch data: %s\n", config.ScratchDirectory)
		fmt.Println()
		fmt.Println("⚠️  WARNING: This action cannot be undone!")
		fmt.Println("💰 Wallet files will be preserved")
		fmt.Println("📊 Plot files will be preserved")

		// Get confirmation
		if !confirmAction("Are you sure you want to reset the blockchain?") {
			fmt.Println("❌ Reset cancelled")
			return
		}

		// Perform reset
		fmt.Println("\n🧹 Resetting blockchain data...")
		
		// Remove blockchain directory
		if err := removeDirectory(config.BlockchainDirectory); err != nil {
			log.Printf("Warning: Failed to remove blockchain directory: %v", err)
		} else {
			fmt.Printf("✅ Removed blockchain directory: %s\n", config.BlockchainDirectory)
		}

		// Remove plot database
		plotDBPath := filepath.Join(config.ScratchDirectory, "plot-lookup")
		if err := removeDirectory(plotDBPath); err != nil {
			log.Printf("Warning: Failed to remove plot database: %v", err)
		} else {
			fmt.Printf("✅ Removed plot database: %s\n", plotDBPath)
		}

		// Remove log files
		if err := removeDirectory(config.LoggingDirectory); err != nil {
			log.Printf("Warning: Failed to remove log directory: %v", err)
		} else {
			fmt.Printf("✅ Removed log directory: %s\n", config.LoggingDirectory)
		}

		// Remove scratch directory contents (except plot files)
		if err := cleanScratchDirectory(config.ScratchDirectory); err != nil {
			log.Printf("Warning: Failed to clean scratch directory: %v", err)
		} else {
			fmt.Printf("✅ Cleaned scratch directory: %s\n", config.ScratchDirectory)
		}

		// Recreate necessary directories
		if err := createDirectory(config.BlockchainDirectory); err != nil {
			log.Printf("Warning: Failed to recreate blockchain directory: %v", err)
		}
		if err := createDirectory(config.LoggingDirectory); err != nil {
			log.Printf("Warning: Failed to recreate log directory: %v", err)
		}
		if err := createDirectory(config.ScratchDirectory); err != nil {
			log.Printf("Warning: Failed to recreate scratch directory: %v", err)
		}

		fmt.Println("\n✅ Blockchain reset completed!")
		fmt.Println("🚀 You can now start the node with a fresh genesis block")
		fmt.Println("💡 The node will create a new genesis block on first startup")
	},
}

func init() {
	rootCmd.AddCommand(resetCmd)
	
	// Add flags for more control
	resetCmd.Flags().Bool("include-wallets", false, "Also remove wallet files (DANGEROUS)")
	resetCmd.Flags().Bool("include-config", false, "Also reset configuration to defaults")
	resetCmd.Flags().Bool("force", false, "Skip confirmation prompts")
}

// confirmAction prompts the user for confirmation
func confirmAction(message string) bool {
	reader := bufio.NewReader(os.Stdin)
	fmt.Printf("\n%s (y/N): ", message)
	
	input, err := reader.ReadString('\n')
	if err != nil {
		return false
	}
	
	input = strings.TrimSpace(strings.ToLower(input))
	return input == "y" || input == "yes"
}

// removeDirectory safely removes a directory and all its contents
func removeDirectory(path string) error {
	if _, err := os.Stat(path); os.IsNotExist(err) {
		return nil // Directory doesn't exist, nothing to remove
	}
	return os.RemoveAll(path)
}

// createDirectory creates a directory if it doesn't exist
func createDirectory(path string) error {
	return os.MkdirAll(path, 0755)
}

// cleanScratchDirectory removes scratch directory contents but preserves plot files
func cleanScratchDirectory(scratchDir string) error {
	// Read directory contents
	entries, err := os.ReadDir(scratchDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil // Directory doesn't exist
		}
		return err
	}

	// Remove each entry except plot files
	for _, entry := range entries {
		entryPath := filepath.Join(scratchDir, entry.Name())
		
		// Skip plot files (they're valuable)
		if strings.HasSuffix(entry.Name(), ".dat") || strings.HasSuffix(entry.Name(), ".plot") {
			continue
		}
		
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entryPath, err)
		}
	}
	
	return nil
}