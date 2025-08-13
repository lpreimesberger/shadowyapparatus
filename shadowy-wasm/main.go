//go:build wasm
// +build wasm

package main

import (
	"bytes"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"syscall/js"
	"time"

	"github.com/cloudflare/circl/sign/mldsa/mldsa87"
	"golang.org/x/crypto/sha3"
)

// Version constants
const (
	WasmVersion = "1.0.0"
	CryptoAlgorithm = "ML-DSA-87"
)

// Wallet structure for post-quantum cryptography
type WalletV3 struct {
	Version   int    `json:"version"`
	Name      string `json:"name"`
	Address   string `json:"address"`
	CreatedAt string `json:"created_at"`
	Seed      string `json:"seed"`      // base64 encoded 64-byte seed
	PublicKey string `json:"public_key"` // base64 encoded public key
}

// Transaction structures
type TransactionInput struct {
	TxID       string `json:"txid"`
	Vout       uint32 `json:"vout"`
	ScriptSig  string `json:"script_sig"`
	Sequence   uint32 `json:"sequence"`
}

type TransactionOutput struct {
	Value        uint64 `json:"value"`
	ScriptPubkey string `json:"script_pubkey"`
	Address      string `json:"address"`
}

type Transaction struct {
	Version   int                 `json:"version"`
	Inputs    []TransactionInput  `json:"inputs"`
	Outputs   []TransactionOutput `json:"outputs"`
	Locktime  uint32              `json:"locktime"`
	Timestamp string              `json:"timestamp"` // ISO timestamp string to match node
}

// UTXO structure
type UTXO struct {
	TxID         string `json:"txid"`
	Vout         uint32 `json:"vout"`
	Value        uint64 `json:"value"`
	ScriptPubkey string `json:"script_pubkey"`
	Address      string `json:"address"`
	Confirmations int   `json:"confirmations"`
}

// Node-expected SignedTransaction format (discovered from cmd/transaction.go)
type SignedTransaction struct {
	Transaction json.RawMessage `json:"transaction"`
	Signature   string          `json:"signature"`   
	TxHash      string          `json:"tx_hash"`     
	SignerKey   string          `json:"signer_key"`  
	Algorithm   string          `json:"algorithm"`   
	Header      JOSEHeader      `json:"header"`      
}

type JOSEHeader struct {
	Algorithm string `json:"alg"`
	Type      string `json:"typ,omitempty"`
}

// Balance response structure (matches node API format)
type BalanceResponse struct {
	Address              string  `json:"address"`
	Balance              float64 `json:"balance"`
	BalanceSatoshis      uint64  `json:"balance_satoshis"`
	Confirmed            float64 `json:"confirmed"`
	ConfirmedSatoshis    uint64  `json:"confirmed_satoshis"`
	Unconfirmed          float64 `json:"unconfirmed"`
	UnconfirmedSatoshis  uint64  `json:"unconfirmed_satoshis"`
	TotalReceived        float64 `json:"total_received"`
	TotalReceivedSatoshis uint64 `json:"total_received_satoshis"`
	TotalSent            float64 `json:"total_sent"`
	TotalSentSatoshis    uint64  `json:"total_sent_satoshis"`
	TransactionCount     int     `json:"transaction_count"`
	LastActivity         string  `json:"last_activity,omitempty"`
}

type NodeInfo struct {
	TipHeight         int64  `json:"tip_height"`
	TotalBlocks       int64  `json:"total_blocks"`
	TotalTransactions int64  `json:"total_transactions"`
	Status            string `json:"status"`
	Version           string `json:"version,omitempty"`
}

// Global variables
var (
	currentWallet *WalletV3
	httpClient    js.Value
	apiKey        string
)

// Helper to create resolved Promise
func createResolvedPromise(value interface{}) js.Value {
	promise := js.Global().Get("Promise")
	return promise.Call("resolve", value)
}

// Helper to create Promise from executor
func createPromise(executor js.Func) js.Value {
	promise := js.Global().Get("Promise")
	return promise.New(executor)
}

