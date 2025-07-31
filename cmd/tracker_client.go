package cmd

import (
    "bytes"
    "crypto/sha256"
    "encoding/json"
    "fmt"
    "log"
    "net"
    "net/http"
    "strconv"
    "time"
)

// TrackerClient handles communication with the tracker service
type TrackerClient struct {
    trackerURL    string
    nodeID        string
    miningAddr    string
    publicKey     string
    httpClient    *http.Client
    lastHeartbeat time.Time
}

// TrackerRegistrationRequest represents a node registration request to tracker
type TrackerRegistrationRequest struct {
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

// TrackerHeartbeatRequest represents a heartbeat update to tracker
type TrackerHeartbeatRequest struct {
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

// TrackerPeer represents a peer from tracker discovery
type TrackerPeer struct {
    NodeID      string    `json:"node_id"`
    Address     string    `json:"address"`
    ClientEth   string    `json:"client_eth"` // External IP observed by tracker
    ChainHeight uint64    `json:"chain_height"`
    ChainHash   string    `json:"chain_hash"`
    ChainID     string    `json:"chain_id"`
    LastSeen    time.Time `json:"last_seen"`
}

// TrackerPeersResponse represents the response from /api/v1/peers
type TrackerPeersResponse struct {
    Peers []TrackerPeer `json:"peers"`
    Count int           `json:"count"`
}

// NewTrackerClient creates a new tracker client
func NewTrackerClient(trackerURL, nodeID, miningAddr, publicKey string) *TrackerClient {
    return &TrackerClient{
        trackerURL: trackerURL,
        nodeID:     nodeID,
        miningAddr: miningAddr,
        publicKey:  publicKey,
        httpClient: &http.Client{
            Timeout: 30 * time.Second,
        },
    }
}

// New package-level variable for build number, set via ldflags

// RegisterWithTracker registers this node with the tracker service
func (tc *TrackerClient) RegisterWithTracker(ce *ConsensusEngine, blockchain *Blockchain, farmingService *FarmingService) error {
    // Get current blockchain state
    stats := blockchain.GetStats()
    height := stats.TipHeight
    tipHash := stats.TipHash

    // Generate chain ID from genesis block hash
    chainID := stats.GenesisHash
    if chainID == "" {
        // Fallback: try to get genesis block directly
        if genesisBlock, err := blockchain.GetBlockByHeight(0); err == nil {
            chainID = genesisBlock.Hash()
        } else {
            chainID = "unknown"
        }
    }

    // Get latest block time
    var lastBlockTime time.Time
    if height > 0 {
        if tip, err := blockchain.GetBlock(tipHash); err == nil {
            lastBlockTime = tip.Header.Timestamp
        }
    }

    // Get farming statistics for plot information
    var totalPlotSize uint64
    var plotCount int
    if farmingService != nil && farmingService.IsRunning() {
        farmingStats := farmingService.GetStats()
        plotCount = farmingStats.PlotFilesIndexed
        // Calculate plot size: each key is approximately 4938 bytes (5.17GB / 1048576 keys)
        totalPlotSize = uint64(farmingStats.TotalKeys) * 4938
    }
    // Get actual ports from consensus engine
    var p2pPort, httpPort int = 8888, 8080 // defaults
    if ce != nil {
        // Extract P2P port from consensus engine listen address
        if ce.listenAddr != "" {
            if _, portStr, err := net.SplitHostPort(ce.listenAddr); err == nil {
                if port, err := strconv.Atoi(portStr); err == nil {
                    p2pPort = port
                }
            }
        }
        // Use HTTP port from consensus engine
        if ce.httpPort > 0 {
            httpPort = ce.httpPort
        }
    }

    // Create registration request
    req := TrackerRegistrationRequest{
        NodeID:          tc.nodeID,
        MiningAddr:      tc.miningAddr,
        PublicKey:       tc.publicKey,
        ExternalIP:      getExternalIP(),
        P2PPort:         p2pPort,
        HTTPPort:        httpPort,
        ChainHeight:     height,
        ChainHash:       tipHash,
        ChainID:         chainID,
        LastBlockTime:   lastBlockTime.Format(time.RFC3339),
        SoftwareVersion: GetVersionString(),
        OSVersion:       getOSVersion(),
        Architecture:    getArchitecture(),
        TotalPlotSize:   totalPlotSize,
        PlotCount:       plotCount,
        Timestamp:       time.Now().Format(time.RFC3339),
        Signature:       "",
    }

    // Generate signature
    req.Signature = tc.generateSimpleSignature(req)

    // Send registration request
    jsonData, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("failed to marshal registration request: %w", err)
    }

    resp, err := tc.httpClient.Post(
        tc.trackerURL+"/api/v1/register",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return fmt.Errorf("failed to send registration request: %w", err)
    }
    defer resp.Body.Close()
    var back []byte
    resp.Body.Read(back)

    if resp.StatusCode != http.StatusOK {
        if resp.StatusCode == 400 {
            return fmt.Errorf("tracker does not support this chain")
        }
        return fmt.Errorf("registration failed with status: %d - %s", resp.StatusCode, string(back))

    }

    log.Printf("✅ Successfully registered with tracker service")
    log.Printf(string(back))
    return nil
}

// SendHeartbeat sends a heartbeat update to the tracker
func (tc *TrackerClient) SendHeartbeat(blockchain *Blockchain, farmingService *FarmingService, status string) error {
    stats := blockchain.GetStats()
    height := stats.TipHeight
    tipHash := stats.TipHash

    // Get latest block time
    var lastBlockTime time.Time
    if height > 0 {
        if tip, err := blockchain.GetBlock(tipHash); err == nil {
            lastBlockTime = tip.Header.Timestamp
        }
    }

    // Get farming statistics for plot information
    var totalPlotSize uint64
    var plotCount int
    if farmingService != nil {
        isRunning := farmingService.IsRunning()
        farmingStats := farmingService.GetStats()
        if isRunning {
            plotCount = farmingStats.PlotFilesIndexed
            // Calculate plot size: each key is approximately 4938 bytes (5.17GB / 1048576 keys)
            totalPlotSize = uint64(farmingStats.TotalKeys) * 4938
            log.Printf("🔍 [HEARTBEAT] Farming service ready - TotalKeys=%d, PlotFilesIndexed=%d, CalculatedSize=%d",
                farmingStats.TotalKeys, farmingStats.PlotFilesIndexed, totalPlotSize)
        } else {
            log.Printf("⚠️ [HEARTBEAT] Farming service exists but not running - TotalKeys=%d, PlotFilesIndexed=%d",
                farmingStats.TotalKeys, farmingStats.PlotFilesIndexed)
        }
    } else {
        log.Printf("⚠️ [HEARTBEAT] Farming service is nil during heartbeat")
    }

    req := TrackerHeartbeatRequest{
        NodeID:        tc.nodeID,
        ChainHeight:   height,
        ChainHash:     tipHash,
        LastBlockTime: lastBlockTime.Format(time.RFC3339),
        Status:        status,
        TotalPlotSize: totalPlotSize,
        PlotCount:     plotCount,
        Timestamp:     time.Now().Format(time.RFC3339),
        Signature:     "",
    }

    // Generate signature
    req.Signature = tc.generateSimpleHeartbeatSignature(req)

    jsonData, err := json.Marshal(req)
    if err != nil {
        return fmt.Errorf("failed to marshal heartbeat request: %w", err)
    }

    resp, err := tc.httpClient.Post(
        tc.trackerURL+"/api/v1/heartbeat",
        "application/json",
        bytes.NewBuffer(jsonData),
    )
    if err != nil {
        return fmt.Errorf("failed to send heartbeat: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return fmt.Errorf("heartbeat failed with status: %d", resp.StatusCode)
    }

    tc.lastHeartbeat = time.Now()
    return nil
}

// DiscoverPeers gets list of peers from tracker for a specific chain ID
func (tc *TrackerClient) DiscoverPeers(chainID string) ([]TrackerPeer, error) {
    // Build URL with chain_id parameter
    url := tc.trackerURL + "/api/v1/peers"
    if chainID != "" {
        url += "?chain_id=" + chainID
    }
    log.Printf("/api/v1/peers?chain_id=%s", chainID)
    resp, err := tc.httpClient.Get(url)
    if err != nil {
        return nil, fmt.Errorf("failed to get peers from tracker: %w", err)
    }
    defer resp.Body.Close()

    if resp.StatusCode != http.StatusOK {
        return nil, fmt.Errorf("peer discovery failed with status: %d", resp.StatusCode)
    }

    var peersResp TrackerPeersResponse
    if err := json.NewDecoder(resp.Body).Decode(&peersResp); err != nil {
        return nil, fmt.Errorf("failed to decode peers response: %w", err)
    }
    if len(peersResp.Peers) == 0 {
        log.Println("No peers found?")
    }
    log.Println(peersResp.Peers)
    return peersResp.Peers, nil
}

// generateSimpleSignature creates a simple signature for development
func (tc *TrackerClient) generateSimpleSignature(req TrackerRegistrationRequest) string {
    // For development, create a simple signature
    message := fmt.Sprintf("%s|%s|%s|%d|%s|%s",
        req.NodeID, req.MiningAddr, req.ExternalIP,
        req.ChainHeight, req.Timestamp, req.SoftwareVersion)

    // Simple hash-based signature for development
    hash := sha256.Sum256([]byte(message))
    return fmt.Sprintf("%x", hash[:16])
}

// generateSimpleHeartbeatSignature creates a simple heartbeat signature
func (tc *TrackerClient) generateSimpleHeartbeatSignature(req TrackerHeartbeatRequest) string {
    message := fmt.Sprintf("%s|%d|%s|%s",
        req.NodeID, req.ChainHeight, req.ChainHash, req.Timestamp)

    hash := sha256.Sum256([]byte(message))
    return fmt.Sprintf("%x", hash[:8])
}

// Helper functions for system info
func getExternalIP() string {
    conn, err := net.Dial("udp", "8.8.8.8:80") // Connect to a known external address (e.g., Google DNS)
    if err != nil {
        log.Fatal(err)
    }
    defer conn.Close()

    localAddress := conn.LocalAddr().(*net.UDPAddr)
    return localAddress.IP.String()
}

func getOSVersion() string {
    return "Linux" // Simplified for development
}

func getArchitecture() string {
    return "amd64" // Simplified for development
}
