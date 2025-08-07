package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
	"golang.org/x/crypto/sha3"
)

const (
	// Shadowy address format constants
	AddressVersion   = 0x42 // 'S' for Shadowy
	AddressChecksumLen = 4   // 4-byte checksum
	AddressLen = 1 + 20 + AddressChecksumLen // version + hash + checksum = 25 bytes
	
	// Wallet file constants
	DefaultWalletDir = ".shadowy"
	WalletFileExt   = ".wallet"
)

var (
	walletDir string
)

type WalletFile struct {
	Name       string    `json:"name"`
	Address    string    `json:"address"`
	PrivateKey string    `json:"private_key"` // Version 1: full private key, Version 2+: seed
	PublicKey  string    `json:"public_key"`
	Identifier string    `json:"identifier"`
	CreatedAt  time.Time `json:"created_at"`
	Version    int       `json:"version"`
}

// WalletBalance represents the balance information for a wallet
type WalletBalance struct {
	Address           string  `json:"address"`
	ConfirmedBalance  uint64  `json:"confirmed_balance_satoshi"`
	PendingBalance    uint64  `json:"pending_balance_satoshi"`
	TotalReceived     uint64  `json:"total_received_satoshi"`
	TotalSent         uint64  `json:"total_sent_satoshi"`
	TransactionCount  int     `json:"transaction_count"`
	LastActivity      *time.Time `json:"last_activity,omitempty"`
	
	// Human-readable amounts
	ConfirmedShadow   float64 `json:"confirmed_shadow"`
	PendingShadow     float64 `json:"pending_shadow"`
	TotalReceivedShadow float64 `json:"total_received_shadow"`
	TotalSentShadow   float64 `json:"total_sent_shadow"`
}

// TransactionReference represents a transaction involving the wallet
type TransactionReference struct {
	TxHash    string    `json:"tx_hash"`
	BlockHeight uint64  `json:"block_height,omitempty"`
	Timestamp time.Time `json:"timestamp"`
	Amount    int64     `json:"amount_satoshi"` // Positive for received, negative for sent
	Type      string    `json:"type"`           // "received", "sent", "coinbase"
	Confirmed bool      `json:"confirmed"`
}

var walletCmd = &cobra.Command{
	Use:   "wallet",
	Short: "Wallet operations for Shadowy addresses",
	Long: `Generate and manage Shadowy post-quantum wallet addresses.
Supports address generation, validation, and key management.`,
}

var generateCmd = &cobra.Command{
	Use:   "generate [name]",
	Short: "Generate a new Shadowy wallet and save to file",
	Long: `Generate a new ML-DSA-87 key pair, derive a Shadowy address, and save to wallet file.
If no name is provided, generates a timestamped wallet name.`,
	Args: cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var walletName string
		if len(args) > 0 {
			walletName = args[0]
		} else {
			walletName = "wallet_" + time.Now().UTC().Format("20060102_150405")
		}
		
		keyPair, err := GenerateKeyPair()
		if err != nil {
			fmt.Printf("Error generating key pair: %v\n", err)
			os.Exit(1)
		}
		
		address := DeriveAddress(keyPair.PublicKey[:])
		
		wallet := WalletFile{
			Name:       walletName,
			Address:    address,
			PrivateKey: keyPair.SeedHex(), // Store seed instead of full private key
			PublicKey:  keyPair.PublicKeyHex(),
			Identifier: keyPair.IdentifierHex(),
			CreatedAt:  time.Now().UTC(),
			Version:    2, // Increment version to indicate seed-based storage
		}
		
		walletPath, err := saveWallet(wallet)
		if err != nil {
			fmt.Printf("Error saving wallet: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Wallet Name: %s\n", wallet.Name)
		fmt.Printf("Address:     %s\n", wallet.Address)
		fmt.Printf("Identifier:  %s\n", wallet.Identifier)
		fmt.Printf("Saved to:    %s\n", walletPath)
	},
}

var validateCmd = &cobra.Command{
	Use:   "validate [address]",
	Short: "Validate a Shadowy address",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		address := args[0]
		
		if IsValidAddress(address) {
			fmt.Printf("✓ Address %s is valid\n", address)
			os.Exit(0)
		} else {
			fmt.Printf("✗ Address %s is invalid\n", address)
			os.Exit(1)
		}
	},
}

var fromKeyCmd = &cobra.Command{
	Use:   "from-key [private-key-hex] [name]",
	Short: "Import wallet from existing private key",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		privKeyHex := args[0]
		
		var walletName string
		if len(args) > 1 {
			walletName = args[1]
		} else {
			walletName = "imported_" + time.Now().UTC().Format("20060102_150405")
		}
		
		if len(privKeyHex) != PrivateKeySize*2 {
			fmt.Printf("Error: Private key must be %d hex characters (%d bytes)\n", 
				PrivateKeySize*2, PrivateKeySize)
			os.Exit(1)
		}
		
		privKeyBytes, err := hex.DecodeString(privKeyHex)
		if err != nil {
			fmt.Printf("Error: Invalid hex private key: %v\n", err)
			os.Exit(1)
		}
		
		keyPair, err := reconstructKeyPair([PrivateKeySize]byte(privKeyBytes))
		if err != nil {
			fmt.Printf("Error reconstructing key pair: %v\n", err)
			os.Exit(1)
		}
		
		address := DeriveAddress(keyPair.PublicKey[:])
		
		wallet := WalletFile{
			Name:       walletName,
			Address:    address,
			PrivateKey: keyPair.SeedHex(), // Store seed instead of full private key
			PublicKey:  keyPair.PublicKeyHex(),
			Identifier: keyPair.IdentifierHex(),
			CreatedAt:  time.Now().UTC(),
			Version:    2, // Increment version to indicate seed-based storage
		}
		
		walletPath, err := saveWallet(wallet)
		if err != nil {
			fmt.Printf("Error saving wallet: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Wallet Name: %s\n", wallet.Name)
		fmt.Printf("Address:     %s\n", wallet.Address)
		fmt.Printf("Identifier:  %s\n", wallet.Identifier)
		fmt.Printf("Saved to:    %s\n", walletPath)
	},
}

