package cmd

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/spf13/cobra"
	"google.golang.org/grpc"
)

// NodeConfig contains configuration for the Shadowy node
type NodeConfig struct {
	// Service configurations
	ShadowConfig    *ShadowConfig    `json:"shadow_config"`
	MempoolConfig   *MempoolConfig   `json:"mempool_config"`
	TimelordConfig  *TimelordConfig  `json:"timelord_config"`
	ConsensusConfig *ConsensusConfig `json:"consensus_config"`
	
	// Network configuration
	HTTPPort         int      `json:"http_port"`
	GRPCPort         int      `json:"grpc_port"`
	EnableHTTP       bool     `json:"enable_http"`
	EnableGRPC       bool     `json:"enable_grpc"`
	EnableFarming    bool     `json:"enable_farming"`
	EnableTimelord   bool     `json:"enable_timelord"`
	EnableMining     bool     `json:"enable_mining"`
	EnableConsensus  bool     `json:"enable_consensus"`
	MiningAddress    string   `json:"mining_address"`
	BootstrapPeers   []string `json:"bootstrap_peers"`
	
	// Service-specific settings
	MaxConnections    int           `json:"max_connections"`
	ShutdownTimeout   time.Duration `json:"shutdown_timeout"`
	HealthCheckPeriod time.Duration `json:"health_check_period"`
}

// DefaultNodeConfig returns default node configuration
func DefaultNodeConfig() *NodeConfig {
	return &NodeConfig{
		ShadowConfig:      defaultShadowConfig(),
		MempoolConfig:     DefaultMempoolConfig(),
		TimelordConfig:    DefaultTimelordConfig(),
		ConsensusConfig:   DefaultConsensusConfig(),
		HTTPPort:          8080,
		GRPCPort:          9090,
		EnableHTTP:        true,
		EnableGRPC:        true,
		EnableFarming:     true,
		EnableTimelord:    false, // Disabled by default (resource intensive)
		EnableMining:      true,  // Enabled by default
		EnableConsensus:   true,  // Enabled by default
		MiningAddress:     "",    // Will be set from default wallet
		BootstrapPeers:    []string{}, // No bootstrap peers by default
		MaxConnections:    1000,
		ShutdownTimeout:   30 * time.Second,
		HealthCheckPeriod: 30 * time.Second,
	}
}

// ShadowNode represents the main Shadowy node service
type ShadowNode struct {
	config *NodeConfig
	
	// Core services
	mempool        *Mempool
	timelord       *Timelord
	farmingService *FarmingService
	blockchain     *Blockchain
	miner          *Miner
	consensus      *ConsensusEngine
	
	// Network services
	httpServer *http.Server
	grpcServer *grpc.Server
	
	// Service management
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
	shutdownCh chan os.Signal
	
	// Health monitoring
	healthStatus map[string]ServiceHealth
	healthMutex  sync.RWMutex
}

// ServiceHealth represents the health status of a service
type ServiceHealth struct {
	Name      string    `json:"name"`
	Status    string    `json:"status"`
	LastCheck time.Time `json:"last_check"`
	Error     string    `json:"error,omitempty"`
	Metrics   map[string]interface{} `json:"metrics,omitempty"`
}

// NewShadowNode creates a new Shadowy node
func NewShadowNode(config *NodeConfig) (*ShadowNode, error) {
	if config == nil {
		config = DefaultNodeConfig()
	}
	
	ctx, cancel := context.WithCancel(context.Background())
	
	node := &ShadowNode{
		config:       config,
		ctx:          ctx,
		cancel:       cancel,
		shutdownCh:   make(chan os.Signal, 1),
		healthStatus: make(map[string]ServiceHealth),
	}
	
	// Initialize core services
	if err := node.initializeServices(); err != nil {
		cancel()
		return nil, fmt.Errorf("failed to initialize services: %w", err)
	}
	
	// Setup signal handling
	signal.Notify(node.shutdownCh, syscall.SIGINT, syscall.SIGTERM)
	
	return node, nil
}