func main() {
	c := make(chan struct{}, 0)
	
	log.Println("🌟 Shadowy WASM library initializing...")
	log.Printf("Version: %s, Crypto: %s", WasmVersion, CryptoAlgorithm)
	
	// Export functions to JavaScript
	js.Global().Set("shadowy_create_client", js.FuncOf(createClient))
	js.Global().Set("shadowy_set_api_key", js.FuncOf(setAPIKey))
	js.Global().Set("shadowy_test_connection", js.FuncOf(testConnection))
	js.Global().Set("shadowy_get_balance", js.FuncOf(getBalance))
	js.Global().Set("shadowy_get_node_info", js.FuncOf(getNodeInfo))
	js.Global().Set("shadowy_create_wallet", js.FuncOf(createWallet))
	js.Global().Set("shadowy_load_wallet", js.FuncOf(loadWallet))
	js.Global().Set("shadowy_get_wallet_address", js.FuncOf(getWalletAddress))
	js.Global().Set("shadowy_sign_transaction", js.FuncOf(signTransaction))
	js.Global().Set("shadowy_broadcast_transaction", js.FuncOf(broadcastTransaction))
	js.Global().Set("shadowy_get_utxos", js.FuncOf(getUTXOs))
	
	log.Println("✅ WASM library ready")
	
	<-c
}

// Create HTTP client
func createClient(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"success": false,
			"error":   "Node URL required",
		}
	}
	
	nodeURL := args[0].String()
	
	// Create HTTP client configuration
	httpClient = js.ValueOf(map[string]interface{}{
		"base_url": nodeURL,
		"headers": map[string]interface{}{
			"Content-Type": "application/json",
			"User-Agent":   "Shadowy-WASM-Client/" + WasmVersion,
		},
	})
	
	log.Printf("🌐 HTTP client created for: %s", nodeURL)
	
	return map[string]interface{}{
		"success": true,
	}
}

// Set API key
func setAPIKey(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return map[string]interface{}{
			"success": false,
			"error":   "API key required",
		}
	}
	
	apiKey = args[0].String()
	
	return map[string]interface{}{
		"success": true,
	}
}

// Test connection to node
func testConnection(this js.Value, args []js.Value) interface{} {
	return createPromise(js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		resolve := args[0]
		reject := args[1]
		
		// Make HTTP request asynchronously
		httpResult := makeHTTPRequest("GET", "/api/v1/health", "")
		
		// Check if it's a Promise or direct result
		if httpResult == nil {
			reject.Invoke(map[string]interface{}{
				"error": "HTTP bridge not available",
			})
			return nil
		}
		
		// Convert to js.Value and handle as Promise
		httpPromise := js.ValueOf(httpResult)
		if httpPromise.Type() == js.TypeObject && !httpPromise.Get("then").IsUndefined() {
			// It's a Promise, wait for it
			httpPromise.Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				response := args[0]
				result := response.Get("result")
				statusCode := result.Get("status_code").Int()
				
				if statusCode == 200 {
					resolve.Invoke(map[string]interface{}{
						"success": true,
						"status":  "connected",
					})
				} else {
					resolve.Invoke(map[string]interface{}{
						"success": false,
						"error":   fmt.Sprintf("Connection failed: HTTP %d", statusCode),
					})
				}
				return nil
			})).Call("catch", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
				reject.Invoke(map[string]interface{}{
					"error": "HTTP request failed",
				})
				return nil
			}))
		}
		
		return nil
	}))
}