var listCmd = &cobra.Command{
	Use:   "list",
	Short: "List all saved wallets",
	Run: func(cmd *cobra.Command, args []string) {
		wallets, err := listWallets()
		if err != nil {
			fmt.Printf("Error listing wallets: %v\n", err)
			os.Exit(1)
		}
		
		if len(wallets) == 0 {
			fmt.Printf("No wallets found in %s\n", getWalletDir())
			return
		}
		
		fmt.Printf("Found %d wallet(s) in %s:\n\n", len(wallets), getWalletDir())
		for i, wallet := range wallets {
			fmt.Printf("%d. %s\n", i+1, wallet.Name)
			fmt.Printf("   Address:    %s\n", wallet.Address)
			fmt.Printf("   Created:    %s\n", wallet.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
			fmt.Printf("   Identifier: %s\n", wallet.Identifier)
			fmt.Printf("\n")
		}
	},
}

var balanceCmd = &cobra.Command{
	Use:   "balance [address]",
	Short: "Check the balance of any Shadowy address",
	Long: `Check the balance and transaction history of any Shadowy address.
Scans the blockchain to calculate confirmed balance and recent activity.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		address := args[0]
		
		// Validate address format
		if !IsValidAddress(address) {
			fmt.Printf("Error: Invalid Shadowy address format: %s\n", address)
			os.Exit(1)
		}
		
		fmt.Printf("╔═══════════════════════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║                              ADDRESS BALANCE                                   ║\n")
		fmt.Printf("╠═══════════════════════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║ Address:     %-64s ║\n", address)
		fmt.Printf("╚═══════════════════════════════════════════════════════════════════════════════╝\n\n")
		
		// Get blockchain directory override if specified
		blockchainDir, _ := cmd.Flags().GetString("data")
		
		// Calculate and display balance
		fmt.Printf("Calculating balance... ")
		balance, err := calculateWalletBalanceWithDir(address, blockchainDir)
		if err != nil {
			fmt.Printf("Error calculating balance: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("✓\n\n")
		
		fmt.Printf("╔═══════════════════════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║                                BALANCE SUMMARY                                ║\n")
		fmt.Printf("╠═══════════════════════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║ Confirmed Balance:    %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.ConfirmedShadow, balance.ConfirmedBalance)
		fmt.Printf("║ Pending Balance:      %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.PendingShadow, balance.PendingBalance)
		fmt.Printf("║                                                                               ║\n")
		fmt.Printf("║ Total Received:       %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.TotalReceivedShadow, balance.TotalReceived)
		fmt.Printf("║ Total Sent:           %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.TotalSentShadow, balance.TotalSent)
		fmt.Printf("║                                                                               ║\n")
		fmt.Printf("║ Transaction Count:    %-59d ║\n", balance.TransactionCount)
		
		if balance.LastActivity != nil {
			fmt.Printf("║ Last Activity:        %-59s ║\n", 
				balance.LastActivity.Format("2006-01-02 15:04:05 UTC"))
		} else {
			fmt.Printf("║ Last Activity:        %-59s ║\n", "No transactions found")
		}
		fmt.Printf("╚═══════════════════════════════════════════════════════════════════════════════╝\n")
		
		// Show recent transactions if any exist
		if balance.TransactionCount > 0 {
			fmt.Printf("\n")
			fmt.Printf("╔═══════════════════════════════════════════════════════════════════════════════╗\n")
			fmt.Printf("║                              RECENT TRANSACTIONS                              ║\n")
			fmt.Printf("╚═══════════════════════════════════════════════════════════════════════════════╝\n")
			
			transactions, err := getWalletTransactions(address, 5)
			if err != nil {
				fmt.Printf("Error loading transactions: %v\n", err)
			} else if len(transactions) > 0 {
				fmt.Printf("\n%-16s %-12s %-19s %-20s %-10s\n", 
					"HASH", "TYPE", "TIMESTAMP", "AMOUNT (SHADOW)", "BLOCK")
				fmt.Printf("─────────────────────────────────────────────────────────────────────────────\n")
				
				for _, tx := range transactions {
					hashShort := tx.TxHash
					if len(hashShort) > 16 {
						hashShort = hashShort[:16]
					}
					
					amountShadow := float64(tx.Amount) / float64(SatoshisPerShadow)
					amountStr := fmt.Sprintf("%+.8f", amountShadow)
					
					blockStr := ""
					if tx.BlockHeight > 0 {
						blockStr = fmt.Sprintf("#%d", tx.BlockHeight)
					} else {
						blockStr = "pending"
					}
					
					fmt.Printf("%-16s %-12s %-19s %-20s %-10s\n",
						hashShort, 
						tx.Type, 
						tx.Timestamp.Format("2006-01-02 15:04:05"),
						amountStr,
						blockStr)
				}
			}
		}
	},
}

var showCmd = &cobra.Command{
	Use:   "show [wallet-name]",
	Short: "Show details and balance of a specific wallet",
	Long: `Show comprehensive wallet information including balance, transaction history, and account details.
Scans the blockchain to calculate confirmed balance and recent transaction activity.`,
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		walletName := args[0]
		
		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}
		
		fmt.Printf("╔═══════════════════════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║                                WALLET DETAILS                                 ║\n")
		fmt.Printf("╠═══════════════════════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║ Name:        %-64s ║\n", wallet.Name)
		fmt.Printf("║ Address:     %-64s ║\n", wallet.Address)
		fmt.Printf("║ Identifier:  %-64s ║\n", wallet.Identifier)
		fmt.Printf("║ Created:     %-64s ║\n", wallet.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
		fmt.Printf("║ Version:     %-64d ║\n", wallet.Version)
		fmt.Printf("╚═══════════════════════════════════════════════════════════════════════════════╝\n\n")
		
		// Calculate and display balance
		fmt.Printf("Calculating balance... ")
		balance, err := calculateWalletBalance(wallet.Address)
		if err != nil {
			fmt.Printf("Error calculating balance: %v\n", err)
			// Continue without balance information
			return
		}
		fmt.Printf("✓\n\n")
		
		fmt.Printf("╔═══════════════════════════════════════════════════════════════════════════════╗\n")
		fmt.Printf("║                                BALANCE SUMMARY                                ║\n")
		fmt.Printf("╠═══════════════════════════════════════════════════════════════════════════════╣\n")
		fmt.Printf("║ Confirmed Balance:    %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.ConfirmedShadow, balance.ConfirmedBalance)
		fmt.Printf("║ Pending Balance:      %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.PendingShadow, balance.PendingBalance)
		fmt.Printf("║                                                                               ║\n")
		fmt.Printf("║ Total Received:       %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.TotalReceivedShadow, balance.TotalReceived)
		fmt.Printf("║ Total Sent:           %15.8f SHADOW (%20d satoshis) ║\n", 
			balance.TotalSentShadow, balance.TotalSent)
		fmt.Printf("║                                                                               ║\n")
		fmt.Printf("║ Transaction Count:    %-59d ║\n", balance.TransactionCount)
		
		if balance.LastActivity != nil {
			fmt.Printf("║ Last Activity:        %-59s ║\n", 
				balance.LastActivity.Format("2006-01-02 15:04:05 UTC"))
		} else {
			fmt.Printf("║ Last Activity:        %-59s ║\n", "No transactions found")
		}
		fmt.Printf("╚═══════════════════════════════════════════════════════════════════════════════╝\n")
		
		// Show recent transactions if any exist
		if balance.TransactionCount > 0 {
			fmt.Printf("\n")
			fmt.Printf("╔═══════════════════════════════════════════════════════════════════════════════╗\n")
			fmt.Printf("║                              RECENT TRANSACTIONS                              ║\n")
			fmt.Printf("╚═══════════════════════════════════════════════════════════════════════════════╝\n")
			
			transactions, err := getWalletTransactions(wallet.Address, 10)
			if err != nil {
				fmt.Printf("Error loading transactions: %v\n", err)
			} else if len(transactions) == 0 {
				fmt.Printf("No transactions found.\n")
			} else {
				fmt.Printf("\n%-16s %-12s %-19s %-20s %-10s\n", 
					"HASH", "TYPE", "TIMESTAMP", "AMOUNT (SHADOW)", "BLOCK")
				fmt.Printf("─────────────────────────────────────────────────────────────────────────────\n")
				
				for _, tx := range transactions {
					hashShort := tx.TxHash
					if len(hashShort) > 16 {
						hashShort = hashShort[:16]
					}
					
					amountShadow := float64(tx.Amount) / float64(SatoshisPerShadow)
					amountStr := fmt.Sprintf("%+.8f", amountShadow)
					
					blockStr := ""
					if tx.BlockHeight > 0 {
						blockStr = fmt.Sprintf("#%d", tx.BlockHeight)
					} else {
						blockStr = "pending"
					}
					
					fmt.Printf("%-16s %-12s %-19s %-20s %-10s\n",
						hashShort, 
						tx.Type, 
						tx.Timestamp.Format("2006-01-02 15:04:05"),
						amountStr,
						blockStr)
				}
				
				if len(transactions) == 10 {
					fmt.Printf("\n(Showing last 10 transactions. Use 'shadowy tx history %s' for complete history)\n", wallet.Address)
				}
			}
		}
		
		fmt.Printf("\n💡 Tip: Use 'shadowy tx send <amount> <to-address> %s' to send SHADOW from this wallet\n", wallet.Name)
	},
}

// Token commands
var tokensCmd = &cobra.Command{
	Use:   "tokens",
	Short: "Token operations for the wallet",
	Long:  `Manage token balances, trust settings, and token operations.`,
}

var tokenBalancesCmd = &cobra.Command{
	Use:   "balances [wallet-name]",
	Short: "Show token balances for a wallet",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var walletName string
		if len(args) > 0 {
			walletName = args[0]
		} else {
			// Use first available wallet
			wallets, err := listWallets()
			if err != nil || len(wallets) == 0 {
				fmt.Printf("Error: No wallets found\n")
				os.Exit(1)
			}
			walletName = wallets[0].Name
		}

		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}

		showTokenBalances := func() error {
			// Load blockchain to get token state
			config, err := loadConfig()
			if err != nil {
				return fmt.Errorf("failed to load config: %w", err)
			}

			blockchain, err := NewBlockchain(config)
			if err != nil {
				return fmt.Errorf("failed to initialize blockchain: %w", err)
			}

			tokenState := blockchain.GetTokenState()
			balances, err := tokenState.GetAllTokenBalances(wallet.Address)
			if err != nil {
				return fmt.Errorf("failed to get token balances: %w", err)
			}

			// Initialize trust manager
			trustManager, err := NewTokenTrustManager(wallet.Name, getWalletDir())
			if err != nil {
				return fmt.Errorf("failed to initialize token trust manager: %w", err)
			}

			fmt.Printf("Token Balances for Wallet: %s\n", wallet.Name)
			fmt.Printf("Address: %s\n", wallet.Address)
			fmt.Printf("==================================================\n\n")

			if len(balances) == 0 {
				fmt.Printf("No token balances found.\n")
				return nil
			}

			// Show each token balance with trust info
			for _, balance := range balances {
				trust := trustManager.GetTokenTrust(balance.TokenID)
				
				// Update metadata cache if we have current info
				if balance.TokenInfo != nil {
					trustManager.UpdateTokenMetadata(balance.TokenID, 
						balance.TokenInfo.Name, balance.TokenInfo.Ticker, balance.TokenInfo.Creator)
				}

				fmt.Printf("Token: %s (%s)\n", balance.TokenInfo.Name, balance.TokenInfo.Ticker)
				fmt.Printf("  Token ID: %s\n", balance.TokenID)
				fmt.Printf("  Balance:  %d\n", balance.Balance)
				fmt.Printf("  Creator:  %s\n", balance.TokenInfo.Creator)
				
				// Trust status with warning for unknown tokens
				switch trust.TrustLevel {
				case TrustUnknown:
					fmt.Printf("  Trust:    ⚠️  UNKNOWN (Score: %.1f/10)\n", trust.TrackerScore*10)
				case TrustBanned:
					fmt.Printf("  Trust:    🚫 BANNED\n")
				case TrustAccepted:
					fmt.Printf("  Trust:    ✅ ACCEPTED (Score: %.1f/10)\n", trust.TrackerScore*10)
				case TrustVerified:
					fmt.Printf("  Trust:    🔒 VERIFIED (Score: %.1f/10)\n", trust.TrackerScore*10)
				}
				
				if trust.Notes != "" {
					fmt.Printf("  Notes:    %s\n", trust.Notes)
				}
				fmt.Printf("\n")
			}

			// Show unknown tokens warning
			unknownCount := 0
			for _, balance := range balances {
				trust := trustManager.GetTokenTrust(balance.TokenID)
				if trust.TrustLevel == TrustUnknown {
					unknownCount++
				}
			}

			if unknownCount > 0 {
				fmt.Printf("⚠️  WARNING: %d unknown tokens detected!\n", unknownCount)
				fmt.Printf("Use 'wallet tokens accept <token-id>' to trust tokens\n")
				fmt.Printf("Use 'wallet tokens ban <token-id>' to hide unwanted tokens\n")
			}

			return nil
		}

		if err := showTokenBalances(); err != nil {
			fmt.Printf("Error: %v\n", err)
			os.Exit(1)
		}
	},
}

var tokenAcceptCmd = &cobra.Command{
	Use:   "accept <token-id> [wallet-name]",
	Short: "Accept a token (mark as trusted)",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		tokenID := args[0]
		
		var walletName string
		if len(args) > 1 {
			walletName = args[1]
		} else {
			// Use first available wallet
			wallets, err := listWallets()
			if err != nil || len(wallets) == 0 {
				fmt.Printf("Error: No wallets found\n")
				os.Exit(1)
			}
			walletName = wallets[0].Name
		}

		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}

		// Initialize trust manager
		trustManager, err := NewTokenTrustManager(wallet.Name, getWalletDir())
		if err != nil {
			fmt.Printf("Error initializing token trust manager: %v\n", err)
			os.Exit(1)
		}

		// Get token info from blockchain
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}

		blockchain, err := NewBlockchain(config)
		if err != nil {
			fmt.Printf("Error initializing blockchain: %v\n", err)
			os.Exit(1)
		}

		tokenState := blockchain.GetTokenState()
		metadata, err := tokenState.GetTokenInfo(tokenID)
		if err != nil {
			fmt.Printf("Error: Token %s not found\n", tokenID)
			os.Exit(1)
		}

		// Update metadata and accept token
		if err := trustManager.UpdateTokenMetadata(tokenID, metadata.Name, metadata.Ticker, metadata.Creator); err != nil {
			fmt.Printf("Error updating token metadata: %v\n", err)
			os.Exit(1)
		}

		notes, _ := cmd.Flags().GetString("notes")
		if err := trustManager.AcceptToken(tokenID, notes); err != nil {
			fmt.Printf("Error accepting token: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("✅ Token accepted successfully!\n")
		fmt.Printf("Token: %s (%s)\n", metadata.Name, metadata.Ticker)
		fmt.Printf("ID: %s\n", tokenID)
		if notes != "" {
			fmt.Printf("Notes: %s\n", notes)
		}
	},
}

var tokenBanCmd = &cobra.Command{
	Use:   "ban <token-id> [wallet-name]",
	Short: "Ban a token (hide from wallet)",
	Args:  cobra.RangeArgs(1, 2),
	Run: func(cmd *cobra.Command, args []string) {
		tokenID := args[0]
		
		var walletName string
		if len(args) > 1 {
			walletName = args[1]
		} else {
			// Use first available wallet
			wallets, err := listWallets()
			if err != nil || len(wallets) == 0 {
				fmt.Printf("Error: No wallets found\n")
				os.Exit(1)
			}
			walletName = wallets[0].Name
		}

		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}

		// Initialize trust manager
		trustManager, err := NewTokenTrustManager(wallet.Name, getWalletDir())
		if err != nil {
			fmt.Printf("Error initializing token trust manager: %v\n", err)
			os.Exit(1)
		}

		reason, _ := cmd.Flags().GetString("reason")
		if err := trustManager.BanToken(tokenID, reason); err != nil {
			fmt.Printf("Error banning token: %v\n", err)
			os.Exit(1)
		}

		fmt.Printf("🚫 Token banned successfully!\n")
		fmt.Printf("ID: %s\n", tokenID)
		if reason != "" {
			fmt.Printf("Reason: %s\n", reason)
		}
	},
}

var tokenMeltCmd = &cobra.Command{
	Use:   "melt <token-id> <amount> [wallet-name]",
	Short: "Melt tokens back to SHADOW (destroys tokens permanently)",
	Long: `Melt (burn) tokens to reclaim locked SHADOW. 

⚠️  WARNING: This action is IRREVERSIBLE! ⚠️
Melted tokens are permanently destroyed and cannot be recovered.
You will receive the proportional amount of locked SHADOW.

Example: wallet tokens melt abc123... 10.5 my-wallet`,
	Args: cobra.RangeArgs(2, 3),
	Run: func(cmd *cobra.Command, args []string) {
		tokenID := args[0]
		amountStr := args[1]
		
		// Parse amount
		amount, err := strconv.ParseFloat(amountStr, 64)
		if err != nil || amount <= 0 {
			fmt.Printf("Error: Invalid amount '%s'. Must be a positive number.\n", amountStr)
			os.Exit(1)
		}
		
		// Get wallet name
		walletName := ""
		if len(args) > 2 {
			walletName = args[2]
		} else {
			wallets, err := listWallets()
			if err != nil || len(wallets) == 0 {
				fmt.Printf("No wallets found. Please create a wallet first.\n")
				os.Exit(1)
			}
			walletName = wallets[0].Name
		}
		
		// Load wallet
		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}
		
		// Initialize blockchain to get token information
		config, err := loadConfig()
		if err != nil {
			fmt.Printf("Error loading config: %v\n", err)
			os.Exit(1)
		}
		blockchain, err := NewBlockchain(config)
		if err != nil {
			fmt.Printf("Error initializing blockchain: %v\n", err)
			os.Exit(1)
		}

		tokenState := blockchain.GetTokenState()
		tokenInfo, err := tokenState.GetTokenInfo(tokenID)
		if err != nil {
			fmt.Printf("Error: Token %s not found\n", tokenID)
			os.Exit(1)
		}
		
		// Get current balance
		balance, err := tokenState.GetTokenBalance(tokenID, wallet.Address)
		if err != nil {
			balance = 0 // No balance if error
		}
		
		// Convert from base units to display units
		var currentBalance float64
		if tokenInfo.Decimals > 0 {
			divisor := float64(1)
			for i := uint8(0); i < tokenInfo.Decimals; i++ {
				divisor *= 10
			}
			currentBalance = float64(balance) / divisor
		} else {
			currentBalance = float64(balance)
		}
		
		// Check sufficient balance
		if amount > currentBalance {
			fmt.Printf("Error: Insufficient balance. You have %.8f %s, trying to melt %.8f\n", 
				currentBalance, tokenInfo.Ticker, amount)
			os.Exit(1)
		}
		
		// Calculate SHADOW to be received
		shadowPerToken := float64(tokenInfo.LockAmount) / 100000000.0 // Convert satoshi to SHADOW
		shadowToReceive := amount * shadowPerToken
		
		// Show scary warning
		fmt.Printf("\n🔥 TOKEN MELT WARNING 🔥\n")
		fmt.Printf("═══════════════════════════════════════════════════\n")
		fmt.Printf("You are about to PERMANENTLY DESTROY tokens!\n\n")
		fmt.Printf("Token:           %s (%s)\n", tokenInfo.Name, tokenInfo.Ticker)
		fmt.Printf("Amount to melt:  %.8f %s\n", amount, tokenInfo.Ticker)
		fmt.Printf("SHADOW received: %.8f SHADOW\n", shadowToReceive)
		fmt.Printf("Your balance:    %.8f %s\n", currentBalance, tokenInfo.Ticker)
		fmt.Printf("Remaining:       %.8f %s\n", currentBalance-amount, tokenInfo.Ticker)
		fmt.Printf("\n⚠️  THIS ACTION CANNOT BE UNDONE! ⚠️\n")
		fmt.Printf("Melted tokens are PERMANENTLY DESTROYED!\n")
		fmt.Printf("═══════════════════════════════════════════════════\n")
		
		// Require explicit confirmation
		fmt.Printf("\nType 'MELT' (all caps) to confirm: ")
		var confirmation string
		fmt.Scanln(&confirmation)
		
		if confirmation != "MELT" {
			fmt.Printf("Melt cancelled.\n")
			os.Exit(0)
		}
		
		// Convert amount to base units
		var amountTokenUnits uint64
		if tokenInfo.Decimals > 0 {
			multiplier := uint64(1)
			for i := uint8(0); i < tokenInfo.Decimals; i++ {
				multiplier *= 10
			}
			amountTokenUnits = uint64(amount * float64(multiplier))
		} else {
			amountTokenUnits = uint64(amount)
		}
		
		// Create transaction
		tx := NewTransaction()
		tx.AddOutput(wallet.Address, 1) // Minimal SHADOW output
		tx.AddTokenMelt(tokenID, amountTokenUnits, wallet.Address)
		
		// Add placeholder input
		placeholderTxHash := "0000000000000000000000000000000000000000000000000000000000000000"
		tx.AddInput(placeholderTxHash, 0)
		
		// Sign transaction
		signedTx, err := SignTransactionWithWallet(tx, wallet)
		if err != nil {
			fmt.Printf("Error signing transaction: %v\n", err)
			os.Exit(1)
		}
		
		// For CLI commands, we output the transaction for manual submission
		fmt.Printf("\n🔥 Token melt transaction created successfully!\n")
		fmt.Printf("Transaction Hash: %s\n", signedTx.TxHash)
		fmt.Printf("\nTo submit this transaction, use the 'shadowy wallet send-raw' command with the following transaction data:\n")
		
		// Output the transaction for manual submission
		txData, err := json.MarshalIndent(signedTx, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling transaction: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("\n%s\n", txData)
		
		fmt.Printf("\n✅ Ready to submit! Copy the transaction data above and use 'shadowy wallet send-raw' to submit it to the network.\n")
		fmt.Printf("Expected result: %.8f %s tokens melted for %.8f SHADOW\n", 
			amount, tokenInfo.Ticker, shadowToReceive)
	},
}

var tokenTrustCmd = &cobra.Command{
	Use:   "trust [wallet-name]",
	Short: "Show token trust settings",
	Args:  cobra.MaximumNArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		var walletName string
		if len(args) > 0 {
			walletName = args[0]
		} else {
			// Use first available wallet
			wallets, err := listWallets()
			if err != nil || len(wallets) == 0 {
				fmt.Printf("Error: No wallets found\n")
				os.Exit(1)
			}
			walletName = wallets[0].Name
		}

		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}

		// Initialize trust manager
		trustManager, err := NewTokenTrustManager(wallet.Name, getWalletDir())
		if err != nil {
			fmt.Printf("Error initializing token trust manager: %v\n", err)
			os.Exit(1)
		}

		summary := trustManager.GetTrustSummary()
		
		fmt.Printf("Token Trust Settings for Wallet: %s\n", wallet.Name)
		fmt.Printf("============================================\n\n")
		
		fmt.Printf("Summary:\n")
		fmt.Printf("  Total tokens: %d\n", summary["total"])
		fmt.Printf("  ✅ Accepted:  %d\n", summary["accepted"])
		fmt.Printf("  ⚠️  Unknown:   %d\n", summary["unknown"])
		fmt.Printf("  🚫 Banned:    %d\n", summary["banned"])
		fmt.Printf("  🔒 Verified:  %d\n", summary["verified"])
		fmt.Printf("\n")

		// Show detailed trust info
		showDetails, _ := cmd.Flags().GetBool("details")
		if showDetails {
			// Show accepted tokens
			accepted := trustManager.ListTrustedTokens()
			if len(accepted) > 0 {
				fmt.Printf("✅ Accepted Tokens:\n")
				for tokenID, trust := range accepted {
					fmt.Printf("  %s (%s) - %s\n", trust.Name, trust.Ticker, tokenID)
					if trust.Notes != "" {
						fmt.Printf("    Notes: %s\n", trust.Notes)
					}
				}
				fmt.Printf("\n")
			}

			// Show unknown tokens
			unknown := trustManager.ListUnknownTokens()
			if len(unknown) > 0 {
				fmt.Printf("⚠️  Unknown Tokens:\n")
				for tokenID, trust := range unknown {
					name := trust.Name
					if name == "" {
						name = "Unknown"
					}
					fmt.Printf("  %s - %s\n", name, tokenID)
				}
				fmt.Printf("\n")
			}

			// Show banned tokens
			banned := trustManager.ListBannedTokens()
			if len(banned) > 0 {
				fmt.Printf("🚫 Banned Tokens:\n")
				for tokenID, trust := range banned {
					name := trust.Name
					if name == "" {
						name = "Unknown"
					}
					fmt.Printf("  %s - %s\n", name, tokenID)
					if trust.Notes != "" {
						fmt.Printf("    Reason: %s\n", trust.Notes)
					}
				}
				fmt.Printf("\n")
			}
		}

		if summary["unknown"] > 0 {
			fmt.Printf("💡 Use 'wallet tokens balances' to see unknown tokens\n")
			fmt.Printf("💡 Use 'wallet tokens accept <token-id>' to trust tokens\n")
		}
	},
}

func init() {
	rootCmd.AddCommand(walletCmd)
	walletCmd.AddCommand(generateCmd)
	walletCmd.AddCommand(validateCmd)
	walletCmd.AddCommand(fromKeyCmd)
	walletCmd.AddCommand(listCmd)
	walletCmd.AddCommand(showCmd)
	walletCmd.AddCommand(balanceCmd)
	walletCmd.AddCommand(tokensCmd)
	
	// Token subcommands
	tokensCmd.AddCommand(tokenBalancesCmd)
	tokensCmd.AddCommand(tokenAcceptCmd)
	tokensCmd.AddCommand(tokenBanCmd)
	tokensCmd.AddCommand(tokenMeltCmd)
	tokensCmd.AddCommand(tokenTrustCmd)
	
	// Add wallet-dir flag to all wallet commands
	walletCmd.PersistentFlags().StringVar(&walletDir, "wallet-dir", "", 
		"Directory for wallet files (default: $HOME/.shadowy)")
	
	// Add data flag to balance command for blockchain directory override
	balanceCmd.Flags().StringP("data", "d", "", "Override blockchain data directory")
	
	// Add flags for token commands
	tokenAcceptCmd.Flags().StringP("notes", "n", "", "Notes about why this token is trusted")
	tokenBanCmd.Flags().StringP("reason", "r", "", "Reason for banning this token")
	tokenTrustCmd.Flags().BoolP("details", "d", false, "Show detailed trust information")
}

// DeriveAddress creates a Shadowy address from a public key
// Format: [version:1][hash:20][checksum:4] = 25 bytes total
func DeriveAddress(publicKey []byte) string {
	// Step 1: Hash the public key with SHAKE256 for better distribution
	shake := sha3.NewShake256()
	shake.Write(publicKey)
	
	// Extract 20 bytes like Ethereum
	hash := make([]byte, 20)
	shake.Read(hash)
	
	// Step 2: Create versioned payload
	payload := make([]byte, 21)
	payload[0] = AddressVersion
	copy(payload[1:], hash)
	
	// Step 3: Calculate checksum (double SHA256 like Bitcoin)
	checksum := calculateChecksum(payload)
	
	// Step 4: Combine version + hash + checksum
	fullAddress := make([]byte, AddressLen)
	copy(fullAddress[:21], payload)
	copy(fullAddress[21:], checksum)
	
	// Step 5: Encode as hex with 'S' prefix for now (simpler than base58)
	return "S" + hex.EncodeToString(fullAddress)
}

// IsValidAddress validates a Shadowy address
func IsValidAddress(address string) bool {
	// Handle different address types
	if len(address) == 0 {
		return false
	}
	
	switch address[0] {
	case 'S':
		// Standard Shadowy address validation
		if len(address) != 1+AddressLen*2 {
			return false
		}
		
		decoded, err := hex.DecodeString(address[1:])
		if err != nil || len(decoded) != AddressLen {
			return false
		}
		
		// Check version
		if decoded[0] != AddressVersion {
			return false
		}
		
		// Verify checksum
		payload := decoded[:21]
		providedChecksum := decoded[21:]
		expectedChecksum := calculateChecksum(payload)
		
		return bytesEqual(providedChecksum, expectedChecksum)
		
	case 'L':
		// Liquidity pool address validation (L-addresses)
		// L-addresses are 41 characters: L + 40 hex chars
		if len(address) != 41 {
			return false
		}
		
		// Verify the remaining 40 characters are valid hex
		_, err := hex.DecodeString(address[1:])
		return err == nil
		
	default:
		return false
	}
}

// calculateChecksum computes 4-byte checksum using double SHA256
func calculateChecksum(data []byte) []byte {
	// First SHA256
	hash1 := sha3.NewLegacyKeccak256()
	hash1.Write(data)
	firstHash := hash1.Sum(nil)
	
	// Second SHA256
	hash2 := sha3.NewLegacyKeccak256()
	hash2.Write(firstHash)
	secondHash := hash2.Sum(nil)
	
	// Return first 4 bytes
	return secondHash[:AddressChecksumLen]
}

// Wallet file management functions

func getWalletDir() string {
	if walletDir != "" {
		return walletDir
	}
	
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return DefaultWalletDir
	}
	
	return filepath.Join(homeDir, DefaultWalletDir)
}

func ensureWalletDir() error {
	walletDirPath := getWalletDir()
	return os.MkdirAll(walletDirPath, 0700)
}

func saveWallet(wallet WalletFile) (string, error) {
	if err := ensureWalletDir(); err != nil {
		return "", fmt.Errorf("failed to create wallet directory: %w", err)
	}
	
	walletDirPath := getWalletDir()
	walletPath := filepath.Join(walletDirPath, wallet.Name+WalletFileExt)
	
	// Check if wallet already exists
	if _, err := os.Stat(walletPath); err == nil {
		return "", fmt.Errorf("wallet '%s' already exists", wallet.Name)
	}
	
	data, err := json.MarshalIndent(wallet, "", "  ")
	if err != nil {
		return "", fmt.Errorf("failed to marshal wallet: %w", err)
	}
	
	if err := os.WriteFile(walletPath, data, 0600); err != nil {
		return "", fmt.Errorf("failed to write wallet file: %w", err)
	}
	
	return walletPath, nil
}

func loadWallet(name string) (*WalletFile, error) {
	walletDirPath := getWalletDir()
	walletPath := filepath.Join(walletDirPath, name+WalletFileExt)
	
	data, err := os.ReadFile(walletPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet file: %w", err)
	}
	
	var wallet WalletFile
	if err := json.Unmarshal(data, &wallet); err != nil {
		return nil, fmt.Errorf("failed to parse wallet file: %w", err)
	}
	
	return &wallet, nil
}

func listWallets() ([]WalletFile, error) {
	// First try to list existing wallets
	wallets, err := listWalletsInternal()
	if err != nil {
		return nil, err
	}
	
	// If no wallets exist, auto-generate a default one
	if len(wallets) == 0 {
		_, err := ensureDefaultWallet()
		if err != nil {
			return nil, fmt.Errorf("failed to create default wallet: %w", err)
		}
		
		// Re-list wallets after creation to get full details
		wallets, err = listWalletsInternal()
		if err != nil {
			return nil, err
		}
	}
	
	// Load full wallet details for each wallet
	var fullWallets []WalletFile
	for _, wallet := range wallets {
		fullWallet, err := loadWallet(wallet.Name)
		if err != nil {
			fmt.Printf("Warning: failed to load wallet '%s': %v\n", wallet.Name, err)
			continue
		}
		fullWallets = append(fullWallets, *fullWallet)
	}
	
	return fullWallets, nil
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// calculateWalletBalance scans the blockchain and mempool to calculate wallet balance
func calculateWalletBalance(address string) (*WalletBalance, error) {
	balance := &WalletBalance{
		Address: address,
	}
	
	// Load blockchain to scan for transactions
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	blockchain, err := NewBlockchain(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
	}
	
	// Get all blocks to scan
	stats := blockchain.GetStats()
	var lastActivity time.Time
	
	// Scan all blocks from genesis to tip
	for height := uint64(0); height <= stats.TipHeight; height++ {
		block, err := blockchain.GetBlockByHeight(height)
		if err != nil {
			continue // Skip missing blocks
		}
		
		// Scan all transactions in the block
		for _, signedTx := range block.Body.Transactions {
			// Parse the transaction
			var tx Transaction
			if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
				continue // Skip invalid transactions
			}
			
			txInvolvement := false
			netAmount := int64(0)
			
			// Check outputs (received funds)
			for _, output := range tx.Outputs {
				if output.Address == address {
					balance.TotalReceived += output.Value
					balance.ConfirmedBalance += output.Value
					netAmount += int64(output.Value)
					txInvolvement = true
				}
			}
			
			// Check inputs (spent funds) - this is more complex as we need to look up previous outputs
			// For now, we'll implement a simplified version that assumes inputs are from this address
			// when the transaction is signed by this address's key
			if len(tx.Inputs) > 0 {
				// Simplified: if transaction has inputs and is not a coinbase, assume it's spending from this address
				// This is a simplification - in a full implementation, we'd need to track UTXOs
				if height > 0 { // Skip genesis block coinbase
					// Check if this transaction was signed by our wallet
					// For now, we'll use a heuristic: if we received outputs in previous blocks
					// and this transaction has inputs, it might be spending our funds
					if signedTx.SignerKey != "" && len(tx.Inputs) > 0 {
						// Try to estimate spent amount by looking at total outputs to other addresses
						totalOut := uint64(0)
						for _, output := range tx.Outputs {
							if output.Address != address {
								totalOut += output.Value
							}
						}
						if totalOut > 0 && balance.TotalReceived > 0 {
							// This is a heuristic - in reality we'd need full UTXO tracking
							spentAmount := totalOut
							if spentAmount <= balance.ConfirmedBalance {
								balance.TotalSent += spentAmount
								balance.ConfirmedBalance -= spentAmount
								netAmount -= int64(spentAmount)
								txInvolvement = true
							}
						}
					}
				}
			}
			
			if txInvolvement {
				balance.TransactionCount++
				if tx.Timestamp.After(lastActivity) {
					lastActivity = tx.Timestamp
				}
			}
		}
	}
	
	// Set last activity if we found any
	if !lastActivity.IsZero() {
		balance.LastActivity = &lastActivity
	}
	
	// TODO: Scan mempool for pending transactions
	// For now, pending balance equals confirmed balance
	balance.PendingBalance = balance.ConfirmedBalance
	
	// Calculate human-readable amounts
	balance.ConfirmedShadow = float64(balance.ConfirmedBalance) / float64(SatoshisPerShadow)
	balance.PendingShadow = float64(balance.PendingBalance) / float64(SatoshisPerShadow)
	balance.TotalReceivedShadow = float64(balance.TotalReceived) / float64(SatoshisPerShadow)
	balance.TotalSentShadow = float64(balance.TotalSent) / float64(SatoshisPerShadow)
	
	return balance, nil
}

// calculateWalletBalanceWithDir scans the blockchain with optional directory override
func calculateWalletBalanceWithDir(address string, blockchainDir string) (*WalletBalance, error) {
	balance := &WalletBalance{
		Address: address,
	}
	
	// Load blockchain configuration  
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	// Override blockchain directory if specified
	if blockchainDir != "" {
		config.BlockchainDirectory = blockchainDir
	}
	
	blockchain, err := NewBlockchain(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
	}
	
	// Get all blocks to scan
	stats := blockchain.GetStats()
	var lastActivity time.Time
	
	// Scan all blocks from genesis to tip
	for height := uint64(0); height <= stats.TipHeight; height++ {
		block, err := blockchain.GetBlockByHeight(height)
		if err != nil {
			// Skip missing blocks
			continue
		}
		
		// Scan all transactions in this block
		for _, signedTx := range block.Body.Transactions {
			// Parse the transaction
			var tx Transaction
			if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
				continue // Skip invalid transactions
			}
			
			txInvolvement := false
			
			// Check outputs (received funds)
			for _, output := range tx.Outputs {
				if output.Address == address {
					balance.TotalReceived += output.Value
					balance.ConfirmedBalance += output.Value
					txInvolvement = true
				}
			}
			
			// Check inputs (spent funds) - simplified implementation
			if len(tx.Inputs) > 0 && height > 0 { // Skip genesis block coinbase
				// This is a simplified version - in a full implementation,
				// we'd need to track UTXOs properly
				for _, output := range tx.Outputs {
					if output.Address != address {
						// This is likely spending from our address
						// (very simplified assumption)
					}
				}
			}
			
			if txInvolvement {
				balance.TransactionCount++
				if tx.Timestamp.After(lastActivity) {
					lastActivity = tx.Timestamp
				}
			}
		}
	}
	
	// Set last activity if we found any
	if !lastActivity.IsZero() {
		balance.LastActivity = &lastActivity
	}
	
	// TODO: Scan mempool for pending transactions
	// For now, pending balance equals confirmed balance
	balance.PendingBalance = balance.ConfirmedBalance
	
	// Calculate human-readable amounts
	balance.ConfirmedShadow = float64(balance.ConfirmedBalance) / float64(SatoshisPerShadow)
	balance.PendingShadow = float64(balance.PendingBalance) / float64(SatoshisPerShadow)
	balance.TotalReceivedShadow = float64(balance.TotalReceived) / float64(SatoshisPerShadow)
	balance.TotalSentShadow = float64(balance.TotalSent) / float64(SatoshisPerShadow)
	
	return balance, nil
}

// getWalletTransactions returns a list of transactions involving the wallet
func getWalletTransactions(address string, limit int) ([]TransactionReference, error) {
	var transactions []TransactionReference
	
	// Load blockchain to scan for transactions
	config, err := loadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	
	blockchain, err := NewBlockchain(config)
	if err != nil {
		return nil, fmt.Errorf("failed to initialize blockchain: %w", err)
	}
	
	stats := blockchain.GetStats()
	
	// Scan blocks from newest to oldest
	for height := stats.TipHeight; height >= 0 && len(transactions) < limit; height-- {
		block, err := blockchain.GetBlockByHeight(height)
		if err != nil {
			continue
		}
		
		// Scan transactions in reverse order (newest first)
		for i := len(block.Body.Transactions) - 1; i >= 0 && len(transactions) < limit; i-- {
			signedTx := block.Body.Transactions[i]
			
			var tx Transaction
			if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
				continue
			}
			
			// Check if transaction involves this address
			netAmount := int64(0)
			txType := ""
			
			// Check outputs (received)
			for _, output := range tx.Outputs {
				if output.Address == address {
					netAmount += int64(output.Value)
					if height == 0 || (len(tx.Inputs) == 0) {
						txType = "coinbase"
					} else {
						txType = "received"
					}
				}
			}
			
			// Check if this was a send transaction (simplified)
			if len(tx.Inputs) > 0 && height > 0 {
				// This is a simplification - we'd need full UTXO tracking for accuracy
				totalOut := uint64(0)
				hasOtherOutputs := false
				for _, output := range tx.Outputs {
					if output.Address != address {
						totalOut += output.Value
						hasOtherOutputs = true
					}
				}
				
				if hasOtherOutputs && netAmount <= 0 {
					// Likely a send transaction
					netAmount = -int64(totalOut)
					txType = "sent"
				}
			}
			
			if netAmount != 0 {
				transactions = append(transactions, TransactionReference{
					TxHash:      signedTx.TxHash,
					BlockHeight: height,
					Timestamp:   tx.Timestamp,
					Amount:      netAmount,
					Type:        txType,
					Confirmed:   true,
				})
			}
		}
		
		if height == 0 {
			break // Avoid underflow
		}
	}
	
	return transactions, nil
}

// ensureDefaultWallet ensures a default wallet exists, creating one if necessary
func ensureDefaultWallet() (*WalletFile, error) {
	// Try to get existing wallets (without triggering auto-creation to avoid recursion)
	wallets, err := listWalletsInternal()
	if err != nil {
		return nil, fmt.Errorf("failed to list wallets: %w", err)
	}

	// If we have wallets, use the first one
	if len(wallets) > 0 {
		wallet, err := loadWallet(wallets[0].Name)
		if err != nil {
			return nil, fmt.Errorf("failed to load existing wallet %s: %w", wallets[0].Name, err)
		}
		return wallet, nil
	}

	// No wallets exist, create a default one
	fmt.Println("📝 No wallets found. Creating default wallet...")
	
	// Ensure wallet directory exists
	if err := ensureWalletDir(); err != nil {
		return nil, fmt.Errorf("failed to create wallet directory: %w", err)
	}

	// Generate new wallet
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	address := DeriveAddress(keyPair.PublicKey[:])
	
	wallet := WalletFile{
		Name:       "default",
		Address:    address,
		PrivateKey: keyPair.PrivateKeyHex(),
		PublicKey:  keyPair.PublicKeyHex(),
		Identifier: keyPair.IdentifierHex(),
		CreatedAt:  time.Now().UTC(),
		Version:    1,
	}

	walletPath, err := saveWallet(wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to save default wallet: %w", err)
	}

	fmt.Printf("✅ Created default wallet: %s\n", wallet.Name)
	fmt.Printf("📍 Wallet address: %s\n", wallet.Address)
	fmt.Printf("💾 Saved to: %s\n", walletPath)
	
	return &wallet, nil
}

// listWalletsInternal lists wallets without auto-generation (internal use only)
func listWalletsInternal() ([]WalletFile, error) {
	walletDirPath := getWalletDir()
	
	// Check if directory exists
	if _, err := os.Stat(walletDirPath); os.IsNotExist(err) {
		return []WalletFile{}, nil
	}
	
	files, err := os.ReadDir(walletDirPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read wallet directory: %w", err)
	}
	
	var wallets []WalletFile
	for _, file := range files {
		if !file.IsDir() && strings.HasSuffix(file.Name(), WalletFileExt) {
			name := strings.TrimSuffix(file.Name(), WalletFileExt)
			info, err := file.Info()
			if err != nil {
				continue
			}
			
			wallets = append(wallets, WalletFile{
				Name:      name,
				CreatedAt: info.ModTime(),
			})
		}
	}
	
	// Sort by creation time (newest first)
	sort.Slice(wallets, func(i, j int) bool {
		return wallets[i].CreatedAt.After(wallets[j].CreatedAt)
	})
	
	return wallets, nil
}