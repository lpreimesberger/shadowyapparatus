package cmd

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// WebWalletSession represents an authenticated web wallet session
type WebWalletSession struct {
	SessionID string    `json:"session_id"`
	Address   string    `json:"address"`
	WalletName string   `json:"wallet_name"`
	CreatedAt time.Time `json:"created_at"`
	ExpiresAt time.Time `json:"expires_at"`
}

// PaymentRequest represents a payment request URI
type PaymentRequest struct {
	Address    string  `json:"address"`
	Amount     float64 `json:"amount"`
	Currency   string  `json:"currency"`
	Label      string  `json:"label,omitempty"`
	Message    string  `json:"message,omitempty"`
	URI        string  `json:"uri"`
}

// SendRequest represents a send transaction request
type SendRequest struct {
	ToAddress string  `json:"to_address"`
	Amount    float64 `json:"amount"`
	Fee       float64 `json:"fee,omitempty"`
	Message   string  `json:"message,omitempty"`
}

// WebWallet session storage (in production, use proper session storage)
var webWalletSessions = make(map[string]*WebWalletSession)

// getPasswordFromFile reads the password from ~/.shadowy/password.txt
func getPasswordFromFile() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	
	passwordPath := filepath.Join(homeDir, ".shadowy", "password.txt")
	
	// Create default password file if it doesn't exist
	if _, err := os.Stat(passwordPath); os.IsNotExist(err) {
		// Ensure directory exists
		if err := os.MkdirAll(filepath.Dir(passwordPath), 0700); err != nil {
			return "", fmt.Errorf("failed to create .shadowy directory: %w", err)
		}
		
		// Create default password
		defaultPassword := "shadow123"
		if err := os.WriteFile(passwordPath, []byte(defaultPassword), 0600); err != nil {
			return "", fmt.Errorf("failed to create password file: %w", err)
		}
		
		fmt.Printf("Created default password file at: %s\n", passwordPath)
		fmt.Printf("Default password: %s\n", defaultPassword)
		
		return defaultPassword, nil
	}
	
	passwordBytes, err := os.ReadFile(passwordPath)
	if err != nil {
		return "", fmt.Errorf("failed to read password file: %w", err)
	}
	
	return strings.TrimSpace(string(passwordBytes)), nil
}

// generateSessionID creates a unique session ID
func generateSessionID(address string) string {
	data := fmt.Sprintf("%s_%d", address, time.Now().UnixNano())
	hash := sha256.Sum256([]byte(data))
	return hex.EncodeToString(hash[:])[:32]
}

// validateSession checks if a session is valid
func validateSession(r *http.Request) (*WebWalletSession, bool) {
	sessionCookie, err := r.Cookie("shadow_session")
	if err != nil {
		return nil, false
	}
	
	session, exists := webWalletSessions[sessionCookie.Value]
	if !exists {
		return nil, false
	}
	
	if time.Now().After(session.ExpiresAt) {
		delete(webWalletSessions, sessionCookie.Value)
		return nil, false
	}
	
	return session, true
}

// handleWebWallet serves the main web wallet interface
func (sn *ShadowNode) handleWebWallet(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	
	// Check if user is authenticated
	if !authenticated {
		sn.serveLoginPage(w, r)
		return
	}
	
	// Serve wallet dashboard
	sn.serveWalletDashboard(w, r, session)
}

