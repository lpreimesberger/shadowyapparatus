package main

import (
    "encoding/json"
    "fmt"
    "log"
    "net"
    "net/http"
    "strings"
    "time"

    "github.com/gorilla/mux"
)

// TrackerService manages network peer discovery and statistics
type TrackerService struct {
    nodes    map[string]*RegisteredNode
    registry *NodeRegistry
    server   *http.Server
}

// RegisteredNode represents a registered blockchain node
type RegisteredNode struct {
    // Identity
    NodeID     string `json:"node_id"`
    MiningAddr string `json:"mining_address"`
    PublicKey  string `json:"public_key"`

    // Network info
    ExternalIP string `json:"external_ip"` // Self-reported IP
    ObservedIP string `json:"observed_ip"` // IP as seen by tracker
    P2PPort    int    `json:"p2p_port"`
    HTTPPort   int    `json:"http_port"`

    // Chain state
    ChainHeight   uint64    `json:"chain_height"`
    ChainHash     string    `json:"chain_hash"`
    ChainID       string    `json:"chain_id"`
    LastBlockTime time.Time `json:"last_block_time"`

    // System info
    SoftwareVersion string `json:"software_version"`
    OSVersion       string `json:"os_version"`
    Architecture    string `json:"architecture"`

    // Farming info
    TotalPlotSize uint64 `json:"total_plot_size_bytes"`
    PlotCount     int    `json:"plot_count"`

    // Registration
    Signature     string    `json:"signature"`
    RegisteredAt  time.Time `json:"registered_at"`
    LastHeartbeat time.Time `json:"last_heartbeat"`
    Status        string    `json:"status"` // "online", "offline", "syncing"
}

// RegistrationRequest represents a node registration request
type RegistrationRequest struct {
    NodeID          string `json:"node_id"`
    MiningAddr      string `json:"mining_address"`
    PublicKey       string `json:"public_key"`
    ExternalIP      string `json:"external_ip"`
    P2PPort         int    `json:"p2p_port"`
    HTTPPort        int    `json:"http_port"`
    ChainHeight     uint64 `json:"chain_height"`
    ChainHash       string `json:"chain_hash"`
    ChainID         string `json:"chain_id"`
    LastBlockTime   string `json:"last_block_time"`
    SoftwareVersion string `json:"software_version"`
    OSVersion       string `json:"os_version"`
    Architecture    string `json:"architecture"`
    TotalPlotSize   uint64 `json:"total_plot_size_bytes"`
    PlotCount       int    `json:"plot_count"`
    Timestamp       string `json:"timestamp"`
    Signature       string `json:"signature"`
}

// HeartbeatRequest represents a node heartbeat update
type HeartbeatRequest struct {
    NodeID        string `json:"node_id"`
    ChainHeight   uint64 `json:"chain_height"`
    ChainHash     string `json:"chain_hash"`
    LastBlockTime string `json:"last_block_time"`
    Status        string `json:"status"`
    TotalPlotSize uint64 `json:"total_plot_size_bytes,omitempty"`
    PlotCount     int    `json:"plot_count,omitempty"`
    Timestamp     string `json:"timestamp"`
    Signature     string `json:"signature"`
}

// NetworkStats represents overall network statistics
type NetworkStats struct {
    TotalNodes      int    `json:"total_nodes"`
    OnlineNodes     int    `json:"online_nodes"`
    SyncingNodes    int    `json:"syncing_nodes"`
    TotalNetspace   uint64 `json:"total_netspace_bytes"`
    HighestHeight   uint64 `json:"highest_height"`
    ConsensusHeight uint64 `json:"consensus_height"`
    ForkCount       int    `json:"fork_count"`
    LastUpdated     string `json:"last_updated"`
}

// NodeRegistry manages the collection of registered nodes
type NodeRegistry struct {
    nodes map[string]*RegisteredNode
}

func NewTrackerService() *TrackerService {
    return &TrackerService{
        nodes:    make(map[string]*RegisteredNode),
        registry: &NodeRegistry{nodes: make(map[string]*RegisteredNode)},
    }
}

