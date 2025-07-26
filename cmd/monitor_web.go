package cmd

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// WebMonitor provides a web-based monitoring dashboard
type WebMonitor struct {
	port         int
	server       *http.Server
	router       *mux.Router
	apiBaseURL   string
	refreshRate  int // seconds
}

// MonitoringData aggregates all monitoring information
type MonitoringData struct {
	Timestamp         time.Time                 `json:"timestamp"`
	NodeHealth        map[string]interface{}    `json:"node_health"`
	BlockchainStatus  map[string]interface{}    `json:"blockchain_status"`
	MiningStats       map[string]interface{}    `json:"mining_stats"`
	ConsensusStatus   map[string]interface{}    `json:"consensus_status"`
	MempoolStats      map[string]interface{}    `json:"mempool_stats"`
	TimelordStats     map[string]interface{}    `json:"timelord_stats"`
	FarmingStats      map[string]interface{}    `json:"farming_stats"`
	SystemMetrics     SystemMetrics             `json:"system_metrics"`
	RecentBlocks      []map[string]interface{}  `json:"recent_blocks"`
	RecentTransactions []map[string]interface{} `json:"recent_transactions"`
}

// SystemMetrics contains system-level metrics
type SystemMetrics struct {
	Uptime       time.Duration `json:"uptime"`
	MemoryUsage  uint64        `json:"memory_usage_mb"`
	CPUUsage     float64       `json:"cpu_usage_percent"`
	DiskUsage    uint64        `json:"disk_usage_mb"`
	NetworkIn    uint64        `json:"network_in_bytes"`
	NetworkOut   uint64        `json:"network_out_bytes"`
	Goroutines   int           `json:"goroutines"`
}

// NewWebMonitor creates a new web monitoring dashboard
func NewWebMonitor(port int, apiBaseURL string) *WebMonitor {
	wm := &WebMonitor{
		port:        port,
		apiBaseURL:  apiBaseURL,
		refreshRate: 5, // 5 seconds default
		router:      mux.NewRouter(),
	}
	
	wm.setupRoutes()
	return wm
}

// setupRoutes configures HTTP routes for the monitoring dashboard
func (wm *WebMonitor) setupRoutes() {
	// Static dashboard routes
	wm.router.HandleFunc("/", wm.handleDashboard).Methods("GET")
	wm.router.HandleFunc("/dashboard", wm.handleDashboard).Methods("GET")
	wm.router.HandleFunc("/health", wm.handleHealthPage).Methods("GET")
	wm.router.HandleFunc("/mining", wm.handleMiningPage).Methods("GET")
	wm.router.HandleFunc("/consensus", wm.handleConsensusPage).Methods("GET")
	wm.router.HandleFunc("/blocks", wm.handleBlocksPage).Methods("GET")
	wm.router.HandleFunc("/transactions", wm.handleTransactionsPage).Methods("GET")
	
	// API routes for AJAX data
	wm.router.HandleFunc("/api/monitoring", wm.handleMonitoringAPI).Methods("GET")
	wm.router.HandleFunc("/api/health", wm.handleHealthAPI).Methods("GET")
	wm.router.HandleFunc("/api/mining", wm.handleMiningAPI).Methods("GET")
	wm.router.HandleFunc("/api/consensus", wm.handleConsensusAPI).Methods("GET")
	wm.router.HandleFunc("/api/blocks", wm.handleBlocksAPI).Methods("GET")
	wm.router.HandleFunc("/api/transactions", wm.handleTransactionsAPI).Methods("GET")
	wm.router.HandleFunc("/api/metrics", wm.handleMetricsAPI).Methods("GET")
	
	// WebSocket for real-time updates
	wm.router.HandleFunc("/ws/monitoring", wm.handleWebSocket).Methods("GET")
	
	// Static assets
	wm.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", 
		http.HandlerFunc(wm.handleStaticAssets)))
}

// Start begins the web monitoring server
func (wm *WebMonitor) Start() error {
	wm.server = &http.Server{
		Addr:         fmt.Sprintf(":%d", wm.port),
		Handler:      wm.router,
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  120 * time.Second,
	}
	
	log.Printf("Starting web monitoring dashboard on port %d", wm.port)
	log.Printf("Dashboard URL: http://localhost:%d", wm.port)
	log.Printf("API Base URL: %s", wm.apiBaseURL)
	
	return wm.server.ListenAndServe()
}

// Stop shuts down the web monitoring server
func (wm *WebMonitor) Stop() error {
	if wm.server != nil {
		log.Println("Shutting down web monitoring dashboard...")
		return wm.server.Close()
	}
	return nil
}

