package cmd

import (
	"encoding/json"
	"fmt"
	"log"
	"time"
)

// handlePeerMessages handles incoming messages from a peer
func (ce *ConsensusEngine) handlePeerMessages(peer *Peer) {
	defer func() {
		// Clean up peer on disconnect
		ce.peersMutex.Lock()
		if existingPeer, exists := ce.peers[peer.ID]; exists {
			existingPeer.Status = "disconnected"
			delete(ce.peers, peer.ID)
		}
		ce.peersMutex.Unlock()
		
		// Send peer disconnected event
		ce.peerChan <- &PeerEvent{
			Type:      "disconnected",
			PeerID:    peer.ID,
			Timestamp: time.Now().UTC(),
		}
		
		log.Printf("Peer %s disconnected", peer.ID)
	}()
	
	for {
		select {
		case <-ce.ctx.Done():
			return
			
		default:
			// Set read deadline
			peer.Connection.SetReadDeadline(time.Now().Add(ce.config.SyncTimeout))
			
			message, err := ce.receiveMessage(peer.Connection)
			if err != nil {
				log.Printf("Failed to receive message from peer %s: %v", peer.ID, err)
				return
			}
			
			// Update peer stats
			ce.peersMutex.Lock()
			if existingPeer, exists := ce.peers[peer.ID]; exists {
				existingPeer.LastSeen = time.Now().UTC()
				existingPeer.MessagesReceived++
			}
			ce.peersMutex.Unlock()
			
			// Handle message
			if err := ce.handleMessage(peer, message); err != nil {
				log.Printf("Failed to handle message from peer %s: %v", peer.ID, err)
				continue
			}
		}
	}
}

// handleMessage handles a specific message from a peer
func (ce *ConsensusEngine) handleMessage(peer *Peer, message *P2PMessage) error {
	switch message.Type {
	case MsgTypeHeartbeat:
		return ce.handleHeartbeat(peer, message)
		
	case MsgTypeNewBlock:
		return ce.handleNewBlock(peer, message)
		
	case MsgTypeNewTransaction:
		return ce.handleNewTransaction(peer, message)
		
	case MsgTypeBlockRequest:
		return ce.handleBlockRequest(peer, message)
		
	case MsgTypeBlockResponse:
		return ce.handleBlockResponse(peer, message)
		
	case MsgTypeChainRequest:
		return ce.handleChainRequest(peer, message)
		
	case MsgTypeChainResponse:
		return ce.handleChainResponse(peer, message)
		
	case MsgTypeMempoolRequest:
		return ce.handleMempoolRequest(peer, message)
		
	case MsgTypeMempoolResponse:
		return ce.handleMempoolResponse(peer, message)
		
	default:
		return fmt.Errorf("unknown message type: %s", message.Type)
	}
}

// handleHeartbeat handles heartbeat messages
func (ce *ConsensusEngine) handleHeartbeat(peer *Peer, message *P2PMessage) error {
	// Update peer chain info if provided
	if data, ok := message.Data.(map[string]interface{}); ok {
		ce.peersMutex.Lock()
		if existingPeer, exists := ce.peers[peer.ID]; exists {
			if height, ok := data["chain_height"].(float64); ok {
				existingPeer.ChainHeight = uint64(height)
			}
			if hash, ok := data["chain_hash"].(string); ok {
				existingPeer.ChainHash = hash
			}
		}
		ce.peersMutex.Unlock()
	}
	
	return nil
}

// handleNewBlock handles new block announcements
func (ce *ConsensusEngine) handleNewBlock(peer *Peer, message *P2PMessage) error {
	// Parse block data
	blockData, err := json.Marshal(message.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal block data: %w", err)
	}
	
	var block Block
	if err := json.Unmarshal(blockData, &block); err != nil {
		return fmt.Errorf("failed to unmarshal block: %w", err)
	}
	
	// Queue block for processing
	select {
	case ce.blockChan <- &block:
		log.Printf("Received new block %d from peer %s", block.Header.Height, peer.ID)
	default:
		log.Printf("Block channel full, dropping block from peer %s", peer.ID)
	}
	
	return nil
}

