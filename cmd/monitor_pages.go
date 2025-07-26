package cmd

import (
	"net/http"
)

// handleHealthPage serves the health monitoring page
func (wm *WebMonitor) handleHealthPage(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Health Monitor - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <!-- Header -->
    <header class="bg-indigo-600 text-white shadow-lg">
        <div class="container mx-auto px-6 py-4">
            <h1 class="text-3xl font-bold">🌑 Health Monitor</h1>
        </div>
    </header>

    <!-- Navigation -->
    <nav class="bg-white shadow-sm border-b">
        <div class="container mx-auto px-6">
            <div class="flex space-x-8">
                <a href="/dashboard" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Dashboard</a>
                <a href="/health" class="py-4 px-2 border-b-2 border-indigo-500 text-indigo-600 font-medium">Health</a>
                <a href="/mining" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Mining</a>
                <a href="/consensus" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Consensus</a>
                <a href="/blocks" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Blocks</a>
                <a href="/transactions" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Transactions</a>
            </div>
        </div>
    </nav>

    <!-- Content -->
    <main class="container mx-auto px-6 py-8">
        <div id="health-content" class="space-y-6">
            <div class="bg-white rounded-lg shadow-md p-6">
                <h2 class="text-xl font-semibold mb-4">Node Health Status</h2>
                <div id="health-status">Loading...</div>
            </div>
        </div>
    </main>

    <script>
        async function loadHealthData() {
            try {
                const response = await fetch('/api/health');
                const data = await response.json();
                document.getElementById('health-status').innerHTML = 
                    '<pre class="bg-gray-100 p-4 rounded">' + JSON.stringify(data, null, 2) + '</pre>';
            } catch (error) {
                document.getElementById('health-status').innerHTML = 
                    '<div class="text-red-600">Failed to load health data: ' + error.message + '</div>';
            }
        }
        
        document.addEventListener('DOMContentLoaded', loadHealthData);
        setInterval(loadHealthData, 5000);
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

// handleMiningPage serves the mining monitoring page
func (wm *WebMonitor) handleMiningPage(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Mining Monitor - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <!-- Header -->
    <header class="bg-indigo-600 text-white shadow-lg">
        <div class="container mx-auto px-6 py-4">
            <h1 class="text-3xl font-bold">⛏️ Mining Monitor</h1>
        </div>
    </header>

    <!-- Navigation -->
    <nav class="bg-white shadow-sm border-b">
        <div class="container mx-auto px-6">
            <div class="flex space-x-8">
                <a href="/dashboard" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Dashboard</a>
                <a href="/health" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Health</a>
                <a href="/mining" class="py-4 px-2 border-b-2 border-indigo-500 text-indigo-600 font-medium">Mining</a>
                <a href="/consensus" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Consensus</a>
                <a href="/blocks" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Blocks</a>
                <a href="/transactions" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Transactions</a>
            </div>
        </div>
    </nav>

    <!-- Content -->
    <main class="container mx-auto px-6 py-8">
        <div class="grid grid-cols-1 lg:grid-cols-2 gap-8">
            <div class="bg-white rounded-lg shadow-md p-6">
                <h2 class="text-xl font-semibold mb-4">Mining Statistics</h2>
                <div id="mining-stats">Loading...</div>
            </div>
            
            <div class="bg-white rounded-lg shadow-md p-6">
                <h2 class="text-xl font-semibold mb-4">Mining Performance</h2>
                <div style="height: 300px;">
                    <canvas id="miningChart"></canvas>
                </div>
            </div>
        </div>
    </main>

    <script>
        let miningChart;
        
        function initMiningChart() {
            const ctx = document.getElementById('miningChart').getContext('2d');
            miningChart = new Chart(ctx, {
                type: 'line',
                data: {
                    labels: [],
                    datasets: [{
                        label: 'Hash Rate',
                        data: [],
                        borderColor: 'rgb(16, 185, 129)',
                        backgroundColor: 'rgba(16, 185, 129, 0.1)',
                        tension: 0.4
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: { y: { beginAtZero: true } }
                }
            });
        }
        
        async function loadMiningData() {
            try {
                const response = await fetch('/api/mining');
                const data = await response.json();
                
                document.getElementById('mining-stats').innerHTML = 
                    '<pre class="bg-gray-100 p-4 rounded text-sm">' + JSON.stringify(data, null, 2) + '</pre>';
                    
                // Update chart
                if (miningChart && data.status) {
                    const now = new Date().toLocaleTimeString();
                    miningChart.data.labels.push(now);
                    miningChart.data.datasets[0].data.push(data.status.hash_rate || 0);
                    
                    if (miningChart.data.labels.length > 20) {
                        miningChart.data.labels.shift();
                        miningChart.data.datasets[0].data.shift();
                    }
                    miningChart.update();
                }
            } catch (error) {
                document.getElementById('mining-stats').innerHTML = 
                    '<div class="text-red-600">Failed to load mining data: ' + error.message + '</div>';
            }
        }
        
        document.addEventListener('DOMContentLoaded', function() {
            initMiningChart();
            loadMiningData();
            setInterval(loadMiningData, 5000);
        });
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

// handleConsensusPage serves the consensus monitoring page
func (wm *WebMonitor) handleConsensusPage(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Consensus Monitor - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <!-- Header -->
    <header class="bg-indigo-600 text-white shadow-lg">
        <div class="container mx-auto px-6 py-4">
            <h1 class="text-3xl font-bold">🤝 Consensus Monitor</h1>
        </div>
    </header>

    <!-- Navigation -->
    <nav class="bg-white shadow-sm border-b">
        <div class="container mx-auto px-6">
            <div class="flex space-x-8">
                <a href="/dashboard" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Dashboard</a>
                <a href="/health" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Health</a>
                <a href="/mining" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Mining</a>
                <a href="/consensus" class="py-4 px-2 border-b-2 border-indigo-500 text-indigo-600 font-medium">Consensus</a>
                <a href="/blocks" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Blocks</a>
                <a href="/transactions" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Transactions</a>
            </div>
        </div>
    </nav>

    <!-- Content -->
    <main class="container mx-auto px-6 py-8">
        <div class="space-y-6">
            <div class="bg-white rounded-lg shadow-md p-6">
                <h2 class="text-xl font-semibold mb-4">Network Consensus Status</h2>
                <div id="consensus-status">Loading...</div>
            </div>
            
            <div class="bg-white rounded-lg shadow-md p-6">
                <h2 class="text-xl font-semibold mb-4">Connected Peers</h2>
                <div id="peer-list">Loading...</div>
            </div>
        </div>
    </main>

    <script>
        async function loadConsensusData() {
            try {
                const response = await fetch('/api/consensus');
                const data = await response.json();
                
                document.getElementById('consensus-status').innerHTML = 
                    '<pre class="bg-gray-100 p-4 rounded text-sm">' + JSON.stringify(data, null, 2) + '</pre>';
                    
                // Format peer list if available
                if (data.peers) {
                    const peerList = data.peers.map(peer => 
                        '<div class="flex items-center justify-between p-3 bg-gray-50 rounded mb-2">' +
                        '<span class="font-medium">' + (peer.id || 'Unknown') + '</span>' +
                        '<span class="text-sm text-gray-500">' + (peer.address || 'N/A') + '</span>' +
                        '<span class="text-xs px-2 py-1 bg-green-100 text-green-800 rounded">' + (peer.status || 'Connected') + '</span>' +
                        '</div>'
                    ).join('');
                    document.getElementById('peer-list').innerHTML = peerList || '<p class="text-gray-500">No peers connected</p>';
                } else {
                    document.getElementById('peer-list').innerHTML = '<p class="text-gray-500">Peer information not available</p>';
                }
            } catch (error) {
                document.getElementById('consensus-status').innerHTML = 
                    '<div class="text-red-600">Failed to load consensus data: ' + error.message + '</div>';
                document.getElementById('peer-list').innerHTML = 
                    '<div class="text-red-600">Failed to load peer data: ' + error.message + '</div>';
            }
        }
        
        document.addEventListener('DOMContentLoaded', loadConsensusData);
        setInterval(loadConsensusData, 5000);
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

// handleBlocksPage serves the blocks monitoring page
func (wm *WebMonitor) handleBlocksPage(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Blocks Monitor - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <!-- Header -->
    <header class="bg-indigo-600 text-white shadow-lg">
        <div class="container mx-auto px-6 py-4">
            <h1 class="text-3xl font-bold">🧱 Blocks Monitor</h1>
        </div>
    </header>

    <!-- Navigation -->
    <nav class="bg-white shadow-sm border-b">
        <div class="container mx-auto px-6">
            <div class="flex space-x-8">
                <a href="/dashboard" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Dashboard</a>
                <a href="/health" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Health</a>
                <a href="/mining" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Mining</a>
                <a href="/consensus" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Consensus</a>
                <a href="/blocks" class="py-4 px-2 border-b-2 border-indigo-500 text-indigo-600 font-medium">Blocks</a>
                <a href="/transactions" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Transactions</a>
            </div>
        </div>
    </nav>

    <!-- Content -->
    <main class="container mx-auto px-6 py-8">
        <div class="bg-white rounded-lg shadow-md p-6">
            <h2 class="text-xl font-semibold mb-4">Recent Blocks</h2>
            <div id="blocks-list">Loading...</div>
        </div>
    </main>

    <script>
        async function loadBlocksData() {
            try {
                const response = await fetch('/api/blocks');
                const data = await response.json();
                
                if (data.blocks && Array.isArray(data.blocks)) {
                    const blocksList = data.blocks.slice(0, 20).map(block => 
                        '<div class="border-b border-gray-200 pb-4 mb-4 last:border-b-0">' +
                        '<div class="flex items-center justify-between mb-2">' +
                        '<h3 class="text-lg font-semibold">Block #' + (block.height || 'N/A') + '</h3>' +
                        '<span class="text-sm text-gray-500">' + (block.timestamp ? new Date(block.timestamp).toLocaleString() : 'Unknown') + '</span>' +
                        '</div>' +
                        '<div class="grid grid-cols-1 md:grid-cols-2 gap-4 text-sm">' +
                        '<div><strong>Hash:</strong> <code class="bg-gray-100 px-2 py-1 rounded text-xs">' + (block.hash || 'N/A').substring(0, 32) + '...</code></div>' +
                        '<div><strong>Previous Hash:</strong> <code class="bg-gray-100 px-2 py-1 rounded text-xs">' + (block.previous_hash || 'N/A').substring(0, 32) + '...</code></div>' +
                        '<div><strong>Transactions:</strong> ' + (block.transaction_count || 0) + '</div>' +
                        '<div><strong>Size:</strong> ' + (block.size || 0) + ' bytes</div>' +
                        '</div>' +
                        '</div>'
                    ).join('');
                    document.getElementById('blocks-list').innerHTML = blocksList || '<p class="text-gray-500">No blocks found</p>';
                } else {
                    document.getElementById('blocks-list').innerHTML = 
                        '<pre class="bg-gray-100 p-4 rounded text-sm">' + JSON.stringify(data, null, 2) + '</pre>';
                }
            } catch (error) {
                document.getElementById('blocks-list').innerHTML = 
                    '<div class="text-red-600">Failed to load blocks data: ' + error.message + '</div>';
            }
        }
        
        document.addEventListener('DOMContentLoaded', loadBlocksData);
        setInterval(loadBlocksData, 10000);
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}

// handleTransactionsPage serves the transactions monitoring page
func (wm *WebMonitor) handleTransactionsPage(w http.ResponseWriter, r *http.Request) {
	tmpl := `
<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Transactions Monitor - Shadowy Blockchain</title>
    <script src="https://cdn.tailwindcss.com"></script>
</head>
<body class="bg-gray-100 min-h-screen">
    <!-- Header -->
    <header class="bg-indigo-600 text-white shadow-lg">
        <div class="container mx-auto px-6 py-4">
            <h1 class="text-3xl font-bold">💸 Transactions Monitor</h1>
        </div>
    </header>

    <!-- Navigation -->
    <nav class="bg-white shadow-sm border-b">
        <div class="container mx-auto px-6">
            <div class="flex space-x-8">
                <a href="/dashboard" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Dashboard</a>
                <a href="/health" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Health</a>
                <a href="/mining" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Mining</a>
                <a href="/consensus" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Consensus</a>
                <a href="/blocks" class="py-4 px-2 text-gray-500 hover:text-indigo-600 transition-colors">Blocks</a>
                <a href="/transactions" class="py-4 px-2 border-b-2 border-indigo-500 text-indigo-600 font-medium">Transactions</a>
            </div>
        </div>
    </nav>

    <!-- Content -->
    <main class="container mx-auto px-6 py-8">
        <div class="space-y-6">
            <div class="bg-white rounded-lg shadow-md p-6">
                <h2 class="text-xl font-semibold mb-4">Mempool Statistics</h2>
                <div id="mempool-stats">Loading...</div>
            </div>
            
            <div class="bg-white rounded-lg shadow-md p-6">
                <h2 class="text-xl font-semibold mb-4">Pending Transactions</h2>
                <div id="transactions-list">Loading...</div>
            </div>
        </div>
    </main>

    <script>
        async function loadTransactionsData() {
            try {
                const response = await fetch('/api/transactions');
                const data = await response.json();
                
                // Display mempool stats
                if (data.stats) {
                    const statsHtml = Object.entries(data.stats).map(([key, value]) => 
                        '<div class="flex justify-between py-2 border-b">' +
                        '<span class="font-medium">' + key.replace(/_/g, ' ').replace(/\b\w/g, l => l.toUpperCase()) + ':</span>' +
                        '<span>' + value + '</span>' +
                        '</div>'
                    ).join('');
                    document.getElementById('mempool-stats').innerHTML = statsHtml || '<p class="text-gray-500">No stats available</p>';
                } else {
                    document.getElementById('mempool-stats').innerHTML = '<p class="text-gray-500">Stats not available</p>';
                }
                
                // Display transactions
                if (data.transactions && Array.isArray(data.transactions)) {
                    const txList = data.transactions.slice(0, 20).map(tx => 
                        '<div class="border-b border-gray-200 pb-3 mb-3 last:border-b-0">' +
                        '<div class="flex items-center justify-between mb-2">' +
                        '<code class="bg-gray-100 px-2 py-1 rounded text-sm">' + (tx.hash || 'N/A').substring(0, 16) + '...</code>' +
                        '<span class="text-sm text-gray-500">' + (tx.timestamp ? new Date(tx.timestamp).toLocaleString() : 'Unknown') + '</span>' +
                        '</div>' +
                        '<div class="text-sm text-gray-600">' +
                        '<span>From: ' + (tx.from || 'N/A').substring(0, 16) + '... → </span>' +
                        '<span>To: ' + (tx.to || 'N/A').substring(0, 16) + '...</span>' +
                        '<span class="ml-4">Amount: ' + (tx.amount || 0) + ' SHADOW</span>' +
                        '</div>' +
                        '</div>'
                    ).join('');
                    document.getElementById('transactions-list').innerHTML = txList || '<p class="text-gray-500">No pending transactions</p>';
                } else {
                    document.getElementById('transactions-list').innerHTML = 
                        '<pre class="bg-gray-100 p-4 rounded text-sm">' + JSON.stringify(data, null, 2) + '</pre>';
                }
            } catch (error) {
                document.getElementById('mempool-stats').innerHTML = 
                    '<div class="text-red-600">Failed to load mempool stats: ' + error.message + '</div>';
                document.getElementById('transactions-list').innerHTML = 
                    '<div class="text-red-600">Failed to load transactions: ' + error.message + '</div>';
            }
        }
        
        document.addEventListener('DOMContentLoaded', loadTransactionsData);
        setInterval(loadTransactionsData, 5000);
    </script>
</body>
</html>`

	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(tmpl))
}