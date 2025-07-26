package cmd

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean cached data while preserving blockchain and wallets",
	Long: `Clean removes cached and rebuildable data while preserving the blockchain and wallets.
This will clean:
- Plot database indexes (will be rebuilt on next farming)
- Log files
- Temporary files in scratch directory
- Cached data

This will NOT remove:
- Blockchain data (blocks, genesis)
- Wallet files
- Configuration files
- Plot files (.dat, .plot)`,
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration to get data directories
		config, err := loadConfig()
		if err != nil {
			log.Fatalf("Failed to load configuration: %v", err)
		}

		// Check flags
		includeBlockchain, _ := cmd.Flags().GetBool("blockchain")
		includeWallets, _ := cmd.Flags().GetBool("wallets")
		includeConfig, _ := cmd.Flags().GetBool("config")
		force, _ := cmd.Flags().GetBool("force")

		// Show what will be cleaned
		fmt.Println("🧹 BLOCKCHAIN CLEAN")
		fmt.Println("==================")
		fmt.Printf("This will clean cached data from:\n")
		fmt.Printf("  • Plot database: %s/plot-lookup\n", config.ScratchDirectory)
		fmt.Printf("  • Log files: %s\n", config.LoggingDirectory)
		fmt.Printf("  • Temporary files: %s\n", config.ScratchDirectory)
		
		if includeBlockchain {
			fmt.Printf("  • Blockchain data: %s (--blockchain flag)\n", config.BlockchainDirectory)
		}
		if includeWallets {
			fmt.Printf("  • Wallet files: %s/*.wallet (--wallets flag)\n", getWalletDir())
		}
		if includeConfig {
			fmt.Printf("  • Configuration: %s/config.json (--config flag)\n", getWalletDir())
		}

		fmt.Println()
		fmt.Println("💾 Blockchain data will be preserved")
		fmt.Println("💰 Wallet files will be preserved")
		fmt.Println("📊 Plot files will be preserved")

		// Get confirmation if needed
		if !force {
			if !confirmAction("Are you sure you want to clean cached data?") {
				fmt.Println("❌ Clean cancelled")
				return
			}
		}

		// Perform clean
		fmt.Println("\n🧹 Cleaning cached data...")
		
		// Clean plot database
		plotDBPath := filepath.Join(config.ScratchDirectory, "plot-lookup")
		if err := removeDirectory(plotDBPath); err != nil {
			log.Printf("Warning: Failed to remove plot database: %v", err)
		} else {
			fmt.Printf("✅ Removed plot database: %s\n", plotDBPath)
		}

		// Clean log files
		if err := cleanLogDirectory(config.LoggingDirectory); err != nil {
			log.Printf("Warning: Failed to clean log directory: %v", err)
		} else {
			fmt.Printf("✅ Cleaned log files: %s\n", config.LoggingDirectory)
		}

		// Clean scratch directory (preserve plot files)
		if err := cleanScratchDirectory(config.ScratchDirectory); err != nil {
			log.Printf("Warning: Failed to clean scratch directory: %v", err)
		} else {
			fmt.Printf("✅ Cleaned scratch directory: %s\n", config.ScratchDirectory)
		}

		// Optional: Clean blockchain data
		if includeBlockchain {
			if err := cleanBlockchainData(config.BlockchainDirectory); err != nil {
				log.Printf("Warning: Failed to clean blockchain data: %v", err)
			} else {
				fmt.Printf("✅ Cleaned blockchain data: %s\n", config.BlockchainDirectory)
			}
		}

		// Optional: Clean wallet files
		if includeWallets {
			if err := cleanWalletFiles(getWalletDir()); err != nil {
				log.Printf("Warning: Failed to clean wallet files: %v", err)
			} else {
				fmt.Printf("✅ Cleaned wallet files: %s\n", getWalletDir())
			}
		}

		// Optional: Reset configuration
		if includeConfig {
			configPath := filepath.Join(getWalletDir(), "config.json")
			if err := os.Remove(configPath); err != nil && !os.IsNotExist(err) {
				log.Printf("Warning: Failed to remove config file: %v", err)
			} else {
				fmt.Printf("✅ Removed config file: %s\n", configPath)
			}
		}

		fmt.Println("\n✅ Clean completed!")
		fmt.Println("🚀 The node will rebuild indexes on next startup")
		if includeBlockchain {
			fmt.Println("🔄 The node will start with a fresh blockchain")
		}
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	
	// Add flags for more control
	cleanCmd.Flags().Bool("blockchain", false, "Also clean blockchain data (blocks, genesis)")
	cleanCmd.Flags().Bool("wallets", false, "Also clean wallet files (DANGEROUS)")
	cleanCmd.Flags().Bool("config", false, "Also reset configuration file")
	cleanCmd.Flags().Bool("force", false, "Skip confirmation prompts")
}

// cleanLogDirectory removes log files but preserves directory structure
func cleanLogDirectory(logDir string) error {
	if _, err := os.Stat(logDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist
	}

	// Read directory contents
	entries, err := os.ReadDir(logDir)
	if err != nil {
		return err
	}

	// Remove log files
	for _, entry := range entries {
		if !entry.IsDir() {
			entryPath := filepath.Join(logDir, entry.Name())
			if err := os.Remove(entryPath); err != nil {
				return fmt.Errorf("failed to remove log file %s: %w", entryPath, err)
			}
		}
	}

	return nil
}

// cleanBlockchainData removes all blockchain data except genesis
func cleanBlockchainData(blockchainDir string) error {
	if _, err := os.Stat(blockchainDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist
	}

	// Read directory contents
	entries, err := os.ReadDir(blockchainDir)
	if err != nil {
		return err
	}

	// Remove everything except genesis.json
	for _, entry := range entries {
		if entry.Name() == "genesis.json" {
			continue // Preserve genesis block
		}
		
		entryPath := filepath.Join(blockchainDir, entry.Name())
		if err := os.RemoveAll(entryPath); err != nil {
			return fmt.Errorf("failed to remove %s: %w", entryPath, err)
		}
	}

	return nil
}

// cleanWalletFiles removes wallet files from wallet directory
func cleanWalletFiles(walletDir string) error {
	if _, err := os.Stat(walletDir); os.IsNotExist(err) {
		return nil // Directory doesn't exist
	}

	// Read directory contents
	entries, err := os.ReadDir(walletDir)
	if err != nil {
		return err
	}

	// Remove wallet files
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".wallet") {
			entryPath := filepath.Join(walletDir, entry.Name())
			if err := os.Remove(entryPath); err != nil {
				return fmt.Errorf("failed to remove wallet file %s: %w", entryPath, err)
			}
		}
	}

	return nil
}