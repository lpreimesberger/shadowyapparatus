package cmd

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"runtime"
	"time"
)

// handleMonitoringAPI returns comprehensive monitoring data
func (wm *WebMonitor) handleMonitoringAPI(w http.ResponseWriter, r *http.Request) {
	data, err := wm.collectMonitoringData()
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to collect monitoring data: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(data)
}

// handleHealthAPI returns node health data
func (wm *WebMonitor) handleHealthAPI(w http.ResponseWriter, r *http.Request) {
	health, err := wm.fetchFromAPI("/api/v1/health")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch health data: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(health)
}

// handleMiningAPI returns mining statistics
func (wm *WebMonitor) handleMiningAPI(w http.ResponseWriter, r *http.Request) {
	mining, err := wm.fetchFromAPI("/api/v1/mining")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch mining data: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(mining)
}

// handleConsensusAPI returns consensus status
func (wm *WebMonitor) handleConsensusAPI(w http.ResponseWriter, r *http.Request) {
	consensus, err := wm.fetchFromAPI("/api/v1/consensus")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch consensus data: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(consensus)
}

// handleBlocksAPI returns recent blocks data
func (wm *WebMonitor) handleBlocksAPI(w http.ResponseWriter, r *http.Request) {
	blocks, err := wm.fetchFromAPI("/api/v1/blockchain")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch blocks data: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(blocks)
}

// handleTransactionsAPI returns mempool transactions
func (wm *WebMonitor) handleTransactionsAPI(w http.ResponseWriter, r *http.Request) {
	mempool, err := wm.fetchFromAPI("/api/v1/mempool")
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch mempool data: %v", err), http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	w.Write(mempool)
}

// handleMetricsAPI returns system metrics
func (wm *WebMonitor) handleMetricsAPI(w http.ResponseWriter, r *http.Request) {
	metrics := wm.collectSystemMetrics()
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// collectMonitoringData aggregates all monitoring information
func (wm *WebMonitor) collectMonitoringData() (*MonitoringData, error) {
	data := &MonitoringData{
		Timestamp:     time.Now(),
		SystemMetrics: wm.collectSystemMetrics(),
	}
	
	// Fetch data from various API endpoints
	if health, err := wm.fetchFromAPIJSON("/api/v1/health"); err == nil {
		data.NodeHealth = health
	}
	
	if blockchain, err := wm.fetchFromAPIJSON("/api/v1/blockchain"); err == nil {
		data.BlockchainStatus = blockchain
		
		// Extract recent blocks if available
		if blocks, ok := blockchain["blocks"].([]interface{}); ok {
			var recentBlocks []map[string]interface{}
			for i, block := range blocks {
				if i >= 10 { break } // Limit to 10 most recent
				if blockMap, ok := block.(map[string]interface{}); ok {
					recentBlocks = append(recentBlocks, blockMap)
				}
			}
			data.RecentBlocks = recentBlocks
		}
	}
	
	if mining, err := wm.fetchFromAPIJSON("/api/v1/mining"); err == nil {
		data.MiningStats = mining
		
		// Calculate blocks per hour
		if status, ok := mining["status"].(map[string]interface{}); ok {
			if blocksMinedToday, ok := status["blocks_mined_today"].(float64); ok {
				// Simple estimation: assume even distribution over 24 hours
				blocksPerHour := blocksMinedToday / 24
				if data.MiningStats == nil {
					data.MiningStats = make(map[string]interface{})
				}
				data.MiningStats["blocks_per_hour"] = blocksPerHour
			}
		}
	}
	
	if consensus, err := wm.fetchFromAPIJSON("/api/v1/consensus"); err == nil {
		data.ConsensusStatus = consensus
	}
	
	if mempool, err := wm.fetchFromAPIJSON("/api/v1/mempool"); err == nil {
		data.MempoolStats = mempool
		
		// Extract recent transactions if available
		if transactions, ok := mempool["transactions"].([]interface{}); ok {
			var recentTxs []map[string]interface{}
			for i, tx := range transactions {
				if i >= 10 { break } // Limit to 10 most recent
				if txMap, ok := tx.(map[string]interface{}); ok {
					recentTxs = append(recentTxs, txMap)
				}
			}
			data.RecentTransactions = recentTxs
		}
	}
	
	if timelord, err := wm.fetchFromAPIJSON("/api/v1/timelord"); err == nil {
		data.TimelordStats = timelord
	}
	
	if farming, err := wm.fetchFromAPIJSON("/api/v1/farming"); err == nil {
		data.FarmingStats = farming
	}
	
	return data, nil
}

// collectSystemMetrics gathers system-level metrics
func (wm *WebMonitor) collectSystemMetrics() SystemMetrics {
	var mem runtime.MemStats
	runtime.ReadMemStats(&mem)
	
	return SystemMetrics{
		Uptime:      time.Since(startTime),
		MemoryUsage: mem.Alloc / 1024 / 1024, // Convert to MB
		Goroutines:  runtime.NumGoroutine(),
		// CPU and disk metrics would require additional libraries
		// For now, we'll use simple placeholders
		CPUUsage:   0.0, // Would use github.com/shirou/gopsutil for real metrics
		DiskUsage:  0,   // Would use github.com/shirou/gopsutil for real metrics
		NetworkIn:  0,   // Would track in real implementation
		NetworkOut: 0,   // Would track in real implementation
	}
}

// fetchFromAPI makes an HTTP request to the blockchain API
func (wm *WebMonitor) fetchFromAPI(endpoint string) ([]byte, error) {
	url := wm.apiBaseURL + endpoint
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %v", url, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d for %s", resp.StatusCode, url)
	}
	
	// Read the full response body
	result, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %v", err)
	}
	
	return result, nil
}

// fetchFromAPIJSON makes an HTTP request and parses JSON response
func (wm *WebMonitor) fetchFromAPIJSON(endpoint string) (map[string]interface{}, error) {
	url := wm.apiBaseURL + endpoint
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %v", url, err)
	}
	defer resp.Body.Close()
	
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API returned status %d for %s", resp.StatusCode, url)
	}
	
	var result map[string]interface{}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %v", err)
	}
	
	return result, nil
}

// handleStaticAssets serves static files (CSS, JS, images)
func (wm *WebMonitor) handleStaticAssets(w http.ResponseWriter, r *http.Request) {
	// For now, we serve a simple 404 for static assets
	// In a full implementation, you'd serve actual static files
	http.NotFound(w, r)
}

// handleWebSocket handles WebSocket connections for real-time updates
func (wm *WebMonitor) handleWebSocket(w http.ResponseWriter, r *http.Request) {
	// WebSocket implementation would go here
	// For now, return a placeholder
	w.Header().Set("Content-Type", "text/plain")
	w.Write([]byte("WebSocket endpoint - not implemented yet"))
}

// startTime tracks when the monitoring service started
var startTime = time.Now()