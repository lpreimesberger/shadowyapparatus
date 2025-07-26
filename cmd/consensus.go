package cmd

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
	"time"
)

// ConsensusEngine manages peer-to-peer consensus for the Shadowy blockchain
type ConsensusEngine struct {
	// Core components
	blockchain *Blockchain
	mempool    *Mempool
	miner      *Miner
	farming    *FarmingService

	// Network configuration
	nodeID     string
	listenAddr string
	httpPort   int
	peers      map[string]*Peer
	peersMutex sync.RWMutex

	// Tracker integration
	tracker *TrackerClient

	// Consensus state
	bestChain   *ChainState
	chainMutex  sync.RWMutex
	syncStatus  SyncStatus
	statusMutex sync.RWMutex

	// Network services
	listener net.Listener
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup

	// Configuration
	config *ConsensusConfig

	// Event channels
	blockChan chan *Block
	txChan    chan *SignedTransaction
	peerChan  chan *PeerEvent

	// Sync management
	syncMutex               sync.RWMutex
	pendingBlocks           map[uint64]*Block // Buffer for out-of-order blocks
	nextExpectedHeight      uint64            // Next block height we expect to process
	lastMissingBlockRequest time.Time         // Last time we requested missing blocks
}

// ConsensusConfig contains consensus engine configuration
type ConsensusConfig struct {
	NodeID                  string        `json:"node_id"`
	ListenAddr              string        `json:"listen_addr"`
	TrackerURL              string        `json:"tracker_url"`
	MaxPeers                int           `json:"max_peers"`
	SyncTimeout             time.Duration `json:"sync_timeout"`
	HeartbeatInterval       time.Duration `json:"heartbeat_interval"`
	BlockPropagationTimeout time.Duration `json:"block_propagation_timeout"`
}

// DefaultConsensusConfig returns default consensus configuration
func DefaultConsensusConfig() *ConsensusConfig {
	return &ConsensusConfig{
		NodeID:                  generateNodeID(),
		ListenAddr:              "0.0.0.0:8888",
		TrackerURL:              "http://boobies.local:8090", // Default tracker service
		MaxPeers:                50,
		SyncTimeout:             30 * time.Second,
		HeartbeatInterval:       10 * time.Second,
		BlockPropagationTimeout: 5 * time.Second,
	}
}

// Peer represents a connected peer node
type Peer struct {
	ID               string        `json:"id"`
	Address          string        `json:"address"`
	Connection       net.Conn      `json:"-"`
	LastSeen         time.Time     `json:"last_seen"`
	ChainHeight      uint64        `json:"chain_height"`
	ChainHash        string        `json:"chain_hash"`
	Status           string        `json:"status"` // "connecting", "connected", "syncing", "active", "disconnected"
	Version          string        `json:"version"`
	Latency          time.Duration `json:"latency"`
	MessagesSent     int64         `json:"messages_sent"`
	MessagesReceived int64         `json:"messages_received"`
}

// ChainState represents the current state of the blockchain
type ChainState struct {
	Height     uint64    `json:"height"`
	Hash       string    `json:"hash"`
	Timestamp  time.Time `json:"timestamp"`
	TotalWork  uint64    `json:"total_work"`
	Difficulty uint64    `json:"difficulty"`
}

// SyncStatus represents blockchain synchronization status
type SyncStatus struct {
	IsSyncing      bool      `json:"is_syncing"`
	SyncProgress   float64   `json:"sync_progress"`
	CurrentHeight  uint64    `json:"current_height"`
	TargetHeight   uint64    `json:"target_height"`
	SyncPeer       string    `json:"sync_peer"`
	LastSyncTime   time.Time `json:"last_sync_time"`
	BlocksReceived int64     `json:"blocks_received"`
	BlocksApplied  int64     `json:"blocks_applied"`
}

