package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/gorilla/mux"
)

// initializeHTTPServer sets up the HTTP API server
func (sn *ShadowNode) initializeHTTPServer() error {
	router := mux.NewRouter()
	
	// API versioning
	v1 := router.PathPrefix("/api/v1").Subrouter()
	
	// Health and status endpoints
	v1.HandleFunc("/health", sn.handleHealth).Methods("GET")
	v1.HandleFunc("/status", sn.handleStatus).Methods("GET")
	
	// Mempool endpoints
	mempool := v1.PathPrefix("/mempool").Subrouter()
	mempool.HandleFunc("", sn.handleMempoolStats).Methods("GET")
	mempool.HandleFunc("/transactions", sn.handleSubmitTransaction).Methods("POST")
	mempool.HandleFunc("/transactions/{hash}", sn.handleGetTransaction).Methods("GET")
	mempool.HandleFunc("/transactions", sn.handleListTransactions).Methods("GET")
	
	// Timelord endpoints (if enabled)
	if sn.config.EnableTimelord {
		timelord := v1.PathPrefix("/timelord").Subrouter()
		timelord.HandleFunc("", sn.handleTimelordStats).Methods("GET")
		timelord.HandleFunc("/jobs", sn.handleSubmitVDFJob).Methods("POST")
		timelord.HandleFunc("/jobs/{id}", sn.handleGetVDFJob).Methods("GET")
	}
	
	// Farming endpoints (if enabled)
	if sn.config.EnableFarming {
		farming := v1.PathPrefix("/farming").Subrouter()
		farming.HandleFunc("", sn.handleFarmingStats).Methods("GET")
		farming.HandleFunc("/status", sn.handleFarmingStatus).Methods("GET")
		farming.HandleFunc("/plots", sn.handleListPlots).Methods("GET")
		farming.HandleFunc("/challenge", sn.handleSubmitChallenge).Methods("POST")
	}
	
	// Blockchain endpoints
	blockchain := v1.PathPrefix("/blockchain").Subrouter()
	blockchain.HandleFunc("", sn.handleBlockchainStats).Methods("GET")
	blockchain.HandleFunc("/tip", sn.handleGetTip).Methods("GET")
	blockchain.HandleFunc("/block/{hash}", sn.handleGetBlock).Methods("GET")
	blockchain.HandleFunc("/block/height/{height}", sn.handleGetBlockByHeight).Methods("GET")
	blockchain.HandleFunc("/recent", sn.handleGetRecentBlocks).Methods("GET")
	
	// Tokenomics endpoints
	tokenomics := v1.PathPrefix("/tokenomics").Subrouter()
	tokenomics.HandleFunc("", sn.handleNetworkStats).Methods("GET")
	tokenomics.HandleFunc("/reward/{height}", sn.handleBlockReward).Methods("GET")
	tokenomics.HandleFunc("/schedule", sn.handleRewardSchedule).Methods("GET")
	tokenomics.HandleFunc("/supply/{height}", sn.handleSupplyAtHeight).Methods("GET")
	tokenomics.HandleFunc("/halvings", sn.handleHalvingHistory).Methods("GET")
	
	// Mining endpoints (if enabled)
	if sn.config.EnableMining {
		mining := v1.PathPrefix("/mining").Subrouter()
		mining.HandleFunc("", sn.handleMiningStats).Methods("GET")
		mining.HandleFunc("/status", sn.handleMiningStatus).Methods("GET")
		mining.HandleFunc("/force", sn.handleForceBlock).Methods("POST")
		mining.HandleFunc("/address", sn.handleGetMiningAddress).Methods("GET")
		mining.HandleFunc("/address", sn.handleSetMiningAddress).Methods("POST")
	}
	
	// Consensus endpoints (if enabled)
	if sn.config.EnableConsensus {
		consensus := v1.PathPrefix("/consensus").Subrouter()
		consensus.HandleFunc("", sn.handleConsensusStatus).Methods("GET")
		consensus.HandleFunc("/peers", sn.handleGetPeers).Methods("GET")
		consensus.HandleFunc("/peers/connect", sn.handleConnectPeer).Methods("POST")
		consensus.HandleFunc("/sync", sn.handleGetSyncStatus).Methods("GET")
		consensus.HandleFunc("/sync/force", sn.handleForceSync).Methods("POST")
		consensus.HandleFunc("/chain", sn.handleGetChainState).Methods("GET")
	}
	
	// Wallet endpoints
	wallet := v1.PathPrefix("/wallet").Subrouter()
	wallet.HandleFunc("", sn.handleListWallets).Methods("GET")
	wallet.HandleFunc("/{name}", sn.handleGetWallet).Methods("GET")
	wallet.HandleFunc("/{name}/balance", sn.handleGetBalance).Methods("GET")
	
	// Transaction utilities
	utils := v1.PathPrefix("/utils").Subrouter()
	utils.HandleFunc("/validate-address", sn.handleValidateAddress).Methods("POST")
	utils.HandleFunc("/transaction/create", sn.handleCreateTransaction).Methods("POST")
	utils.HandleFunc("/transaction/sign", sn.handleSignTransaction).Methods("POST")
	
	// Add CORS middleware
	router.Use(corsMiddleware)
	
	// Add logging middleware
	router.Use(loggingMiddleware)
	
	sn.httpServer = &http.Server{
		Addr:         fmt.Sprintf(":%d", sn.config.HTTPPort),
		Handler:      router,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
		IdleTimeout:  60 * time.Second,
	}
	
	sn.updateHealthStatus("http", "healthy", nil, map[string]interface{}{
		"port": sn.config.HTTPPort,
	})
	
	return nil
}

