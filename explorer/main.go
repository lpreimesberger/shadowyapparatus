package main

import (
    "encoding/json"
    "fmt"
    "html/template"
    "log"
    "math/rand"
    "net/http"
    "os"
    "strconv"
    "strings"
    "time"

    "github.com/dgraph-io/badger/v4"
    "github.com/gorilla/mux"
)

// ExplorerServer serves the Shadowy blockchain explorer
type ExplorerServer struct {
    shadowyNodeURL string // URL to connect to local Shadowy node
    database       *Database
    syncService    *SyncService
}

// NewExplorerServer creates a new explorer server
func NewExplorerServer(shadowyNodeURL string, database *Database, syncService *SyncService) *ExplorerServer {
    return &ExplorerServer{
        shadowyNodeURL: shadowyNodeURL,
        database:       database,
        syncService:    syncService,
    }
}

// Start starts the explorer web server
func (es *ExplorerServer) Start() error {
    router := mux.NewRouter()

    // Serve static files
    router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./static/"))))

    // API routes
    api := router.PathPrefix("/api/v1").Subrouter()
    api.HandleFunc("/health", es.handleHealth).Methods("GET")
    api.HandleFunc("/stats", es.handleStats).Methods("GET")
    api.HandleFunc("/blocks", es.handleBlocks).Methods("GET")
    api.HandleFunc("/block/{hash}", es.handleBlockDetails).Methods("GET")
    api.HandleFunc("/wallet/{address}", es.handleWalletAPI).Methods("GET")
    api.HandleFunc("/tokens", es.handleTokensAPI).Methods("GET")
    api.HandleFunc("/token/{tokenId}", es.handleTokenDetailsAPI).Methods("GET")
    api.HandleFunc("/pools", es.handlePoolsAPI).Methods("GET")
    api.HandleFunc("/pool/{poolId}", es.handlePoolDetailsAPI).Methods("GET")
    api.HandleFunc("/storage", es.handleStorageAPI).Methods("GET")
    api.HandleFunc("/wallets", es.handleWalletsAPI).Methods("GET")
    api.HandleFunc("/admin/reset", es.handleReset).Methods("POST")
    api.HandleFunc("/admin/test-token", es.handleTestToken).Methods("POST")
    api.HandleFunc("/admin/test-pool", es.handleTestPool).Methods("POST")
    api.HandleFunc("/admin/debug-db", es.handleDebugDB).Methods("GET")
    api.HandleFunc("/admin/debug-tx/{txHash}", es.handleDebugTransaction).Methods("GET")
    api.HandleFunc("/admin/debug-wallet/{address}", es.handleDebugWallet).Methods("GET")

    // Web routes
    router.HandleFunc("/", es.handleHome).Methods("GET")
    router.HandleFunc("/blocks", es.handleBlocksPage).Methods("GET")
    router.HandleFunc("/block/{hash}", es.handleBlockDetailsPage).Methods("GET")
    router.HandleFunc("/wallet/{address}", es.handleWalletPage).Methods("GET")
    router.HandleFunc("/tokens", es.handleTokensPage).Methods("GET")
    router.HandleFunc("/token/{tokenId}", es.handleTokenDetailsPage).Methods("GET")
    router.HandleFunc("/pools", es.handlePoolsPage).Methods("GET")
    router.HandleFunc("/pool/{poolId}", es.handlePoolDetailsPage).Methods("GET")
    router.HandleFunc("/storage", es.handleStoragePage).Methods("GET")
    router.HandleFunc("/wallets", es.handleWalletsPage).Methods("GET")

    log.Printf("🌐 Shadowy Explorer starting on http://localhost:10001")
    log.Printf("📡 Connecting to Shadowy node at %s", es.shadowyNodeURL)

    return http.ListenAndServe(":10001", router)
}

