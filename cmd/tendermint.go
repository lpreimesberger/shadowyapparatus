package cmd

import (
	"archive/tar"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	cmtcfg "github.com/cometbft/cometbft/config"
	"github.com/cometbft/cometbft/p2p"
	"github.com/cometbft/cometbft/privval"
	"github.com/cometbft/cometbft/types"
	"github.com/gorilla/mux"
	"shadowyapparatus/tendermint/abci"
	"shadowyapparatus/tendermint/node"
)

var tendermintCmd = &cobra.Command{
	Use:   "tendermint",
	Short: "Start Shadowy blockchain with Tendermint consensus",
	Long: `Start the Shadowy blockchain node using Tendermint Core for consensus.
This replaces the custom consensus engine with battle-tested BFT consensus
while preserving ML-DSA-87 post-quantum cryptography and all existing features.`,
	Run: runTendermintNode,
}

var bootstrapCmd = &cobra.Command{
	Use:   "bootstrap [bootstrap-archive.tar.gz]",
	Short: "Initialize a new node from a bootstrap package",
	Long: `Initialize a new Shadowy Tendermint node using a bootstrap package.
This extracts the genesis block and configuration files needed to join
the existing network without needing to sync from block 0.`,
	Args: cobra.MaximumNArgs(1),
	Run: runBootstrapNode,
}

var (
	tendermintConfigDir string
	tendermintDataDir   string
	tendermintStandalone bool
	tendermintSeeds     string
	tendermintPeers     string
	tendermintExtAddr   string
	tendermintHTTPPort  int
	tendermintDisableHTTP bool
	tendermintMinerAddress string
	tendermintDisableFarming bool
)

// Adapter types to bridge cmd types to ABCI interfaces

// BlockchainAdapter adapts cmd.Blockchain to abci.BlockchainInterface
type BlockchainAdapter struct {
	blockchain *Blockchain
}

func (ba *BlockchainAdapter) AddBlock(block abci.BlockInterface) error {
	// Convert interface block back to concrete block
	if b, ok := block.(*abci.Block); ok {
		// Create cmd.Block from abci.Block
		cmdBlock := &Block{
			Header: BlockHeader{
				Height:            b.Header.Height,
				Timestamp:         b.Header.Timestamp,
				PreviousBlockHash: b.Header.PreviousBlockHash,
				MerkleRoot:        b.Header.MerkleRoot,
				Nonce:             b.Header.Nonce,
				Version:           uint32(b.Header.Version),
			},
			Body: BlockBody{
				Transactions: convertSignedTransactions(b.Body.Transactions),
			},
		}
		return ba.blockchain.AddBlock(cmdBlock)
	}
	return fmt.Errorf("invalid block type")
}

func (ba *BlockchainAdapter) GetTip() (abci.BlockInterface, error) {
	tip, err := ba.blockchain.GetTip()
	if err != nil {
		return nil, err
	}
	
	// Convert cmd.Block to abci.Block
	abciBlock := &abci.Block{
		Header: abci.BlockHeader{
			Height:            tip.Header.Height,
			Timestamp:         tip.Header.Timestamp,
			PreviousBlockHash: tip.Header.PreviousBlockHash,
			MerkleRoot:        tip.Header.MerkleRoot,
			Nonce:             tip.Header.Nonce,
			Difficulty:        0, // Not used in cmd.BlockHeader
			Version:           int(tip.Header.Version),
		},
		Body: abci.BlockBody{
			Transactions: convertToABCITransactions(tip.Body.Transactions),
		},
	}
	
	return abciBlock, nil
}

func (ba *BlockchainAdapter) GetStats() abci.BlockchainStats {
	stats := ba.blockchain.GetStats()
	return abci.BlockchainStats{
		TipHeight:   stats.TipHeight,
		TotalBlocks: int(stats.TotalBlocks),
		GenesisHash: stats.GenesisHash,
	}
}

// GetUTXOsForAddress scans the blockchain to find unspent transaction outputs for an address
func (ba *BlockchainAdapter) GetUTXOsForAddress(address string) ([]UTXOResponse, error) {
	var utxos []UTXOResponse
	spentOutputs := make(map[string]bool) // Track spent outputs: "txid:vout" -> true

	stats := ba.blockchain.GetStats()

	// First pass: find all spent outputs by scanning all transaction inputs
	for height := uint64(1); height <= stats.TipHeight; height++ { // Skip genesis block
		block, err := ba.blockchain.GetBlockByHeight(height)
		if err != nil {
			continue
		}

		for _, signedTx := range block.Body.Transactions {
			var tx Transaction
			if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
				continue
			}

			// Mark all inputs as spent
			for _, input := range tx.Inputs {
				spentKey := fmt.Sprintf("%s:%d", input.PreviousTxHash, input.OutputIndex)
				spentOutputs[spentKey] = true
			}
		}
	}

	// Second pass: find all outputs for this address and check if they're unspent
	for height := uint64(0); height <= stats.TipHeight; height++ {
		block, err := ba.blockchain.GetBlockByHeight(height)
		if err != nil {
			continue
		}

		for _, signedTx := range block.Body.Transactions {
			var tx Transaction
			if err := json.Unmarshal(signedTx.Transaction, &tx); err != nil {
				continue
			}

			// Check all outputs for this address
			for outputIndex, output := range tx.Outputs {
				if output.Address == address {
					// Create UTXO key to check if spent
					utxoKey := fmt.Sprintf("%s:%d", signedTx.TxHash, outputIndex)

					// Only include if not spent
					if !spentOutputs[utxoKey] {
						// Calculate confirmations
						confirmations := int(stats.TipHeight - height + 1)

						// Generate script pubkey (simplified)
						scriptPubkey := fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address[1:41])
						if len(address) > 41 {
							scriptPubkey = fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address[1:41])
						}

						utxos = append(utxos, UTXOResponse{
							TxID:         signedTx.TxHash,
							Vout:         uint32(outputIndex),
							Value:        output.Value,
							ScriptPubkey: scriptPubkey,
							Address:      address,
							Confirmations: confirmations,
						})
					}
				}
			}
		}
	}

	// Also add mining rewards as UTXOs (they're not in regular transactions)
	// Get mining reward data from explorer API
	log.Printf("DEBUG: Checking mining rewards for address: %s", address)
	resp, err := http.Get("http://localhost:10001/api/v1/wallet/" + address)
	log.Printf("DEBUG: Explorer API call error: %v", err)
	if err == nil && resp.StatusCode == http.StatusOK {
		log.Printf("DEBUG: Explorer API success, status: %d", resp.StatusCode)
		body, err := io.ReadAll(resp.Body)
		resp.Body.Close()
		if err == nil {
			var walletData struct {
				Transactions []struct {
					TxHash      string `json:"tx_hash"`
					BlockHeight uint64 `json:"block_height"`
					Type        string `json:"type"`
					Amount      uint64 `json:"amount"`
					ToAddress   string `json:"to_address"`
				} `json:"transactions"`
			}
			if json.Unmarshal(body, &walletData) == nil {
				log.Printf("DEBUG: Found %d transactions in explorer data", len(walletData.Transactions))
				// Add mining rewards as synthetic UTXOs
				for _, tx := range walletData.Transactions {
					if tx.Type == "mining_reward" && tx.ToAddress == address {
						log.Printf("DEBUG: Found mining reward: %s, amount: %d", tx.TxHash, tx.Amount)
						// Check if this mining reward UTXO has been spent
						utxoKey := fmt.Sprintf("%s:0", tx.TxHash)
						if !spentOutputs[utxoKey] {
							confirmations := int(stats.TipHeight - tx.BlockHeight + 1)
							log.Printf("DEBUG: Adding unspent mining reward UTXO: %s", utxoKey)
							utxos = append(utxos, UTXOResponse{
								TxID:         tx.TxHash,
								Vout:         0, // Mining rewards are always output 0
								Value:        tx.Amount,
								ScriptPubkey: func() string {
									if len(address) > 41 {
										return fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address[1:41])
									}
									return fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address[1:])
								}(),
								Address:      address,
								Confirmations: confirmations,
							})
						}
					}
				}
			}
		}
	}

	return utxos, nil
}

// MempoolAdapter adapts cmd.Mempool to abci.MempoolInterface
type MempoolAdapter struct {
	mempool *Mempool
}

func (ma *MempoolAdapter) RemoveTransaction(txHash string) {
	ma.mempool.RemoveTransaction(txHash)
}

// TransactionValidatorAdapter adapts cmd functions to abci.TransactionValidator
type TransactionValidatorAdapter struct{}

func (tva *TransactionValidatorAdapter) VerifySignedTransaction(tx *abci.SignedTransaction) (*abci.Transaction, error) {
	// Convert abci.SignedTransaction to cmd.SignedTransaction
	cmdTx := SignedTransaction{
		TxHash:      tx.TxHash,
		SignerKey:   tx.SignerKey,
		Algorithm:   tx.Algorithm,
		Signature:   tx.Signature,
		Transaction: tx.Transaction,
	}
	
	// Use existing cmd.VerifySignedTransaction
	parsedTx, err := VerifySignedTransaction(&cmdTx)
	if err != nil {
		return nil, err
	}
	
	// Convert cmd.Transaction to abci.Transaction
	abciTx := &abci.Transaction{
		Version:  parsedTx.Version,
		Inputs:   convertToABCIInputs(parsedTx.Inputs),
		Outputs:  convertToABCIOutputs(parsedTx.Outputs),
		TokenOps: convertToABCITokenOps(parsedTx.TokenOps),
		Nonce:    parsedTx.Nonce,
	}
	
	return abciTx, nil
}

// FarmingServiceAdapter adapts cmd.FarmingService to abci.FarmingServiceInterface
type FarmingServiceAdapter struct {
	service *FarmingService
}

func (fsa *FarmingServiceAdapter) IsRunning() bool {
	if fsa.service == nil {
		return false
	}
	return fsa.service.IsRunning()
}