// Health endpoint
func (sn *ShadowNode) handleHealth(w http.ResponseWriter, r *http.Request) {
	health := sn.GetHealthStatus()
	
	// Determine overall health
	overallHealthy := true
	for _, service := range health {
		if service.Status != "healthy" {
			overallHealthy = false
			break
		}
	}
	
	response := map[string]interface{}{
		"status":   "ok",
		"healthy":  overallHealthy,
		"services": health,
		"timestamp": time.Now().UTC(),
	}
	
	if !overallHealthy {
		w.WriteHeader(http.StatusServiceUnavailable)
	}
	
	json.NewEncoder(w).Encode(response)
}

// Status endpoint
func (sn *ShadowNode) handleStatus(w http.ResponseWriter, r *http.Request) {
	status := map[string]interface{}{
		"node_id":     "shadowy-node-1", // TODO: Generate unique node ID
		"version":     "0.1.0",
		"uptime":      time.Since(time.Now().Add(-time.Hour)), // TODO: Track actual uptime
		"services": map[string]bool{
			"blockchain": sn.blockchain != nil,
			"mempool":    sn.mempool != nil,
			"timelord":   sn.timelord != nil,
			"farming":    sn.farmingService != nil,
			"miner":      sn.miner != nil,
			"http":       sn.httpServer != nil,
			"grpc":       sn.grpcServer != nil,
		},
		"config": map[string]interface{}{
			"http_port":       sn.config.HTTPPort,
			"grpc_port":       sn.config.GRPCPort,
			"enable_timelord": sn.config.EnableTimelord,
		},
	}
	
	json.NewEncoder(w).Encode(status)
}

// Mempool stats endpoint
func (sn *ShadowNode) handleMempoolStats(w http.ResponseWriter, r *http.Request) {
	if sn.mempool == nil {
		http.Error(w, "Mempool not available", http.StatusServiceUnavailable)
		return
	}
	
	stats := sn.mempool.GetStats()
	json.NewEncoder(w).Encode(stats)
}

// Submit transaction endpoint
func (sn *ShadowNode) handleSubmitTransaction(w http.ResponseWriter, r *http.Request) {
	if sn.mempool == nil {
		http.Error(w, "Mempool not available", http.StatusServiceUnavailable)
		return
	}
	
	var signedTx SignedTransaction
	if err := json.NewDecoder(r.Body).Decode(&signedTx); err != nil {
		http.Error(w, "Invalid transaction format", http.StatusBadRequest)
		return
	}
	
	// Add transaction to mempool
	err := sn.mempool.AddTransaction(&signedTx, SourceAPI)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add transaction: %v", err), http.StatusBadRequest)
		return
	}
	
	response := map[string]interface{}{
		"status": "accepted",
		"tx_hash": signedTx.TxHash,
		"message": "Transaction added to mempool",
	}
	
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

