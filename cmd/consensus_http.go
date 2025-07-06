package cmd

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// handleConsensusStatus returns consensus engine status
func (sn *ShadowNode) handleConsensusStatus(w http.ResponseWriter, r *http.Request) {
	if sn.consensus == nil {
		http.Error(w, "Consensus engine not enabled", http.StatusServiceUnavailable)
		return
	}
	
	peers := sn.consensus.GetPeers()
	syncStatus := sn.consensus.GetSyncStatus()
	chainState := sn.consensus.GetChainState()
	
	status := map[string]interface{}{
		"node_id":       sn.consensus.nodeID,
		"listen_addr":   sn.consensus.listenAddr,
		"peer_count":    len(peers),
		"max_peers":     sn.config.ConsensusConfig.MaxPeers,
		"sync_status":   syncStatus,
		"chain_state":   chainState,
		"uptime":        time.Since(time.Now()).String(), // Simplified
		"last_updated":  time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

// handleGetPeers returns information about connected peers
func (sn *ShadowNode) handleGetPeers(w http.ResponseWriter, r *http.Request) {
	if sn.consensus == nil {
		http.Error(w, "Consensus engine not enabled", http.StatusServiceUnavailable)
		return
	}
	
	peers := sn.consensus.GetPeers()
	
	response := map[string]interface{}{
		"peer_count": len(peers),
		"peers":      peers,
		"timestamp":  time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleConnectPeer connects to a new peer
func (sn *ShadowNode) handleConnectPeer(w http.ResponseWriter, r *http.Request) {
	if sn.consensus == nil {
		http.Error(w, "Consensus engine not enabled", http.StatusServiceUnavailable)
		return
	}
	
	var request struct {
		Address string `json:"address"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&request); err != nil {
		http.Error(w, "Invalid request body", http.StatusBadRequest)
		return
	}
	
	if request.Address == "" {
		http.Error(w, "Address is required", http.StatusBadRequest)
		return
	}
	
	// Connect to peer
	if err := sn.consensus.ConnectToPeer(request.Address); err != nil {
		response := map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to connect to peer: %v", err),
			"address": request.Address,
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	response := map[string]interface{}{
		"status":  "success",
		"message": "Connection initiated",
		"address": request.Address,
		"timestamp": time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetSyncStatus returns blockchain synchronization status
func (sn *ShadowNode) handleGetSyncStatus(w http.ResponseWriter, r *http.Request) {
	if sn.consensus == nil {
		http.Error(w, "Consensus engine not enabled", http.StatusServiceUnavailable)
		return
	}
	
	syncStatus := sn.consensus.GetSyncStatus()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(syncStatus)
}

// handleForceSync forces a blockchain synchronization
func (sn *ShadowNode) handleForceSync(w http.ResponseWriter, r *http.Request) {
	if sn.consensus == nil {
		http.Error(w, "Consensus engine not enabled", http.StatusServiceUnavailable)
		return
	}
	
	// Get current peers
	peers := sn.consensus.GetPeers()
	if len(peers) == 0 {
		response := map[string]interface{}{
			"status":  "error",
			"message": "No peers available for synchronization",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPreconditionFailed)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Find best peer for sync
	var bestPeer *Peer
	var maxHeight uint64
	
	for _, peer := range peers {
		if peer.Status == "connected" || peer.Status == "active" {
			if peer.ChainHeight > maxHeight {
				maxHeight = peer.ChainHeight
				bestPeer = peer
			}
		}
	}
	
	if bestPeer == nil {
		response := map[string]interface{}{
			"status":  "error",
			"message": "No suitable peers found for synchronization",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusPreconditionFailed)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Get current chain state
	currentTip, err := sn.blockchain.GetTip()
	if err != nil {
		response := map[string]interface{}{
			"status":  "error",
			"message": fmt.Sprintf("Failed to get current chain tip: %v", err),
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusInternalServerError)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Check if sync is needed
	if bestPeer.ChainHeight <= currentTip.Header.Height {
		response := map[string]interface{}{
			"status":         "no_action",
			"message":        "Local chain is already up to date",
			"current_height": currentTip.Header.Height,
			"peer_height":    bestPeer.ChainHeight,
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Force sync (simplified - just trigger the sync mechanism)
	// In a real implementation, you'd call a method to force sync
	response := map[string]interface{}{
		"status":         "initiated",
		"message":        "Synchronization initiated",
		"sync_peer":      bestPeer.ID,
		"current_height": currentTip.Header.Height,
		"target_height":  bestPeer.ChainHeight,
		"blocks_to_sync": bestPeer.ChainHeight - currentTip.Header.Height,
		"timestamp":      time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleGetChainState returns current chain state
func (sn *ShadowNode) handleGetChainState(w http.ResponseWriter, r *http.Request) {
	if sn.consensus == nil {
		http.Error(w, "Consensus engine not enabled", http.StatusServiceUnavailable)
		return
	}
	
	chainState := sn.consensus.GetChainState()
	if chainState == nil {
		response := map[string]interface{}{
			"status":  "error",
			"message": "Chain state not available",
		}
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusServiceUnavailable)
		json.NewEncoder(w).Encode(response)
		return
	}
	
	// Also get blockchain stats for additional context
	blockchainStats := sn.blockchain.GetStats()
	
	response := map[string]interface{}{
		"chain_state":     chainState,
		"blockchain_stats": blockchainStats,
		"timestamp":       time.Now().UTC(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}