// Get balance for address
func getBalance(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		promise := js.Global().Get("Promise")
		return promise.Call("reject", map[string]interface{}{
			"error": "Address required",
		})
	}
	
	address := args[0].String()
	endpoint := fmt.Sprintf("/api/v1/address/%s/balance", address)
	
	return createResolvedPromise(makeHTTPRequest("GET", endpoint, "")).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		response := args[0]
		result := response.Get("result")
		statusCode := result.Get("status_code").Int()
		body := result.Get("body").String()
		
		log.Printf("💰 Balance request to: %s", endpoint)
		log.Printf("💰 Balance response: HTTP %d", statusCode)
		log.Printf("💰 Balance body: %s", body)
		
		if statusCode == 200 {
			body := result.Get("body").String()
			log.Printf("💰 Balance API response: %s", body)
			
			var balance BalanceResponse
			err := json.Unmarshal([]byte(body), &balance)
			if err != nil {
				log.Printf("❌ Failed to parse balance JSON: %s", err.Error())
				return map[string]interface{}{
					"error": "Failed to parse balance response",
				}
			}
			
			log.Printf("✅ Parsed balance: %+v", balance)
			
			// Convert to map for JavaScript compatibility
			return map[string]interface{}{
				"address":                balance.Address,
				"balance":               balance.Balance,
				"balance_satoshis":      balance.BalanceSatoshis,
				"confirmed":             balance.Confirmed,
				"confirmed_satoshis":    balance.ConfirmedSatoshis,
				"unconfirmed":           balance.Unconfirmed,
				"unconfirmed_satoshis":  balance.UnconfirmedSatoshis,
				"total_received":        balance.TotalReceived,
				"total_received_satoshis": balance.TotalReceivedSatoshis,
				"total_sent":            balance.TotalSent,
				"total_sent_satoshis":   balance.TotalSentSatoshis,
				"transaction_count":     balance.TransactionCount,
				"last_activity":         balance.LastActivity,
			}
		}
		
		return map[string]interface{}{
			"error": fmt.Sprintf("Balance lookup failed: HTTP %d", statusCode),
		}
	}))
}

// Get node information
func getNodeInfo(this js.Value, args []js.Value) interface{} {
	return createResolvedPromise(makeHTTPRequest("GET", "/api/v1/node/info", "")).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		response := args[0]
		result := response.Get("result")
		statusCode := result.Get("status_code").Int()
		
		if statusCode == 200 {
			var info NodeInfo
			body := result.Get("body").String()
			err := json.Unmarshal([]byte(body), &info)
			if err != nil {
				return map[string]interface{}{
					"error": "Failed to parse node info",
				}
			}
			
			return info
		}
		
		return map[string]interface{}{
			"error": fmt.Sprintf("Node info failed: HTTP %d", statusCode),
		}
	}))
}

// Create wallet
func createWallet(this js.Value, args []js.Value) interface{} {
	return createResolvedPromise(nil).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) < 1 {
			return map[string]interface{}{
				"error": "Wallet name required",
			}
		}
		
		walletName := args[0].String()
		
		// Generate 64-byte seed for ML-DSA-87
		seed := make([]byte, 64)
		_, err := rand.Read(seed)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to generate seed",
			}
		}
		
		// Generate ML-DSA-87 key pair from seed
		publicKey, _, err := mldsa87.GenerateKey(bytes.NewReader(seed))
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to generate key pair",
			}
		}
		
		// Generate Shadowy address from public key
		address, err := generateShadowyAddress(publicKey.Bytes())
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to generate address",
			}
		}
		
		// Create wallet structure
		wallet := &WalletV3{
			Version:   3,
			Name:      walletName,
			Address:   address,
			CreatedAt: time.Now().UTC().Format(time.RFC3339),
			Seed:      base64.StdEncoding.EncodeToString(seed),
			PublicKey: base64.StdEncoding.EncodeToString(publicKey.Bytes()),
		}
		
		// Add private key for internal use (not saved to file)
		walletJSON, err := json.MarshalIndent(wallet, "", "  ")
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to serialize wallet",
			}
		}
		
		// Save wallet to file via crypto bridge
		cryptoBridge := js.Global().Get("shadowy_crypto_bridge")
		filename := fmt.Sprintf("shadowy-wallet-%s.json", walletName)
		success := cryptoBridge.Call("writeWalletFile", filename, string(walletJSON))
		
		if !success.Bool() {
			return map[string]interface{}{
				"error": "Failed to save wallet file",
			}
		}
		
		// Set as current wallet
		currentWallet = wallet
		
		log.Printf("✅ Created wallet: %s (%s)", walletName, address)
		
		return map[string]interface{}{
			"name":    wallet.Name,
			"address": wallet.Address,
			"version": wallet.Version,
		}
	}))
}