func main() {
    log.Println("🚀 Starting Shadowy Network Tracker Service")

    tracker := NewTrackerService()

    // Set up HTTP routes
    r := mux.NewRouter()

    // API routes
    api := r.PathPrefix("/api/v1").Subrouter()
    api.HandleFunc("/register", tracker.handleRegister).Methods("POST")
    api.HandleFunc("/heartbeat", tracker.handleHeartbeat).Methods("POST")
    api.HandleFunc("/peers", tracker.handleGetPeers).Methods("GET")
    api.HandleFunc("/stats", tracker.handleGetStats).Methods("GET")
    api.HandleFunc("/nodes", tracker.handleGetNodes).Methods("GET")
    api.HandleFunc("/node/{nodeId}", tracker.handleGetNode).Methods("GET")

    // Genesis endpoint for node bootstrapping
    r.HandleFunc("/v1/sxe", tracker.handleGetGenesis).Methods("GET")

    // Web dashboard routes
    r.HandleFunc("/", tracker.handleDashboard).Methods("GET")
    r.HandleFunc("/dashboard", tracker.handleDashboard).Methods("GET")
    r.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

    // Configure server
    tracker.server = &http.Server{
        Addr:         ":8090",
        Handler:      r,
        ReadTimeout:  15 * time.Second,
        WriteTimeout: 15 * time.Second,
    }

    // Start cleanup routine
    go tracker.cleanupOfflineNodes()

    log.Println("📡 Tracker service listening on :8090")
    log.Println("🌐 Dashboard available at http://boobies.local:8090")
    log.Println("📊 API available at http://boobies.local:8090/api/v1")

    if err := tracker.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
        log.Fatalf("❌ Failed to start server: %v", err)
    }
}

// handleRegister processes node registration requests
func (ts *TrackerService) handleRegister(w http.ResponseWriter, r *http.Request) {
    var req RegistrationRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // TODO: Verify signature against mining address
    // For now, accept all registrations
    if req.ChainID != testnet0 {
        log.Printf("client connecting with unknown chain for this tracker: %s", req.ChainID)
        http.Error(w, "your genesis block does not match any known active chains", http.StatusBadRequest)
        return
    }

    // Parse timestamps
    lastBlockTime, _ := time.Parse(time.RFC3339, req.LastBlockTime)

    // Extract client's actual IP address
    clientIP := extractClientIP(r)

    log.Printf("Incoming client for chain %s from %s", req.ChainID, clientIP)

    // Create registered node
    node := &RegisteredNode{
        NodeID:          req.NodeID,
        MiningAddr:      req.MiningAddr,
        PublicKey:       req.PublicKey,
        ExternalIP:      req.ExternalIP,
        ObservedIP:      clientIP,
        P2PPort:         req.P2PPort,
        HTTPPort:        req.HTTPPort,
        ChainHeight:     req.ChainHeight,
        ChainHash:       req.ChainHash,
        ChainID:         hash2chain(req.ChainID),
        LastBlockTime:   lastBlockTime,
        SoftwareVersion: req.SoftwareVersion,
        OSVersion:       req.OSVersion,
        Architecture:    req.Architecture,
        TotalPlotSize:   req.TotalPlotSize,
        PlotCount:       req.PlotCount,
        Signature:       req.Signature,
        RegisteredAt:    time.Now(),
        LastHeartbeat:   time.Now(),
        Status:          "online",
    }

    // Store node
    ts.nodes[req.NodeID] = node
    ts.registry.nodes[req.NodeID] = node

    log.Printf("✅ Registered node %s (mining: %s, height: %d, plots: %d)",
        req.NodeID, req.MiningAddr[:16]+"...", req.ChainHeight, req.PlotCount)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Node registered successfully",
        "node_id": req.NodeID,
    })
}

// handleHeartbeat processes node heartbeat updates
func (ts *TrackerService) handleHeartbeat(w http.ResponseWriter, r *http.Request) {
    var req HeartbeatRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Find existing node
    node, exists := ts.nodes[req.NodeID]
    if !exists {
        http.Error(w, "Node not registered", http.StatusNotFound)
        return
    }

    // Update node state
    lastBlockTime, _ := time.Parse(time.RFC3339, req.LastBlockTime)
    node.ChainHeight = req.ChainHeight
    node.ChainHash = req.ChainHash
    node.LastBlockTime = lastBlockTime
    node.Status = req.Status
    node.LastHeartbeat = time.Now()

    // Update plot information if provided
    if req.TotalPlotSize > 0 {
        node.TotalPlotSize = req.TotalPlotSize
        node.PlotCount = req.PlotCount
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "success": true,
        "message": "Heartbeat received",
    })
}

// handleGetPeers returns list of peers for node discovery
func (ts *TrackerService) handleGetPeers(w http.ResponseWriter, r *http.Request) {
    var activePeers []map[string]interface{}

    // Get requested chain ID from query parameter
    requestedChainIDRaw := r.URL.Query().Get("chain_id")
    requestedChainID := hash2chain(requestedChainIDRaw)
    log.Printf("client wants %s (nee %s)", requestedChainID, requestedChainIDRaw)
    for _, node := range ts.nodes {
        if node.Status == "online" && time.Since(node.LastHeartbeat) < 5*time.Minute {
            // Filter by chain ID if specified
            if requestedChainID != "" && node.ChainID != requestedChainID {
                log.Printf("Ignoring node %s with different chain ID %s", node.NodeID, node.ChainID)
                continue // Skip nodes with different chain IDs
            }

            // Use observed IP instead of self-reported IP for peer discovery
            ip := node.ObservedIP
            if ip == "" || ip == "unknown" {
                ip = node.ExternalIP // Fallback to self-reported IP
            }

            peer := map[string]interface{}{
                "node_id":      node.NodeID,
                "address":      fmt.Sprintf("%s:%d", ip, node.P2PPort),
                "chain_height": node.ChainHeight,
                "chain_hash":   node.ChainHash,
                "chain_id":     node.ChainID,
                "last_seen":    node.LastHeartbeat,
            }
            activePeers = append(activePeers, peer)
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "peers": activePeers,
        "count": len(activePeers),
    })
}

