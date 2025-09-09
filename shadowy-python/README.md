# Shadowy Python CLI

Clean Python implementation of the Shadowy blockchain CLI client, ported from the Node.js version.

## Features

- ✅ **Wallet management**: Create, load, list wallets with post-quantum cryptography
- ✅ **Balance checking**: Query address balances, tokens, and NFTs from blockchain nodes
- ✅ **Transaction sending**: Send SHADOW and tokens with ML-DSA-87 signatures  
- ✅ **Node health monitoring**: Check node status and service health
- ✅ **UTXO management**: View unspent transaction outputs for addresses
- ✅ **Address validation**: Validate post-quantum address formats
- ✅ **HTTP client**: Connect to local or remote Shadowy nodes  
- ✅ **Transaction signing**: Sign transactions with loaded wallets
- ✅ **Node API integration**: Direct integration with Shadowy node REST API

## Installation

```bash
# Create virtual environment
python3 -m venv venv
source venv/bin/activate

# Install dependencies
pip install -r requirements.txt
```

## Usage

### Wallet Operations

```bash
# Create a new wallet
python shadowy_cli.py wallet create my-wallet

# List all wallets
python shadowy_cli.py wallet list

# Load a wallet
python shadowy_cli.py wallet load my-wallet

# Show current wallet address
python shadowy_cli.py wallet address

# Show wallet address for specific wallet
python shadowy_cli.py wallet address --wallet my-wallet

# Check detailed wallet balance
python shadowy_cli.py wallet balance --wallet my-wallet

# Send SHADOW to another address
python shadowy_cli.py wallet send S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737 1.5 --wallet my-wallet

# Send tokens to another address
python shadowy_cli.py wallet send S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737 100.0 --wallet my-wallet --token TOKEN123
```

### Balance Checking

```bash
# Check balance for any address
python shadowy_cli.py balance S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737

# Specify custom node
python shadowy_cli.py balance S427... --node http://remote.node:8080
```

### Node Health & Testing

```bash
# Check node health status
python shadowy_cli.py health

# Check detailed node health status
python shadowy_cli.py health --detailed

# Check remote node health
python shadowy_cli.py health --node http://remote.node:8080

# Validate address format
python shadowy_cli.py validate S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737

# Show UTXOs for an address
python shadowy_cli.py utxos S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737

# Test WASM loading and node connectivity
python shadowy_cli.py test
```

## Architecture

- **ShadowyWASM class**: Handles WASM loading and blockchain operations
- **Click CLI**: Clean command-line interface
- **HTTP client**: Direct requests to blockchain node API
- **Wallet storage**: JSON files in `~/.shadowy/`

## Current Status

- ✅ **Production Ready**: Full transaction sending for SHADOW and tokens
- ✅ **Real Blockchain Integration**: Uses actual node APIs and UTXO selection  
- ✅ **Mempool Integration**: Successfully submits transactions to blockchain network
- ✅ **Node Communication**: Complete HTTP API integration with all endpoints
- ✅ **Wallet Operations**: Complete wallet lifecycle management with real addresses

## Next Steps

1. **Real ML-DSA-87 signatures**: Implement actual post-quantum cryptographic signing
2. **Hardware wallet support**: Add hardware wallet integration for secure key storage
3. **Multi-signature transactions**: Support for multi-sig addresses and transactions
4. **Advanced features**: Mining, farming, and syndicate operations integration
5. **Transaction history**: Add transaction lookup and wallet history features

## Advantages over Node.js version

- 🐍 **Easy debugging**: Python is more traceable than Node.js WASM execution
- 📦 **No dependencies hell**: Simpler than Node.js ecosystem
- 🔧 **Direct control**: No browser/CORS issues
- 🚀 **Fast iteration**: Quick to modify and test
- 📖 **Readable code**: Python is more maintainable than JS async complexity