// PeerEvent represents peer-related events
type PeerEvent struct {
	Type      string      `json:"type"` // "connected", "disconnected", "new_block", "new_transaction"
	PeerID    string      `json:"peer_id"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
}

// Message types for peer communication
const (
	MsgTypeHandshake       = "handshake"
	MsgTypeHeartbeat       = "heartbeat"
	MsgTypeBlockRequest    = "block_request"
	MsgTypeBlockResponse   = "block_response"
	MsgTypeNewBlock        = "new_block"
	MsgTypeNewTransaction  = "new_transaction"
	MsgTypeChainRequest    = "chain_request"
	MsgTypeChainResponse   = "chain_response"
	MsgTypeMempoolRequest  = "mempool_request"
	MsgTypeMempoolResponse = "mempool_response"
)

// P2PMessage represents a peer-to-peer message
type P2PMessage struct {
	Type      string      `json:"type"`
	From      string      `json:"from"`
	To        string      `json:"to"`
	Data      interface{} `json:"data"`
	Timestamp time.Time   `json:"timestamp"`
	Signature string      `json:"signature"`
}

// HandshakeData contains initial peer connection data
type HandshakeData struct {
	NodeID      string    `json:"node_id"`
	Version     string    `json:"version"`
	ChainHeight uint64    `json:"chain_height"`
	ChainHash   string    `json:"chain_hash"`
	Timestamp   time.Time `json:"timestamp"`
	ListenAddr  string    `json:"listen_addr"`
}

// NewConsensusEngine creates a new consensus engine
func NewConsensusEngine(config *ConsensusConfig, blockchain *Blockchain, mempool *Mempool, miner *Miner, farming *FarmingService, httpPort int) *ConsensusEngine {
	if config == nil {
		config = DefaultConsensusConfig()
	}

	ctx, cancel := context.WithCancel(context.Background())

	engine := &ConsensusEngine{
		blockchain:    blockchain,
		mempool:       mempool,
		miner:         miner,
		farming:       farming,
		nodeID:        config.NodeID,
		listenAddr:    config.ListenAddr,
		httpPort:      httpPort,
		peers:         make(map[string]*Peer),
		config:        config,
		ctx:           ctx,
		cancel:        cancel,
		blockChan:     make(chan *Block, 100),
		txChan:        make(chan *SignedTransaction, 1000),
		peerChan:      make(chan *PeerEvent, 100),
		pendingBlocks: make(map[uint64]*Block),
	}

	// Initialize tracker client if tracker URL is configured
	if config.TrackerURL != "" {
		// Get mining address for tracker registration
		miningAddr := ""
		if miner != nil {
			miningAddr = miner.minerAddress
		}
		log.Printf("🔗 Initializing tracker client with URL: %s", config.TrackerURL)
		engine.tracker = NewTrackerClient(config.TrackerURL, config.NodeID, miningAddr, "")
	} else {
		log.Printf("⚠️ No tracker URL configured, skipping tracker client initialization")
	}

	// Initialize best chain state
	tip, err := blockchain.GetTip()
	if err == nil {
		engine.bestChain = &ChainState{
			Height:    tip.Header.Height,
			Hash:      tip.Hash(),
			Timestamp: tip.Header.Timestamp,
		}
		engine.nextExpectedHeight = tip.Header.Height + 1
		log.Printf("✅ [CONSENSUS] Initialized from blockchain tip: height=%d, nextExpected=%d", 
			tip.Header.Height, engine.nextExpectedHeight)
	} else {
		log.Printf("⚠️  [CONSENSUS] Failed to get blockchain tip during initialization: %v", err)
		log.Printf("⚠️  [CONSENSUS] nextExpectedHeight will start from 0 - this may cause sync issues")
		
		// Try to get height from blockchain stats as fallback
		stats := blockchain.GetStats()
		if stats.TipHeight > 0 {
			engine.nextExpectedHeight = stats.TipHeight + 1
			log.Printf("✅ [CONSENSUS] Fallback: set nextExpectedHeight=%d from blockchain stats", 
				engine.nextExpectedHeight)
		}
	}

	return engine
}

// Start starts the consensus engine
func (ce *ConsensusEngine) Start() error {
	log.Printf("Starting consensus engine on %s", ce.listenAddr)

	// Start listening for incoming connections
	var err error
	ce.listener, err = net.Listen("tcp", ce.listenAddr)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", ce.listenAddr, err)
	}

	// Start main event loop
	ce.wg.Add(1)
	go ce.mainLoop()

	// Start peer management
	ce.wg.Add(1)
	go ce.peerManager()

	// Start block processor
	ce.wg.Add(1)
	go ce.blockProcessor()

	// Start transaction processor
	ce.wg.Add(1)
	go ce.transactionProcessor()

	// Start network server
	ce.wg.Add(1)
	go ce.networkServer()

	// Register with tracker service if configured
	if ce.tracker != nil {
		go ce.registerWithTracker()

		// Start tracker heartbeat loop
		ce.wg.Add(1)
		go ce.trackerHeartbeatLoop()

		// Start tracker peer discovery
		ce.wg.Add(1)
		go ce.trackerPeerDiscovery()
	}

	log.Printf("Consensus engine started with Node ID: %s", ce.nodeID)
	return nil
}

// Stop stops the consensus engine
func (ce *ConsensusEngine) Stop() error {
	log.Printf("Stopping consensus engine...")

	// Cancel context to signal shutdown
	ce.cancel()

	// Close listener
	if ce.listener != nil {
		ce.listener.Close()
	}

	// Disconnect all peers
	ce.peersMutex.Lock()
	for _, peer := range ce.peers {
		if peer.Connection != nil {
			peer.Connection.Close()
		}
	}
	ce.peersMutex.Unlock()

	// Wait for all goroutines to finish
	ce.wg.Wait()

	log.Printf("Consensus engine stopped")
	return nil
}

// mainLoop runs the main consensus event loop
func (ce *ConsensusEngine) mainLoop() {
	defer ce.wg.Done()

	heartbeatTicker := time.NewTicker(ce.config.HeartbeatInterval)
	defer heartbeatTicker.Stop()

	for {
		select {
		case <-ce.ctx.Done():
			return

		case <-heartbeatTicker.C:
			ce.sendHeartbeats()
			ce.cleanupPeers()

		case event := <-ce.peerChan:
			ce.handlePeerEvent(event)
		}
	}
}

// peerManager manages peer connections and discovery
func (ce *ConsensusEngine) peerManager() {
	defer ce.wg.Done()

	syncTicker := time.NewTicker(10 * time.Second)
	defer syncTicker.Stop()

	for {
		select {
		case <-ce.ctx.Done():
			return

		case <-syncTicker.C:
			ce.performSync()
		}
	}
}

// blockProcessor processes incoming blocks
func (ce *ConsensusEngine) blockProcessor() {
	defer ce.wg.Done()

	for {
		select {
		case <-ce.ctx.Done():
			return

		case block := <-ce.blockChan:
			ce.processIncomingBlock(block)
		}
	}
}

// transactionProcessor processes incoming transactions
func (ce *ConsensusEngine) transactionProcessor() {
	defer ce.wg.Done()

	for {
		select {
		case <-ce.ctx.Done():
			return

		case tx := <-ce.txChan:
			ce.processIncomingTransaction(tx)
		}
	}
}

// networkServer handles incoming network connections
func (ce *ConsensusEngine) networkServer() {
	defer ce.wg.Done()

	for {
		select {
		case <-ce.ctx.Done():
			return

		default:
			conn, err := ce.listener.Accept()
			if err != nil {
				if ce.ctx.Err() != nil {
					return // Context cancelled
				}
				log.Printf("Failed to accept connection: %v", err)
				continue
			}

			// Handle connection in goroutine
			go ce.handleConnection(conn)
		}
	}
}

// ConnectToPeer connects to a peer
func (ce *ConsensusEngine) ConnectToPeer(address string) error {
	ce.peersMutex.RLock()
	peerCount := len(ce.peers)
	ce.peersMutex.RUnlock()

	if peerCount >= ce.config.MaxPeers {
		return fmt.Errorf("maximum peers reached: %d", ce.config.MaxPeers)
	}

	conn, err := net.Dial("tcp", address)
	if err != nil {
		return fmt.Errorf("failed to connect to %s: %w", address, err)
	}

	go ce.handleConnection(conn)
	return nil
}

// BroadcastBlock broadcasts a new block to all connected peers
func (ce *ConsensusEngine) BroadcastBlock(block *Block) {
	message := &P2PMessage{
		Type:      MsgTypeNewBlock,
		From:      ce.nodeID,
		Data:      block,
		Timestamp: time.Now().UTC(),
	}

	ce.broadcastMessage(message)
}

// BroadcastTransaction broadcasts a new transaction to all connected peers
func (ce *ConsensusEngine) BroadcastTransaction(tx *SignedTransaction) {
	message := &P2PMessage{
		Type:      MsgTypeNewTransaction,
		From:      ce.nodeID,
		Data:      tx,
		Timestamp: time.Now().UTC(),
	}

	ce.broadcastMessage(message)
}

// GetPeers returns information about connected peers
func (ce *ConsensusEngine) GetPeers() map[string]*Peer {
	ce.peersMutex.RLock()
	defer ce.peersMutex.RUnlock()

	result := make(map[string]*Peer)
	for id, peer := range ce.peers {
		result[id] = &Peer{
			ID:               peer.ID,
			Address:          peer.Address,
			LastSeen:         peer.LastSeen,
			ChainHeight:      peer.ChainHeight,
			ChainHash:        peer.ChainHash,
			Status:           peer.Status,
			Version:          peer.Version,
			Latency:          peer.Latency,
			MessagesSent:     peer.MessagesSent,
			MessagesReceived: peer.MessagesReceived,
		}
	}

	return result
}

// GetSyncStatus returns current synchronization status
func (ce *ConsensusEngine) GetSyncStatus() SyncStatus {
	ce.statusMutex.RLock()
	defer ce.statusMutex.RUnlock()

	return ce.syncStatus
}

// GetChainState returns current chain state
func (ce *ConsensusEngine) GetChainState() *ChainState {
	ce.chainMutex.RLock()
	defer ce.chainMutex.RUnlock()

	if ce.bestChain == nil {
		return nil
	}

	return &ChainState{
		Height:     ce.bestChain.Height,
		Hash:       ce.bestChain.Hash,
		Timestamp:  ce.bestChain.Timestamp,
		TotalWork:  ce.bestChain.TotalWork,
		Difficulty: ce.bestChain.Difficulty,
	}
}

// Helper functions

// generateNodeID generates a unique node ID
func generateNodeID() string {
	hash := sha256.Sum256([]byte(fmt.Sprintf("%d-%d", time.Now().UnixNano(), time.Now().Unix())))
	return hex.EncodeToString(hash[:])[:16]
}

// handleConnection handles an incoming or outgoing peer connection
func (ce *ConsensusEngine) handleConnection(conn net.Conn) {
	defer conn.Close()

	// Set connection timeout
	conn.SetReadDeadline(time.Now().Add(ce.config.SyncTimeout))

	// Perform handshake
	peer, err := ce.performHandshake(conn)
	if err != nil {
		log.Printf("Handshake failed with %s: %v", conn.RemoteAddr(), err)
		return
	}

	// Add peer to peers map
	ce.peersMutex.Lock()
	ce.peers[peer.ID] = peer
	ce.peersMutex.Unlock()

	log.Printf("Connected to peer %s (%s)", peer.ID, peer.Address)

	// Send peer connected event
	ce.peerChan <- &PeerEvent{
		Type:      "connected",
		PeerID:    peer.ID,
		Data:      peer,
		Timestamp: time.Now().UTC(),
	}

	// Start message handling loop
	ce.handlePeerMessages(peer)
}

// performHandshake performs the initial handshake with a peer
func (ce *ConsensusEngine) performHandshake(conn net.Conn) (*Peer, error) {
	tip, err := ce.blockchain.GetTip()
	if err != nil {
		return nil, fmt.Errorf("failed to get blockchain tip: %w", err)
	}

	handshake := &HandshakeData{
		NodeID:      ce.nodeID,
		Version:     "1.0.0",
		ChainHeight: tip.Header.Height,
		ChainHash:   tip.Hash(),
		Timestamp:   time.Now().UTC(),
		ListenAddr:  ce.listenAddr,
	}

	// Send handshake
	message := &P2PMessage{
		Type:      MsgTypeHandshake,
		From:      ce.nodeID,
		Data:      handshake,
		Timestamp: time.Now().UTC(),
	}

	if err := ce.sendMessage(conn, message); err != nil {
		return nil, fmt.Errorf("failed to send handshake: %w", err)
	}

	// Receive handshake response
	response, err := ce.receiveMessage(conn)
	if err != nil {
		return nil, fmt.Errorf("failed to receive handshake response: %w", err)
	}

	if response.Type != MsgTypeHandshake {
		return nil, fmt.Errorf("expected handshake response, got %s", response.Type)
	}

	peerHandshake, ok := response.Data.(map[string]interface{})
	if !ok {
		return nil, fmt.Errorf("invalid handshake data format")
	}

	peer := &Peer{
		ID:         response.From,
		Address:    conn.RemoteAddr().String(),
		Connection: conn,
		LastSeen:   time.Now().UTC(),
		Status:     "connected",
		Version:    getStringFromMap(peerHandshake, "version"),
	}

	if heightFloat, ok := peerHandshake["chain_height"].(float64); ok {
		peer.ChainHeight = uint64(heightFloat)
	}

	if hash, ok := peerHandshake["chain_hash"].(string); ok {
		peer.ChainHash = hash
	}

	return peer, nil
}

// Helper function to safely get string from map
func getStringFromMap(m map[string]interface{}, key string) string {
	if val, ok := m[key].(string); ok {
		return val
	}
	return ""
}

// sendMessage sends a message to a peer connection
func (ce *ConsensusEngine) sendMessage(conn net.Conn, message *P2PMessage) error {
	data, err := json.Marshal(message)
	if err != nil {
		return fmt.Errorf("failed to marshal message: %w", err)
	}

	// Send message length first
	lengthBytes := make([]byte, 4)
	lengthBytes[0] = byte(len(data) >> 24)
	lengthBytes[1] = byte(len(data) >> 16)
	lengthBytes[2] = byte(len(data) >> 8)
	lengthBytes[3] = byte(len(data))

	if _, err := conn.Write(lengthBytes); err != nil {
		return fmt.Errorf("failed to write message length: %w", err)
	}

	// Send message data
	if _, err := conn.Write(data); err != nil {
		return fmt.Errorf("failed to write message data: %w", err)
	}

	return nil
}

// receiveMessage receives a message from a peer connection
func (ce *ConsensusEngine) receiveMessage(conn net.Conn) (*P2PMessage, error) {
	// Read message length (must read exactly 4 bytes)
	lengthBytes := make([]byte, 4)
	if _, err := io.ReadFull(conn, lengthBytes); err != nil {
		return nil, fmt.Errorf("failed to read message length: %w", err)
	}

	length := int(lengthBytes[0])<<24 | int(lengthBytes[1])<<16 | int(lengthBytes[2])<<8 | int(lengthBytes[3])

	if length <= 0 || length > 1024*1024 { // 1MB limit
		return nil, fmt.Errorf("invalid message length: %d", length)
	}

	// Read message data (must read exactly 'length' bytes to prevent null character issues)
	data := make([]byte, length)
	if _, err := io.ReadFull(conn, data); err != nil {
		return nil, fmt.Errorf("failed to read message data: %w", err)
	}

	// Unmarshal message
	var message P2PMessage
	if err := json.Unmarshal(data, &message); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return &message, nil
}

// registerWithTracker registers this node with the tracker service
func (ce *ConsensusEngine) registerWithTracker() {
	if ce.tracker == nil {
		return
	}

	log.Printf("🔗 Registering with tracker service...")

	// Wait a bit for blockchain to initialize
	time.Sleep(2 * time.Second)

	if err := ce.tracker.RegisterWithTracker(ce, ce.blockchain, ce.farming); err != nil {
		log.Fatalln(" Failed to register with tracker: " + err.Error())
	}
}

// trackerHeartbeatLoop sends periodic heartbeats to tracker
func (ce *ConsensusEngine) trackerHeartbeatLoop() {
	defer ce.wg.Done()

	if ce.tracker == nil {
		return
	}

	ticker := time.NewTicker(30 * time.Second) // Send heartbeat every 30 seconds
	defer ticker.Stop()

	for {
		select {
		case <-ce.ctx.Done():
			return
		case <-ticker.C:
			// Determine current status
			status := "online"
			ce.statusMutex.RLock()
			if ce.syncStatus.IsSyncing {
				status = "syncing"
			}
			ce.statusMutex.RUnlock()

			if err := ce.tracker.SendHeartbeat(ce.blockchain, ce.farming, status); err != nil {
				log.Printf("⚠️ Failed to send heartbeat to tracker: %v", err)
			}
		}
	}
}

// trackerPeerDiscovery discovers peers from tracker service
func (ce *ConsensusEngine) trackerPeerDiscovery() {
	defer ce.wg.Done()

	if ce.tracker == nil {
		return
	}

	ticker := time.NewTicker(60 * time.Second) // Discover peers every minute
	defer ticker.Stop()

	// Initial discovery
	ce.discoverPeersFromTracker()

	for {
		select {
		case <-ce.ctx.Done():
			return
		case <-ticker.C:
			ce.discoverPeersFromTracker()
		}
	}
}

// discoverPeersFromTracker gets peers from tracker and connects to them
func (ce *ConsensusEngine) discoverPeersFromTracker() {
	// Get our chain ID from genesis block
	stats := ce.blockchain.GetStats()
	chainID := stats.GenesisHash
	if chainID == "" {
		if genesisBlock, err := ce.blockchain.GetBlockByHeight(0); err == nil {
			chainID = genesisBlock.Hash()
		} else {
			log.Printf("⚠️ Could not determine chain ID for peer discovery")
			chainID = "unknown"
		}
	}

	// Transform chainID to tracker format
	peers, err := ce.tracker.DiscoverPeers(chainID)
	if err != nil {
		log.Printf("⚠️ Failed to discover peers from tracker: %v", err)
		return
	}

	log.Printf("📡 Discovered %d peers from tracker", len(peers))

	for _, trackerPeer := range peers {
		// Skip ourselves
		if trackerPeer.NodeID == ce.nodeID {
			continue
		}

		// Check if we're already connected to this peer
		ce.peersMutex.RLock()
		_, exists := ce.peers[trackerPeer.NodeID]
		ce.peersMutex.RUnlock()

		if !exists {
			// Try to connect to this peer
			go ce.ConnectToPeer(trackerPeer.Address)
		}
	}
}
