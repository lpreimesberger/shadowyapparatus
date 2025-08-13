# 🌟 Shadowy WASM Library

A unified WebAssembly library for interacting with Shadowy blockchain nodes, written in Go. This single library powers both CLI tools and web3 applications with consistent behavior.

## ✨ Features

- **Universal**: Same logic works in CLI, browser, and mobile apps
- **Secure**: Handles private keys and API authentication safely
- **Fast**: Compiled to WASM for optimal performance  
- **Remote Ready**: Built-in support for remote nodes with API keys
- **Auto-Discovery**: Automatically detects local running nodes

## 🏗️ Architecture

```
┌─────────────────────────────────────────┐
│           Shadowy WASM Library          │
│  ┌─────────────────────────────────────┤
│  │ • HTTP client (local & remote)      │
│  │ • API key authentication            │
│  │ • Address validation                │
│  │ • Balance queries                   │
│  │ • Node information                  │
│  │ • Connection management             │
│  └─────────────────────────────────────┤
└─────────────────────────────────────────┘
       │                        │
┌──────▼────────┐      ┌────────▼────────┐
│   CLI Tool    │      │   Web3 App      │
│  (Node.js)    │      │  (Browser)      │
└───────────────┘      └─────────────────┘
```

## 🚀 Quick Start

### 1. Build the WASM Library

```bash
cd shadowy-wasm
chmod +x build.sh
./build.sh
```

This creates:
- `shadowy.wasm` - The compiled WASM module
- `wasm_exec.js` - Go's WASM runtime support

### 2. Test the Library

```bash
# Test with Node.js
node test.js

# Test in browser
python3 -m http.server 8000
# Open http://localhost:8000/web-example.html
```

### 3. Use the CLI Tool

```bash
cd ../shadowy-cli

# Make it executable
chmod +x main.js

# Check balance
./main.js balance S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737

# Get node info
./main.js node

# Help
./main.js help
```

## 🔧 API Reference

### JavaScript/WASM Functions

```javascript
// Create client for local or remote node
shadowy_create_client(url)
// Returns: {success: true, message: "..."} or {error: "..."}

// Set API key for remote authentication  
shadowy_set_api_key(apiKey)
// Returns: {success: true, message: "..."} or {error: "..."}

// Test node connection
await shadowy_test_connection()
// Returns: {success: true, node_info: {...}} or throws error

// Get detailed node information
await shadowy_get_node_info()
// Returns: {tip_height: 1234, total_blocks: 1234, ...}

// Get wallet balance
await shadowy_get_balance(address)
// Returns: {address: "...", confirmed_balance_satoshi: 1000000000, ...}
```

## 🌐 Usage Examples

### CLI Usage

```bash
# Local node (auto-detected)
shadowy-cli balance S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737

# Remote node with API key
shadowy-cli balance S427... --node https://api.shadowy.network --api-key sk-123...

# Node information
shadowy-cli node
```

### Web3/Browser Usage

```html
<script src="wasm_exec.js"></script>
<script>
async function initShadowy() {
    // Load WASM
    const go = new Go();
    const result = await WebAssembly.instantiateStreaming(
        fetch('shadowy.wasm'), 
        go.importObject
    );
    go.run(result.instance);
    
    // Create client
    shadowy_create_client('https://api.shadowy.network');
    shadowy_set_api_key('sk-your-api-key');
    
    // Get balance
    const balance = await shadowy_get_balance('S427a724...');
    console.log('Balance:', balance.formatted_balance);
}
</script>
```

### Node.js Integration

```javascript
const fs = require('fs');
require('./wasm_exec.js');

async function useShadowy() {
    // Load WASM module
    const go = new Go();
    const wasmData = fs.readFileSync('./shadowy.wasm');
    const wasmModule = await WebAssembly.instantiate(wasmData, go.importObject);
    go.run(wasmModule.instance);
    
    // Use Shadowy functions
    shadowy_create_client('http://localhost:8080');
    const balance = await shadowy_get_balance('S427a724...');
    return balance;
}
```

## 🔑 Authentication

For remote nodes (like hosted Shadowy services), use API key authentication:

```javascript
// Set API key for all requests
shadowy_set_api_key('sk-your-secret-api-key-here');

// The key is sent as: Authorization: Bearer sk-your-secret-api-key-here
```

## 🏃‍♂️ Development

### Adding New Functions

1. Add the Go function to `main.go`:
```go
func newFeature(this js.Value, args []js.Value) interface{} {
    // Implementation
}

// Register in main()
js.Global().Set("shadowy_new_feature", js.FuncOf(newFeature))
```

2. Rebuild the WASM:
```bash
./build.sh
```

3. Use in JavaScript:
```javascript
const result = await shadowy_new_feature(param1, param2);
```

### Building for Production

```bash
# Optimize for size
export GOOS=js GOARCH=wasm
go build -ldflags="-s -w" -o shadowy.wasm main.go

# Further compress with wasm-opt (if installed)
wasm-opt -Oz shadowy.wasm -o shadowy.wasm
```

## 🔍 Troubleshooting

### Common Issues

**WASM loading fails:**
- Ensure web server serves `.wasm` files with correct MIME type
- Check browser console for detailed errors
- Verify `wasm_exec.js` matches your Go version

**Connection refused:**
- Start a Shadowy node: `go run . node`
- Check node is running on expected port (8080, 8081, etc.)
- Verify firewall/network settings for remote nodes

**Balance lookup fails:**
- Verify address format (should start with 'S' or 'L')
- Ensure node is fully synced
- Check API key for remote nodes

## 🎯 Next Steps

This proof-of-concept demonstrates the core architecture. To extend it:

1. **Add Transaction Signing**: Include private key handling and transaction creation
2. **Token Operations**: Support token creation, transfers, and LP swaps  
3. **Wallet Management**: Full wallet file handling and key derivation
4. **Mobile SDKs**: Create React Native/Flutter wrappers
5. **Error Handling**: Enhanced error messages and retry logic
6. **Caching**: Local storage for performance optimization

## 📝 License

This Shadowy WASM library is part of the Shadowy blockchain project.