// Get transaction endpoint
func (sn *ShadowNode) handleGetTransaction(w http.ResponseWriter, r *http.Request) {
	if sn.mempool == nil {
		http.Error(w, "Mempool not available", http.StatusServiceUnavailable)
		return
	}
	
	vars := mux.Vars(r)
	txHash := vars["hash"]
	
	tx, err := sn.mempool.GetTransaction(txHash)
	if err != nil {
		http.Error(w, "Transaction not found", http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(tx)
}

// List transactions endpoint
func (sn *ShadowNode) handleListTransactions(w http.ResponseWriter, r *http.Request) {
	if sn.mempool == nil {
		http.Error(w, "Mempool not available", http.StatusServiceUnavailable)
		return
	}
	
	// Parse query parameters
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	
	// Get highest priority transactions
	transactions := sn.mempool.GetHighestPriorityTransactions(limit)
	
	response := map[string]interface{}{
		"transactions": transactions,
		"count":        len(transactions),
	}
	
	json.NewEncoder(w).Encode(response)
}

// Timelord stats endpoint
func (sn *ShadowNode) handleTimelordStats(w http.ResponseWriter, r *http.Request) {
	if sn.timelord == nil {
		http.Error(w, "Timelord not available", http.StatusServiceUnavailable)
		return
	}
	
	stats := sn.timelord.GetStats()
	json.NewEncoder(w).Encode(stats)
}

// Submit VDF job endpoint
func (sn *ShadowNode) handleSubmitVDFJob(w http.ResponseWriter, r *http.Request) {
	if sn.timelord == nil {
		http.Error(w, "Timelord not available", http.StatusServiceUnavailable)
		return
	}
	
	var request struct {
		Data     []byte `json:"data"`
		Priority int    `json:"priority"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	job, err := sn.timelord.SubmitChallenge(request.Data, request.Priority)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to submit job: %v", err), http.StatusBadRequest)
		return
	}
	
	response := map[string]interface{}{
		"status": "accepted",
		"job_id": job.ID,
		"message": "VDF job submitted",
	}
	
	w.WriteHeader(http.StatusAccepted)
	json.NewEncoder(w).Encode(response)
}

// Get VDF job endpoint
func (sn *ShadowNode) handleGetVDFJob(w http.ResponseWriter, r *http.Request) {
	if sn.timelord == nil {
		http.Error(w, "Timelord not available", http.StatusServiceUnavailable)
		return
	}
	
	vars := mux.Vars(r)
	jobID := vars["id"]
	
	job, err := sn.timelord.GetJob(jobID)
	if err != nil {
		http.Error(w, "Job not found", http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(job)
}

// List wallets endpoint
func (sn *ShadowNode) handleListWallets(w http.ResponseWriter, r *http.Request) {
	wallets, err := listWallets()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list wallets: %v", err), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"wallets": wallets,
		"count":   len(wallets),
	}
	
	json.NewEncoder(w).Encode(response)
}

// Get wallet endpoint
func (sn *ShadowNode) handleGetWallet(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	walletName := vars["name"]
	
	wallet, err := loadWallet(walletName)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}
	
	// Return public information only
	response := map[string]interface{}{
		"name":       wallet.Name,
		"address":    wallet.Address,
		"created_at": wallet.CreatedAt,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Get balance endpoint
func (sn *ShadowNode) handleGetBalance(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	walletName := vars["name"]
	
	wallet, err := loadWallet(walletName)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}
	
	// TODO: Implement actual balance calculation from blockchain/UTXO set
	response := map[string]interface{}{
		"address": wallet.Address,
		"balance": 0, // Placeholder
		"confirmed": 0,
		"unconfirmed": 0,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Validate address endpoint
func (sn *ShadowNode) handleValidateAddress(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Address string `json:"address"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	isValid := IsValidAddress(request.Address)
	
	response := map[string]interface{}{
		"address": request.Address,
		"valid":   isValid,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Create transaction endpoint
func (sn *ShadowNode) handleCreateTransaction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Inputs  []TransactionInput  `json:"inputs"`
		Outputs []TransactionOutput `json:"outputs"`
		NotUntil *time.Time        `json:"not_until,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	// Create transaction
	tx := &Transaction{
		Version:   1,
		Inputs:    request.Inputs,
		Outputs:   request.Outputs,
		Timestamp: time.Now().UTC(),
		Nonce:     uint64(time.Now().UnixNano()),
	}
	
	if request.NotUntil != nil {
		tx.NotUntil = *request.NotUntil
	} else {
		tx.NotUntil = time.Now().UTC()
	}
	
	// Validate transaction
	if err := tx.IsValid(); err != nil {
		http.Error(w, fmt.Sprintf("Invalid transaction: %v", err), http.StatusBadRequest)
		return
	}
	
	hash, err := tx.Hash()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to generate hash: %v", err), http.StatusInternalServerError)
		return
	}

	response := map[string]interface{}{
		"transaction": tx,
		"hash":        hash,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Sign transaction endpoint
func (sn *ShadowNode) handleSignTransaction(w http.ResponseWriter, r *http.Request) {
	var request struct {
		Transaction json.RawMessage `json:"transaction"`
		WalletName  string          `json:"wallet_name"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	// Parse transaction
	var tx Transaction
	if err := json.Unmarshal(request.Transaction, &tx); err != nil {
		http.Error(w, "Invalid transaction format", http.StatusBadRequest)
		return
	}
	
	// Load wallet
	wallet, err := loadWallet(request.WalletName)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}
	
	// Sign transaction
	signedTx, err := SignTransactionWithWallet(&tx, wallet)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
		return
	}
	
	json.NewEncoder(w).Encode(signedTx)
}

// CORS middleware
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}

// Logging middleware
func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		duration := time.Since(start)
		
		// Simple access log
		fmt.Printf("[HTTP] %s %s %v\n", r.Method, r.URL.Path, duration)
	})
}