// serveLoginPage serves the login page
func (sn *ShadowNode) serveLoginPage(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html")
	
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Web Wallet - Login</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: linear-gradient(135deg, #2d1b4e 0%, #1a1a2e 100%);
            min-height: 100vh;
            display: flex;
            align-items: center;
            justify-content: center;
        }
        .container {
            background: #2d2d2d;
            padding: 2rem;
            border-radius: 10px;
            box-shadow: 0 10px 30px rgba(0,0,0,0.5);
            width: 100%;
            max-width: 400px;
            border: 1px solid #404040;
        }
        .logo {
            text-align: center;
            margin-bottom: 2rem;
        }
        .logo h1 {
            color: #8b5cf6;
            font-size: 2rem;
            margin-bottom: 0.5rem;
        }
        .logo p {
            color: #a0a0a0;
            font-size: 0.9rem;
        }
        .form-group {
            margin-bottom: 1.5rem;
        }
        label {
            display: block;
            margin-bottom: 0.5rem;
            color: #e0e0e0;
            font-weight: 500;
        }
        input, select {
            width: 100%;
            padding: 0.75rem;
            border: 2px solid #404040;
            border-radius: 5px;
            font-size: 1rem;
            transition: border-color 0.3s;
            background: #1a1a1a;
            color: #e0e0e0;
        }
        input:focus, select:focus {
            outline: none;
            border-color: #8b5cf6;
        }
        .btn {
            width: 100%;
            padding: 0.75rem;
            background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
            color: white;
            border: none;
            border-radius: 5px;
            font-size: 1rem;
            cursor: pointer;
            transition: transform 0.2s;
        }
        .btn:hover {
            transform: translateY(-2px);
        }
        .error {
            color: #e74c3c;
            margin-top: 1rem;
            text-align: center;
        }
        .info {
            background: #e8f4fd;
            border: 1px solid #bee5eb;
            border-radius: 5px;
            padding: 1rem;
            margin-bottom: 1rem;
            font-size: 0.9rem;
            color: #0c5460;
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="logo">
            <h1>🌘 Shadowy</h1>
            <p>Web Wallet</p>
        </div>
        
        <div class="info">
            <strong>First time setup:</strong><br>
            Password is stored in <code>~/.shadowy/password.txt</code><br>
            Default password: <code>shadow123</code>
        </div>
        
        <form id="loginForm">
            <div class="form-group">
                <label for="wallet">Select Wallet:</label>
                <select id="wallet" name="wallet" required>
                    <option value="">Loading wallets...</option>
                </select>
            </div>
            
            <div class="form-group">
                <label for="password">Password:</label>
                <input type="password" id="password" name="password" required placeholder="Enter your password">
            </div>
            
            <button type="submit" class="btn">Login</button>
            <div id="error" class="error"></div>
        </form>
    </div>

    <script>
        // Load wallets on page load
        fetch('/api/v1/wallet')
            .then(response => response.json())
            .then(wallets => {
                const select = document.getElementById('wallet');
                select.innerHTML = '<option value="">Select a wallet...</option>';
                wallets.forEach(wallet => {
                    const option = document.createElement('option');
                    option.value = wallet.name;
                    option.textContent = wallet.name + ' (' + wallet.address.substring(0, 16) + '...)';
                    select.appendChild(option);
                });
            })
            .catch(error => {
                document.getElementById('wallet').innerHTML = '<option value="">Error loading wallets</option>';
            });

        document.getElementById('loginForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const formData = new FormData(e.target);
            const data = {
                wallet: formData.get('wallet'),
                password: formData.get('password')
            };
            
            try {
                const response = await fetch('/wallet/login', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                
                if (response.ok) {
                    window.location.reload();
                } else {
                    const error = await response.text();
                    document.getElementById('error').textContent = error;
                }
            } catch (error) {
                document.getElementById('error').textContent = 'Login failed: ' + error.message;
            }
        });
    </script>
</body>
</html>`
	
	w.Write([]byte(html))
}

// serveWalletDashboard serves the main wallet dashboard
func (sn *ShadowNode) serveWalletDashboard(w http.ResponseWriter, r *http.Request, session *WebWalletSession) {
	w.Header().Set("Content-Type", "text/html")
	
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Web Wallet - Dashboard</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            min-height: 100vh;
        }
        .header {
            background: linear-gradient(135deg, #2d1b4e 0%, #1a1a2e 100%);
            color: white;
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid #333;
        }
        .logo {
            font-size: 1.5rem;
            font-weight: bold;
        }
        .user-info {
            display: flex;
            align-items: center;
            gap: 1rem;
        }
        .logout-btn {
            background: rgba(255,255,255,0.2);
            border: 1px solid rgba(255,255,255,0.3);
            color: white;
            padding: 0.5rem 1rem;
            border-radius: 5px;
            cursor: pointer;
            transition: background 0.3s;
        }
        .logout-btn:hover {
            background: rgba(255,255,255,0.3);
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 1rem;
        }
        .stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 0.75rem;
            margin-bottom: 1.5rem;
        }
        .stat-card {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 0.75rem;
            text-align: center;
        }
        .stat-value {
            font-size: 1.2rem;
            font-weight: bold;
            color: #8b5cf6;
            margin-bottom: 0.25rem;
        }
        .stat-label {
            color: #a0a0a0;
            font-size: 0.8rem;
        }
        .countdown {
            font-family: monospace;
            color: #fbbf24;
        }
        .clickable {
            cursor: pointer;
            transition: transform 0.2s, box-shadow 0.2s;
        }
        .clickable:hover {
            transform: translateY(-2px);
            box-shadow: 0 6px 20px rgba(139, 92, 246, 0.3);
        }
        
        /* Balance Section */
        .balance-section {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 10px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
            text-align: center;
        }
        .balance-amount {
            font-size: 2.5rem;
            font-weight: bold;
            color: #8b5cf6;
            margin-bottom: 0.5rem;
        }
        .balance-label {
            color: #a0a0a0;
            margin-bottom: 1rem;
        }
        .address-display {
            background: #1a1a1a;
            padding: 1rem;
            border-radius: 5px;
            font-family: monospace;
            word-break: break-all;
            border: 1px solid #404040;
            color: #a0a0a0;
            cursor: pointer;
            transition: background 0.3s, border-color 0.3s;
            position: relative;
        }
        .address-display:hover {
            background: #2a2a2a;
            border-color: #8b5cf6;
        }
        .address-display:after {
            content: "📋 Click to copy";
            position: absolute;
            top: -30px;
            left: 50%;
            transform: translateX(-50%);
            background: #2d2d2d;
            padding: 0.25rem 0.5rem;
            border-radius: 3px;
            font-size: 0.8rem;
            white-space: nowrap;
            opacity: 0;
            transition: opacity 0.3s;
            pointer-events: none;
        }
        .address-display:hover:after {
            opacity: 1;
        }

        /* Tab System */
        .tab-container {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 10px;
            overflow: hidden;
        }
        .tab-header {
            display: flex;
            background: #1a1a1a;
            border-bottom: 1px solid #404040;
        }
        .tab-button {
            flex: 1;
            padding: 1rem;
            background: none;
            border: none;
            color: #a0a0a0;
            cursor: pointer;
            transition: all 0.3s;
            font-size: 0.9rem;
            position: relative;
        }
        .tab-button:hover {
            background: #2a2a2a;
            color: #e0e0e0;
        }
        .tab-button.active {
            color: #8b5cf6;
            background: #2d2d2d;
        }
        .tab-button.active:after {
            content: '';
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            height: 2px;
            background: #8b5cf6;
        }
        .tab-content {
            padding: 1.5rem;
            display: none;
        }
        .tab-content.active {
            display: block;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        label {
            display: block;
            margin-bottom: 0.5rem;
            color: #e0e0e0;
            font-weight: 500;
        }
        input, textarea, select {
            width: 100%;
            padding: 0.75rem;
            border: 2px solid #404040;
            border-radius: 5px;
            font-size: 1rem;
            transition: border-color 0.3s;
            background: #1a1a1a;
            color: #e0e0e0;
        }
        input:focus, textarea:focus, select:focus {
            outline: none;
            border-color: #8b5cf6;
        }
        .btn {
            padding: 0.75rem 1.5rem;
            background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
            color: white;
            border: none;
            border-radius: 5px;
            font-size: 1rem;
            cursor: pointer;
            transition: transform 0.2s;
        }
        .btn:hover {
            transform: translateY(-2px);
        }
        .btn-secondary {
            background: #6c757d;
        }
        .transactions-table, .blocks-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        .transactions-table th, .transactions-table td,
        .blocks-table th, .blocks-table td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid #404040;
        }
        .transactions-table th, .blocks-table th {
            background: #1a1a1a;
            font-weight: 600;
            color: #e0e0e0;
        }
        .amount-positive {
            color: #28a745;
            font-weight: bold;
        }
        .amount-negative {
            color: #dc3545;
            font-weight: bold;
        }
        .loading {
            text-align: center;
            padding: 2rem;
            color: #666;
        }
        .error {
            color: #e74c3c;
            background: rgba(231, 76, 60, 0.1);
            padding: 1rem;
            border-radius: 5px;
            margin: 1rem 0;
            border: 1px solid rgba(231, 76, 60, 0.3);
        }
        .success {
            color: #27ae60;
            background: rgba(39, 174, 96, 0.1);
            padding: 1rem;
            border-radius: 5px;
            margin: 1rem 0;
            border: 1px solid rgba(39, 174, 96, 0.3);
        }
        .uri-display {
            background: #1a1a1a;
            padding: 1rem;
            border-radius: 5px;
            font-family: monospace;
            word-break: break-all;
            border: 1px solid #404040;
            margin-top: 1rem;
            color: #a0a0a0;
        }
        .copy-btn {
            margin-left: 0.5rem;
            padding: 0.25rem 0.5rem;
            font-size: 0.8rem;
        }
        .block-row {
            cursor: pointer;
            transition: background 0.2s;
        }
        .block-row:hover {
            background: #2a2a2a;
        }
        .block-detail {
            background: #1a1a1a;
            margin-top: 1rem;
            padding: 1rem;
            border-radius: 5px;
            border: 1px solid #404040;
            display: none;
        }
        .block-detail.active {
            display: block;
        }
        @media (max-width: 768px) {
            .stats-grid {
                grid-template-columns: repeat(auto-fit, minmax(150px, 1fr));
            }
            .container {
                padding: 0.5rem;
            }
            .tab-header {
                flex-wrap: wrap;
            }
            .tab-button {
                font-size: 0.8rem;
                padding: 0.75rem 0.5rem;
            }
        }
        
        /* Footer styles */
        .footer {
            text-align: center;
            padding: 1rem;
            margin-top: 2rem;
            border-top: 1px solid #404040;
            color: #666;
            font-size: 0.8rem;
        }
        .version-info {
            margin-bottom: 0.5rem;
        }
        .version-info span {
            margin: 0 0.5rem;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="logo">🌘 Shadowy Web Wallet</div>
        <div class="user-info">
            <span>` + session.WalletName + `</span>
            <button class="logout-btn" onclick="logout()">Logout</button>
        </div>
    </div>

    <div class="container">
        <!-- Network & Farm Statistics -->
        <div class="stats-grid">
            <div class="stat-card">
                <div class="stat-value" id="farmSize">Loading...</div>
                <div class="stat-label">🌾 My Farm Size</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="networkSize">Loading...</div>
                <div class="stat-label">🌐 Network Size</div>
            </div>
            <div class="stat-card clickable" onclick="viewNetworkPeers()">
                <div class="stat-value" id="networkNodes">Loading...</div>
                <div class="stat-label">🔗 Network Nodes</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="currentBlock">Loading...</div>
                <div class="stat-label">📦 Current Block</div>
            </div>
            <div class="stat-card">
                <div class="stat-value countdown" id="blockCountdown">--:--</div>
                <div class="stat-label">⏱️ Next Block In</div>
            </div>
            <div class="stat-card clickable" onclick="viewMempool()">
                <div class="stat-value" id="mempoolTxs">Loading...</div>
                <div class="stat-label">📋 Mempool Txs</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="totalSupply">Loading...</div>
                <div class="stat-label">💎 Total Supply</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="nextHalving">Loading...</div>
                <div class="stat-label">📉 Next Halving</div>
            </div>
        </div>

        <!-- Balance Section -->
        <div class="balance-section">
            <div class="balance-amount" id="balanceAmount">Loading...</div>
            <div class="balance-label">SHADOW</div>
            <div class="address-display" id="walletAddress" onclick="copyAddress()" title="Click to copy address">` + session.Address + `</div>
        </div>

        <!-- Tabbed Interface -->
        <div class="tab-container">
            <div class="tab-header">
                <button class="tab-button active" onclick="switchTab('request')">📥 Request Payment</button>
                <button class="tab-button" onclick="switchTab('send')">📤 Send Payment</button>
                <button class="tab-button" onclick="switchTab('transactions')">📊 Transactions</button>
                <button class="tab-button" onclick="switchTab('blocks')">🗂️ Block Browser</button>
            </div>

            <!-- Request Payment Tab -->
            <div id="request-tab" class="tab-content active">
                <form id="requestForm">
                    <div class="form-group">
                        <label for="requestAmount">Amount (SHADOW):</label>
                        <input type="number" id="requestAmount" name="requestAmount" step="0.00000001" min="0" required>
                    </div>
                    <div class="form-group">
                        <label for="requestLabel">Label (optional):</label>
                        <input type="text" id="requestLabel" name="requestLabel" placeholder="Payment for...">
                    </div>
                    <div class="form-group">
                        <label for="requestMessage">Message (optional):</label>
                        <textarea id="requestMessage" name="requestMessage" rows="3" placeholder="Additional message..."></textarea>
                    </div>
                    <button type="submit" class="btn">Generate Payment URI</button>
                </form>
                <div id="requestResult"></div>
            </div>

            <!-- Send Payment Tab -->
            <div id="send-tab" class="tab-content">
                <form id="sendForm">
                    <div class="form-group">
                        <label for="sendAddress">To Address:</label>
                        <input type="text" id="sendAddress" name="sendAddress" placeholder="S..." required>
                    </div>
                    <div class="form-group">
                        <label for="sendAmount">Amount (SHADOW):</label>
                        <input type="number" id="sendAmount" name="sendAmount" step="0.00000001" min="0" required>
                    </div>
                    <div class="form-group">
                        <label for="sendFee">Transaction Fee (SHADOW):</label>
                        <input type="number" id="sendFee" name="sendFee" step="0.00000001" min="0" value="0.1" placeholder="0.1">
                    </div>
                    <div class="form-group">
                        <label for="sendMessage">Message (optional):</label>
                        <input type="text" id="sendMessage" name="sendMessage" placeholder="Payment message...">
                    </div>
                    <button type="submit" class="btn">Send Payment</button>
                </form>
                <div id="sendResult"></div>
            </div>

            <!-- Transactions Tab -->
            <div id="transactions-tab" class="tab-content">
                <div id="transactionsContainer">
                    <div class="loading">Loading transactions...</div>
                </div>
            </div>

            <!-- Blocks Tab -->
            <div id="blocks-tab" class="tab-content">
                <div id="blocksContainer">
                    <div class="loading">Loading recent blocks...</div>
                </div>
            </div>
        </div>
        
        <!-- Footer -->
        <div class="footer">
            <div class="version-info">
                <span id="nodeVersion">Loading...</span>
                <span>•</span>
                <span id="buildTime">Loading...</span>
            </div>
            <div>Shadowy Blockchain Web Wallet</div>
        </div>
    </div>

    <script>
        let walletData = null;
        let lastBlockTime = null;
        let blockInterval = 30; // 30 seconds in dev mode
        let countdownInterval = null;
        let currentActiveTab = 'request';
        let recentBlocks = [];

        // Tab switching functionality
        function switchTab(tabName) {
            // Hide all tab contents
            document.querySelectorAll('.tab-content').forEach(content => {
                content.classList.remove('active');
            });
            
            // Remove active class from all tab buttons
            document.querySelectorAll('.tab-button').forEach(button => {
                button.classList.remove('active');
            });
            
            // Show selected tab content
            document.getElementById(tabName + '-tab').classList.add('active');
            
            // Add active class to selected tab button
            event.target.classList.add('active');
            
            currentActiveTab = tabName;
            
            // Load data for specific tabs
            if (tabName === 'transactions') {
                loadTransactions();
            } else if (tabName === 'blocks') {
                loadRecentBlocks();
            }
        }

        // Copy address functionality
        function copyAddress() {
            const address = document.getElementById('walletAddress').textContent;
            navigator.clipboard.writeText(address).then(() => {
                const addressElement = document.getElementById('walletAddress');
                const originalText = addressElement.textContent;
                addressElement.textContent = '✓ Copied!';
                addressElement.style.color = '#28a745';
                setTimeout(() => {
                    addressElement.textContent = originalText;
                    addressElement.style.color = '#a0a0a0';
                }, 2000);
            });
        }

        // Load wallet data on page load
        async function loadWalletData() {
            try {
                // Get the current wallet address from the UI
                const address = document.getElementById('walletAddress').textContent;
                const response = await fetch('/wallet/balance?address=' + encodeURIComponent(address));
                walletData = await response.json();
                
                document.getElementById('balanceAmount').textContent = 
                    walletData.confirmed_shadow.toFixed(8);
                
                // Load transactions only if we're on the transactions tab
                if (currentActiveTab === 'transactions') {
                    loadTransactions();
                }
            } catch (error) {
                document.getElementById('balanceAmount').textContent = 'Error loading';
                console.error('Error loading wallet data:', error);
            }
        }

        // Load farm and network statistics
        async function loadNetworkStats() {
            try {
                // Load farming stats
                const farmingResponse = await fetch('/api/v1/farming');
                const farmingData = await farmingResponse.json();
                
                // Load blockchain stats  
                const blockchainResponse = await fetch('/api/v1/blockchain');
                const blockchainData = await blockchainResponse.json();
                
                // Load mempool stats
                const mempoolResponse = await fetch('/api/v1/mempool');
                const mempoolData = await mempoolResponse.json();
                
                // Load network stats (tokenomics)
                const networkResponse = await fetch('/api/v1/tokenomics');
                const networkData = await networkResponse.json();
                
                // Load consensus stats for peer count
                let consensusData = { peer_count: 0 };
                try {
                    const consensusResponse = await fetch('/api/v1/consensus');
                    consensusData = await consensusResponse.json();
                } catch (consensusError) {
                    console.warn('Consensus API not available:', consensusError);
                }
                
                // Update farm size (k32 plot: ~101.4 GiB per 1048576 keys)
                const farmSizeGiB = (farmingData.total_keys * 101.4 / 1048576).toFixed(2);
                document.getElementById('farmSize').textContent = farmSizeGiB + ' GiB';
                
                // Update network size (same as farm size for now since single node)
                document.getElementById('networkSize').textContent = farmSizeGiB + ' GiB';
                
                // Update current block
                document.getElementById('currentBlock').textContent = '#' + blockchainData.tip_height;
                
                // Update new statistics
                document.getElementById('mempoolTxs').textContent = mempoolData.transaction_count;
                document.getElementById('totalSupply').textContent = 
                    (networkData.total_supply_shadow || 0).toFixed(0) + ' SHADOW';
                document.getElementById('nextHalving').textContent = 
                    (networkData.blocks_until_halving || 0) + ' blocks';
                
                // Update network nodes count with actual peer count
                document.getElementById('networkNodes').textContent = consensusData.peer_count || 0;
                
                // Start countdown if we have block data
                if (blockchainData.last_block_time) {
                    lastBlockTime = new Date(blockchainData.last_block_time);
                    startBlockCountdown();
                }
                
            } catch (error) {
                console.error('Error loading network stats:', error);
                document.getElementById('farmSize').textContent = 'Error';
                document.getElementById('networkSize').textContent = 'Error';
                document.getElementById('currentBlock').textContent = 'Error';
                document.getElementById('mempoolTxs').textContent = 'Error';
                document.getElementById('totalSupply').textContent = 'Error';
                document.getElementById('nextHalving').textContent = 'Error';
                document.getElementById('networkNodes').textContent = 'Error';
            }
        }

        // Start block countdown timer
        function startBlockCountdown() {
            if (countdownInterval) clearInterval(countdownInterval);
            
            countdownInterval = setInterval(() => {
                if (!lastBlockTime) return;
                
                const now = new Date();
                const nextBlockTime = new Date(lastBlockTime.getTime() + (blockInterval * 1000));
                const timeLeft = nextBlockTime - now;
                
                if (timeLeft <= 0) {
                    document.getElementById('blockCountdown').textContent = '00:00';
                    // Refresh stats to check for new block
                    loadNetworkStats();
                } else {
                    const minutes = Math.floor(timeLeft / 60000);
                    const seconds = Math.floor((timeLeft % 60000) / 1000);
                    document.getElementById('blockCountdown').textContent = 
                        minutes.toString().padStart(2, '0') + ':' + seconds.toString().padStart(2, '0');
                }
            }, 1000);
        }

        // Load recent transactions
        async function loadTransactions() {
            try {
                const response = await fetch('/wallet/transactions');
                const transactions = await response.json();
                
                const container = document.getElementById('transactionsContainer');
                
                if (transactions.length === 0) {
                    container.innerHTML = '<p>No transactions found.</p>';
                    return;
                }
                
                let html = '<table class="transactions-table">';
                html += '<thead><tr><th>Hash</th><th>Type</th><th>Amount</th><th>Date</th></tr></thead><tbody>';
                
                transactions.forEach(tx => {
                    const amountClass = tx.amount_satoshi >= 0 ? 'amount-positive' : 'amount-negative';
                    const amountText = (tx.amount_satoshi / 100000000).toFixed(8);
                    const hashShort = tx.tx_hash.substring(0, 16) + '...';
                    const date = new Date(tx.timestamp).toLocaleDateString();
                    
                    html += '<tr>';
                    html += '<td><code>' + hashShort + '</code></td>';
                    html += '<td>' + tx.type + '</td>';
                    html += '<td class="' + amountClass + '">' + (tx.amount_satoshi >= 0 ? '+' : '') + amountText + '</td>';
                    html += '<td>' + date + '</td>';
                    html += '</tr>';
                });
                
                html += '</tbody></table>';
                container.innerHTML = html;
            } catch (error) {
                document.getElementById('transactionsContainer').innerHTML = 
                    '<div class="error">Error loading transactions: ' + error.message + '</div>';
            }
        }

        // Load recent blocks
        async function loadRecentBlocks() {
            try {
                const response = await fetch('/api/v1/blockchain/recent?limit=10');
                const data = await response.json();
                recentBlocks = data.blocks || [];
                
                const container = document.getElementById('blocksContainer');
                
                if (recentBlocks.length === 0) {
                    container.innerHTML = '<p>No blocks found.</p>';
                    return;
                }
                
                let html = '<table class="blocks-table">';
                html += '<thead><tr><th>Height</th><th>Hash</th><th>Transactions</th><th>Time</th><th>Farmer</th></tr></thead><tbody>';
                
                recentBlocks.forEach(block => {
                    // Access block properties from header and body
                    const header = block.header || {};
                    const body = block.body || {};
                    
                    const blockHash = header.hash || 'Unknown';
                    const hashShort = blockHash.substring(0, 16) + '...';
                    const farmerShort = header.farmer_address ? (header.farmer_address.substring(0, 16) + '...') : 'Unknown';
                    const date = new Date(header.timestamp).toLocaleString();
                    const txCount = body.transactions ? body.transactions.length : 0;
                    
                    html += '<tr class="block-row" onclick="toggleBlockDetail(\'' + blockHash + '\')">';
                    html += '<td><strong>#' + header.height + '</strong></td>';
                    html += '<td><code>' + hashShort + '</code></td>';
                    html += '<td>' + txCount + '</td>';
                    html += '<td>' + date + '</td>';
                    html += '<td><code>' + farmerShort + '</code></td>';
                    html += '</tr>';
                    html += '<tr><td colspan="5"><div id="block-detail-' + blockHash + '" class="block-detail"></div></td></tr>';
                });
                
                html += '</tbody></table>';
                container.innerHTML = html;
            } catch (error) {
                document.getElementById('blocksContainer').innerHTML = 
                    '<div class="error">Error loading blocks: ' + error.message + '</div>';
            }
        }

        // Toggle block detail view
        async function toggleBlockDetail(blockHash) {
            const detailDiv = document.getElementById('block-detail-' + blockHash);
            
            if (detailDiv.classList.contains('active')) {
                detailDiv.classList.remove('active');
                return;
            }
            
            // Close all other details first
            document.querySelectorAll('.block-detail.active').forEach(detail => {
                detail.classList.remove('active');
            });
            
            try {
                // Load full block details
                const response = await fetch('/api/v1/blockchain/block/' + blockHash);
                const block = await response.json();
                
                let html = '<h4>Block Details</h4>';
                html += '<div style="display: grid; grid-template-columns: 1fr 1fr; gap: 1rem; margin-bottom: 1rem;">';
                html += '<div><strong>Height:</strong> #' + block.height + '</div>';
                html += '<div><strong>Hash:</strong> <code>' + block.hash + '</code></div>';
                html += '<div><strong>Previous Hash:</strong> <code>' + (block.previous_hash || 'None').substring(0, 32) + '...</code></div>';
                html += '<div><strong>Timestamp:</strong> ' + new Date(block.timestamp).toLocaleString() + '</div>';
                html += '<div><strong>Size:</strong> ' + (block.size || 'Unknown') + ' bytes</div>';
                html += '<div><strong>Farmer:</strong> <code>' + (block.farmer_address || 'Unknown') + '</code></div>';
                html += '</div>';
                
                if (block.transactions && block.transactions.length > 0) {
                    html += '<h5>Transactions (' + block.transactions.length + ')</h5>';
                    html += '<table class="transactions-table" style="font-size: 0.9rem;">';
                    html += '<thead><tr><th>Hash</th><th>Type</th><th>Amount</th></tr></thead><tbody>';
                    
                    block.transactions.forEach(tx => {
                        // Parse the nested transaction structure
                        let actualTx = tx;
                        if (tx.transaction) {
                            try {
                                actualTx = typeof tx.transaction === 'string' ? JSON.parse(tx.transaction) : tx.transaction;
                            } catch (e) {
                                actualTx = tx.transaction;
                            }
                        }
                        
                        const txHashShort = tx.tx_hash ? (tx.tx_hash.substring(0, 16) + '...') : 'Unknown';
                        const txType = actualTx.inputs && actualTx.inputs.length === 0 ? 'Coinbase' : 'Transfer';
                        const amount = actualTx.outputs ? actualTx.outputs.reduce((sum, output) => sum + (output.value || 0), 0) : 0;
                        const amountShadow = (amount / 100000000).toFixed(8);
                        
                        html += '<tr>';
                        html += '<td><code>' + txHashShort + '</code></td>';
                        html += '<td>' + txType + '</td>';
                        html += '<td>' + amountShadow + ' SHADOW</td>';
                        html += '</tr>';
                    });
                    
                    html += '</tbody></table>';
                } else {
                    html += '<p>No transactions in this block.</p>';
                }
                
                detailDiv.innerHTML = html;
                detailDiv.classList.add('active');
            } catch (error) {
                detailDiv.innerHTML = '<div class="error">Error loading block details: ' + error.message + '</div>';
                detailDiv.classList.add('active');
            }
        }

        // Handle payment request form
        document.getElementById('requestForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const formData = new FormData(e.target);
            const data = {
                amount: parseFloat(formData.get('requestAmount')),
                label: formData.get('requestLabel') || '',
                message: formData.get('requestMessage') || ''
            };
            
            try {
                const response = await fetch('/wallet/request', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                
                if (response.ok) {
                    const result = await response.json();
                    document.getElementById('requestResult').innerHTML = 
                        '<div class="success">Payment URI generated!</div>' +
                        '<div class="uri-display">' + result.uri + 
                        '<button class="btn copy-btn" onclick="copyToClipboard(\'' + result.uri + '\')">Copy</button></div>';
                } else {
                    const error = await response.text();
                    document.getElementById('requestResult').innerHTML = 
                        '<div class="error">Error: ' + error + '</div>';
                }
            } catch (error) {
                document.getElementById('requestResult').innerHTML = 
                    '<div class="error">Error: ' + error.message + '</div>';
            }
        });

        // Handle send payment form
        document.getElementById('sendForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const formData = new FormData(e.target);
            const data = {
                to_address: formData.get('sendAddress'),
                amount: parseFloat(formData.get('sendAmount')),
                fee: parseFloat(formData.get('sendFee')) || 0.1,
                message: formData.get('sendMessage') || ''
            };
            
            try {
                const response = await fetch('/wallet/send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });
                
                if (response.ok) {
                    const result = await response.json();
                    document.getElementById('sendResult').innerHTML = 
                        '<div class="success">Transaction submitted! Hash: ' + result.tx_hash + '</div>';
                    
                    // Reload wallet data
                    setTimeout(loadWalletData, 2000);
                    
                    // Clear form
                    e.target.reset();
                    // Reset fee field to default
                    document.getElementById('sendFee').value = '0.1';
                } else {
                    const error = await response.text();
                    document.getElementById('sendResult').innerHTML = 
                        '<div class="error">Error: ' + error + '</div>';
                }
            } catch (error) {
                document.getElementById('sendResult').innerHTML = 
                    '<div class="error">Error: ' + error.message + '</div>';
            }
        });

        // Copy to clipboard function
        function copyToClipboard(text) {
            navigator.clipboard.writeText(text).then(() => {
                alert('Copied to clipboard!');
            });
        }

        // Logout function
        async function logout() {
            try {
                await fetch('/wallet/logout', { method: 'POST' });
                window.location.reload();
            } catch (error) {
                console.error('Logout error:', error);
                window.location.reload();
            }
        }

        // View mempool function
        function viewMempool() {
            window.open('/wallet/mempool', '_blank');
        }
        
        // View network peers function
        function viewNetworkPeers() {
            window.open('/wallet/peers', '_blank');
        }
        
        // Load version information
        async function loadVersionInfo() {
            try {
                const response = await fetch('/api/v1/version');
                const versionData = await response.json();
                
                document.getElementById('nodeVersion').textContent = versionData.short_version || 'v0.0';
                
                if (versionData.build_time && versionData.build_time !== 'unknown') {
                    const buildDate = new Date(versionData.build_time);
                    document.getElementById('buildTime').textContent = 
                        'Built ' + buildDate.toLocaleDateString() + ' ' + buildDate.toLocaleTimeString();
                } else {
                    document.getElementById('buildTime').textContent = 'Development Build';
                }
            } catch (error) {
                console.error('Error loading version info:', error);
                document.getElementById('nodeVersion').textContent = 'v0.0';
                document.getElementById('buildTime').textContent = 'Version unknown';
            }
        }
        
        // Initialize wallet on page load
        loadWalletData();
        loadNetworkStats();
        loadVersionInfo();
        
        // Refresh balance every 30 seconds
        setInterval(loadWalletData, 30000);
        
        // Refresh network stats every 10 seconds
        setInterval(loadNetworkStats, 10000);
    </script>
</body>
</html>`
	
	w.Write([]byte(html))
}

// handleWebWalletLogin handles login authentication
func (sn *ShadowNode) handleWebWalletLogin(w http.ResponseWriter, r *http.Request) {
	var loginData struct {
		Wallet   string `json:"wallet"`
		Password string `json:"password"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&loginData); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Verify password
	expectedPassword, err := getPasswordFromFile()
	if err != nil {
		http.Error(w, "Password verification failed", http.StatusInternalServerError)
		return
	}
	
	if loginData.Password != expectedPassword {
		http.Error(w, "Invalid password", http.StatusUnauthorized)
		return
	}
	
	// Load wallet to get address
	wallet, err := loadWallet(loginData.Wallet)
	if err != nil {
		http.Error(w, "Wallet not found", http.StatusNotFound)
		return
	}
	
	// Create session
	sessionID := generateSessionID(wallet.Address)
	session := &WebWalletSession{
		SessionID:  sessionID,
		Address:    wallet.Address,
		WalletName: wallet.Name,
		CreatedAt:  time.Now(),
		ExpiresAt:  time.Now().Add(24 * time.Hour), // 24 hour session
	}
	
	webWalletSessions[sessionID] = session
	
	// Set session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "shadow_session",
		Value:    sessionID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false, // Set to true in production with HTTPS
		SameSite: http.SameSiteLaxMode,
		Expires:  session.ExpiresAt,
	})
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Login successful"))
}

// handleWebWalletLogout handles logout
func (sn *ShadowNode) handleWebWalletLogout(w http.ResponseWriter, r *http.Request) {
	sessionCookie, err := r.Cookie("shadow_session")
	if err == nil {
		delete(webWalletSessions, sessionCookie.Value)
	}
	
	// Clear session cookie
	http.SetCookie(w, &http.Cookie{
		Name:     "shadow_session",
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Expires:  time.Unix(0, 0),
	})
	
	w.WriteHeader(http.StatusOK)
	w.Write([]byte("Logout successful"))
}

// handleWebWalletBalance returns wallet balance
func (sn *ShadowNode) handleWebWalletBalance(w http.ResponseWriter, r *http.Request) {
	// Check for optional address parameter (for public balance queries)
	queryAddress := r.URL.Query().Get("address")
	
	var targetAddress string
	if queryAddress != "" {
		// Validate the provided address
		if !IsValidAddress(queryAddress) {
			http.Error(w, "Invalid address format", http.StatusBadRequest)
			return
		}
		targetAddress = queryAddress
	} else {
		// Use authenticated session address
		session, authenticated := validateSession(r)
		if !authenticated {
			http.Error(w, "Not authenticated", http.StatusUnauthorized)
			return
		}
		targetAddress = session.Address
	}
	
	balance, err := calculateWalletBalance(targetAddress)
	if err != nil {
		http.Error(w, "Failed to calculate balance", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(balance)
}

// handleWebWalletRequest generates a payment request URI
func (sn *ShadowNode) handleWebWalletRequest(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	
	var requestData struct {
		Amount  float64 `json:"amount"`
		Label   string  `json:"label"`
		Message string  `json:"message"`
	}
	
	if err := json.NewDecoder(r.Body).Decode(&requestData); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Validate amount
	if requestData.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}
	
	// Generate payment URI (using standard format similar to Bitcoin)
	uri := fmt.Sprintf("shadow:%s?amount=%.8f&currency=SHADOW", 
		session.Address, requestData.Amount)
	
	if requestData.Label != "" {
		uri += "&label=" + requestData.Label
	}
	
	if requestData.Message != "" {
		uri += "&message=" + requestData.Message
	}
	
	paymentRequest := PaymentRequest{
		Address:  session.Address,
		Amount:   requestData.Amount,
		Currency: "SHADOW",
		Label:    requestData.Label,
		Message:  requestData.Message,
		URI:      uri,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(paymentRequest)
}

// handleWebWalletSend handles sending transactions
func (sn *ShadowNode) handleWebWalletSend(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	
	var sendData SendRequest
	if err := json.NewDecoder(r.Body).Decode(&sendData); err != nil {
		http.Error(w, "Invalid request", http.StatusBadRequest)
		return
	}
	
	// Validate address format
	if !IsValidAddress(sendData.ToAddress) {
		http.Error(w, "Invalid destination address format", http.StatusBadRequest)
		return
	}
	
	// Validate amount
	if sendData.Amount <= 0 {
		http.Error(w, "Amount must be positive", http.StatusBadRequest)
		return
	}
	
	// Set default fee if not provided
	if sendData.Fee <= 0 {
		sendData.Fee = 0.1 // Default fee of 0.1 SHADOW
	}
	
	// Simplified balance check - assume sufficient balance for now
	// TODO: Implement proper balance calculation without blocking
	balance := &WalletBalance{
		Address:          session.Address,
		ConfirmedBalance: 10000 * uint64(SatoshisPerShadow), // Assume 10,000 SHADOW for testing
		ConfirmedShadow:  10000.0, // 10,000 SHADOW
	}
	
	// Check if user has sufficient balance including fee
	totalRequired := sendData.Amount + sendData.Fee
	if totalRequired > balance.ConfirmedShadow {
		http.Error(w, fmt.Sprintf("Insufficient balance: need %.8f SHADOW (%.8f + %.8f fee), have %.8f SHADOW", 
			totalRequired, sendData.Amount, sendData.Fee, balance.ConfirmedShadow), http.StatusBadRequest)
		return
	}
	
	// Load the wallet to get private key for signing
	wallet, err := loadWallet(session.WalletName)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to load wallet '%s': %v", session.WalletName, err), http.StatusInternalServerError)
		return
	}
	
	// Validate wallet data before signing
	if wallet == nil {
		http.Error(w, "Wallet is nil", http.StatusInternalServerError)
		return
	}
	if wallet.PrivateKey == "" {
		http.Error(w, "Wallet private key is empty - wallet may be corrupted", http.StatusInternalServerError)
		return
	}
	
	// Convert amount and fee to satoshis
	amountSatoshis := uint64(sendData.Amount * float64(SatoshisPerShadow))
	// feeSatoshis := uint64(sendData.Fee * float64(SatoshisPerShadow)) // TODO: Use in proper UTXO implementation
	
	// Create transaction
	tx := NewTransaction()
	
	// Add output for the recipient
	tx.AddOutput(sendData.ToAddress, amountSatoshis)
	
	// For now, create a simplified transaction without proper UTXO tracking
	// In a real implementation, you would need to:
	// 1. Find unspent outputs (UTXOs) for the sender
	// 2. Add them as inputs
	// 3. Calculate change and add change output if needed
	// 4. Calculate and add appropriate fees
	
	// Add a placeholder input with a valid hash format to prevent mempool crashes
	// This is a temporary solution until proper UTXO tracking is implemented
	placeholderTxHash := "0000000000000000000000000000000000000000000000000000000000000000"
	tx.AddInput(placeholderTxHash, 0)
	
	// Sign the transaction
	signedTx, err := SignTransactionWithWallet(tx, wallet)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
		return
	}
	
	// Submit to mempool
	if sn.mempool != nil {
		err = sn.mempool.AddTransaction(signedTx, SourceAPI)
		if err != nil {
			http.Error(w, fmt.Sprintf("Failed to submit transaction: %v", err), http.StatusBadRequest)
			return
		}
	}
	
	response := map[string]interface{}{
		"tx_hash": signedTx.TxHash,
		"status":  "submitted",
		"message": "Transaction submitted to mempool",
		"amount":  sendData.Amount,
		"fee":     sendData.Fee,
		"total":   sendData.Amount + sendData.Fee,
		"to_address": sendData.ToAddress,
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleWebWalletSendRaw handles sending pre-signed transactions
func (sn *ShadowNode) handleWebWalletSendRaw(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	
	var signedTx SignedTransaction
	if err := json.NewDecoder(r.Body).Decode(&signedTx); err != nil {
		http.Error(w, "Invalid signed transaction format", http.StatusBadRequest)
		return
	}
	
	// Verify the signature and validate the transaction
	tx, err := VerifySignedTransaction(&signedTx)
	if err != nil {
		http.Error(w, fmt.Sprintf("Transaction verification failed: %v", err), http.StatusBadRequest)
		return
	}
	
	// Additional validation: check if transaction is well-formed
	if err := tx.IsValid(); err != nil {
		http.Error(w, fmt.Sprintf("Transaction validation failed: %v", err), http.StatusBadRequest)
		return
	}
	
	// Check that the transaction has valid inputs and outputs
	if len(tx.Outputs) == 0 {
		http.Error(w, "Transaction must have at least one output", http.StatusBadRequest)
		return
	}
	
	// Validate all output addresses
	for i, output := range tx.Outputs {
		if !IsValidAddress(output.Address) {
			http.Error(w, fmt.Sprintf("Invalid address in output %d: %s", i, output.Address), http.StatusBadRequest)
			return
		}
		if output.Value == 0 {
			http.Error(w, fmt.Sprintf("Output %d has zero value", i), http.StatusBadRequest)
			return
		}
	}
	
	// Check that the signer has permission to spend (basic check)
	// In a full implementation, you would verify that the signer owns the UTXOs being spent
	signerPubKeyBytes, err := hex.DecodeString(signedTx.SignerKey)
	if err != nil {
		http.Error(w, "Invalid signer public key format", http.StatusBadRequest)
		return
	}
	
	signerAddress := DeriveAddress(signerPubKeyBytes)
	if signerAddress != session.Address {
		// Allow transactions signed by other addresses (for advanced use cases)
		// but log it for security monitoring
		fmt.Printf("Warning: Transaction signed by %s but session is for %s\n", 
			signerAddress, session.Address)
	}
	
	// Submit to mempool
	if sn.mempool == nil {
		http.Error(w, "Mempool not available", http.StatusServiceUnavailable)
		return
	}
	
	err = sn.mempool.AddTransaction(&signedTx, SourceAPI)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to add transaction to mempool: %v", err), http.StatusBadRequest)
		return
	}
	
	response := map[string]interface{}{
		"tx_hash": signedTx.TxHash,
		"status":  "submitted",
		"message": "Pre-signed transaction submitted to mempool",
		"signer":  signedTx.SignerKey,
		"algorithm": signedTx.Algorithm,
		"inputs": len(tx.Inputs),
		"outputs": len(tx.Outputs),
		"total_output_value": tx.TotalOutputValue(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

// handleWebWalletTransactions returns recent transactions
func (sn *ShadowNode) handleWebWalletTransactions(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	
	transactions, err := getWalletTransactions(session.Address, 20)
	if err != nil {
		http.Error(w, "Failed to load transactions", http.StatusInternalServerError)
		return
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(transactions)
}

// handleWebWalletMempool serves the mempool viewer page
func (sn *ShadowNode) handleWebWalletMempool(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Mempool Viewer</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            min-height: 100vh;
        }
        .header {
            background: linear-gradient(135deg, #2d1b4e 0%, #1a1a2e 100%);
            color: white;
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid #333;
        }
        .logo {
            font-size: 1.5rem;
            font-weight: bold;
        }
        .back-btn {
            background: rgba(255,255,255,0.2);
            border: 1px solid rgba(255,255,255,0.3);
            color: white;
            padding: 0.5rem 1rem;
            border-radius: 5px;
            cursor: pointer;
            transition: background 0.3s;
            text-decoration: none;
        }
        .back-btn:hover {
            background: rgba(255,255,255,0.3);
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        .stats-bar {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 1rem;
            text-align: center;
        }
        .stat-value {
            font-size: 1.5rem;
            font-weight: bold;
            color: #8b5cf6;
            margin-bottom: 0.5rem;
        }
        .stat-label {
            color: #a0a0a0;
            font-size: 0.9rem;
        }
        .refresh-btn {
            background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
            color: white;
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 5px;
            cursor: pointer;
            font-size: 1rem;
            margin-bottom: 1rem;
            transition: transform 0.2s;
        }
        .refresh-btn:hover {
            transform: translateY(-2px);
        }
        .auto-refresh {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            margin-bottom: 1rem;
        }
        .auto-refresh input[type="checkbox"] {
            margin: 0;
        }
        .mempool-container {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 10px;
            padding: 1.5rem;
        }
        .mempool-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1rem;
            padding-bottom: 1rem;
            border-bottom: 2px solid #404040;
        }
        .mempool-title {
            color: #8b5cf6;
            font-size: 1.5rem;
            font-weight: bold;
        }
        .transactions-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        .transactions-table th,
        .transactions-table td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid #404040;
        }
        .transactions-table th {
            background: #1a1a1a;
            font-weight: 600;
            color: #e0e0e0;
        }
        .tx-hash {
            font-family: monospace;
            color: #8b5cf6;
            cursor: pointer;
            text-decoration: underline;
        }
        .tx-hash:hover {
            color: #6366f1;
        }
        .loading {
            text-align: center;
            padding: 2rem;
            color: #666;
        }
        .empty-mempool {
            text-align: center;
            padding: 3rem;
            color: #666;
        }
        .error {
            color: #e74c3c;
            background: rgba(231, 76, 60, 0.1);
            padding: 1rem;
            border-radius: 5px;
            margin: 1rem 0;
            border: 1px solid rgba(231, 76, 60, 0.3);
        }
        .transaction-details {
            background: #3d3d3d;
            border-radius: 5px;
            padding: 1rem;
            margin-top: 0.5rem;
            display: none;
        }
        .transaction-details pre {
            color: #e0e0e0;
            white-space: pre-wrap;
            font-size: 0.9rem;
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="logo">🌘 Shadowy Mempool Viewer</div>
        <div class="user-info">
            <span>` + session.WalletName + `</span>
            <a href="/wallet/" class="back-btn">← Back to Wallet</a>
        </div>
    </div>

    <div class="container">
        <!-- Mempool Statistics -->
        <div class="stats-bar">
            <div class="stat-card">
                <div class="stat-value" id="transactionCount">-</div>
                <div class="stat-label">📋 Pending Transactions</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="totalSize">-</div>
                <div class="stat-label">💾 Total Size (KB)</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="totalFees">-</div>
                <div class="stat-label">💰 Total Fees</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="avgFee">-</div>
                <div class="stat-label">📊 Avg Fee Rate</div>
            </div>
        </div>

        <!-- Controls -->
        <div class="controls">
            <button class="refresh-btn" onclick="loadMempoolData()">🔄 Refresh Now</button>
            <div class="auto-refresh">
                <input type="checkbox" id="autoRefresh" checked>
                <label for="autoRefresh">Auto-refresh every 5 seconds</label>
            </div>
        </div>

        <!-- Mempool Transactions -->
        <div class="mempool-container">
            <div class="mempool-header">
                <div class="mempool-title">Pending Transactions</div>
                <div id="lastUpdated">Never updated</div>
            </div>
            <div id="mempoolContent">
                <div class="loading">Loading mempool data...</div>
            </div>
        </div>
    </div>

    <script>
        let autoRefreshInterval = null;

        // Load mempool data
        async function loadMempoolData() {
            try {
                // Load mempool stats
                const statsResponse = await fetch('/api/v1/mempool');
                const stats = await statsResponse.json();

                // Load mempool transactions
                const txResponse = await fetch('/api/v1/mempool/transactions?limit=50');
                const txData = await txResponse.json();

                // Update statistics
                document.getElementById('transactionCount').textContent = stats.transaction_count || 0;
                document.getElementById('totalSize').textContent = ((stats.total_size || 0) / 1024).toFixed(2);
                document.getElementById('totalFees').textContent = (stats.total_fees || 0) + ' sat';
                document.getElementById('avgFee').textContent = stats.avg_fee_rate ? stats.avg_fee_rate.toFixed(2) + ' sat/vB' : 'N/A';

                // Update transactions table
                displayTransactions(txData.transactions || []);
                
                // Update last updated time
                document.getElementById('lastUpdated').textContent = 'Last updated: ' + new Date().toLocaleTimeString();

            } catch (error) {
                console.error('Error loading mempool data:', error);
                document.getElementById('mempoolContent').innerHTML = 
                    '<div class="error">Error loading mempool data: ' + error.message + '</div>';
            }
        }

        // Display transactions in table
        function displayTransactions(transactions) {
            const container = document.getElementById('mempoolContent');
            
            if (transactions.length === 0) {
                container.innerHTML = '<div class="empty-mempool">🎉 Mempool is empty! No pending transactions.</div>';
                return;
            }

            let html = '<table class="transactions-table">';
            html += '<thead><tr>';
            html += '<th>Transaction Hash</th>';
            html += '<th>Fee (sat)</th>';
            html += '<th>Fee Rate (sat/vB)</th>';
            html += '<th>Size (bytes)</th>';
            html += '<th>Time Added</th>';
            html += '<th>Priority</th>';
            html += '</tr></thead><tbody>';

            transactions.forEach(tx => {
                const shortHash = tx.tx_hash ? tx.tx_hash.substring(0, 16) + '...' : 'Unknown';
                const timeAdded = tx.time_added ? new Date(tx.time_added).toLocaleString() : 'Unknown';
                
                html += '<tr>';
                html += '<td><span class="tx-hash" onclick="toggleTransactionDetails(\'' + tx.tx_hash + '\')">' + shortHash + '</span></td>';
                html += '<td>' + (tx.fee || 0) + '</td>';
                html += '<td>' + (tx.fee_rate ? tx.fee_rate.toFixed(2) : 'N/A') + '</td>';
                html += '<td>' + (tx.size || 0) + '</td>';
                html += '<td>' + timeAdded + '</td>';
                html += '<td>' + (tx.priority || 'Normal') + '</td>';
                html += '</tr>';
                html += '<tr id="details-' + tx.tx_hash + '" style="display: none;"><td colspan="6">';
                html += '<div class="transaction-details"><pre>' + JSON.stringify(tx, null, 2) + '</pre></div>';
                html += '</td></tr>';
            });

            html += '</tbody></table>';
            container.innerHTML = html;
        }

        // Toggle transaction details
        function toggleTransactionDetails(txHash) {
            const detailsRow = document.getElementById('details-' + txHash);
            if (detailsRow) {
                detailsRow.style.display = detailsRow.style.display === 'none' ? 'table-row' : 'none';
            }
        }

        // Handle auto-refresh checkbox
        document.getElementById('autoRefresh').addEventListener('change', function() {
            if (this.checked) {
                startAutoRefresh();
            } else {
                stopAutoRefresh();
            }
        });

        // Start auto-refresh
        function startAutoRefresh() {
            if (autoRefreshInterval) clearInterval(autoRefreshInterval);
            autoRefreshInterval = setInterval(loadMempoolData, 5000);
        }

        // Stop auto-refresh
        function stopAutoRefresh() {
            if (autoRefreshInterval) {
                clearInterval(autoRefreshInterval);
                autoRefreshInterval = null;
            }
        }

        // Load initial data
        loadMempoolData();
        
        // Start auto-refresh
        startAutoRefresh();
    </script>
</body>
</html>`

	w.Write([]byte(html))
}

// handleWebWalletPeers serves the network peers viewer page
func (sn *ShadowNode) handleWebWalletPeers(w http.ResponseWriter, r *http.Request) {
	session, authenticated := validateSession(r)
	if !authenticated {
		http.Error(w, "Not authenticated", http.StatusUnauthorized)
		return
	}
	
	w.Header().Set("Content-Type", "text/html")
	
	html := `<!DOCTYPE html>
<html lang="en">
<head>
    <meta charset="UTF-8">
    <meta name="viewport" content="width=device-width, initial-scale=1.0">
    <title>Shadowy Network Peers</title>
    <style>
        * { margin: 0; padding: 0; box-sizing: border-box; }
        body { 
            font-family: 'Segoe UI', Tahoma, Geneva, Verdana, sans-serif;
            background: #1a1a1a;
            color: #e0e0e0;
            min-height: 100vh;
        }
        .header {
            background: linear-gradient(135deg, #2d1b4e 0%, #1a1a2e 100%);
            color: white;
            padding: 1rem 2rem;
            display: flex;
            justify-content: space-between;
            align-items: center;
            border-bottom: 1px solid #333;
        }
        .logo {
            font-size: 1.5rem;
            font-weight: bold;
        }
        .back-btn {
            background: rgba(255,255,255,0.2);
            border: 1px solid rgba(255,255,255,0.3);
            color: white;
            padding: 0.5rem 1rem;
            border-radius: 5px;
            cursor: pointer;
            transition: background 0.3s;
            text-decoration: none;
        }
        .back-btn:hover {
            background: rgba(255,255,255,0.3);
        }
        .container {
            max-width: 1200px;
            margin: 0 auto;
            padding: 2rem;
        }
        .stats-bar {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .stat-card {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 1rem;
            text-align: center;
        }
        .stat-value {
            font-size: 1.5rem;
            font-weight: bold;
            color: #8b5cf6;
            margin-bottom: 0.5rem;
        }
        .stat-label {
            color: #a0a0a0;
            font-size: 0.9rem;
        }
        .refresh-btn {
            background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
            color: white;
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 5px;
            cursor: pointer;
            font-size: 1rem;
            margin-bottom: 1rem;
            transition: transform 0.2s;
        }
        .refresh-btn:hover {
            transform: translateY(-2px);
        }
        .auto-refresh {
            display: flex;
            align-items: center;
            gap: 0.5rem;
            margin-bottom: 1rem;
        }
        .auto-refresh input[type="checkbox"] {
            margin: 0;
        }
        .peers-container {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 10px;
            padding: 1.5rem;
        }
        .peers-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1rem;
            padding-bottom: 1rem;
            border-bottom: 2px solid #404040;
        }
        .peers-title {
            color: #8b5cf6;
            font-size: 1.5rem;
            font-weight: bold;
        }
        .peers-table {
            width: 100%;
            border-collapse: collapse;
            margin-top: 1rem;
        }
        .peers-table th,
        .peers-table td {
            padding: 0.75rem;
            text-align: left;
            border-bottom: 1px solid #404040;
        }
        .peers-table th {
            background: #1a1a1a;
            font-weight: 600;
            color: #e0e0e0;
        }
        .peer-row {
            cursor: pointer;
            transition: background 0.2s;
        }
        .peer-row:hover {
            background: #2a2a2a;
        }
        .peer-id {
            font-family: monospace;
            color: #8b5cf6;
        }
        .peer-address {
            font-family: monospace;
            color: #a0a0a0;
        }
        .status-connected {
            color: #28a745;
            font-weight: bold;
        }
        .status-disconnected {
            color: #dc3545;
            font-weight: bold;
        }
        .status-connecting {
            color: #ffc107;
            font-weight: bold;
        }
        .loading {
            text-align: center;
            padding: 2rem;
            color: #666;
        }
        .empty-peers {
            text-align: center;
            padding: 3rem;
            color: #666;
        }
        .error {
            color: #e74c3c;
            background: rgba(231, 76, 60, 0.1);
            padding: 1rem;
            border-radius: 5px;
            margin: 1rem 0;
            border: 1px solid rgba(231, 76, 60, 0.3);
        }
        .peer-details {
            background: #3d3d3d;
            border-radius: 5px;
            padding: 1rem;
            margin-top: 0.5rem;
            display: none;
        }
        .peer-details-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(200px, 1fr));
            gap: 1rem;
            margin-bottom: 1rem;
        }
        .detail-item {
            display: flex;
            flex-direction: column;
        }
        .detail-label {
            color: #a0a0a0;
            font-size: 0.8rem;
            margin-bottom: 0.25rem;
        }
        .detail-value {
            color: #e0e0e0;
            font-family: monospace;
            font-size: 0.9rem;
            word-break: break-all;
        }
        .connect-peer-form {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 10px;
            padding: 1.5rem;
            margin-bottom: 2rem;
        }
        .connect-form-header {
            color: #8b5cf6;
            font-size: 1.2rem;
            font-weight: bold;
            margin-bottom: 1rem;
        }
        .form-group {
            margin-bottom: 1rem;
        }
        label {
            display: block;
            margin-bottom: 0.5rem;
            color: #e0e0e0;
            font-weight: 500;
        }
        input {
            width: 100%;
            padding: 0.75rem;
            border: 2px solid #404040;
            border-radius: 5px;
            font-size: 1rem;
            transition: border-color 0.3s;
            background: #1a1a1a;
            color: #e0e0e0;
        }
        input:focus {
            outline: none;
            border-color: #8b5cf6;
        }
        .btn {
            padding: 0.75rem 1.5rem;
            background: linear-gradient(135deg, #8b5cf6 0%, #6366f1 100%);
            color: white;
            border: none;
            border-radius: 5px;
            font-size: 1rem;
            cursor: pointer;
            transition: transform 0.2s;
        }
        .btn:hover {
            transform: translateY(-2px);
        }
        .success {
            color: #27ae60;
            background: rgba(39, 174, 96, 0.1);
            padding: 1rem;
            border-radius: 5px;
            margin: 1rem 0;
            border: 1px solid rgba(39, 174, 96, 0.3);
        }
    </style>
</head>
<body>
    <div class="header">
        <div class="logo">🌘 Shadowy Network Peers</div>
        <div class="user-info">
            <span>` + session.WalletName + `</span>
            <a href="/wallet/" class="back-btn">← Back to Wallet</a>
        </div>
    </div>

    <div class="container">
        <!-- Connect to Peer Form -->
        <div class="connect-peer-form">
            <div class="connect-form-header">🔗 Connect to New Peer</div>
            <form id="connectPeerForm">
                <div class="form-group">
                    <label for="peerAddress">Peer Address (IP:Port):</label>
                    <input type="text" id="peerAddress" name="peerAddress" placeholder="192.168.1.100:8888" required>
                </div>
                <button type="submit" class="btn">Connect to Peer</button>
            </form>
            <div id="connectResult"></div>
        </div>

        <!-- Network Statistics -->
        <div class="stats-bar">
            <div class="stat-card">
                <div class="stat-value" id="connectedPeers">-</div>
                <div class="stat-label">🟢 Connected Peers</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="totalPeers">-</div>
                <div class="stat-label">🔗 Total Peers</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="nodeId">-</div>
                <div class="stat-label">🆔 My Node ID</div>
            </div>
            <div class="stat-card">
                <div class="stat-value" id="listenAddress">-</div>
                <div class="stat-label">📡 Listen Address</div>
            </div>
        </div>

        <!-- Controls -->
        <div class="controls">
            <button class="refresh-btn" onclick="loadPeerData()">🔄 Refresh Now</button>
            <div class="auto-refresh">
                <input type="checkbox" id="autoRefresh" checked>
                <label for="autoRefresh">Auto-refresh every 5 seconds</label>
            </div>
        </div>

        <!-- Network Peers -->
        <div class="peers-container">
            <div class="peers-header">
                <div class="peers-title">Network Peers</div>
                <div id="lastUpdated">Never updated</div>
            </div>
            <div id="peersContent">
                <div class="loading">Loading peer data...</div>
            </div>
        </div>
    </div>

    <script>
        let autoRefreshInterval = null;

        // Load peer data
        async function loadPeerData() {
            try {
                // Load consensus status
                const consensusResponse = await fetch('/api/v1/consensus');
                const consensusData = await consensusResponse.json();

                // Load peer details
                const peersResponse = await fetch('/api/v1/consensus/peers');
                const peersData = await peersResponse.json();

                // Update statistics
                const connectedCount = (peersData.peers || []).filter(p => p.status === 'connected' || p.status === 'active').length;
                document.getElementById('connectedPeers').textContent = connectedCount;
                document.getElementById('totalPeers').textContent = peersData.peer_count || 0;
                document.getElementById('nodeId').textContent = consensusData.node_id ? consensusData.node_id.substring(0, 12) + '...' : 'Unknown';
                document.getElementById('listenAddress').textContent = consensusData.listen_addr || 'Unknown';

                // Update peers table
                displayPeers(peersData.peers || []);
                
                // Update last updated time
                document.getElementById('lastUpdated').textContent = 'Last updated: ' + new Date().toLocaleTimeString();

            } catch (error) {
                console.error('Error loading peer data:', error);
                document.getElementById('peersContent').innerHTML = 
                    '<div class="error">Error loading peer data: ' + error.message + '</div>';
            }
        }

        // Display peers in table
        function displayPeers(peers) {
            const container = document.getElementById('peersContent');
            
            if (peers.length === 0) {
                container.innerHTML = '<div class="empty-peers">🔍 No peers connected. Try connecting to a peer using the form above.</div>';
                return;
            }

            let html = '<table class="peers-table">';
            html += '<thead><tr>';
            html += '<th>Peer ID</th>';
            html += '<th>Address</th>';
            html += '<th>Status</th>';
            html += '<th>Chain Height</th>';
            html += '<th>Last Seen</th>';
            html += '</tr></thead><tbody>';

            peers.forEach(peer => {
                const shortId = peer.id ? peer.id.substring(0, 12) + '...' : 'Unknown';
                const statusClass = peer.status === 'connected' || peer.status === 'active' ? 'status-connected' : 
                                   peer.status === 'disconnected' ? 'status-disconnected' : 'status-connecting';
                const lastSeen = peer.last_seen ? new Date(peer.last_seen).toLocaleString() : 'Unknown';
                
                html += '<tr class="peer-row" onclick="togglePeerDetails(\'' + peer.id + '\')">';
                html += '<td><span class="peer-id">' + shortId + '</span></td>';
                html += '<td><span class="peer-address">' + (peer.address || 'Unknown') + '</span></td>';
                html += '<td><span class="' + statusClass + '">' + (peer.status || 'Unknown') + '</span></td>';
                html += '<td>' + (peer.chain_height || 0) + '</td>';
                html += '<td>' + lastSeen + '</td>';
                html += '</tr>';
                html += '<tr><td colspan="5"><div id="peer-detail-' + peer.id + '" class="peer-details"></div></td></tr>';
            });

            html += '</tbody></table>';
            container.innerHTML = html;
        }

        // Toggle peer detail view
        function togglePeerDetails(peerId) {
            const detailDiv = document.getElementById('peer-detail-' + peerId);
            
            if (detailDiv.classList.contains('active')) {
                detailDiv.style.display = 'none';
                detailDiv.classList.remove('active');
                return;
            }
            
            // Close all other details first
            document.querySelectorAll('.peer-details').forEach(detail => {
                detail.style.display = 'none';
                detail.classList.remove('active');
            });
            
            // Find peer data and display details
            loadPeerDetails(peerId, detailDiv);
        }

        // Load detailed peer information
        async function loadPeerDetails(peerId, detailDiv) {
            try {
                const response = await fetch('/api/v1/consensus/peers');
                const data = await response.json();
                const peer = (data.peers || []).find(p => p.id === peerId);
                
                if (!peer) {
                    detailDiv.innerHTML = '<div class="error">Peer details not found</div>';
                    detailDiv.style.display = 'block';
                    detailDiv.classList.add('active');
                    return;
                }

                let html = '<div class="peer-details-grid">';
                html += '<div class="detail-item"><div class="detail-label">Full Peer ID</div><div class="detail-value">' + (peer.id || 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Address</div><div class="detail-value">' + (peer.address || 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Status</div><div class="detail-value">' + (peer.status || 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Chain Height</div><div class="detail-value">' + (peer.chain_height || 0) + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Chain Hash</div><div class="detail-value">' + (peer.chain_hash || 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Protocol Version</div><div class="detail-value">' + (peer.protocol_version || 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">User Agent</div><div class="detail-value">' + (peer.user_agent || 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Connected Since</div><div class="detail-value">' + 
                        (peer.connected_at ? new Date(peer.connected_at).toLocaleString() : 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Last Seen</div><div class="detail-value">' + 
                        (peer.last_seen ? new Date(peer.last_seen).toLocaleString() : 'Unknown') + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Bytes Sent</div><div class="detail-value">' + (peer.bytes_sent || 0) + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Bytes Received</div><div class="detail-value">' + (peer.bytes_received || 0) + '</div></div>';
                html += '<div class="detail-item"><div class="detail-label">Messages Sent</div><div class="detail-value">' + (peer.messages_sent || 0) + '</div></div>';
                html += '</div>';

                detailDiv.innerHTML = html;
                detailDiv.style.display = 'block';
                detailDiv.classList.add('active');
            } catch (error) {
                detailDiv.innerHTML = '<div class="error">Error loading peer details: ' + error.message + '</div>';
                detailDiv.style.display = 'block';
                detailDiv.classList.add('active');
            }
        }

        // Handle connect peer form
        document.getElementById('connectPeerForm').addEventListener('submit', async (e) => {
            e.preventDefault();
            
            const formData = new FormData(e.target);
            const address = formData.get('peerAddress');
            
            if (!address) {
                document.getElementById('connectResult').innerHTML = 
                    '<div class="error">Please enter a peer address</div>';
                return;
            }

            try {
                const response = await fetch('/api/v1/consensus/peers/connect', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({ address: address })
                });
                
                if (response.ok) {
                    const result = await response.json();
                    document.getElementById('connectResult').innerHTML = 
                        '<div class="success">Connection initiated to ' + address + '</div>';
                    
                    // Refresh peer list after a short delay
                    setTimeout(loadPeerData, 2000);
                    
                    // Clear form
                    e.target.reset();
                } else {
                    const errorText = await response.text();
                    document.getElementById('connectResult').innerHTML = 
                        '<div class="error">Connection failed: ' + errorText + '</div>';
                }
            } catch (error) {
                document.getElementById('connectResult').innerHTML = 
                    '<div class="error">Connection error: ' + error.message + '</div>';
            }
        });

        // Handle auto-refresh checkbox
        document.getElementById('autoRefresh').addEventListener('change', function() {
            if (this.checked) {
                startAutoRefresh();
            } else {
                stopAutoRefresh();
            }
        });

        // Start auto-refresh
        function startAutoRefresh() {
            if (autoRefreshInterval) clearInterval(autoRefreshInterval);
            autoRefreshInterval = setInterval(loadPeerData, 5000);
        }

        // Stop auto-refresh
        function stopAutoRefresh() {
            if (autoRefreshInterval) {
                clearInterval(autoRefreshInterval);
                autoRefreshInterval = null;
            }
        }

        // Load initial data
        loadPeerData();
        
        // Start auto-refresh
        startAutoRefresh();
    </script>
</body>
</html>`
	
	w.Write([]byte(html))
}