// handleDashboard serves the main monitoring dashboard
func (wm *WebMonitor) handleDashboard(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Blockchain Monitor</title>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <script src="https://cdn.tailwindcss.com"></script>
    <style>
        .metric-card { transition: all 0.3s ease; }
        .metric-card:hover { transform: translateY(-2px); box-shadow: 0 8px 25px rgba(0,0,0,0.15); }
        .status-online { color: #10b981; }
        .status-warning { color: #f59e0b; }
        .status-offline { color: #ef4444; }
        .chart-container { height: 300px; }
    </style>
</head>
<body class="bg-gray-100 min-h-screen">
    <!-- Header -->
    <header class="bg-indigo-600 text-white shadow-lg">
        <div class="container mx-auto px-6 py-4">
            <div class="flex items-center justify-between">
                <h1 class="text-3xl font-bold">🌑 Shadowy Blockchain Monitor</h1>
                <div class="flex items-center space-x-4">
                    <span id="status-indicator" class="flex items-center">
                        <div class="w-3 h-3 rounded-full bg-green-400 mr-2 animate-pulse"></div>
                        <span class="font-medium">Online</span>
                    </span>
                    <span id="last-update" class="text-indigo-200 text-sm"></span>
                </div>
            </div>
        </div>
    </header>

    <!-- Navigation -->
    <nav class="bg-white shadow-sm border-b">
        <div class="container mx-auto px-6">
            <div class="flex space-x-8">
                <a href="/dashboard" class="py-4 px-2 border-b-2 border-indigo-500 text-indigo-600 font-medium">Dashboard</a>
                <a href="/health" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Health</a>
                <a href="/mining" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Mining</a>
                <a href="/consensus" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Consensus</a>
                <a href="/blocks" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Blocks</a>
                <a href="/transactions" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Transactions</a>
            </div>
        </div>
    </nav>

    <!-- Main Content -->
    <main class="container mx-auto px-6 py-8">
        <!-- Key Metrics Row -->
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-5 gap-6 mb-8">
            <!-- Sync Status Card -->
            <div class="metric-card bg-white rounded-lg shadow-md p-6">
                <div class="flex items-center justify-between">
                    <div>
                        <p class="text-gray-500 text-sm font-medium">Sync Status</p>
                        <p id="sync-status" class="text-lg font-bold text-gray-900">--</p>
                    </div>
                    <div id="sync-icon" class="bg-indigo-100 p-3 rounded-full">
                        <svg class="w-6 h-6 text-indigo-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>
                        </svg>
                    </div>
                </div>
                <div class="mt-2">
                    <div id="sync-progress-bar" class="w-full bg-gray-200 rounded-full h-2 mb-1" style="display: none;">
                        <div id="sync-progress-fill" class="bg-indigo-600 h-2 rounded-full transition-all duration-300" style="width: 0%"></div>
                    </div>
                    <span id="sync-details" class="text-sm text-gray-500">Ready</span>
                </div>
            </div>
            <div class="metric-card bg-white rounded-lg shadow-md p-6">
                <div class="flex items-center justify-between">
                    <div>
                        <p class="text-gray-500 text-sm font-medium">Block Height</p>
                        <p id="block-height" class="text-3xl font-bold text-gray-900">--</p>
                    </div>
                    <div class="bg-blue-100 p-3 rounded-full">
                        <svg class="w-6 h-6 text-blue-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 11H5m14 0a2 2 0 012 2v6a2 2 0 01-2 2H5a2 2 0 01-2-2v-6a2 2 0 012-2m14 0V9a2 2 0 00-2-2M5 11V9a2 2 0 012-2m0 0V5a2 2 0 012-2h6a2 2 0 012 2v2M7 7h10"/>
                        </svg>
                    </div>
                </div>
                <div class="mt-2">
                    <span id="blocks-trend" class="text-sm text-green-600">+12 in last hour</span>
                </div>
            </div>

            <div class="metric-card bg-white rounded-lg shadow-md p-6">
                <div class="flex items-center justify-between">
                    <div>
                        <p class="text-gray-500 text-sm font-medium">Mining Rate</p>
                        <p id="mining-rate" class="text-3xl font-bold text-gray-900">--</p>
                    </div>
                    <div class="bg-green-100 p-3 rounded-full">
                        <svg class="w-6 h-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M13 10V3L4 14h7v7l9-11h-7z"/>
                        </svg>
                    </div>
                </div>
                <div class="mt-2">
                    <span id="mining-trend" class="text-sm text-gray-500">blocks/hour</span>
                </div>
            </div>

            <div class="metric-card bg-white rounded-lg shadow-md p-6">
                <div class="flex items-center justify-between">
                    <div>
                        <p class="text-gray-500 text-sm font-medium">Active Peers</p>
                        <p id="peer-count" class="text-3xl font-bold text-gray-900">--</p>
                    </div>
                    <div class="bg-purple-100 p-3 rounded-full">
                        <svg class="w-6 h-6 text-purple-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M17 20h5v-2a3 3 0 00-5.356-1.857M17 20H7m10 0v-2c0-.656-.126-1.283-.356-1.857M7 20H2v-2a3 3 0 015.356-1.857M7 20v-2c0-.656.126-1.283.356-1.857m0 0a5.002 5.002 0 019.288 0M15 7a3 3 0 11-6 0 3 3 0 016 0zm6 3a2 2 0 11-4 0 2 2 0 014 0zM7 10a2 2 0 11-4 0 2 2 0 014 0z"/>
                        </svg>
                    </div>
                </div>
                <div class="mt-2">
                    <span id="peer-status" class="text-sm status-online">All healthy</span>
                </div>
            </div>

            <div class="metric-card bg-white rounded-lg shadow-md p-6">
                <div class="flex items-center justify-between">
                    <div>
                        <p class="text-gray-500 text-sm font-medium">Mempool Size</p>
                        <p id="mempool-size" class="text-3xl font-bold text-gray-900">--</p>
                    </div>
                    <div class="bg-yellow-100 p-3 rounded-full">
                        <svg class="w-6 h-6 text-yellow-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">
                            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M12 8c-1.657 0-3 .895-3 2s1.343 2 3 2 3 .895 3 2-1.343 2-3 2m0-8c1.11 0 2.08.402 2.599 1M12 8V7m0 1v8m0 0v1m0-1c-1.11 0-2.08-.402-2.599-1"/>
                        </svg>
                    </div>
                </div>
                <div class="mt-2">
                    <span id="mempool-trend" class="text-sm text-gray-500">pending transactions</span>
                </div>
            </div>
        </div>

        <!-- Charts Row -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-8 mb-8">
            <div class="bg-white rounded-lg shadow-md p-6">
                <h3 class="text-lg font-semibold text-gray-900 mb-4">Block Production Rate</h3>
                <div class="chart-container">
                    <canvas id="blockChart"></canvas>
                </div>
            </div>

            <div class="bg-white rounded-lg shadow-md p-6">
                <h3 class="text-lg font-semibold text-gray-900 mb-4">System Resources</h3>
                <div class="chart-container">
                    <canvas id="resourceChart"></canvas>
                </div>
            </div>
        </div>

        <!-- Recent Activity -->
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
            <div class="bg-white rounded-lg shadow-md p-6">
                <h3 class="text-lg font-semibold text-gray-900 mb-4">Recent Blocks</h3>
                <div id="recent-blocks" class="space-y-3">
                    <!-- Populated by JavaScript -->
                </div>
            </div>

            <div class="bg-white rounded-lg shadow-md p-6">
                <h3 class="text-lg font-semibold text-gray-900 mb-4">System Status</h3>
                <div id="system-status" class="space-y-3">
                    <!-- Populated by JavaScript -->
                </div>
            </div>
        </div>
    </main>

    <script>
        // Global variables
        let blockChart, resourceChart;
        const refreshInterval = {{.RefreshRate}} * 1000;
        
        // Initialize dashboard
        document.addEventListener('DOMContentLoaded', function() {
            initializeCharts();
            updateDashboard();
            setInterval(updateDashboard, refreshInterval);
        });

        // Initialize Chart.js charts
        function initializeCharts() {
            // Block production chart
            const blockCtx = document.getElementById('blockChart').getContext('2d');
            blockChart = new Chart(blockCtx, {
                type: 'line',
                data: {
                    labels: [],
                    datasets: [{
                        label: 'Blocks per Hour',
                        data: [],
                        borderColor: 'rgb(59, 130, 246)',
                        backgroundColor: 'rgba(59, 130, 246, 0.1)',
                        tension: 0.4
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        y: { beginAtZero: true }
                    }
                }
            });

            // Resource usage chart
            const resourceCtx = document.getElementById('resourceChart').getContext('2d');
            resourceChart = new Chart(resourceCtx, {
                type: 'doughnut',
                data: {
                    labels: ['CPU', 'Memory', 'Available'],
                    datasets: [{
                        data: [0, 0, 100],
                        backgroundColor: [
                            'rgba(239, 68, 68, 0.8)',
                            'rgba(245, 158, 11, 0.8)',
                            'rgba(16, 185, 129, 0.8)'
                        ]
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false
                }
            });
        }

        // Update dashboard data
        async function updateDashboard() {
            try {
                const response = await fetch('/api/monitoring');
                const data = await response.json();
                
                updateMetrics(data);
                updateCharts(data);
                updateRecentActivity(data);
                updateLastRefresh();
                
                // Update status indicator
                document.getElementById('status-indicator').innerHTML = 
                    '<div class="w-3 h-3 rounded-full bg-green-400 mr-2 animate-pulse"></div><span class="font-medium">Online</span>';
                    
            } catch (error) {
                console.error('Failed to update dashboard:', error);
                document.getElementById('status-indicator').innerHTML = 
                    '<div class="w-3 h-3 rounded-full bg-red-400 mr-2"></div><span class="font-medium">Offline</span>';
            }
        }

        // Update key metrics
        function updateMetrics(data) {
            // Update sync status
            if (data.consensus_status && data.consensus_status.sync_status) {
                const syncStatus = data.consensus_status.sync_status;
                const syncStatusEl = document.getElementById('sync-status');
                const syncDetailsEl = document.getElementById('sync-details');
                const syncProgressBar = document.getElementById('sync-progress-bar');
                const syncProgressFill = document.getElementById('sync-progress-fill');
                const syncIcon = document.getElementById('sync-icon');
                
                if (syncStatus.is_syncing) {
                    syncStatusEl.textContent = 'Syncing';
                    syncStatusEl.className = 'text-lg font-bold text-yellow-600';
                    
                    // Show progress bar and update progress
                    syncProgressBar.style.display = 'block';
                    const progress = Math.round((syncStatus.sync_progress || 0) * 100);
                    syncProgressFill.style.width = progress + '%';
                    
                    // Update details with current/target height
                    const current = syncStatus.current_height || 0;
                    const target = syncStatus.target_height || 0;
                    syncDetailsEl.innerHTML = 
                        '<span class="text-sm text-yellow-600">' + 
                        current + ' / ' + target + ' (' + progress + '%)</span>';
                    
                    // Update icon to spinning/syncing
                    syncIcon.className = 'bg-yellow-100 p-3 rounded-full animate-pulse';
                    syncIcon.innerHTML = 
                        '<svg class="w-6 h-6 text-yellow-600 animate-spin" fill="none" stroke="currentColor" viewBox="0 0 24 24">' +
                        '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M4 4v5h.582m15.356 2A8.001 8.001 0 004.582 9m0 0H9m11 11v-5h-.581m0 0a8.003 8.003 0 01-15.357-2m15.357 2H15"/>' +
                        '</svg>';
                } else {
                    syncStatusEl.textContent = 'Synced';
                    syncStatusEl.className = 'text-lg font-bold text-green-600';
                    
                    // Hide progress bar
                    syncProgressBar.style.display = 'none';
                    
                    // Show current height
                    syncDetailsEl.innerHTML = 
                        '<span class="text-sm text-green-600">Height: ' + 
                        (syncStatus.current_height || '--') + '</span>';
                    
                    // Update icon to success
                    syncIcon.className = 'bg-green-100 p-3 rounded-full';
                    syncIcon.innerHTML = 
                        '<svg class="w-6 h-6 text-green-600" fill="none" stroke="currentColor" viewBox="0 0 24 24">' +
                        '<path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M5 13l4 4L19 7"/>' +
                        '</svg>';
                }
            } else {
                // No sync data available
                document.getElementById('sync-status').textContent = 'Unknown';
                document.getElementById('sync-status').className = 'text-lg font-bold text-gray-500';
                document.getElementById('sync-details').innerHTML = '<span class="text-sm text-gray-500">No data</span>';
            }
            
            if (data.blockchain_status) {
                document.getElementById('block-height').textContent = 
                    data.blockchain_status.height || '--';
            }
            
            if (data.mining_stats) {
                document.getElementById('mining-rate').textContent = 
                    (data.mining_stats.blocks_per_hour || '--') + '/hr';
            }
            
            if (data.consensus_status) {
                document.getElementById('peer-count').textContent = 
                    data.consensus_status.connected_peers || '--';
            }
            
            if (data.mempool_stats) {
                document.getElementById('mempool-size').textContent = 
                    data.mempool_stats.pending_count || '--';
            }
        }

        // Update charts with new data
        function updateCharts(data) {
            // Update block chart (simplified)
            if (data.mining_stats && blockChart) {
                const now = new Date().toLocaleTimeString();
                blockChart.data.labels.push(now);
                blockChart.data.datasets[0].data.push(data.mining_stats.blocks_per_hour || 0);
                
                // Keep only last 20 data points
                if (blockChart.data.labels.length > 20) {
                    blockChart.data.labels.shift();
                    blockChart.data.datasets[0].data.shift();
                }
                blockChart.update();
            }
            
            // Update resource chart
            if (data.system_metrics && resourceChart) {
                const cpu = data.system_metrics.cpu_usage_percent || 0;
                const memory = (data.system_metrics.memory_usage_mb / 1024) || 0; // Convert to GB, simplified
                resourceChart.data.datasets[0].data = [cpu, memory, Math.max(0, 100 - cpu - memory)];
                resourceChart.update();
            }
        }

        // Update recent activity sections
        function updateRecentActivity(data) {
            // Recent blocks
            const blocksContainer = document.getElementById('recent-blocks');
            if (data.recent_blocks) {
                blocksContainer.innerHTML = data.recent_blocks.slice(0, 5).map(block => 
                    '<div class="flex items-center justify-between p-3 bg-gray-50 rounded">' +
                    '<div>' +
                    '<p class="font-medium">Block #' + (block.height || 'N/A') + '</p>' +
                    '<p class="text-sm text-gray-500">' + (block.hash ? block.hash.substring(0, 16) + '...' : 'N/A') + '</p>' +
                    '</div>' +
                    '<span class="text-sm text-gray-500">' + timeAgo(block.timestamp) + '</span>' +
                    '</div>'
                ).join('');
            }
            
            // System status
            const statusContainer = document.getElementById('system-status');
            const statusItems = [];
            
            if (data.node_health) {
                const status = data.node_health.status === 'healthy' ? 'status-online' : 'status-warning';
                statusItems.push(
                    '<div class="flex items-center justify-between p-3 bg-gray-50 rounded">' +
                    '<span>Node Health</span>' +
                    '<span class="' + status + ' font-medium">' + (data.node_health.status || 'Unknown') + '</span>' +
                    '</div>'
                );
            }
            
            if (data.system_metrics) {
                statusItems.push(
                    '<div class="flex items-center justify-between p-3 bg-gray-50 rounded">' +
                    '<span>Uptime</span>' +
                    '<span class="font-medium">' + formatUptime(data.system_metrics.uptime) + '</span>' +
                    '</div>'
                );
                
                statusItems.push(
                    '<div class="flex items-center justify-between p-3 bg-gray-50 rounded">' +
                    '<span>Goroutines</span>' +
                    '<span class="font-medium">' + (data.system_metrics.goroutines || 0) + '</span>' +
                    '</div>'
                );
            }
            
            statusContainer.innerHTML = statusItems.join('');
        }

        // Update last refresh timestamp
        function updateLastRefresh() {
            document.getElementById('last-update').textContent = 
                'Last updated: ' + new Date().toLocaleTimeString();
        }

        // Utility functions
        function timeAgo(timestamp) {
            if (!timestamp) return 'Unknown';
            const now = new Date();
            const time = new Date(timestamp);
            const diff = Math.floor((now - time) / 1000);
            
            if (diff < 60) return diff + 's ago';
            if (diff < 3600) return Math.floor(diff / 60) + 'm ago';
            if (diff < 86400) return Math.floor(diff / 3600) + 'h ago';
            return Math.floor(diff / 86400) + 'd ago';
        }
        
        function formatUptime(uptimeNs) {
            if (!uptimeNs) return 'Unknown';
            const seconds = Math.floor(uptimeNs / 1000000000);
            const days = Math.floor(seconds / 86400);
            const hours = Math.floor((seconds % 86400) / 3600);
            const mins = Math.floor((seconds % 3600) / 60);
            
            if (days > 0) return days + 'd ' + hours + 'h';
            if (hours > 0) return hours + 'h ' + mins + 'm';
            return mins + 'm ' + (seconds % 60) + 's';
        }
    </script>
</body>
</html>
`

	t := template.Must(template.New("dashboard").Parse(tmpl))
	data := struct {
		RefreshRate int
	}{
		RefreshRate: wm.refreshRate,
	}
	
	w.Header().Set("Content-Type", "text/html")
	t.Execute(w, data)
}