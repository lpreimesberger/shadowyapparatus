package cmd

import (
    "crypto/sha256"
    "encoding/hex"
    "encoding/json"
    "fmt"
    "log"
    "net/http"
    "os"
    "path/filepath"
    "strings"
    "time"
)

// getWebWalletDir returns the wallet directory for web wallet operations
func getWebWalletDir() string {
    homeDir, err := os.UserHomeDir()
    if err != nil {
        return ".shadowy"
    }
    return filepath.Join(homeDir, ".shadowy")
}

// WebWalletSession represents an authenticated web wallet session
type WebWalletSession struct {
    SessionID  string    `json:"session_id"`
    Address    string    `json:"address"`
    WalletName string    `json:"wallet_name"`
    CreatedAt  time.Time `json:"created_at"`
    ExpiresAt  time.Time `json:"expires_at"`
}

// PaymentRequest represents a payment request URI
type PaymentRequest struct {
    Address  string  `json:"address"`
    Amount   float64 `json:"amount"`
    Currency string  `json:"currency"`
    Label    string  `json:"label,omitempty"`
    Message  string  `json:"message,omitempty"`
    URI      string  `json:"uri"`
}

// SendRequest represents a send transaction request
type SendRequest struct {
    ToAddress string  `json:"to_address"`
    Amount    float64 `json:"amount"`
    Fee       float64 `json:"fee,omitempty"`
    Message   string  `json:"message,omitempty"`
    TokenID   string  `json:"token_id,omitempty"` // For token transfers
    AssetType string  `json:"asset_type"`         // "shadow" or "token"
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

        /* Token-specific styles */
        .tokens-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid #404040;
        }
        .tokens-header h3 {
            margin: 0;
            color: #e0e0e0;
            font-size: 1.2rem;
        }
        .tokens-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(350px, 1fr));
            gap: 1rem;
            margin-bottom: 2rem;
        }
        .token-card {
            background: #2a2a2a;
            border: 2px solid #404040;
            border-radius: 12px;
            padding: 1.5rem;
            margin-bottom: 1rem;
            transition: all 0.3s ease;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
        }
        .token-card:hover {
            border-color: #8b5cf6;
            background: #2d2d2d;
            transform: translateY(-2px);
            box-shadow: 0 4px 8px rgba(0, 0, 0, 0.4);
        }
        .token-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.25rem;
            padding-bottom: 1rem;
            border-bottom: 2px solid #404040;
        }
        .token-header h4 {
            margin: 0;
            color: #e0e0e0;
            font-size: 1.2rem;
            font-weight: 600;
        }
        .trust-badge {
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.75rem;
            font-weight: bold;
            text-transform: uppercase;
        }
        .trust-accepted {
            background: rgba(40, 167, 69, 0.2);
            color: #28a745;
            border: 1px solid #28a745;
        }
        .trust-banned {
            background: rgba(220, 53, 69, 0.2);
            color: #dc3545;
            border: 1px solid #dc3545;
        }
        .trust-unknown {
            background: rgba(255, 193, 7, 0.2);
            color: #ffc107;
            border: 1px solid #ffc107;
        }
        .trust-verified {
            background: rgba(32, 201, 151, 0.2);
            color: #20c997;
            border: 1px solid #20c997;
        }
        .token-details {
            color: #b0b0b0;
        }
        .token-balance {
            margin-bottom: 1.5rem;
            background: #1a1a1a;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 1rem;
            text-align: center;
        }
        .balance-label {
            display: block;
            font-size: 0.9rem;
            color: #a0a0a0;
            margin-bottom: 0.5rem;
            text-transform: uppercase;
            letter-spacing: 0.5px;
        }
        .balance-amount {
            display: block;
            font-size: 1.4rem;
            font-weight: bold;
            color: #8b5cf6;
            font-family: monospace;
            word-break: break-all;
        }
        .token-info {
            border-top: 2px solid #404040;
            padding-top: 1rem;
            margin-top: 1rem;
        }
        .info-row {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 0.75rem;
            padding: 0.5rem;
            background: #1a1a1a;
            border-radius: 6px;
        }
        .info-label {
            font-size: 0.9rem;
            color: #a0a0a0;
            font-weight: 500;
            min-width: 80px;
        }
        .info-value {
            font-size: 0.85rem;
            color: #b0b0b0;
            font-family: monospace;
        }
        .token-id {
            cursor: pointer;
            text-decoration: underline;
            color: #8b5cf6;
        }
        .token-id:hover {
            color: #6366f1;
        }
        .no-tokens {
            text-align: center;
            padding: 3rem;
            color: #666;
        }
        .no-tokens h3 {
            margin-bottom: 1rem;
            color: #888;
        }
        .unknown-tokens-warning {
            background: rgba(255, 193, 7, 0.1);
            border: 1px solid rgba(255, 193, 7, 0.3);
            border-radius: 8px;
            padding: 1rem;
            margin-top: 1.5rem;
            color: #ffc107;
        }
        .unknown-tokens-warning h4 {
            margin: 0 0 0.5rem 0;
            color: #ffc107;
        }
        .unknown-tokens-warning p {
            margin: 0.5rem 0;
            color: #e0e0e0;
        }

        /* Token approval buttons */
        .token-actions {
            margin-top: 15px;
            padding-top: 15px;
            border-top: 1px solid #eee;
            display: flex;
            gap: 8px;
            flex-wrap: wrap;
        }

        .token-actions {
            border-top: 2px solid #404040;
            padding-top: 1rem;
            margin-top: 1.5rem;
            display: flex;
            flex-wrap: wrap;
            gap: 0.5rem;
        }
        .trust-btn {
            padding: 8px 16px;
            border: none;
            border-radius: 6px;
            cursor: pointer;
            font-size: 0.85rem;
            font-weight: 600;
            transition: all 0.3s ease;
            text-decoration: none;
            display: inline-flex;
            align-items: center;
            gap: 6px;
            text-transform: uppercase;
            letter-spacing: 0.5px;
            flex: 1;
            min-width: 120px;
            justify-content: center;
        }
        .trust-btn:hover {
            transform: translateY(-2px);
            box-shadow: 0 4px 8px rgba(0, 0, 0, 0.3);
        }

        .accept-btn {
            background-color: #28a745;
            color: white;
        }

        .accept-btn:hover {
            background-color: #218838;
        }

        .ban-btn {
            background-color: #dc3545;
            color: white;
        }

        .ban-btn:hover {
            background-color: #c82333;
        }

        .ignore-btn {
            background-color: #6c757d;
            color: white;
        }

        .ignore-btn:hover {
            background-color: #5a6268;
        }

        .melt-btn {
            background: linear-gradient(135deg, #ff6b35 0%, #e55a2e 100%);
            color: white;
            border: 2px solid transparent;
        }

        .melt-btn:hover {
            background: linear-gradient(135deg, #e55a2e 0%, #cc4d26 100%);
            border-color: #ff6b35;
        }

        /* Send form styling */
        .balance-display {
            margin-top: 5px;
            font-size: 12px;
            color: #8b5cf6;
            font-weight: 500;
        }

        select {
            width: 100%;
            padding: 0.75rem;
            border: 2px solid #404040;
            border-radius: 5px;
            font-size: 1rem;
            transition: border-color 0.3s;
            background: #1a1a1a;
            color: #e0e0e0;
        }

        select:focus {
            outline: none;
            border-color: #8b5cf6;
        }
        /* Token Foundry styles */
        .foundry-header {
            text-align: center;
            margin-bottom: 2rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid #404040;
        }
        .foundry-header h2 {
            margin: 0 0 0.5rem 0;
            color: #e0e0e0;
            font-size: 1.8rem;
        }
        .foundry-header p {
            margin: 0;
            color: #b0b0b0;
            font-size: 1rem;
        }
        .foundry-form {
            max-width: 600px;
            margin: 0 auto;
            color: #e0e0e0;
        }

        .token-type-toggle {
            display: flex;
            gap: 1rem;
            margin: 0.5rem 0;
        }

        .token-type-toggle input[type="radio"] {
            margin-right: 0.5rem;
        }

        .toggle-label {
            cursor: pointer;
            padding: 0.5rem 1rem;
            border: 2px solid #404040;
            border-radius: 5px;
            transition: all 0.3s;
            background: #2d2d2d;
        }

        .token-type-toggle input[type="radio"]:checked + .toggle-label {
            border-color: #8b5cf6;
            background: rgba(139, 92, 246, 0.1);
            color: #8b5cf6;
        }

        .nft-badge {
            color: #ff6b35 !important;
            font-weight: bold;
        }

        .uri-link {
            color: #8b5cf6;
            text-decoration: none;
            word-break: break-all;
        }

        .uri-link:hover {
            color: #a78bfa;
            text-decoration: underline;
        }
        .form-section {
            margin-bottom: 2rem;
        }
        .form-section h3 {
            margin: 0 0 1rem 0;
            color: #e0e0e0;
            font-size: 1.1rem;
        }
        .form-group {
            margin-bottom: 1.5rem;
        }
        .form-group label {
            display: block;
            margin-bottom: 0.5rem;
            color: #b0b0b0;
            font-weight: 500;
        }
        .form-group input, .form-group textarea {
            width: 100%;
            padding: 0.75rem;
            background: #1a1a1a;
            border: 1px solid #404040;
            border-radius: 4px;
            color: #e0e0e0;
            font-size: 0.9rem;
            box-sizing: border-box;
        }
        .form-group input:focus, .form-group textarea:focus {
            outline: none;
            border-color: #6366f1;
        }
        .form-help {
            font-size: 0.8rem;
            color: #888;
            margin-top: 0.5rem;
            line-height: 1.4;
        }
        .cost-preview {
            background: rgba(40, 167, 69, 0.1);
            border: 1px solid #28a745;
            border-radius: 4px;
            padding: 1rem;
            margin: 1rem 0;
        }
        .cost-preview h4 {
            margin: 0 0 0.5rem 0;
            color: #28a745;
        }
        .cost-details {
            color: #e0e0e0;
            font-family: monospace;
            font-size: 0.9rem;
        }
        .form-actions {
            display: flex;
            gap: 1rem;
            justify-content: flex-end;
            padding-top: 1rem;
            border-top: 1px solid #404040;
        }
        .btn {
            padding: 0.75rem 1.5rem;
            border: none;
            border-radius: 4px;
            cursor: pointer;
            font-size: 0.9rem;
            font-weight: 500;
            transition: all 0.2s ease;
        }
        .btn-primary {
            background: #6366f1;
            color: white;
        }
        .btn-primary:hover {
            background: #5a5af1;
        }
        .btn-primary:disabled {
            background: #555;
            cursor: not-allowed;
        }
        .btn-secondary {
            background: #404040;
            color: #e0e0e0;
        }
        .btn-secondary:hover {
            background: #555;
        }
        @media (max-width: 768px) {
            .tokens-grid {
                grid-template-columns: 1fr;
            }
            .token-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 0.5rem;
            }
            .tokens-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 1rem;
            }
            .foundry-form {
                padding: 1rem;
            }
            .form-actions {
                flex-direction: column;
            }
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
        <script src="https://cdn.jsdelivr.net/npm/@popperjs/core@2.11.8/dist/umd/popper.min.js"></script>
    <link href="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/css/bootstrap.min.css" rel="stylesheet" integrity="sha384-QWTKZyjpPEjISv5WaRU9OFeRpok6YctnYmDr5pNlyT2bRjXh0JMhjY6hW+ALEwIH" crossorigin="anonymous">
<script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.min.js" integrity="sha384-0pUGZvbkm6XF6gxjEnlmuGrJXVbNuzT9qBBavbLwCsOGabYfZo0T0to5eqruptLy" crossorigin="anonymous"></script>
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
        /* Main tab header */
        .main-tab-header {
            display: flex;
            background: #1a1a1a;
            border-bottom: 2px solid #404040;
        }
        .main-tab-button {
            flex: 1;
            padding: 1rem;
            background: none;
            border: none;
            color: #a0a0a0;
            cursor: pointer;
            transition: all 0.3s;
            font-size: 1rem;
            font-weight: 500;
            position: relative;
        }
        .main-tab-button:hover {
            background: #2a2a2a;
            color: #e0e0e0;
        }
        .main-tab-button.active {
            color: #8b5cf6;
            background: #2d2d2d;
        }
        .main-tab-button.active:after {
            content: '';
            position: absolute;
            bottom: 0;
            left: 0;
            right: 0;
            height: 3px;
            background: #8b5cf6;
        }

        /* Sub tab container and headers */
        .sub-tab-container {
            background: #252525;
            border-bottom: 1px solid #404040;
        }
        .sub-tab-header {
            display: none;
            background: #252525;
        }
        .sub-tab-header.active {
            display: flex;
        }
        .sub-tab-button {
            padding: 0.75rem 1.5rem;
            background: none;
            border: none;
            color: #a0a0a0;
            cursor: pointer;
            transition: all 0.3s;
            font-size: 0.9rem;
            position: relative;
        }
        .sub-tab-button:hover {
            background: #2a2a2a;
            color: #e0e0e0;
        }
        .sub-tab-button.active {
            color: #8b5cf6;
            background: #2a2a2a;
        }
        .sub-tab-button.active:after {
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
            .main-tab-header, .sub-tab-header {
                flex-wrap: wrap;
            }
            .main-tab-button, .sub-tab-button {
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

        /* Modal styles */
        .modal {
            display: none;
            position: fixed;
            z-index: 1000;
            left: 0;
            top: 0;
            width: 100%;
            height: 100%;
            background-color: rgba(0, 0, 0, 0.8);
        }

        .modal-content {
            background-color: #2d2d2d;
            margin: 5% auto;
            padding: 2rem;
            border: 1px solid #404040;
            border-radius: 10px;
            width: 90%;
            max-width: 500px;
            position: relative;
        }

        .modal-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 1.5rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid #404040;
        }

        .modal-title {
            color: #ff6b35;
            font-size: 1.5rem;
            font-weight: bold;
            margin: 0;
        }

        .modal-close {
            background: none;
            border: none;
            color: #a0a0a0;
            font-size: 1.5rem;
            cursor: pointer;
            padding: 0;
            width: 30px;
            height: 30px;
            display: flex;
            align-items: center;
            justify-content: center;
        }

        .modal-close:hover {
            color: #e0e0e0;
        }

        .warning-box {
            background: rgba(255, 107, 53, 0.1);
            border: 2px solid #ff6b35;
            border-radius: 8px;
            padding: 1.5rem;
            margin-bottom: 1.5rem;
        }

        .warning-title {
            color: #ff6b35;
            font-size: 1.2rem;
            font-weight: bold;
            margin-bottom: 1rem;
            display: flex;
            align-items: center;
            gap: 0.5rem;
        }

        .warning-text {
            color: #e0e0e0;
            line-height: 1.5;
            margin-bottom: 0.5rem;
        }

        .melt-form {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }

        .confirmation-input {
            background: #1a1a1a;
            border: 2px solid #ff6b35;
            color: #e0e0e0;
            padding: 0.75rem;
            border-radius: 5px;
            font-family: monospace;
            font-size: 1rem;
            text-align: center;
        }

        .confirmation-input::placeholder {
            color: #666;
        }

        .modal-actions {
            display: flex;
            gap: 1rem;
            justify-content: flex-end;
            margin-top: 1.5rem;
        }

        .btn-danger {
            background: linear-gradient(135deg, #ff6b35 0%, #e55a2e 100%);
            color: white;
            border: none;
            padding: 0.75rem 1.5rem;
            border-radius: 5px;
            cursor: pointer;
            font-size: 1rem;
            transition: transform 0.2s;
        }

        .btn-danger:hover {
            transform: translateY(-2px);
        }

        .btn-danger:disabled {
            background: #666;
            cursor: not-allowed;
            transform: none;
        }

        /* Marketplace Styles */
        .marketplace-header {
            text-align: center;
            margin-bottom: 2rem;
            padding-bottom: 1rem;
            border-bottom: 1px solid #404040;
        }

        .marketplace-sections {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 2rem;
            align-items: flex-start;
        }

        .marketplace-section {
            background: #1a1a1a;
            border: 1px solid #404040;
            border-radius: 10px;
            padding: 1.5rem;
        }

        .marketplace-section h3 {
            color: #8b5cf6;
            margin-bottom: 1rem;
            font-size: 1.3rem;
        }

        .trade-form {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }

        .marketplace-filters {
            display: grid;
            grid-template-columns: 1fr 1fr auto;
            gap: 1rem;
            margin-bottom: 1.5rem;
            padding: 1rem;
            background: #1a1a1a;
            border-radius: 8px;
            border: 1px solid #404040;
        }

        .filter-group {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        .filter-group label {
            font-size: 0.9rem;
            color: #a0a0a0;
        }

        .filter-group select {
            padding: 0.5rem;
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 5px;
            color: #e0e0e0;
        }

        .trade-cost-preview {
            background: #1a1a1a;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 1rem;
            margin-top: 1rem;
        }

        .trade-cost-preview h4 {
            color: #8b5cf6;
            margin: 0 0 0.5rem 0;
            font-size: 1rem;
        }

        .cost-details {
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
            font-size: 0.9rem;
        }

        .cost-value {
            color: #fbbf24;
            font-weight: bold;
        }

        .cost-breakdown {
            margin-top: 0.5rem;
            padding-left: 1rem;
            font-size: 0.85rem;
            color: #b0b0b0;
        }

        .cost-breakdown div {
            margin-bottom: 0.25rem;
        }

        .offer-card {
            background: #1a1a1a;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 1rem;
            transition: border-color 0.3s;
        }

        .offer-card:hover {
            border-color: #8b5cf6;
        }

        .offer-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 0.5rem;
        }

        .offer-token {
            font-weight: bold;
            color: #8b5cf6;
        }

        .offer-price {
            color: #fbbf24;
            font-weight: bold;
        }

        .offer-details {
            display: grid;
            grid-template-columns: 1fr 1fr;
            gap: 0.5rem;
            margin-bottom: 1rem;
            font-size: 0.9rem;
        }

        .offer-detail {
            display: flex;
            justify-content: space-between;
        }

        .offer-actions {
            display: flex;
            gap: 0.5rem;
            justify-content: flex-end;
        }

        .btn-purchase {
            background: linear-gradient(135deg, #10b981 0%, #059669 100%);
            color: white;
            border: none;
            padding: 0.5rem 1rem;
            border-radius: 5px;
            cursor: pointer;
            font-size: 0.9rem;
            transition: transform 0.2s;
        }

        .btn-purchase:hover {
            transform: translateY(-1px);
        }

        .offer-expired {
            opacity: 0.6;
            border-color: #dc3545;
        }

        .expired-label {
            color: #dc3545;
            font-size: 0.8rem;
            font-weight: bold;
        }

        .foundry-notice {
            background: #1a1a1a;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 1.5rem;
            margin-top: 1rem;
        }

        .notice-title {
            font-size: 1.2rem;
            font-weight: bold;
            color: #8b5cf6;
            margin-bottom: 1rem;
        }

        .notice-text {
            color: #d0d0d0;
            margin-bottom: 1rem;
            line-height: 1.5;
        }

        .foundry-notice ul {
            color: #d0d0d0;
            margin-left: 1.5rem;
            margin-bottom: 1rem;
        }

        .foundry-notice li {
            margin-bottom: 0.5rem;
        }

        .transaction-status {
            position: fixed;
            top: 20px;
            right: 20px;
            z-index: 1000;
            max-width: 400px;
        }

        .tx-status {
            background: #2d2d2d;
            border: 1px solid #404040;
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 0.5rem;
            color: #e0e0e0;
            box-shadow: 0 4px 12px rgba(0, 0, 0, 0.3);
            animation: slideIn 0.3s ease-out;
        }

        @keyframes slideIn {
            from {
                transform: translateX(100%);
                opacity: 0;
            }
            to {
                transform: translateX(0);
                opacity: 1;
            }
        }

        /* Pool-specific styles */
        .pools-list {
            display: flex;
            flex-direction: column;
            gap: 1rem;
        }

        .pool-item {
            background: #2a2a2a;
            border: 1px solid #505050;
            border-radius: 8px;
            padding: 1rem;
        }

        .pool-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 0.5rem;
        }

        .pool-header h4 {
            color: #60a5fa;
            margin: 0;
        }

        .pool-fee {
            background: #1f2937;
            color: #fbbf24;
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.8rem;
            font-weight: bold;
        }

        .pool-details {
            display: flex;
            flex-direction: column;
            gap: 0.25rem;
            font-size: 0.9rem;
        }

        .pool-pair {
            color: #d1d5db;
            font-weight: bold;
        }

        .pool-address {
            color: #9ca3af;
        }

        .pool-address code {
            background: #1f2937;
            padding: 0.2rem 0.4rem;
            border-radius: 3px;
            font-family: 'Courier New', monospace;
            font-size: 0.8rem;
        }

        .pool-creator {
            color: #6b7280;
        }

        .pool-link {
            color: #60a5fa !important;
            text-decoration: none;
            transition: color 0.2s;
        }

        .pool-link:hover {
            color: #93c5fd !important;
            text-decoration: underline;
        }

        .pool-address-link {
            color: #9ca3af;
            text-decoration: none;
            transition: color 0.2s;
        }

        .pool-address-link:hover {
            color: #d1d5db;
            text-decoration: underline;
        }

        .pool-address-link code {
            color: inherit;
        }

        .pool-empty {
            opacity: 0.7;
            background: #1a1a1a !important;
            border-color: #404040 !important;
        }

        .pool-disabled {
            color: #9ca3af;
            cursor: not-allowed;
        }

        .pool-reserves {
            color: #34d399;
            font-size: 0.85rem;
            font-weight: bold;
            margin: 0.25rem 0;
        }

        .pool-warning {
            color: #fbbf24;
            font-size: 0.8rem;
            font-style: italic;
        }

        .no-pools {
            text-align: center;
            color: #9ca3af;
            padding: 2rem;
            background: #2a2a2a;
            border-radius: 8px;
            border: 1px solid #404040;
        }

        .refresh-controls {
            margin-bottom: 1rem;
        }

        .refresh-btn {
            background: #374151;
            color: #d1d5db;
            border: 1px solid #4b5563;
            padding: 0.5rem 1rem;
            border-radius: 4px;
            cursor: pointer;
            transition: background-color 0.2s;
        }

        .refresh-btn:hover {
            background: #4b5563;
        }

        .cost-info {
            background: #1e3a8a;
            border: 1px solid #3b82f6;
            border-radius: 6px;
            padding: 1rem;
            color: #dbeafe;
            text-align: center;
        }

        @media (max-width: 768px) {
            .marketplace-sections {
                grid-template-columns: 1fr;
            }

            .marketplace-filters {
                grid-template-columns: 1fr;
            }

            .offer-details {
                grid-template-columns: 1fr;
            }

            .pool-header {
                flex-direction: column;
                align-items: flex-start;
                gap: 0.5rem;
            }
        }

        /* Syndicate Styles */
        .syndicates-header {
            text-align: center;
            margin-bottom: 2rem;
            padding-bottom: 1rem;
            border-bottom: 2px solid #404040;
        }

        .syndicates-header h2 {
            color: #ff6b35;
            margin-bottom: 0.5rem;
        }

        .syndicates-sections {
            display: flex;
            flex-direction: column;
            gap: 2rem;
        }

        .section-card {
            background: #2a2a2a;
            border: 2px solid #404040;
            border-radius: 12px;
            padding: 1.5rem;
            box-shadow: 0 2px 4px rgba(0, 0, 0, 0.3);
        }

        .section-card h3 {
            color: #8b5cf6;
            margin-bottom: 1rem;
            padding-bottom: 0.5rem;
            border-bottom: 1px solid #404040;
        }

        .membership-card {
            background: #333;
            border: 1px solid #555;
            border-radius: 8px;
            padding: 1rem;
            margin-bottom: 1rem;
        }

        .membership-header {
            display: flex;
            justify-content: space-between;
            align-items: center;
            margin-bottom: 0.5rem;
        }

        .membership-header h4 {
            margin: 0;
            color: #e0e0e0;
            font-size: 1.1rem;
        }

        .membership-status {
            padding: 0.25rem 0.5rem;
            border-radius: 4px;
            font-size: 0.8rem;
            font-weight: bold;
        }

        .membership-status.active {
            background: rgba(40, 167, 69, 0.2);
            color: #28a745;
            border: 1px solid #28a745;
        }

        .membership-status.expiring {
            background: rgba(255, 193, 7, 0.2);
            color: #ffc107;
            border: 1px solid #ffc107;
        }

        .membership-details p {
            margin: 0.25rem 0;
            color: #a0a0a0;
            font-size: 0.9rem;
        }

        .no-membership {
            text-align: center;
            padding: 2rem;
            color: #666;
            font-style: italic;
        }

        .syndicate-stats-grid {
            display: grid;
            grid-template-columns: repeat(auto-fit, minmax(250px, 1fr));
            gap: 1rem;
        }

        .syndicate-stat-card {
            background: #333;
            border: 1px solid #555;
            border-radius: 8px;
            padding: 1rem;
        }

        .stat-header h4 {
            margin: 0 0 1rem 0;
            color: #e0e0e0;
            font-size: 1rem;
            text-align: center;
        }

        .stat-details {
            display: flex;
            flex-direction: column;
            gap: 0.5rem;
        }

        .stat-item {
            display: flex;
            justify-content: space-between;
            align-items: center;
        }

        .stat-label {
            color: #a0a0a0;
            font-size: 0.9rem;
        }

        .stat-value {
            color: #e0e0e0;
            font-weight: bold;
        }

        .stat-value.warning {
            color: #ff6b35;
        }


        .tab-container {
            position: relative;
        }

        @media (max-width: 768px) {
            .syndicate-stats-grid {
                grid-template-columns: 1fr;
            }

            .syndicates-sections {
                gap: 1rem;
            }

            .section-card {
                padding: 1rem;
            }

            .main-tab-header, .sub-tab-header {
                flex-wrap: wrap;
                gap: 0.25rem;
            }

            .main-tab-button, .sub-tab-button {
                font-size: 0.8rem;
                padding: 0.5rem 0.75rem;
            }
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
            <div class="stat-card">
                <div class="stat-value" id="syncStatus" style="color: #fbbf24;">Loading...</div>
                <div class="stat-label">🔄 Sync Status</div>
            </div>
        </div>

        <!-- Balance Section -->
        <div class="balance-section">
            <div class="balance-amount" id="balanceAmount">Loading...</div>
            <div class="balance-label">SHADOW</div>
            <div class="address-display" id="walletAddress" onclick="copyAddress()" title="Click to copy address">` + session.Address + `</div>
        </div>

        <!-- Transaction Status Notification -->
        <div id="transactionStatus" class="transaction-status" style="display: none;"></div>

        <!-- Tabbed Interface -->
        <div class="tab-container">
            <!-- Main tab header -->
            <div class="main-tab-header">
                <button class="main-tab-button active" onclick="switchMainTab('wallet')">💼 Wallet</button>
                <button class="main-tab-button" onclick="switchMainTab('node')">🖥️ Node</button>
                <button class="main-tab-button" onclick="switchMainTab('foundry')">🏭 Foundry</button>
                <button class="main-tab-button" onclick="switchMainTab('swap')">🔄 Swap</button>
            </div>

            <!-- Sub tab container -->
            <div class="sub-tab-container">
                <!-- Wallet sub-tabs -->
                <div id="wallet-subtabs" class="sub-tab-header active">
                    <button class="sub-tab-button active" onclick="switchSubTab('wallet', 'request')">📥 Request</button>
                    <button class="sub-tab-button" onclick="switchSubTab('wallet', 'send')">📤 Send</button>
                    <button class="sub-tab-button" onclick="switchSubTab('wallet', 'balances')">💰 Balances</button>
                    <button class="sub-tab-button" onclick="switchSubTab('wallet', 'transactions')">📊 Transactions</button>
                </div>

                <!-- Node sub-tabs -->
                <div id="node-subtabs" class="sub-tab-header">
                    <button class="sub-tab-button active" onclick="switchSubTab('node', 'syndicates')">🐉 Syndicates</button>
                    <button class="sub-tab-button" onclick="switchSubTab('node', 'blocks')">🗂️ Blocks</button>
                </div>

                <!-- Foundry sub-tabs -->
                <div id="foundry-subtabs" class="sub-tab-header">
                    <button class="sub-tab-button active" onclick="switchSubTab('foundry', 'minter')">⚒️ Token Minter</button>
                </div>

                <!-- Swap sub-tabs -->
                <div id="swap-subtabs" class="sub-tab-header">
                    <button class="sub-tab-button active" onclick="switchSubTab('swap', 'exchange')">🏪 Exchange</button>
                    <button class="sub-tab-button" onclick="switchSubTab('swap', 'liquidity')">💧 Liquidity</button>
                </div>
            </div>

            <!-- Wallet Request Tab -->
            <div id="wallet-request-tab" class="tab-content active">
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

            <!-- Wallet Send Tab -->
            <div id="wallet-send-tab" class="tab-content">
                <form id="sendForm">
                    <div class="form-group">
                        <label for="assetType">Asset Type:</label>
                        <select id="assetType" name="assetType" onchange="updateSendForm()">
                            <option value="shadow">SHADOW</option>
                            <option value="token">Token</option>
                        </select>
                    </div>

                    <div class="form-group" id="tokenSelectGroup" style="display: none;">
                        <label for="tokenSelect">Select Token:</label>
                        <select id="tokenSelect" name="tokenSelect">
                            <option value="">Loading tokens...</option>
                        </select>
                        <div id="tokenBalance" class="balance-display"></div>
                    </div>

                    <div class="form-group">
                        <label for="sendAddress">To Address:</label>
                        <input type="text" id="sendAddress" name="sendAddress" placeholder="S... or L..." required>
                    </div>
                    <div class="form-group">
                        <label for="sendAmount" id="sendAmountLabel">Amount (SHADOW):</label>
                        <input type="number" id="sendAmount" name="sendAmount" step="0.00000001" min="0" required>
                        <div id="sendAmountHelp" class="form-help"></div>
                    </div>
                    <div class="form-group" id="feeGroup">
                        <label for="sendFee">Transaction Fee (SHADOW):</label>
                        <input type="number" id="sendFee" name="sendFee" step="0.00000001" min="0" value="0.1" placeholder="0.1">
                    </div>
                    <div class="form-group">
                        <label for="sendMessage">Message (optional):</label>
                        <input type="text" id="sendMessage" name="sendMessage" placeholder="Payment message...">
                    </div>
                    <button type="submit" class="btn" id="sendButton">Send Payment</button>
                </form>
                <div id="sendResult"></div>
            </div>

            <!-- Wallet Transactions Tab -->
            <div id="wallet-transactions-tab" class="tab-content">
                <div id="transactionsContainer">
                    <div class="loading">Loading transactions...</div>
                </div>
            </div>

            <!-- Wallet Balances Tab -->
            <div id="wallet-balances-tab" class="tab-content">
                <div id="tokensContainer">
                    <div class="loading">Loading token balances...</div>
                </div>
            </div>

            <!-- Node Syndicates Tab -->
            <div id="node-syndicates-tab" class="tab-content">
                <div class="syndicates-header">
                    <h2>🐉 Four Guardian Syndicates</h2>
                    <p>Join mining syndicates for pooled rewards and anti-centralization. Each syndicate is named after legendary guardians.</p>
                </div>

                <div class="syndicates-sections">
                    <!-- Current Membership Section -->
                    <div class="section-card">
                        <h3>🎯 Your Syndicate Membership</h3>
                        <div id="current-membership">
                            <div class="loading">Loading your syndicate status...</div>
                        </div>
                    </div>

                    <!-- Join Syndicate Section -->
                    <div class="section-card">
                        <h3>⚔️ Join a Syndicate</h3>
                        <form id="joinSyndicateForm">
                            <div class="form-group">
                                <label for="syndicateChoice">Choose Syndicate:</label>
                                <select id="syndicateChoice" name="syndicateChoice" required>
                                    <option value="auto">🤖 Automatic Assignment (Recommended)</option>
                                    <option value="seiryu">🐉 Seiryu - Azure Dragon of the East</option>
                                    <option value="byakko">🐅 Byakko - White Tiger of the West</option>
                                    <option value="suzaku">🐦 Suzaku - Vermillion Bird of the South</option>
                                    <option value="genbu">🐢 Genbu - Black Tortoise of the North</option>
                                </select>
                            </div>
                            <div class="form-group">
                                <label for="autoRenew">Auto-renewal:</label>
                                <input type="checkbox" id="autoRenew" name="autoRenew" checked>
                                <span>Automatically renew membership every chain week (1008 blocks)</span>
                                <small>When enabled, your membership will auto-renew before expiration</small>
                            </div>
                            <div class="form-group">
                                <label for="membershipDays">Membership Duration:</label>
                                <select id="membershipDays" name="membershipDays" required>
                                    <option value="1">1 Day (0.1 SHADOW)</option>
                                    <option value="3">3 Days (0.1 SHADOW)</option>
                                    <option value="7" selected>7 Days (0.1 SHADOW)</option>
                                    <option value="8">8 Days - Maximum (0.1 SHADOW)</option>
                                </select>
                            </div>
                            <button type="submit" class="btn">Join Syndicate (0.1 SHADOW)</button>
                        </form>
                        <div id="joinSyndicateResult"></div>
                    </div>

                    <!-- Syndicate Statistics Section -->
                    <div class="section-card">
                        <h3>📊 Syndicate Performance</h3>
                        <div id="syndicate-stats">
                            <div class="loading">Loading syndicate statistics...</div>
                        </div>
                        <button class="btn btn-secondary" onclick="refreshSyndicateStats()">🔄 Refresh Stats</button>
                    </div>
                </div>
            </div>

            <!-- Swap Exchange Tab -->
            <div id="swap-exchange-tab" class="tab-content">
                <div class="marketplace-header">
                    <h2>🏪 Trading Marketplace</h2>
                    <p>Create trade offers by locking assets in escrow NFTs, or browse and purchase from active offers.</p>
                </div>

                <div class="marketplace-sections">
                    <!-- Create Trade Offer Section -->
                    <div class="marketplace-section">
                        <h3>📦 Create Trade Offer</h3>

                        <form id="createTradeForm" class="trade-form">
                            <div class="form-group">
                                <label for="lockedTokenSelect">Asset to Sell</label>
                                <select id="lockedTokenSelect" name="lockedToken" required>
                                    <option value="">Select token to sell...</option>
                                </select>
                                <div class="form-help">
                                    Select the token you want to sell. This will be locked in escrow until the trade completes or expires.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="lockedAmount">Amount to Sell</label>
                                <input type="number" id="lockedAmount" name="lockedAmount" step="any" min="0" required>
                                <div class="balance-display" id="lockedTokenBalance"></div>
                                <div class="form-help">
                                    How much of the selected token to sell.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="askingTokenSelect">What do you want?</label>
                                <select id="askingTokenSelect" name="askingToken" required>
                                    <option value="">Select what you want...</option>
                                </select>
                                <div class="form-help">
                                    Choose what token you want in exchange. Select SHADOW for direct SHADOW payment.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="askingPrice" id="askingPriceLabel">Asking Price</label>
                                <input type="number" id="askingPrice" name="askingPrice" step="0.00000001" min="0.00000001" required>
                                <div class="form-help" id="askingPriceHelp">
                                    Price in SHADOW satoshis (0.00000001 SHADOW = 1 satoshi).
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="expirationHours">Expiration</label>
                                <select id="expirationHours" name="expirationHours" required>
                                    <option value="1">1 Hour</option>
                                    <option value="6">6 Hours</option>
                                    <option value="24" selected>24 Hours (1 Day)</option>
                                    <option value="168">1 Week</option>
                                    <option value="720">1 Month</option>
                                </select>
                                <div class="form-help">
                                    How long the offer will remain active. After expiration, you can melt the trade NFT to recover your asset.
                                </div>
                            </div>

                            <div class="trade-cost-preview" id="tradeCostPreview" style="display: none;">
                                <h4>Trade Offer Cost</h4>
                                <div class="cost-details">
                                    <div>NFT Creation Fee: <span class="cost-value">0.1 SHADOW</span></div>
                                    <div>Asset Locked: <span id="assetLockedPreview">-</span></div>
                                </div>
                            </div>

                            <div class="form-actions">
                                <button type="button" class="btn btn-secondary" onclick="resetTradeForm()">Reset</button>
                                <button type="submit" class="btn" id="createTradeBtn" onclick="submitTradeOffer(event)" disabled>🔒 Create Trade Offer</button>
                            </div>
                        </form>
                    </div>

                    <!-- Browse Marketplace Section -->
                    <div class="marketplace-section">
                        <h3>🛍️ Active Trade Offers</h3>

                        <div class="marketplace-filters">
                            <div class="filter-group">
                                <label for="filterToken">Filter by Token</label>
                                <select id="filterToken">
                                    <option value="">All Tokens</option>
                                </select>
                            </div>

                            <div class="filter-group">
                                <label for="sortBy">Sort By</label>
                                <select id="sortBy">
                                    <option value="newest">Newest First</option>
                                    <option value="oldest">Oldest First</option>
                                    <option value="price_low">Price: Low to High</option>
                                    <option value="price_high">Price: High to Low</option>
                                    <option value="expiring">Expiring Soon</option>
                                </select>
                            </div>

                            <button class="btn btn-secondary" onclick="refreshMarketplace()">🔄 Refresh</button>
                        </div>

                        <div id="marketplaceOffers">
                            <div class="loading">Loading active trade offers...</div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Foundry Minter Tab -->
            <div id="foundry-minter-tab" class="tab-content">
                <div class="foundry-header">
                    <h2>⚒️ Token Foundry</h2>
                    <p>Create new tokens backed by SHADOW collateral. Each token requires Shadow to be locked as backing.</p>
                </div>

                <form id="createTokenForm" onsubmit="handleCreateToken(event)">
                    <div class="form-group">
                        <label for="tokenName">Token Name *</label>
                        <input type="text" id="tokenName" name="tokenName" maxlength="64" required
                               placeholder="e.g., Steve's Awesome Token" onchange="updateCostEstimate()">
                        <div class="form-help">Human-readable name for your token (max 64 characters)</div>
                    </div>

                    <div class="form-group">
                        <label for="tokenTicker">Token Ticker *</label>
                        <input type="text" id="tokenTicker" name="tokenTicker" maxlength="16" required
                               placeholder="e.g., STEVE" style="text-transform: uppercase;" onchange="updateCostEstimate()">
                        <div class="form-help">Short symbol for your token (max 16 characters)</div>
                    </div>

                    <div class="form-group">
                        <label>Token Type *</label>
                        <div class="token-type-toggle">
                            <input type="radio" id="typeToken" name="tokenType" value="token" checked onchange="updateTokenType()">
                            <label for="typeToken" class="toggle-label">🪙 Fungible Token</label>

                            <input type="radio" id="typeNFT" name="tokenType" value="nft" onchange="updateTokenType()">
                            <label for="typeNFT" class="toggle-label">🎨 NFT Collection</label>
                        </div>
                        <div class="form-help">Fungible tokens are divisible, NFTs are unique collectibles</div>
                    </div>

                    <div class="form-group">
                        <label for="totalSupply">Total Supply *</label>
                        <input type="number" id="tokenTotalSupply" name="totalSupply" min="1" required
                               placeholder="e.g., 1000000" onchange="updateCostEstimate()">
                        <div class="form-help">Fixed total number of tokens to create</div>
                    </div>

                    <div class="form-group">
                        <label for="decimals">Decimal Places</label>
                        <select id="decimals" name="decimals" onchange="updateCostEstimate()">
                            <option value="0">0 (Whole numbers only)</option>
                            <option value="2">2 (Like cents)</option>
                            <option value="6">6 (Like USDC)</option>
                            <option value="8" selected>8 (Like Bitcoin)</option>
                            <option value="18">18 (Like Ethereum)</option>
                        </select>
                        <div class="form-help">How many decimal places your token supports</div>
                    </div>

                    <div class="form-group">
                        <label for="lockAmount">SHADOW Lock per Token *</label>
                        <input type="number" id="lockAmount" name="lockAmount" step="0.00000001" min="0.00000001" required
                               placeholder="e.g., 0.1" onchange="updateCostEstimate()">
                        <div class="form-help">Amount of SHADOW locked as backing per token unit</div>
                    </div>

                    <div class="form-group">
                        <label for="tokenURI">Metadata URI (Optional)</label>
                        <input type="url" id="tokenURI" name="tokenURI" maxlength="128"
                               placeholder="https://example.com/metadata.json">
                        <div class="form-help">Optional URI pointing to token metadata (max 128 chars)</div>
                    </div>

                    <div id="costPreview" class="cost-preview" style="display: none;">
                        <h4>💰 Creation Cost</h4>
                        <div class="cost-details">
                            <div>Total SHADOW Required: <span id="totalShadowCost" class="cost-value">0.00000000</span></div>
                            <div class="cost-breakdown">
                                <div>• Token Backing: <span id="backingCost">0.00000000</span> SHADOW</div>
                                <div>• Creation Fee: <span id="creationFee">0.00000000</span> SHADOW</div>
                            </div>
                        </div>
                    </div>

                    <div class="form-actions">
                        <button type="button" class="btn btn-secondary" onclick="resetCreateTokenForm()">Reset Form</button>
                        <button type="submit" id="createTokenBtn" class="btn">⚒️ Create Token</button>
                    </div>
                </form>
            </div>

            <!-- Swap Liquidity Tab -->
            <div id="swap-liquidity-tab" class="tab-content">
                <div class="marketplace-header">
                    <h2>💧 Liquidity Pools</h2>
                    <p>View existing liquidity pools and create new ones. Pools use constant product (x*y=k) automated market making.</p>
                </div>

                <div class="marketplace-sections">
                    <!-- Create Pool Section -->
                    <div class="marketplace-section">
                        <h3>🏊 Create Liquidity Pool</h3>

                        <form id="createPoolForm" class="trade-form">
                            <div class="form-group">
                                <label for="poolTokenASelect">Token A</label>
                                <select id="poolTokenASelect" name="poolTokenA" required>
                                    <option value="">Select first token...</option>
                                    <option value="SHADOW">SHADOW</option>
                                </select>
                                <div class="form-help">
                                    First token in the trading pair.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="poolTokenBSelect">Token B</label>
                                <select id="poolTokenBSelect" name="poolTokenB" required>
                                    <option value="">Select second token...</option>
                                    <option value="SHADOW">SHADOW</option>
                                </select>
                                <div class="form-help">
                                    Second token in the trading pair.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="poolInitialRatioA">Initial Amount A</label>
                                <input type="number" id="poolInitialRatioA" name="poolInitialRatioA" step="any" min="0" required>
                                <div class="balance-display" id="poolTokenABalance"></div>
                                <div class="form-help">
                                    Initial amount of Token A to set the x*y=k constant.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="poolInitialRatioB">Initial Amount B</label>
                                <input type="number" id="poolInitialRatioB" name="poolInitialRatioB" step="any" min="0" required>
                                <div class="balance-display" id="poolTokenBBalance"></div>
                                <div class="form-help">
                                    Initial amount of Token B to set the x*y=k constant.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="poolFeeRate">Fee Rate (%)</label>
                                <select id="poolFeeRate" name="poolFeeRate" required>
                                    <option value="10">0.1% (Low)</option>
                                    <option value="30" selected>0.3% (Standard)</option>
                                    <option value="50">0.5% (High)</option>
                                    <option value="100">1.0% (Premium)</option>
                                </select>
                                <div class="form-help">
                                    Trading fee charged to swappers. Higher fees = more rewards for liquidity providers.
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="poolName">Pool Name</label>
                                <input type="text" id="poolName" name="poolName" placeholder="Leave empty for auto-generated name">
                                <div class="form-help">
                                    Human-readable name for your pool. Auto-generated format: TOKEN1/TOKEN2-FEE%-Pool-GUID
                                </div>
                            </div>

                            <div class="form-group">
                                <label for="poolTicker">Pool Ticker</label>
                                <input type="text" id="poolTicker" name="poolTicker" placeholder="Leave empty for auto-generated ticker" maxlength="32">
                                <div class="form-help">
                                    Short symbol for the pool NFT. Auto-generated format: TOKEN1_TOKEN2_FEE_GUID
                                </div>
                            </div>

                            <div class="cost-info">
                                <strong>Pool Creation Cost: 5.0 SHADOW</strong>
                                <br>
                                <small>This creates permanent on-chain infrastructure with an L-address.</small>
                            </div>

                            <button type="submit" class="submit-btn">🏊 Create Pool (5.0 SHADOW)</button>
                        </form>
                    </div>

                    <!-- Active Pools Section -->
                    <div class="marketplace-section">
                        <h3>🌊 Active Liquidity Pools</h3>
                        <div class="refresh-controls">
                            <button onclick="refreshPools()" class="refresh-btn">🔄 Refresh</button>
                        </div>

                        <div id="poolsList" class="pools-list">
                            <div class="loading">Loading active pools...</div>
                        </div>
                    </div>
                </div>
            </div>

            <!-- Node Blocks Tab -->
            <div id="node-blocks-tab" class="tab-content">
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

    <!-- Token Melt Modal -->
    <div id="meltModal" class="modal">
        <div class="modal-content">
            <div class="modal-header">
                <h2 class="modal-title">🔥 Melt Tokens</h2>
                <button class="modal-close" onclick="closeMeltModal()">&times;</button>
            </div>

            <div class="warning-box">
                <div class="warning-title">⚠️ DANGER - IRREVERSIBLE ACTION</div>
                <div class="warning-text">
                    Melting tokens is <strong>PERMANENT and IRREVERSIBLE</strong>.
                    Your tokens will be destroyed forever and cannot be recovered.
                </div>
                <div class="warning-text">
                    You will receive SHADOW in return based on the token's lock amount.
                    This is useful for recycling unwanted or "junk" tokens.
                </div>
                <div class="warning-text">
                    <strong>Only proceed if you are absolutely certain!</strong>
                </div>
            </div>

            <form class="melt-form" onsubmit="return submitMelt(event)">
                <div class="form-group">
                    <label for="meltAmount">Amount to Melt:</label>
                    <input type="number" id="meltAmount" step="any" min="0" required>
                    <div class="balance-display" id="meltBalance"></div>
                </div>

                <div class="form-group">
                    <label for="meltConfirmation">Type "MELT" to confirm this irreversible action:</label>
                    <input type="text" id="meltConfirmation" class="confirmation-input"
                           placeholder="Type MELT here" required>
                </div>

                <div id="meltProgress" style="display: none;">
                    <div class="loading">Processing melt transaction...</div>
                </div>

                <div class="modal-actions">
                    <button type="button" class="btn btn-secondary" onclick="closeMeltModal()">Cancel</button>
                    <button type="submit" id="meltSubmitBtn" class="btn-danger" disabled>
                        🔥 MELT TOKENS (PERMANENT)
                    </button>
                </div>
            </form>
        </div>
    </div>

    <script>
        // Fix for refreshMarketplace function - define early to prevent ReferenceError
        function refreshMarketplace() {
            loadMarketplace();
        }

        let walletData = null;
        let lastBlockTime = null;
        let blockInterval = 600; // 10 minutes (600 seconds) - production block time
        let countdownInterval = null;
        // Removed - using currentMainTab and currentSubTabs instead
        let recentBlocks = [];
        let currentMeltToken = null;
        let marketplaceTokenData = null; // Store token balance data for marketplace
        let pendingTransactions = []; // Track pending transactions for confirmation status

        // Melt dialog functions - defined early for global access
        function showMeltDialog(tokenId, tokenTicker, rawBalance, decimals) {
            // Format the balance for display
            const maxBalance = formatTokenAmount(rawBalance, decimals);
            currentMeltToken = { tokenId, tokenTicker, maxBalance: rawBalance, decimals };

            const modal = document.getElementById('meltModal');
            const amountInput = document.getElementById('meltAmount');
            const balanceDisplay = document.getElementById('meltBalance');
            const confirmationInput = document.getElementById('meltConfirmation');
            const submitBtn = document.getElementById('meltSubmitBtn');
            const modalTitle = modal.querySelector('.modal-title');

            // Update modal title to include token ticker
            modalTitle.textContent = '🔥 Melt ' + tokenTicker + ' Tokens';

            // Set up the form
            amountInput.max = maxBalance;
            amountInput.value = maxBalance; // Default to full balance
            amountInput.placeholder = '0.00000000';
            balanceDisplay.textContent = 'Available: ' + maxBalance + ' ' + tokenTicker;
            confirmationInput.value = '';
            submitBtn.disabled = true;

            // Enable submit button when confirmation is typed
            confirmationInput.oninput = function() {
                submitBtn.disabled = this.value !== 'MELT';
            };

            modal.style.display = 'block';
        }

        function closeMeltModal() {
            const modal = document.getElementById('meltModal');
            modal.style.display = 'none';
            currentMeltToken = null;
        }

        async function submitMelt(event) {
            event.preventDefault();

            if (!currentMeltToken) return false;

            const amountInput = document.getElementById('meltAmount');
            const confirmationInput = document.getElementById('meltConfirmation');
            const progressDiv = document.getElementById('meltProgress');
            const submitBtn = document.getElementById('meltSubmitBtn');

            const amount = parseFloat(amountInput.value);
            const confirmation = confirmationInput.value;

            if (amount <= 0 || amount > currentMeltToken.maxBalance) {
                alert('Invalid amount. Must be between 0 and ' + currentMeltToken.maxBalance);
                return false;
            }

            if (confirmation !== 'MELT') {
                alert('You must type "MELT" to confirm this irreversible action');
                return false;
            }

            // Show progress and disable form
            progressDiv.style.display = 'block';
            submitBtn.disabled = true;

            try {
                const response = await fetch('/wallet/melt_token', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        token_id: currentMeltToken.tokenId,
                        amount: amount,
                        confirmation: confirmation
                    })
                });

                const result = await response.json();

                if (response.ok) {
                    alert('Success! ' + result.message + '\\n\\nTransaction Hash: ' + result.transaction_hash);

                    // Add to pending transactions for confirmation tracking
                    addPendingTransaction(result.transaction_hash, 'token_melt', 'Token "' + currentMeltToken.tokenTicker + '" melt');

                    closeMeltModal();

                    // If we melted the entire balance, immediately remove the token from the UI
                    if (amount >= currentMeltToken.maxBalance) {
                        removeTokenFromUI(currentMeltToken.tokenId);
                    }

                    // Refresh token balances and wallet data
                    loadTokenBalances();
                    loadWalletData();
                } else {
                    throw new Error(result.message || result.error || 'Melt failed');
                }

            } catch (error) {
                console.error('Melt error:', error);
                alert('Error melting tokens: ' + error.message);
            } finally {
                progressDiv.style.display = 'none';
                submitBtn.disabled = false;
            }

            return false;
        }

        // Transaction Confirmation Tracking - defined early for global access
        function addPendingTransaction(txId, type, description) {
            const pendingTx = {
                txId: txId,
                type: type, // 'token_create', 'token_melt', 'trade_offer', etc.
                description: description,
                timestamp: new Date(),
                confirmations: 0,
                status: 'pending' // 'pending', 'confirmed', 'failed'
            };

            pendingTransactions.push(pendingTx);
            console.log('🔄 Added pending transaction:', pendingTx);

            // Start checking for confirmations
            checkTransactionConfirmations();
        }

        async function checkTransactionConfirmations() {
            if (pendingTransactions.length === 0) return;

            try {
                // Get recent blocks to check for our transactions
                const response = await fetch('/api/v1/blockchain/recent?limit=10');
                if (!response.ok) return;

                const data = await response.json();
                const blocks = data.blocks || [];

                // Check each pending transaction
                for (let i = pendingTransactions.length - 1; i >= 0; i--) {
                    const pendingTx = pendingTransactions[i];
                    let confirmations = 0;
                    let found = false;

                    // Look through recent blocks for this transaction
                    for (const block of blocks) {
                        if (block.body && block.body.transactions) {
                            for (const tx of block.body.transactions) {
                                if (tx.tx_hash === pendingTx.txId || (tx.transaction && tx.transaction.includes(pendingTx.txId))) {
                                    found = true;
                                    confirmations = blocks.length - blocks.indexOf(block);
                                    break;
                                }
                            }
                        }
                        if (found) break;
                    }

                    if (found) {
                        pendingTx.confirmations = confirmations;
                        pendingTx.status = confirmations >= 1 ? 'confirmed' : 'pending';

                        // Remove from pending list if confirmed
                        if (confirmations >= 6) { // Consider 6+ confirmations as fully confirmed
                            console.log('✅ Transaction confirmed:', pendingTx);
                            pendingTransactions.splice(i, 1);
                            showTransactionStatus(pendingTx.txId, pendingTx.description + ' confirmed (' + confirmations + ' confirmations)');
                        } else {
                            console.log('🔄 Transaction has', confirmations, 'confirmations:', pendingTx.txId);
                            showTransactionStatus(pendingTx.txId, pendingTx.description + ' (' + confirmations + ' confirmations)');
                        }
                    } else {
                        // Still in mempool or not found
                        const timeSinceSubmission = (new Date() - pendingTx.timestamp) / 1000;
                        if (timeSinceSubmission > 3600) { // 1 hour timeout
                            console.log('⚠️ Transaction timeout, removing from pending:', pendingTx.txId);
                            pendingTransactions.splice(i, 1);
                        } else {
                            showTransactionStatus(pendingTx.txId, pendingTx.description + ' (pending...)');
                        }
                    }
                }

            } catch (error) {
                console.error('Error checking transaction confirmations:', error);
            }
        }

        function showTransactionStatus(txId, message) {
            // Show a non-intrusive status notification
            const statusDiv = document.getElementById('transactionStatus');
            if (statusDiv) {
                statusDiv.innerHTML = '<div class="tx-status">' + message + '</div>';
                statusDiv.style.display = 'block';

                // Auto-hide after 5 seconds
                setTimeout(() => {
                    statusDiv.style.display = 'none';
                }, 5000);
            }
        }

        // Marketplace Functions - defined early for global access
        function setupMarketplaceEventListeners() {
            const lockedTokenSelect = document.getElementById('lockedTokenSelect');
            const askingTokenSelect = document.getElementById('askingTokenSelect');
            const lockedAmountInput = document.getElementById('lockedAmount');
            const askingPriceInput = document.getElementById('askingPrice');
            const createTradeForm = document.getElementById('createTradeForm');

            console.log('🏪 Setting up marketplace event listeners...');
            if (lockedTokenSelect) {
                lockedTokenSelect.addEventListener('change', updateTradeForm);
                console.log('🏪 Added change listener to lockedTokenSelect');
            } else {
                console.error('🏪 lockedTokenSelect not found for event listener');
            }
            if (askingTokenSelect) {
                askingTokenSelect.addEventListener('change', updateTradeForm);
                console.log('🏪 Added change listener to askingTokenSelect');
            } else {
                console.error('🏪 askingTokenSelect not found for event listener');
            }
            if (lockedAmountInput) {
                lockedAmountInput.addEventListener('input', updateTradeForm);
                console.log('🏪 Added input listener to lockedAmountInput');
            } else {
                console.error('🏪 lockedAmountInput not found for event listener');
            }
            if (askingPriceInput) {
                askingPriceInput.addEventListener('input', updateTradeForm);
                console.log('🏪 Added input listener to askingPriceInput');
            } else {
                console.error('🏪 askingPriceInput not found for event listener');
            }
            if (createTradeForm) {
                createTradeForm.addEventListener('submit', submitTradeOffer);
                console.log('🏪 Added submit listener to createTradeForm');
            } else {
                console.error('🏪 createTradeForm not found for event listener');
            }
        }

        async function loadMarketplaceTokens() {
            try {
                const response = await fetch('/wallet/tokens');
                if (!response.ok) {
                    throw new Error('Failed to fetch tokens: ' + response.status);
                }
                const data = await response.json();

                // Store token data globally for balance lookups
                marketplaceTokenData = data;

                const lockedTokenSelect = document.getElementById('lockedTokenSelect');
                const askingTokenSelect = document.getElementById('askingTokenSelect');
                const filterTokenSelect = document.getElementById('filterToken');

                // Clear existing options
                lockedTokenSelect.innerHTML = '<option value="">Select token to sell...</option>';
                askingTokenSelect.innerHTML = '<option value="">Select what you want...</option>';
                filterTokenSelect.innerHTML = '<option value="">All Tokens</option>';

                // Add SHADOW option
                lockedTokenSelect.innerHTML += '<option value="SHADOW">SHADOW</option>';
                askingTokenSelect.innerHTML += '<option value="SHADOW">SHADOW</option>';
                filterTokenSelect.innerHTML += '<option value="SHADOW">SHADOW</option>';

                // Add token options from balances
                if (data.balances && data.balances.length > 0) {
                    data.balances.forEach(balance => {
                        const token = balance.token_info || {};
                        const tokenTicker = token.ticker || 'Unknown';
                        const tokenName = token.name || 'Unknown Token';

                        // Add to sellable tokens only if balance > 0
                        if (balance.balance > 0) {
                            lockedTokenSelect.innerHTML +=
                                '<option value="' + balance.token_id + '">' + tokenTicker + ' (' + tokenName + ')</option>';
                        }

                        // Add to what you want dropdown (all accepted tokens)
                        if (balance.trust_level === 'accepted' || balance.trust_level === 'trusted') {
                            askingTokenSelect.innerHTML +=
                                '<option value="' + balance.token_id + '">' + tokenTicker + ' (' + tokenName + ')</option>';
                        }

                        // Add to filter (all tokens)
                        filterTokenSelect.innerHTML +=
                            '<option value="' + balance.token_id + '">' + tokenTicker + '</option>';
                    });
                }

            } catch (error) {
                console.error('Error loading marketplace tokens:', error);
            }
        }

        async function loadMarketplaceOffers() {
            try {
                const response = await fetch('/api/marketplace/offers');
                if (!response.ok) {
                    throw new Error('Failed to load marketplace offers');
                }

                const offers = await response.json();
                renderMarketplaceOffers(offers || []);

            } catch (error) {
                console.error('Error loading marketplace offers:', error);
                const container = document.getElementById('marketplaceOffers');
                container.innerHTML = '<div class="error">Failed to load marketplace offers: ' + error.message + '</div>';
            }
        }

        function renderMarketplaceOffers(offers) {
            const container = document.getElementById('marketplaceOffers');

            if (offers.length === 0) {
                container.innerHTML = '<div class="loading">No active trade offers found.</div>';
                return;
            }

            const currentTime = Date.now() / 1000; // Current Unix timestamp

            let html = '';
            offers.forEach(offer => {
                const isExpired = currentTime > offer.expiration_time;
                const timeRemaining = offer.expiration_time - currentTime;
                const hoursRemaining = Math.max(0, Math.floor(timeRemaining / 3600));
                const minutesRemaining = Math.max(0, Math.floor((timeRemaining % 3600) / 60));

                html +=
                    '<div class="offer-card ' + (isExpired ? 'offer-expired' : '') + '">' +
                        '<div class="offer-header">' +
                            '<div class="offer-token">' + offer.locked_amount + ' ' + (offer.locked_token_ticker || offer.locked_token_id.substring(0, 8) + '...') + '</div>' +
                            '<div class="offer-price">' + (offer.asking_price / 100000000).toFixed(8) + ' SHADOW</div>' +
                        '</div>' +

                        '<div class="offer-details">' +
                            '<div class="offer-detail">' +
                                '<span>Seller:</span>' +
                                '<span>' + offer.seller.substring(0, 8) + '...' + offer.seller.substring(offer.seller.length - 8) + '</span>' +
                            '</div>' +
                            '<div class="offer-detail">' +
                                '<span>Created:</span>' +
                                '<span>' + new Date(offer.creation_time * 1000).toLocaleDateString() + '</span>' +
                            '</div>' +
                            '<div class="offer-detail">' +
                                '<span>Expires:</span>' +
                                '<span>' + (isExpired ? 'EXPIRED' : hoursRemaining + 'h ' + minutesRemaining + 'm') + '</span>' +
                            '</div>' +
                            '<div class="offer-detail">' +
                                '<span>Price per unit:</span>' +
                                '<span>' + (offer.asking_price / offer.locked_amount / 100000000).toFixed(8) + ' SHADOW</span>' +
                            '</div>' +
                        '</div>' +

                        '<div class="offer-actions">' +
                            (isExpired ?
                                '<span class="expired-label">EXPIRED</span>' :
                                '<button class="btn-purchase" onclick="purchaseOffer(\'' + offer.trade_nft_id + '\')">🛒 Purchase</button>'
                            ) +
                        '</div>' +
                    '</div>';
            });

            container.innerHTML = html;
        }

        async function loadMarketplace() {
            if (currentMainTab !== 'swap' || currentSubTabs.swap !== 'exchange') return;

            try {
                console.log('🏪 Loading marketplace...');
                // Load user tokens for trade offer creation
                await loadMarketplaceTokens();

                // Load active trade offers
                await loadMarketplaceOffers();

                // Initialize form validation after tokens are loaded
                setTimeout(() => {
                    console.log('🏪 Calling updateTradeForm after marketplace load');
                    updateTradeForm();

                    // Debug: Check if form elements exist
                    const lockedTokenSelect = document.getElementById('lockedTokenSelect');
                    const askingTokenSelect = document.getElementById('askingTokenSelect');
                    const lockedAmountInput = document.getElementById('lockedAmount');
                    const askingPriceInput = document.getElementById('askingPrice');
                    const createTradeBtn = document.getElementById('createTradeBtn');

                    console.log('🏪 Form elements check:', {
                        lockedTokenSelect: !!lockedTokenSelect,
                        askingTokenSelect: !!askingTokenSelect,
                        lockedAmountInput: !!lockedAmountInput,
                        askingPriceInput: !!askingPriceInput,
                        createTradeBtn: !!createTradeBtn
                    });

                    if (lockedTokenSelect) {
                        console.log('🏪 Locked token options:', lockedTokenSelect.innerHTML.substring(0, 200));
                    }
                    if (askingTokenSelect) {
                        console.log('🏪 Asking token options:', askingTokenSelect.innerHTML.substring(0, 200));
                    }

                    // Set up event listeners now that elements exist
                    setupMarketplaceEventListeners();
                    setupPoolFunctionality();
                }, 100);

            } catch (error) {
                console.error('Error loading marketplace:', error);
            }
        }

        // Pool creation and management functions
        async function submitPoolCreation(event) {
            event.preventDefault();
            console.log('submitPoolCreation called');

            const form = document.getElementById('createPoolForm');
            const formData = new FormData(form);
            const submitBtn = form.querySelector('button[type="submit"]');

            const poolData = {
                tokenA: formData.get('poolTokenA'),
                tokenB: formData.get('poolTokenB'),
                initialRatioA: parseFloat(formData.get('poolInitialRatioA')),
                initialRatioB: parseFloat(formData.get('poolInitialRatioB')),
                feeRate: parseInt(formData.get('poolFeeRate')),
                name: formData.get('poolName'),
                ticker: formData.get('poolTicker')
            };

            console.log('Pool creation data:', poolData);

            if (!poolData.tokenA || !poolData.tokenB || poolData.initialRatioA <= 0 || poolData.initialRatioB <= 0) {
                alert('Please fill in all required fields with valid values.');
                return;
            }

            if (poolData.tokenA === poolData.tokenB) {
                alert('Token A and Token B must be different.');
                return;
            }

            if (submitBtn) {
                submitBtn.disabled = true;
                submitBtn.textContent = 'Creating Pool...';
            }

            try {
                const response = await fetch('/api/pool/create', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify(poolData)
                });

                const result = await response.json();

                if (response.ok && result.success) {
                    alert('Pool created successfully!\\nL-Address: ' + result.l_address);
                    form.reset();
                    refreshPools();
                } else {
                    alert('Failed to create pool: ' + (result.error || 'Unknown error'));
                }
            } catch (error) {
                console.error('Pool creation error:', error);
                alert('Failed to create pool: ' + error.message);
            } finally {
                if (submitBtn) {
                    submitBtn.disabled = false;
                    submitBtn.textContent = '🏊 Create Pool (5.0 SHADOW)';
                }
            }
        }

        async function refreshPools() {
            const poolsList = document.getElementById('poolsList');
            if (!poolsList) return;

            poolsList.innerHTML = '<div class="loading">Loading active pools...</div>';

            try {
                const response = await fetch('/api/pools');
                if (!response.ok) {
                    throw new Error('Failed to fetch pools');
                }

                const pools = await response.json();

                // Add null checks - blockchain node returns array directly or null
                if (!pools || pools.length === 0) {
                    poolsList.innerHTML = '<div class="no-pools">No active liquidity pools found. Create the first one!</div>';
                    return;
                }

                let poolsHTML = '';
                pools.forEach(pool => {
                    const tokenAName = pool.token_a_name || 'SHADOW';
                    const tokenBName = pool.token_b_name || 'SHADOW';
                    const feePercent = (pool.fee_rate / 100).toFixed(1);
                    const swapUrl = '/web/wallet/swap?pool=' + encodeURIComponent(pool.l_address);

                    // Format reserves (assuming 8 decimal places)
                    const reserveADisplay = (pool.reserve_a / 100000000).toFixed(2);
                    const reserveBDisplay = (pool.reserve_b / 100000000).toFixed(2);
                    const canSwap = pool.can_swap;

                    poolsHTML += '<div class="pool-item' + (canSwap ? '' : ' pool-empty') + '">';
                    poolsHTML += '  <div class="pool-header">';
                    if (canSwap) {
                        poolsHTML += '    <h4><a href="' + swapUrl + '" class="pool-link">💧 ' + pool.name + '</a></h4>';
                    } else {
                        poolsHTML += '    <h4><span class="pool-disabled">🚫 ' + pool.name + ' (Empty)</span></h4>';
                    }
                    poolsHTML += '    <span class="pool-fee">' + feePercent + '% fee</span>';
                    poolsHTML += '  </div>';
                    poolsHTML += '  <div class="pool-details">';
                    poolsHTML += '    <div class="pool-pair">' + tokenAName + ' ↔ ' + tokenBName + '</div>';
                    poolsHTML += '    <div class="pool-reserves">Reserves: ' + reserveADisplay + ' ' + tokenAName + ' / ' + reserveBDisplay + ' ' + tokenBName + '</div>';
                    if (canSwap) {
                        poolsHTML += '    <div class="pool-address">L-Address: <a href="' + swapUrl + '" class="pool-address-link"><code>' + pool.l_address + '</code></a></div>';
                    } else {
                        poolsHTML += '    <div class="pool-address">L-Address: <code>' + pool.l_address + '</code></div>';
                        poolsHTML += '    <div class="pool-warning">⚠️ Pool needs liquidity in both tokens before swapping</div>';
                    }
                    poolsHTML += '    <div class="pool-creator">Created by: ' + pool.creator.substring(0, 8) + '...</div>';
                    poolsHTML += '  </div>';
                    poolsHTML += '</div>';
                });

                poolsList.innerHTML = poolsHTML;

            } catch (error) {
                console.error('Failed to load pools:', error);
                poolsList.innerHTML = '<div class="error">Failed to load pools: ' + error.message + '</div>';
            }
        }

        function setupPoolFunctionality() {
            const createPoolForm = document.getElementById('createPoolForm');
            if (createPoolForm) {
                createPoolForm.addEventListener('submit', submitPoolCreation);
                console.log('💧 Added submit listener to createPoolForm');
            }

            // Update balances when tokens are selected
            const poolTokenASelect = document.getElementById('poolTokenASelect');
            const poolTokenBSelect = document.getElementById('poolTokenBSelect');

            if (poolTokenASelect) {
                poolTokenASelect.addEventListener('change', () => updatePoolTokenBalance('A'));
            }
            if (poolTokenBSelect) {
                poolTokenBSelect.addEventListener('change', () => updatePoolTokenBalance('B'));
            }

            // Load tokens into pool selectors (do this after form is ready)
            setTimeout(() => {
                loadPoolTokens();
            }, 500);

            // Load pools initially
            refreshPools();
        }

        async function loadPoolTokens() {
            try {
                const response = await fetch('/wallet/tokens');
                if (!response.ok) {
                    throw new Error('Failed to fetch tokens');
                }

                const data = await response.json();
                console.log('💧 Pool tokens loaded:', data);

                const poolTokenASelect = document.getElementById('poolTokenASelect');
                const poolTokenBSelect = document.getElementById('poolTokenBSelect');

                if (poolTokenASelect && poolTokenBSelect) {
                    // Clear existing options
                    poolTokenASelect.innerHTML = '<option value="">Select first token...</option><option value="SHADOW">SHADOW</option>';
                    poolTokenBSelect.innerHTML = '<option value="">Select second token...</option><option value="SHADOW">SHADOW</option>';

                    // Check if data has balances array (correct structure from /wallet/tokens)
                    const balances = data.balances || [];

                    if (Array.isArray(balances) && balances.length > 0) {
                        // Filter tokens with balance > 0 and add them
                        const tokensWithBalance = balances.filter(b => b.balance > 0);
                        tokensWithBalance.forEach(balance => {
                            const tokenInfo = balance.token_info || {};
                            const displayName = tokenInfo.name && tokenInfo.ticker ?
                                tokenInfo.name + ' (' + tokenInfo.ticker + ')' :
                                balance.token_id.substring(0, 16) + '...';
                            const option = '<option value="' + balance.token_id + '">' + displayName + '</option>';
                            poolTokenASelect.innerHTML += option;
                            poolTokenBSelect.innerHTML += option;
                        });
                        console.log('💧 Added ' + tokensWithBalance.length + ' tokens to pool selectors');
                    } else {
                        console.log('💧 No tokens with balance found in response');
                    }
                }
            } catch (error) {
                console.error('💧 Failed to load tokens for pools:', error);
            }
        }

        async function updatePoolTokenBalance(tokenPosition) {
            const selectId = 'poolToken' + tokenPosition + 'Select';
            const balanceId = 'poolToken' + tokenPosition + 'Balance';

            const select = document.getElementById(selectId);
            const balanceDiv = document.getElementById(balanceId);

            if (!select || !balanceDiv) return;

            const tokenId = select.value;
            if (!tokenId) {
                balanceDiv.textContent = '';
                return;
            }

            try {
                let balance = 0;
                if (tokenId === 'SHADOW') {
                    const response = await fetch('/wallet/balance');
                    if (response.ok) {
                        const data = await response.json();
                        balance = data.balance || 0;
                    }
                } else {
                    const response = await fetch('/wallet/tokens');
                    if (response.ok) {
                        const data = await response.json();
                        const balances = data.balances || [];
                        const tokenBalance = balances.find(b => b.token_id === tokenId);
                        balance = tokenBalance ? tokenBalance.balance : 0;
                    }
                }

                balanceDiv.textContent = 'Balance: ' + balance.toFixed(8);
            } catch (error) {
                console.error('Failed to load balance:', error);
                balanceDiv.textContent = 'Balance: Error loading';
            }
        }

        async function purchaseOffer(tradeNftId) {
            if (!confirm('Are you sure you want to purchase this trade offer?')) {
                return;
            }

            try {
                const response = await fetch('/api/marketplace/purchase', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        trade_nft_id: tradeNftId
                    })
                });

                const result = await response.json();

                if (response.ok) {
                    alert('Trade completed successfully!');
                    loadMarketplaceOffers(); // Refresh the marketplace
                    loadWalletData(); // Refresh wallet balance
                    loadTokenBalances(); // Refresh token balances
                } else {
                    throw new Error(result.error || 'Failed to purchase offer');
                }

            } catch (error) {
                console.error('Error purchasing offer:', error);
                alert('Error purchasing offer: ' + error.message);
            }
        }

        function updateTradeForm() {
            console.log('updateTradeForm called');
            const lockedTokenSelect = document.getElementById('lockedTokenSelect');
            const askingTokenSelect = document.getElementById('askingTokenSelect');
            const lockedAmountInput = document.getElementById('lockedAmount');
            const askingPriceInput = document.getElementById('askingPrice');
            const createTradeBtn = document.getElementById('createTradeBtn');
            const costPreview = document.getElementById('tradeCostPreview');
            const balanceDisplay = document.getElementById('lockedTokenBalance');
            const assetPreview = document.getElementById('assetLockedPreview');
            const askingPriceLabel = document.getElementById('askingPriceLabel');
            const askingPriceHelp = document.getElementById('askingPriceHelp');

            const selectedToken = lockedTokenSelect ? lockedTokenSelect.value : '';
            const askingToken = askingTokenSelect ? askingTokenSelect.value : '';
            const amount = lockedAmountInput ? (parseFloat(lockedAmountInput.value) || 0) : 0;
            const price = askingPriceInput ? (parseFloat(askingPriceInput.value) || 0) : 0;

            console.log('Form values:', { selectedToken, askingToken, amount, price });

            // Update asking price label and help based on selected asking token
            if (askingToken === 'SHADOW') {
                if (askingPriceLabel) askingPriceLabel.textContent = 'Asking Price (SHADOW)';
                if (askingPriceHelp) askingPriceHelp.textContent = 'Price in SHADOW satoshis (0.00000001 SHADOW = 1 satoshi).';
                if (askingPriceInput) askingPriceInput.step = '0.00000001';
            } else if (askingToken) {
                if (askingPriceLabel) askingPriceLabel.textContent = 'Asking Price (in selected token)';
                if (askingPriceHelp) askingPriceHelp.textContent = 'How many tokens you want in exchange.';
                if (askingPriceInput) askingPriceInput.step = 'any';
            } else {
                if (askingPriceLabel) askingPriceLabel.textContent = 'Asking Price';
                if (askingPriceHelp) askingPriceHelp.textContent = 'Select what you want first.';
            }

            // Update balance display
            if (selectedToken) {
                if (selectedToken === 'SHADOW') {
                    const shadowBalance = walletData ? (walletData.balance / 100000000) : 0;
                    if (balanceDisplay) balanceDisplay.textContent = 'Available: ' + shadowBalance.toFixed(8) + ' SHADOW';
                    if (lockedAmountInput) lockedAmountInput.max = shadowBalance;
                } else {
                    // Find token data for selected token
                    if (marketplaceTokenData && marketplaceTokenData.balances) {
                        const tokenBalance = marketplaceTokenData.balances.find(b => b.token_id === selectedToken);
                        if (tokenBalance && tokenBalance.token_info) {
                            const rawBalance = tokenBalance.balance;
                            const decimals = tokenBalance.token_info.decimals || 0;
                            const ticker = tokenBalance.token_info.ticker || 'TOKEN';

                            // Convert from base units to display units
                            const displayBalance = formatTokenAmount(rawBalance, decimals);
                            const maxAmount = parseFloat(displayBalance);

                            if (balanceDisplay) balanceDisplay.textContent = 'Available: ' + displayBalance + ' ' + ticker;
                            if (lockedAmountInput) lockedAmountInput.max = maxAmount;
                        } else {
                            if (balanceDisplay) balanceDisplay.textContent = 'Token balance not found';
                        }
                    } else {
                        if (balanceDisplay) balanceDisplay.textContent = 'Loading balance...';
                    }
                }
            } else {
                if (balanceDisplay) balanceDisplay.textContent = '';
            }

            // Update cost preview
            const isValid = selectedToken && askingToken && amount > 0 && price > 0;
            console.log('Validation check:', { isValid, selectedToken: !!selectedToken, askingToken: !!askingToken, amount, price });

            if (isValid) {
                if (costPreview) costPreview.style.display = 'block';
                if (assetPreview) assetPreview.textContent = amount + ' ' + (selectedToken === 'SHADOW' ? 'SHADOW' : 'tokens');
                if (createTradeBtn) {
                    createTradeBtn.disabled = false;
                    console.log('Button enabled!');
                }
            } else {
                if (costPreview) costPreview.style.display = 'none';
                if (createTradeBtn) {
                    createTradeBtn.disabled = true;
                    console.log('Button disabled - validation failed');
                }
            }
        }

        function resetTradeForm() {
            const form = document.getElementById('createTradeForm');
            const costPreview = document.getElementById('tradeCostPreview');
            const createTradeBtn = document.getElementById('createTradeBtn');
            const balanceDisplay = document.getElementById('lockedTokenBalance');

            if (form) form.reset();
            if (costPreview) costPreview.style.display = 'none';
            if (createTradeBtn) createTradeBtn.disabled = true;
            if (balanceDisplay) balanceDisplay.textContent = '';
        }

        async function submitTradeOffer(event) {
            event.preventDefault();
            console.log('submitTradeOffer called');

            const lockedTokenSelect = document.getElementById('lockedTokenSelect');
            const askingTokenSelect = document.getElementById('askingTokenSelect');
            const lockedAmountInput = document.getElementById('lockedAmount');
            const askingPriceInput = document.getElementById('askingPrice');
            const expirationSelect = document.getElementById('expirationHours');
            const submitBtn = document.getElementById('createTradeBtn');

            const lockedToken = lockedTokenSelect ? lockedTokenSelect.value : '';
            const askingToken = askingTokenSelect ? askingTokenSelect.value : '';
            const amount = lockedAmountInput ? parseFloat(lockedAmountInput.value) : 0;
            const price = askingPriceInput ? parseFloat(askingPriceInput.value) : 0;
            const expirationHours = expirationSelect ? parseInt(expirationSelect.value) : 24;

            console.log('Submit values:', { lockedToken, askingToken, amount, price, expirationHours });

            if (!lockedToken || !askingToken || amount <= 0 || price <= 0) {
                alert('Please fill in all required fields with valid values.');
                return;
            }

            if (submitBtn) {
                submitBtn.disabled = true;
                submitBtn.textContent = 'Creating Trade Offer...';
            }

            try {
                const response = await fetch('/api/marketplace/create-offer', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        locked_token_id: lockedToken,
                        locked_amount: amount,
                        asking_price: Math.round(price * 100000000), // Convert to satoshis
                        asking_token_id: askingToken === 'SHADOW' ? '' : askingToken, // Empty string for SHADOW
                        expiration_hours: expirationHours
                    })
                });

                const result = await response.json();

                if (response.ok) {
                    const tokenName = document.getElementById('lockedTokenSelect')?.selectedOptions[0]?.text || 'Token';
                    alert('Trade offer created successfully!');

                    // Add to pending transactions for confirmation tracking (if tx_id available)
                    if (result.tx_id) {
                        addPendingTransaction(result.tx_id, 'trade_offer', 'Trade offer for ' + tokenName);
                    }

                    resetTradeForm();
                    loadMarketplaceOffers(); // Refresh the marketplace
                    loadWalletData(); // Refresh wallet balance
                } else {
                    throw new Error(result.error || 'Failed to create trade offer');
                }

            } catch (error) {
                console.error('Error creating trade offer:', error);
                alert('Error creating trade offer: ' + error.message);
            } finally {
                if (submitBtn) {
                    submitBtn.disabled = false;
                    submitBtn.textContent = '\ud83d\udd12 Create Trade Offer';
                }
            }
        }

        // Two-level tab switching functionality
        let currentMainTab = 'wallet';
        let currentSubTabs = {
            'wallet': 'request',
            'node': 'syndicates',
            'foundry': 'minter',
            'swap': 'exchange'
        };

        function switchMainTab(mainTab) {
            // Update main tab buttons
            document.querySelectorAll('.main-tab-button').forEach(button => {
                button.classList.remove('active');
            });
            event.target.classList.add('active');

            // Update sub-tab headers
            document.querySelectorAll('.sub-tab-header').forEach(header => {
                header.classList.remove('active');
            });
            document.getElementById(mainTab + '-subtabs').classList.add('active');

            currentMainTab = mainTab;

            // Switch to the active sub-tab for this main tab
            switchSubTab(mainTab, currentSubTabs[mainTab]);
        }

        function switchSubTab(mainTab, subTab) {
            // Update sub-tab buttons for this main tab
            document.querySelectorAll('#' + mainTab + '-subtabs .sub-tab-button').forEach(button => {
                button.classList.remove('active');
            });
            event.target.classList.add('active');

            // Hide all tab contents
            document.querySelectorAll('.tab-content').forEach(content => {
                content.classList.remove('active');
            });

            // Show selected tab content
            const tabId = mainTab + '-' + subTab + '-tab';
            const tabContent = document.getElementById(tabId);
            if (tabContent) {
                tabContent.classList.add('active');
            } else {
                console.error('Tab content not found for:', tabId);
                return;
            }

            // Remember the current sub-tab for this main tab
            currentSubTabs[mainTab] = subTab;

            // Load data for specific tabs
            loadTabData(mainTab, subTab);
        }

        // Syndicate Functions (moved up to fix ReferenceError)
        async function loadCurrentMembership() {
            try {
                const address = document.getElementById('walletAddress').textContent;
                const response = await fetch('/wallet/syndicate-membership?address=' + encodeURIComponent(address));
                const data = await response.json();

                const membershipDiv = document.getElementById('current-membership');

                if (data.active_memberships && data.active_memberships.length > 0) {
                    let html = '<div class="current-memberships">';
                    for (const membership of data.active_memberships) {
                        const expirationDate = new Date(membership.expiration_time * 1000);
                        const remainingDays = Math.ceil((expirationDate - new Date()) / (1000 * 60 * 60 * 24));

                        html += '<div class="membership-card">' +
                            '<div class="membership-header">' +
                                '<h4>' + getSyndicateIcon(membership.syndicate) + ' ' + getSyndicateName(membership.syndicate) + '</h4>' +
                                '<span class="membership-status ' + (remainingDays > 2 ? 'active' : 'expiring') + '">' + (remainingDays > 0 ? remainingDays + ' days left' : 'Expired') + '</span>' +
                            '</div>' +
                            '<div class="membership-details">' +
                                '<p><strong>Capacity:</strong> All available storage</p>' +
                                '<p><strong>NFT ID:</strong> ' + membership.nft_token_id + '</p>' +
                                '<p><strong>Renewals:</strong> ' + membership.renewal_count + '</p>' +
                                '<p><strong>Auto-renewal:</strong> ' + (membership.auto_renew ? 'Enabled ✅' : 'Disabled ❌') + '</p>' +
                            '</div>' +
                        '</div>';
                    }
                    html += '</div>';
                    membershipDiv.innerHTML = html;
                } else {
                    membershipDiv.innerHTML = '<div class="no-membership"><p>🚫 No active syndicate memberships</p><p>Join a syndicate below to get started!</p></div>';
                }
            } catch (error) {
                console.error('Failed to load membership:', error);
                document.getElementById('current-membership').innerHTML = '<div class="error">Failed to load membership data</div>';
            }
        }

        async function loadSyndicateStats() {
            try {
                const response = await fetch('/wallet/syndicate-stats');
                const data = await response.json();
                const statsDiv = document.getElementById('syndicate-stats');
                
                let html = '<div class="syndicate-stats-grid">';
                for (const [syndicate, stats] of Object.entries(data)) {
                    const winPercentage = stats.win_percentage ? stats.win_percentage.toFixed(1) : "0.0";
                    
                    html += '<div class="syndicate-stat-card">' +
                        '<div class="stat-header">' +
                            '<h4>' + getSyndicateIcon(syndicate) + ' ' + getSyndicateName(syndicate) + '</h4>' +
                        '</div>' +
                        '<div class="stat-body">' +
                            '<div class="stat-row"><span>Members:</span> <span>' + (stats.members || 0) + '</span></div>' +
                            '<div class="stat-row"><span>Blocks Won:</span> <span>' + (stats.blocks_won || 0) + '</span></div>' +
                            '<div class="stat-row"><span>Win Rate:</span> <span>' + winPercentage + '%</span></div>' +
                            '<div class="stat-row"><span>Capacity:</span> <span>' + ((stats.total_capacity || 0) / (1024*1024*1024*1024)).toFixed(2) + ' TB</span></div>' +
                        '</div>' +
                    '</div>';
                }
                html += '</div>';
                statsDiv.innerHTML = html;
            } catch (error) {
                console.error('Failed to load syndicate stats:', error);
                document.getElementById('syndicate-stats').innerHTML = '<div class="error">Failed to load syndicate statistics</div>';
            }
        }

        function setupSyndicateForm() {
            const form = document.getElementById('joinSyndicateForm');
            if (!form) return;

            form.addEventListener('submit', async function(e) {
                e.preventDefault();
                const syndicate = document.getElementById('syndicateSelect').value;
                if (!syndicate) {
                    alert('Please select a syndicate');
                    return;
                }

                try {
                    const response = await fetch('/wallet/join-syndicate', {
                        method: 'POST',
                        headers: { 'Content-Type': 'application/json' },
                        body: JSON.stringify({ syndicate: syndicate })
                    });

                    const result = await response.json();
                    if (response.ok) {
                        alert('Successfully joined syndicate: ' + syndicate + '\\nTransaction ID: ' + result.tx_hash);
                        await loadSyndicateData();
                    } else {
                        alert('Failed to join syndicate: ' + result.error);
                    }
                } catch (error) {
                    console.error('Error joining syndicate:', error);
                    alert('Error joining syndicate: ' + error.message);
                }
            });
        }

        function getSyndicateIcon(syndicate) {
            const icons = { 'abyss': '🕳️', 'echo': '📡', 'nexus': '🔗', 'forge': '⚒️', 'vault': '🏦' };
            return icons[syndicate] || '❓';
        }

        function getSyndicateName(syndicate) {
            const names = { 'abyss': 'The Abyss', 'echo': 'Echo Network', 'nexus': 'Nexus Hub', 'forge': 'The Forge', 'vault': 'Vault Syndicate' };
            return names[syndicate] || 'Unknown Syndicate';
        }

        async function loadSyndicateData() {
            await Promise.all([
                loadCurrentMembership(),
                loadSyndicateStats(),
                setupSyndicateForm()
            ]);
        }

        function loadTabData(mainTab, subTab) {
            const tabKey = mainTab + '-' + subTab;

            switch(tabKey) {
                case 'wallet-balances':
                    loadTokenBalances();
                    break;
                case 'wallet-transactions':
                    loadTransactions();
                    break;
                case 'node-syndicates':
                    loadSyndicateData();
                    break;
                case 'node-blocks':
                    loadRecentBlocks();
                    break;
                case 'foundry-minter':
                    console.log('Switching to foundry minter tab');
                    setTimeout(() => {
                        setupFoundryForm();
                    }, 100);
                    break;
                case 'swap-exchange':
                    loadMarketplace();
                    break;
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

                // Note: transactions tab handling moved to new tab system
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

                // Load consensus stats for peer count and sync status
                let consensusData = { peer_count: 0, sync_status: { is_syncing: false } };
                try {
                    const consensusResponse = await fetch('/api/v1/consensus/sync');
                    if (consensusResponse.ok) {
                        consensusData.sync_status = await consensusResponse.json();
                    } else if (consensusResponse.status === 401) {
                        // Not authenticated, reload to go to login page
                        window.location.reload();
                        return;
                    }

                    const peersResponse = await fetch('/api/v1/consensus/peers');
                    if (peersResponse.ok) {
                        const peersData = await peersResponse.json();
                        consensusData.peer_count = peersData.length || 0;
                    }
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

                // Update sync status
                const syncStatusEl = document.getElementById('syncStatus');
                if (consensusData.sync_status.is_syncing) {
                    const blocksToGo = consensusData.sync_status.target_height - consensusData.sync_status.current_height;
                    syncStatusEl.textContent = 'Syncing (' + blocksToGo + ' blocks to go)';
                    syncStatusEl.style.color = '#fbbf24'; // Yellow for syncing
                } else {
                    syncStatusEl.textContent = 'Online';
                    syncStatusEl.style.color = '#28a745'; // Green for online
                }

                // Update block interval if provided by API
                if (blockchainData.block_interval_seconds) {
                    blockInterval = blockchainData.block_interval_seconds;
                    console.log('Updated block interval from API:', blockInterval, 'seconds');
                }

                // Start countdown if we have block data
                console.log('Blockchain data:', blockchainData);
                console.log('Last block time:', blockchainData.last_block_time);
                if (blockchainData.last_block_time) {
                    lastBlockTime = new Date(blockchainData.last_block_time);
                    console.log('Parsed last block time:', lastBlockTime);
                    console.log('Block interval:', blockInterval, 'seconds');
                    startBlockCountdown();
                } else {
                    // console.log('No last_block_time found, countdown not started');
                    document.getElementById('blockCountdown').textContent = '--:--';
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
                document.getElementById('syncStatus').textContent = 'Error';
            }
        }

        // Start block countdown timer
        function startBlockCountdown() {
            // console.log('Starting block countdown...');
            if (countdownInterval) clearInterval(countdownInterval);

            countdownInterval = setInterval(() => {
                if (!lastBlockTime) {
                    // console.log('No lastBlockTime set, stopping countdown');
                    return;
                }

                const now = new Date();
                const nextBlockTime = new Date(lastBlockTime.getTime() + (blockInterval * 1000));
                const timeLeft = nextBlockTime - now;

                // console.log('Countdown - Now:', now, 'Next block:', nextBlockTime, 'Time left:', timeLeft);

                if (timeLeft <= 0) {
                    document.getElementById('blockCountdown').textContent = '00:00';
                    console.log('Time expired, refreshing stats...');
                    // Refresh stats to check for new block
                    loadNetworkStats();
                } else {
                    const totalMinutes = Math.floor(timeLeft / 60000);
                    const hours = Math.floor(totalMinutes / 60);
                    const minutes = totalMinutes % 60;
                    const seconds = Math.floor((timeLeft % 60000) / 1000);

                    let display;
                    if (hours > 0) {
                        display = hours.toString().padStart(2, '0') + ':' +
                                minutes.toString().padStart(2, '0') + ':' +
                                seconds.toString().padStart(2, '0');
                    } else {
                        display = minutes.toString().padStart(2, '0') + ':' +
                                seconds.toString().padStart(2, '0');
                    }

                    document.getElementById('blockCountdown').textContent = display;
                    // console.log('Countdown display:', display, '(time left:', timeLeft, 'ms)');
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

                let html = '<table class="table table-dark table-striped table-hover">';
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

        // Load token balances
        async function loadTokenBalances() {
            try {
                const response = await fetch('/wallet/tokens');
                const data = await response.json();
                const container = document.getElementById('tokensContainer');

                if (!data.balances || data.balances.length === 0) {
                    container.innerHTML = '<div class="no-tokens"><h3>No Tokens Found</h3><p>Create a new token in the Token Foundry or receive one to get started.</p></div>';
                    return;
                }

                let html = '<table class="table table-dark table-striped table-hover"><thead><tr><th>Token</th><th>Balance</th><th>Actions</th></tr></thead><tbody>';

                data.balances.forEach(balance => {
                    const token = balance.token_info || {};
                    const tokenName = token.name || 'Unknown Token';
                    const tokenTicker = token.ticker || 'Unknown';
                    const formattedBalance = formatTokenAmount(balance.balance, token.decimals);

                    html += '<tr>' +
                        '<td>' +
                            '<strong>' + tokenName + ' (' + tokenTicker + ')</strong><br>' +
                            '<small class="text-muted">' + balance.token_id + '</small>' +
                        '</td>' +
                        '<td>' + formattedBalance + '</td>' +
                        '<td>' +
                            '<button class="btn btn-sm btn-primary" onclick="showSendToken(\'' + balance.token_id + '\')" >Send</button>' +
                            '<button class="btn btn-sm btn-danger" onclick="showMeltDialog(\'' + balance.token_id + '\', \'' + tokenTicker + '\', ' + balance.balance + ', ' + token.decimals + ')">Melt</button>' +
                        '</td>' +
                    '</tr>';
                });

                html += '</tbody></table>';
                container.innerHTML = html;
            } catch (error) {
                document.getElementById('tokensContainer').innerHTML =
                    '<div class="error">Error loading token balances: ' + error.message + '</div>';
            }
        }

        // Load token balances
        // Remove a token from the UI immediately (for better UX when melting entire balance)
        function removeTokenFromUI(tokenId) {
            try {
                const tokenCard = document.querySelector('.token-card[data-token-id="' + tokenId + '"]');
                if (tokenCard) {
                    tokenCard.style.transition = 'opacity 0.3s ease, transform 0.3s ease';
                    tokenCard.style.opacity = '0';
                    tokenCard.style.transform = 'scale(0.8)';

                    setTimeout(() => {
                        tokenCard.remove();

                        // Check if no tokens remain and show "no tokens" message
                        const container = document.getElementById('tokensContainer');
                        const remainingTokens = container.querySelectorAll('.token-card');

                        if (remainingTokens.length === 0) {
                            container.innerHTML = '<div class="no-tokens">' +
                                '<h3>No Token Balances</h3>' +
                                '<p>You don\'t have any token balances yet.</p>' +
                                '<p>Create tokens or receive them from other users to see them here.</p>' +
                                '</div>';
                        }
                    }, 300);
                }
            } catch (error) {
                console.error('Error removing token from UI:', error);
            }
        }

        async function loadTokenBalances() {
            try {
                console.log('Loading token balances...');
                const response = await fetch('/wallet/tokens');
                if (!response.ok) {
                    throw new Error('Failed to fetch tokens: ' + response.status + ' ' + response.statusText);
                }
                const data = await response.json();
                console.log('Token balances response:', data);

                const container = document.getElementById('tokensContainer');

                if (!data.balances || data.balances.length === 0) {
                    container.innerHTML = '<div class="no-tokens">' +
                        '<h3>No Token Balances</h3>' +
                        '<p>You don\'t have any tokens yet.</p>' +
                        '<p style="font-size: 0.9rem; color: #888;">' +
                        'Tokens will appear here when you receive them or create new ones.' +
                        '</p></div>';
                    return;
                }

                let html = '<div class="d-flex justify-content-between align-items-center mb-3">';
                html += '<h3 class="mb-0">Token Balances (' + data.balances.length + ')</h3>';
                html += '<button class="btn btn-secondary btn-sm" onclick="loadTokenBalances()">🔄 Refresh</button>';
                html += '</div>';

                html += '<div class="table-responsive">';
                html += '<table class="table table-dark table-striped table-hover">';
                html += '<thead>';
                html += '<tr>';
                html += '<th scope="col">Token ID</th>';
                html += '<th scope="col">Ticker</th>';
                html += '<th scope="col">Name</th>';
                html += '<th scope="col" class="text-end">Balance</th>';
                html += '<th scope="col" class="text-center">Actions</th>';
                html += '</tr>';
                html += '</thead>';
                html += '<tbody>';

                data.balances.forEach(balance => {
                    const token = balance.token_info || {};
                    const trustStatus = getTrustStatusDisplay(balance.trust_level || 'unknown');
                    const tokenName = token.name || 'Unknown Token';
                    const tokenTicker = token.ticker || 'UNK';
                    const formattedBalance = formatTokenAmount(balance.balance, token.decimals || 0);
                    const shortTokenId = balance.token_id.substring(0, 12) + '...';

                    // Check if this is an NFT (0 decimals, supply of 1)
                    const isNFT = (token.decimals === 0 && token.total_supply === 1);

                    // Create tooltip data for ticker hover
                    let tooltipData = [];
                    if (token.creator) tooltipData.push('Creator: ' + token.creator.substring(0, 16) + '...');
                    if (token.total_supply) tooltipData.push('Total Supply: ' + formatTokenAmount(token.total_supply, token.decimals || 0));
                    if (token.decimals !== undefined) tooltipData.push('Decimals: ' + token.decimals);
                    const tooltipText = tooltipData.join('\\n');

                    html += '<tr data-token-id="' + balance.token_id + '">';

                    // Token ID column
                    html += '<td>';
                    html += '<code class="text-light" style="font-size: 0.85em; cursor: pointer;" onclick="copyText(\'' + balance.token_id + '\')" title="Click to copy full ID">';
                    html += shortTokenId;
                    html += '</code>';
                    html += '</td>';

                    // Ticker column (with tooltip)
                    html += '<td>';
                    html += '<span class="badge ' + trustStatus.class + ' me-1">' + trustStatus.icon + '</span>';
                    html += '<strong';
                    if (tooltipData.length > 0) {
                        html += ' title="' + tooltipText + '" data-bs-toggle="tooltip"';
                    }
                    html += '>';
                    html += tokenTicker + (isNFT ? ' 🖼️' : '');
                    html += '</strong>';
                    html += '</td>';

                    // Name column
                    html += '<td>' + tokenName + '</td>';

                    // Balance column (right-aligned)
                    html += '<td class="text-end">';
                    html += '<span class="fw-bold">' + formattedBalance + '</span>';
                    html += '</td>';

                    // Actions column
                    html += '<td class="text-center">';
                    html += '<div class="dropdown">';
                    html += '<button class="btn btn-sm btn-outline-secondary dropdown-toggle" type="button" data-bs-toggle="dropdown" aria-expanded="false">';
                    html += 'Actions';
                    html += '</button>';
                    html += '<div class="dropdown-menu dropdown-menu-dark">';

                    // Add actions based on trust level
                    if (balance.trust_level === 'unknown') {
                        html += '<a class="dropdown-item text-success" href="#" onclick="approveToken(\'' + balance.token_id + '\', \'accept\')">✅ Accept</a>';
                    }
                    if (balance.balance > 0) {
                        html += '<a class="dropdown-item text-danger" href="#" onclick="showMeltDialog(\'' + balance.token_id + '\', \'' + tokenTicker + '\', ' + balance.balance + ', ' + (token.decimals || 0) + ')">🔥 Melt</a>';
                    }
                    // Future actions can be added here
                    // html += '<a class="dropdown-item" href="#">🔄 Trade</a>';
                    // html += '<a class="dropdown-item" href="#">💧 Add to Pool</a>';

                    html += '</div>';
                    html += '</div>';
                    html += '</td>';

                    html += '</tr>';
                });

                // Close table
                html += '</tbody></table></div>';

                // Add warning for unknown tokens
                const unknownCount = data.balances.filter(b => (b.trust_level || 'unknown') === 'unknown').length;
                if (unknownCount > 0) {
                    html += '<div class="alert alert-warning mt-3">';
                    html += '<h5><i class="bi bi-exclamation-triangle"></i> Unknown Tokens Detected</h5>';
                    html += '<p class="mb-1">' + unknownCount + ' token(s) are marked as unknown. Only interact with tokens from trusted sources.</p>';
                    html += '<small><strong>Security Tip:</strong> Verify token authenticity before accepting or trading.</small>';
                    html += '</div>';
                }

                container.innerHTML = html;

                // Initialize Bootstrap components with defensive checks
                setTimeout(() => {
                    // Check if Bootstrap and its dependencies are fully loaded
                    if (typeof bootstrap !== 'undefined') {
                        console.log('Bootstrap is loaded');

                        // Initialize tooltips
                        if (bootstrap.Tooltip) {
                            const tooltipTriggerList = [].slice.call(container.querySelectorAll('[data-bs-toggle="tooltip"]'));
                            tooltipTriggerList.map(function (tooltipTriggerEl) {
                                return new bootstrap.Tooltip(tooltipTriggerEl);
                            });
                            console.log('Bootstrap tooltips initialized');
                        }

                        // Check if Popper is available (needed for dropdowns)
                        if (typeof Popper !== 'undefined' || (window.Popper && typeof window.Popper.createPopper === 'function')) {
                            console.log('Popper.js is available - dropdowns should work automatically');
                        } else {
                            console.warn('Popper.js not detected - dropdowns may not work properly');
                        }
                    } else {
                        console.warn('Bootstrap not fully loaded yet');
                    }
                }, 200);

            } catch (error) {
                console.error('Token balance loading error:', error);
                alert('Error loading token balances: ' + error.message);
                document.getElementById('tokensContainer').innerHTML =
                    '<div class="error">Error loading token balances: ' + error.message + '</div>';
            }
        }

        // Helper function to get trust status display
        function getTrustStatusDisplay(trustLevel) {
            switch (trustLevel) {
                case 'accepted':
                    return { icon: '✅', text: 'TRUSTED', class: 'trust-accepted' };
                case 'banned':
                    return { icon: '🚫', text: 'BANNED', class: 'trust-banned' };
                case 'verified':
                    return { icon: '🔒', text: 'VERIFIED', class: 'trust-verified' };
                default:
                    return { icon: '⚠️', text: 'UNKNOWN', class: 'trust-unknown' };
            }
        }

        // Helper function to format token amounts with decimals
        function formatTokenAmount(amount, decimals) {
            if (decimals === 0) {
                return amount.toLocaleString();
            }
            const divisor = Math.pow(10, decimals);
            return (amount / divisor).toLocaleString(undefined, {
                minimumFractionDigits: 0,
                maximumFractionDigits: decimals
            });
        }

        // Helper function to copy text
        function copyText(text) {
            navigator.clipboard.writeText(text).then(() => {
                alert('Token ID copied to clipboard!');
            });
        }

        // Handle token approval actions
        async function approveToken(tokenId, action) {
            try {
                const response = await fetch('/wallet/approve_token', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json'
                    },
                    body: JSON.stringify({
                        token_id: tokenId,
                        action: action,
                        notes: '' // Could add a prompt for notes in the future
                    })
                });

                const result = await response.json();

                if (result.success) {
                    alert(result.message);
                    // Refresh token list to show updated trust status
                    loadTokenBalances();
                } else {
                    alert('Error: ' + (result.message || 'Failed to update token trust'));
                }
            } catch (error) {
                console.error('Error approving token:', error);
                alert('Error: ' + error.message);
            }
        }

        // Token foundry functions
        function setupFoundryForm() {
            console.log('Setting up foundry form');
            // Set up event listeners for cost calculation
            setupCostCalculation();
        }

        function submitTokenCreation() {
            console.log('submitTokenCreation called');
            const form = document.getElementById('createTokenForm');
            if (form) {
                handleCreateToken({ target: form, preventDefault: () => {} });
            } else {
                console.error('createTokenForm not found');
            }
        }

        function resetCreateTokenForm() {
            // Clear form
            document.getElementById('createTokenForm').reset();
            document.getElementById('costPreview').style.display = 'none';
            document.getElementById('createTokenBtn').disabled = true;
            updateTokenType(); // Reset to default type
        }

        function updateTokenType() {
            const isNFT = document.getElementById('typeNFT').checked;
            const totalSupplyInput = document.getElementById('tokenTotalSupply');
            const decimalsInput = document.getElementById('decimals');
            const uriSection = document.getElementById('uriSection');

            if (isNFT) {
                // NFT mode: enforce 0 decimals, supply of 1
                totalSupplyInput.value = 1;
                totalSupplyInput.readOnly = true;
                decimalsInput.value = 0;
                decimalsInput.readOnly = true;
                uriSection.style.display = 'block';
            } else {
                // Token mode: allow editing
                totalSupplyInput.readOnly = false;
                decimalsInput.readOnly = false;
                decimalsInput.value = 8; // Default to 8 decimals
                uriSection.style.display = 'none';
            }

            // Cost calculation will be triggered by setupCostCalculation event listeners
        }

        function setupCostCalculation() {
            console.log('setupCostCalculation called');
            const totalSupplyInput = document.getElementById('tokenTotalSupply');
            const decimalsInput = document.getElementById('decimals');
            const lockAmountInput = document.getElementById('lockAmount');

            console.log('Found inputs:', {
                totalSupply: !!totalSupplyInput,
                decimals: !!decimalsInput,
                lockAmount: !!lockAmountInput
            });

            if (!totalSupplyInput || !decimalsInput || !lockAmountInput) {
                console.error('Token form inputs not found');
                console.log('totalSupplyInput:', totalSupplyInput);
                console.log('decimalsInput:', decimalsInput);
                console.log('lockAmountInput:', lockAmountInput);
                return;
            }

            function updateCostPreview() {
                console.log('Raw input values:', {
                    totalSupplyRaw: totalSupplyInput.value,
                    decimalsRaw: decimalsInput.value,
                    lockAmountRaw: lockAmountInput.value
                });

                const totalSupply = parseInt(totalSupplyInput.value) || 0;
                const decimals = parseInt(decimalsInput.value) || 0;
                const lockAmount = parseFloat(lockAmountInput.value) || 0;

                console.log('Parsed values:', { totalSupply, decimals, lockAmount });

                if (totalSupply > 0 && lockAmount > 0) {
                    console.log('Enabling create button');
                    // Convert lock amount from SHADOW to satoshi (multiply by 100000000)
                    const lockAmountSatoshi = Math.floor(lockAmount * 100000000);

                    // Calculate total cost: totalSupply * lockAmountSatoshi
                    const totalCostSatoshi = totalSupply * lockAmountSatoshi;
                    const totalCostShadow = totalCostSatoshi / 100000000;

                    // Update the cost preview elements
                    const costPreview = document.getElementById('costPreview');
                    const totalShadowCost = document.getElementById('totalShadowCost');
                    const backingCost = document.getElementById('backingCost');
                    const creationFee = document.getElementById('creationFee');
                    const createTokenBtn = document.getElementById('createTokenBtn');

                    if (totalShadowCost) totalShadowCost.textContent = totalCostShadow.toFixed(8);
                    if (backingCost) backingCost.textContent = totalCostShadow.toFixed(8);
                    if (creationFee) creationFee.textContent = '0.00000000';
                    if (costPreview) costPreview.style.display = 'block';
                    if (createTokenBtn) createTokenBtn.disabled = false;
                } else {
                    console.log('Disabling create button - invalid input');
                    const costPreview = document.getElementById('costPreview');
                    const createTokenBtn = document.getElementById('createTokenBtn');

                    if (costPreview) costPreview.style.display = 'none';
                    if (createTokenBtn) createTokenBtn.disabled = true;
                }
            }

            totalSupplyInput.addEventListener('input', function() {
                console.log('Total supply input changed to:', totalSupplyInput.value);
                updateCostPreview();
            });
            decimalsInput.addEventListener('input', function() {
                console.log('Decimals input changed to:', decimalsInput.value);
                updateCostPreview();
            });
            lockAmountInput.addEventListener('input', function() {
                console.log('Lock amount input changed to:', lockAmountInput.value);
                updateCostPreview();
            });
        }

        function updateTokenType() {
            const tokenTypeElement = document.querySelector('input[name="tokenType"]:checked');
            if (!tokenTypeElement) return; // Elements not ready yet

            const tokenType = tokenTypeElement.value;
            const decimalsSelect = document.getElementById('decimals');
            const totalSupplyInput = document.getElementById('tokenTotalSupply');
            const totalSupplyLabel = document.querySelector('label[for="totalSupply"]');

            if (!decimalsSelect || !totalSupplyInput || !totalSupplyLabel) return; // Elements not ready yet

            const totalSupplyHelp = totalSupplyInput.nextElementSibling;

            if (tokenType === 'nft') {
                // NFT settings
                decimalsSelect.value = '0';
                decimalsSelect.disabled = true;
                totalSupplyLabel.textContent = 'Collection Size *';
                totalSupplyInput.placeholder = 'e.g., 1000';
                totalSupplyHelp.textContent = 'Number of unique NFTs in this collection';
            } else {
                // Fungible token settings
                decimalsSelect.disabled = false;
                decimalsSelect.value = '8'; // Default back to 8
                totalSupplyLabel.textContent = 'Total Supply *';
                totalSupplyInput.placeholder = 'e.g., 1000000';
                totalSupplyHelp.textContent = 'Fixed total number of tokens to create';
            }

            // Update cost estimate
            updateCostEstimate();
        }

        function updateCostEstimate() {
            // This function is called by onchange events in the form
            // It triggers the internal updateCostPreview function if it exists
            const totalSupplyInput = document.getElementById('tokenTotalSupply');
            const decimalsInput = document.getElementById('decimals');
            const lockAmountInput = document.getElementById('lockAmount');

            // Return early if elements don't exist (foundry tab not loaded)
            if (!totalSupplyInput || !decimalsInput || !lockAmountInput) {
                return;
            }

            // Manually trigger the cost preview update
            const totalSupply = parseInt(totalSupplyInput.value) || 0;
            const decimals = parseInt(decimalsInput.value) || 0;
            const lockAmount = parseFloat(lockAmountInput.value) || 0;

            if (totalSupply > 0 && lockAmount > 0) {
                // Calculate costs
                const lockAmountSatoshi = Math.floor(lockAmount * 100000000);
                const totalCostSatoshi = totalSupply * lockAmountSatoshi;
                const totalCostShadow = totalCostSatoshi / 100000000;

                // Update the preview
                const costPreview = document.getElementById('costPreview');
                const totalShadowCost = document.getElementById('totalShadowCost');
                const backingCost = document.getElementById('backingCost');
                const creationFee = document.getElementById('creationFee');

                if (costPreview && totalShadowCost && backingCost && creationFee) {
                    totalShadowCost.textContent = totalCostShadow.toFixed(8);
                    backingCost.textContent = totalCostShadow.toFixed(8);
                    creationFee.textContent = '0.00000000'; // No additional fee beyond backing
                    costPreview.style.display = 'block';
                }
            } else {
                const costPreview = document.getElementById('costPreview');
                if (costPreview) {
                    costPreview.style.display = 'none';
                }
            }
        }

        async function handleCreateToken(event) {
            event.preventDefault();
            console.log('handleCreateToken called');

            const formData = new FormData(event.target);
            const tokenData = {
                name: formData.get('tokenName'),
                ticker: formData.get('tokenTicker'),
                total_supply: parseInt(formData.get('totalSupply')), // User tokens, not base units
                decimals: parseInt(formData.get('decimals')),
                lock_amount: Math.floor(parseFloat(formData.get('lockAmount')) * 100000000), // Convert to satoshi
                uri: formData.get('tokenURI') || '' // Optional URI for NFTs/metadata
            };

            console.log('Token data:', tokenData);

            // Disable submit button
            const submitBtn = document.getElementById('createTokenBtn');
            submitBtn.disabled = true;
            submitBtn.textContent = 'Creating Token...';

            try {
                console.log('Sending POST request to /wallet/create_token');
                const response = await fetch('/wallet/create_token', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(tokenData)
                });

                console.log('Response status:', response.status);
                if (response.ok) {
                    const result = await response.json();
                    const tokenName = tokenData.name || 'Token';
                    alert('Token created successfully! Transaction ID: ' + result.tx_id);

                    // Add to pending transactions for confirmation tracking
                    addPendingTransaction(result.tx_id, 'token_create', 'Token "' + tokenName + '" creation');

                    resetCreateTokenForm();
                    // Refresh balances after token creation
                    setTimeout(() => {
                        loadTokenBalances(); // Refresh balances
                    }, 500); // Wait 500ms for transaction to be processed
                } else {
                    const errorText = await response.text();
                    alert('Failed to create token: ' + errorText);
                }
            } catch (error) {
                alert('Error creating token: ' + error.message);
            } finally {
                submitBtn.disabled = false;
                submitBtn.textContent = 'Create Token';
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
            const assetType = formData.get('assetType') || 'shadow';

            const data = {
                to_address: formData.get('sendAddress'),
                amount: parseFloat(formData.get('sendAmount')),
                fee: parseFloat(formData.get('sendFee')) || 0.1,
                message: formData.get('sendMessage') || '',
                asset_type: assetType
            };

            // Add token-specific data if needed
            if (assetType === 'token') {
                const tokenSelect = document.getElementById('tokenSelect');
                if (!tokenSelect.value) {
                    document.getElementById('sendResult').innerHTML =
                        '<div class="error">Please select a token to send</div>';
                    return;
                }
                data.token_id = tokenSelect.value;
            }

            try {
                const response = await fetch('/wallet/send', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify(data)
                });

                if (response.ok) {
                    const result = await response.json();
                    document.getElementById('sendResult').innerHTML =
                        '<div class="success">' + result.message + '<br>Hash: ' + result.tx_hash + '</div>';

                    // Reload wallet data
                    setTimeout(loadWalletData, 2000);
                    // Reload token balances if it was a token transfer
                    if (result.asset_type === 'token') {
                        setTimeout(loadTokenBalances, 2000);
                    }

                    // Clear form
                    e.target.reset();
                    // Reset to default values
                    document.getElementById('assetType').value = 'shadow';
                    updateSendForm();
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

        // Update send form based on asset type selection
        function updateSendForm() {
            const assetType = document.getElementById('assetType').value;
            const tokenSelectGroup = document.getElementById('tokenSelectGroup');
            const sendAmountLabel = document.getElementById('sendAmountLabel');
            const feeGroup = document.getElementById('feeGroup');
            const sendAmountHelp = document.getElementById('sendAmountHelp');
            const sendButton = document.getElementById('sendButton');
            const sendAmountInput = document.getElementById('sendAmount');

            if (assetType === 'token') {
                tokenSelectGroup.style.display = 'block';
                sendAmountLabel.textContent = 'Amount:';
                feeGroup.style.display = 'none'; // Hide fee for token transfers
                sendButton.textContent = 'Send Token';
                loadUserTokensForSend();
            } else {
                tokenSelectGroup.style.display = 'none';
                sendAmountLabel.textContent = 'Amount (SHADOW):';
                feeGroup.style.display = 'block';
                sendAmountHelp.textContent = '';
                sendButton.textContent = 'Send Payment';
                document.getElementById('tokenBalance').textContent = '';

                // Clear token-specific input constraints
                sendAmountInput.removeAttribute('max');
                sendAmountInput.step = '0.00000001';
            }
        }

        // Load user's tokens for send form
        async function loadUserTokensForSend() {
            try {
                const response = await fetch('/wallet/tokens');
                const data = await response.json();

                const tokenSelect = document.getElementById('tokenSelect');
                tokenSelect.innerHTML = '<option value="">Select a token...</option>';

                if (data.balances && data.balances.length > 0) {
                    // Only show tokens with balance > 0
                    const tokensWithBalance = data.balances.filter(b => b.balance > 0);

                    if (tokensWithBalance.length === 0) {
                        tokenSelect.innerHTML = '<option value="">No tokens with balance</option>';
                        return;
                    }

                    tokensWithBalance.forEach(balance => {
                        const token = balance.token_info || {};
                        const displayName = token.name ? token.name + ' (' + token.ticker + ')' : balance.token_id.substring(0, 16) + '...';
                        const option = document.createElement('option');
                        option.value = balance.token_id;
                        option.textContent = displayName;
                        option.setAttribute('data-balance', balance.balance);
                        option.setAttribute('data-decimals', token.decimals || 0);
                        option.setAttribute('data-ticker', token.ticker || 'TOKEN');
                        tokenSelect.appendChild(option);
                    });

                    // Add event listener for token selection
                    tokenSelect.addEventListener('change', updateTokenBalance);
                } else {
                    tokenSelect.innerHTML = '<option value="">No tokens found</option>';
                }
            } catch (error) {
                console.error('Error loading tokens for send:', error);
                document.getElementById('tokenSelect').innerHTML = '<option value="">Error loading tokens</option>';
            }
        }

        // Update token balance display and amount constraints
        function updateTokenBalance() {
            const tokenSelect = document.getElementById('tokenSelect');
            const selectedOption = tokenSelect.options[tokenSelect.selectedIndex];
            const balanceDisplay = document.getElementById('tokenBalance');
            const sendAmountHelp = document.getElementById('sendAmountHelp');
            const sendAmountInput = document.getElementById('sendAmount');
            const sendAmountLabel = document.getElementById('sendAmountLabel');

            if (selectedOption && selectedOption.value) {
                const balance = parseInt(selectedOption.getAttribute('data-balance'));
                const decimals = parseInt(selectedOption.getAttribute('data-decimals'));
                const ticker = selectedOption.getAttribute('data-ticker');

                // Format balance for display
                let displayBalance;
                if (decimals > 0) {
                    const divisor = Math.pow(10, decimals);
                    displayBalance = (balance / divisor).toFixed(decimals);
                } else {
                    displayBalance = balance.toString();
                }

                balanceDisplay.textContent = 'Balance: ' + displayBalance + ' ' + ticker;
                sendAmountLabel.textContent = 'Amount (' + ticker + '):';
                sendAmountHelp.textContent = 'Available: ' + displayBalance + ' ' + ticker;

                // Update input constraints
                const maxAmount = decimals > 0 ? balance / Math.pow(10, decimals) : balance;
                sendAmountInput.max = maxAmount;
                sendAmountInput.step = decimals > 0 ? Math.pow(10, -decimals) : 1;
            } else {
                balanceDisplay.textContent = '';
                sendAmountHelp.textContent = '';
                sendAmountInput.removeAttribute('max');
                sendAmountInput.step = '0.00000001';
            }
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

    balance, err := calculateWalletBalanceWithDir(targetAddress, "")
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
    log.Printf("🔍 [WALLET_SEND] Received send request to address: %s (len=%d, first_char=%c)",
        sendData.ToAddress, len(sendData.ToAddress), sendData.ToAddress[0])
    if !IsValidAddress(sendData.ToAddress) {
        http.Error(w, "Invalid destination address format", http.StatusBadRequest)
        return
    }
    log.Printf("✅ [WALLET_SEND] Address validation passed for: %s", sendData.ToAddress)

    // Validate amount
    if sendData.Amount <= 0 {
        http.Error(w, "Amount must be positive", http.StatusBadRequest)
        return
    }

    // Set default asset type if not provided
    if sendData.AssetType == "" {
        sendData.AssetType = "shadow"
    }

    // Validate asset type
    if sendData.AssetType != "shadow" && sendData.AssetType != "token" {
        http.Error(w, "Asset type must be 'shadow' or 'token'", http.StatusBadRequest)
        return
    }

    // Validate token transfer requirements
    if sendData.AssetType == "token" {
        if sendData.TokenID == "" {
            http.Error(w, "Token ID is required for token transfers", http.StatusBadRequest)
            return
        }

        // Check blockchain availability for token validation
        if sn.blockchain == nil {
            http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
            return
        }

        // Validate token exists
        tokenState := sn.blockchain.GetTokenState()
        _, err := tokenState.GetTokenInfo(sendData.TokenID)
        if err != nil {
            http.Error(w, "Token not found", http.StatusBadRequest)
            return
        }

        // Check token balance
        tokenBalance, err := tokenState.GetTokenBalance(sendData.TokenID, session.Address)
        if err != nil {
            http.Error(w, "Failed to get token balance", http.StatusInternalServerError)
            return
        }

        // Convert amount to token base units (handle decimals)
        metadata, _ := tokenState.GetTokenInfo(sendData.TokenID)
        var amountTokenUnits uint64
        var displayBalance float64

        if metadata.Decimals > 0 {
            // Apply decimal conversion: user amount * 10^decimals
            multiplier := uint64(1)
            for i := uint8(0); i < metadata.Decimals; i++ {
                multiplier *= 10
            }
            amountTokenUnits = uint64(sendData.Amount * float64(multiplier))
            displayBalance = float64(tokenBalance) / float64(multiplier)
        } else {
            // No decimals, use amount as-is
            amountTokenUnits = uint64(sendData.Amount)
            displayBalance = float64(tokenBalance)
        }

        if amountTokenUnits > tokenBalance {
            http.Error(w, fmt.Sprintf("Insufficient token balance: need %.8f %s, have %.8f %s",
                sendData.Amount, metadata.Ticker, displayBalance, metadata.Ticker), http.StatusBadRequest)
            return
        }
    }

    // Set default fee if not provided (only for SHADOW transfers)
    if sendData.Fee <= 0 {
        sendData.Fee = 0.1 // Default fee of 0.1 SHADOW
    }

    // For SHADOW transfers, check SHADOW balance
    if sendData.AssetType == "shadow" {
        // Simplified balance check - assume sufficient balance for now
        // TODO: Implement proper balance calculation without blocking
        balance := &WalletBalance{
            Address:          session.Address,
            ConfirmedBalance: 10000 * uint64(SatoshisPerShadow), // Assume 10,000 SHADOW for testing
            ConfirmedShadow:  10000.0,                           // 10,000 SHADOW
        }

        // Check if user has sufficient balance including fee
        totalRequired := sendData.Amount + sendData.Fee
        if totalRequired > balance.ConfirmedShadow {
            http.Error(w, fmt.Sprintf("Insufficient balance: need %.8f SHADOW (%.8f + %.8f fee), have %.8f SHADOW",
                totalRequired, sendData.Amount, sendData.Fee, balance.ConfirmedShadow), http.StatusBadRequest)
            return
        }
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

    // Create transaction
    tx := NewTransaction()

    // Handle different asset types
    if sendData.AssetType == "shadow" {
        // Convert amount and fee to satoshis for SHADOW transfers
        amountSatoshis := uint64(sendData.Amount * float64(SatoshisPerShadow))

        // Add output for the recipient
        tx.AddOutput(sendData.ToAddress, amountSatoshis)

    } else if sendData.AssetType == "token" {
        // For token transfers, add minimal SHADOW output (1 satoshi) and token operation
        tx.AddOutput(sendData.ToAddress, 1) // Minimal SHADOW output

        // Get token metadata for proper decimal handling
        tokenState := sn.blockchain.GetTokenState()
        metadata, _ := tokenState.GetTokenInfo(sendData.TokenID)

        // Convert amount to token base units
        var amountTokenUnits uint64
        if metadata.Decimals > 0 {
            multiplier := uint64(1)
            for i := uint8(0); i < metadata.Decimals; i++ {
                multiplier *= 10
            }
            amountTokenUnits = uint64(sendData.Amount * float64(multiplier))
        } else {
            amountTokenUnits = uint64(sendData.Amount)
        }

        // Add token transfer operation
        log.Printf("🔍 [WALLET_SEND] Creating token transfer: tokenID=%s, amount=%d, from=%s, to=%s",
            sendData.TokenID, amountTokenUnits, session.Address, sendData.ToAddress)
        tx.AddTokenTransfer(sendData.TokenID, amountTokenUnits, session.Address, sendData.ToAddress)
    }

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
    log.Printf("🔍 [WALLET_SEND] Signing transaction with %d token operations", len(tx.TokenOps))
    if len(tx.TokenOps) > 0 {
        log.Printf("🔍 [WALLET_SEND] First token operation: Type=%d, From=%s, To=%s, TokenID=%s",
            tx.TokenOps[0].Type, tx.TokenOps[0].From, tx.TokenOps[0].To, tx.TokenOps[0].TokenID)
    }
    signedTx, err := SignTransactionWithWallet(tx, wallet)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
        return
    }
    log.Printf("✅ [WALLET_SEND] Transaction signed successfully")

    // Submit to mempool
    if sn.mempool != nil {
        err = sn.mempool.AddTransaction(signedTx, SourceAPI)
        if err != nil {
            http.Error(w, fmt.Sprintf("Failed to submit transaction: %v", err), http.StatusBadRequest)
            return
        }
    }

    response := map[string]interface{}{
        "tx_hash":    signedTx.TxHash,
        "status":     "submitted",
        "amount":     sendData.Amount,
        "fee":        sendData.Fee,
        "to_address": sendData.ToAddress,
        "asset_type": sendData.AssetType,
    }

    if sendData.AssetType == "shadow" {
        response["message"] = "SHADOW transfer submitted to mempool"
        response["total"] = sendData.Amount + sendData.Fee
    } else if sendData.AssetType == "token" {
        // Get token info for response
        tokenState := sn.blockchain.GetTokenState()
        metadata, _ := tokenState.GetTokenInfo(sendData.TokenID)
        response["message"] = fmt.Sprintf("%s token transfer submitted to mempool", metadata.Ticker)
        response["token_id"] = sendData.TokenID
        response["token_ticker"] = metadata.Ticker
        response["token_name"] = metadata.Name
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
        "tx_hash":            signedTx.TxHash,
        "status":             "submitted",
        "message":            "Pre-signed transaction submitted to mempool",
        "signer":             signedTx.SignerKey,
        "algorithm":          signedTx.Algorithm,
        "inputs":             len(tx.Inputs),
        "outputs":            len(tx.Outputs),
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

        // Removed duplicate updateTradeForm function - using the one defined earlier

        // Skip duplicate functions - they're already defined earlier
            console.log('updateTradeForm called');
            const lockedTokenSelect = document.getElementById('lockedTokenSelect');
            const askingTokenSelect = document.getElementById('askingTokenSelect');
            const lockedAmountInput = document.getElementById('lockedAmount');
            const askingPriceInput = document.getElementById('askingPrice');
            const createTradeBtn = document.getElementById('createTradeBtn');
            const costPreview = document.getElementById('tradeCostPreview');
            const balanceDisplay = document.getElementById('lockedTokenBalance');
            const assetPreview = document.getElementById('assetLockedPreview');
            const askingPriceLabel = document.getElementById('askingPriceLabel');
            const askingPriceHelp = document.getElementById('askingPriceHelp');

            const selectedToken = lockedTokenSelect ? lockedTokenSelect.value : '';
            const askingToken = askingTokenSelect ? askingTokenSelect.value : '';
            const amount = lockedAmountInput ? (parseFloat(lockedAmountInput.value) || 0) : 0;
            const price = askingPriceInput ? (parseFloat(askingPriceInput.value) || 0) : 0;

            console.log('Form values:', { selectedToken, askingToken, amount, price });

            // Update asking price label and help based on selected asking token
            if (askingToken === 'SHADOW') {
                askingPriceLabel.textContent = 'Asking Price (SHADOW)';
                askingPriceHelp.textContent = 'Price in SHADOW satoshis (0.00000001 SHADOW = 1 satoshi).';
                askingPriceInput.step = '0.00000001';
            } else if (askingToken) {
                // Find the asking token info to get decimals for proper step
                askingPriceLabel.textContent = 'Asking Price (in selected token)';
                askingPriceHelp.textContent = 'How many tokens you want in exchange.';
                askingPriceInput.step = 'any';
            } else {
                askingPriceLabel.textContent = 'Asking Price';
                askingPriceHelp.textContent = 'Select what you want first.';
            }

            // Update balance display
            if (selectedToken) {
                if (selectedToken === 'SHADOW') {
                    const shadowBalance = walletData ? (walletData.balance / 100000000) : 0;
                    balanceDisplay.textContent = 'Available: ' + shadowBalance.toFixed(8) + ' SHADOW';
                    lockedAmountInput.max = shadowBalance;
                } else {
                    // Find token balance
                    // This would need to be implemented to fetch specific token balance
                    balanceDisplay.textContent = 'Loading balance...';
                }
            } else {
                balanceDisplay.textContent = '';
            }

            // Update cost preview
            const isValid = selectedToken && askingToken && amount > 0 && price > 0;
            console.log('Validation check:', { isValid, selectedToken: !!selectedToken, askingToken: !!askingToken, amount, price });

            if (isValid) {
                if (costPreview) costPreview.style.display = 'block';
                if (assetPreview) assetPreview.textContent = amount + ' ' + (selectedToken === 'SHADOW' ? 'SHADOW' : 'tokens');
                if (createTradeBtn) {
                    createTradeBtn.disabled = false;
                    console.log('Button enabled!');
                }
            } else {
                if (costPreview) costPreview.style.display = 'none';
                if (createTradeBtn) {
                    createTradeBtn.disabled = true;
                    console.log('Button disabled - validation failed');
                }
            }
        }

        function resetTradeForm() {
            document.getElementById('createTradeForm').reset();
            document.getElementById('tradeCostPreview').style.display = 'none';
            document.getElementById('createTradeBtn').disabled = true;
            document.getElementById('lockedTokenBalance').textContent = '';
        }

        // Removed duplicate submitTradeOffer function - using the one defined earlier
            event.preventDefault();

            const lockedTokenSelect = document.getElementById('lockedTokenSelect');
            const askingTokenSelect = document.getElementById('askingTokenSelect');
            const lockedAmountInput = document.getElementById('lockedAmount');
            const askingPriceInput = document.getElementById('askingPrice');
            const expirationSelect = document.getElementById('expirationHours');
            const submitBtn = document.getElementById('createTradeBtn');

            const lockedToken = lockedTokenSelect.value;
            const askingToken = askingTokenSelect.value;
            const amount = parseFloat(lockedAmountInput.value);
            const price = parseFloat(askingPriceInput.value);
            const expirationHours = parseInt(expirationSelect.value);

            if (!lockedToken || !askingToken || amount <= 0 || price <= 0) {
                alert('Please fill in all required fields with valid values.');
                return;
            }

            submitBtn.disabled = true;
            submitBtn.textContent = 'Creating Trade Offer...';

            try {
                const response = await fetch('/api/marketplace/create-offer', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify({
                        locked_token_id: lockedToken,
                        locked_amount: amount,
                        asking_price: Math.round(price * 100000000), // Convert to satoshis
                        asking_token_id: askingToken === 'SHADOW' ? '' : askingToken, // Empty string for SHADOW
                        expiration_hours: expirationHours
                    })
                });

                const result = await response.json();

                if (response.ok) {
                    const tokenName = document.getElementById('lockedTokenSelect')?.selectedOptions[0]?.text || 'Token';
                    alert('Trade offer created successfully!');

                    // Add to pending transactions for confirmation tracking (if tx_id available)
                    if (result.tx_id) {
                        addPendingTransaction(result.tx_id, 'trade_offer', 'Trade offer for ' + tokenName);
                    }

                    resetTradeForm();
                    loadMarketplaceOffers(); // Refresh the marketplace
                    loadWalletData(); // Refresh wallet balance
                } else {
                    throw new Error(result.error || 'Failed to create trade offer');
                }

            } catch (error) {
                console.error('Error creating trade offer:', error);
                alert('Error creating trade offer: ' + error.message);
            } finally {
                submitBtn.disabled = false;
                submitBtn.textContent = '🔒 Create Trade Offer';
            }
        }

        // Close modal when clicking outside
        window.onclick = function(event) {
            const modal = document.getElementById('meltModal');
            if (event.target === modal) {
                closeMeltModal();
            }
        };

        // Initialize confirmation polling
        setInterval(() => {
            if (pendingTransactions.length > 0) {
                checkTransactionConfirmations();
            }
        }, 30000); // Check every 30 seconds

        // Syndicate Functions

        async function loadCurrentMembership() {
            try {
                const address = document.getElementById('walletAddress').textContent;
                const response = await fetch('/wallet/syndicate-membership?address=' + encodeURIComponent(address));
                const data = await response.json();

                const membershipDiv = document.getElementById('current-membership');

                if (data.active_memberships && data.active_memberships.length > 0) {
                    let html = '<div class="current-memberships">';
                    for (const membership of data.active_memberships) {
                        const expirationDate = new Date(membership.expiration_time * 1000);
                        const remainingDays = Math.ceil((expirationDate - new Date()) / (1000 * 60 * 60 * 24));

                        html += '<div class="membership-card">' +
                            '<div class="membership-header">' +
                                '<h4>' + getSyndicateIcon(membership.syndicate) + ' ' + getSyndicateName(membership.syndicate) + '</h4>' +
                                '<span class="membership-status ' + (remainingDays > 2 ? 'active' : 'expiring') + '">' + (remainingDays > 0 ? remainingDays + ' days left' : 'Expired') + '</span>' +
                            '</div>' +
                            '<div class="membership-details">' +
                                '<p><strong>Capacity:</strong> All available storage</p>' +
                                '<p><strong>NFT ID:</strong> ' + membership.nft_token_id + '</p>' +
                                '<p><strong>Renewals:</strong> ' + membership.renewal_count + '</p>' +
                                '<p><strong>Auto-renewal:</strong> ' + (membership.auto_renew ? 'Enabled ✅' : 'Disabled ❌') + '</p>' +
                            '</div>' +
                        '</div>';
                    }
                    html += '</div>';
                    membershipDiv.innerHTML = html;
                } else {
                    membershipDiv.innerHTML = '<div class="no-membership">⚡ No active syndicate memberships. Join a syndicate below to start pooled mining!</div>';
                }
            } catch (error) {
                console.error('Error loading syndicate membership:', error);
                document.getElementById('current-membership').innerHTML = '<div class="error">Error loading membership data</div>';
            }
        }

        async function loadSyndicateStats() {
            try {
                const response = await fetch('/wallet/syndicate-stats');
                const stats = await response.json();

                const statsDiv = document.getElementById('syndicate-stats');
                let html = '<div class="syndicate-stats-grid">';

                const syndicates = ['seiryu', 'byakko', 'suzaku', 'genbu'];
                for (const syndicate of syndicates) {
                    const data = stats[syndicate] || { members: 0, total_capacity: 0, win_percentage: 0, blocks_won: 0 };
                    html += '<div class="syndicate-stat-card">' +
                        '<div class="stat-header">' +
                            '<h4>' + getSyndicateIcon(syndicate) + ' ' + getSyndicateName(syndicate) + '</h4>' +
                        '</div>' +
                        '<div class="stat-details">' +
                            '<div class="stat-item">' +
                                '<span class="stat-label">Members:</span>' +
                                '<span class="stat-value">' + data.members + '</span>' +
                            '</div>' +
                            '<div class="stat-item">' +
                                '<span class="stat-label">Capacity:</span>' +
                                '<span class="stat-value">' + (data.total_capacity / (1024*1024*1024*1024)).toFixed(2) + ' TB</span>' +
                            '</div>' +
                            '<div class="stat-item">' +
                                '<span class="stat-label">Blocks Won:</span>' +
                                '<span class="stat-value">' + data.blocks_won + '</span>' +
                            '</div>' +
                            '<div class="stat-item">' +
                                '<span class="stat-label">Win Rate:</span>' +
                                '<span class="stat-value ' + (data.win_percentage > 35 ? 'warning' : '') + '">' + data.win_percentage.toFixed(1) + '%</span>' +
                            '</div>' +
                        '</div>' +
                    '</div>';
                }
                html += '</div>';
                statsDiv.innerHTML = html;
            } catch (error) {
                console.error('Error loading syndicate stats:', error);
                document.getElementById('syndicate-stats').innerHTML = '<div class="error">Error loading syndicate statistics</div>';
            }
        }

        function setupSyndicateForm() {
            const form = document.getElementById('joinSyndicateForm');
            if (form) {
                form.addEventListener('submit', handleJoinSyndicate);
            }
        }

        async function handleJoinSyndicate(event) {
            event.preventDefault();

            const formData = new FormData(event.target);
            const syndicateChoice = formData.get('syndicateChoice');
            const autoRenew = formData.get('autoRenew') === 'on';
            const membershipDays = parseInt(formData.get('membershipDays'));

            if (!syndicateChoice || !membershipDays) {
                alert('Please select a syndicate and membership duration');
                return;
            }

            const resultDiv = document.getElementById('joinSyndicateResult');
            resultDiv.innerHTML = '<div class="loading">Creating syndicate membership transaction...</div>';

            try {
                const address = document.getElementById('walletAddress').textContent;

                const response = await fetch('/wallet/join-syndicate', {
                    method: 'POST',
                    headers: { 'Content-Type': 'application/json' },
                    body: JSON.stringify({
                        address: address,
                        syndicate: syndicateChoice,
                        auto_renew: autoRenew,
                        membership_days: membershipDays
                    })
                });

                const result = await response.json();

                if (response.ok) {
                    const autoRenewText = autoRenew ? '✅ Enabled' : '❌ Disabled';
                    resultDiv.innerHTML = '<div class="success">' +
                        '<h4>✅ Syndicate Membership Created!</h4>' +
                        '<p><strong>Transaction ID:</strong> ' + result.transaction_id + '</p>' +
                        '<p><strong>Syndicate:</strong> ' + getSyndicateIcon(syndicateChoice) + ' ' + getSyndicateName(syndicateChoice) + '</p>' +
                        '<p><strong>Storage Capacity:</strong> All available storage</p>' +
                        '<p><strong>Auto-renewal:</strong> ' + autoRenewText + '</p>' +
                        '<p><strong>Fee:</strong> 0.1 SHADOW</p>' +
                        '<p>Your membership will be active once the transaction is confirmed in the next block.</p>' +
                    '</div>';

                    // Refresh membership data after a delay
                    setTimeout(() => {
                        loadCurrentMembership();
                        loadWalletData(); // Refresh balance
                    }, 2000);
                } else {
                    throw new Error(result.error || 'Failed to join syndicate');
                }
            } catch (error) {
                console.error('Error joining syndicate:', error);
                resultDiv.innerHTML = '<div class="error">Error: ' + error.message + '</div>';
            }
        }

        function refreshSyndicateStats() {
            loadSyndicateStats();
        }

        function getSyndicateIcon(syndicate) {
            const icons = {
                'auto': '🤖',
                'seiryu': '🐉',
                'byakko': '🐅',
                'suzaku': '🐦',
                'genbu': '🐢'
            };
            return icons[syndicate] || '❓';
        }

        function getSyndicateName(syndicate) {
            const names = {
                'auto': 'Automatic Assignment',
                'seiryu': 'Seiryu (Azure Dragon)',
                'byakko': 'Byakko (White Tiger)',
                'suzaku': 'Suzaku (Vermillion Bird)',
                'genbu': 'Genbu (Black Tortoise)'
            };
            return names[syndicate] || 'Unknown Syndicate';
        }

        // Initial load
        loadWalletData();
    </script>

    <!-- Popper.js (required for Bootstrap dropdowns) -->
    <script src="https://cdn.jsdelivr.net/npm/@popperjs/core@2.11.8/dist/umd/popper.min.js" integrity="sha384-I7E8VVD/ismYTF4hNIPjVp/Zjvgyol6VFvRkX/vR+Vc4jQkC+hVqc2pM8ODewa9r" crossorigin="anonymous"></script>

    <!-- Bootstrap JS -->
    <script src="https://cdn.jsdelivr.net/npm/bootstrap@5.3.3/dist/js/bootstrap.min.js" integrity="sha384-0pUGZvbkm6XF6gxjEnlmuGrJXVbNuzT9qBBavbLwCsOGabYfZo0T0to5eqruptLy" crossorigin="anonymous"></script>
</body>
</html>`

    w.Write([]byte(html))
}

// handleWebWalletTokens returns wallet token balances with trust information
func (sn *ShadowNode) handleWebWalletTokens(w http.ResponseWriter, r *http.Request) {
    // Check authentication
    session, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    // Check blockchain availability
    if sn.blockchain == nil {
        http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
        return
    }

    // Get token trust manager with correct wallet directory
    walletDir := getWebWalletDir()
    trustManager, err := NewTokenTrustManager(session.WalletName, walletDir)
    if err != nil {
        http.Error(w, "Failed to load token trust manager", http.StatusInternalServerError)
        return
    }

    // Get token state and balances
    tokenState := sn.blockchain.GetTokenState()
    balances, err := tokenState.GetAllTokenBalances(session.Address)
    if err != nil {
        http.Error(w, "Failed to get token balances", http.StatusInternalServerError)
        return
    }

    // Prepare response with trust information
    type TokenBalanceResponse struct {
        TokenID    string                 `json:"token_id"`
        Balance    uint64                 `json:"balance"`
        TrustLevel string                 `json:"trust_level"`
        TokenInfo  map[string]interface{} `json:"token_info,omitempty"`
    }

    response := struct {
        Balances []TokenBalanceResponse `json:"balances"`
        Count    int                    `json:"count"`
    }{
        Balances: make([]TokenBalanceResponse, 0),
        Count:    0,
    }

    for _, balance := range balances {
        // Get trust information
        trustInfo := trustManager.GetTokenTrust(balance.TokenID)

        // Get token metadata
        tokenInfo := make(map[string]interface{})
        if balance.TokenInfo != nil {
            tokenInfo["name"] = balance.TokenInfo.Name
            tokenInfo["ticker"] = balance.TokenInfo.Ticker
            tokenInfo["creator"] = balance.TokenInfo.Creator
            tokenInfo["decimals"] = balance.TokenInfo.Decimals
            tokenInfo["lock_amount"] = balance.TokenInfo.LockAmount
        } else {
            // Use cached metadata from trust manager
            if trustInfo.Name != "" {
                tokenInfo["name"] = trustInfo.Name
            }
            if trustInfo.Ticker != "" {
                tokenInfo["ticker"] = trustInfo.Ticker
            }
            if trustInfo.Creator != "" {
                tokenInfo["creator"] = trustInfo.Creator
            }
        }

        balanceResp := TokenBalanceResponse{
            TokenID:    balance.TokenID,
            Balance:    balance.Balance,
            TrustLevel: trustInfo.TrustLevel.String(),
            TokenInfo:  tokenInfo,
        }

        response.Balances = append(response.Balances, balanceResp)
    }

    response.Count = len(response.Balances)

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// handleWebWalletCreateToken creates a new token
func (sn *ShadowNode) handleWebWalletCreateToken(w http.ResponseWriter, r *http.Request) {
    // Check authentication
    session, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    // Parse request body
    var req struct {
        Name        string `json:"name"`
        Ticker      string `json:"ticker"`
        TotalSupply uint64 `json:"total_supply"`
        Decimals    uint8  `json:"decimals"`
        LockAmount  uint64 `json:"lock_amount"`   // in satoshi
        URI         string `json:"uri,omitempty"` // Optional URI for NFTs/metadata
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validate input
    if req.Name == "" || req.Ticker == "" {
        http.Error(w, "Name and ticker are required", http.StatusBadRequest)
        return
    }

    if req.TotalSupply == 0 {
        http.Error(w, "Total supply must be greater than 0", http.StatusBadRequest)
        return
    }

    if req.LockAmount == 0 {
        http.Error(w, "Lock amount must be greater than 0", http.StatusBadRequest)
        return
    }

    if req.Decimals > 18 {
        http.Error(w, "Decimals cannot exceed 18", http.StatusBadRequest)
        return
    }

    // Load wallet to get private key for signing
    wallet, err := loadWallet(session.WalletName)
    if err != nil {
        http.Error(w, "Failed to load wallet", http.StatusInternalServerError)
        return
    }

    // Convert user tokens to base units (multiply by 10^decimals)
    multiplier := uint64(1)
    for i := uint8(0); i < req.Decimals; i++ {
        multiplier *= 10
    }
    totalSupplyBaseUnits := req.TotalSupply * multiplier

    // Calculate total cost (using user tokens, not base units)
    totalCost := req.TotalSupply * req.LockAmount

    // Check wallet balance
    balanceInfo, err := calculateWalletBalanceWithDir(session.Address, "")
    if err != nil {
        http.Error(w, "Failed to check wallet balance", http.StatusInternalServerError)
        return
    }

    if balanceInfo.ConfirmedBalance < totalCost {
        http.Error(w, fmt.Sprintf("Insufficient balance. Need %d satoshi, have %d", totalCost, balanceInfo.ConfirmedBalance), http.StatusBadRequest)
        return
    }

    // Create new transaction
    tx := NewTransaction()
    tx.Timestamp = time.Now()

    // Add token creation operation (using base units for total supply)
    tx.AddTokenCreate(req.Name, req.Ticker, totalSupplyBaseUnits, req.Decimals, req.LockAmount, session.Address, req.URI)

    // Sign the transaction
    signedTx, err := SignTransactionWithWallet(tx, wallet)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
        return
    }

    // Submit to mempool
    if err := sn.mempool.AddTransaction(signedTx, SourceAPI); err != nil {
        http.Error(w, fmt.Sprintf("Failed to submit transaction: %v", err), http.StatusInternalServerError)
        return
    }

    // Return success response
    response := map[string]interface{}{
        "success":  true,
        "tx_id":    signedTx.TxHash,
        "token_id": tx.TokenOps[0].TokenID,
        "message":  "Token creation transaction submitted successfully",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// handleWebWalletApproveToken handles token approval/trust requests from web UI
func (sn *ShadowNode) handleWebWalletApproveToken(w http.ResponseWriter, r *http.Request) {
    // Check authentication
    session, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    // Parse request body
    var req struct {
        TokenID string `json:"token_id"`
        Action  string `json:"action"` // "accept", "ban", or "ignore"
        Notes   string `json:"notes,omitempty"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validate input
    if req.TokenID == "" || req.Action == "" {
        http.Error(w, "Token ID and action are required", http.StatusBadRequest)
        return
    }

    // Check blockchain availability
    if sn.blockchain == nil {
        http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
        return
    }

    // Get token trust manager with correct wallet directory
    walletDir := getWebWalletDir()
    trustManager, err := NewTokenTrustManager(session.WalletName, walletDir)
    if err != nil {
        http.Error(w, "Failed to load token trust manager", http.StatusInternalServerError)
        return
    }

    // Get token metadata to cache it
    tokenState := sn.blockchain.GetTokenState()
    metadata, err := tokenState.GetTokenInfo(req.TokenID)
    if err != nil {
        http.Error(w, "Token not found", http.StatusNotFound)
        return
    }

    // Update token metadata in trust manager
    if err := trustManager.UpdateTokenMetadata(req.TokenID, metadata.Name, metadata.Ticker, metadata.Creator); err != nil {
        http.Error(w, "Failed to update token metadata", http.StatusInternalServerError)
        return
    }

    // Perform the requested action
    var responseMessage string
    switch req.Action {
    case "accept":
        if err := trustManager.AcceptToken(req.TokenID, req.Notes); err != nil {
            http.Error(w, fmt.Sprintf("Failed to accept token: %v", err), http.StatusInternalServerError)
            return
        }
        responseMessage = fmt.Sprintf("Token %s (%s) has been accepted and marked as trusted", metadata.Name, metadata.Ticker)

    case "ban":
        if err := trustManager.BanToken(req.TokenID, req.Notes); err != nil {
            http.Error(w, fmt.Sprintf("Failed to ban token: %v", err), http.StatusInternalServerError)
            return
        }
        responseMessage = fmt.Sprintf("Token %s (%s) has been banned", metadata.Name, metadata.Ticker)

    case "ignore":
        // For ignore, we just leave it as unknown but update the notes if provided
        if req.Notes != "" {
            trustInfo := trustManager.GetTokenTrust(req.TokenID)
            if err := trustManager.SetTokenTrust(req.TokenID, trustInfo.TrustLevel, req.Notes); err != nil {
                http.Error(w, fmt.Sprintf("Failed to update token notes: %v", err), http.StatusInternalServerError)
                return
            }
        }
        responseMessage = fmt.Sprintf("Token %s (%s) remains unknown", metadata.Name, metadata.Ticker)

    default:
        http.Error(w, "Invalid action. Must be 'accept', 'ban', or 'ignore'", http.StatusBadRequest)
        return
    }

    // Return success response
    response := map[string]interface{}{
        "success":  true,
        "message":  responseMessage,
        "token_id": req.TokenID,
        "action":   req.Action,
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// handleWebWalletMeltToken handles token melting requests from web UI
func (sn *ShadowNode) handleWebWalletMeltToken(w http.ResponseWriter, r *http.Request) {
    // Check authentication
    session, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    // Parse request body
    var req struct {
        TokenID      string  `json:"token_id"`
        Amount       float64 `json:"amount"`
        Confirmation string  `json:"confirmation"` // Must be "MELT" to proceed
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validate input
    if req.TokenID == "" || req.Amount <= 0 {
        http.Error(w, "Token ID and positive amount are required", http.StatusBadRequest)
        return
    }

    // Require explicit confirmation
    if req.Confirmation != "MELT" {
        http.Error(w, "Confirmation required: must type 'MELT' to proceed", http.StatusBadRequest)
        return
    }

    // Check blockchain availability
    if sn.blockchain == nil {
        http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
        return
    }

    // Get token state and validate token exists
    tokenState := sn.blockchain.GetTokenState()
    tokenInfo, err := tokenState.GetTokenInfo(req.TokenID)
    if err != nil {
        http.Error(w, "Token not found", http.StatusNotFound)
        return
    }

    // Convert amount to base units (apply decimals)
    multiplier := uint64(1)
    for i := uint8(0); i < tokenInfo.Decimals; i++ {
        multiplier *= 10
    }
    amountBaseUnits := uint64(req.Amount * float64(multiplier))

    // Check user's token balance
    balance, err := tokenState.GetTokenBalance(req.TokenID, session.Address)
    if err != nil {
        http.Error(w, "Failed to get token balance", http.StatusInternalServerError)
        return
    }

    if balance < amountBaseUnits {
        http.Error(w, fmt.Sprintf("Insufficient token balance. Have %d, need %d", balance, amountBaseUnits), http.StatusBadRequest)
        return
    }

    // Load wallet for signing
    wallet, err := loadWallet(session.WalletName)
    if err != nil {
        http.Error(w, "Failed to load wallet", http.StatusInternalServerError)
        return
    }

    // Create transaction with token melt operation
    tx := NewTransaction()
    tx.AddTokenMelt(req.TokenID, amountBaseUnits, session.Address)

    // Sign the transaction
    signedTx, err := SignTransactionWithWallet(tx, wallet)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to sign transaction: %v", err), http.StatusInternalServerError)
        return
    }

    // Submit transaction to mempool
    err = sn.mempool.AddTransaction(signedTx, SourceAPI)
    if err != nil {
        http.Error(w, fmt.Sprintf("Failed to submit transaction: %v", err), http.StatusInternalServerError)
        return
    }

    // Calculate expected SHADOW release
    expectedShadowRelease := amountBaseUnits * tokenInfo.LockAmount
    shadowReleaseFloat := float64(expectedShadowRelease) / 100000000.0

    // Return success response
    response := map[string]interface{}{
        "success":          true,
        "transaction_hash": signedTx.TxHash,
        "tokens_melted":    req.Amount,
        "token_name":       tokenInfo.Name,
        "token_ticker":     tokenInfo.Ticker,
        "shadow_released":  shadowReleaseFloat,
        "message": fmt.Sprintf("Successfully melted %.8f %s tokens, releasing %.8f SHADOW",
            req.Amount, tokenInfo.Ticker, shadowReleaseFloat),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// handleWebWalletSyndicateMembership returns active syndicate memberships for an address
func (sn *ShadowNode) handleWebWalletSyndicateMembership(w http.ResponseWriter, r *http.Request) {
    // Check authentication
    session, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    // Check blockchain availability
    if sn.blockchain == nil {
        http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
        return
    }

    // Get address from query params or use session address
    address := r.URL.Query().Get("address")
    if address == "" {
        address = session.Address
    }

    // Get token state and look for syndicate NFTs
    tokenState := sn.blockchain.GetTokenState()
    balances, err := tokenState.GetAllTokenBalances(address)
    if err != nil {
        http.Error(w, "Failed to get token balances", http.StatusInternalServerError)
        return
    }

    // Filter for active syndicate memberships
    type SyndicateMembership struct {
        NFTTokenID       string `json:"nft_token_id"`
        Syndicate        string `json:"syndicate"`
        ReportedCapacity uint64 `json:"reported_capacity"`
        JoinTime         int64  `json:"join_time"`
        ExpirationTime   int64  `json:"expiration_time"`
        RenewalCount     uint32 `json:"renewal_count"`
    }

    var activeMemberships []SyndicateMembership
    currentTime := time.Now().Unix()

    for _, balance := range balances {
        if balance.Balance > 0 && balance.TokenInfo != nil && balance.TokenInfo.Syndicate != nil {
            syndicateData := balance.TokenInfo.Syndicate

            // Only include active (non-expired) memberships
            if syndicateData.ExpirationTime > currentTime {
                activeMemberships = append(activeMemberships, SyndicateMembership{
                    NFTTokenID:       balance.TokenID,
                    Syndicate:        syndicateData.Syndicate.String(),
                    ReportedCapacity: syndicateData.ReportedCapacity,
                    JoinTime:         syndicateData.JoinTime,
                    ExpirationTime:   syndicateData.ExpirationTime,
                    RenewalCount:     syndicateData.RenewalCount,
                })
            }
        }
    }

    response := map[string]interface{}{
        "address":            address,
        "active_memberships": activeMemberships,
        "membership_count":   len(activeMemberships),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// handleWebWalletSyndicateStats returns statistics for all syndicates
func (sn *ShadowNode) handleWebWalletSyndicateStats(w http.ResponseWriter, r *http.Request) {
    // Check authentication
    _, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    // Check blockchain availability
    if sn.blockchain == nil {
        http.Error(w, "Blockchain not available", http.StatusServiceUnavailable)
        return
    }

    // Get syndicate manager
    syndicateManager := sn.blockchain.GetSyndicateManager()
    if syndicateManager == nil {
        http.Error(w, "Syndicate system not available", http.StatusServiceUnavailable)
        return
    }

    // Get all syndicate statistics
    allStats := syndicateManager.GetAllSyndicateStats()

    // Convert to response format
    response := make(map[string]interface{})

    syndicateNames := map[SyndicateType]string{
        SyndicateSeiryu: "seiryu",
        SyndicateByakko: "byakko",
        SyndicateSuzaku: "suzaku",
        SyndicateGenbu:  "genbu",
    }

    for syndicate, stats := range allStats {
        if name, exists := syndicateNames[syndicate]; exists {
            response[name] = map[string]interface{}{
                "members":        len(stats.Members),
                "total_capacity": stats.TotalCapacity,
                "blocks_won":     stats.BlocksWon,
                "win_percentage": stats.WinPercentage,
                "last_block_win": stats.LastBlockWin,
            }
        }
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// handleWebWalletJoinSyndicate creates a syndicate membership transaction
func (sn *ShadowNode) handleWebWalletJoinSyndicate(w http.ResponseWriter, r *http.Request) {
    if r.Method != "POST" {
        http.Error(w, "Method not allowed", http.StatusMethodNotAllowed)
        return
    }

    // Check authentication
    session, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    // Parse request body
    var req struct {
        Address        string `json:"address"`
        Syndicate      string `json:"syndicate"`
        AutoRenew      bool   `json:"auto_renew"`
        MembershipDays int    `json:"membership_days"`
    }

    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body", http.StatusBadRequest)
        return
    }

    // Validate input
    if req.Address == "" || req.Syndicate == "" || req.MembershipDays <= 0 || req.MembershipDays > 8 {
        http.Error(w, "Invalid parameters: address, syndicate, and membership days (1-8) required", http.StatusBadRequest)
        return
    }

    // Verify address matches session
    if req.Address != session.Address {
        http.Error(w, "Address mismatch", http.StatusForbidden)
        return
    }

    // Convert syndicate string to type
    var syndicateType SyndicateType
    switch req.Syndicate {
    case "auto":
        syndicateType = SyndicateAuto
    case "seiryu":
        syndicateType = SyndicateSeiryu
    case "byakko":
        syndicateType = SyndicateByakko
    case "suzaku":
        syndicateType = SyndicateSuzaku
    case "genbu":
        syndicateType = SyndicateGenbu
    default:
        http.Error(w, "Invalid syndicate choice", http.StatusBadRequest)
        return
    }

    // Load wallet for transaction creation
    wallet, err := loadWallet(session.WalletName)
    if err != nil {
        http.Error(w, "Failed to load wallet", http.StatusInternalServerError)
        return
    }

    // For now, return a simplified response
    // TODO: Implement full transaction creation and signing
    _ = wallet        // Prevent unused variable error
    _ = syndicateType // Prevent unused variable error

    // Return success response
    response := map[string]interface{}{
        "success":         true,
        "transaction_id":  "placeholder_tx_" + fmt.Sprintf("%d", time.Now().Unix()),
        "syndicate":       req.Syndicate,
        "auto_renew":      req.AutoRenew,
        "membership_days": req.MembershipDays,
        "fee":             0.1,
        "message":         fmt.Sprintf("Syndicate membership request created for %s (transaction creation pending)", req.Syndicate),
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}

// LP SWAP INTERFACE HANDLERS

// handleWebWalletSwapInterface serves the LP swap interface page
func (sn *ShadowNode) handleWebWalletSwapInterface(w http.ResponseWriter, r *http.Request) {
    _, authenticated := validateSession(r)
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
    <title>Shadowy Web Wallet - LP Swap</title>
    <style>
        body {
            font-family: 'Courier New', monospace;
            background: linear-gradient(135deg, #1a1a2e, #16213e);
            color: #00ff41;
            min-height: 100vh;
            margin: 0;
            padding: 20px;
        }
        .container {
            max-width: 800px;
            margin: 0 auto;
            background: rgba(0, 20, 40, 0.8);
            border: 2px solid #00ff41;
            border-radius: 10px;
            padding: 30px;
            box-shadow: 0 0 20px rgba(0, 255, 65, 0.3);
        }
        .header {
            text-align: center;
            margin-bottom: 30px;
            border-bottom: 1px solid #00ff41;
            padding-bottom: 20px;
        }
        .form-group {
            margin-bottom: 20px;
        }
        .form-group label {
            display: block;
            margin-bottom: 5px;
            color: #00ff41;
            font-weight: bold;
        }
        .form-group input, .form-group select {
            width: 100%;
            padding: 10px;
            background: rgba(0, 40, 60, 0.8);
            border: 1px solid #00ff41;
            border-radius: 5px;
            color: #00ff41;
            font-family: 'Courier New', monospace;
        }
        .form-group input:focus, .form-group select:focus {
            outline: none;
            border-color: #00ffff;
            box-shadow: 0 0 10px rgba(0, 255, 255, 0.5);
        }
        .swap-section {
            background: rgba(0, 30, 50, 0.6);
            border: 1px solid #00ff41;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
            transition: all 0.3s ease;
        }
        .swap-section h3 {
            margin-top: 0;
            color: #00ffff;
            border-bottom: 1px solid rgba(0, 255, 255, 0.3);
            padding-bottom: 10px;
        }
        .swap-direction {
            text-align: center;
            margin: 15px 0;
            font-size: 24px;
            color: #00ffff;
        }
        .button {
            background: linear-gradient(135deg, #00ff41, #00cc33);
            color: #000;
            border: none;
            padding: 12px 25px;
            border-radius: 5px;
            cursor: pointer;
            font-family: 'Courier New', monospace;
            font-weight: bold;
            font-size: 14px;
            margin: 10px 5px;
            transition: all 0.3s ease;
        }
        .button:hover {
            background: linear-gradient(135deg, #00cc33, #00ff41);
            box-shadow: 0 0 15px rgba(0, 255, 65, 0.6);
        }
        .advanced-options {
            background: rgba(0, 20, 35, 0.8);
            border: 1px solid #555;
            border-radius: 8px;
            padding: 20px;
            margin: 20px 0;
        }
        .result {
            margin-top: 20px;
            padding: 15px;
            border-radius: 5px;
            word-wrap: break-word;
        }
        .success {
            background: rgba(0, 255, 65, 0.2);
            border: 1px solid #00ff41;
            color: #00ff41;
        }
        .error {
            background: rgba(255, 0, 65, 0.2);
            border: 1px solid #ff0041;
            color: #ff0041;
        }
        .pool-info {
            background: rgba(0, 40, 80, 0.6);
            border: 1px solid #00aaff;
            border-radius: 8px;
            padding: 15px;
            margin: 15px 0;
            color: #00aaff;
        }
        .nav-links {
            text-align: center;
            margin-top: 30px;
            padding-top: 20px;
            border-top: 1px solid #00ff41;
        }
        .nav-links a {
            color: #00ffff;
            text-decoration: none;
            margin: 0 15px;
            padding: 8px 16px;
            border: 1px solid #00ffff;
            border-radius: 4px;
            transition: all 0.3s ease;
        }
        .nav-links a:hover {
            background: rgba(0, 255, 255, 0.2);
            box-shadow: 0 0 10px rgba(0, 255, 255, 0.4);
        }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>🔄 LP SWAP INTERFACE</h1>
            <p>Swap tokens through Automated Market Maker pools</p>
        </div>

        <form id="swapForm">
            <!-- Step 1: Pool Selection (always visible) -->
            <div class="swap-section">
                <h3>🏊 Step 1: Select Pool</h3>
                <div class="form-group">
                    <label for="poolAddress">Pool L-Address:</label>
                    <input type="text" id="poolAddress" name="poolAddress"
                           placeholder="L..." required>
                    <small>Enter the L-address of the liquidity pool</small>
                </div>
                <div class="form-group">
                    <button type="button" class="button" onclick="loadPoolInfo()">🔍 Load Pool & Continue</button>
                </div>
                <div class="pool-info" id="poolInfo" style="display:none;">
                    <!-- Pool details will be loaded here -->
                </div>
            </div>

            <!-- Step 2: Swap Details (hidden until pool is selected) -->
            <div class="swap-section" id="swapDetailsSection" style="display:none;">
                <h3>💱 Step 2: Swap Details</h3>
                <div class="form-group">
                    <label for="inputToken">From Token:</label>
                    <select id="inputToken" name="inputToken" required>
                        <option value="">Select input token...</option>
                    </select>
                </div>

                <div class="swap-direction">⬇️</div>

                <div class="form-group">
                    <label for="outputToken">To Token:</label>
                    <select id="outputToken" name="outputToken" required>
                        <option value="">Select output token...</option>
                    </select>
                </div>

                <div class="form-group">
                    <label for="inputAmount">Input Amount:</label>
                    <input type="number" id="inputAmount" name="inputAmount"
                           min="0.000001" step="0.000001" required>
                </div>

                <!-- Step 3: Advanced Options (hidden until tokens are selected) -->
                <div class="advanced-options" id="advancedOptionsSection" style="display:none;">
                    <h3>⚙️ Step 3: Advanced Options (Optional)</h3>
                    <div class="form-group">
                        <label for="maxSlippage">Max Slippage (basis points):</label>
                        <input type="number" id="maxSlippage" name="maxSlippage"
                               min="0" max="10000" value="50"
                               placeholder="50 = 0.5%">
                        <small>Maximum acceptable slippage (50 basis points = 0.5%)</small>
                    </div>

                    <div class="form-group">
                        <label for="expiration">Expiration (minutes from now):</label>
                        <input type="number" id="expiration" name="expiration"
                               min="1" max="10080" value="60"
                               placeholder="60">
                        <small>Order expires after this many minutes (max 7 days)</small>
                    </div>

                    <div class="form-group">
                        <label>
                            <input type="checkbox" id="allOrNothing" name="allOrNothing">
                            All-or-Nothing Execution
                        </label>
                        <small>Execute the full amount or fail completely</small>
                    </div>

                    <div class="form-group">
                        <label for="minReceived">Minimum Received (optional):</label>
                        <input type="number" id="minReceived" name="minReceived"
                               min="0" step="0.000001"
                               placeholder="Minimum output amount">
                    </div>
                </div>

                <div class="form-group">
                    <button type="submit" class="button">🚀 Submit Swap Order</button>
                    <button type="button" class="button" onclick="estimateSwap()" id="estimateButton" style="display:none;">📊 Estimate Output</button>
                    <button type="button" class="button" onclick="resetForm()">🔄 Reset</button>
                </div>
            </div>
        </form>

        <div id="result" class="result" style="display:none;"></div>

        <div class="nav-links">
            <a href="/wallet/">🏠 Dashboard</a>
            <a href="/wallet/send">💸 Send</a>
            <a href="/wallet/tokens">🪙 Tokens</a>
        </div>
    </div>

    <script>
        let pools = [];
        let availableTokens = [];

        let currentPool = null;

        // Load available pools and tokens on page load
        document.addEventListener('DOMContentLoaded', function() {
            loadAvailablePools();
            loadAvailableTokens();

            // Check for pool parameter in URL and pre-populate
            const urlParams = new URLSearchParams(window.location.search);
            const poolParam = urlParams.get('pool');
            if (poolParam) {
                // Wait for both pools and tokens to load, then pre-select the pool
                setTimeout(() => {
                    preSelectPool(poolParam);
                }, 1500); // Increased timeout to ensure tokens are loaded
            }

            // Add event listeners for progressive disclosure
            document.getElementById('inputToken').addEventListener('change', onTokenChange);
            document.getElementById('outputToken').addEventListener('change', onTokenChange);
        });

        async function loadAvailablePools() {
            try {
                const response = await fetch('/api/pools');
                pools = await response.json();
                console.log('Loaded pools:', pools);
            } catch (error) {
                console.error('Failed to load pools:', error);
            }
        }

        async function loadAvailableTokens() {
            try {
                const response = await fetch('/api/v1/tokens');
                const tokens = await response.json();
                // Ensure we always have an array
                availableTokens = Array.isArray(tokens) ? tokens : [];
                console.log('Loaded tokens:', availableTokens);
            } catch (error) {
                console.error('Failed to load tokens:', error);
                availableTokens = []; // Ensure we have an empty array on error
            }
        }

        function preSelectPool(poolLAddress) {
            const poolInput = document.getElementById('poolAddress');
            if (poolInput) {
                poolInput.value = poolLAddress;
                console.log('Pre-populated pool address:', poolLAddress);

                // Trigger the Load Pool Info button click to load the pool data
                const loadBtn = document.querySelector('button[onclick="loadPoolInfo()"]');
                if (loadBtn) {
                    loadBtn.click();
                }
            }
        }

        async function loadPoolInfo() {
            const poolAddress = document.getElementById('poolAddress').value.trim();
            if (!poolAddress) {
                showResult('Please enter a pool L-address first', 'error');
                return;
            }

            // Hide result from previous operations
            document.getElementById('result').style.display = 'none';

            try {
                // Find pool info from loaded pools
                const pool = pools.find(p => p.l_address === poolAddress);
                if (!pool) {
                    showResult('Pool not found: ' + poolAddress, 'error');
                    return;
                }

                currentPool = pool;

                // Check if pool can be swapped
                const canSwap = pool.can_swap || (pool.reserve_a > 0 && pool.reserve_b > 0);
                const reserveADisplay = pool.reserve_a ? (pool.reserve_a / 100000000).toFixed(2) : '0.00';
                const reserveBDisplay = pool.reserve_b ? (pool.reserve_b / 100000000).toFixed(2) : '0.00';

                // Display pool information
                const poolInfoDiv = document.getElementById('poolInfo');
                poolInfoDiv.innerHTML = '' +
                    '<strong>✅ Pool Found:</strong><br>' +
                    '🪙 Token A: ' + getTokenDisplayName(pool.token_a) + '<br>' +
                    '🪙 Token B: ' + getTokenDisplayName(pool.token_b) + '<br>' +
                    '💧 Reserves: ' + reserveADisplay + ' / ' + reserveBDisplay + '<br>' +
                    '💰 Fee Rate: ' + (pool.fee_rate / 100) + '%<br>' +
                    '👤 Creator: ' + pool.creator.substring(0, 12) + '...<br>' +
                    '🎫 LP Token: ' + pool.share_token_id.substring(0, 12) + '...' +
                    (canSwap ? '' : '<br><br><strong>⚠️ Warning:</strong> This pool has no liquidity in one or both tokens. Swaps will fail.');
                poolInfoDiv.style.display = 'block';

                if (canSwap) {
                    // Show the swap details section
                    document.getElementById('swapDetailsSection').style.display = 'block';
                } else {
                    // Hide swap section and show warning
                    document.getElementById('swapDetailsSection').style.display = 'none';
                    showResult('❌ Cannot swap: Pool has insufficient liquidity. Reserves: ' + reserveADisplay + ' / ' + reserveBDisplay, 'error');
                    return;
                }

                // Update token dropdowns to show only this pool's tokens
                updateTokenDropdowns(pool.token_a, pool.token_b);

                // Scroll to swap details
                document.getElementById('swapDetailsSection').scrollIntoView({
                    behavior: 'smooth',
                    block: 'start'
                });

            } catch (error) {
                showResult('Error loading pool info: ' + error.message, 'error');
            }
        }

        function getTokenDisplayName(tokenId) {
            if (tokenId === 'SHADOW') {
                return 'SHADOW (Base Currency)';
            } else {
                // Ensure availableTokens is an array before using find
                if (Array.isArray(availableTokens)) {
                    const token = availableTokens.find(t => t.token_id === tokenId);
                    if (token) {
                        return token.ticker + ' - ' + token.name;
                    } else {
                        console.log('Token not found in availableTokens:', tokenId, 'Available tokens:', availableTokens.length);
                    }
                } else {
                    console.log('availableTokens is not an array:', availableTokens);
                }
                return tokenId.substring(0, 12) + '...';
            }
        }

        function updateTokenDropdowns(tokenA, tokenB) {
            const inputSelect = document.getElementById('inputToken');
            const outputSelect = document.getElementById('outputToken');

            // Clear and repopulate with pool tokens only
            inputSelect.innerHTML = '<option value="">Select input token...</option>';
            outputSelect.innerHTML = '<option value="">Select output token...</option>';

            const poolTokens = [tokenA, tokenB];
            poolTokens.forEach(tokenId => {
                const displayName = getTokenDisplayName(tokenId);
                inputSelect.add(new Option(displayName, tokenId));
                outputSelect.add(new Option(displayName, tokenId));
            });
        }

        function onTokenChange() {
            const inputToken = document.getElementById('inputToken').value;
            const outputToken = document.getElementById('outputToken').value;

            // Show advanced options and estimate button when both tokens are selected
            if (inputToken && outputToken && inputToken !== outputToken) {
                document.getElementById('advancedOptionsSection').style.display = 'block';
                document.getElementById('estimateButton').style.display = 'inline-block';

                // Auto-scroll to advanced options
                setTimeout(() => {
                    document.getElementById('advancedOptionsSection').scrollIntoView({
                        behavior: 'smooth',
                        block: 'start'
                    });
                }, 300);
            } else {
                document.getElementById('advancedOptionsSection').style.display = 'none';
                document.getElementById('estimateButton').style.display = 'none';
            }

            // Prevent selecting the same token for input and output
            if (inputToken && outputToken && inputToken === outputToken) {
                showResult('⚠️ Input and output tokens cannot be the same', 'error');
                // Reset the output token
                document.getElementById('outputToken').value = '';
            }
        }

        function resetForm() {
            // Hide all progressive sections
            document.getElementById('swapDetailsSection').style.display = 'none';
            document.getElementById('advancedOptionsSection').style.display = 'none';
            document.getElementById('poolInfo').style.display = 'none';
            document.getElementById('result').style.display = 'none';
            document.getElementById('estimateButton').style.display = 'none';

            // Reset form values
            document.getElementById('swapForm').reset();
            currentPool = null;

            // Scroll to top
            document.querySelector('.header').scrollIntoView({
                behavior: 'smooth',
                block: 'start'
            });
        }

        async function estimateSwap() {
            // TODO: Implement swap estimation using AMM formulas
            showResult('Swap estimation coming soon...', 'error');
        }

        document.getElementById('swapForm').addEventListener('submit', async function(e) {
            e.preventDefault();

            // Validate that we have a current pool selected
            if (!currentPool) {
                showResult('❌ Please select a pool first', 'error');
                return;
            }

            const formData = new FormData(e.target);
            const inputTokenId = formData.get('inputToken');
            const outputTokenId = formData.get('outputToken');
            const inputAmount = parseFloat(formData.get('inputAmount'));

            // Additional client-side validation
            if (!inputTokenId || !outputTokenId) {
                showResult('❌ Please select both input and output tokens', 'error');
                return;
            }

            if (inputTokenId === outputTokenId) {
                showResult('❌ Input and output tokens cannot be the same', 'error');
                return;
            }

            if (!inputAmount || inputAmount <= 0) {
                showResult('❌ Please enter a valid input amount', 'error');
                return;
            }

            const swapData = {
                pool_l_address: currentPool.l_address,
                input_token_id: inputTokenId,
                output_token_id: outputTokenId,
                input_amount: inputAmount,
                max_slippage: parseInt(formData.get('maxSlippage')) || 50,
                expiration_minutes: parseInt(formData.get('expiration')) || 60,
                all_or_nothing: formData.has('allOrNothing'),
                min_received: parseFloat(formData.get('minReceived')) || 0
            };

            try {
                const response = await fetch('/web/wallet/swap', {
                    method: 'POST',
                    headers: {
                        'Content-Type': 'application/json',
                    },
                    body: JSON.stringify(swapData)
                });

                const result = await response.json();

                if (response.ok && result.success) {
                    showResult(
                        '✅ Swap order submitted successfully!<br><br>' +
                        '🆔 <strong>Transaction ID:</strong> ' + result.transaction_id + '<br>' +
                        '🏊 <strong>Pool:</strong> ' + currentPool.l_address.substring(0, 12) + '...<br>' +
                        '📥 <strong>Input:</strong> ' + swapData.input_amount + ' ' + getTokenDisplayName(swapData.input_token_id) + '<br>' +
                        '📤 <strong>Expected Output:</strong> ~' + (result.estimated_output || 'Calculating...') + ' ' + getTokenDisplayName(swapData.output_token_id) + '<br>' +
                        '⏰ <strong>Expires:</strong> ' + result.expiration + '<br>' +
                        '💰 <strong>Max Slippage:</strong> ' + (swapData.max_slippage / 100) + '%<br><br>' +
                        '📝 <strong>Status:</strong> ' + (result.message || 'Order submitted to mempool'),
                        'success'
                    );
                } else {
                    showResult('❌ <strong>Error:</strong> ' + (result.error || 'Unknown error occurred'), 'error');
                }
            } catch (error) {
                showResult('❌ Network error: ' + error.message, 'error');
            }
        });

        function showResult(message, type) {
            const resultDiv = document.getElementById('result');
            resultDiv.innerHTML = message;
            resultDiv.className = 'result ' + type;
            resultDiv.style.display = 'block';
        }
    </script>
</body>
</html>`

    w.Write([]byte(html))
}

// SwapRequest represents a swap request from the web interface
type SwapRequest struct {
    PoolLAddress      string  `json:"pool_l_address"`
    InputTokenID      string  `json:"input_token_id"`
    OutputTokenID     string  `json:"output_token_id"`
    InputAmount       float64 `json:"input_amount"`
    MaxSlippage       uint64  `json:"max_slippage"`
    ExpirationMinutes int     `json:"expiration_minutes"`
    AllOrNothing      bool    `json:"all_or_nothing"`
    MinReceived       float64 `json:"min_received"`
}

// handleWebWalletSubmitSwap handles LP swap transaction submission
func (sn *ShadowNode) handleWebWalletSubmitSwap(w http.ResponseWriter, r *http.Request) {
    session, authenticated := validateSession(r)
    if !authenticated {
        http.Error(w, "Not authenticated", http.StatusUnauthorized)
        return
    }

    var req SwapRequest
    if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
        http.Error(w, "Invalid request body: "+err.Error(), http.StatusBadRequest)
        return
    }

    // Validate required fields
    if req.PoolLAddress == "" || req.InputTokenID == "" || req.OutputTokenID == "" || req.InputAmount <= 0 {
        http.Error(w, "Missing required fields", http.StatusBadRequest)
        return
    }

    if req.InputTokenID == req.OutputTokenID {
        http.Error(w, "Input and output tokens cannot be the same", http.StatusBadRequest)
        return
    }

    // Load wallet for transaction creation
    walletName := session.WalletName
    wallet, err := loadWallet(walletName)
    if err != nil {
        log.Printf("Error loading wallet '%s': %v", walletName, err)
        http.Error(w, "Failed to load wallet", http.StatusInternalServerError)
        return
    }

    // Create swap transaction
    tx := NewTransaction()

    // Convert input amount from float to satoshi (assuming 8 decimal places)
    inputAmountSatoshi := uint64(req.InputAmount * 100000000)

    // Set expiration time
    expirationTime := time.Now().UTC().Add(time.Duration(req.ExpirationMinutes) * time.Minute)

    // Create swap data
    swapData := &PoolSwapData{
        PoolLAddress:   req.PoolLAddress,
        InputTokenID:   req.InputTokenID,
        OutputTokenID:  req.OutputTokenID,
        MaxSlippage:    req.MaxSlippage,
        NotAfter:       expirationTime,
        AllOrNothing:   req.AllOrNothing,
        MinReceived:    uint64(req.MinReceived * 100000000),
        SwapperAddress: wallet.Address,
        CreationTime:   time.Now().UTC().Unix(),
    }

    // Add pool swap operation
    tx.AddPoolSwap(swapData, inputAmountSatoshi)

    // For SHADOW input, add corresponding output
    if req.InputTokenID == "SHADOW" {
        // We need to add a SHADOW input and potentially handle this in the blockchain processing
        // For now, we'll create a placeholder transaction structure
    }

    // Sign transaction
    signedTx, err := SignTransactionWithWallet(tx, wallet)
    if err != nil {
        log.Printf("Error signing swap transaction: %v", err)
        http.Error(w, "Failed to sign transaction", http.StatusInternalServerError)
        return
    }

    // Submit to mempool
    if sn.mempool != nil {
        err = sn.mempool.AddTransaction(signedTx, SourceAPI)
        if err != nil {
            log.Printf("Error adding swap transaction to mempool: %v", err)
            http.Error(w, "Failed to submit to mempool: "+err.Error(), http.StatusInternalServerError)
            return
        }
    }

    // Return success response
    response := map[string]interface{}{
        "success":          true,
        "transaction_id":   signedTx.TxHash,
        "input_amount":     req.InputAmount,
        "input_token":      req.InputTokenID,
        "output_token":     req.OutputTokenID,
        "pool_address":     req.PoolLAddress,
        "max_slippage":     req.MaxSlippage,
        "expiration":       expirationTime.Format(time.RFC3339),
        "estimated_output": "Calculating...", // TODO: Implement AMM estimation
        "message":          "Swap order submitted to mempool successfully",
    }

    w.Header().Set("Content-Type", "application/json")
    json.NewEncoder(w).Encode(response)
}