// Farming stats endpoint
func (sn *ShadowNode) handleFarmingStats(w http.ResponseWriter, r *http.Request) {
	if sn.farmingService == nil {
		http.Error(w, "Farming service not available", http.StatusServiceUnavailable)
		return
	}
	
	stats := sn.farmingService.GetStats()
	json.NewEncoder(w).Encode(stats)
}

// Farming status endpoint
func (sn *ShadowNode) handleFarmingStatus(w http.ResponseWriter, r *http.Request) {
	if sn.farmingService == nil {
		http.Error(w, "Farming service not available", http.StatusServiceUnavailable)
		return
	}
	
	response := map[string]interface{}{
		"running": sn.farmingService.IsRunning(),
		"stats":   sn.farmingService.GetStats(),
	}
	
	json.NewEncoder(w).Encode(response)
}

// List plots endpoint
func (sn *ShadowNode) handleListPlots(w http.ResponseWriter, r *http.Request) {
	if sn.farmingService == nil {
		http.Error(w, "Farming service not available", http.StatusServiceUnavailable)
		return
	}
	
	plots, err := sn.farmingService.ListPlotFiles()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to list plots: %v", err), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"plots": plots,
		"count": len(plots),
	}
	
	json.NewEncoder(w).Encode(response)
}

// Submit challenge endpoint
func (sn *ShadowNode) handleSubmitChallenge(w http.ResponseWriter, r *http.Request) {
	if sn.farmingService == nil {
		http.Error(w, "Farming service not available", http.StatusServiceUnavailable)
		return
	}
	
	var request struct {
		Challenge  []byte `json:"challenge"`
		Difficulty uint32 `json:"difficulty,omitempty"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	// Create storage challenge
	challenge := &StorageChallenge{
		ID:         fmt.Sprintf("api_%d", time.Now().UnixNano()),
		Challenge:  request.Challenge,
		Timestamp:  time.Now().UTC(),
		Difficulty: request.Difficulty,
	}
	
	if challenge.Difficulty == 0 {
		challenge.Difficulty = 1 // Default difficulty
	}
	
	// Submit challenge and get response
	proof := sn.farmingService.SubmitChallenge(challenge)
	
	if proof.Error != "" {
		w.WriteHeader(http.StatusInternalServerError)
	} else {
		w.WriteHeader(http.StatusOK)
	}
	
	json.NewEncoder(w).Encode(proof)
}

// Blockchain stats endpoint
func (sn *ShadowNode) handleBlockchainStats(w http.ResponseWriter, r *http.Request) {
	if sn.blockchain == nil {
		http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
		return
	}
	
	stats := sn.blockchain.GetStats()
	json.NewEncoder(w).Encode(stats)
}

// Get tip block endpoint
func (sn *ShadowNode) handleGetTip(w http.ResponseWriter, r *http.Request) {
	if sn.blockchain == nil {
		http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
		return
	}
	
	tip, err := sn.blockchain.GetTip()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get tip: %v", err), http.StatusInternalServerError)
		return
	}
	
	json.NewEncoder(w).Encode(tip)
}

// Get block by hash endpoint
func (sn *ShadowNode) handleGetBlock(w http.ResponseWriter, r *http.Request) {
	if sn.blockchain == nil {
		http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
		return
	}
	
	vars := mux.Vars(r)
	hash := vars["hash"]
	
	if hash == "" {
		http.Error(w, "Block hash required", http.StatusBadRequest)
		return
	}
	
	block, err := sn.blockchain.GetBlock(hash)
	if err != nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(block)
}

// Get block by height endpoint
func (sn *ShadowNode) handleGetBlockByHeight(w http.ResponseWriter, r *http.Request) {
	if sn.blockchain == nil {
		http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
		return
	}
	
	vars := mux.Vars(r)
	heightStr := vars["height"]
	
	if heightStr == "" {
		http.Error(w, "Block height required", http.StatusBadRequest)
		return
	}
	
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid height format", http.StatusBadRequest)
		return
	}
	
	block, err := sn.blockchain.GetBlockByHeight(height)
	if err != nil {
		http.Error(w, "Block not found", http.StatusNotFound)
		return
	}
	
	json.NewEncoder(w).Encode(block)
}

// Get recent blocks endpoint
func (sn *ShadowNode) handleGetRecentBlocks(w http.ResponseWriter, r *http.Request) {
	if sn.blockchain == nil {
		http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
		return
	}
	
	// Parse limit parameter
	limitStr := r.URL.Query().Get("limit")
	limit := 10 // default
	if limitStr != "" {
		if l, err := strconv.Atoi(limitStr); err == nil && l > 0 && l <= 100 {
			limit = l
		}
	}
	
	blocks, err := sn.blockchain.GetRecentBlocks(limit)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to get recent blocks: %v", err), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"blocks": blocks,
		"count":  len(blocks),
		"limit":  limit,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Network stats endpoint (tokenomics overview)
func (sn *ShadowNode) handleNetworkStats(w http.ResponseWriter, r *http.Request) {
	if sn.blockchain == nil {
		http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
		return
	}
	
	currentHeight := sn.blockchain.tipHeight
	stats := GetNetworkStats(currentHeight)
	
	json.NewEncoder(w).Encode(stats)
}

// Block reward endpoint
func (sn *ShadowNode) handleBlockReward(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	heightStr := vars["height"]
	
	if heightStr == "" {
		http.Error(w, "Block height required", http.StatusBadRequest)
		return
	}
	
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid height format", http.StatusBadRequest)
		return
	}
	
	reward := CalculateBlockReward(height)
	
	response := map[string]interface{}{
		"height":         height,
		"reward_satoshi": reward,
		"reward_shadow":  float64(reward) / float64(SatoshisPerShadow),
		"halving_era":    height / HalvingInterval + 1,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Reward schedule endpoint
func (sn *ShadowNode) handleRewardSchedule(w http.ResponseWriter, r *http.Request) {
	schedule := GetRewardSchedule()
	
	response := map[string]interface{}{
		"schedule":           schedule,
		"halving_interval":   HalvingInterval,
		"initial_reward":     float64(InitialBlockReward) / float64(SatoshisPerShadow),
		"max_supply":         float64(MaxSatoshis) / float64(SatoshisPerShadow),
		"target_block_time":  TargetBlockTime,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Supply at height endpoint
func (sn *ShadowNode) handleSupplyAtHeight(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	heightStr := vars["height"]
	
	if heightStr == "" {
		http.Error(w, "Block height required", http.StatusBadRequest)
		return
	}
	
	height, err := strconv.ParseUint(heightStr, 10, 64)
	if err != nil {
		http.Error(w, "Invalid height format", http.StatusBadRequest)
		return
	}
	
	supply := GetTotalSupplyAtHeight(height)
	inflationRate := GetInflationRate(height)
	
	response := map[string]interface{}{
		"height":            height,
		"supply_satoshi":    supply,
		"supply_shadow":     float64(supply) / float64(SatoshisPerShadow),
		"inflation_rate":    inflationRate,
		"percent_mined":     float64(supply) / float64(MaxSatoshis) * 100.0,
		"remaining_supply":  float64(MaxSatoshis - supply) / float64(SatoshisPerShadow),
	}
	
	json.NewEncoder(w).Encode(response)
}

// Halving history endpoint
func (sn *ShadowNode) handleHalvingHistory(w http.ResponseWriter, r *http.Request) {
	history := GetHalvingHistory()
	
	// Add current network information
	var currentHalving int
	if sn.blockchain != nil {
		currentHalving = int(sn.blockchain.tipHeight / HalvingInterval) + 1
	}
	
	response := map[string]interface{}{
		"halving_history":    history,
		"current_halving":    currentHalving,
		"halving_interval":   HalvingInterval,
		"blocks_per_year":    365 * 24 * 6, // Assuming 10-minute blocks
	}
	
	json.NewEncoder(w).Encode(response)
}

// Mining stats endpoint
func (sn *ShadowNode) handleMiningStats(w http.ResponseWriter, r *http.Request) {
	if sn.miner == nil {
		http.Error(w, "Mining service not available", http.StatusServiceUnavailable)
		return
	}
	
	stats := sn.miner.GetStats()
	json.NewEncoder(w).Encode(stats)
}

// Mining status endpoint
func (sn *ShadowNode) handleMiningStatus(w http.ResponseWriter, r *http.Request) {
	if sn.miner == nil {
		http.Error(w, "Mining service not available", http.StatusServiceUnavailable)
		return
	}
	
	stats := sn.miner.GetStats()
	nextBlock := sn.miner.GetEstimatedNextBlock()
	
	response := map[string]interface{}{
		"running":              sn.miner.IsRunning(),
		"mining_address":       sn.miner.GetMiningAddress(),
		"blocks_mined":         stats.BlocksMined,
		"total_rewards":        stats.TotalRewards,
		"total_rewards_shadow": float64(stats.TotalRewards) / float64(SatoshisPerShadow),
		"avg_block_time":       stats.AverageBlockTime.String(),
		"last_block_time":      stats.LastBlockTime,
		"estimated_next_block": nextBlock,
		"proof_success_rate":   stats.ProofSuccessRate,
		"fees_collected":       stats.FeesCollected,
	}
	
	json.NewEncoder(w).Encode(response)
}

// Force block generation endpoint
func (sn *ShadowNode) handleForceBlock(w http.ResponseWriter, r *http.Request) {
	if sn.miner == nil {
		http.Error(w, "Mining service not available", http.StatusServiceUnavailable)
		return
	}
	
	if !sn.miner.IsRunning() {
		http.Error(w, "Miner is not running", http.StatusBadRequest)
		return
	}
	
	if err := sn.miner.ForceBlockGeneration(); err != nil {
		http.Error(w, fmt.Sprintf("Failed to force block generation: %v", err), http.StatusInternalServerError)
		return
	}
	
	response := map[string]interface{}{
		"status":  "success",
		"message": "Block generation forced",
	}
	
	json.NewEncoder(w).Encode(response)
}

// Get mining address endpoint
func (sn *ShadowNode) handleGetMiningAddress(w http.ResponseWriter, r *http.Request) {
	if sn.miner == nil {
		http.Error(w, "Mining service not available", http.StatusServiceUnavailable)
		return
	}
	
	response := map[string]interface{}{
		"mining_address": sn.miner.GetMiningAddress(),
	}
	
	json.NewEncoder(w).Encode(response)
}

// Set mining address endpoint
func (sn *ShadowNode) handleSetMiningAddress(w http.ResponseWriter, r *http.Request) {
	if sn.miner == nil {
		http.Error(w, "Mining service not available", http.StatusServiceUnavailable)
		return
	}
	
	var request struct {
		Address string `json:"address"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request format", http.StatusBadRequest)
		return
	}
	
	if err := sn.miner.SetMiningAddress(request.Address); err != nil {
		http.Error(w, fmt.Sprintf("Failed to set mining address: %v", err), http.StatusBadRequest)
		return
	}
	
	response := map[string]interface{}{
		"status":         "success",
		"mining_address": request.Address,
		"message":        "Mining address updated",
	}
	
	json.NewEncoder(w).Encode(response)
}