// initializeServices initializes all node services
func (sn *ShadowNode) initializeServices() error {
	log.Printf("Initializing Shadowy node services...")
	
	// Initialize blockchain
	blockchain, err := NewBlockchain(sn.config.ShadowConfig)
	if err != nil {
		return fmt.Errorf("failed to initialize blockchain: %w", err)
	}
	sn.blockchain = blockchain
	sn.updateHealthStatus("blockchain", "healthy", nil, map[string]interface{}{
		"data_directory": sn.config.ShadowConfig.BlockchainDirectory,
	})

	// Initialize mempool
	sn.mempool = NewMempool(sn.config.MempoolConfig)
	sn.updateHealthStatus("mempool", "healthy", nil, map[string]interface{}{
		"max_size": sn.config.MempoolConfig.MaxMempoolSize,
		"max_txs":  sn.config.MempoolConfig.MaxTransactions,
	})
	
	// Initialize timelord (if enabled)
	if sn.config.EnableTimelord {
		sn.timelord = NewTimelord(sn.config.TimelordConfig)
		sn.updateHealthStatus("timelord", "healthy", nil, map[string]interface{}{
			"workers":    sn.config.TimelordConfig.WorkerPoolSize,
			"difficulty": sn.config.TimelordConfig.VDFConfig.TimeParameter,
		})
	}
	
	// Initialize farming service (if enabled)
	if sn.config.EnableFarming {
		sn.farmingService = NewFarmingService(sn.config.ShadowConfig)
		sn.updateHealthStatus("farming", "healthy", nil, map[string]interface{}{
			"plot_directories": len(sn.config.ShadowConfig.PlotDirectories),
		})
	}
	
	// Initialize miner (if enabled)
	if sn.config.EnableMining {
		// Get mining address (use default wallet if not specified)
		miningAddress := sn.config.MiningAddress
		if miningAddress == "" {
			// Try to get address from first available wallet
			wallets, err := listWallets()
			if err == nil && len(wallets) > 0 {
				if wallet, err := loadWallet(wallets[0].Name); err == nil {
					miningAddress = wallet.Address
				}
			}
			// Fallback to genesis address if no wallets
			if miningAddress == "" {
				miningAddress = "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
			}
		}
		
		sn.miner = NewMiner(sn.config.ShadowConfig, sn.blockchain, sn.mempool, sn.farmingService, miningAddress)
		sn.updateHealthStatus("miner", "healthy", nil, map[string]interface{}{
			"mining_address": miningAddress,
		})
		log.Printf("Miner initialized with address: %s", miningAddress)
	}
	
	// Initialize consensus engine (if enabled)
	if sn.config.EnableConsensus {
		sn.consensus = NewConsensusEngine(sn.config.ConsensusConfig, sn.blockchain, sn.mempool)
		sn.updateHealthStatus("consensus", "healthy", nil, map[string]interface{}{
			"node_id":     sn.consensus.nodeID,
			"listen_addr": sn.consensus.listenAddr,
			"max_peers":   sn.config.ConsensusConfig.MaxPeers,
		})
		log.Printf("Consensus engine initialized with Node ID: %s", sn.consensus.nodeID)
	}
	
	// Initialize HTTP server (if enabled)
	if sn.config.EnableHTTP {
		if err := sn.initializeHTTPServer(); err != nil {
			return fmt.Errorf("failed to initialize HTTP server: %w", err)
		}
	}
	
	// Initialize gRPC server (if enabled)
	if sn.config.EnableGRPC {
		if err := sn.initializeGRPCServer(); err != nil {
			return fmt.Errorf("failed to initialize gRPC server: %w", err)
		}
	}
	
	log.Printf("All services initialized successfully")
	return nil
}