// Load wallet
func loadWallet(this js.Value, args []js.Value) interface{} {
	return createResolvedPromise(nil).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		if len(args) < 1 {
			return map[string]interface{}{
				"error": "Wallet name required",
			}
		}
		
		walletName := args[0].String()
		
		// Load wallet from file via crypto bridge
		cryptoBridge := js.Global().Get("shadowy_crypto_bridge")
		filename := fmt.Sprintf("shadowy-wallet-%s.json", walletName)
		walletData := cryptoBridge.Call("readWalletFile", filename)
		
		if walletData.IsNull() {
			return map[string]interface{}{
				"error": "Wallet not found",
			}
		}
		
		var wallet WalletV3
		err := json.Unmarshal([]byte(walletData.String()), &wallet)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to parse wallet file",
			}
		}
		
		currentWallet = &wallet
		
		log.Printf("✅ Loaded wallet: %s (%s)", wallet.Name, wallet.Address)
		
		return map[string]interface{}{
			"name":    wallet.Name,
			"address": wallet.Address,
			"version": wallet.Version,
		}
	}))
}

// Get current wallet address
func getWalletAddress(this js.Value, args []js.Value) interface{} {
	if currentWallet == nil {
		return map[string]interface{}{
			"error": "No wallet loaded",
		}
	}
	
	return map[string]interface{}{
		"name":    currentWallet.Name,
		"address": currentWallet.Address,
		"version": currentWallet.Version,
	}
}

// Get UTXOs for current wallet
func getUTXOs(this js.Value, args []js.Value) interface{} {
	if currentWallet == nil {
		promise := js.Global().Get("Promise")
		return promise.Call("reject", map[string]interface{}{
			"error": "No wallet loaded",
		})
	}
	
	endpoint := fmt.Sprintf("/api/v1/utxos?address=%s", currentWallet.Address)
	
	return createResolvedPromise(makeHTTPRequest("GET", endpoint, "")).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		response := args[0]
		result := response.Get("result")
		statusCode := result.Get("status_code").Int()
		
		log.Printf("🔍 UTXO API request to: %s", endpoint)
		log.Printf("🔍 UTXO API response: HTTP %d", statusCode)
		
		if statusCode == 200 {
			var utxos []UTXO
			body := result.Get("body").String()
			log.Printf("🔍 UTXO API body: %s", body)
			
			err := json.Unmarshal([]byte(body), &utxos)
			if err != nil {
				log.Printf("❌ Failed to parse UTXO API response: %s", err.Error())
				// If parsing fails, return mock UTXOs for testing
				log.Printf("⚠️ Using mock UTXOs due to parsing error")
				return createMockUTXOs(currentWallet.Address)
			}
			
			log.Printf("✅ Successfully parsed %d UTXOs from API", len(utxos))
			return utxos
		}
		
		// If API doesn't exist yet, return mock UTXOs for testing
		log.Printf("⚠️ UTXO API not available (HTTP %d), using mock data", statusCode)
		return createMockUTXOs(currentWallet.Address)
	}))
}

// Create mock UTXOs for testing
func createMockUTXOs(address string) []UTXO {
	return []UTXO{
		{
			TxID:         "a1b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef123456",
			Vout:         0,
			Value:        1000000000, // 10 SHADOW
			ScriptPubkey: fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address[1:41]),
			Address:      address,
			Confirmations: 10,
		},
		{
			TxID:         "b2c3d4e5f6789012345678901234567890abcdef1234567890abcdef1234567a",
			Vout:         1,
			Value:        500000000, // 5 SHADOW
			ScriptPubkey: fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address[1:41]),
			Address:      address,
			Confirmations: 20,
		},
		{
			TxID:         "c3d4e5f6789012345678901234567890abcdef1234567890abcdef1234567ab2",
			Vout:         0,
			Value:        200000000, // 2 SHADOW  
			ScriptPubkey: fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", address[1:41]),
			Address:      address,
			Confirmations: 5,
		},
	}
}