func (fsa *FarmingServiceAdapter) SubmitChallenge(challenge *abci.StorageChallenge) *abci.StorageProof {
	if fsa.service == nil {
		return &abci.StorageProof{
			ChallengeID: challenge.ID,
			Valid:       false,
			Error:       "farming service not available",
		}
	}
	
	// Convert ABCI challenge to cmd challenge
	cmdChallenge := &StorageChallenge{
		ID:        challenge.ID,
		Challenge: challenge.Challenge,
		Timestamp: challenge.Timestamp,
		Difficulty: challenge.Difficulty,
		ResponseChan: make(chan *StorageProof, 1),
	}
	
	// Submit to farming service
	cmdProof := fsa.service.SubmitChallenge(cmdChallenge)
	
	// Convert cmd proof to ABCI proof
	abciProof := &abci.StorageProof{
		ChallengeID:  cmdProof.ChallengeID,
		PlotFile:     cmdProof.PlotFile,
		Offset:       cmdProof.Offset,
		PrivateKey:   cmdProof.PrivateKey,
		Signature:    cmdProof.Signature,
		Valid:        cmdProof.Valid,
		ResponseTime: cmdProof.ResponseTime,
		Error:        cmdProof.Error,
		Quality:      0, // Calculate from cmd proof if available
	}
	
	return abciProof
}

func (fsa *FarmingServiceAdapter) GetStats() abci.FarmingStats {
	if fsa.service == nil {
		return abci.FarmingStats{}
	}
	
	cmdStats := fsa.service.GetStats()
	return abci.FarmingStats{
		StartTime:           cmdStats.StartTime,
		PlotFilesIndexed:    cmdStats.PlotFilesIndexed,
		TotalKeys:           cmdStats.TotalKeys,
		ChallengesHandled:   cmdStats.ChallengesHandled,
		LastChallengeTime:   cmdStats.LastChallengeTime,
		AverageResponseTime: cmdStats.AverageResponseTime,
		ErrorCount:          cmdStats.ErrorCount,
	}
}

// Helper conversion functions
func convertSignedTransactions(abciTxs []*abci.SignedTransaction) []SignedTransaction {
	var cmdTxs []SignedTransaction
	for _, tx := range abciTxs {
		cmdTxs = append(cmdTxs, SignedTransaction{
			TxHash:      tx.TxHash,
			SignerKey:   tx.SignerKey,
			Algorithm:   tx.Algorithm,
			Signature:   tx.Signature,
			Transaction: tx.Transaction,
		})
	}
	return cmdTxs
}

func convertToABCITransactions(cmdTxs []SignedTransaction) []*abci.SignedTransaction {
	var abciTxs []*abci.SignedTransaction
	for _, tx := range cmdTxs {
		abciTxs = append(abciTxs, &abci.SignedTransaction{
			TxHash:      tx.TxHash,
			SignerKey:   tx.SignerKey,
			Algorithm:   tx.Algorithm,
			Signature:   tx.Signature,
			Transaction: tx.Transaction,
		})
	}
	return abciTxs
}

func convertToABCIInputs(inputs []TransactionInput) []abci.TransactionInput {
	var abciInputs []abci.TransactionInput
	for _, input := range inputs {
		abciInputs = append(abciInputs, abci.TransactionInput{
			PreviousTxHash: input.PreviousTxHash,
			OutputIndex:    input.OutputIndex,
			ScriptSig:      input.ScriptSig,
			Sequence:       input.Sequence,
		})
	}
	return abciInputs
}

func convertToABCIOutputs(outputs []TransactionOutput) []abci.TransactionOutput {
	var abciOutputs []abci.TransactionOutput
	for _, output := range outputs {
		abciOutputs = append(abciOutputs, abci.TransactionOutput{
			Value:   output.Value,
			Address: output.Address,
		})
	}
	return abciOutputs
}

func convertToABCITokenOps(tokenOps []TokenOperation) []abci.TokenOperation {
	var abciTokenOps []abci.TokenOperation
	for _, op := range tokenOps {
		abciTokenOps = append(abciTokenOps, abci.TokenOperation{
			Type:    int(op.Type),
			TokenID: op.TokenID,
			Amount:  op.Amount,
			From:    op.From,
			To:      op.To,
		})
	}
	return abciTokenOps
}

func init() {
	rootCmd.AddCommand(tendermintCmd)
	
	tendermintCmd.PersistentFlags().StringVar(&tendermintConfigDir, "config-dir", "./tendermint-config", 
		"Directory for Tendermint configuration files")
	tendermintCmd.PersistentFlags().StringVar(&tendermintDataDir, "data-dir", "./data", 
		"Directory for blockchain data storage")
	tendermintCmd.Flags().BoolVar(&tendermintStandalone, "standalone", false, 
		"Run ABCI app in standalone mode (for testing)")
	tendermintCmd.Flags().StringVar(&tendermintSeeds, "seeds", "", 
		"Comma-separated list of seed nodes (node_id@host:port) to bootstrap network discovery")
	tendermintCmd.Flags().StringVar(&tendermintPeers, "persistent-peers", "", 
		"Comma-separated list of persistent peers (node_id@host:port) to maintain connections")
	tendermintCmd.Flags().StringVar(&tendermintExtAddr, "external-address", "", 
		"External address to advertise to other peers (host:port)")
	tendermintCmd.Flags().IntVar(&tendermintHTTPPort, "http-port", 8080, 
		"HTTP API server port")
	tendermintCmd.Flags().BoolVar(&tendermintDisableHTTP, "disable-http", false, 
		"Disable HTTP API server")
	tendermintCmd.Flags().StringVar(&tendermintMinerAddress, "miner-address", "", 
		"Address to receive mining rewards (default: auto-detect from default wallet)")
	tendermintCmd.Flags().BoolVar(&tendermintDisableFarming, "disable-farming", false,
		"Disable proof-of-storage farming service integration (farming enabled by default)")
}

// getDefaultWalletAddress attempts to find or create a default wallet address
func getDefaultWalletAddress() (string, error) {
	// Use existing ensureDefaultWallet function from wallet.go
	wallet, err := ensureDefaultWallet()
	if err != nil {
		return "", fmt.Errorf("failed to ensure default wallet: %w", err)
	}
	
	return wallet.Address, nil
}

func runTendermintNode(cmd *cobra.Command, args []string) {
	log.Printf("🚀 Starting Shadowy blockchain with Tendermint consensus")
	log.Printf("📁 Config directory: %s", tendermintConfigDir)
	log.Printf("📁 Data directory: %s", tendermintDataDir)
	
	// Ensure config directory exists
	if err := os.MkdirAll(tendermintConfigDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create config directory: %v", err)
	}
	
	// Ensure data directory exists  
	if err := os.MkdirAll(tendermintDataDir, 0755); err != nil {
		log.Fatalf("❌ Failed to create data directory: %v", err)
	}
	
	// Auto-detect miner address if not provided
	if tendermintMinerAddress == "" {
		log.Printf("🔍 Auto-detecting miner address from default wallet...")
		autoAddress, err := getDefaultWalletAddress()
		if err != nil {
			log.Printf("⚠️  Failed to auto-detect wallet address: %v", err)
			log.Printf("⚠️  Mining rewards will be disabled")
		} else {
			tendermintMinerAddress = autoAddress
			log.Printf("✅ Auto-detected miner address: %s", tendermintMinerAddress)
		}
	} else {
		log.Printf("💰 Using specified miner address: %s", tendermintMinerAddress)
	}
	
	// Initialize blockchain storage
	log.Printf("🔧 Initializing blockchain storage...")
	blockchainConfig := &ShadowConfig{
		BlockchainDirectory: filepath.Join(tendermintDataDir, "blocks.db"),
		LogLevel:           "info",
		LoggingDirectory:   tendermintDataDir,
		ScratchDirectory:   tendermintDataDir,
	}
	blockchain, err := NewBlockchain(blockchainConfig)
	if err != nil {
		log.Fatalf("❌ Failed to initialize blockchain: %v", err)
	}
	
	// Initialize mempool
	log.Printf("🔧 Initializing mempool...")
	mempoolConfig := &MempoolConfig{
		MaxTransactions:  1000,
		MaxMempoolSize:   100 * 1024 * 1024, // 100MB
		EnableValidation: true,
		EnableBroadcast:  false, // Tendermint will handle broadcasting
	}
	mempool := NewMempool(mempoolConfig)
	
	// Initialize farming service (enabled by default, unless --disable-farming)
	var farmingService *FarmingService
	var farmingAdapter *FarmingServiceAdapter
	
	if !tendermintDisableFarming {
		log.Printf("🌾 Initializing farming service...")
		
		// Ensure plots directory exists
		plotsDir := filepath.Join(tendermintDataDir, "plots")
		if err := os.MkdirAll(plotsDir, 0755); err != nil {
			log.Printf("⚠️  Failed to create plots directory: %v", err)
		}
		
		farmingConfig := &ShadowConfig{
			PlotDirectories:  []string{plotsDir},
			ScratchDirectory: tendermintDataDir,
			LogLevel:        "info",
		}
		
		farmingService = NewFarmingService(farmingConfig)
		if err := farmingService.Start(); err != nil {
			log.Printf("⚠️  Failed to start farming service: %v", err)
			log.Printf("⚠️  Farming will be disabled, mining rewards will still work")
			farmingAdapter = &FarmingServiceAdapter{service: nil}
		} else {
			farmingAdapter = &FarmingServiceAdapter{service: farmingService}
			log.Printf("✅ Farming service initialized")
		}
	} else {
		log.Printf("🚫 Farming service disabled by --disable-farming flag")
		farmingAdapter = &FarmingServiceAdapter{service: nil}
	}
	
	// Log mining configuration
	if tendermintMinerAddress != "" {
		log.Printf("💰 Mining rewards enabled for address: %s", tendermintMinerAddress)
		log.Printf("🎯 Block reward at height 1: %.8f SHADOW", float64(abci.CalculateBlockReward(1))/100000000.0)
	} else {
		log.Printf("⚠️  Mining rewards disabled (no miner address specified)")
	}
	
	// Create ABCI application
	log.Printf("🔧 Creating ABCI application...")
	blockchainAdapter := &BlockchainAdapter{blockchain: blockchain}
	mempoolAdapter := &MempoolAdapter{mempool: mempool}
	validatorAdapter := &TransactionValidatorAdapter{}
	app := abci.NewShadowyABCIApp(blockchainAdapter, mempoolAdapter, validatorAdapter, farmingAdapter, tendermintMinerAddress)
	
	// Create Tendermint node
	log.Printf("🔧 Creating Tendermint node...")
	tendermintNode, err := node.NewShadowyNode(tendermintConfigDir, app)
	if err != nil {
		log.Fatalf("❌ Failed to create Tendermint node: %v", err)
	}
	
	// Configure P2P networking from command line flags
	config := tendermintNode.GetConfig()
	if tendermintSeeds != "" {
		config.P2P.Seeds = tendermintSeeds
		log.Printf("🌱 Using seed nodes: %s", tendermintSeeds)
	}
	if tendermintPeers != "" {
		config.P2P.PersistentPeers = tendermintPeers
		log.Printf("🤝 Using persistent peers: %s", tendermintPeers)
	}
	if tendermintExtAddr != "" {
		config.P2P.ExternalAddress = tendermintExtAddr
		log.Printf("📡 External address: %s", tendermintExtAddr)
	}
	
	// Setup HTTP API server if enabled
	var httpServer *http.Server
	if !tendermintDisableHTTP {
		log.Printf("🔧 Starting HTTP API server on port %d...", tendermintHTTPPort)
		httpServer = createTendermintHTTPServer(blockchainAdapter, mempoolAdapter, tendermintHTTPPort, tendermintMinerAddress)
		
		// Start HTTP server in background
		go func() {
			if err := httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("⚠️ HTTP server error: %v", err)
			}
		}()
		log.Printf("✅ HTTP API server started on port %d", tendermintHTTPPort)
	}

	// Setup signal handling for graceful shutdown
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	
	// Start the node
	if tendermintStandalone {
		log.Printf("🔧 Starting in standalone ABCI mode...")
		if err := tendermintNode.StartStandalone("tcp://0.0.0.0:26658"); err != nil {
			log.Fatalf("❌ Failed to start standalone ABCI app: %v", err)
		}
	} else {
		log.Printf("🔧 Starting full Tendermint node...")
		if err := tendermintNode.Start(); err != nil {
			log.Fatalf("❌ Failed to start Tendermint node: %v", err)
		}
	}
	
	// Print node information
	if tendermintNode.IsRunning() {
		nodeInfo := tendermintNode.NodeInfo()
		log.Printf("✅ Shadowy Tendermint node started successfully!")
		log.Printf("🆔 Node ID: %s", nodeInfo.ID())
		log.Printf("🔗 P2P Address: %s@%s", nodeInfo.ID(), nodeInfo.ListenAddr)
		
		if !tendermintStandalone {
			config := tendermintNode.GetConfig()
			log.Printf("🌐 RPC Address: %s", config.RPC.ListenAddress)
			log.Printf("📡 P2P Address: %s", config.P2P.ListenAddress)
		}
		
		log.Printf("🛡️  Post-Quantum Security: ML-DSA-87 enabled")
		log.Printf("⚡ Consensus: Tendermint BFT")
		log.Printf("📊 Blockchain: Initialized and ready")
		log.Printf("💾 Mempool: Ready for transactions")
		
		if tendermintStandalone {
			log.Printf("🧪 Running in standalone mode - connect external Tendermint node to tcp://localhost:26658")
		}
		
		log.Printf("🎯 Press Ctrl+C to stop the node")
	}
	
	// Wait for shutdown signal
	<-sigChan
	log.Printf("🛑 Shutdown signal received, stopping node...")
	
	// Stop the HTTP server gracefully
	if httpServer != nil {
		log.Printf("🛑 Stopping HTTP API server...")
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := httpServer.Shutdown(ctx); err != nil {
			log.Printf("⚠️ Error stopping HTTP server: %v", err)
		}
	}

	// Stop the node gracefully
	if err := tendermintNode.Stop(); err != nil {
		log.Printf("⚠️ Error stopping Tendermint node: %v", err)
	}
	
	log.Printf("✅ Shadowy Tendermint node stopped successfully")
}