// Start starts all node services
func (sn *ShadowNode) Start() error {
	log.Printf("Starting Shadowy node...")
	
	// Start timelord service
	if sn.config.EnableTimelord && sn.timelord != nil {
		sn.wg.Add(1)
		go func() {
			defer sn.wg.Done()
			if err := sn.timelord.Start(); err != nil {
				log.Printf("Timelord service error: %v", err)
				sn.updateHealthStatus("timelord", "unhealthy", err, nil)
			}
		}()
	}
	
	// Start farming service
	if sn.config.EnableFarming && sn.farmingService != nil {
		sn.wg.Add(1)
		go func() {
			defer sn.wg.Done()
			if err := sn.farmingService.Start(); err != nil {
				log.Printf("Farming service error: %v", err)
				sn.updateHealthStatus("farming", "unhealthy", err, nil)
			}
		}()
	}
	
	// Start consensus engine
	if sn.config.EnableConsensus && sn.consensus != nil {
		sn.wg.Add(1)
		go func() {
			defer sn.wg.Done()
			if err := sn.consensus.Start(); err != nil {
				log.Printf("Consensus engine error: %v", err)
				sn.updateHealthStatus("consensus", "unhealthy", err, nil)
			}
		}()
	}
	
	// Start miner (after farming is ready)
	if sn.config.EnableMining && sn.miner != nil {
		// Wait a moment for farming service to be ready
		go func() {
			time.Sleep(5 * time.Second) // Give farming service time to start
			sn.wg.Add(1)
			defer sn.wg.Done()
			if err := sn.miner.Start(); err != nil {
				log.Printf("Miner error: %v", err)
				sn.updateHealthStatus("miner", "unhealthy", err, nil)
			}
		}()
	}
	
	// Start HTTP server
	if sn.config.EnableHTTP && sn.httpServer != nil {
		sn.wg.Add(1)
		go func() {
			defer sn.wg.Done()
			log.Printf("Starting HTTP server on port %d", sn.config.HTTPPort)
			if err := sn.httpServer.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				log.Printf("HTTP server error: %v", err)
				sn.updateHealthStatus("http", "unhealthy", err, nil)
			}
		}()
	}
	
	// Start gRPC server
	if sn.config.EnableGRPC && sn.grpcServer != nil {
		sn.wg.Add(1)
		go func() {
			defer sn.wg.Done()
			log.Printf("Starting gRPC server on port %d", sn.config.GRPCPort)
			if err := sn.startGRPCServer(); err != nil {
				log.Printf("gRPC server error: %v", err)
				sn.updateHealthStatus("grpc", "unhealthy", err, nil)
			}
		}()
	}
	
	// Start health monitoring
	sn.wg.Add(1)
	go sn.healthMonitor()
	
	// Start main event loop
	sn.wg.Add(1)
	go sn.mainLoop()
	
	log.Printf("Shadowy node started successfully")
	return nil
}

// Stop gracefully shuts down all node services
func (sn *ShadowNode) Stop() error {
	log.Printf("Shutting down Shadowy node...")
	
	// Cancel context to signal all services to stop
	sn.cancel()
	
	// Create shutdown timeout context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), sn.config.ShutdownTimeout)
	defer shutdownCancel()
	
	// Shutdown HTTP server
	if sn.httpServer != nil {
		if err := sn.httpServer.Shutdown(shutdownCtx); err != nil {
			log.Printf("HTTP server shutdown error: %v", err)
		}
	}
	
	// Shutdown gRPC server
	if sn.grpcServer != nil {
		sn.grpcServer.GracefulStop()
	}
	
	// Stop timelord
	if sn.timelord != nil {
		if err := sn.timelord.Stop(); err != nil {
			log.Printf("Timelord shutdown error: %v", err)
		}
	}
	
	// Stop consensus engine
	if sn.consensus != nil {
		if err := sn.consensus.Stop(); err != nil {
			log.Printf("Consensus engine shutdown error: %v", err)
		}
	}
	
	// Stop miner
	if sn.miner != nil {
		if err := sn.miner.Stop(); err != nil {
			log.Printf("Miner shutdown error: %v", err)
		}
	}
	
	// Stop farming service
	if sn.farmingService != nil {
		if err := sn.farmingService.Stop(); err != nil {
			log.Printf("Farming service shutdown error: %v", err)
		}
	}
	
	// Wait for all goroutines to finish or timeout
	done := make(chan struct{})
	go func() {
		sn.wg.Wait()
		close(done)
	}()
	
	select {
	case <-done:
		log.Printf("All services shut down gracefully")
	case <-shutdownCtx.Done():
		log.Printf("Shutdown timeout reached, forcing exit")
	}
	
	return nil
}

// mainLoop runs the main node event loop
func (sn *ShadowNode) mainLoop() {
	defer sn.wg.Done()
	
	log.Printf("Starting main event loop")
	
	for {
		select {
		case <-sn.ctx.Done():
			log.Printf("Main loop shutting down")
			return
			
		case sig := <-sn.shutdownCh:
			log.Printf("Received signal %v, initiating shutdown", sig)
			go func() {
				if err := sn.Stop(); err != nil {
					log.Printf("Error during shutdown: %v", err)
				}
			}()
			return
			
		default:
			// Main processing loop - this is where we'd handle:
			// - Incoming transactions
			// - Block validation
			// - Consensus operations
			// - Peer communication
			
			// For now, just sleep to prevent busy loop
			time.Sleep(100 * time.Millisecond)
		}
	}
}