// Health check endpoint
func (es *ExplorerServer) handleHealth(w http.ResponseWriter, r *http.Request) {
    response := map[string]interface{}{
        "status":    "ok",
        "service":   "shadowy-explorer",
        "timestamp": time.Now().UTC(),
        "node_url":  es.shadowyNodeURL,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Stats endpoint
func (es *ExplorerServer) handleStats(w http.ResponseWriter, r *http.Request) {
    stats, err := es.syncService.GetNetworkStats()
    if err != nil {
        http.Error(w, "Failed to get stats", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(stats)
}

// Blocks API endpoint with pagination
func (es *ExplorerServer) handleBlocks(w http.ResponseWriter, r *http.Request) {
    // Parse pagination parameters
    page := 1
    perPage := 20

    if p := r.URL.Query().Get("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
            page = parsed
        }
    }

    if pp := r.URL.Query().Get("per_page"); pp != "" {
        if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
            perPage = parsed
        }
    }

    blocks, err := es.database.GetBlocks(page, perPage)
    if err != nil {
        http.Error(w, "Failed to get blocks", http.StatusInternalServerError)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(blocks)
}

// Home page handler
func (es *ExplorerServer) handleHome(w http.ResponseWriter, r *http.Request) {
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Blockchain Explorer</title>
    <meta name="description" content="Explore the Shadowy blockchain - a proof-of-storage cryptocurrency with built-in AMM and timelord consensus">
    <meta name="keywords" content="blockchain, cryptocurrency, proof-of-storage, shadowy, explorer, DeFi, AMM">
    <style>
        * {
            margin: 0;
            padding: 0;
            box-sizing: border-box;
        }

        body {
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            color: #ffffff;
            min-height: 100vh;
            display: flex;
            flex-direction: column;
        }

        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 0 20px;
            flex: 1;
            display: flex;
            flex-direction: column;
            justify-content: center;
            align-items: center;
            text-align: center;
        }

        .logo {
            font-size: 4rem;
            font-weight: bold;
            background: linear-gradient(45deg, #64b5f6, #42a5f5, #2196f3);
            -webkit-background-clip: text;
            -webkit-text-fill-color: transparent;
            background-clip: text;
            margin-bottom: 1rem;
        }

        .subtitle {
            font-size: 1.5rem;
            color: #b0bec5;
            margin-bottom: 2rem;
        }

        .description {
            font-size: 1.1rem;
            color: #cfd8dc;
            max-width: 600px;
            margin-bottom: 3rem;
            line-height: 1.6;
        }

        .features {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 2rem;
            margin-bottom: 3rem;
            width: 100%;
        }

        .feature {
            background: rgba(255, 255, 255, 0.05);
            backdrop-filter: blur(10px);
            border: 1px solid rgba(255, 255, 255, 0.1);
            border-radius: 12px;
            padding: 2rem;
            transition: transform 0.3s ease;
        }

        .feature:hover {
            transform: translateY(-5px);
        }

        .feature-icon {
            font-size: 2.5rem;
            margin-bottom: 1rem;
        }

        .feature-title {
            font-size: 1.3rem;
            margin-bottom: 0.5rem;
            color: #64b5f6;
        }

        .feature-desc {
            color: #b0bec5;
            font-size: 0.9rem;
        }

        .status {
            background: rgba(76, 175, 80, 0.2);
            border: 1px solid #4caf50;
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 2rem;
        }

        .footer {
            padding: 2rem 0;
            text-align: center;
            color: #78909c;
            border-top: 1px solid rgba(255, 255, 255, 0.1);
        }

        @media (max-width: 768px) {
            .logo {
                font-size: 2.5rem;
            }

            .subtitle {
                font-size: 1.2rem;
            }

            .features {
                grid-template-columns: 1fr;
            }
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">⚫ SHADOWY</div>
        <div class="subtitle">Blockchain Explorer & Web3 Gateway</div>

        <div class="description">
            Explore the Shadowy blockchain - a next-generation proof-of-storage cryptocurrency
            featuring built-in AMM, timelord consensus, and sustainable mining.
        </div>

        <div class="status">
            🟢 Explorer Online - Connected to Shadowy Network
        </div>

        <div class="features">
            <div class="feature">
                <div class="feature-icon">🏗️</div>
                <div class="feature-title"><a href="/blocks" style="color: #64b5f6; text-decoration: none;">Block Explorer</a></div>
                <div class="feature-desc">Browse blocks, transactions, and network statistics</div>
            </div>

            <div class="feature">
                <div class="feature-icon">🌐</div>
                <div class="feature-title">Web3 API</div>
                <div class="feature-desc">JSON-RPC interface for dApp development</div>
            </div>

            <div class="feature">
                <div class="feature-icon">💧</div>
                <div class="feature-title"><a href="/pools" style="color: #64b5f6; text-decoration: none;">Liquidity Pools</a></div>
                <div class="feature-desc">Built-in AMM with L-address routing</div>
            </div>

            <div class="feature">
                <div class="feature-icon">⚡</div>
                <div class="feature-title">Proof-of-Storage</div>
                <div class="feature-desc">Environmentally sustainable consensus</div>
            </div>

            <div class="feature">
                <div class="feature-icon">🪙</div>
                <div class="feature-title"><a href="/tokens" style="color: #64b5f6; text-decoration: none;">Token System</a></div>
                <div class="feature-desc">Native token creation and management</div>
            </div>

            <div class="feature">
                <div class="feature-icon">💰</div>
                <div class="feature-title"><a href="/wallets" style="color: #64b5f6; text-decoration: none;">Wallet Explorer</a></div>
                <div class="feature-desc">Browse wallets with SHADOW and token balances</div>
            </div>

            <div class="feature">
                <div class="feature-icon">💾</div>
                <div class="feature-title"><a href="/storage" style="color: #64b5f6; text-decoration: none;">Proof of Storage</a></div>
                <div class="feature-desc">Network storage capacity and farming nodes</div>
            </div>

            <div class="feature">
                <div class="feature-icon">⏰</div>
                <div class="feature-title">Timelord</div>
                <div class="feature-desc">VDF-based timing consensus</div>
            </div>
        </div>

        <div style="color: #78909c;">
            <p>🚧 <a href="/blocks" style="color: #64b5f6; text-decoration: underline;">Block Explorer</a> is live! - Building the future of blockchain exploration</p>
        </div>
    </div>

    <div class="footer">
        <p>&copy; 2025 Shadowy Network - Powered by Proof-of-Storage</p>
        <p>Node: {{.NodeURL}} | Explorer Version: 1.0.0</p>
    </div>
</body>
</html>`

    t, err := template.New("home").Parse(tmpl)
    if err != nil {
        http.Error(w, "Template error", http.StatusInternalServerError)
        return
    }

    data := struct {
        NodeURL string
    }{
        NodeURL: es.shadowyNodeURL,
    }

    w.Header().Set("Content-Type", "text/html")
    t.Execute(w, data)
}

// Blocks page handler
func (es *ExplorerServer) handleBlocksPage(w http.ResponseWriter, r *http.Request) {
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Block Explorer - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        body {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
        }
    </style>
</head>
<body class="text-white">
    <div class="container mx-auto px-4 py-8">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-center mb-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">⚫ SHADOWY</a>
            </h1>
            <h2 class="text-2xl text-center text-gray-300">Block Explorer</h2>
        </div>

        <!-- Stats -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-4 mb-8">
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-blue-400" id="blockHeight">-</div>
                <div class="text-sm text-gray-400">Latest Block</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-green-400" id="totalBlocks">-</div>
                <div class="text-sm text-gray-400">Total Blocks</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-yellow-400" id="syncStatus">-</div>
                <div class="text-sm text-gray-400">Sync Status</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-purple-400" id="lastSync">-</div>
                <div class="text-sm text-gray-400">Last Sync</div>
            </div>
        </div>

        <!-- Blocks Table -->
        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg overflow-hidden">
            <div class="px-6 py-4 border-b border-gray-700">
                <h3 class="text-xl font-semibold">Recent Blocks</h3>
            </div>
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead class="bg-gray-700">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Height</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Hash</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Timestamp</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Transactions</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Farmer</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Size</th>
                        </tr>
                    </thead>
                    <tbody id="blocksTable" class="divide-y divide-gray-700">
                        <!-- Blocks will be loaded here -->
                    </tbody>
                </table>
            </div>
        </div>

        <!-- Pagination -->
        <div class="mt-6 flex justify-center">
            <nav class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px" id="pagination">
                <!-- Pagination will be loaded here -->
            </nav>
        </div>
    </div>

    <script>
        let currentPage = 1;
        const perPage = 20;

        // Load stats
        async function loadStats() {
            try {
                const response = await fetch('/api/v1/stats');
                const stats = await response.json();

                document.getElementById('blockHeight').textContent = stats.height || '-';
                document.getElementById('totalBlocks').textContent = stats.total_blocks || '-';
                document.getElementById('syncStatus').textContent = stats.sync_status || '-';

                const lastSync = stats.last_sync ? new Date(stats.last_sync).toLocaleTimeString() : '-';
                document.getElementById('lastSync').textContent = lastSync;
            } catch (error) {
                console.error('Failed to load stats:', error);
            }
        }

        // Load blocks
        async function loadBlocks(page = 1) {
            try {
                const response = await fetch(` + "`" + `/api/v1/blocks?page=${page}&per_page=${perPage}` + "`" + `);
                const data = await response.json();

                const tbody = document.getElementById('blocksTable');
                tbody.innerHTML = '';

                data.blocks.forEach((block, index) => {
                    const row = document.createElement('tr');
                    row.className = index % 2 === 0 ? 'bg-gray-800 bg-opacity-30' : 'bg-gray-700 bg-opacity-30';

                    const timestamp = new Date(block.timestamp).toLocaleString();
                    const shortHash = block.hash.substring(0, 16) + '...';
                    const shortFarmer = block.farmer_address.substring(0, 16) + '...';

                    row.innerHTML = ` + "`" + `
                        <td class="px-6 py-4 whitespace-nowrap text-sm font-medium text-blue-400">${block.height}</td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm font-mono">
                            <a href="/block/${block.hash}" class="text-blue-400 hover:text-blue-300 cursor-pointer" title="Click to view block details">${shortHash}</a>
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${timestamp}</td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${block.tx_count}</td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm font-mono">
                            <a href="/wallet/${block.farmer_address}" class="text-blue-400 hover:text-blue-300 cursor-pointer" title="Click to view wallet details">${shortFarmer}</a>
                        </td>
                        <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${(block.size / 1024).toFixed(1)} KB</td>
                    ` + "`" + `;

                    tbody.appendChild(row);
                });

                // Update pagination
                updatePagination(data.current_page, data.total_pages);

            } catch (error) {
                console.error('Failed to load blocks:', error);
            }
        }

        // Update pagination
        function updatePagination(current, total) {
            const pagination = document.getElementById('pagination');
            pagination.innerHTML = '';

            // Previous button
            const prevBtn = document.createElement('button');
            prevBtn.className = ` + "`" + `relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-600 bg-gray-800 text-sm font-medium text-gray-400 hover:bg-gray-700 ${current === 1 ? 'cursor-not-allowed opacity-50' : ''}` + "`" + `;
            prevBtn.innerHTML = '‹ Previous';
            prevBtn.disabled = current === 1;
            prevBtn.onclick = () => current > 1 && loadPage(current - 1);
            pagination.appendChild(prevBtn);

            // Page numbers
            const startPage = Math.max(1, current - 2);
            const endPage = Math.min(total, current + 2);

            for (let i = startPage; i <= endPage; i++) {
                const pageBtn = document.createElement('button');
                pageBtn.className = ` + "`" + `relative inline-flex items-center px-4 py-2 border border-gray-600 text-sm font-medium ${i === current ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:bg-gray-700'}` + "`" + `;
                pageBtn.textContent = i;
                pageBtn.onclick = () => loadPage(i);
                pagination.appendChild(pageBtn);
            }

            // Next button
            const nextBtn = document.createElement('button');
            nextBtn.className = ` + "`" + `relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-600 bg-gray-800 text-sm font-medium text-gray-400 hover:bg-gray-700 ${current === total ? 'cursor-not-allowed opacity-50' : ''}` + "`" + `;
            nextBtn.innerHTML = 'Next ›';
            nextBtn.disabled = current === total;
            nextBtn.onclick = () => current < total && loadPage(current + 1);
            pagination.appendChild(nextBtn);
        }

        // Load specific page
        function loadPage(page) {
            currentPage = page;
            loadBlocks(page);
        }

        // Initial load
        loadStats();
        loadBlocks();

        // Auto-refresh every 30 seconds
        setInterval(() => {
            loadStats();
            if (currentPage === 1) {
                loadBlocks(1); // Only refresh first page automatically
            }
        }, 30000);
    </script>
</body>
</html>`

    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

// Block details API endpoint
func (es *ExplorerServer) handleBlockDetails(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    blockHash := vars["hash"]
    
    block, err := es.database.GetBlock(blockHash)
    if err != nil {
        http.Error(w, "Block not found", http.StatusNotFound)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(block)
}

// Wallet API endpoint
func (es *ExplorerServer) handleWalletAPI(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    address := vars["address"]
    
    summary, err := es.database.GetWalletSummary(address)
    if err != nil {
        http.Error(w, "Failed to get wallet data", http.StatusInternalServerError)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(summary)
}

// Tokens API endpoint
func (es *ExplorerServer) handleTokensAPI(w http.ResponseWriter, r *http.Request) {
    // Parse pagination and search parameters
    page := 1
    perPage := 20
    search := ""
    
    if p := r.URL.Query().Get("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
            page = parsed
        }
    }
    
    if pp := r.URL.Query().Get("per_page"); pp != "" {
        if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
            perPage = parsed
        }
    }
    
    if s := r.URL.Query().Get("search"); s != "" {
        search = s
    }
    
    tokens, err := es.database.GetTokens(page, perPage, search)
    if err != nil {
        log.Printf("❌ API: Failed to get tokens: %v", err)
        http.Error(w, "Failed to get tokens", http.StatusInternalServerError)
        return
    }
    
    log.Printf("📊 API: Returning %d tokens (page %d, search='%s')", len(tokens.Tokens), page, search)
    for i, token := range tokens.Tokens {
        log.Printf("🪙 Token %d: %s (%s) - ID: %.8s", i, token.Name, token.Ticker, token.TokenID)
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(tokens)
}

// Token details API endpoint
func (es *ExplorerServer) handleTokenDetailsAPI(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    tokenID := vars["tokenId"]
    
    tokenDetails, err := es.database.GetTokenDetails(tokenID)
    if err != nil {
        http.Error(w, "Token not found", http.StatusNotFound)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(tokenDetails)
}

// Pool API endpoints
func (es *ExplorerServer) handlePoolsAPI(w http.ResponseWriter, r *http.Request) {
    page := 1
    if p := r.URL.Query().Get("page"); p != "" {
        if parsedPage, err := strconv.Atoi(p); err == nil && parsedPage > 0 {
            page = parsedPage
        }
    }
    
    perPage := 20
    if pp := r.URL.Query().Get("per_page"); pp != "" {
        if parsedPerPage, err := strconv.Atoi(pp); err == nil && parsedPerPage > 0 && parsedPerPage <= 100 {
            perPage = parsedPerPage
        }
    }
    
    search := r.URL.Query().Get("search")
    
    log.Printf("📊 API: GetPools called - page=%d, perPage=%d, search='%s'", page, perPage, search)
    
    pools, err := es.database.GetPools(page, perPage, search)
    if err != nil {
        log.Printf("❌ API: Failed to get pools: %v", err)
        http.Error(w, "Failed to get pools", http.StatusInternalServerError)
        return
    }
    
    log.Printf("📊 API: Returning %d pools (page %d, search='%s')", len(pools.Pools), page, search)
    for i, pool := range pools.Pools {
        log.Printf("💧 Pool %d: %s/%s - ID: %.8s", i, pool.TokenASymbol, pool.TokenBSymbol, pool.PoolID)
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(pools)
}

func (es *ExplorerServer) handlePoolDetailsAPI(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    poolID := vars["poolId"]
    
    poolDetails, err := es.database.GetPoolDetails(poolID)
    if err != nil {
        http.Error(w, "Pool not found", http.StatusNotFound)
        return
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(poolDetails)
}

// Storage/farming network API endpoint
func (es *ExplorerServer) handleStorageAPI(w http.ResponseWriter, r *http.Request) {
    // Fetch tracker network statistics and nodes
    trackerURL := "https://playatarot.com/api/v1/stats"
    nodesURL := "https://playatarot.com/api/v1/nodes"
    
    // Create HTTP client with timeout
    client := &http.Client{Timeout: 10 * time.Second}
    
    // Fetch network stats
    statsResp, err := client.Get(trackerURL)
    var trackerStats map[string]interface{}
    if err != nil {
        log.Printf("❌ Failed to fetch tracker stats: %v", err)
    } else {
        defer statsResp.Body.Close()
        if err := json.NewDecoder(statsResp.Body).Decode(&trackerStats); err != nil {
            log.Printf("❌ Failed to parse tracker stats: %v", err)
            trackerStats = nil
        }
    }
    
    // Fetch detailed node information
    nodesResp, err := client.Get(nodesURL)
    var nodesData map[string]interface{}
    var nodesList []map[string]interface{}
    if err != nil {
        log.Printf("❌ Failed to fetch nodes data: %v", err)
    } else {
        defer nodesResp.Body.Close()
        if err := json.NewDecoder(nodesResp.Body).Decode(&nodesData); err != nil {
            log.Printf("❌ Failed to parse nodes data: %v", err)
        } else {
            // Extract nodes from response
            if nodes, ok := nodesData["nodes"].(map[string]interface{}); ok {
                for _, node := range nodes {
                    if nodeData, ok := node.(map[string]interface{}); ok {
                        nodesList = append(nodesList, nodeData)
                    }
                }
            }
        }
    }
    
    // If tracker data failed, return mock data
    if trackerStats == nil {
        log.Printf("📊 Using mock storage data - tracker unavailable")
        mockData := map[string]interface{}{
            "total_nodes": 5,
            "online_nodes": 3,
            "total_netspace": uint64(1024 * 1024 * 1024 * 1024 * 50), // 50TB
            "consensus_height": 1000,
            "avg_success_rate": 75.5,
            "nodes": []map[string]interface{}{
                {
                    "node_id": "mock_node_1_abcdef123456",
                    "plot_size": uint64(1024 * 1024 * 1024 * 1024 * 10), // 10TB
                    "status": "online",
                    "success_rate": 85.2,
                    "blocks_found": 15,
                    "last_block_time": "2025-01-15T10:30:00Z",
                },
                {
                    "node_id": "mock_node_2_fedcba654321", 
                    "plot_size": uint64(1024 * 1024 * 1024 * 1024 * 20), // 20TB
                    "status": "online",
                    "success_rate": 78.9,
                    "blocks_found": 25,
                    "last_block_time": "2025-01-15T09:45:00Z",
                },
                {
                    "node_id": "mock_node_3_987654321abc",
                    "plot_size": uint64(1024 * 1024 * 1024 * 1024 * 20), // 20TB 
                    "status": "syncing",
                    "success_rate": 62.1,
                    "blocks_found": 8,
                    "last_block_time": "2025-01-15T08:20:00Z",
                },
            },
        }
        
        w.Header().Set("Content-Type", "application/json")
        json.NewEncoder(w).Encode(mockData)
        return
    }
    
    // Transform tracker data and calculate farming metrics
    totalNodes := getIntFromInterface(trackerStats["total_nodes"])
    onlineNodes := getIntFromInterface(trackerStats["online_nodes"])
    totalNetspace := getUint64FromInterface(trackerStats["total_netspace_bytes"])
    consensusHeight := getUint64FromInterface(trackerStats["consensus_height"])
    
    // Transform node data for storage view
    var transformedNodes []map[string]interface{}
    var totalSuccessRate float64
    var nodeCount int
    
    for _, nodeData := range nodesList {
        nodeID, _ := nodeData["node_id"].(string)
        if nodeID == "" {
            continue
        }
        
        plotSize := getUint64FromInterface(nodeData["total_plot_size_bytes"])
        status, _ := nodeData["status"].(string)
        lastBlockTime, _ := nodeData["last_block_time"].(string)
        
        // Calculate success rate based on node's block mining performance
        // This is a simplified calculation - in reality this would be based on:
        // blocks_found / expected_blocks_based_on_plot_size_and_time
        successRate := calculateNodeSuccessRate(plotSize, totalNetspace, status)
        totalSuccessRate += successRate
        nodeCount++
        
        transformedNode := map[string]interface{}{
            "node_id":         nodeID,
            "plot_size":       plotSize,
            "status":          status,
            "success_rate":    successRate,
            "blocks_found":    calculateBlocksFound(plotSize, successRate), // Estimated
            "last_block_time": lastBlockTime,
        }
        transformedNodes = append(transformedNodes, transformedNode)
    }
    
    // Calculate average success rate
    avgSuccessRate := 0.0
    if nodeCount > 0 {
        avgSuccessRate = totalSuccessRate / float64(nodeCount)
    }
    
    // Return enhanced storage data
    storageData := map[string]interface{}{
        "total_nodes":      totalNodes,
        "online_nodes":     onlineNodes,
        "total_netspace":   totalNetspace,
        "consensus_height": consensusHeight,
        "avg_success_rate": avgSuccessRate,
        "nodes":           transformedNodes,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(storageData)
}

// Helper functions for data extraction and calculation
func getIntFromInterface(val interface{}) int {
    if val == nil {
        return 0
    }
    switch v := val.(type) {
    case float64:
        return int(v)
    case int:
        return v
    case int64:
        return int(v)
    default:
        return 0
    }
}

func getUint64FromInterface(val interface{}) uint64 {
    if val == nil {
        return 0
    }
    switch v := val.(type) {
    case float64:
        return uint64(v)
    case int:
        return uint64(v)
    case int64:
        return uint64(v)
    case uint64:
        return v
    default:
        return 0
    }
}

func calculateNodeSuccessRate(plotSize, totalNetspace uint64, status string) float64 {
    if totalNetspace == 0 || plotSize == 0 {
        return 0.0
    }
    
    // Base success rate proportional to plot size
    baseRate := float64(plotSize) / float64(totalNetspace) * 100.0
    
    // Adjust based on node status
    switch status {
    case "online":
        return baseRate * (0.8 + (rand.Float64() * 0.4)) // 80-120% of expected
    case "syncing":
        return baseRate * (0.3 + (rand.Float64() * 0.4)) // 30-70% of expected
    default:
        return baseRate * (0.1 + (rand.Float64() * 0.2)) // 10-30% of expected
    }
}

func calculateBlocksFound(plotSize uint64, successRate float64) int {
    // Rough estimation: larger plots with higher success rates find more blocks
    baseBlocks := float64(plotSize) / (1024 * 1024 * 1024 * 1024) // Blocks per TB
    return int(baseBlocks * successRate / 10.0) // Scale down for realism
}

// Reset database endpoint (for development)
func (es *ExplorerServer) handleReset(w http.ResponseWriter, r *http.Request) {
    log.Printf("🔄 Resetting explorer database...")
    
    if err := es.database.ResetDatabase(); err != nil {
        log.Printf("❌ Failed to reset database: %v", err)
        http.Error(w, "Failed to reset database", http.StatusInternalServerError)
        return
    }
    
    log.Printf("✅ Explorer database reset successfully")
    
    response := map[string]interface{}{
        "status":  "success",
        "message": "Database reset successfully",
        "note":    "Background sync will repopulate data",
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Test token creation endpoint (for development/testing)
func (es *ExplorerServer) handleTestToken(w http.ResponseWriter, r *http.Request) {
    log.Printf("🧪 Creating test token...")
    
    // Create a test token
    testToken := &TokenInfo{
        TokenID:       "test123456789abcdef",
        Name:          "Test Token",
        Ticker:        "TEST",
        TotalSupply:   1000000000000, // 1M tokens with 6 decimals
        Decimals:      6,
        Creator:       "test_creator_address_123456789",
        CreationTime:  time.Now(),
        CreationBlock: 12345,
        URI:           "https://example.com/test-token",
        
        // Statistics
        HolderCount:       1,
        TransferCount:     0,
        LastActivity:      time.Now(),
        TotalMelted:       0,
        CirculatingSupply: 1000000000000,
        MeltValue:         5000000, // 5 SHADOW locked
    }
    
    if err := es.database.StoreToken(testToken); err != nil {
        log.Printf("❌ Failed to store test token: %v", err)
        http.Error(w, "Failed to create test token", http.StatusInternalServerError)
        return
    }
    
    // Create test holder
    if err := es.database.UpdateTokenHolder(testToken.TokenID, testToken.Creator, testToken.TotalSupply); err != nil {
        log.Printf("❌ Failed to create test holder: %v", err)
    }
    
    log.Printf("✅ Test token created successfully")
    
    response := map[string]interface{}{
        "status":  "success",
        "message": "Test token created",
        "token":   testToken,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Test pool creation endpoint (for development/testing)
func (es *ExplorerServer) handleTestPool(w http.ResponseWriter, r *http.Request) {
    log.Printf("🧪 Creating test pool...")
    
    // Create a test liquidity pool
    testPool := &LiquidityPool{
        PoolID:         "testpool123456789abcdef",
        TokenA:         "test123456789abcdef", // Reference to test token
        TokenB:         "",                    // SHADOW pair
        TokenASymbol:   "TEST",
        TokenBSymbol:   "SHADOW",
        ReserveA:       50000000000, // 50K TEST tokens
        ReserveB:       10000000,    // 10 SHADOW
        TotalLiquidity: 22360679,    // sqrt(50000000000 * 10000000)
        Creator:        "test_pool_creator_123456789",
        CreationTime:   time.Now(),
        CreationBlock:  12350,
        
        // Statistics
        TradeCount:   3,
        VolumeA:      5000000000, // 5K TEST traded
        VolumeB:      1000000,    // 1 SHADOW traded
        LastActivity: time.Now().Add(-time.Hour * 2), // 2 hours ago
        APR:          12.5,        // 12.5% APR
        TVL:          10050000,    // 10.05 SHADOW TVL
    }
    
    if err := es.database.StorePool(testPool); err != nil {
        log.Printf("❌ Failed to store test pool: %v", err)
        http.Error(w, "Failed to create test pool", http.StatusInternalServerError)
        return
    }
    
    // Create test pool transactions
    testTx1 := &PoolTransaction{
        TxHash:      "testpooltx1234567890abcdef",
        BlockHash:   "testpoolblock1234567890abcdef",
        BlockHeight: 12350,
        Timestamp:   testPool.CreationTime,
        Type:        "create",
        AmountA:     testPool.ReserveA,
        AmountB:     testPool.ReserveB,
        Address:     testPool.Creator,
        LPTokens:    testPool.TotalLiquidity,
    }
    
    testTx2 := &PoolTransaction{
        TxHash:      "testpooltx2234567890abcdef",
        BlockHash:   "testpoolblock2234567890abcdef",
        BlockHeight: 12360,
        Timestamp:   time.Now().Add(-time.Hour),
        Type:        "swap",
        AmountA:     1000000000, // 1K TEST
        AmountB:     200000,     // 0.2 SHADOW
        Address:     "test_trader_123456789",
        LPTokens:    0,
    }
    
    if err := es.database.StorePoolTransaction(testPool.PoolID, testTx1); err != nil {
        log.Printf("❌ Failed to store test pool transaction 1: %v", err)
    }
    
    if err := es.database.StorePoolTransaction(testPool.PoolID, testTx2); err != nil {
        log.Printf("❌ Failed to store test pool transaction 2: %v", err)
    }
    
    log.Printf("✅ Test pool created successfully")
    
    response := map[string]interface{}{
        "status":  "success",
        "message": "Test pool created",
        "pool":    testPool,
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Debug database keys endpoint
func (es *ExplorerServer) handleDebugDB(w http.ResponseWriter, r *http.Request) {
    log.Printf("🔍 Debugging database keys...")
    
    var keys []string
    err := es.database.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false // We only want keys
        it := txn.NewIterator(opts)
        defer it.Close()
        
        for it.Rewind(); it.Valid(); it.Next() {
            key := string(it.Item().Key())
            keys = append(keys, key)
        }
        return nil
    })
    
    if err != nil {
        http.Error(w, "Failed to debug database", http.StatusInternalServerError)
        return
    }
    
    // Filter and categorize keys
    var tokenKeys, tokenTimeKeys, tokenTickerKeys, tokenNameKeys []string
    var txKeys, addrTxKeys, blockKeys []string
    for _, key := range keys {
        if strings.HasPrefix(key, "token:") {
            tokenKeys = append(tokenKeys, key)
        } else if strings.HasPrefix(key, "token_time:") {
            tokenTimeKeys = append(tokenTimeKeys, key)
        } else if strings.HasPrefix(key, "token_ticker:") {
            tokenTickerKeys = append(tokenTickerKeys, key)
        } else if strings.HasPrefix(key, "token_name:") {
            tokenNameKeys = append(tokenNameKeys, key)
        } else if strings.HasPrefix(key, "tx:") {
            txKeys = append(txKeys, key)
        } else if strings.HasPrefix(key, "addr_tx:") {
            addrTxKeys = append(addrTxKeys, key)
        } else if strings.HasPrefix(key, "block:") || strings.HasPrefix(key, "height:") {
            blockKeys = append(blockKeys, key)
        }
    }
    
    response := map[string]interface{}{
        "total_keys":        len(keys),
        "token_keys":        tokenKeys,
        "token_time_keys":   tokenTimeKeys,
        "token_ticker_keys": tokenTickerKeys,
        "token_name_keys":   tokenNameKeys,
        "tx_keys":           txKeys[:min(10, len(txKeys))],
        "addr_tx_keys":      addrTxKeys[:min(10, len(addrTxKeys))],
        "block_keys":        blockKeys[:min(10, len(blockKeys))],
        "tx_keys_count":     len(txKeys),
        "addr_tx_keys_count": len(addrTxKeys),
        "block_keys_count":  len(blockKeys),
        "sample_keys":       keys[:min(10, len(keys))],
    }
    
    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Wallets API endpoint
func (es *ExplorerServer) handleWalletsAPI(w http.ResponseWriter, r *http.Request) {
    // Parse pagination parameters
    page := 1
    perPage := 20

    if p := r.URL.Query().Get("page"); p != "" {
        if parsed, err := strconv.Atoi(p); err == nil && parsed > 0 {
            page = parsed
        }
    }

    if pp := r.URL.Query().Get("per_page"); pp != "" {
        if parsed, err := strconv.Atoi(pp); err == nil && parsed > 0 && parsed <= 100 {
            perPage = parsed
        }
    }

    offset := (page - 1) * perPage

    // Get wallets from database
    wallets, totalWallets, err := es.database.GetAllWallets(perPage, offset)
    if err != nil {
        log.Printf("❌ Failed to get wallets: %v", err)
        http.Error(w, "Failed to get wallets", http.StatusInternalServerError)
        return
    }

    totalPages := int((totalWallets + int64(perPage) - 1) / int64(perPage))

    response := PaginatedWallets{
        Wallets:      wallets,
        CurrentPage:  page,
        TotalPages:   totalPages,
        TotalWallets: totalWallets,
        PerPage:      perPage,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// Debug transaction endpoint
func (es *ExplorerServer) handleDebugTransaction(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    txHash := vars["txHash"]

    log.Printf("🔍 Debugging transaction: %s", txHash)

    var txData map[string]interface{}
    err := es.database.db.View(func(txn *badger.Txn) error {
        // Check for tx key
        txKey := fmt.Sprintf("tx:%s", txHash)
        item, err := txn.Get([]byte(txKey))
        if err != nil {
            log.Printf("❌ Failed to find tx key %s: %v", txKey, err)
            return err
        }

        return item.Value(func(val []byte) error {
            var walletTx WalletTransaction
            if err := json.Unmarshal(val, &walletTx); err != nil {
                log.Printf("❌ Failed to unmarshal tx data: %v", err)
                return err
            }

            txData = map[string]interface{}{
                "tx_hash": walletTx.TxHash,
                "block_hash": walletTx.BlockHash,
                "block_height": walletTx.BlockHeight,
                "timestamp": walletTx.Timestamp,
                "type": walletTx.Type,
                "amount": walletTx.Amount,
                "fee": walletTx.Fee,
                "from_address": walletTx.FromAddress,
                "to_address": walletTx.ToAddress,
                "token_symbol": walletTx.TokenSymbol,
                "token_amount": walletTx.TokenAmount,
            }

            log.Printf("✅ Found transaction data: %+v", txData)
            return nil
        })
    })

    if err != nil {
        log.Printf("❌ Failed to debug transaction: %v", err)
        http.Error(w, fmt.Sprintf("Failed to find transaction: %v", err), http.StatusNotFound)
        return
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(txData)
}

// Debug wallet endpoint to test transaction retrieval
func (es *ExplorerServer) handleDebugWallet(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    address := vars["address"]

    log.Printf("🔍 Debug Wallet: Testing transaction retrieval for %s", address)

    // Test manual key search first
    var foundKeys []string
    err2 := es.database.db.View(func(txn *badger.Txn) error {
        opts := badger.DefaultIteratorOptions
        opts.PrefetchValues = false
        it := txn.NewIterator(opts)
        defer it.Close()

        targetPrefix := fmt.Sprintf("addr_tx:%s:", address)
        log.Printf("🔍 Manual Search: Looking for keys starting with '%s'", targetPrefix)

        count := 0
        for it.Rewind(); it.Valid(); it.Next() {
            key := string(it.Item().Key())
            if strings.HasPrefix(key, "addr_tx:") {
                count++
                if strings.Contains(key, address) {
                    foundKeys = append(foundKeys, key)
                    if len(foundKeys) <= 5 {
                        log.Printf("🔍 Manual Search: Found matching key: %s", key)
                    }
                }
                if count <= 10 {
                    log.Printf("🔍 Manual Search: Sample addr_tx key: %s", key)
                }
            }
        }
        log.Printf("🔍 Manual Search: Total addr_tx keys scanned: %d, matching address: %d", count, len(foundKeys))
        return nil
    })

    // Test GetWalletTransactions directly
    transactions, err := es.database.GetWalletTransactions(address, 10)
    if err != nil {
        log.Printf("❌ Debug Wallet: Error getting transactions: %v", err)
    }

    log.Printf("🔍 Debug Wallet: Retrieved %d transactions", len(transactions))

    debugInfo := map[string]interface{}{
        "address": address,
        "manual_search_error": nil,
        "manual_keys_found": len(foundKeys),
        "sample_keys": foundKeys[:min(5, len(foundKeys))],
        "transactions_found": len(transactions),
        "transactions": transactions,
        "error": nil,
    }

    if err2 != nil {
        debugInfo["manual_search_error"] = err2.Error()
    }

    if err != nil {
        debugInfo["error"] = err.Error()
    }

    // Also test wallet summary
    summary, summaryErr := es.database.GetWalletSummary(address)
    if summaryErr != nil {
        log.Printf("❌ Debug Wallet: Error getting wallet summary: %v", summaryErr)
        debugInfo["summary_error"] = summaryErr.Error()
    } else {
        debugInfo["summary"] = summary
        log.Printf("🔍 Debug Wallet: Wallet summary - Balance: %d, TxCount: %d, BlocksMined: %d",
            summary.Balance, summary.TransactionCount, summary.BlocksMined)
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(debugInfo)
}

// Wallets page handler
func (es *ExplorerServer) handleWalletsPage(w http.ResponseWriter, r *http.Request) {
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Wallets - Shadowy Explorer</title>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f5f5f5; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        h1 { color: #333; text-align: center; margin-bottom: 30px; }
        .nav { margin-bottom: 20px; }
        .nav a { margin-right: 20px; text-decoration: none; color: #007bff; }
        .nav a:hover { text-decoration: underline; }
        .wallets-table { width: 100%; border-collapse: collapse; margin-top: 20px; }
        .wallets-table th, .wallets-table td { padding: 12px; text-align: left; border-bottom: 1px solid #ddd; }
        .wallets-table th { background-color: #f8f9fa; font-weight: bold; }
        .wallets-table tr:hover { background-color: #f8f9fa; }
        .address { font-family: monospace; font-size: 12px; }
        .balance { text-align: right; font-weight: bold; }
        .token-list { font-size: 11px; color: #666; max-width: 200px; }
        .token-item { margin: 2px 0; padding: 2px 6px; background: #e3f2fd; border-radius: 3px; display: inline-block; margin-right: 4px; }
        .loading { text-align: center; padding: 40px; color: #666; }
        .error { color: #dc3545; text-align: center; padding: 20px; }
        .pagination { text-align: center; margin-top: 20px; }
        .pagination a { margin: 0 5px; padding: 8px 12px; text-decoration: none; border: 1px solid #ddd; border-radius: 4px; color: #007bff; }
        .pagination a.current { background: #007bff; color: white; }
        .stats { margin-bottom: 20px; padding: 15px; background: #e8f4fd; border-radius: 5px; }
    </style>
</head>
<body>
    <div class="container">
        <h1>💰 Shadowy Wallets</h1>

        <div class="nav">
            <a href="/">🏠 Home</a>
            <a href="/blocks">📦 Blocks</a>
            <a href="/tokens">🪙 Tokens</a>
            <a href="/pools">🏊 Pools</a>
            <a href="/wallets">💰 Wallets</a>
        </div>

        <div class="stats">
            <p><strong>Total Wallets:</strong> <span id="totalWallets">Loading...</span></p>
        </div>

        <div id="walletsContent" class="loading">Loading wallets...</div>

        <div id="pagination" class="pagination"></div>
    </div>

    <script>
        let currentPage = 1;
        const perPage = 20;

        function formatBalance(balance) {
            return (balance / 100000000).toFixed(8) + ' SHADOW';
        }

        function formatTokenBalance(tokenBalance) {
            const decimals = tokenBalance.decimals || 0;
            const divisor = Math.pow(10, decimals);
            return (tokenBalance.balance / divisor).toFixed(decimals) + ' ' + tokenBalance.token_ticker;
        }

        function formatAddress(address) {
            return address.substring(0, 8) + '...' + address.substring(address.length - 8);
        }

        function loadWallets(page = 1) {
            document.getElementById('walletsContent').innerHTML = '<div class="loading">Loading wallets...</div>';

            fetch('/api/v1/wallets?page=' + page + '&per_page=' + perPage)
                .then(response => response.json())
                .then(data => {
                    displayWallets(data);
                    updatePagination(data);
                    document.getElementById('totalWallets').textContent = data.total_wallets;
                })
                .catch(error => {
                    document.getElementById('walletsContent').innerHTML = '<div class="error">Failed to load wallets: ' + error + '</div>';
                });
        }

        function displayWallets(data) {
            const container = document.getElementById('walletsContent');

            if (data.wallets.length === 0) {
                container.innerHTML = '<p style="text-align: center; color: #666;">No wallets found.</p>';
                return;
            }

            let html = '<table class="wallets-table">';
            html += '<thead><tr>';
            html += '<th>Address</th>';
            html += '<th>SHADOW Balance</th>';
            html += '<th>Transactions</th>';
            html += '<th>Blocks Mined</th>';
            html += '<th>Token Balances</th>';
            html += '<th>Last Activity</th>';
            html += '</tr></thead>';
            html += '<tbody>';

            data.wallets.forEach(wallet => {
                html += '<tr>';
                html += '<td><a href="/wallet/' + wallet.address + '" class="address">' + formatAddress(wallet.address) + '</a></td>';
                html += '<td class="balance">' + formatBalance(wallet.balance) + '</td>';
                html += '<td style="text-align: center;">' + wallet.transaction_count + '</td>';
                html += '<td style="text-align: center;">' + wallet.blocks_mined + '</td>';
                html += '<td class="token-list">';

                if (wallet.token_balances && wallet.token_balances.length > 0) {
                    wallet.token_balances.forEach(token => {
                        html += '<div class="token-item">' + formatTokenBalance(token) + '</div>';
                    });
                } else {
                    html += '<span style="color: #999;">None</span>';
                }

                html += '</td>';
                html += '<td>' + (wallet.last_activity === '0001-01-01T00:00:00Z' ? 'Never' : new Date(wallet.last_activity).toLocaleString()) + '</td>';
                html += '</tr>';
            });

            html += '</tbody></table>';
            container.innerHTML = html;
        }

        function updatePagination(data) {
            const container = document.getElementById('pagination');
            let html = '';

            if (data.current_page > 1) {
                html += '<a href="#" onclick="loadWallets(' + (data.current_page - 1) + ')">← Previous</a>';
            }

            const maxButtons = 5;
            const startPage = Math.max(1, data.current_page - Math.floor(maxButtons / 2));
            const endPage = Math.min(data.total_pages, startPage + maxButtons - 1);

            for (let i = startPage; i <= endPage; i++) {
                const className = i === data.current_page ? 'current' : '';
                html += '<a href="#" class="' + className + '" onclick="loadWallets(' + i + ')">' + i + '</a>';
            }

            if (data.current_page < data.total_pages) {
                html += '<a href="#" onclick="loadWallets(' + (data.current_page + 1) + ')">Next →</a>';
            }

            container.innerHTML = html;
        }

        // Load wallets on page load
        loadWallets();
    </script>
</body>
</html>`

    w.Header().Set("Content-Type", "text/html")
    fmt.Fprint(w, tmpl)
}

func min(a, b int) int {
    if a < b {
        return a
    }
    return b
}

// Block details page handler
func (es *ExplorerServer) handleBlockDetailsPage(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    blockHash := vars["hash"]
    
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Block Details - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        body {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
        }
        .json-container {
            background-color: #1f2937;
            border-radius: 8px;
            padding: 1rem;
            overflow-x: auto;
        }
    </style>
</head>
<body class="text-white">
    <div class="container mx-auto px-4 py-8">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-center mb-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">⚫ SHADOWY</a>
            </h1>
            <h2 class="text-2xl text-center text-gray-300">Block Details</h2>
            <div class="text-center mt-4">
                <a href="/blocks" class="text-blue-400 hover:text-blue-300">← Back to Block Explorer</a>
            </div>
        </div>

        <!-- Block Details -->
        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6" id="blockDetails">
            <div class="text-center text-gray-400">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-400 mx-auto"></div>
                <p class="mt-2">Loading block details...</p>
            </div>
        </div>
    </div>

    <script>
        const blockHash = '` + blockHash + `';
        
        async function loadBlockDetails() {
            try {
                const response = await fetch('/api/v1/block/' + blockHash);
                if (!response.ok) {
                    throw new Error('Block not found');
                }
                const block = await response.json();
                
                const container = document.getElementById('blockDetails');
                container.innerHTML = ` + "`" + `
                    <h3 class="text-2xl font-bold mb-6 text-blue-400">Block ${block.header.height}</h3>
                    
                    <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                        <!-- Block Header -->
                        <div class="space-y-4">
                            <h4 class="text-xl font-semibold text-gray-300">Header</h4>
                            <div class="space-y-2 text-sm">
                                <div><span class="text-gray-400">Height:</span> <span class="text-white font-mono">${block.header.height}</span></div>
                                <div><span class="text-gray-400">Hash:</span> <span class="text-white font-mono break-all">${blockHash}</span></div>
                                <div><span class="text-gray-400">Previous Hash:</span> <span class="text-white font-mono break-all">${block.header.previous_hash}</span></div>
                                <div><span class="text-gray-400">Timestamp:</span> <span class="text-white">${new Date(block.header.timestamp).toLocaleString()}</span></div>
                                <div><span class="text-gray-400">Farmer:</span> 
                                    <a href="/wallet/${block.header.farmer_address}" class="text-blue-400 hover:text-blue-300 font-mono break-all">${block.header.farmer_address}</a>
                                </div>
                                <div><span class="text-gray-400">Merkle Root:</span> <span class="text-white font-mono break-all">${block.header.merkle_root}</span></div>
                                <div><span class="text-gray-400">Plot ID:</span> <span class="text-white font-mono">${block.header.plot_id}</span></div>
                                <div><span class="text-gray-400">Challenge:</span> <span class="text-white font-mono break-all">${block.header.challenge}</span></div>
                                <div><span class="text-gray-400">Proof:</span> <span class="text-white font-mono break-all">${block.header.proof}</span></div>
                            </div>
                        </div>
                        
                        <!-- Block Body -->
                        <div class="space-y-4">
                            <h4 class="text-xl font-semibold text-gray-300">Body</h4>
                            <div class="space-y-2 text-sm">
                                <div><span class="text-gray-400">Transaction Count:</span> <span class="text-white">${block.body.tx_count}</span></div>
                                <div><span class="text-gray-400">Transactions Hash:</span> <span class="text-white font-mono break-all">${block.body.transactions_hash}</span></div>
                            </div>
                            
                            ${block.body.transactions && block.body.transactions.length > 0 ? 
                                ` + "`" + `<div class="mt-4">
                                    <h5 class="text-lg font-semibold text-gray-300 mb-2">Transactions</h5>
                                    <div class="space-y-2">
                                        ${block.body.transactions.map((signedTx, index) => {
                                            let tx;
                                            try {
                                                tx = JSON.parse(signedTx.transaction);
                                            } catch (e) {
                                                return ` + "`" + `<div class="bg-gray-700 p-3 rounded">
                                                    <div class="text-xs text-red-400">Transaction ${index + 1}: Invalid JSON</div>
                                                </div>` + "`" + `;
                                            }
                                            
                                            return ` + "`" + `<div class="bg-gray-700 p-3 rounded">
                                                <div class="text-xs text-gray-400 mb-2"><strong>Transaction ${index + 1}</strong></div>
                                                <div class="text-xs text-gray-400">Hash: <span class="text-white font-mono">${signedTx.tx_hash || 'N/A'}</span></div>
                                                ${tx.outputs && tx.outputs.length > 0 ? 
                                                    ` + "`" + `<div class="text-xs text-gray-400 mt-2">Outputs:</div>
                                                    <div class="ml-4 space-y-1">
                                                        ${tx.outputs.map((output, outputIndex) => 
                                                            ` + "`" + `<div class="text-xs">
                                                                <span class="text-gray-400">To:</span> <span class="text-white font-mono">${output.address}</span><br>
                                                                <span class="text-gray-400">Value:</span> <span class="text-white">${(output.value / 100000000).toFixed(8)} SHADOW</span>
                                                                ${output.address && output.address.startsWith('L') ? '<span class="text-green-400 ml-2">[L-address]</span>' : ''}
                                                            </div>` + "`" + `
                                                        ).join('')}
                                                    </div>` + "`" + ` : 
                                                    '<div class="text-xs text-gray-400">No outputs</div>'
                                                }
                                                ${tx.token_ops && tx.token_ops.length > 0 ? 
                                                    ` + "`" + `<div class="text-xs text-gray-400 mt-2">Token Operations:</div>
                                                    <div class="ml-4">
                                                        ${tx.token_ops.map(op => 
                                                            ` + "`" + `<div class="text-xs">
                                                                <span class="text-blue-400">${op.type || 'Unknown'} operation</span>
                                                            </div>` + "`" + `
                                                        ).join('')}
                                                    </div>` + "`" + ` : 
                                                    ''
                                                }
                                            </div>` + "`" + `;
                                        }).join('')}
                                    </div>
                                </div>` + "`" + ` : 
                                '<div class="text-gray-400 text-sm">No transactions in this block</div>'
                            }
                        </div>
                    </div>
                    
                    <div class="mt-8">
                        <h4 class="text-xl font-semibold text-gray-300 mb-4">Raw Block Data</h4>
                        <div class="json-container">
                            <pre class="text-xs text-gray-300 whitespace-pre-wrap">${JSON.stringify(block, null, 2)}</pre>
                        </div>
                    </div>
                ` + "`" + `;
                
            } catch (error) {
                const container = document.getElementById('blockDetails');
                container.innerHTML = ` + "`" + `
                    <div class="text-center text-red-400">
                        <p class="text-xl">❌ Block not found</p>
                        <p class="text-gray-400 mt-2">Hash: ${blockHash}</p>
                        <a href="/blocks" class="text-blue-400 hover:text-blue-300 mt-4 inline-block">← Back to Block Explorer</a>
                    </div>
                ` + "`" + `;
            }
        }
        
        loadBlockDetails();
    </script>
</body>
</html>`;
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

// Wallet page handler
func (es *ExplorerServer) handleWalletPage(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    address := vars["address"]
    
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Wallet - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        body {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
        }
    </style>
</head>
<body class="text-white">
    <div class="container mx-auto px-4 py-8">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-center mb-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">⚫ SHADOWY</a>
            </h1>
            <h2 class="text-2xl text-center text-gray-300">Wallet Details</h2>
            <div class="text-center mt-4">
                <a href="/blocks" class="text-blue-400 hover:text-blue-300">← Back to Block Explorer</a>
            </div>
        </div>

        <!-- Wallet Details -->
        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6" id="walletDetails">
            <div class="text-center text-gray-400">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-400 mx-auto"></div>
                <p class="mt-2">Loading wallet details...</p>
            </div>
        </div>
    </div>

    <script>
        const address = '` + address + `';
        
        async function loadWalletDetails() {
            try {
                const response = await fetch('/api/v1/wallet/' + address);
                if (!response.ok) {
                    throw new Error('Wallet data not found');
                }
                const wallet = await response.json();
                
                const container = document.getElementById('walletDetails');
                
                // Format balance (convert from satoshi-like units)
                const balanceFormatted = (wallet.balance / 100000000).toFixed(8);
                const firstActivity = wallet.first_activity ? new Date(wallet.first_activity).toLocaleDateString() : 'Never';
                const lastActivity = wallet.last_activity ? new Date(wallet.last_activity).toLocaleDateString() : 'Never';
                
                container.innerHTML = ` + "`" + `
                    <h3 class="text-2xl font-bold mb-6 text-blue-400">Wallet Information</h3>
                    
                    <div class="space-y-6">
                        <!-- Address Display -->
                        <div>
                            <span class="text-gray-400">Address:</span>
                            <div class="text-white font-mono break-all text-sm mt-1 bg-gray-700 p-2 rounded">${address}</div>
                        </div>
                        
                        <!-- Stats Grid -->
                        <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
                            <div class="bg-gray-700 bg-opacity-50 p-4 rounded">
                                <div class="text-2xl font-bold text-blue-400">${balanceFormatted}</div>
                                <div class="text-sm text-gray-400">Balance (SHADOW)</div>
                            </div>
                            <div class="bg-gray-700 bg-opacity-50 p-4 rounded">
                                <div class="text-2xl font-bold text-green-400">${wallet.transaction_count}</div>
                                <div class="text-sm text-gray-400">Transactions</div>
                            </div>
                            <div class="bg-gray-700 bg-opacity-50 p-4 rounded">
                                <div class="text-2xl font-bold text-purple-400">${wallet.blocks_mined}</div>
                                <div class="text-sm text-gray-400">Blocks Mined</div>
                            </div>
                            <div class="bg-gray-700 bg-opacity-50 p-4 rounded">
                                <div class="text-2xl font-bold text-yellow-400">${lastActivity}</div>
                                <div class="text-sm text-gray-400">Last Activity</div>
                            </div>
                        </div>
                        
                        <!-- Activity Summary -->
                        <div class="bg-gray-700 bg-opacity-30 p-4 rounded">
                            <h4 class="text-lg font-semibold text-gray-300 mb-2">Activity Summary</h4>
                            <div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">
                                <div><span class="text-gray-400">First Activity:</span> <span class="text-white">${firstActivity}</span></div>
                                <div><span class="text-gray-400">Last Activity:</span> <span class="text-white">${lastActivity}</span></div>
                            </div>
                        </div>
                        
                        <!-- Recent Transactions -->
                        ${wallet.transactions && wallet.transactions.length > 0 ? 
                            ` + "`" + `<div>
                                <h4 class="text-xl font-semibold text-gray-300 mb-4">Recent Transactions</h4>
                                <div class="space-y-2 max-h-96 overflow-y-auto">
                                    ${wallet.transactions.map(tx => {
                                        const timestamp = new Date(tx.timestamp).toLocaleString();
                                        const amount = (tx.amount / 100000000).toFixed(8);
                                        const isReceived = tx.to_address === address;
                                        const typeColor = tx.type === 'mining_reward' ? 'text-yellow-400' : 
                                                         isReceived ? 'text-green-400' : 'text-red-400';
                                        const typeIcon = tx.type === 'mining_reward' ? '⛏️' : 
                                                        isReceived ? '📥' : '📤';
                                        
                                        return ` + "`" + `<div class="bg-gray-700 bg-opacity-50 p-3 rounded">
                                            <div class="flex justify-between items-start">
                                                <div>
                                                    <div class="flex items-center space-x-2">
                                                        <span>${typeIcon}</span>
                                                        <span class="${typeColor} font-semibold capitalize">${tx.type.replace('_', ' ')}</span>
                                                        <span class="text-gray-400 text-xs">${timestamp}</span>
                                                    </div>
                                                    <div class="text-xs text-gray-400 mt-1">
                                                        <a href="/block/${tx.block_hash}" class="text-blue-400 hover:text-blue-300">Block ${tx.block_height}</a>
                                                    </div>
                                                    ${tx.from_address && tx.from_address !== address ? 
                                                        ` + "`" + `<div class="text-xs text-gray-400">From: 
                                                            <a href="/wallet/${tx.from_address}" class="text-blue-400 hover:text-blue-300 font-mono">${tx.from_address.substring(0, 16)}...</a>
                                                        </div>` + "`" + ` : ''}
                                                    ${tx.to_address && tx.to_address !== address ? 
                                                        ` + "`" + `<div class="text-xs text-gray-400">To: 
                                                            <a href="/wallet/${tx.to_address}" class="text-blue-400 hover:text-blue-300 font-mono">${tx.to_address.substring(0, 16)}...</a>
                                                        </div>` + "`" + ` : ''}
                                                </div>
                                                <div class="text-right">
                                                    <div class="${typeColor} font-bold">${isReceived ? '+' : '-'}${amount} SHADOW</div>
                                                    ${tx.token_symbol ? ` + "`" + `<div class="text-xs text-gray-400">${tx.token_amount} ${tx.token_symbol}</div>` + "`" + ` : ''}
                                                </div>
                                            </div>
                                        </div>` + "`" + `;
                                    }).join('')}
                                </div>
                            </div>` + "`" + ` : 
                            ` + "`" + `<div class="bg-gray-700 bg-opacity-30 p-6 rounded text-center">
                                <div class="text-gray-400">
                                    <div class="text-4xl mb-2">📭</div>
                                    <p class="text-lg">No transactions found</p>
                                    <p class="text-sm">This address has no recorded activity yet.</p>
                                </div>
                            </div>` + "`" + `
                        }
                    </div>
                ` + "`" + `;
                
            } catch (error) {
                const container = document.getElementById('walletDetails');
                container.innerHTML = ` + "`" + `
                    <div class="text-center text-red-400">
                        <p class="text-xl">❌ Wallet data not found</p>
                        <p class="text-gray-400 mt-2">Address: ${address}</p>
                        <p class="text-sm text-gray-400 mt-2">This address may not have any recorded activity yet.</p>
                        <a href="/blocks" class="text-blue-400 hover:text-blue-300 mt-4 inline-block">← Back to Block Explorer</a>
                    </div>
                ` + "`" + `;
            }
        }
        
        // Placeholder function to prevent ReferenceError
        function refreshMarketplace() {
            console.log('Marketplace refresh not implemented');
            // Could reload the page or show a message
            location.reload();
        }
        
        loadWalletDetails();
    </script>
</body>
</html>`;
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

// Tokens page handler
func (es *ExplorerServer) handleTokensPage(w http.ResponseWriter, r *http.Request) {
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Token Explorer - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        body {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
        }
    </style>
</head>
<body class="text-white">
    <div class="container mx-auto px-4 py-8">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-center mb-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">⚫ SHADOWY</a>
            </h1>
            <h2 class="text-2xl text-center text-gray-300">Token Explorer</h2>
            <div class="text-center mt-4">
                <a href="/blocks" class="text-blue-400 hover:text-blue-300">← Back to Block Explorer</a>
            </div>
        </div>

        <!-- Search Bar -->
        <div class="mb-6">
            <div class="max-w-md mx-auto">
                <input type="text" id="searchInput" placeholder="Search tokens by name or ticker..." 
                       class="w-full px-4 py-2 bg-gray-700 text-white rounded-lg border border-gray-600 focus:border-blue-400 focus:outline-none">
            </div>
        </div>

        <!-- Token Stats -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-blue-400" id="totalTokens">-</div>
                <div class="text-sm text-gray-400">Total Tokens</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-green-400" id="activeTokens">-</div>
                <div class="text-sm text-gray-400">Active Tokens</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-purple-400" id="totalValue">-</div>
                <div class="text-sm text-gray-400">Total Value Locked</div>
            </div>
        </div>

        <!-- Tokens Table -->
        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg overflow-hidden">
            <div class="px-6 py-4 border-b border-gray-700">
                <h3 class="text-xl font-semibold">Tokens</h3>
            </div>
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead class="bg-gray-700">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Token</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Supply</th>
                            <th class="px-6 py-3 text-right text-xs font-medium text-gray-300 uppercase tracking-wider">Melt Value</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Holders</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Transfers</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Creator</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Created</th>
                        </tr>
                    </thead>
                    <tbody id="tokensTable" class="divide-y divide-gray-700">
                        <!-- Tokens will be loaded here -->
                    </tbody>
                </table>
            </div>
        </div>

        <!-- Pagination -->
        <div class="mt-6 flex justify-center">
            <nav class="relative z-0 inline-flex rounded-md shadow-sm -space-x-px" id="pagination">
                <!-- Pagination will be loaded here -->
            </nav>
        </div>
    </div>

    <script>
        let currentPage = 1;
        let currentSearch = '';
        const perPage = 20;

        // Load tokens
        async function loadTokens(page = 1, search = '') {
            try {
                let url = ` + "`" + `/api/v1/tokens?page=${page}&per_page=${perPage}` + "`" + `;
                if (search) {
                    url += ` + "`" + `&search=${encodeURIComponent(search)}` + "`" + `;
                }
                
                const response = await fetch(url);
                const data = await response.json();
                
                const tbody = document.getElementById('tokensTable');
                tbody.innerHTML = '';
                
                // Update stats
                document.getElementById('totalTokens').textContent = data.total_tokens || 0;
                document.getElementById('activeTokens').textContent = data.tokens ? data.tokens.length : 0;
                
                if (data.tokens && data.tokens.length > 0) {
                    data.tokens.forEach((token, index) => {
                        const row = document.createElement('tr');
                        row.className = index % 2 === 0 ? 'bg-gray-800 bg-opacity-30' : 'bg-gray-700 bg-opacity-30';
                        
                        const createdDate = new Date(token.creation_time).toLocaleDateString();
                        const supplyFormatted = (token.total_supply / Math.pow(10, token.decimals)).toLocaleString();
                        const shortCreator = token.creator.substring(0, 16) + '...';
                        const shortTokenId = token.token_id.substring(0, 16) + '...';
                        
                        row.innerHTML = ` + "`" + `
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="flex items-center">
                                    <div>
                                        <div class="text-sm font-medium text-white">
                                            <a href="/token/${token.token_id}" class="text-blue-400 hover:text-blue-300">${token.name}</a>
                                        </div>
                                        <div class="text-sm text-gray-400 font-mono">${token.ticker}</div>
                                        <div class="text-xs text-gray-500 font-mono">${shortTokenId}</div>
                                    </div>
                                </div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="text-sm text-white">${supplyFormatted}</div>
                                <div class="text-xs text-gray-400">Circulating: ${(token.circulating_supply / Math.pow(10, token.decimals)).toLocaleString()}</div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-right">
                                <div class="text-sm font-bold text-yellow-400">${(token.melt_value || 0).toFixed(8)} SHADOW</div>
                                <div class="text-xs text-gray-400">Melt Value</div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${token.holder_count}</td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${token.transfer_count}</td>
                            <td class="px-6 py-4 whitespace-nowrap">
                                <a href="/wallet/${token.creator}" class="text-blue-400 hover:text-blue-300 text-sm font-mono">${shortCreator}</a>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-sm text-gray-300">${createdDate}</td>
                        ` + "`" + `;
                        
                        tbody.appendChild(row);
                    });
                } else {
                    tbody.innerHTML = ` + "`" + `
                        <tr>
                            <td colspan="7" class="px-6 py-8 text-center text-gray-400">
                                <div class="text-4xl mb-2">🪙</div>
                                <p class="text-lg">No tokens found</p>
                                <p class="text-sm">No tokens have been created yet${search ? ' matching your search' : ''}.</p>
                            </td>
                        </tr>
                    ` + "`" + `;
                }
                
                // Update pagination
                updatePagination(data.current_page, data.total_pages);
                
            } catch (error) {
                console.error('Failed to load tokens:', error);
                document.getElementById('tokensTable').innerHTML = ` + "`" + `
                    <tr>
                        <td colspan="6" class="px-6 py-8 text-center text-red-400">
                            <p class="text-lg">❌ Failed to load tokens</p>
                        </td>
                    </tr>
                ` + "`" + `;
            }
        }

        // Update pagination
        function updatePagination(current, total) {
            const pagination = document.getElementById('pagination');
            pagination.innerHTML = '';
            
            if (total <= 1) return;
            
            // Previous button
            const prevBtn = document.createElement('button');
            prevBtn.className = ` + "`" + `relative inline-flex items-center px-2 py-2 rounded-l-md border border-gray-600 bg-gray-800 text-sm font-medium text-gray-400 hover:bg-gray-700 ${current === 1 ? 'cursor-not-allowed opacity-50' : ''}` + "`" + `;
            prevBtn.innerHTML = '‹ Previous';
            prevBtn.disabled = current === 1;
            prevBtn.onclick = () => current > 1 && loadPage(current - 1);
            pagination.appendChild(prevBtn);
            
            // Page numbers
            const startPage = Math.max(1, current - 2);
            const endPage = Math.min(total, current + 2);
            
            for (let i = startPage; i <= endPage; i++) {
                const pageBtn = document.createElement('button');
                pageBtn.className = ` + "`" + `relative inline-flex items-center px-4 py-2 border border-gray-600 text-sm font-medium ${i === current ? 'bg-blue-600 text-white' : 'bg-gray-800 text-gray-400 hover:bg-gray-700'}` + "`" + `;
                pageBtn.textContent = i;
                pageBtn.onclick = () => loadPage(i);
                pagination.appendChild(pageBtn);
            }
            
            // Next button
            const nextBtn = document.createElement('button');
            nextBtn.className = ` + "`" + `relative inline-flex items-center px-2 py-2 rounded-r-md border border-gray-600 bg-gray-800 text-sm font-medium text-gray-400 hover:bg-gray-700 ${current === total ? 'cursor-not-allowed opacity-50' : ''}` + "`" + `;
            nextBtn.innerHTML = 'Next ›';
            nextBtn.disabled = current === total;
            nextBtn.onclick = () => current < total && loadPage(current + 1);
            pagination.appendChild(nextBtn);
        }

        // Load specific page
        function loadPage(page) {
            currentPage = page;
            loadTokens(page, currentSearch);
        }

        // Search functionality
        let searchTimeout;
        document.getElementById('searchInput').addEventListener('input', (e) => {
            clearTimeout(searchTimeout);
            searchTimeout = setTimeout(() => {
                currentSearch = e.target.value.trim();
                currentPage = 1;
                loadTokens(1, currentSearch);
            }, 500);
        });

        // Initial load
        loadTokens();
    </script>
</body>
</html>`;
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

// Token details page handler
func (es *ExplorerServer) handleTokenDetailsPage(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    tokenID := vars["tokenId"]
    
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Token Details - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        body {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
        }
    </style>
</head>
<body class="text-white">
    <div class="container mx-auto px-4 py-8">
        <!-- Header -->
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-center mb-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">⚫ SHADOWY</a>
            </h1>
            <h2 class="text-2xl text-center text-gray-300">Token Details</h2>
            <div class="text-center mt-4">
                <a href="/tokens" class="text-blue-400 hover:text-blue-300">← Back to Token Explorer</a>
            </div>
        </div>

        <!-- Token Details -->
        <div id="tokenDetails">
            <div class="text-center text-gray-400">
                <div class="animate-spin rounded-full h-8 w-8 border-b-2 border-blue-400 mx-auto"></div>
                <p class="mt-2">Loading token details...</p>
            </div>
        </div>
    </div>

    <script>
        const tokenId = '` + tokenID + `';
        
        async function loadTokenDetails() {
            try {
                const response = await fetch('/api/v1/token/' + tokenId);
                if (!response.ok) {
                    throw new Error('Token not found');
                }
                const token = await response.json();
                
                const container = document.getElementById('tokenDetails');
                
                const supplyFormatted = (token.total_supply / Math.pow(10, token.decimals)).toLocaleString();
                const circulatingFormatted = (token.circulating_supply / Math.pow(10, token.decimals)).toLocaleString();
                const meltValueFormatted = (token.melt_value / 1000000).toFixed(6);
                const createdDate = new Date(token.creation_time).toLocaleDateString();
                const lastActivityDate = token.last_activity ? new Date(token.last_activity).toLocaleDateString() : 'Never';
                
                container.innerHTML = ` + "`" + `
                    <div class="space-y-6">
                        <!-- Token Header -->
                        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6">
                            <div class="flex items-center justify-between mb-4">
                                <div>
                                    <h3 class="text-3xl font-bold text-blue-400">${token.name}</h3>
                                    <p class="text-xl text-gray-300">${token.ticker}</p>
                                </div>
                                <div class="text-right">
                                    <div class="text-sm text-gray-400">Token ID</div>
                                    <div class="text-sm font-mono text-white break-all">${token.token_id}</div>
                                </div>
                            </div>
                            
                            <div class="grid grid-cols-1 md:grid-cols-4 gap-4">
                                <div class="bg-gray-700 bg-opacity-50 p-3 rounded">
                                    <div class="text-lg font-bold text-green-400">${supplyFormatted}</div>
                                    <div class="text-sm text-gray-400">Total Supply</div>
                                </div>
                                <div class="bg-gray-700 bg-opacity-50 p-3 rounded">
                                    <div class="text-lg font-bold text-blue-400">${circulatingFormatted}</div>
                                    <div class="text-sm text-gray-400">Circulating</div>
                                </div>
                                <div class="bg-gray-700 bg-opacity-50 p-3 rounded">
                                    <div class="text-lg font-bold text-purple-400">${token.holder_count}</div>
                                    <div class="text-sm text-gray-400">Holders</div>
                                </div>
                                <div class="bg-gray-700 bg-opacity-50 p-3 rounded">
                                    <div class="text-lg font-bold text-yellow-400">${meltValueFormatted}</div>
                                    <div class="text-sm text-gray-400">Value Locked (SHADOW)</div>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Token Info -->
                        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6">
                                <h4 class="text-xl font-semibold text-gray-300 mb-4">Token Information</h4>
                                <div class="space-y-3 text-sm">
                                    <div><span class="text-gray-400">Creator:</span> 
                                        <a href="/wallet/${token.creator}" class="text-blue-400 hover:text-blue-300 font-mono">${token.creator}</a>
                                    </div>
                                    <div><span class="text-gray-400">Created:</span> <span class="text-white">${createdDate}</span></div>
                                    <div><span class="text-gray-400">Creation Block:</span> 
                                        <a href="/block/${token.creation_block}" class="text-blue-400 hover:text-blue-300">${token.creation_block}</a>
                                    </div>
                                    <div><span class="text-gray-400">Decimals:</span> <span class="text-white">${token.decimals}</span></div>
                                    <div><span class="text-gray-400">Last Activity:</span> <span class="text-white">${lastActivityDate}</span></div>
                                    <div><span class="text-gray-400">Transfer Count:</span> <span class="text-white">${token.transfer_count}</span></div>
                                    ${token.uri ? ` + "`" + `<div><span class="text-gray-400">URI:</span> <a href="${token.uri}" class="text-blue-400 hover:text-blue-300" target="_blank">${token.uri}</a></div>` + "`" + ` : ''}
                                </div>
                            </div>
                            
                            <!-- Statistics -->
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6">
                                <h4 class="text-xl font-semibold text-gray-300 mb-4">Statistics</h4>
                                <div class="space-y-3 text-sm">
                                    <div><span class="text-gray-400">Market Cap:</span> <span class="text-white">${meltValueFormatted} SHADOW</span></div>
                                    <div><span class="text-gray-400">Total Melted:</span> <span class="text-white">${(token.total_melted / Math.pow(10, token.decimals)).toLocaleString()}</span></div>
                                    <div><span class="text-gray-400">Melt Ratio:</span> <span class="text-white">${((token.total_melted / token.total_supply) * 100).toFixed(2)}%</span></div>
                                    <div><span class="text-gray-400">Avg. per Holder:</span> <span class="text-white">${token.holder_count > 0 ? (token.circulating_supply / Math.pow(10, token.decimals) / token.holder_count).toLocaleString() : '0'}</span></div>
                                </div>
                            </div>
                        </div>
                        
                        <!-- Holders and Transactions -->
                        <div class="grid grid-cols-1 lg:grid-cols-2 gap-6">
                            <!-- Top Holders -->
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6">
                                <h4 class="text-xl font-semibold text-gray-300 mb-4">Top Holders</h4>
                                ${token.holders && token.holders.length > 0 ? 
                                    ` + "`" + `<div class="space-y-2">
                                        ${token.holders.map((holder, index) => {
                                            const percentage = ((holder.balance / token.circulating_supply) * 100).toFixed(2);
                                            const balanceFormatted = (holder.balance / Math.pow(10, token.decimals)).toLocaleString();
                                            return ` + "`" + `<div class="flex justify-between items-center bg-gray-700 bg-opacity-50 p-3 rounded">
                                                <div>
                                                    <div class="text-sm font-medium">
                                                        <a href="/wallet/${holder.address}" class="text-blue-400 hover:text-blue-300 font-mono">${holder.address.substring(0, 16)}...</a>
                                                    </div>
                                                    <div class="text-xs text-gray-400">#${index + 1} holder</div>
                                                </div>
                                                <div class="text-right">
                                                    <div class="text-sm font-bold text-white">${balanceFormatted}</div>
                                                    <div class="text-xs text-gray-400">${percentage}%</div>
                                                </div>
                                            </div>` + "`" + `;
                                        }).join('')}
                                    </div>` + "`" + ` : 
                                    '<div class="text-center text-gray-400"><p>No holders found</p></div>'
                                }
                            </div>
                            
                            <!-- Recent Transactions -->
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6">
                                <h4 class="text-xl font-semibold text-gray-300 mb-4">Recent Transactions</h4>
                                ${token.recent_transactions && token.recent_transactions.length > 0 ? 
                                    ` + "`" + `<div class="space-y-2 max-h-80 overflow-y-auto">
                                        ${token.recent_transactions.map(tx => {
                                            const timestamp = new Date(tx.timestamp).toLocaleString();
                                            const amountFormatted = (tx.amount / Math.pow(10, token.decimals)).toLocaleString();
                                            const typeIcon = tx.type === 'create' ? '🎨' : tx.type === 'transfer' ? '↔️' : '🔥';
                                            const typeColor = tx.type === 'create' ? 'text-green-400' : tx.type === 'transfer' ? 'text-blue-400' : 'text-red-400';
                                            
                                            return ` + "`" + `<div class="bg-gray-700 bg-opacity-50 p-3 rounded">
                                                <div class="flex justify-between items-start">
                                                    <div>
                                                        <div class="flex items-center space-x-2">
                                                            <span>${typeIcon}</span>
                                                            <span class="${typeColor} font-semibold capitalize">${tx.type}</span>
                                                            <span class="text-gray-400 text-xs">${timestamp}</span>
                                                        </div>
                                                        <div class="text-xs text-gray-400 mt-1">
                                                            <a href="/block/${tx.block_hash}" class="text-blue-400 hover:text-blue-300">Block ${tx.block_height}</a>
                                                        </div>
                                                        ${tx.from_address ? ` + "`" + `<div class="text-xs text-gray-400">From: <a href="/wallet/${tx.from_address}" class="text-blue-400 hover:text-blue-300 font-mono">${tx.from_address.substring(0, 16)}...</a></div>` + "`" + ` : ''}
                                                        ${tx.to_address ? ` + "`" + `<div class="text-xs text-gray-400">To: <a href="/wallet/${tx.to_address}" class="text-blue-400 hover:text-blue-300 font-mono">${tx.to_address.substring(0, 16)}...</a></div>` + "`" + ` : ''}
                                                    </div>
                                                    <div class="text-right">
                                                        <div class="${typeColor} font-bold">${amountFormatted}</div>
                                                    </div>
                                                </div>
                                            </div>` + "`" + `;
                                        }).join('')}
                                    </div>` + "`" + ` : 
                                    '<div class="text-center text-gray-400"><p>No transactions found</p></div>'
                                }
                            </div>
                        </div>
                    </div>
                ` + "`" + `;
                
            } catch (error) {
                const container = document.getElementById('tokenDetails');
                container.innerHTML = ` + "`" + `
                    <div class="text-center text-red-400">
                        <p class="text-xl">❌ Token not found</p>
                        <p class="text-gray-400 mt-2">Token ID: ${tokenId}</p>
                        <a href="/tokens" class="text-blue-400 hover:text-blue-300 mt-4 inline-block">← Back to Token Explorer</a>
                    </div>
                ` + "`" + `;
            }
        }
        
        loadTokenDetails();
    </script>
</body>
</html>`;
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

// Pool page handlers  
func (es *ExplorerServer) handlePoolsPage(w http.ResponseWriter, r *http.Request) {
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Liquidity Pools - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        body {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
        }
    </style>
</head>
<body class="text-white">
    <div class="container mx-auto px-4 py-8">
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-center mb-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">⚫ SHADOWY</a>
            </h1>
            <h2 class="text-2xl text-center text-gray-300">Liquidity Pools</h2>
            <div class="text-center mt-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">← Back to Explorer</a>
            </div>
        </div>

        <!-- Pool Stats -->
        <div class="grid grid-cols-1 md:grid-cols-3 gap-4 mb-8">
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-blue-400" id="totalPools">-</div>
                <div class="text-sm text-gray-400">Total Pools</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-green-400" id="totalTVL">-</div>
                <div class="text-sm text-gray-400">Total Value Locked</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                <div class="text-2xl font-bold text-purple-400" id="totalVolume">-</div>
                <div class="text-sm text-gray-400">24h Volume</div>
            </div>
        </div>

        <!-- Pools Table -->
        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg">
            <div class="p-6">
                <h3 class="text-xl font-semibold">Liquidity Pools</h3>
            </div>
            
            <div id="poolsTable" class="border-t border-gray-700">
                <div class="text-center p-8">
                    <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-400 mx-auto mb-4"></div>
                    <p class="text-gray-400">Loading pools...</p>
                </div>
            </div>
        </div>

        <!-- Pagination -->
        <div id="pagination" class="mt-6 flex justify-center space-x-2"></div>
    </div>

    <script>
        let currentPage = 1;
        const perPage = 20;

        async function loadPools(page = 1, search = '') {
            try {
                const response = await fetch(` + "`" + `/api/v1/pools?page=${page}&per_page=${perPage}&search=${encodeURIComponent(search)}` + "`" + `);
                const data = await response.json();
                
                displayPools(data);
                updateStats(data);
                updatePagination(data);
                currentPage = page;
            } catch (error) {
                console.error('Error loading pools:', error);
                document.getElementById('poolsTable').innerHTML = '<div class="text-center p-8 text-red-400">Failed to load pools</div>';
            }
        }

        function displayPools(data) {
            const pools = data.pools || [];
            const tableContainer = document.getElementById('poolsTable');
            
            if (pools.length === 0) {
                tableContainer.innerHTML = '<div class="text-center p-8 text-gray-400">No pools found</div>';
                return;
            }

            let html = ` + "`" + `
                <table class="w-full">
                    <thead>
                        <tr class="border-b border-gray-700">
                            <th class="text-left p-4">Pool</th>
                            <th class="text-left p-4">TVL</th>
                            <th class="text-left p-4">Volume 24h</th>
                            <th class="text-left p-4">APR</th>
                            <th class="text-left p-4">Trades</th>
                        </tr>
                    </thead>
                    <tbody>
            ` + "`" + `;

            pools.forEach(pool => {
                const tvl = (pool.tvl / 1000000).toFixed(2);
                const volume24h = ((pool.volume_a + pool.volume_b) / 1000000).toFixed(2);
                
                html += ` + "`" + `
                    <tr class="border-b border-gray-700 hover:bg-gray-700 hover:bg-opacity-50">
                        <td class="p-4">
                            <a href="/pool/${pool.pool_id}" class="text-blue-400 hover:text-blue-300">
                                <div class="font-semibold">${pool.token_a_symbol}/${pool.token_b_symbol}</div>
                                <div class="text-xs text-gray-400">ID: ${pool.pool_id.substring(0, 8)}...</div>
                            </a>
                        </td>
                        <td class="p-4">
                            <div class="font-mono">${tvl} SHADOW</div>
                        </td>
                        <td class="p-4">
                            <div class="font-mono">${volume24h} SHADOW</div>
                        </td>
                        <td class="p-4">
                            <div class="font-mono ${pool.apr > 0 ? 'text-green-400' : 'text-gray-400'}">${pool.apr.toFixed(1)}%</div>
                        </td>
                        <td class="p-4">
                            <div class="text-gray-300">${pool.trade_count}</div>
                        </td>
                    </tr>
                ` + "`" + `;
            });

            html += '</tbody></table>';
            tableContainer.innerHTML = html;
        }

        function updateStats(data) {
            document.getElementById('totalPools').textContent = data.total_pools || 0;
            
            if (data.pools) {
                const totalTVL = data.pools.reduce((sum, pool) => sum + (pool.tvl || 0), 0);
                const totalVolume = data.pools.reduce((sum, pool) => sum + (pool.volume_a || 0) + (pool.volume_b || 0), 0);
                
                document.getElementById('totalTVL').textContent = (totalTVL / 1000000).toFixed(2) + ' SHADOW';
                document.getElementById('totalVolume').textContent = (totalVolume / 1000000).toFixed(2) + ' SHADOW';
            }
        }

        function updatePagination(data) {
            const pagination = document.getElementById('pagination');
            if (data.total_pages <= 1) {
                pagination.innerHTML = '';
                return;
            }
            
            let html = '';
            if (currentPage > 1) {
                html += ` + "`" + `<button onclick="loadPools(${currentPage - 1})" class="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded">Previous</button>` + "`" + `;
            }
            
            html += ` + "`" + `<span class="px-3 py-1">Page ${currentPage} of ${data.total_pages}</span>` + "`" + `;
            
            if (currentPage < data.total_pages) {
                html += ` + "`" + `<button onclick="loadPools(${currentPage + 1})" class="px-3 py-1 bg-gray-700 hover:bg-gray-600 rounded">Next</button>` + "`" + `;
            }
            
            pagination.innerHTML = html;
        }

        loadPools();
    </script>
</body>
</html>`;
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

func (es *ExplorerServer) handlePoolDetailsPage(w http.ResponseWriter, r *http.Request) {
    vars := mux.Vars(r)
    poolID := vars["poolId"]
    
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Pool Details - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        body {
            background: linear-gradient(135deg, #1a1a2e 0%, #16213e 50%, #0f3460 100%);
            min-height: 100vh;
        }
    </style>
</head>
<body class="text-white">
    <div class="container mx-auto px-4 py-8">
        <div class="mb-8">
            <h1 class="text-4xl font-bold text-center mb-4">
                <a href="/" class="text-blue-400 hover:text-blue-300">⚫ SHADOWY</a>
            </h1>
            <div class="text-center mt-4">
                <a href="/pools" class="text-blue-400 hover:text-blue-300">← Back to Pools</a>
            </div>
        </div>

        <div id="poolDetails" class="text-center">
            <div class="animate-spin rounded-full h-12 w-12 border-b-2 border-blue-400 mx-auto mb-4"></div>
            <p class="text-gray-400">Loading pool details...</p>
        </div>
    </div>

    <script>
        const poolId = '` + poolID + `';
        
        async function loadPoolDetails() {
            try {
                const response = await fetch(` + "`" + `/api/v1/pool/${poolId}` + "`" + `);
                const pool = await response.json();
                
                document.getElementById('poolDetails').innerHTML = ` + "`" + `
                    <div class="max-w-4xl mx-auto">
                        <div class="text-center mb-8">
                            <h2 class="text-3xl font-bold mb-2">${pool.token_a_symbol}/${pool.token_b_symbol}</h2>
                            <p class="text-gray-400">Pool ID: ${pool.pool_id}</p>
                        </div>
                        
                        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-4 mb-8">
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                                <div class="text-2xl font-bold text-green-400">${(pool.tvl / 1000000).toFixed(2)}</div>
                                <div class="text-sm text-gray-400">TVL (SHADOW)</div>
                            </div>
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                                <div class="text-2xl font-bold text-blue-400">${pool.apr.toFixed(1)}%</div>
                                <div class="text-sm text-gray-400">APR</div>
                            </div>
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                                <div class="text-2xl font-bold text-purple-400">${pool.trade_count}</div>
                                <div class="text-sm text-gray-400">Total Trades</div>
                            </div>
                            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-4 text-center">
                                <div class="text-2xl font-bold text-yellow-400">${(pool.total_liquidity / 1000000).toFixed(2)}</div>
                                <div class="text-sm text-gray-400">LP Tokens</div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6 mb-8">
                            <h3 class="text-xl font-semibold mb-4">Pool Reserves</h3>
                            <div class="grid grid-cols-1 md:grid-cols-2 gap-4">
                                <div class="text-center">
                                    <div class="text-2xl font-bold text-blue-400">${(pool.reserve_a / Math.pow(10, 8)).toFixed(2)}</div>
                                    <div class="text-gray-400">${pool.token_a_symbol}</div>
                                </div>
                                <div class="text-center">
                                    <div class="text-2xl font-bold text-green-400">${(pool.reserve_b / Math.pow(10, 8)).toFixed(2)}</div>
                                    <div class="text-gray-400">${pool.token_b_symbol}</div>
                                </div>
                            </div>
                        </div>
                        
                        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6">
                            <h3 class="text-xl font-semibold mb-4">Recent Transactions</h3>
                            <div id="recentTransactions">
                                ${pool.recent_transactions && pool.recent_transactions.length > 0 ? 
                                    pool.recent_transactions.map(tx => ` + "`" + `
                                        <div class="border-b border-gray-700 py-3 last:border-b-0">
                                            <div class="flex justify-between items-center">
                                                <div>
                                                    <div class="font-mono text-sm">${tx.tx_hash.substring(0, 16)}...</div>
                                                    <div class="text-xs text-gray-400">${tx.type.toUpperCase()}</div>
                                                </div>
                                                <div class="text-right">
                                                    <div class="text-sm">${(tx.amount_a / Math.pow(10, 8)).toFixed(2)} ${pool.token_a_symbol}</div>
                                                    <div class="text-xs text-gray-400">${new Date(tx.timestamp).toLocaleString()}</div>
                                                </div>
                                            </div>
                                        </div>
                                    ` + "`" + `).join('') : 
                                    '<div class="text-center text-gray-400"><p>No transactions found</p></div>'
                                }
                            </div>
                        </div>
                    </div>
                ` + "`" + `;
                
            } catch (error) {
                document.getElementById('poolDetails').innerHTML = ` + "`" + `
                    <div class="text-center text-red-400">
                        <p class="text-xl">❌ Pool not found</p>
                        <p class="text-gray-400 mt-2">Pool ID: ${poolId}</p>
                        <a href="/pools" class="text-blue-400 hover:text-blue-300 mt-4 inline-block">← Back to Pools</a>
                    </div>
                ` + "`" + `;
            }
        }
        
        loadPoolDetails();
    </script>
</body>
</html>`;
    
    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

// Storage/farming network page handler
func (es *ExplorerServer) handleStoragePage(w http.ResponseWriter, r *http.Request) {
    tmpl := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Proof of Storage - Shadowy Explorer</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        .gradient-bg {
            background: linear-gradient(135deg, #1a1a1a 0%, #2d2d2d 100%);
        }
        .card-hover:hover {
            transform: translateY(-2px);
            transition: transform 0.3s ease;
        }
        .pulse-dot {
            animation: pulse 2s infinite;
        }
        @keyframes pulse {
            0%, 100% { opacity: 1; }
            50% { opacity: 0.5; }
        }
    </style>
</head>
<body class="gradient-bg text-white min-h-screen">
    <!-- Navigation -->
    <nav class="bg-gray-900 bg-opacity-80 backdrop-blur-sm border-b border-gray-700">
        <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8">
            <div class="flex justify-between h-16">
                <div class="flex items-center space-x-8">
                    <a href="/" class="text-xl font-bold text-blue-400">Shadowy Explorer</a>
                    <div class="hidden md:flex space-x-6">
                        <a href="/blocks" class="text-gray-300 hover:text-white transition-colors">Blocks</a>
                        <a href="/tokens" class="text-gray-300 hover:text-white transition-colors">Tokens</a>
                        <a href="/pools" class="text-gray-300 hover:text-white transition-colors">Pools</a>
                        <a href="/storage" class="text-blue-400 font-medium">Storage</a>
                    </div>
                </div>
            </div>
        </div>
    </nav>

    <div class="max-w-7xl mx-auto px-4 sm:px-6 lg:px-8 py-8">
        <!-- Page Header -->
        <div class="text-center mb-8">
            <h1 class="text-4xl font-bold mb-4">💾 Proof of Storage Network</h1>
            <p class="text-xl text-gray-300">Farming nodes and network storage capacity</p>
        </div>

        <!-- Network Stats -->
        <div class="grid grid-cols-1 md:grid-cols-4 gap-6 mb-8">
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6 text-center card-hover">
                <div class="text-3xl font-bold text-green-400" id="onlineNodes">-</div>
                <div class="text-sm text-gray-400 mt-1">Online Nodes</div>
                <div class="text-xs text-gray-500 mt-2">
                    <span id="totalNodes">-</span> total nodes
                </div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6 text-center card-hover">
                <div class="text-3xl font-bold text-blue-400" id="totalNetspace">-</div>
                <div class="text-sm text-gray-400 mt-1">Total Netspace</div>
                <div class="text-xs text-gray-500 mt-2">Network storage capacity</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6 text-center card-hover">
                <div class="text-3xl font-bold text-purple-400" id="avgSuccessRate">-</div>
                <div class="text-sm text-gray-400 mt-1">Avg Success Rate</div>
                <div class="text-xs text-gray-500 mt-2">Farming efficiency</div>
            </div>
            <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg p-6 text-center card-hover">
                <div class="text-3xl font-bold text-orange-400" id="consensusHeight">-</div>
                <div class="text-sm text-gray-400 mt-1">Current Height</div>
                <div class="text-xs text-gray-500 mt-2">Network consensus</div>
            </div>
        </div>

        <!-- Farming Nodes Table -->
        <div class="bg-gray-800 bg-opacity-50 backdrop-blur rounded-lg overflow-hidden">
            <div class="px-6 py-4 border-b border-gray-700">
                <h3 class="text-xl font-semibold">Farming Nodes</h3>
            </div>
            <div class="overflow-x-auto">
                <table class="w-full">
                    <thead class="bg-gray-700">
                        <tr>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Node ID</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Status</th>
                            <th class="px-6 py-3 text-right text-xs font-medium text-gray-300 uppercase tracking-wider">Plot Size</th>
                            <th class="px-6 py-3 text-right text-xs font-medium text-gray-300 uppercase tracking-wider">Success Rate</th>
                            <th class="px-6 py-3 text-right text-xs font-medium text-gray-300 uppercase tracking-wider">Blocks Found</th>
                            <th class="px-6 py-3 text-left text-xs font-medium text-gray-300 uppercase tracking-wider">Last Block</th>
                        </tr>
                    </thead>
                    <tbody id="nodesTable" class="divide-y divide-gray-700">
                        <!-- Nodes will be loaded here -->
                    </tbody>
                </table>
            </div>
        </div>

        <div class="mt-6 text-center text-gray-400">
            <p>Data refreshed every 30 seconds from network tracker</p>
        </div>
    </div>

    <script>
        // Load storage network data
        async function loadStorageData() {
            try {
                const response = await fetch('/api/v1/storage');
                const data = await response.json();
                
                // Update stats
                document.getElementById('onlineNodes').textContent = data.online_nodes || 0;
                document.getElementById('totalNodes').textContent = data.total_nodes || 0;
                document.getElementById('totalNetspace').textContent = formatBytes(data.total_netspace || 0);
                document.getElementById('avgSuccessRate').textContent = (data.avg_success_rate || 0).toFixed(1) + '%';
                document.getElementById('consensusHeight').textContent = (data.consensus_height || 0).toLocaleString();
                
                // Update nodes table
                const tbody = document.getElementById('nodesTable');
                tbody.innerHTML = '';
                
                if (data.nodes && data.nodes.length > 0) {
                    data.nodes.forEach((node, index) => {
                        const row = document.createElement('tr');
                        row.className = index % 2 === 0 ? 'bg-gray-800 bg-opacity-30' : 'bg-gray-700 bg-opacity-30';
                        
                        const statusClass = node.status === 'online' ? 'text-green-400' : 
                                           node.status === 'syncing' ? 'text-yellow-400' : 'text-red-400';
                        const statusDot = node.status === 'online' ? '<div class="w-2 h-2 bg-green-400 rounded-full pulse-dot inline-block mr-2"></div>' : 
                                         '<div class="w-2 h-2 bg-gray-400 rounded-full inline-block mr-2"></div>';
                        
                        const shortNodeId = node.node_id.length > 16 ? node.node_id.substring(0, 16) + '...' : node.node_id;
                        const lastBlockDate = node.last_block_time ? new Date(node.last_block_time).toLocaleDateString() : 'Never';
                        
                        row.innerHTML = ` + "`" + `
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="text-sm font-mono text-white">${shortNodeId}</div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="flex items-center">
                                    ${statusDot}
                                    <span class="text-sm font-medium ${statusClass} capitalize">${node.status}</span>
                                </div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-right">
                                <div class="text-sm font-bold text-blue-400">${formatBytes(node.plot_size)}</div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-right">
                                <div class="text-sm font-bold text-purple-400">${(node.success_rate || 0).toFixed(1)}%</div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap text-right">
                                <div class="text-sm text-white">${(node.blocks_found || 0).toLocaleString()}</div>
                            </td>
                            <td class="px-6 py-4 whitespace-nowrap">
                                <div class="text-sm text-gray-300">${lastBlockDate}</div>
                            </td>
                        ` + "`" + `;
                        
                        tbody.appendChild(row);
                    });
                } else {
                    tbody.innerHTML = ` + "`" + `
                        <tr>
                            <td colspan="6" class="px-6 py-8 text-center text-gray-400">
                                <div class="text-4xl mb-2">💾</div>
                                <p class="text-lg">No farming nodes detected</p>
                                <p class="text-sm">Waiting for nodes to connect to the tracker...</p>
                            </td>
                        </tr>
                    ` + "`" + `;
                }
                
            } catch (error) {
                console.error('Failed to load storage data:', error);
                document.getElementById('nodesTable').innerHTML = ` + "`" + `
                    <tr>
                        <td colspan="6" class="px-6 py-8 text-center text-gray-400">
                            <div class="text-4xl mb-2">⚠️</div>
                            <p class="text-lg">Failed to load storage data</p>
                            <p class="text-sm">Network tracker may be unavailable</p>
                        </td>
                    </tr>
                ` + "`" + `;
            }
        }
        
        function formatBytes(bytes) {
            if (bytes === 0) return '0 B';
            const k = 1024;
            const sizes = ['B', 'KB', 'MB', 'GB', 'TB', 'PB'];
            const i = Math.floor(Math.log(bytes) / Math.log(k));
            return parseFloat((bytes / Math.pow(k, i)).toFixed(2)) + ' ' + sizes[i];
        }
        
        // Load data on page load
        loadStorageData();
        
        // Refresh data every 30 seconds
        setInterval(loadStorageData, 30000);
    </script>
</body>
</html>`;

    w.Header().Set("Content-Type", "text/html")
    w.Write([]byte(tmpl))
}

// detectShadowyNode attempts to find the running Tendermint node
func detectShadowyNode() string {
    ports := []string{"26657"}
    client := &http.Client{Timeout: 3 * time.Second}

    for _, port := range ports {
        url := fmt.Sprintf("http://localhost:%s", port)
        log.Printf("🔍 Checking for Tendermint node at %s...", url)

        resp, err := client.Get(url + "/status")
        if err != nil {
            log.Printf("❌ Failed to connect to Tendermint node at %s: %v", url, err)
        }
        if err == nil && resp.StatusCode == http.StatusOK {
            resp.Body.Close()
            log.Printf("✅ Found Tendermint node at %s", url)
            return url
        }
        if resp != nil {
            log.Printf("❌ Failed to connect to Tendermint node at %s/status / %d", url, resp.StatusCode)
            resp.Body.Close()
        }
    }

    log.Fatalln("⚠️ No Tendermint node found on port 26657, shutting down...")
    return "http://localhost:26657"
}

func main() {
    fmt.Println("🌟 Starting Shadowy Blockchain Explorer...")

    // Auto-detect Tendermint node or use environment variable
    shadowyNodeURL := "http://localhost:26657"
    if url := os.Getenv("SHADOWY_NODE_URL"); url != "" {
        shadowyNodeURL = url
        log.Printf("📍 Using SHADOWY_NODE_URL: %s", shadowyNodeURL)
    } else {
        shadowyNodeURL = detectShadowyNode()
    }

    // Initialize database
    database, err := NewDatabase("./explorer_data")
    if err != nil {
        log.Fatal("Failed to initialize database:", err)
    }
    defer database.Close()

    // Initialize sync service
    syncService := NewSyncService(shadowyNodeURL, database)

    // Start background sync
    syncService.Start()
    defer syncService.Stop()

    // Create and start explorer server
    explorer := NewExplorerServer(shadowyNodeURL, database, syncService)

    if err := explorer.Start(); err != nil {
        log.Fatal("Failed to start explorer:", err)
    }
}
