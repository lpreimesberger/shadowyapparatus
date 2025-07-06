package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
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
	PrivateKey string    `json:"private_key"`
	PublicKey  string    `json:"public_key"`
	Identifier string    `json:"identifier"`
	CreatedAt  time.Time `json:"created_at"`
	Version    int       `json:"version"`
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
			PrivateKey: keyPair.PrivateKeyHex(),
			PublicKey:  keyPair.PublicKeyHex(),
			Identifier: keyPair.IdentifierHex(),
			CreatedAt:  time.Now().UTC(),
			Version:    1,
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
			PrivateKey: keyPair.PrivateKeyHex(),
			PublicKey:  keyPair.PublicKeyHex(),
			Identifier: keyPair.IdentifierHex(),
			CreatedAt:  time.Now().UTC(),
			Version:    1,
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

var showCmd = &cobra.Command{
	Use:   "show [wallet-name]",
	Short: "Show details of a specific wallet",
	Args:  cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		walletName := args[0]
		
		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}
		
		fmt.Printf("Wallet Details:\n")
		fmt.Printf("Name:        %s\n", wallet.Name)
		fmt.Printf("Address:     %s\n", wallet.Address)
		fmt.Printf("Public Key:  %s\n", wallet.PublicKey)
		fmt.Printf("Identifier:  %s\n", wallet.Identifier)
		fmt.Printf("Created:     %s\n", wallet.CreatedAt.Format("2006-01-02 15:04:05 UTC"))
		fmt.Printf("Version:     %d\n", wallet.Version)
	},
}

func init() {
	rootCmd.AddCommand(walletCmd)
	walletCmd.AddCommand(generateCmd)
	walletCmd.AddCommand(validateCmd)
	walletCmd.AddCommand(fromKeyCmd)
	walletCmd.AddCommand(listCmd)
	walletCmd.AddCommand(showCmd)
	
	// Add wallet-dir flag to all wallet commands
	walletCmd.PersistentFlags().StringVar(&walletDir, "wallet-dir", "", 
		"Directory for wallet files (default: $HOME/.shadowy)")
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
	// Check prefix and decode hex
	if len(address) != 1+AddressLen*2 || address[0] != 'S' {
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
		if !file.IsDir() && filepath.Ext(file.Name()) == WalletFileExt {
			name := file.Name()[:len(file.Name())-len(WalletFileExt)]
			wallet, err := loadWallet(name)
			if err != nil {
				fmt.Printf("Warning: failed to load wallet '%s': %v\n", name, err)
				continue
			}
			wallets = append(wallets, *wallet)
		}
	}
	
	return wallets, nil
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