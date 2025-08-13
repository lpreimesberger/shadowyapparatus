# Shadowy Blockchain Explorer

A web-based explorer and Web3 gateway for the Shadowy blockchain network.

## Features

- 🏗️ **Block Explorer** - Browse blocks, transactions, and network statistics
- 🌐 **Web3 API** - JSON-RPC interface for dApp development  
- 💧 **Liquidity Pools** - Built-in AMM exploration
- ⚡ **Proof-of-Storage** - Unique consensus mechanism
- 🪙 **Token System** - Native token creation and management
- ⏰ **Timelord** - VDF-based timing consensus

## Quick Start

```bash
# Navigate to explorer directory
cd explorer

# Install dependencies
go mod tidy

# Start the explorer (connects to local Shadowy node)
go run main.go
```

The explorer will be available at: http://localhost:10001

## Configuration

By default, the explorer connects to a local Shadowy node at `http://localhost:8080`. This can be configured in the code or via environment variables (coming soon).

## Architecture

- **Port 10001** - Web interface and API
- **Backend** - Go-based HTTP server
- **Frontend** - Modern responsive web interface
- **WASM Integration** - Coming soon for Web3 functionality

## API Endpoints

- `GET /` - Main explorer interface
- `GET /api/v1/health` - Health check endpoint
- More endpoints coming soon...

## Development

This explorer is designed to be lightweight and fast, providing both human-readable blockchain exploration and programmatic Web3 access for developers.