// handleNewTransaction handles new transaction announcements
func (ce *ConsensusEngine) handleNewTransaction(peer *Peer, message *P2PMessage) error {
	// Parse transaction data
	txData, err := json.Marshal(message.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal transaction data: %w", err)
	}
	
	var tx SignedTransaction
	if err := json.Unmarshal(txData, &tx); err != nil {
		return fmt.Errorf("failed to unmarshal transaction: %w", err)
	}
	
	// Queue transaction for processing
	select {
	case ce.txChan <- &tx:
		log.Printf("Received new transaction %s from peer %s", tx.TxHash, peer.ID)
	default:
		log.Printf("Transaction channel full, dropping transaction from peer %s", peer.ID)
	}
	
	return nil
}

// handleBlockRequest handles block requests
func (ce *ConsensusEngine) handleBlockRequest(peer *Peer, message *P2PMessage) error {
	requestData, ok := message.Data.(map[string]interface{})
	if !ok {
		return fmt.Errorf("invalid block request data")
	}
	
	var block *Block
	var err error
	
	if height, ok := requestData["height"].(float64); ok {
		block, err = ce.blockchain.GetBlockByHeight(uint64(height))
	} else if hash, ok := requestData["hash"].(string); ok {
		block, err = ce.blockchain.GetBlock(hash)
	} else {
		return fmt.Errorf("block request must specify height or hash")
	}
	
	if err != nil {
		return fmt.Errorf("failed to get requested block: %w", err)
	}
	
	// Send block response
	response := &P2PMessage{
		Type:      MsgTypeBlockResponse,
		From:      ce.nodeID,
		To:        peer.ID,
		Data:      block,
		Timestamp: time.Now().UTC(),
	}
	
	return ce.sendMessage(peer.Connection, response)
}

// handleBlockResponse handles block responses
func (ce *ConsensusEngine) handleBlockResponse(peer *Peer, message *P2PMessage) error {
	// Parse block data
	blockData, err := json.Marshal(message.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal block data: %w", err)
	}
	
	var block Block
	if err := json.Unmarshal(blockData, &block); err != nil {
		return fmt.Errorf("failed to unmarshal block: %w", err)
	}
	
	// Queue block for processing
	select {
	case ce.blockChan <- &block:
		log.Printf("Received block response %d from peer %s", block.Header.Height, peer.ID)
	default:
		log.Printf("Block channel full, dropping block response from peer %s", peer.ID)
	}
	
	return nil
}

// handleChainRequest handles chain state requests
func (ce *ConsensusEngine) handleChainRequest(peer *Peer, message *P2PMessage) error {
	tip, err := ce.blockchain.GetTip()
	if err != nil {
		return fmt.Errorf("failed to get blockchain tip: %w", err)
	}
	
	stats := ce.blockchain.GetStats()
	
	chainState := &ChainState{
		Height:    tip.Header.Height,
		Hash:      tip.Hash(),
		Timestamp: tip.Header.Timestamp,
		TotalWork: uint64(stats.TotalBlocks), // Simplified work calculation
	}
	
	response := &P2PMessage{
		Type:      MsgTypeChainResponse,
		From:      ce.nodeID,
		To:        peer.ID,
		Data:      chainState,
		Timestamp: time.Now().UTC(),
	}
	
	return ce.sendMessage(peer.Connection, response)
}

// handleChainResponse handles chain state responses
func (ce *ConsensusEngine) handleChainResponse(peer *Peer, message *P2PMessage) error {
	chainData, err := json.Marshal(message.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal chain data: %w", err)
	}
	
	var chainState ChainState
	if err := json.Unmarshal(chainData, &chainState); err != nil {
		return fmt.Errorf("failed to unmarshal chain state: %w", err)
	}
	
	// Update peer chain info
	ce.peersMutex.Lock()
	if existingPeer, exists := ce.peers[peer.ID]; exists {
		existingPeer.ChainHeight = chainState.Height
		existingPeer.ChainHash = chainState.Hash
	}
	ce.peersMutex.Unlock()
	
	// Check if we need to sync
	currentTip, err := ce.blockchain.GetTip()
	if err != nil {
		return fmt.Errorf("failed to get current tip: %w", err)
	}
	
	if chainState.Height > currentTip.Header.Height {
		log.Printf("Peer %s has higher chain height (%d vs %d), considering sync", 
			peer.ID, chainState.Height, currentTip.Header.Height)
		ce.considerSync(peer, &chainState)
	}
	
	return nil
}