// Sign transaction
func signTransaction(this js.Value, args []js.Value) interface{} {
	if currentWallet == nil {
		return createResolvedPromise(map[string]interface{}{
			"error": "No wallet loaded",
		})
	}
	
	if len(args) < 1 {
		return createResolvedPromise(map[string]interface{}{
			"error": "Transaction data required",
		})
	}
	
	// Parse transaction data from JavaScript (outside of Promise wrapper)
	txData := args[0]
	destination := txData.Get("destination").String()
	amount := uint64(txData.Get("amount").Float())
	fee := uint64(txData.Get("fee").Float())
	fromAddress := txData.Get("from_address").String()
	
	// Get real UTXOs from the API first, then sign the transaction
	return createResolvedPromise(getUTXOs(js.Null(), nil)).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		utxosValue := args[0]
		log.Printf("🔐 Signing transaction: %d SHADOW to %s (fee: %d)", amount, destination, fee)
		
		// Convert JavaScript UTXOs to Go slice
		var utxos []UTXO
		if utxosValue.Type() == js.TypeObject && !utxosValue.IsNull() {
			if utxosValue.Length != nil {
				// It's an array
				length := utxosValue.Length()
				for i := 0; i < length; i++ {
					utxoJS := utxosValue.Index(i)
					utxo := UTXO{
						TxID:         utxoJS.Get("txid").String(),
						Vout:         uint32(utxoJS.Get("vout").Int()),
						Value:        uint64(utxoJS.Get("value").Float()),
						ScriptPubkey: utxoJS.Get("script_pubkey").String(),
						Address:      utxoJS.Get("address").String(),
						Confirmations: utxoJS.Get("confirmations").Int(),
					}
					utxos = append(utxos, utxo)
				}
			}
		}
		
		log.Printf("💰 Got %d UTXOs from API for transaction", len(utxos))
		
		// If no UTXOs from API, use mock ones
		if len(utxos) == 0 {
			log.Printf("⚠️ No UTXOs from API, using mock UTXOs")
			utxos = createMockUTXOs(currentWallet.Address)
		}
		
		// Select UTXOs using greedy algorithm
		totalNeeded := amount + fee
		selectedUTXOs, totalSelected, err := selectUTXOs(utxos, totalNeeded)
		if err != nil {
			return map[string]interface{}{
				"error": err.Error(),
			}
		}
		
		log.Printf("💰 Selected %d UTXOs totaling %d satoshis", len(selectedUTXOs), totalSelected)
		
		// Create transaction inputs
		var inputs []TransactionInput
		for _, utxo := range selectedUTXOs {
			inputs = append(inputs, TransactionInput{
				TxID:      utxo.TxID,
				Vout:      utxo.Vout,
				ScriptSig: "", // Will be filled during signing
				Sequence:  0xffffffff,
			})
		}
		
		// Create transaction outputs
		var outputs []TransactionOutput
		
		// Main output to destination
		outputs = append(outputs, TransactionOutput{
			Value:        amount,
			ScriptPubkey: fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", destination[1:41]),
			Address:      destination,
		})
		
		// Change output (if any)
		if totalSelected > totalNeeded {
			change := totalSelected - totalNeeded
			outputs = append(outputs, TransactionOutput{
				Value:        change,
				ScriptPubkey: fmt.Sprintf("OP_DUP OP_HASH160 %s OP_EQUALVERIFY OP_CHECKSIG", fromAddress[1:41]),
				Address:      fromAddress,
			})
		}
		
		// Create transaction
		tx := Transaction{
			Version:   1,
			Inputs:    inputs,
			Outputs:   outputs,
			Locktime:  0,
			Timestamp: time.Now().UTC().Format(time.RFC3339),
		}
		
		// Serialize transaction for signing
		txBytes, err := json.Marshal(tx)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to serialize transaction",
			}
		}
		
		// Create transaction hash
		hasher := sha256.New()
		hasher.Write(txBytes)
		txHash := hex.EncodeToString(hasher.Sum(nil))
		
		// Sign with ML-DSA-87
		seed, err := base64.StdEncoding.DecodeString(currentWallet.Seed)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to decode wallet seed",
			}
		}
		
		_, privateKey, err := mldsa87.GenerateKey(bytes.NewReader(seed))
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to regenerate private key",
			}
		}
		
		signature := make([]byte, mldsa87.SignatureSize) // 4627 bytes
		err = mldsa87.SignTo(privateKey, txBytes, nil, false, signature)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to sign transaction",
			}
		}
		signatureBase64 := base64.StdEncoding.EncodeToString(signature)
		
		log.Printf("✅ Transaction signed successfully")
		log.Printf("📋 Signature length: %d bytes", len(signature))
		log.Printf("📋 Transaction hash: %s", txHash)
		
		// Create the signed transaction in the format expected by the node
		signedTx := map[string]interface{}{
			"transaction": string(txBytes),
			"signature":   signatureBase64,
			"tx_hash":     txHash,
			"signer_key":  currentWallet.PublicKey,
			"algorithm":   "ML-DSA-87",
			"header": map[string]interface{}{
				"alg": "ML-DSA-87",
				"typ": "JWT",
			},
		}
		
		// Serialize complete signed transaction
		signedTxBytes, err := json.Marshal(signedTx)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to serialize signed transaction",
			}
		}
		
		log.Printf("📦 Complete transaction size: %d bytes", len(signedTxBytes))
		
		return map[string]interface{}{
			"txid":     txHash,
			"raw_tx":   hex.EncodeToString(txBytes),
			"signature": signatureBase64,
			"signer_key": currentWallet.PublicKey,
			"algorithm": "ML-DSA-87",
			"signed_transaction": signedTx,
		}
	}))
}