// tendermintInitCmd initializes Tendermint configuration
var tendermintInitCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize Tendermint configuration for Shadowy blockchain",
	Run: func(cmd *cobra.Command, args []string) {
		log.Printf("🔧 Initializing Tendermint configuration...")
		log.Printf("📁 Target directory: %s", tendermintConfigDir)
		
		// Create basic directory structure
		if err := os.MkdirAll(tendermintConfigDir, 0755); err != nil {
			log.Fatalf("❌ Failed to create config directory: %v", err)
		}
		
		configDir := filepath.Join(tendermintConfigDir, "config")
		dataDir := filepath.Join(tendermintConfigDir, "data")
		
		if err := os.MkdirAll(configDir, 0755); err != nil {
			log.Fatalf("❌ Failed to create config subdirectory: %v", err)
		}
		
		if err := os.MkdirAll(dataDir, 0755); err != nil {
			log.Fatalf("❌ Failed to create data directory: %v", err)
		}
		
		// Initialize Tendermint configuration
		config := cmtcfg.DefaultConfig()
		config.SetRoot(tendermintConfigDir)
		
		// Configure for Shadowy blockchain
		config.Moniker = "shadowy-validator"
		config.P2P.ListenAddress = "tcp://0.0.0.0:26656"
		config.RPC.ListenAddress = "tcp://0.0.0.0:26657"
		config.Consensus.CreateEmptyBlocks = true
		config.Consensus.CreateEmptyBlocksInterval = 30 * time.Second
		config.Mempool.Size = 5000
		config.Mempool.MaxTxsBytes = 1024 * 1024 * 1024 // 1GB
		
		// Write config.toml
		cmtcfg.WriteConfigFile(filepath.Join(configDir, "config.toml"), config)
		log.Printf("✅ Created config.toml")
		
		// Generate node key
		nodeKey, err := p2p.LoadOrGenNodeKey(config.NodeKeyFile())
		if err != nil {
			log.Fatalf("❌ Failed to generate node key: %v", err)
		}
		log.Printf("✅ Generated node key: %s", nodeKey.ID())
		
		// Generate validator keys
		pv := privval.LoadOrGenFilePV(config.PrivValidatorKeyFile(), config.PrivValidatorStateFile())
		log.Printf("✅ Generated validator keys")
		
		// Create genesis document
		genDoc := &types.GenesisDoc{
			ChainID:         "shadowy-testnet",
			GenesisTime:     time.Now(),
			InitialHeight:   1,
			ConsensusParams: types.DefaultConsensusParams(),
			AppHash:         []byte{},
			AppState:        json.RawMessage(`{"initial_supply": "100000000"}`),
		}
		
		// Add the validator to genesis
		pubKey, err := pv.GetPubKey()
		if err != nil {
			log.Fatalf("❌ Failed to get validator public key: %v", err)
		}
		
		validator := types.GenesisValidator{
			Address: pv.GetAddress(),
			PubKey:  pubKey,
			Power:   1,
			Name:    "shadowy-validator",
		}
		genDoc.Validators = []types.GenesisValidator{validator}
		
		// Write genesis.json
		if err := genDoc.SaveAs(config.GenesisFile()); err != nil {
			log.Fatalf("❌ Failed to save genesis file: %v", err)
		}
		log.Printf("✅ Created genesis.json")
		
		log.Printf("✅ Tendermint configuration initialized at %s", tendermintConfigDir)
		log.Printf("📁 Config directory: %s", configDir)
		log.Printf("📁 Data directory: %s", dataDir)
		log.Printf("🆔 Node ID: %s", nodeKey.ID())
		log.Printf("⚡ Chain ID: %s", genDoc.ChainID)
		log.Printf("🚀 Start with: './shadowy tendermint --config-dir %s'", tendermintConfigDir)
	},
}

// tendermintStatusCmd shows Tendermint node status  
var tendermintStatusCmd = &cobra.Command{
	Use:   "status",
	Short: "Show Tendermint node status",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🔍 Checking Tendermint node status...")
		
		// TODO: Implement status checking
		// This would:
		// 1. Connect to RPC endpoint
		// 2. Get node info, validator info, consensus state
		// 3. Show blockchain height, peer count, etc.
		// 4. Display health status
		
		fmt.Println("📊 Node status: [Not implemented yet]")
	},
}