// handleMempoolRequest handles mempool requests
func (ce *ConsensusEngine) handleMempoolRequest(peer *Peer, message *P2PMessage) error {
	// Get top 100 highest priority transactions
	mempoolTxs := ce.mempool.GetHighestPriorityTransactions(100)
	
	response := &P2PMessage{
		Type:      MsgTypeMempoolResponse,
		From:      ce.nodeID,
		To:        peer.ID,
		Data:      mempoolTxs,
		Timestamp: time.Now().UTC(),
	}
	
	return ce.sendMessage(peer.Connection, response)
}

// handleMempoolResponse handles mempool responses
func (ce *ConsensusEngine) handleMempoolResponse(peer *Peer, message *P2PMessage) error {
	txData, err := json.Marshal(message.Data)
	if err != nil {
		return fmt.Errorf("failed to marshal mempool data: %w", err)
	}
	
	var transactions []*MempoolTransaction
	if err := json.Unmarshal(txData, &transactions); err != nil {
		return fmt.Errorf("failed to unmarshal mempool transactions: %w", err)
	}
	
	// Add transactions to mempool
	for _, mempoolTx := range transactions {
		if err := ce.mempool.AddTransaction(mempoolTx.Transaction, SourceNetwork); err != nil {
			log.Printf("Failed to add transaction from peer %s: %v", peer.ID, err)
		}
	}
	
	log.Printf("Received %d mempool transactions from peer %s", len(transactions), peer.ID)
	return nil
}

// processIncomingBlock processes a block received from a peer
func (ce *ConsensusEngine) processIncomingBlock(block *Block) {
	// Check if we already have this block
	if _, err := ce.blockchain.GetBlock(block.Hash()); err == nil {
		log.Printf("Block %s already exists", block.Hash())
		return
	}
	
	// Try to add block to blockchain (this includes validation)
	if err := ce.blockchain.AddBlock(block); err != nil {
		log.Printf("Failed to add block to blockchain: %v", err)
		return
	}
	
	log.Printf("Successfully added block %d (hash: %s) to blockchain", 
		block.Header.Height, block.Hash())
	
	// Update best chain state
	ce.chainMutex.Lock()
	ce.bestChain = &ChainState{
		Height:    block.Header.Height,
		Hash:      block.Hash(),
		Timestamp: block.Header.Timestamp,
	}
	ce.chainMutex.Unlock()
	
	// Remove transactions from mempool
	for _, tx := range block.Body.Transactions {
		ce.mempool.RemoveTransaction(tx.TxHash)
	}
}

// processIncomingTransaction processes a transaction received from a peer
func (ce *ConsensusEngine) processIncomingTransaction(tx *SignedTransaction) {
	// Add transaction to mempool
	if err := ce.mempool.AddTransaction(tx, SourceNetwork); err != nil {
		log.Printf("Failed to add transaction to mempool: %v", err)
		return
	}
	
	log.Printf("Added transaction %s to mempool", tx.TxHash)
}

// handlePeerEvent handles peer-related events
func (ce *ConsensusEngine) handlePeerEvent(event *PeerEvent) {
	switch event.Type {
	case "connected":
		log.Printf("Peer %s connected", event.PeerID)
		
	case "disconnected":
		log.Printf("Peer %s disconnected", event.PeerID)
		
	case "new_block":
		log.Printf("Peer %s announced new block", event.PeerID)
		
	case "new_transaction":
		log.Printf("Peer %s announced new transaction", event.PeerID)
	}
}

// sendHeartbeats sends heartbeats to all connected peers
func (ce *ConsensusEngine) sendHeartbeats() {
	tip, err := ce.blockchain.GetTip()
	if err != nil {
		log.Printf("Failed to get tip for heartbeat: %v", err)
		return
	}
	
	heartbeatData := map[string]interface{}{
		"chain_height": tip.Header.Height,
		"chain_hash":   tip.Hash(),
		"timestamp":    time.Now().UTC(),
	}
	
	message := &P2PMessage{
		Type:      MsgTypeHeartbeat,
		From:      ce.nodeID,
		Data:      heartbeatData,
		Timestamp: time.Now().UTC(),
	}
	
	ce.broadcastMessage(message)
}

// broadcastMessage broadcasts a message to all connected peers
func (ce *ConsensusEngine) broadcastMessage(message *P2PMessage) {
	ce.peersMutex.RLock()
	peers := make([]*Peer, 0, len(ce.peers))
	for _, peer := range ce.peers {
		if peer.Status == "connected" || peer.Status == "active" {
			peers = append(peers, peer)
		}
	}
	ce.peersMutex.RUnlock()
	
	for _, peer := range peers {
		go func(p *Peer) {
			if err := ce.sendMessage(p.Connection, message); err != nil {
				log.Printf("Failed to send message to peer %s: %v", p.ID, err)
			} else {
				ce.peersMutex.Lock()
				if existingPeer, exists := ce.peers[p.ID]; exists {
					existingPeer.MessagesSent++
				}
				ce.peersMutex.Unlock()
			}
		}(peer)
	}
}

