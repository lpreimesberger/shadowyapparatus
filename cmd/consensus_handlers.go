package cmd

import (
    "encoding/json"
    "fmt"
    "log"
    "strings"
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

    // Check if we already have this block to avoid relay loops
    if _, err := ce.blockchain.GetBlock(block.Hash()); err == nil {
        log.Printf("Block %d already exists, not relaying", block.Header.Height)
        return nil
    }

    // Queue block for processing
    select {
    case ce.blockChan <- &block:
        log.Printf("Received new block %d from peer %s", block.Header.Height, peer.ID)

        // Relay block to other peers (but not back to sender)
        ce.relayBlockToPeers(&block, peer.ID)

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
    log.Printf("peer %s wants block %d", peer.ID, block.Header.Height)
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
        log.Printf("🚨 [CONSENSUS] Peer %s has longer chain: %d vs our %d - implementing longest chain rule",
            peer.ID, chainState.Height, currentTip.Header.Height)
        ce.handleLongerChain(peer, &chainState)
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

// processIncomingBlock processes a block received from a peer with ordering
func (ce *ConsensusEngine) processIncomingBlock(block *Block) {
    // Check if we already have this block
    if _, err := ce.blockchain.GetBlock(block.Hash()); err == nil {
        log.Printf("Block %d already exists, skipping", block.Header.Height)
        return
    }

    ce.syncMutex.Lock()
    defer ce.syncMutex.Unlock()

    blockHeight := block.Header.Height

    // If this is the next expected block, try to process it immediately
    if blockHeight == ce.nextExpectedHeight {
        ce.processBlockSequentially(block)

        // Check if we can now process any pending blocks
        ce.processPendingBlocks()
    } else if blockHeight > ce.nextExpectedHeight {
        // Future block - store for later processing
        log.Printf("Received future block %d (expecting %d), buffering...",
            blockHeight, ce.nextExpectedHeight)
        ce.pendingBlocks[blockHeight] = block

        // Check for potential sync loop: if we keep receiving blocks far ahead of what we expect,
        // our nextExpectedHeight might be stale. Try to recover by checking blockchain state.
        if blockHeight > ce.nextExpectedHeight+10 {
            log.Printf("⚠️  Large gap detected: expecting %d but received %d", 
                ce.nextExpectedHeight, blockHeight)
            
            // Check if our blockchain actually has more blocks than nextExpectedHeight suggests
            if tip, err := ce.blockchain.GetTip(); err == nil {
                correctNextHeight := tip.Header.Height + 1
                if correctNextHeight > ce.nextExpectedHeight {
                    log.Printf("🔧 [SYNC RECOVERY] Blockchain tip is at height %d, but nextExpectedHeight was %d", 
                        tip.Header.Height, ce.nextExpectedHeight)
                    log.Printf("🔧 [SYNC RECOVERY] Updating nextExpectedHeight from %d to %d", 
                        ce.nextExpectedHeight, correctNextHeight)
                    ce.nextExpectedHeight = correctNextHeight
                    
                    // Re-evaluate the current block with the corrected height
                    if blockHeight == ce.nextExpectedHeight {
                        log.Printf("🔧 [SYNC RECOVERY] Block %d is now the expected next block, processing...", blockHeight)
                        ce.processBlockSequentially(block)
                        ce.processPendingBlocks()
                        return
                    }
                }
            }
        }

        // Check if we need to request missing blocks when there's a gap
        // Throttle requests to avoid spamming - only request every 10 seconds
        if blockHeight > ce.nextExpectedHeight+1 {
            now := time.Now()
            if now.Sub(ce.lastMissingBlockRequest) > 10*time.Second {
                ce.lastMissingBlockRequest = now
                go func() {
                    log.Printf("🔍 Detected gap: expecting %d but received %d, requesting missing blocks",
                        ce.nextExpectedHeight, blockHeight)
                    ce.requestMissingBlocks()
                }()
            }
        }

        // Limit buffer size to prevent memory issues
        if len(ce.pendingBlocks) > 500 {
            log.Printf("Warning: Pending blocks buffer is large (%d blocks)", len(ce.pendingBlocks))
            // Remove oldest pending blocks (keep only 200 blocks ahead of next expected)
            for height := range ce.pendingBlocks {
                if height > ce.nextExpectedHeight+200 { // Remove blocks too far ahead
                    delete(ce.pendingBlocks, height)
                }
            }
        }
    } else {
        // Past block - we might have missed it or it's a duplicate
        log.Printf("Received old block %d (expecting %d), attempting to process anyway",
            blockHeight, ce.nextExpectedHeight)
        ce.processBlockSequentially(block)
    }
}

// processBlockSequentially processes a block and updates the chain state
func (ce *ConsensusEngine) processBlockSequentially(block *Block) {
    // Try to add block to blockchain (this includes validation)
    if err := ce.blockchain.AddBlock(block); err != nil {
        log.Printf("❌ Failed to add block %d to blockchain: %v", block.Header.Height, err)
        
        // Check if this is a duplicate block error (already exists)
        // In this case, we should still advance nextExpectedHeight to avoid getting stuck
        if strings.Contains(err.Error(), "already exists") || strings.Contains(err.Error(), "duplicate") {
            log.Printf("⚠️  Block %d appears to be duplicate, advancing nextExpectedHeight to avoid sync loop", 
                block.Header.Height)
            ce.nextExpectedHeight = block.Header.Height + 1
        } else {
            // For other errors (validation, storage), don't advance to prevent skipping blocks
            log.Printf("⚠️  Block %d processing failed with non-duplicate error, keeping nextExpectedHeight=%d", 
                block.Header.Height, ce.nextExpectedHeight)
        }
        return
    }

    log.Printf("✅ Successfully added block %d (hash: %s) to blockchain",
        block.Header.Height, block.Hash())

    // Update best chain state
    ce.chainMutex.Lock()
    ce.bestChain = &ChainState{
        Height:    block.Header.Height,
        Hash:      block.Hash(),
        Timestamp: block.Header.Timestamp,
    }
    ce.chainMutex.Unlock()

    // Update next expected height
    ce.nextExpectedHeight = block.Header.Height + 1

    // Remove transactions from mempool
    for _, tx := range block.Body.Transactions {
        ce.mempool.RemoveTransaction(tx.TxHash)
    }

    // Check if chain reorganization sync is complete
    ce.statusMutex.RLock()
    isSyncing := ce.syncStatus.IsSyncing
    targetHeight := ce.syncStatus.TargetHeight
    ce.statusMutex.RUnlock()

    if isSyncing && block.Header.Height >= targetHeight {
        log.Printf("🎉 [CONSENSUS] Chain reorganization complete! Reached target height %d", targetHeight)
        log.Printf("🔄 [CONSENSUS] Updated to longer chain - longest chain wins consensus applied")

        // Mark sync as complete
        ce.statusMutex.Lock()
        ce.syncStatus.IsSyncing = false
        ce.statusMutex.Unlock()

        // Restart mining on the new chain tip
        if ce.miner != nil && !ce.miner.IsRunning() {
            if err := ce.miner.Start(); err != nil {
                log.Printf("❌ [CONSENSUS] Failed to restart miner after sync completion: %v", err)
            } else {
                log.Printf("🔨 [CONSENSUS] Miner restarted successfully after sync completion")
            }
        } else {
            log.Printf("🔨 [CONSENSUS] Ready to resume mining on reorganized chain")
        }
    }
}

// processPendingBlocks processes any pending blocks that can now be added
func (ce *ConsensusEngine) processPendingBlocks() {
    for {
        // Check if we have the next expected block in pending
        nextBlock, exists := ce.pendingBlocks[ce.nextExpectedHeight]
        if !exists {
            break // No more sequential blocks available
        }

        // Remove from pending and process it
        delete(ce.pendingBlocks, ce.nextExpectedHeight)

        log.Printf("Processing pending block %d from buffer", ce.nextExpectedHeight)
        ce.processBlockSequentially(nextBlock)
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

    // Also request chain state from peers to detect longer chains
    ce.requestChainStatesFromPeers()
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

// performSync performs blockchain synchronization with improved logic
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
        // Check if sync is complete
        ce.statusMutex.Lock()
        if ce.syncStatus.IsSyncing {
            log.Printf("✅ Sync complete! Current height: %d", currentTip.Header.Height)
            ce.syncStatus.IsSyncing = false
        }
        ce.statusMutex.Unlock()
        return
    }

    // Check if we're already syncing and making progress
    ce.statusMutex.RLock()
    isSyncing := ce.syncStatus.IsSyncing
    lastHeight := ce.syncStatus.CurrentHeight
    ce.statusMutex.RUnlock()

    currentHeight := currentTip.Header.Height

    if isSyncing {
        if currentHeight > lastHeight {
            // Making progress, update status
            ce.statusMutex.Lock()
            ce.syncStatus.CurrentHeight = currentHeight
            ce.syncStatus.SyncProgress = float64(currentHeight-lastHeight) / float64(bestPeer.ChainHeight-lastHeight)
            ce.statusMutex.Unlock()
            log.Printf("🔄 Sync progress: %d/%d (%.1f%%)",
                currentHeight, bestPeer.ChainHeight, ce.syncStatus.SyncProgress*100)
        }

        // Don't start new sync if already in progress
        return
    }

    log.Printf("🚀 Starting sync with peer %s (height %d vs our %d)",
        bestPeer.ID, bestPeer.ChainHeight, currentHeight)

    ce.statusMutex.Lock()
    ce.syncStatus = SyncStatus{
        IsSyncing:     true,
        CurrentHeight: currentHeight,
        TargetHeight:  bestPeer.ChainHeight,
        SyncPeer:      bestPeer.ID,
        LastSyncTime:  time.Now().UTC(),
    }
    ce.statusMutex.Unlock()

    // Use next expected height for sync to ensure proper ordering
    ce.syncMutex.RLock()
    nextHeight := ce.nextExpectedHeight
    ce.syncMutex.RUnlock()

    // Request blocks from next expected height
    ce.requestBlocksFromPeer(bestPeer, nextHeight, bestPeer.ChainHeight)
}

// requestMissingBlocks requests missing blocks when a gap is detected
func (ce *ConsensusEngine) requestMissingBlocks() {
    ce.syncMutex.RLock()
    nextExpected := ce.nextExpectedHeight

    // Find the lowest buffered block to determine the immediate gap
    var lowestBuffered uint64 = 0
    for height := range ce.pendingBlocks {
        if lowestBuffered == 0 || height < lowestBuffered {
            lowestBuffered = height
        }
    }
    ce.syncMutex.RUnlock()

    if lowestBuffered == 0 {
        return // No buffered blocks
    }

    // Find best peer to request from
    bestPeer := ce.findBestPeer()
    if bestPeer == nil {
        log.Printf("No peers available to request missing blocks")
        return
    }

    // Request only a reasonable batch size to fill the immediate gap
    // This prevents requesting too many blocks that can't be applied
    endHeight := lowestBuffered - 1
    if endHeight >= nextExpected {
        // Limit request to a reasonable batch size (e.g., 100 blocks)
        batchSize := uint64(100)
        if endHeight-nextExpected+1 > batchSize {
            endHeight = nextExpected + batchSize - 1
        }

        log.Printf("🔄 Requesting missing blocks %d-%d from peer %s (immediate gap fill)",
            nextExpected, endHeight, bestPeer.ID)
        ce.requestBlocksFromPeer(bestPeer, nextExpected, endHeight)
    }
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

// requestBlocksFromPeer requests a limited range of blocks from a peer
func (ce *ConsensusEngine) requestBlocksFromPeer(peer *Peer, startHeight, endHeight uint64) {
    // Limit batch size to prevent overwhelming the network
    const maxBatchSize = 50

    batchEnd := startHeight + maxBatchSize - 1
    if batchEnd > endHeight {
        batchEnd = endHeight
    }

    log.Printf("Requesting blocks %d-%d from peer %s", startHeight, batchEnd, peer.ID)

    for height := startHeight; height <= batchEnd; height++ {
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
        time.Sleep(50 * time.Millisecond)
    }

    // Schedule next batch if there are more blocks to sync
    if batchEnd < endHeight {
        go func() {
            time.Sleep(500 * time.Millisecond) // Wait before requesting next batch
            ce.requestBlocksFromPeer(peer, batchEnd+1, endHeight)
        }()
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

// isConnectedToPeerAddress checks if we're already connected to a peer address
func (ce *ConsensusEngine) isConnectedToPeerAddress(address string) bool {
    ce.peersMutex.RLock()
    defer ce.peersMutex.RUnlock()

    for _, peer := range ce.peers {
        if peer.Address == address && (peer.Status == "connected" || peer.Status == "active") {
            return true
        }
    }
    return false
}

// relayBlockToPeers relays a block to all connected peers except the sender
func (ce *ConsensusEngine) relayBlockToPeers(block *Block, senderID string) {
    relayMessage := &P2PMessage{
        Type:      MsgTypeNewBlock,
        From:      ce.nodeID,
        Data:      block,
        Timestamp: time.Now().UTC(),
    }

    ce.peersMutex.RLock()
    var relayPeers []*Peer
    for _, peer := range ce.peers {
        // Relay to all connected peers except the sender
        if peer.ID != senderID && (peer.Status == "connected" || peer.Status == "active") {
            relayPeers = append(relayPeers, peer)
        }
    }
    ce.peersMutex.RUnlock()

    if len(relayPeers) > 0 {
        log.Printf("📡 Relaying block %d to %d peers (excluding sender %s)",
            block.Header.Height, len(relayPeers), senderID)

        for _, peer := range relayPeers {
            go func(p *Peer) {
                if err := ce.sendMessage(p.Connection, relayMessage); err != nil {
                    log.Printf("Failed to relay block to peer %s: %v", p.ID, err)
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
}

// handleLongerChain implements longest-chain-wins consensus when a peer has a longer chain
func (ce *ConsensusEngine) handleLongerChain(peer *Peer, peerChainState *ChainState) {
    currentTip, err := ce.blockchain.GetTip()
    if err != nil {
        log.Printf("❌ [CONSENSUS] Failed to get current tip: %v", err)
        return
    }

    heightDiff := peerChainState.Height - currentTip.Header.Height
    log.Printf("⛓️  [CONSENSUS] Chain reorganization needed:")
    log.Printf("   📊 Current chain: height %d, hash %s", currentTip.Header.Height, currentTip.Hash()[:16]+"...")
    log.Printf("   📈 Peer chain: height %d, hash %s", peerChainState.Height, peerChainState.Hash[:16]+"...")
    log.Printf("   📏 Height difference: +%d blocks", heightDiff)

    // Critical: Stop mining immediately to prevent working on wrong chain
    log.Printf("⏸️  [CONSENSUS] Stopping mining to reorganize to longer chain...")
    if ce.miner != nil && ce.miner.IsRunning() {
        if err := ce.miner.Stop(); err != nil {
            log.Printf("❌ [CONSENSUS] Failed to stop miner: %v", err)
            return
        }
        log.Printf("✅ [CONSENSUS] Miner stopped successfully")
    }

    // Set sync status
    ce.statusMutex.Lock()
    ce.syncStatus = SyncStatus{
        IsSyncing:     true,
        CurrentHeight: currentTip.Header.Height,
        TargetHeight:  peerChainState.Height,
        SyncPeer:      peer.ID,
        LastSyncTime:  time.Now().UTC(),
    }
    ce.statusMutex.Unlock()

    // Request the full chain from the peer for reorganization
    // We need to get enough blocks to find the common ancestor
    startHeight := uint64(0) // Start from genesis to ensure we can find common ancestor
    if currentTip.Header.Height > 100 {
        // Optimize: only get last 100 blocks plus the new ones for fork detection
        startHeight = currentTip.Header.Height - 100
    }

    log.Printf("🔄 [CONSENSUS] Requesting full reorganization chain blocks %d-%d from peer %s",
        startHeight, peerChainState.Height, peer.ID)

    // Start chain reorganization process
    go ce.performChainReorganization(peer, startHeight, peerChainState.Height, peerChainState)
}

// performChainReorganization performs a complete chain reorganization with the peer's longer chain
func (ce *ConsensusEngine) performChainReorganization(peer *Peer, startHeight, endHeight uint64, peerChainState *ChainState) {
    log.Printf("🔄 [CONSENSUS] Starting chain reorganization process...")

    // Collect all blocks from the peer's chain
    reorganizationBlocks := make(map[uint64]*Block)
    const batchSize = 10

    // Request blocks in batches
    for height := startHeight; height <= endHeight; height += batchSize {
        batchEnd := height + batchSize - 1
        if batchEnd > endHeight {
            batchEnd = endHeight
        }

        log.Printf("📥 [CONSENSUS] Requesting reorganization batch: blocks %d-%d", height, batchEnd)

        // Request each block in the batch
        for h := height; h <= batchEnd; h++ {
            request := &P2PMessage{
                Type: MsgTypeBlockRequest,
                From: ce.nodeID,
                To:   peer.ID,
                Data: map[string]interface{}{
                    "height": h,
                },
                Timestamp: time.Now().UTC(),
            }

            if err := ce.sendMessage(peer.Connection, request); err != nil {
                log.Printf("❌ [CONSENSUS] Failed to request block %d: %v", h, err)
                return
            }
        }

        // Wait for blocks to arrive (with timeout)
        timeout := time.NewTimer(30 * time.Second)
        startTime := time.Now()

        for receivedBlocks := height; receivedBlocks <= batchEnd; {
            select {
            case block := <-ce.blockChan:
                if block.Header.Height >= height && block.Header.Height <= batchEnd {
                    reorganizationBlocks[block.Header.Height] = block
                    receivedBlocks++
                    log.Printf("   📦 Collected reorganization block %d", block.Header.Height)
                } else {
                    // Put back on channel if not part of our reorganization range
                    select {
                    case ce.blockChan <- block:
                    default:
                        log.Printf("Warning: Dropped block %d (reorganization in progress)", block.Header.Height)
                    }
                }
            case <-timeout.C:
                log.Printf("❌ [CONSENSUS] Timeout waiting for reorganization blocks %d-%d", height, batchEnd)
                return
            }

            // Check if we got all blocks in this batch
            allReceived := true
            for h := height; h <= batchEnd; h++ {
                if _, exists := reorganizationBlocks[h]; !exists {
                    allReceived = false
                    break
                }
            }
            if allReceived {
                break
            }
        }

        timeout.Stop()
        log.Printf("✅ [CONSENSUS] Collected batch %d-%d in %v", height, batchEnd, time.Since(startTime))

        // Small delay between batches
        time.Sleep(1 * time.Second)
    }

    // Convert map to sorted slice
    var newChainBlocks []*Block
    for h := startHeight; h <= endHeight; h++ {
        if block, exists := reorganizationBlocks[h]; exists {
            newChainBlocks = append(newChainBlocks, block)
        } else {
            log.Printf("❌ [CONSENSUS] Missing block %d for reorganization", h)
            return
        }
    }

    log.Printf("📚 [CONSENSUS] Collected %d blocks for reorganization", len(newChainBlocks))

    // Perform the blockchain reorganization
    if err := ce.blockchain.ReorganizeChain(newChainBlocks, peerChainState.Height); err != nil {
        log.Printf("❌ [CONSENSUS] Chain reorganization failed: %v", err)

        // Mark sync as failed and restart miner on original chain
        ce.statusMutex.Lock()
        ce.syncStatus.IsSyncing = false
        ce.statusMutex.Unlock()

        // Restart miner since reorganization failed
        if ce.miner != nil && !ce.miner.IsRunning() {
            if err := ce.miner.Start(); err != nil {
                log.Printf("❌ [CONSENSUS] Failed to restart miner after failed reorganization: %v", err)
            } else {
                log.Printf("🔨 [CONSENSUS] Miner restarted on original chain after failed reorganization")
            }
        }
        return
    }

    // Mark reorganization as complete
    ce.statusMutex.Lock()
    ce.syncStatus.IsSyncing = false
    ce.statusMutex.Unlock()

    log.Printf("🎉 [CONSENSUS] Chain reorganization completed successfully!")
    log.Printf("   📏 New chain height: %d", peerChainState.Height)
    log.Printf("   🔄 Longest-chain consensus rule applied")

    // Restart mining on the reorganized chain
    if ce.miner != nil && !ce.miner.IsRunning() {
        if err := ce.miner.Start(); err != nil {
            log.Printf("❌ [CONSENSUS] Failed to restart miner after reorganization: %v", err)
        } else {
            log.Printf("🔨 [CONSENSUS] Miner restarted successfully on reorganized chain")
        }
    }
}

// requestChainStatesFromPeers actively requests chain state from all peers to detect longer chains
func (ce *ConsensusEngine) requestChainStatesFromPeers() {
    ce.peersMutex.RLock()
    var activePeers []*Peer
    for _, peer := range ce.peers {
        if peer.Status == "connected" || peer.Status == "active" {
            activePeers = append(activePeers, peer)
        }
    }
    ce.peersMutex.RUnlock()

    for _, peer := range activePeers {
        go func(p *Peer) {
            request := &P2PMessage{
                Type:      MsgTypeChainRequest,
                From:      ce.nodeID,
                To:        p.ID,
                Timestamp: time.Now().UTC(),
            }

            if err := ce.sendMessage(p.Connection, request); err != nil {
                log.Printf("Failed to request chain state from peer %s: %v", p.ID, err)
            }
        }(peer)
    }
}