// handleGetStats returns network statistics
func (ts *TrackerService) handleGetStats(w http.ResponseWriter, r *http.Request) {
    stats := ts.calculateNetworkStats()

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}

// handleGetNodes returns all registered nodes
func (ts *TrackerService) handleGetNodes(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(map[string]interface{}{
        "nodes": ts.nodes,
        "count": len(ts.nodes),
    })
}

// handleGetNode returns specific node details
func (ts *TrackerService) handleGetNode(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    nodeID := vars["nodeId"]

    node, exists := ts.nodes[nodeID]
    if !exists {
        http.Error(w, "Node not found", http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(node)
}

// handleGetGenesis returns the active genesis block for node bootstrapping
func (ts *TrackerService) handleGetGenesis(w http.ResponseWriter, r *http.Request) {
    w.Header().Set("Content-Type", "application/json")
    w.Write([]byte(activeGenesis))
}

// calculateNetworkStats computes overall network statistics
func (ts *TrackerService) calculateNetworkStats() NetworkStats {
    var stats NetworkStats
    var totalNetspace uint64
    var maxHeight uint64
    heightCounts := make(map[uint64]int)

    stats.TotalNodes = len(ts.nodes)

    for _, node := range ts.nodes {
        // Count online nodes
        if node.Status == "online" && time.Since(node.LastHeartbeat) < 5*time.Minute {
            stats.OnlineNodes++
        } else if node.Status == "syncing" {
            stats.SyncingNodes++
        }

        // Sum total netspace
        totalNetspace += node.TotalPlotSize

        // Track highest height
        if node.ChainHeight > maxHeight {
            maxHeight = node.ChainHeight
        }

        // Count nodes at each height (for consensus calculation)
        heightCounts[node.ChainHeight]++
    }

    stats.TotalNetspace = totalNetspace
    stats.HighestHeight = maxHeight

    // Find consensus height (height with most nodes)
    var consensusHeight uint64
    var maxCount int
    for height, count := range heightCounts {
        if count > maxCount {
            maxCount = count
            consensusHeight = height
        }
    }
    stats.ConsensusHeight = consensusHeight

    // Count forks (heights with multiple nodes)
    for _, count := range heightCounts {
        if count > 1 {
            stats.ForkCount++
        }
    }

    stats.LastUpdated = time.Now().Format(time.RFC3339)

    return stats
}

// extractClientIP extracts the client's IP address from the HTTP request
func extractClientIP(r *http.Request) string {
    // Check X-Forwarded-For header (for proxy/load balancer scenarios)
    if xff := r.Header.Get("X-Forwarded-For"); xff != "" {
        // X-Forwarded-For can contain multiple IPs, take the first one
        if idx := strings.Index(xff, ","); idx != -1 {
            return strings.TrimSpace(xff[:idx])
        }
        return strings.TrimSpace(xff)
    }

    // Check X-Real-IP header (common nginx proxy header)
    if xri := r.Header.Get("X-Real-IP"); xri != "" {
        return strings.TrimSpace(xri)
    }

    // Fall back to RemoteAddr from the connection
    if addr := r.RemoteAddr; addr != "" {
        // RemoteAddr is in format "IP:port", extract just the IP
        if host, _, err := net.SplitHostPort(addr); err == nil {
            return host
        }
        // If splitting fails, return the full address
        return addr
    }

    // Default fallback
    return "unknown"
}

// cleanupOfflineNodes removes nodes that haven't sent heartbeats
func (ts *TrackerService) cleanupOfflineNodes() {
    ticker := time.NewTicker(1 * time.Minute)
    defer ticker.Stop()

    for range ticker.C {
        cutoff := time.Now().Add(-10 * time.Minute)

        for nodeID, node := range ts.nodes {
            if node.LastHeartbeat.Before(cutoff) {
                log.Printf("🧹 Removing offline node %s", nodeID)
                delete(ts.nodes, nodeID)
                delete(ts.registry.nodes, nodeID)
            }
        }
    }
}

// handleDashboard serves the web dashboard
func (ts *TrackerService) handleDashboard(w http.ResponseWriter, r *http.Request) {
    stats := ts.calculateNetworkStats()

    html := fmt.Sprintf(`
<!DOCTYPE html>
<html>
<head>
    <title>Shadowy Network Tracker</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 20px; background: #f5f5f5; }
        .container { max-width: 1600px; margin: 0 auto; }
        .header { background: #333; color: white; padding: 20px; border-radius: 8px; margin-bottom: 20px; }
        .stats { display: grid; grid-template-columns: repeat(auto-fit, minmax(200px, 1fr)); gap: 15px; margin-bottom: 20px; }
        .stat-card { background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        .stat-value { font-size: 2em; font-weight: bold; color: #007acc; }
        .stat-label { color: #666; margin-top: 5px; }
        .nodes-table { background: white; border-radius: 8px; overflow: hidden; box-shadow: 0 2px 4px rgba(0,0,0,0.1); }
        table { width: 100%%; border-collapse: collapse; }
        th, td { padding: 12px; text-align: left; border-bottom: 1px solid #eee; }
        th { background: #f8f9fa; font-weight: bold; }
        .status-online { color: #28a745; }
        .status-offline { color: #dc3545; }
        .status-syncing { color: #ffc107; }
        .ip-column { font-family: monospace; font-size: 0.9em; }
        .ports-column { font-family: monospace; font-size: 0.85em; color: #666; }
        .refresh { margin-bottom: 20px; }
        .refresh button { padding: 10px 20px; background: #007acc; color: white; border: none; border-radius: 4px; cursor: pointer; }
    </style>
    <script>
        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KiB', 'MiB', 'GiB', 'TiB', 'PiB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            const value = parseFloat((bytes / Math.pow(k, i)).toFixed(2));
            return value + ' ' + sizes[i];
        }

        function refreshData() {
            location.reload();
        }

        setInterval(refreshData, 30000); // Auto-refresh every 30 seconds
    </script>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>&#127760; Shadowy Network Tracker</h1>
            <p>Real-time blockchain network monitoring and peer discovery</p>
        </div>

        <div class="refresh">
            <button onclick="refreshData()">&#8634; Refresh</button>
            <span style="margin-left: 10px;">Auto-refresh: 30s</span>
        </div>

        <div class="stats">
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Total Nodes</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Online Nodes</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Syncing Nodes</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="netspace">%d</div>
                <div class="stat-label">Total Netspace</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Highest Height</div>
            </div>
            <div class="stat-card">
                <div class="stat-value">%d</div>
                <div class="stat-label">Consensus Height</div>
            </div>
        </div>

        <div class="nodes-table">
            <table>
                <thead>
                    <tr>
                        <th>Node ID</th>
                        <th>Status</th>
                        <th>Chain Height</th>
                        <th>Plot Size</th>
                        <th>IP Address</th>
                        <th>Chain ID</th>
                        <th>Software</th>
                        <th>Last Seen</th>
                    </tr>
                </thead>
                <tbody>`,
        stats.TotalNodes, stats.OnlineNodes, stats.SyncingNodes,
        stats.TotalNetspace, stats.HighestHeight, stats.ConsensusHeight)

    // Add node rows
    for _, node := range ts.nodes {
        statusClass := "status-offline"
        if node.Status == "online" && time.Since(node.LastHeartbeat) < 5*time.Minute {
            statusClass = "status-online"
        } else if node.Status == "syncing" {
            statusClass = "status-syncing"
        }

        // Format IP addresses and ports
        observedIP := node.ObservedIP
        if observedIP == "" {
            observedIP = "unknown"
        }

        // Format chain ID (show first 8 characters)
        chainID := node.ChainID

        html += fmt.Sprintf(`
                    <tr>
                        <td>%s</td>
                        <td class="%s">%s</td>
                        <td>%d</td>
                        <td id="plot-size-%s">%d</td>
                        <td class="ip-column">%s</td>
                        <td class="ip-column">%s</td>
                        <td>%s</td>
                        <td>%s</td>
                    </tr>`,
            node.NodeID[:8]+"...", statusClass, node.Status,
            node.ChainHeight, node.NodeID, node.TotalPlotSize,
            observedIP, chainID,
            node.SoftwareVersion, node.LastHeartbeat.Format("15:04:05"))
    }

    html += `
                </tbody>
            </table>
        </div>
    </div>

    <script>
        // Format netspace display
        const netspaceElement = document.getElementById('netspace');
        const netspaceValue = parseInt(netspaceElement.textContent);
        netspaceElement.textContent = formatBytes(netspaceValue);

        // Format individual plot sizes
        document.querySelectorAll('[id^="plot-size-"]').forEach(function(el) {
            el.textContent = formatBytes(parseInt(el.textContent));
        });
    </script>
</body>
</html>`

    w.Header().Set("Content-Type", "text/html")
    fmt.Fprint(w, html)
}