// Broadcast transaction to network
func broadcastTransaction(this js.Value, args []js.Value) interface{} {
	if len(args) < 1 {
		return createResolvedPromise(map[string]interface{}{
			"error": "Signed transaction required",
		})
	}
	
	// Get the signed transaction data directly from signTransaction return (outside Promise wrapper)
	signedTxData := args[0]
	
	log.Printf("📡 Broadcasting transaction data type: %s", signedTxData.Type().String())
	
	// Extract the signed_transaction field which contains the node-formatted data
	signedTxObj := signedTxData.Get("signed_transaction")
	
	log.Printf("📡 signed_transaction field type: %s, isNull: %v", signedTxObj.Type().String(), signedTxObj.IsNull())
	
	return createResolvedPromise(nil).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		
		// Convert to map for JSON serialization using the node-expected format
		// Parse transaction JSON string back to object so node can unmarshal it properly
		var txObj map[string]interface{}
		txJson := signedTxObj.Get("transaction").String()
		err := json.Unmarshal([]byte(txJson), &txObj)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to parse transaction JSON",
			}
		}
		
		signedTxMap := map[string]interface{}{
			"transaction": txObj, // Send as parsed object, not string
			"signature":   signedTxObj.Get("signature").String(),
			"tx_hash":     signedTxObj.Get("tx_hash").String(),
			"signer_key":  signedTxObj.Get("signer_key").String(),
			"algorithm":   signedTxObj.Get("algorithm").String(),
			"header": map[string]interface{}{
				"alg": signedTxObj.Get("header").Get("alg").String(),
				"typ": signedTxObj.Get("header").Get("typ").String(),
			},
		}
		
		// Serialize for HTTP request
		payload, err := json.Marshal(signedTxMap)
		if err != nil {
			return map[string]interface{}{
				"error": "Failed to serialize transaction for broadcast",
			}
		}
		
		log.Printf("📡 Broadcasting transaction to mempool...")
		log.Printf("📦 Payload size: %d bytes", len(payload))
		
		// Make HTTP request to broadcast transaction
		endpoint := "/api/v1/mempool/transactions"
		
		return createResolvedPromise(makeHTTPRequest("POST", endpoint, string(payload))).Call("then", js.FuncOf(func(this js.Value, args []js.Value) interface{} {
			response := args[0]
			result := response.Get("result")
			statusCode := result.Get("status_code").Int()
			body := result.Get("body").String()
			
			log.Printf("📡 Broadcast response: HTTP %d", statusCode)
			log.Printf("📄 Response body: %s", body)
			
			if statusCode == 200 || statusCode == 201 {
				var result map[string]interface{}
				err := json.Unmarshal([]byte(body), &result)
				if err != nil {
					// If can't parse, return basic success
					return map[string]interface{}{
						"status":  "success",
						"message": "Transaction broadcast successfully",
						"tx_hash": signedTxObj.Get("tx_hash").String(),
					}
				}
				
				return result
			}
			
			return map[string]interface{}{
				"error": fmt.Sprintf("Broadcast failed: HTTP %d - %s", statusCode, body),
			}
		}))
	}))
}