// createTendermintHTTPServer creates an HTTP API server for Tendermint integration
func createTendermintHTTPServer(blockchain *BlockchainAdapter, mempool *MempoolAdapter, port int, defaultMinerAddress string) *http.Server {
	router := mux.NewRouter()
	
	// CORS middleware
	router.Use(func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", "*")
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
			if r.Method == "OPTIONS" {
				w.WriteHeader(http.StatusOK)
				return
			}
			next.ServeHTTP(w, r)
		})
	})
	
	// API versioning
	v1 := router.PathPrefix("/api/v1").Subrouter()
	
	// Health and status endpoints
	v1.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := blockchain.GetStats()
		health := map[string]interface{}{
			"status": "healthy",
			"height": stats.TipHeight,
			"consensus": "tendermint",
		}
		json.NewEncoder(w).Encode(health)
	}).Methods("GET", "OPTIONS")
	
	v1.HandleFunc("/status", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		stats := blockchain.GetStats()
		status := map[string]interface{}{
			"consensus": "tendermint",
			"height": stats.TipHeight,
			"total_blocks": stats.TotalBlocks,
			"genesis_hash": stats.GenesisHash,
		}
		json.NewEncoder(w).Encode(status)
	}).Methods("GET")
	
	// Basic blockchain endpoints
	v1.HandleFunc("/chain/tip", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		tip, err := blockchain.GetTip()
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		json.NewEncoder(w).Encode(tip)
	}).Methods("GET")

	// Address balance endpoint (for addresses without wallet files)
	v1.HandleFunc("/address/{address}/balance", func(w http.ResponseWriter, r *http.Request) {
		vars := mux.Vars(r)
		address := vars["address"]

		// Validate address format
		if !IsValidAddress(address) {
			http.Error(w, "Invalid address format", http.StatusBadRequest)
			return
		}

		// Proxy to explorer balance API
		resp, err := http.Get("http://localhost:10001/api/v1/wallet/" + address)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to get balance from explorer: %v", err), http.StatusInternalServerError)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			http.Error(w, "Balance API returned error", resp.StatusCode)
			return
		}

		// Forward the response directly
		w.Header().Set("Content-Type", "application/json")
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to read balance response: %v", err), http.StatusInternalServerError)
			return
		}

		w.Write(body)
	}).Methods("GET")

	// Full web wallet API endpoints (restored from original ShadowNode)
	wallet := router.PathPrefix("/wallet").Subrouter()
	wallet.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleWebWallet(w, r, blockchain, mempool)
	}).Methods("GET")
	wallet.HandleFunc("/login", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletLogin(w, r)
	}).Methods("POST")
	wallet.HandleFunc("/logout", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletLogout(w, r)
	}).Methods("POST")
	wallet.HandleFunc("/balance", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletBalance(w, r, blockchain, defaultMinerAddress)
	}).Methods("GET")
	wallet.HandleFunc("/send", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletSend(w, r, blockchain, mempool)
	}).Methods("POST")
	wallet.HandleFunc("/send_raw", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletSendRaw(w, r, mempool)
	}).Methods("POST")
	wallet.HandleFunc("/transactions", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletTransactions(w, r)
	}).Methods("GET")
	wallet.HandleFunc("/mempool", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletMempool(w, r, mempool)
	}).Methods("GET")
	wallet.HandleFunc("/peers", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletPeers(w, r)
	}).Methods("GET")
	wallet.HandleFunc("/tokens", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletTokens(w, r, blockchain)
	}).Methods("GET")
	wallet.HandleFunc("/list", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletList(w, r)
	}).Methods("GET")
	wallet.HandleFunc("/generate", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletGenerate(w, r)
	}).Methods("POST")
	wallet.HandleFunc("/network/status", func(w http.ResponseWriter, r *http.Request) {
		handleNetworkStatus(w, r)
	}).Methods("GET")
	wallet.HandleFunc("/network/peers", func(w http.ResponseWriter, r *http.Request) {
		handleNetworkPeers(w, r)
	}).Methods("GET")
	wallet.HandleFunc("/network/consensus", func(w http.ResponseWriter, r *http.Request) {
		handleNetworkConsensus(w, r)
	}).Methods("GET")
	
	// Web wallet interface routes (for the HTML UI)
	webwalletWeb := router.PathPrefix("/web/wallet").Subrouter()
	webwalletWeb.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleWebWallet(w, r, blockchain, mempool)
	}).Methods("GET")
	webwalletWeb.HandleFunc("/swap", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletSwapInterface(w, r)
	}).Methods("GET")
	webwalletWeb.HandleFunc("/swap", func(w http.ResponseWriter, r *http.Request) {
		handleWebWalletSubmitSwap(w, r, blockchain, mempool)
	}).Methods("POST")
	
	// Serve static files for web interface (WASM wallet)
	router.PathPrefix("/web/wallet/").Handler(http.StripPrefix("/web/wallet/", http.FileServer(http.Dir("./shadow-web3/wallet/"))))
	router.PathPrefix("/").Handler(http.FileServer(http.Dir("./static/")))
	
	return &http.Server{
		Addr:    fmt.Sprintf(":%d", port),
		Handler: router,
	}
}

// tendermintNetworkCmd shows network configuration help
var tendermintNetworkCmd = &cobra.Command{
	Use:   "network-setup",
	Short: "Help with network configuration and migration from directory service",
	Long: `Guide for migrating from puzzlingevidence.art directory service to Tendermint P2P discovery.

Tendermint uses a different peer discovery system than the custom directory service.
Here are the main configuration options:

1. SEED NODES (recommended for initial bootstrap):
   - Set up 1-3 reliable seed nodes that new nodes can contact
   - Seeds help with initial peer discovery then disconnect
   - Use --seeds flag: node_id@host:port,node_id2@host2:port

2. PERSISTENT PEERS (for stable connections):
   - Maintain permanent connections to specific peers
   - Good for critical validators or high-availability setups  
   - Use --persistent-peers flag: node_id@host:port

3. PEER EXCHANGE (PEX):
   - Enabled by default - peers share lists of other peers
   - Helps network grow organically after initial bootstrap

4. EXTERNAL ADDRESS:
   - Tell other peers how to reach this node
   - Required if behind NAT/firewall
   - Use --external-address flag: your.public.ip:26656

MIGRATION STEPS:
1. Pick 1-3 existing nodes to run as seed nodes
2. Get their node IDs: ./shadowy tendermint status
3. Configure other nodes with --seeds pointing to seed nodes  
4. Gradually migrate nodes to new network`,
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Println("🌐 Shadowy Network Configuration Guide")
		fmt.Println("=====================================")
		fmt.Println("")
		fmt.Println("📋 Current Node Information:")
		
		// Try to show current node info if available
		if _, err := os.Stat(filepath.Join(tendermintConfigDir, "config", "node_key.json")); err == nil {
			nodeKey, err := p2p.LoadOrGenNodeKey(filepath.Join(tendermintConfigDir, "config", "node_key.json"))
			if err == nil {
				fmt.Printf("   🆔 Node ID: %s\n", nodeKey.ID())
				fmt.Printf("   📡 P2P Address: %s@<your_ip>:26656\n", nodeKey.ID())
				fmt.Printf("   🌐 Full Seed Format: %s@<your_public_ip>:26656\n", nodeKey.ID())
			}
		} else {
			fmt.Println("   ⚠️  No node key found. Run 'init' first.")
		}
		
		fmt.Println("")
		fmt.Println("🔧 Example Usage:")
		fmt.Println("   # Start as seed node:")
		fmt.Printf("   ./shadowy tendermint --external-address %s:26656\n", "<your_public_ip>")
		fmt.Println("")
		fmt.Println("   # Connect to seed nodes:")
		fmt.Println("   ./shadowy tendermint --seeds node_id1@seed1.com:26656,node_id2@seed2.com:26656")
		fmt.Println("")
		fmt.Println("🚀 Migration from puzzlingevidence.art:")
		fmt.Println("   1. Choose existing nodes to become seed nodes")
		fmt.Println("   2. Start them with --external-address")  
		fmt.Println("   3. Get their node IDs and public IPs")
		fmt.Println("   4. Configure other nodes with --seeds")
	},
}

// Note: WebWalletSession and webWalletSessions are defined in web_wallet.go

// handleWebWallet serves the main web wallet interface (restored from original)
func handleWebWallet(w http.ResponseWriter, r *http.Request, blockchain *BlockchainAdapter, mempool *MempoolAdapter) {
	session, authenticated := validateSession(r)
	// Check if user is authenticated
	if !authenticated {
		serveLoginPage(w, r)
		return
	}
	// Serve wallet dashboard
	serveWalletDashboard(w, r, session, blockchain)
}

// Note: validateSession is defined in web_wallet.go