// cleanupPeers removes inactive peers
func (ce *ConsensusEngine) cleanupPeers() {
	ce.peersMutex.Lock()
	defer ce.peersMutex.Unlock()
	
	now := time.Now().UTC()
	for id, peer := range ce.peers {
		if now.Sub(peer.LastSeen) > ce.config.SyncTimeout*2 {
			log.Printf("Removing inactive peer %s", id)
			if peer.Connection != nil {
				peer.Connection.Close()
			}
			delete(ce.peers, id)
		}
	}
}

// bootstrapPeers connects to bootstrap peers
func (ce *ConsensusEngine) bootstrapPeers() {
	for _, peerAddr := range ce.config.BootstrapPeers {
		if err := ce.ConnectToPeer(peerAddr); err != nil {
			log.Printf("Failed to connect to bootstrap peer %s: %v", peerAddr, err)
		} else {
			log.Printf("Connected to bootstrap peer %s", peerAddr)
		}
		
		// Add delay between connections
		time.Sleep(time.Second)
	}
}

// performSync performs blockchain synchronization
func (ce *ConsensusEngine) performSync() {
	// Find the best peer to sync from
	bestPeer := ce.findBestPeer()
	if bestPeer == nil {
		return
	}
	
	currentTip, err := ce.blockchain.GetTip()
	if err != nil {
		log.Printf("Failed to get current tip for sync: %v", err)
		return
	}
	
	// Check if we need to sync
	if bestPeer.ChainHeight <= currentTip.Header.Height {
		return
	}
	
	log.Printf("Starting sync with peer %s (height %d vs our %d)", 
		bestPeer.ID, bestPeer.ChainHeight, currentTip.Header.Height)
	
	ce.statusMutex.Lock()
	ce.syncStatus = SyncStatus{
		IsSyncing:     true,
		CurrentHeight: currentTip.Header.Height,
		TargetHeight:  bestPeer.ChainHeight,
		SyncPeer:      bestPeer.ID,
		LastSyncTime:  time.Now().UTC(),
	}
	ce.statusMutex.Unlock()
	
	// Request blocks from peer
	ce.requestBlocksFromPeer(bestPeer, currentTip.Header.Height+1, bestPeer.ChainHeight)
}

// findBestPeer finds the peer with the highest chain height
func (ce *ConsensusEngine) findBestPeer() *Peer {
	ce.peersMutex.RLock()
	defer ce.peersMutex.RUnlock()
	
	var bestPeer *Peer
	var maxHeight uint64
	
	for _, peer := range ce.peers {
		if peer.Status == "connected" || peer.Status == "active" {
			if peer.ChainHeight > maxHeight {
				maxHeight = peer.ChainHeight
				bestPeer = peer
			}
		}
	}
	
	return bestPeer
}

// requestBlocksFromPeer requests a range of blocks from a peer
func (ce *ConsensusEngine) requestBlocksFromPeer(peer *Peer, startHeight, endHeight uint64) {
	for height := startHeight; height <= endHeight; height++ {
		request := &P2PMessage{
			Type: MsgTypeBlockRequest,
			From: ce.nodeID,
			To:   peer.ID,
			Data: map[string]interface{}{
				"height": height,
			},
			Timestamp: time.Now().UTC(),
		}
		
		if err := ce.sendMessage(peer.Connection, request); err != nil {
			log.Printf("Failed to request block %d from peer %s: %v", height, peer.ID, err)
			break
		}
		
		// Add small delay to avoid overwhelming peer
		time.Sleep(10 * time.Millisecond)
	}
}

// considerSync considers whether to sync with a peer
func (ce *ConsensusEngine) considerSync(peer *Peer, chainState *ChainState) {
	// Simple sync strategy: always sync with peers that have higher chain height
	currentTip, err := ce.blockchain.GetTip()
	if err != nil {
		log.Printf("Failed to get current tip: %v", err)
		return
	}
	
	if chainState.Height > currentTip.Header.Height {
		// Request blocks from this peer
		ce.requestBlocksFromPeer(peer, currentTip.Header.Height+1, chainState.Height)
	}
}