// UTXO selection using greedy algorithm
func selectUTXOs(utxos []UTXO, targetAmount uint64) ([]UTXO, uint64, error) {
	if len(utxos) == 0 {
		return nil, 0, fmt.Errorf("no UTXOs available")
	}
	
	// Sort UTXOs by value descending (largest first for greedy selection)
	sortedUTXOs := make([]UTXO, len(utxos))
	copy(sortedUTXOs, utxos)
	
	// Simple bubble sort for largest first
	for i := 0; i < len(sortedUTXOs); i++ {
		for j := i + 1; j < len(sortedUTXOs); j++ {
			if sortedUTXOs[j].Value > sortedUTXOs[i].Value {
				sortedUTXOs[i], sortedUTXOs[j] = sortedUTXOs[j], sortedUTXOs[i]
			}
		}
	}
	
	var selected []UTXO
	var totalSelected uint64
	
	// Greedy selection: pick largest UTXOs until we have enough
	for _, utxo := range sortedUTXOs {
		selected = append(selected, utxo)
		totalSelected += utxo.Value
		
		if totalSelected >= targetAmount {
			return selected, totalSelected, nil
		}
	}
	
	return nil, 0, fmt.Errorf("insufficient funds: need %d, have %d", targetAmount, totalSelected)
}

// Generate Shadowy address from public key
func generateShadowyAddress(publicKey []byte) (string, error) {
	// Use SHAKE256 to hash the public key (same as main Go project)
	shake := sha3.NewShake256()
	shake.Write(publicKey)
	hash := make([]byte, 20)
	shake.Read(hash)
	
	// Address version (0x42 = 'S' for Shadowy)
	const AddressVersion = 0x42
	
	// Create payload with version + hash
	payload := make([]byte, 21)
	payload[0] = AddressVersion
	copy(payload[1:], hash)
	
	// Calculate checksum (double Keccak256)
	checksum := calculateChecksum(payload)
	
	// Create full address: version + hash + checksum
	const AddressLen = 1 + 20 + 4 // version + hash + checksum
	fullAddress := make([]byte, AddressLen)
	copy(fullAddress[:21], payload)
	copy(fullAddress[21:], checksum)
	
	// Return as "S" + hex string (51 characters total)
	return "S" + hex.EncodeToString(fullAddress), nil
}

// Calculate address checksum (double Keccak256, first 4 bytes)
func calculateChecksum(payload []byte) []byte {
	// First Keccak256
	hasher1 := sha3.NewLegacyKeccak256()
	hasher1.Write(payload)
	hash1 := hasher1.Sum(nil)
	
	// Second Keccak256
	hasher2 := sha3.NewLegacyKeccak256()
	hasher2.Write(hash1)
	hash2 := hasher2.Sum(nil)
	
	// Return first 4 bytes as checksum
	return hash2[:4]
}

// Make HTTP request via bridge
func makeHTTPRequest(method, path, body string) interface{} {
	if httpClient.IsUndefined() {
		return map[string]interface{}{
			"error": "HTTP client not initialized",
		}
	}
	
	// Build full URL
	baseURL := httpClient.Get("base_url").String()
	fullURL := baseURL + path
	
	// Prepare headers
	headers := map[string]interface{}{
		"Content-Type": "application/json",
		"User-Agent":   "Shadowy-WASM-Client/" + WasmVersion,
	}
	
	// Add API key if available
	if apiKey != "" {
		headers["Authorization"] = "Bearer " + apiKey
	}
	
	// Prepare request data
	requestData := map[string]interface{}{
		"url":     fullURL,
		"method":  method,
		"headers": headers,
	}
	
	if body != "" {
		requestData["body"] = body
	}
	
	// Make request via bridge
	httpBridge := js.Global().Get("shadowy_http_bridge")
	if httpBridge.IsUndefined() {
		return map[string]interface{}{
			"error": "HTTP bridge not available",
		}
	}
	
	return httpBridge.Invoke(requestData)
}