// serveLoginPage serves the wallet login page
func serveLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Web Wallet - Login</title>
    <style>
        body { font-family: monospace; background: #1a1a2e; color: #00ff41; padding: 20px; }
        .container { max-width: 400px; margin: 50px auto; }
        .login-form { background: rgba(0,255,65,0.1); padding: 30px; border: 1px solid #00ff41; border-radius: 10px; }
        input { width: 100%; padding: 10px; margin: 10px 0; background: #0f0f23; border: 1px solid #00ff41; color: #00ff41; }
        button { width: 100%; padding: 12px; background: #00ff41; color: #1a1a2e; border: none; font-weight: bold; cursor: pointer; }
        button:hover { background: #00cc33; }
        .error { color: #ff4444; margin: 10px 0; }
        .wallet-list { margin: 20px 0; }
        .wallet-item { padding: 8px; background: rgba(0,255,65,0.05); margin: 5px 0; cursor: pointer; }
        .wallet-item:hover { background: rgba(0,255,65,0.15); }
        #generateWalletBtn { margin-top: 15px; background: #ff6600; border: 1px solid #ff6600; }
        #generateWalletBtn:hover { background: #ff8833; }
    </style>
</head>
<body>
    <div class="container">
        <div class="login-form">
            <h2>🔐 Shadowy Web Wallet</h2>
            <p>Access your ~/.shadowy wallets</p>
            
            <form id="loginForm">
                <input type="text" id="wallet" placeholder="Wallet Name" required>
                <input type="password" id="password" placeholder="Password" required>
                <button type="submit">Login</button>
            </form>
            
            <div id="error" class="error"></div>
            
            <div class="wallet-list">
                <h3>Available Wallets:</h3>
                <div id="walletList">Loading...</div>
                <button id="generateWalletBtn" type="button">✨ Generate New Wallet</button>
            </div>
        </div>
    </div>
    
    <script>
        // Load available wallets
        fetch('/wallet/list')
            .then(r => r.json())
            .then(wallets => {
                const list = document.getElementById('walletList');
                if (wallets.length === 0) {
                    list.innerHTML = '<p>No wallets found in ~/.shadowy</p>';
                } else {
                    list.innerHTML = wallets.map(w => 
                        '<div class="wallet-item" onclick="selectWallet(\'' + w + '\')">' + w + '</div>'
                    ).join('');
                }
            })
            .catch(() => {
                document.getElementById('walletList').innerHTML = '<p>Error loading wallets</p>';
            });
        
        function selectWallet(name) {
            document.getElementById('wallet').value = name;
        }
        
        document.getElementById('loginForm').onsubmit = async (e) => {
            e.preventDefault();
            const data = {
                wallet: document.getElementById('wallet').value,
                password: document.getElementById('password').value
            };
            
            try {
                const response = await fetch('/wallet/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                
                if (response.ok) {
                    window.location.reload();
                } else {
                    const error = await response.text();
                    document.getElementById('error').textContent = error;
                }
            } catch (err) {
                document.getElementById('error').textContent = 'Login failed: ' + err.message;
            }
        };

        // Handle generate new wallet button
        document.getElementById('generateWalletBtn').onclick = async () => {
            const walletName = prompt('Enter wallet name (leave empty for auto-generated):');
            if (walletName === null) return; // User cancelled

            try {
                const response = await fetch('/wallet/generate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ name: walletName || '' })
                });

                if (response.ok) {
                    const result = await response.json();
                    alert('✅ ' + result.message + '\nAddress: ' + result.wallet.address);

                    // Refresh wallet list
                    fetch('/wallet/list')
                        .then(r => r.json())
                        .then(wallets => {
                            const list = document.getElementById('walletList');
                            if (wallets.length === 0) {
                                list.innerHTML = '<p>No wallets found in ~/.shadowy</p>';
                            } else {
                                list.innerHTML = wallets.map(w =>
                                    '<div class="wallet-item" onclick="selectWallet(\'' + w + '\')">' + w + '</div>'
                                ).join('');
                            }
                        });
                } else {
                    const error = await response.text();
                    alert('❌ Failed to generate wallet: ' + error);
                }
            } catch (err) {
                alert('❌ Error generating wallet: ' + err.message);
            }
        };
    </script>
</body>
</html>`
	w.Write([]byte(html))
}

// serveWalletDashboard serves the main wallet dashboard
func serveWalletDashboard(w http.ResponseWriter, r *http.Request, session *WebWalletSession, blockchain *BlockchainAdapter) {
	w.Header().Set("Content-Type", "text/html")
	html := fmt.Sprintf(`<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Web Wallet - Dashboard</title>
    <style>
        body { font-family: monospace; background: #1a1a2e; color: #00ff41; padding: 20px; }
        .container { max-width: 1000px; margin: 0 auto; }
        .header { display: flex; justify-content: space-between; align-items: center; margin-bottom: 30px; }
        .wallet-info { background: rgba(0,255,65,0.1); padding: 20px; border: 1px solid #00ff41; margin-bottom: 20px; }
        .section { background: rgba(0,255,65,0.05); padding: 15px; border: 1px solid #333; margin: 10px 0; }
        .balance { font-size: 1.5em; font-weight: bold; }
        .address { word-break: break-all; font-size: 0.9em; color: #cccccc; }
        button { padding: 8px 15px; background: #00ff41; color: #1a1a2e; border: none; cursor: pointer; margin: 5px; }
        button:hover { background: #00cc33; }
        .logout { background: #ff4444; color: white; }
        .send-form { display: none; background: rgba(255,255,255,0.05); padding: 20px; margin: 20px 0; }
        input, textarea { width: 100%%; padding: 8px; background: #0f0f23; border: 1px solid #00ff41; color: #00ff41; margin: 5px 0; }
        .nav { margin: 20px 0; }
        .nav button { background: #333; color: #00ff41; }
        .nav button.active { background: #00ff41; color: #1a1a2e; }
        .content { display: none; }
        .content.active { display: block; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔐 Shadowy Web Wallet</h1>
            <button class="logout" onclick="logout()">Logout</button>
        </div>
        
        <div class="wallet-info">
            <div><strong>Wallet:</strong> %s</div>
            <div><strong>Address:</strong> <span class="address">%s</span></div>
            <div class="balance">Balance: <span id="balance">Loading...</span> SHADOW</div>
        </div>
        
        <div class="nav">
            <button onclick="showSection('overview')" class="active" id="btn-overview">Overview</button>
            <button onclick="showSection('send')" id="btn-send">Send</button>
            <button onclick="showSection('transactions')" id="btn-transactions">Transactions</button>
            <button onclick="showSection('tokens')" id="btn-tokens">Tokens</button>
            <button onclick="showSection('mempool')" id="btn-mempool">Mempool</button>
            <button onclick="showSection('network')" id="btn-network">Network</button>
        </div>
        
        <div id="overview" class="content active">
            <div class="section">
                <h3>📊 Account Overview</h3>
                <div id="accountDetails">Loading account details...</div>
            </div>
        </div>
        
        <div id="send" class="content">
            <div class="section">
                <h3>💸 Send Transaction</h3>
                <form id="sendForm">
                    <input type="text" id="toAddress" placeholder="Recipient Address" required>
                    <input type="number" id="amount" placeholder="Amount (SHADOW)" step="0.00000001" required>
                    <input type="number" id="fee" placeholder="Fee (optional, default 0.011)" step="0.00000001">
                    <textarea id="message" placeholder="Message (optional)" rows="3"></textarea>
                    <button type="submit">Send Transaction</button>
                </form>
                <div id="sendResult"></div>
            </div>
        </div>
        
        <div id="transactions" class="content">
            <div class="section">
                <h3>📋 Recent Transactions</h3>
                <div id="transactionList">Loading...</div>
            </div>
        </div>
        
        <div id="tokens" class="content">
            <div class="section">
                <h3>🪙 Token Balances</h3>
                <div id="tokenList">Loading...</div>
            </div>
        </div>

        <div id="mempool" class="content">
            <div class="section">
                <h3>🔄 Mempool Status</h3>
                <div id="mempoolStats">
                    <div class="status-item">
                        <strong>Transaction Count:</strong> <span id="mempoolCount">Loading...</span>
                    </div>
                    <div class="status-item">
                        <strong>Total Size:</strong> <span id="mempoolSize">Loading...</span> bytes
                    </div>
                </div>
                <button onclick="refreshMempool()" style="margin-top: 10px; background: #00ff41; color: #000; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;">🔄 Refresh</button>
            </div>

            <div class="section" style="margin-top: 20px;">
                <h3>📋 Pending Transactions</h3>
                <div id="mempoolTransactions" style="max-height: 400px; overflow-y: auto;">Loading transactions...</div>
            </div>
        </div>

        <div id="network" class="content">
            <div class="section">
                <h3>🌐 Network Status</h3>
                <div id="networkStatus">
                    <div class="status-item">
                        <strong>Node ID:</strong> <span id="nodeId">Loading...</span>
                    </div>
                    <div class="status-item">
                        <strong>Network:</strong> <span id="networkName">Loading...</span>
                    </div>
                    <div class="status-item">
                        <strong>Latest Height:</strong> <span id="latestHeight">Loading...</span>
                    </div>
                    <div class="status-item">
                        <strong>Sync Status:</strong> <span id="syncStatus">Loading...</span>
                    </div>
                    <div class="status-item">
                        <strong>Connected Peers:</strong> <span id="peerCount">Loading...</span>
                    </div>
                </div>
                <button onclick="refreshNetworkStatus()" style="margin-top: 10px; background: #00ff41; color: #000; border: none; padding: 8px 16px; border-radius: 4px; cursor: pointer;">🔄 Refresh</button>
            </div>

            <div class="section" style="margin-top: 20px;">
                <h3>👥 Connected Peers</h3>
                <div id="peersList">Loading peers...</div>
            </div>
        </div>
    </div>
    
    <script>
        function showSection(section) {
            // Hide all content sections
            document.querySelectorAll('.content').forEach(c => c.classList.remove('active'));
            document.querySelectorAll('.nav button').forEach(b => b.classList.remove('active'));
            
            // Show selected section
            document.getElementById(section).classList.add('active');
            document.getElementById('btn-' + section).classList.add('active');
            
            // Load section data
            if (section === 'transactions') loadTransactions();
            if (section === 'tokens') loadTokens();
            if (section === 'network') loadNetworkData();
        }
        
        function logout() {
            fetch('/wallet/logout', { method: 'POST' })
                .then(() => window.location.reload());
        }
        
        // Load balance
        fetch('/wallet/balance')
            .then(r => r.json())
            .then(data => {
                document.getElementById('balance').textContent = data.balance || '0.00000000';
            })
            .catch(() => {
                document.getElementById('balance').textContent = 'Error';
            });
        
        // Handle send form
        document.getElementById('sendForm').onsubmit = async (e) => {
            e.preventDefault();
            const result = document.getElementById('sendResult');
            result.innerHTML = 'Sending transaction...';
            
            const data = {
                to_address: document.getElementById('toAddress').value,
                amount: parseFloat(document.getElementById('amount').value),
                fee: parseFloat(document.getElementById('fee').value) || 0.011,
                message: document.getElementById('message').value
            };
            
            try {
                const response = await fetch('/wallet/send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                
                const result_data = await response.json();
                if (response.ok) {
                    result.innerHTML = '<div style="color: #00ff41;">✅ Transaction sent! TX: ' + result_data.txHash + '</div>';
                } else {
                    result.innerHTML = '<div style="color: #ff4444;">❌ ' + result_data.message + '</div>';
                }
            } catch (err) {
                result.innerHTML = '<div style="color: #ff4444;">❌ Error: ' + err.message + '</div>';
            }
        };
        
        function loadTransactions() {
            fetch('/wallet/transactions')
                .then(r => r.json())
                .then(txs => {
                    const list = document.getElementById('transactionList');
                    if (!txs || txs.length === 0) {
                        list.innerHTML = '<p>No recent transactions</p>';
                    } else {
                        list.innerHTML = txs.map(tx => 
                            '<div class="section">' +
                            '<strong>' + tx.txHash + '</strong><br>' +
                            'Amount: ' + tx.amount + ' SHADOW<br>' +
                            'Date: ' + new Date(tx.timestamp).toLocaleString() +
                            '</div>'
                        ).join('');
                    }
                })
                .catch(() => {
                    document.getElementById('transactionList').innerHTML = '<p>Error loading transactions</p>';
                });
        }
        
        function loadTokens() {
            fetch('/wallet/tokens')
                .then(r => r.json())
                .then(tokens => {
                    const list = document.getElementById('tokenList');
                    if (!tokens || tokens.length === 0) {
                        list.innerHTML = '<p>No token balances</p>';
                    } else {
                        list.innerHTML = tokens.map(token => 
                            '<div class="section">' +
                            '<strong>' + token.name + ' (' + token.ticker + ')</strong><br>' +
                            'Balance: ' + token.balance + '<br>' +
                            'Token ID: ' + token.tokenId +
                            '</div>'
                        ).join('');
                    }
                })
                .catch(() => {
                    document.getElementById('tokenList').innerHTML = '<p>Error loading tokens</p>';
                });
        }

        function loadNetworkData() {
            loadNetworkStatus();
            loadPeersList();
        }

        function refreshNetworkStatus() {
            loadNetworkData();
        }

        function loadNetworkStatus() {
            fetch('/wallet/network/status')
                .then(r => r.json())
                .then(data => {
                    document.getElementById('nodeId').textContent = (data.node_id || 'Unknown').substring(0, 16) + '...';
                    document.getElementById('networkName').textContent = data.network || 'Unknown';
                    document.getElementById('latestHeight').textContent = data.latest_height || 'Unknown';
                    document.getElementById('syncStatus').textContent = data.catching_up ? 'Syncing' : 'Synced';
                    document.getElementById('syncStatus').style.color = data.catching_up ? '#f59e0b' : '#10b981';
                })
                .catch(() => {
                    document.getElementById('nodeId').textContent = 'Error';
                    document.getElementById('networkName').textContent = 'Error';
                    document.getElementById('latestHeight').textContent = 'Error';
                    document.getElementById('syncStatus').textContent = 'Error';
                });
        }

        function loadPeersList() {
            fetch('/wallet/network/peers')
                .then(r => r.json())
                .then(data => {
                    document.getElementById('peerCount').textContent = data.n_peers || '0';
                    const peersList = document.getElementById('peersList');
                    if (!data.peers || data.peers.length === 0) {
                        peersList.innerHTML = '<p>No peers connected</p>';
                    } else {
                        peersList.innerHTML = data.peers.map(peer =>
                            '<div style="border: 1px solid #333; padding: 10px; margin: 5px 0; border-radius: 4px;">' +
                            '<strong>' + (peer.moniker || 'Unknown Node') + '</strong><br>' +
                            'ID: ' + (peer.id || '').substring(0, 16) + '...<br>' +
                            'IP: ' + (peer.remote_ip || 'Unknown') + '<br>' +
                            'Version: ' + (peer.version || 'Unknown') +
                            '</div>'
                        ).join('');
                    }
                })
                .catch(() => {
                    document.getElementById('peerCount').textContent = 'Error';
                    document.getElementById('peersList').innerHTML = '<p>Error loading peers</p>';
                });
        }

        // Mempool functions
        function loadMempoolData() {
            fetch('/wallet/mempool')
                .then(r => r.json())
                .then(data => {
                    document.getElementById('mempoolCount').textContent = data.count || '0';
                    document.getElementById('mempoolSize').textContent = (data.total_size || 0).toLocaleString();

                    const transactionsList = document.getElementById('mempoolTransactions');
                    if (!data.transactions || data.transactions.length === 0) {
                        transactionsList.innerHTML = '<p>No pending transactions</p>';
                    } else {
                        transactionsList.innerHTML = data.transactions.map(tx => {
                            const statusColor = tx.validated ? '#10b981' : '#f59e0b';
                            return '<div style="border: 1px solid #333; padding: 10px; margin: 5px 0; border-radius: 4px; font-family: monospace; font-size: 12px;">' +
                                '<div style="display: flex; justify-content: space-between; margin-bottom: 5px;">' +
                                '<strong>Hash:</strong> <span>' + (tx.hash || '').substring(0, 16) + '...</span>' +
                                '</div>' +
                                '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 10px;">' +
                                '<div>Amount: ' + (tx.amount || 0).toFixed(8) + ' SHADOW</div>' +
                                '<div>Fee: ' + (tx.fee || 0).toFixed(8) + ' SHADOW</div>' +
                                '<div>Priority: ' + (tx.priority || 0).toFixed(2) + '</div>' +
                                '<div>Size: ' + (tx.size || 0) + ' bytes</div>' +
                                '<div>Inputs: ' + (tx.inputs || 0) + '</div>' +
                                '<div>Outputs: ' + (tx.outputs || 0) + '</div>' +
                                '<div>Time: ' + (tx.received_at || 'Unknown') + '</div>' +
                                '<div style="color: ' + statusColor + '">Status: ' + (tx.validated ? 'Valid' : 'Pending') + '</div>' +
                                '</div>' +
                                (tx.to_address ? '<div style="margin-top: 5px;">To: ' + tx.to_address + '</div>' : '') +
                                '<div style="margin-top: 5px;">Source: ' + (tx.source || 'unknown') + '</div>' +
                                '</div>';
                        }).join('');
                    }
                })
                .catch(() => {
                    document.getElementById('mempoolCount').textContent = 'Error';
                    document.getElementById('mempoolSize').textContent = 'Error';
                    document.getElementById('mempoolTransactions').innerHTML = '<p>Error loading mempool data</p>';
                });
        }

        function refreshMempool() {
            loadMempoolData();
        }
    </script>
</body>
</html>`, session.WalletName, session.Address)
	w.Write([]byte(html))
}

// handleWebWalletLogin handles wallet login authentication
func handleWebWalletLogin(w http.ResponseWriter, r *http.Request) {
	var loginData struct {
		Wallet   string `json:"wallet"`
		Password string `json:"password"`
	}
	if err := json.NewDecoder(r.Body).Decode(&loginData); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	// Load wallet from ~/.shadowy directory
	walletData, err := loadWalletFromFile(loginData.Wallet, loginData.Password)
	if err != nil {
		http.Error(w, "Invalid wallet or password", http.StatusUnauthorized)
		return
	}

	// Create session
	sessionID := fmt.Sprintf("session_%d_%s", time.Now().Unix(), loginData.Wallet)
	session := &WebWalletSession{
		SessionID:  sessionID,
		Address:    walletData.Address,
		WalletName: loginData.Wallet,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour), // 24 hour session
	}

	webWalletSessions[sessionID] = session

	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "shadow_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Expires:  session.ExpiresAt,
	})

	w.WriteHeader(http.StatusOK)
	json.NewEncoder(w).Encode(map[string]string{"status": "success"})
}

// handleWebWalletLogout handles logout
func handleWebWalletLogout(w http.ResponseWriter, r *http.Request) {
	sessionCookie, err := r.Cookie("shadow_session")
	if err == nil {
		delete(webWalletSessions, sessionCookie.Value)
	}

	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "shadow_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	})

	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logout successful"))
}

// handleWebWalletBalance returns wallet balance
func handleWebWalletBalance(w http.ResponseWriter, r *http.Request, blockchain *BlockchainAdapter, defaultMinerAddress string) {
	queryAddress := r.URL.Query().Get("address")
	var targetAddress string

	if queryAddress != "" {
		if !IsValidAddress(queryAddress) {
			http.Error(w, "Invalid address format", http.StatusBadRequest)
			return
		}
		targetAddress = queryAddress
	} else {
		session, authenticated := validateSession(r)
		if !authenticated {
			// Use default mining address when not authenticated
			targetAddress = defaultMinerAddress
		} else {
			targetAddress = session.Address
		}
	}

	// Get balance from explorer API
	resp, err := http.Get("http://localhost:10001/api/v1/wallet/" + targetAddress)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get balance from explorer: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		http.Error(w, "Balance API returned error", resp.StatusCode)
		return
	}

	// Parse the explorer response to extract just balance
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to read balance response: %v", err), http.StatusInternalServerError)
		return
	}

	var walletData map[string]interface{}
	if err := json.Unmarshal(body, &walletData); err != nil {
		http.Error(w, fmt.Sprintf("Failed to parse balance response: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract balance and convert from satoshis to SHADOW
	balanceSatoshis, ok := walletData["balance"].(float64)
	if !ok {
		http.Error(w, "Invalid balance format in response", http.StatusInternalServerError)
		return
	}

	balanceShadow := balanceSatoshis / 100000000.0 // Convert from satoshis to SHADOW

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"address": targetAddress,
		"balance": balanceShadow,
	})
}

// handleWebWalletSend handles sending transactions
func handleWebWalletSend(w http.ResponseWriter, r *http.Request, blockchain *BlockchainAdapter, mempool *MempoolAdapter) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	var sendData struct {
		ToAddress string  `json:"to_address"`
		Amount    float64 `json:"amount"`
		Fee       float64 `json:"fee"`
		Message   string  `json:"message"`
	}
	if err := json.NewDecoder(r.Body).Decode(&sendData); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}

	if !IsValidAddress(sendData.ToAddress) {
		http.Error(w, "Invalid destination address format", http.StatusBadRequest)
		return
	}

	// Get UTXOs for the sender address
	utxos, err := blockchain.GetUTXOsForAddress(session.Address)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"message": fmt.Sprintf("Failed to get UTXOs: %v", err),
		})
		return
	}

	// Convert amounts to atomic units (1 SHADOW = 100000000 units)
	amountUnits := uint64(sendData.Amount * 100000000)
	feeUnits := uint64(sendData.Fee * 100000000)
	totalNeeded := amountUnits + feeUnits

	// Select UTXOs to spend (simple first-fit algorithm)
	var selectedUTXOs []UTXOResponse
	var totalInput uint64
	for _, utxo := range utxos {
		selectedUTXOs = append(selectedUTXOs, utxo)
		totalInput += utxo.Value
		if totalInput >= totalNeeded {
			break
		}
	}

	if totalInput < totalNeeded {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"message": fmt.Sprintf("Insufficient balance: need %d, have %d", totalNeeded, totalInput),
			"needed_shadow": float64(totalNeeded) / 100000000,
			"available_shadow": float64(totalInput) / 100000000,
		})
		return
	}

	// Create transaction inputs from selected UTXOs
	var inputs []TransactionInput
	for _, utxo := range selectedUTXOs {
		inputs = append(inputs, TransactionInput{
			PreviousTxHash: utxo.TxID,
			OutputIndex:    utxo.Vout,
			ScriptSig:      "", // Will be filled during signing
			Sequence:       0xffffffff,
		})
	}

	// Create transaction outputs
	outputs := []TransactionOutput{
		{
			Value:        amountUnits,
			ScriptPubKey: "",
			Address:      sendData.ToAddress,
		},
	}

	// Add change output if needed
	change := totalInput - totalNeeded
	if change > 0 {
		outputs = append(outputs, TransactionOutput{
			Value:        change,
			ScriptPubKey: "",
			Address:      session.Address, // Send change back to sender
		})
	}

	// Create transaction
	tx := &Transaction{
		Version:   1,
		Inputs:    inputs,
		Outputs:   outputs,
		TokenOps:  []TokenOperation{},
		Timestamp: time.Now().UTC(),
		NotUntil:  time.Now().UTC(),
		Nonce:     uint64(time.Now().UnixNano()),
	}

	// Serialize transaction for signing
	txBytes, err := json.Marshal(tx)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"message": fmt.Sprintf("Failed to serialize transaction: %v", err),
		})
		return
	}

	// Create hash for the transaction
	hash := sha256.Sum256(txBytes)
	txHash := hex.EncodeToString(hash[:])

	// Create a simple signed transaction structure
	// TODO: Implement proper cryptographic signing with wallet keys
	signedTx := &SignedTransaction{
		TxHash:      txHash,
		SignerKey:   session.Address, // Using address as placeholder
		Algorithm:   "ML-DSA-87",
		Signature:   "placeholder_signature", // Placeholder signature
		Transaction: txBytes,
	}

	// Add to mempool using the correct signature
	err = mempool.mempool.AddTransaction(signedTx, SourceAPI)
	if err != nil {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "error",
			"message": fmt.Sprintf("Failed to add to mempool: %v", err),
		})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status":  "success",
		"message": "Transaction created with real UTXOs and added to mempool",
		"tx_hash": txHash,
		"details": map[string]interface{}{
			"inputs_used": len(selectedUTXOs),
			"total_input": float64(totalInput) / 100000000,
			"amount_sent": float64(amountUnits) / 100000000,
			"fee_paid": float64(feeUnits) / 100000000,
			"change_returned": float64(change) / 100000000,
			"outputs_created": len(outputs),
		},
		"note": "Real UTXO selection implemented. Only cryptographic signing needs wallet integration.",
	})
}

// handleWebWalletSendRaw handles sending pre-signed transactions
func handleWebWalletSendRaw(w http.ResponseWriter, r *http.Request, mempool *MempoolAdapter) {
	_, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	var signedTx SignedTransaction
	if err := json.NewDecoder(r.Body).Decode(&signedTx); err != nil {
		http.Error(w, "Invalid signed transaction format", http.StatusBadRequest)
		return
	}

	// Verify the signature and validate the transaction
	_, err := VerifySignedTransaction(&signedTx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Transaction verification failed: %v", err), http.StatusBadRequest)
		return
	}

	// Submit to mempool (fix arguments)
	if err := mempool.mempool.AddTransaction(&signedTx, SourceAPI); err != nil {
		http.Error(w, fmt.Sprintf("Failed to add to mempool: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "success",
		"txHash": signedTx.TxHash,
	})
}

// handleWebWalletTransactions returns recent transactions
func handleWebWalletTransactions(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Get transactions from explorer API (consistent with UTXO implementation)
	log.Printf("DEBUG: Getting transactions for address: %s", session.Address)
	resp, err := http.Get("http://localhost:10001/api/v1/wallet/" + session.Address)
	if err != nil {
		log.Printf("Failed to get transactions from explorer: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		log.Printf("Explorer API returned status: %d", resp.StatusCode)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.Printf("Failed to read explorer response: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	var walletData struct {
		Transactions []struct {
			TxHash      string `json:"tx_hash"`
			BlockHeight uint64 `json:"block_height"`
			Type        string `json:"type"`
			Amount      uint64 `json:"amount"`
			ToAddress   string `json:"to_address"`
			FromAddress string `json:"from_address"`
			Timestamp   string `json:"timestamp"`
		} `json:"transactions"`
	}

	if err := json.Unmarshal(body, &walletData); err != nil {
		log.Printf("Failed to parse explorer response: %v", err)
		log.Printf("Explorer response body: %s", string(body))
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]interface{}{})
		return
	}

	log.Printf("DEBUG: Parsed %d transactions from explorer", len(walletData.Transactions))

	// Format transactions for web wallet display (limit to 20)
	var formattedTxs []map[string]interface{}
	limit := 20
	for i, tx := range walletData.Transactions {
		if i >= limit {
			break
		}
		formattedTxs = append(formattedTxs, map[string]interface{}{
			"txHash":    tx.TxHash,
			"amount":    float64(tx.Amount) / 100000000.0, // Convert to SHADOW
			"type":      tx.Type,
			"timestamp": tx.Timestamp, // Keep as string for now
			"to":        tx.ToAddress,
			"from":      tx.FromAddress,
		})
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(formattedTxs)
}

// handleWebWalletMempool serves the mempool viewer page
func handleWebWalletMempool(w http.ResponseWriter, r *http.Request, mempool *MempoolAdapter) {
	_, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// Get mempool transactions (limit to 50 for display)
	transactions := mempool.mempool.GetHighestPriorityTransactions(50)

	// Format transactions for display
	var formattedTxs []map[string]interface{}
	for _, mempoolTx := range transactions {
		// Try to parse the transaction to get readable details
		var parsedTx Transaction
		var totalOutput uint64
		var inputCount, outputCount int
		var toAddress string

		if err := json.Unmarshal([]byte(mempoolTx.Transaction.Transaction), &parsedTx); err == nil {
			// Successfully parsed transaction
			inputCount = len(parsedTx.Inputs)
			outputCount = len(parsedTx.Outputs)

			// Calculate total output amount
			for _, output := range parsedTx.Outputs {
				totalOutput += output.Value
			}

			// Get first output address if available
			if len(parsedTx.Outputs) > 0 {
				toAddress = parsedTx.Outputs[0].Address
			}
		} else {
			// Couldn't parse transaction, use defaults
			totalOutput = 0
			inputCount = 0
			outputCount = 0
			toAddress = "Unknown"
		}

		formattedTx := map[string]interface{}{
			"hash":         mempoolTx.TxHash,
			"fee":          float64(mempoolTx.Fee) / 100000000, // Convert to SHADOW
			"amount":       float64(totalOutput) / 100000000,   // Convert to SHADOW
			"priority":     mempoolTx.Priority,
			"received_at":  mempoolTx.ReceivedAt.Format("15:04:05"),
			"source":       mempoolTx.Source.String(),
			"size":         mempoolTx.Size,
			"inputs":       inputCount,
			"outputs":      outputCount,
			"validated":    mempoolTx.IsValidated,
			"to_address":   toAddress,
		}

		formattedTxs = append(formattedTxs, formattedTx)
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"transactions": formattedTxs,
		"count":        len(formattedTxs),
	})
}

// handleWebWalletPeers serves the network peers viewer
func handleWebWalletPeers(w http.ResponseWriter, r *http.Request) {
	_, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"peers": []interface{}{},
		"count": 0,
	})
}

// handleWebWalletTokens returns wallet token balances
func handleWebWalletTokens(w http.ResponseWriter, r *http.Request, blockchain *BlockchainAdapter) {
	_, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	// For now, return empty tokens array with proper structure
	// Future implementation would check blockchain.GetTokenState() if available
	var tokens []map[string]interface{}

	// Check if blockchain has token state (this method may not exist in current implementation)
	// In a future version, this would be:
	// if tokenState := blockchain.GetTokenState(); tokenState != nil {
	//     balances, err := tokenState.GetAllTokenBalances(session.Address)
	//     if err == nil {
	//         for tokenId, balance := range balances {
	//             tokens = append(tokens, map[string]interface{}{
	//                 "tokenId": tokenId,
	//                 "balance": balance,
	//                 "name": "Token Name", // Would be fetched from token metadata
	//                 "ticker": "TKN",      // Would be fetched from token metadata
	//             })
	//         }
	//     }
	// }

	// For now, return empty array (no tokens implemented yet)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(tokens)
}

// handleWebWalletList returns available wallets in ~/.shadowy directory
func handleWebWalletList(w http.ResponseWriter, r *http.Request) {
	wallets, err := listWalletsFromDirectory()
	if err != nil {
		log.Printf("Error listing wallets: %v", err)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode([]string{})
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(wallets)
}

// handleWebWalletGenerate creates a new wallet with post-quantum cryptography
func handleWebWalletGenerate(w http.ResponseWriter, r *http.Request) {
	type GenerateRequest struct {
		Name string `json:"name"`
	}

	var req GenerateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}

	if req.Name == "" {
		req.Name = fmt.Sprintf("wallet_%d", time.Now().Unix())
	}

	// Generate new wallet
	wallet, err := generateNewWallet(req.Name)
	if err != nil {
		log.Printf("Error generating wallet: %v", err)
		http.Error(w, fmt.Sprintf("Failed to generate wallet: %v", err), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"success": true,
		"wallet": wallet,
		"message": fmt.Sprintf("Wallet '%s' generated successfully", wallet.Name),
	})
}

// handleWebWalletSwapInterface serves the LP swap interface page
func handleWebWalletSwapInterface(w http.ResponseWriter, r *http.Request) {
	_, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte("<h1>Swap Interface - Not yet implemented</h1>"))
}

// handleWebWalletSubmitSwap handles LP swap transaction submission
func handleWebWalletSubmitSwap(w http.ResponseWriter, r *http.Request, blockchain *BlockchainAdapter, mempool *MempoolAdapter) {
	_, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(map[string]interface{}{
		"status": "error",
		"message": "Swap functionality not yet implemented",
	})
}

// Helper functions for wallet operations

// loadWalletFromFile loads a wallet from ~/.shadowy directory
func loadWalletFromFile(walletName, password string) (*WalletData, error) {
	wallet, err := loadWallet(walletName)
	if err != nil {
		return nil, err
	}

	return &WalletData{
		Name:    wallet.Name,
		Address: wallet.Address,
		Version: fmt.Sprintf("V%d", wallet.Version),
	}, nil
}

// listWalletsFromDirectory lists available wallets in ~/.shadowy
func listWalletsFromDirectory() ([]string, error) {
	wallets, err := listWallets()
	if err != nil {
		return nil, err
	}

	// Extract wallet names
	var names []string
	for _, wallet := range wallets {
		names = append(names, wallet.Name)
	}

	return names, nil
}

// generateNewWallet creates a new wallet with the specified name
func generateNewWallet(walletName string) (*WalletData, error) {
	// Ensure wallet directory exists
	if err := ensureWalletDir(); err != nil {
		return nil, fmt.Errorf("failed to create wallet directory: %w", err)
	}

	// Generate new wallet
	keyPair, err := GenerateKeyPair()
	if err != nil {
		return nil, fmt.Errorf("failed to generate key pair: %w", err)
	}

	// Format address with S prefix (matching existing wallet format)
	address := "S" + keyPair.AddressHex()

	// Create wallet file structure
	wallet := WalletFile{
		Name:       walletName,
		Address:    address,
		PrivateKey: keyPair.PrivateKeyHex(),
		PublicKey:  keyPair.PublicKeyHex(),
		Identifier: generateWalletIdentifier(address),
		CreatedAt:  time.Now(),
		Version:    3, // Current version
	}

	// Save wallet to file
	_, err = saveWallet(wallet)
	if err != nil {
		return nil, fmt.Errorf("failed to save wallet: %w", err)
	}

	return &WalletData{
		Name:    wallet.Name,
		Address: wallet.Address,
		Version: fmt.Sprintf("V%d", wallet.Version),
	}, nil
}

// generateWalletIdentifier creates a unique identifier for the wallet
func generateWalletIdentifier(address string) string {
	return fmt.Sprintf("wallet_%s_%d", address[:8], time.Now().Unix())
}

// WalletData represents loaded wallet data
type WalletData struct {
	Name    string
	Address string
	Version string
}

// Network status handlers for the wallet interface
func handleNetworkStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Call Tendermint RPC status endpoint
	resp, err := http.Get("http://localhost:26657/status")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get Tendermint status: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var tendermintStatus map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tendermintStatus); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode Tendermint status: %v", err), http.StatusInternalServerError)
		return
	}

	// Extract relevant information
	result := tendermintStatus["result"].(map[string]interface{})
	nodeInfo := result["node_info"].(map[string]interface{})
	syncInfo := result["sync_info"].(map[string]interface{})
	validatorInfo := result["validator_info"].(map[string]interface{})

	status := map[string]interface{}{
		"node_id":         nodeInfo["id"],
		"network":         nodeInfo["network"],
		"version":         nodeInfo["version"],
		"latest_height":   syncInfo["latest_block_height"],
		"latest_hash":     syncInfo["latest_block_hash"],
		"latest_time":     syncInfo["latest_block_time"],
		"catching_up":     syncInfo["catching_up"],
		"validator_power": validatorInfo["voting_power"],
		"validator_address": validatorInfo["address"],
		"status":          "healthy",
	}

	json.NewEncoder(w).Encode(status)
}

func handleNetworkPeers(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Call Tendermint RPC net_info endpoint
	resp, err := http.Get("http://localhost:26657/net_info")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get network info: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var netInfo map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&netInfo); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode network info: %v", err), http.StatusInternalServerError)
		return
	}

	result := netInfo["result"].(map[string]interface{})
	peers := result["peers"].([]interface{})

	peerList := make([]map[string]interface{}, 0)
	for _, peer := range peers {
		p := peer.(map[string]interface{})
		nodeInfo := p["node_info"].(map[string]interface{})
		connInfo := p["connection_status"].(map[string]interface{})

		peerInfo := map[string]interface{}{
			"id":           nodeInfo["id"],
			"network":      nodeInfo["network"],
			"version":      nodeInfo["version"],
			"moniker":      nodeInfo["moniker"],
			"remote_ip":    p["remote_ip"],
			"is_outbound":  connInfo["SendMonitor"].(map[string]interface{})["Active"],
			"duration":     connInfo["Duration"],
		}
		peerList = append(peerList, peerInfo)
	}

	response := map[string]interface{}{
		"listening":       result["listening"],
		"n_peers":         result["n_peers"],
		"peers":           peerList,
	}

	json.NewEncoder(w).Encode(response)
}

func handleNetworkConsensus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	// Get consensus state
	resp, err := http.Get("http://localhost:26657/consensus_state")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get consensus state: %v", err), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	var consensusState map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&consensusState); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode consensus state: %v", err), http.StatusInternalServerError)
		return
	}

	// Get validators
	validatorsResp, err := http.Get("http://localhost:26657/validators")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get validators: %v", err), http.StatusInternalServerError)
		return
	}
	defer validatorsResp.Body.Close()

	var validators map[string]interface{}
	if err := json.NewDecoder(validatorsResp.Body).Decode(&validators); err != nil {
		http.Error(w, fmt.Sprintf("Failed to decode validators: %v", err), http.StatusInternalServerError)
		return
	}

	result := consensusState["result"].(map[string]interface{})
	roundState := result["round_state"].(map[string]interface{})

	validatorResult := validators["result"].(map[string]interface{})

	response := map[string]interface{}{
		"height":           roundState["height"],
		"round":            roundState["round"],
		"step":             roundState["step"],
		"start_time":       roundState["start_time"],
		"commit_time":      roundState["commit_time"],
		"validators":       validatorResult["validators"],
		"total_validators": validatorResult["total"],
	}

	json.NewEncoder(w).Encode(response)
}

// runBootstrapNode initializes a new node from a bootstrap package
func runBootstrapNode(cmd *cobra.Command, args []string) {
	var archivePath string

	// Use provided archive or look for default bootstrap package
	if len(args) > 0 {
		archivePath = args[0]
	} else {
		// Look for bootstrap package in current directory
		archivePath = "shadowy-bootstrap-20250917.tar.gz"
	}

	// Check if archive exists
	if _, err := os.Stat(archivePath); os.IsNotExist(err) {
		log.Fatalf("Bootstrap archive not found: %s", archivePath)
	}

	log.Printf("Extracting bootstrap package: %s", archivePath)

	// Create tendermint-config directory if it doesn't exist
	configDir := "tendermint-config"
	if err := os.MkdirAll(configDir, 0755); err != nil {
		log.Fatalf("Failed to create config directory: %v", err)
	}

	// Extract the archive
	if err := extractBootstrapArchive(archivePath, "."); err != nil {
		log.Fatalf("Failed to extract bootstrap archive: %v", err)
	}

	// Copy configuration files from extracted directory to tendermint-config
	sourceDir := "shadowy-bootstrap/config"
	if _, err := os.Stat(sourceDir); err == nil {
		// Copy all files from extracted config directory to tendermint-config
		if err := copyConfigFiles(sourceDir, configDir); err != nil {
			log.Fatalf("Failed to copy config files: %v", err)
		}
		log.Printf("📁 Configuration files copied to %s/", configDir)

		// Clean up extracted directory
		os.RemoveAll("shadowy-bootstrap")
		log.Printf("🧹 Cleaned up temporary files")
	}

	log.Printf("✅ Bootstrap setup complete!")
	log.Printf("🚀 You can now start your node with: ./shadowy-tendermint tendermint")
	log.Printf("🌐 Network: shadowy-testnet")
	log.Printf("📋 Remember to configure unique node_key.json and priv_validator_key.json")
}

// extractBootstrapArchive extracts a tar.gz bootstrap archive
func extractBootstrapArchive(archivePath, destDir string) error {
	// Open the archive file
	file, err := os.Open(archivePath)
	if err != nil {
		return fmt.Errorf("failed to open archive: %v", err)
	}
	defer file.Close()

	// Create gzip reader
	gzReader, err := gzip.NewReader(file)
	if err != nil {
		return fmt.Errorf("failed to create gzip reader: %v", err)
	}
	defer gzReader.Close()

	// Create tar reader
	tarReader := tar.NewReader(gzReader)

	// Extract files
	for {
		header, err := tarReader.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return fmt.Errorf("error reading tar: %v", err)
		}

		// Get the full path for extraction
		targetPath := filepath.Join(destDir, header.Name)

		// Basic path traversal protection (prevent ../ attacks)
		if strings.Contains(header.Name, "..") {
			return fmt.Errorf("invalid file path (path traversal attempt): %s", header.Name)
		}

		switch header.Typeflag {
		case tar.TypeDir:
			// Create directory
			if err := os.MkdirAll(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to create directory %s: %v", targetPath, err)
			}
			log.Printf("📁 Created directory: %s", targetPath)

		case tar.TypeReg:
			// Create parent directory if it doesn't exist
			if err := os.MkdirAll(filepath.Dir(targetPath), 0755); err != nil {
				return fmt.Errorf("failed to create parent directory: %v", err)
			}

			// Create and write file
			outFile, err := os.Create(targetPath)
			if err != nil {
				return fmt.Errorf("failed to create file %s: %v", targetPath, err)
			}

			if _, err := io.Copy(outFile, tarReader); err != nil {
				outFile.Close()
				return fmt.Errorf("failed to write file %s: %v", targetPath, err)
			}
			outFile.Close()

			// Set file permissions
			if err := os.Chmod(targetPath, os.FileMode(header.Mode)); err != nil {
				return fmt.Errorf("failed to set permissions on %s: %v", targetPath, err)
			}

			log.Printf("📄 Extracted file: %s", targetPath)
		}
	}

	return nil
}

// copyConfigFiles copies all files from source directory to destination directory
func copyConfigFiles(sourceDir, destDir string) error {
	// Ensure destination directory exists
	if err := os.MkdirAll(destDir, 0755); err != nil {
		return fmt.Errorf("failed to create destination directory: %v", err)
	}

	// Read source directory
	entries, err := os.ReadDir(sourceDir)
	if err != nil {
		return fmt.Errorf("failed to read source directory: %v", err)
	}

	// Copy each file
	for _, entry := range entries {
		if entry.IsDir() {
			continue // Skip subdirectories for now
		}

		sourcePath := filepath.Join(sourceDir, entry.Name())
		destPath := filepath.Join(destDir, entry.Name())

		// Copy file
		if err := copyFile(sourcePath, destPath); err != nil {
			return fmt.Errorf("failed to copy %s: %v", entry.Name(), err)
		}

		log.Printf("📄 Copied: %s", entry.Name())
	}

	return nil
}

// copyFile copies a single file from source to destination
func copyFile(sourcePath, destPath string) error {
	sourceFile, err := os.Open(sourcePath)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destFile, err := os.Create(destPath)
	if err != nil {
		return err
	}
	defer destFile.Close()

	_, err = io.Copy(destFile, sourceFile)
	if err != nil {
		return err
	}

	// Copy file permissions
	sourceInfo, err := sourceFile.Stat()
	if err != nil {
		return err
	}

	return os.Chmod(destPath, sourceInfo.Mode())
}

func init() {
	tendermintCmd.AddCommand(tendermintInitCmd)
	tendermintCmd.AddCommand(tendermintStatusCmd)
	tendermintCmd.AddCommand(tendermintNetworkCmd)
	tendermintCmd.AddCommand(bootstrapCmd)
}