// healthMonitor monitors the health of all services
func (sn *ShadowNode) healthMonitor() {
	defer sn.wg.Done()
	
	ticker := time.NewTicker(sn.config.HealthCheckPeriod)
	defer ticker.Stop()
	
	for {
		select {
		case <-sn.ctx.Done():
			return
		case <-ticker.C:
			sn.performHealthChecks()
		}
	}
}

// performHealthChecks checks the health of all services
func (sn *ShadowNode) performHealthChecks() {
	// Check blockchain health
	if sn.blockchain != nil {
		stats := sn.blockchain.GetStats()
		sn.updateHealthStatus("blockchain", "healthy", nil, map[string]interface{}{
			"tip_height":         stats.TipHeight,
			"total_blocks":       stats.TotalBlocks,
			"total_transactions": stats.TotalTransactions,
			"avg_block_size":     stats.AvgBlockSize,
		})
	}

	// Check mempool health
	if sn.mempool != nil {
		stats := sn.mempool.GetStats()
		sn.updateHealthStatus("mempool", "healthy", nil, map[string]interface{}{
			"transaction_count": stats.TransactionCount,
			"total_size":       stats.TotalSize,
			"valid_txs":        stats.ValidationStats.ValidTransactions,
		})
	}
	
	// Check timelord health
	if sn.timelord != nil {
		stats := sn.timelord.GetStats()
		sn.updateHealthStatus("timelord", "healthy", nil, map[string]interface{}{
			"total_jobs":     stats.TotalJobs,
			"completed_jobs": stats.CompletedJobs,
			"pending_jobs":   stats.PendingJobs,
			"avg_proof_time": stats.AverageProofTime.String(),
		})
	}
	
	// Check farming service health
	if sn.farmingService != nil {
		stats := sn.farmingService.GetStats()
		status := "healthy"
		if !sn.farmingService.IsRunning() {
			status = "unhealthy"
		}
		sn.updateHealthStatus("farming", status, nil, map[string]interface{}{
			"plot_files_indexed":   stats.PlotFilesIndexed,
			"total_keys":          stats.TotalKeys,
			"challenges_handled":   stats.ChallengesHandled,
			"last_challenge_time": stats.LastChallengeTime,
			"avg_response_time":   stats.AverageResponseTime.String(),
			"error_count":         stats.ErrorCount,
			"database_size":       stats.DatabaseSize,
		})
	}
	
	// Check miner health
	if sn.miner != nil {
		stats := sn.miner.GetStats()
		status := "healthy"
		if !sn.miner.IsRunning() {
			status = "unhealthy"
		}
		sn.updateHealthStatus("miner", status, nil, map[string]interface{}{
			"blocks_mined":        stats.BlocksMined,
			"total_rewards":       stats.TotalRewards,
			"mining_address":      sn.miner.GetMiningAddress(),
			"avg_block_time":      stats.AverageBlockTime.String(),
			"proof_success_rate":  stats.ProofSuccessRate,
			"fees_collected":      stats.FeesCollected,
		})
	}
	
	// Check consensus engine health
	if sn.consensus != nil {
		peers := sn.consensus.GetPeers()
		syncStatus := sn.consensus.GetSyncStatus()
		chainState := sn.consensus.GetChainState()
		
		sn.updateHealthStatus("consensus", "healthy", nil, map[string]interface{}{
			"node_id":         sn.consensus.nodeID,
			"peer_count":      len(peers),
			"is_syncing":      syncStatus.IsSyncing,
			"sync_progress":   syncStatus.SyncProgress,
			"chain_height":    chainState.Height,
			"chain_hash":      chainState.Hash,
		})
	}
}

// updateHealthStatus updates the health status of a service
func (sn *ShadowNode) updateHealthStatus(serviceName, status string, err error, metrics map[string]interface{}) {
	sn.healthMutex.Lock()
	defer sn.healthMutex.Unlock()
	
	health := ServiceHealth{
		Name:      serviceName,
		Status:    status,
		LastCheck: time.Now().UTC(),
		Metrics:   metrics,
	}
	
	if err != nil {
		health.Error = err.Error()
	}
	
	sn.healthStatus[serviceName] = health
}

