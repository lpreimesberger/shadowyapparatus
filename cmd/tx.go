package cmd

import (
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/spf13/cobra"
)

var txCmd = &cobra.Command{
	Use:   "tx",
	Short: "Transaction operations for Shadowy blockchain",
	Long: `Create, sign, and verify Shadowy blockchain transactions.
Supports Bitcoin-style inputs/outputs with JOSE signatures.`,
}

var createTxCmd = &cobra.Command{
	Use:   "create",
	Short: "Create a new transaction",
	Long: `Create a new unsigned transaction with specified inputs and outputs.
Example: shadowy tx create --output addr1:1000 --output addr2:500`,
	Run: func(cmd *cobra.Command, args []string) {
		tx := NewTransaction()
		
		// Set not_until if specified
		if notUntilStr, _ := cmd.Flags().GetString("not-until"); notUntilStr != "" {
			notUntil, err := time.Parse(time.RFC3339, notUntilStr)
			if err != nil {
				fmt.Printf("Error parsing not-until time: %v\n", err)
				os.Exit(1)
			}
			tx.SetNotUntil(notUntil)
		}
		
		// Add inputs if specified
		inputs, _ := cmd.Flags().GetStringSlice("input")
		for _, input := range inputs {
			// Parse input format: txhash:index
			parts := strings.Split(input, ":")
			if len(parts) != 2 {
				fmt.Printf("Error parsing input '%s': expected format txhash:index\n", input)
				os.Exit(1)
			}
			
			txHash := parts[0]
			index, err := strconv.ParseUint(parts[1], 10, 32)
			if err != nil {
				fmt.Printf("Error parsing input index '%s': %v\n", parts[1], err)
				os.Exit(1)
			}
			
			tx.AddInput(txHash, uint32(index))
		}
		
		// Add outputs if specified
		outputs, _ := cmd.Flags().GetStringSlice("output")
		for _, output := range outputs {
			// Parse output format: address:value
			parts := strings.Split(output, ":")
			if len(parts) != 2 {
				fmt.Printf("Error parsing output '%s': expected format address:value\n", output)
				os.Exit(1)
			}
			
			address := parts[0]
			value, err := strconv.ParseUint(parts[1], 10, 64)
			if err != nil {
				fmt.Printf("Error parsing output value '%s': %v\n", parts[1], err)
				os.Exit(1)
			}
			
			tx.AddOutput(address, value)
		}
		
		// Validate transaction
		if err := tx.IsValid(); err != nil {
			fmt.Printf("Invalid transaction: %v\n", err)
			os.Exit(1)
		}
		
		// Output transaction as JSON
		data, err := json.MarshalIndent(tx, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling transaction: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println(string(data))
	},
}

var signTxCmd = &cobra.Command{
	Use:   "sign [transaction-json] [wallet-name]",
	Short: "Sign a transaction with a wallet",
	Long: `Sign a transaction using a wallet's private key.
The transaction must be provided as JSON and the wallet must exist.`,
	Args: cobra.ExactArgs(2),
	Run: func(cmd *cobra.Command, args []string) {
		txJSON := args[0]
		walletName := args[1]
		
		// Parse transaction
		var tx Transaction
		if err := json.Unmarshal([]byte(txJSON), &tx); err != nil {
			fmt.Printf("Error parsing transaction JSON: %v\n", err)
			os.Exit(1)
		}
		
		// Load wallet
		wallet, err := loadWallet(walletName)
		if err != nil {
			fmt.Printf("Error loading wallet '%s': %v\n", walletName, err)
			os.Exit(1)
		}
		
		// Reconstruct key pair from wallet
		privKeyBytes, err := hex.DecodeString(wallet.PrivateKey)
		if err != nil {
			fmt.Printf("Error decoding private key: %v\n", err)
			os.Exit(1)
		}
		
		keyPair, err := reconstructKeyPair([PrivateKeySize]byte(privKeyBytes))
		if err != nil {
			fmt.Printf("Error reconstructing key pair: %v\n", err)
			os.Exit(1)
		}
		
		// Sign transaction
		signedTx, err := SignTransaction(&tx, keyPair)
		if err != nil {
			fmt.Printf("Error signing transaction: %v\n", err)
			os.Exit(1)
		}
		
		// Output signed transaction as JSON
		data, err := json.MarshalIndent(signedTx, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling signed transaction: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Println(string(data))
	},
}

var verifyTxCmd = &cobra.Command{
	Use:   "verify [signed-transaction-json]",
	Short: "Verify a signed transaction",
	Long: `Verify the signature and validity of a signed transaction.
Returns the original transaction if valid.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		signedTxJSON := args[0]
		
		// Parse signed transaction
		var signedTx SignedTransaction
		if err := json.Unmarshal([]byte(signedTxJSON), &signedTx); err != nil {
			fmt.Printf("Error parsing signed transaction JSON: %v\n", err)
			os.Exit(1)
		}
		
		// Verify transaction
		tx, err := VerifySignedTransaction(&signedTx)
		if err != nil {
			fmt.Printf("Transaction verification failed: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("✓ Transaction signature is valid\n")
		fmt.Printf("✓ Transaction is well-formed\n")
		
		// Show transaction summary
		summary := tx.Summary()
		summary.Signer = signedTx.SignerKey[:16] + "..." // Truncate for display
		
		summaryData, err := json.MarshalIndent(summary, "", "  ")
		if err != nil {
			fmt.Printf("Error marshaling summary: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("\nTransaction Summary:\n%s\n", string(summaryData))
	},
}

var infoTxCmd = &cobra.Command{
	Use:   "info [transaction-json]",
	Short: "Show transaction information",
	Long: `Display detailed information about a transaction without verifying signatures.
Accepts either signed or unsigned transaction JSON.`,
	Args: cobra.ExactArgs(1),
	Run: func(cmd *cobra.Command, args []string) {
		txJSON := args[0]
		
		// Try to parse as signed transaction first
		var signedTx SignedTransaction
		if err := json.Unmarshal([]byte(txJSON), &signedTx); err == nil {
			fmt.Printf("Signed Transaction Information:\n")
			fmt.Printf("  Hash:      %s\n", signedTx.TxHash)
			fmt.Printf("  Algorithm: %s\n", signedTx.Algorithm)
			fmt.Printf("  Signer:    %s\n", signedTx.SignerKey[:16]+"...")
			fmt.Printf("  Signature: %d bytes\n\n", len(signedTx.Signature)/2)
			
			// Try to verify and show transaction details
			if tx, err := VerifySignedTransaction(&signedTx); err == nil {
				showTransactionDetails(tx)
			} else {
				fmt.Printf("Warning: Could not verify signature: %v\n", err)
			}
			return
		}
		
		// Parse as unsigned transaction
		var tx Transaction
		if err := json.Unmarshal([]byte(txJSON), &tx); err != nil {
			fmt.Printf("Error parsing transaction JSON: %v\n", err)
			os.Exit(1)
		}
		
		fmt.Printf("Unsigned Transaction Information:\n\n")
		showTransactionDetails(&tx)
	},
}

func showTransactionDetails(tx *Transaction) {
	hash, _ := tx.Hash()
	summary := tx.Summary()
	
	fmt.Printf("Transaction Details:\n")
	fmt.Printf("  Hash:        %s\n", hash)
	fmt.Printf("  Version:     %d\n", tx.Version)
	fmt.Printf("  Timestamp:   %s\n", tx.Timestamp.Format(time.RFC3339))
	fmt.Printf("  Not Until:   %s\n", tx.NotUntil.Format(time.RFC3339))
	fmt.Printf("  Nonce:       %d\n", tx.Nonce)
	fmt.Printf("  Valid:       %t\n\n", summary.Valid)
	
	fmt.Printf("Inputs (%d):\n", len(tx.Inputs))
	for i, input := range tx.Inputs {
		fmt.Printf("  %d. Prev TX: %s\n", i+1, input.PreviousTxHash)
		fmt.Printf("     Output:  %d\n", input.OutputIndex)
		fmt.Printf("     Script:  %s\n", input.ScriptSig)
		if i < len(tx.Inputs)-1 {
			fmt.Printf("\n")
		}
	}
	
	fmt.Printf("\nOutputs (%d):\n", len(tx.Outputs))
	for i, output := range tx.Outputs {
		fmt.Printf("  %d. Address: %s\n", i+1, output.Address)
		fmt.Printf("     Value:   %d\n", output.Value)
		fmt.Printf("     Script:  %s\n", output.ScriptPubKey)
		if i < len(tx.Outputs)-1 {
			fmt.Printf("\n")
		}
	}
	
	fmt.Printf("\nTotal Output Value: %d\n", summary.TotalValue)
}

func init() {
	rootCmd.AddCommand(txCmd)
	txCmd.AddCommand(createTxCmd)
	txCmd.AddCommand(signTxCmd)
	txCmd.AddCommand(verifyTxCmd)
	txCmd.AddCommand(infoTxCmd)
	
	// Flags for create command
	createTxCmd.Flags().StringSlice("input", []string{}, "Transaction inputs (format: txhash:index)")
	createTxCmd.Flags().StringSlice("output", []string{}, "Transaction outputs (format: address:value)")
	createTxCmd.Flags().String("not-until", "", "Not valid until timestamp (ISO 8601 format)")
	
	// Add wallet-dir flag to sign command
	signTxCmd.Flags().StringVar(&walletDir, "wallet-dir", "", 
		"Directory for wallet files (default: $HOME/.shadowy)")
}