// GetHealthStatus returns the current health status of all services
func (sn *ShadowNode) GetHealthStatus() map[string]ServiceHealth {
	sn.healthMutex.RLock()
	defer sn.healthMutex.RUnlock()
	
	// Return a copy to avoid race conditions
	result := make(map[string]ServiceHealth)
	for k, v := range sn.healthStatus {
		result[k] = v
	}
	
	return result
}

// GetMempool returns the node's mempool
func (sn *ShadowNode) GetMempool() *Mempool {
	return sn.mempool
}

// GetTimelord returns the node's timelord service
func (sn *ShadowNode) GetTimelord() *Timelord {
	return sn.timelord
}

// GetFarmingService returns the node's farming service
func (sn *ShadowNode) GetFarmingService() *FarmingService {
	return sn.farmingService
}

// GetBlockchain returns the node's blockchain
func (sn *ShadowNode) GetBlockchain() *Blockchain {
	return sn.blockchain
}

// GetMiner returns the node's miner
func (sn *ShadowNode) GetMiner() *Miner {
	return sn.miner
}

// GetConsensus returns the node's consensus engine
func (sn *ShadowNode) GetConsensus() *ConsensusEngine {
	return sn.consensus
}

// defaultShadowConfig returns a basic shadow config
func defaultShadowConfig() *ShadowConfig {
	return &ShadowConfig{
		PlotDirectories:     []string{"./plots"},
		DirectoryServices:   []string{},
		ListenOn:           "0.0.0.0:8888",
		MaxPeers:           50,
		LogLevel:           "info",
		LoggingDirectory:   "./logs",
		ScratchDirectory:   "./scratch",
		BlockchainDirectory: "./blockchain",
		Version:            1,
	}
}

// Node CLI command
var nodeCmd = &cobra.Command{
	Use:   "node",
	Short: "Start the Shadowy blockchain node",
	Long:  "Starts the complete Shadowy blockchain node with all services (mempool, timelord, HTTP API, gRPC)",
	Run: func(cmd *cobra.Command, args []string) {
		// Load configuration
		config := DefaultNodeConfig()
		
		// Override with command line flags
		if httpPort, _ := cmd.Flags().GetInt("http-port"); httpPort != 0 {
			config.HTTPPort = httpPort
		}
		if grpcPort, _ := cmd.Flags().GetInt("grpc-port"); grpcPort != 0 {
			config.GRPCPort = grpcPort
		}
		if enableTimelord, _ := cmd.Flags().GetBool("enable-timelord"); enableTimelord {
			config.EnableTimelord = true
		}
		if disableConsensus, _ := cmd.Flags().GetBool("disable-consensus"); disableConsensus {
			config.EnableConsensus = false
		}
		if consensusPort, _ := cmd.Flags().GetString("consensus-port"); consensusPort != "" {
			config.ConsensusConfig.ListenAddr = "0.0.0.0:" + consensusPort
		}
		if bootstrapPeers, _ := cmd.Flags().GetStringSlice("bootstrap-peers"); len(bootstrapPeers) > 0 {
			config.BootstrapPeers = bootstrapPeers
			config.ConsensusConfig.BootstrapPeers = bootstrapPeers
		}
		
		// Create and start node
		node, err := NewShadowNode(config)
		if err != nil {
			fmt.Printf("Error creating node: %v\n", err)
			os.Exit(1)
		}
		
		if err := node.Start(); err != nil {
			fmt.Printf("Error starting node: %v\n", err)
			os.Exit(1)
		}
		
		// Wait for shutdown
		<-node.ctx.Done()
		
		log.Printf("Node shutdown complete")
	},
}

func init() {
	rootCmd.AddCommand(nodeCmd)
	
	nodeCmd.Flags().Int("http-port", 8080, "HTTP API server port")
	nodeCmd.Flags().Int("grpc-port", 9090, "gRPC server port")
	nodeCmd.Flags().Bool("enable-timelord", false, "Enable timelord VDF service")
	nodeCmd.Flags().Bool("disable-http", false, "Disable HTTP API server")
	nodeCmd.Flags().Bool("disable-grpc", false, "Disable gRPC server")
	nodeCmd.Flags().Bool("disable-consensus", false, "Disable consensus engine")
	nodeCmd.Flags().String("consensus-port", "8888", "Consensus P2P port")
	nodeCmd.Flags().StringSlice("bootstrap-peers", []string{}, "Bootstrap peer addresses (e.g., host:port)")
	nodeCmd.Flags().Bool("farming-only", false, "Run in farming-only mode")
}