# Shadowy Web3 API

A modern TypeScript Web3 interface for the Shadowy post-quantum blockchain. Provides browser-based wallet functionality with secure key storage and WebAssembly integration for cryptographic operations.

## Features

- 🔐 **Post-Quantum Security**: Built for ML-DSA-87 (Dilithium Mode3) signatures
- 🌐 **Browser-Based**: Works entirely in the browser with secure key storage
- 📱 **Offline Capable**: Create wallets and sign transactions without a node connection
- 🔗 **Node Integration**: Connect to local or remote Shadowy nodes for blockchain operations
- 🛡️ **Secure Storage**: Browser sandboxed storage with encryption
- ⚡ **WASM Powered**: Uses WebAssembly for fast cryptographic operations
- 📦 **TypeScript**: Full type safety and modern development experience

## Quick Start

### Installation

```bash
npm install
npm run build
```

### Basic Usage

```typescript
import ShadowyWeb3 from 'shadowy-web3';

// Create instance connected to local node
const web3 = ShadowyWeb3.createLocal(8080);
await web3.initialize();

// Create a wallet
const wallet = await web3.createWallet({ name: 'my-wallet' });
console.log('Wallet created:', wallet.address);

// Get balance
const balance = await web3.getBalance();
console.log('Balance:', balance.balance, 'SHADOW');

// Send transaction
const result = await web3.sendTransaction({
  to: 'S1234567890abcdef...',
  amount: 1.5, // SHADOW
  fee: 0.011   // SHADOW
});
console.log('Transaction:', result.txHash);
```

### Offline Usage

```typescript
// Work without a node connection
const web3 = ShadowyWeb3.createOffline();
await web3.initialize();

// Create and manage wallets offline
const wallet = await web3.createWallet({ name: 'offline-wallet' });

// Sign transactions (broadcast when connected)
const signedTx = await web3.sendTransaction({
  to: 'S1234567890abcdef...',
  amount: 1.0
});
```

## API Reference

### ShadowyWeb3 Class

#### Constructor Options
```typescript
interface ShadowyWeb3Config {
  node?: {
    url: string;
    apiKey?: string;
    timeout?: number;
  };
  wasmUrl?: string;
  storage?: 'localStorage' | 'sessionStorage' | 'memory';
  network?: string;
}
```

#### Wallet Management
- `createWallet(options?)` - Create a new wallet
- `loadWallet(name)` - Load existing wallet
- `listWallets()` - List available wallets
- `deleteWallet(name)` - Delete a wallet
- `lockWallet()` - Clear wallet from memory
- `isWalletUnlocked()` - Check if wallet is loaded

#### Blockchain Operations
- `getBalance()` - Get current wallet balance
- `getAddressBalance(address)` - Get balance for any address
- `getUTXOs()` - Get wallet UTXOs
- `getAddressUTXOs(address)` - Get UTXOs for any address
- `sendTransaction(options)` - Send a transaction
- `validateAddress(address)` - Validate address format

#### Network Operations
- `getNodeInfo()` - Get blockchain status
- `testConnection()` - Test node connectivity
- `connectToNode(config)` - Connect to a node
- `disconnectFromNode()` - Work offline

### Address Formats

- **S-addresses**: 51 characters, format `S[0-9a-fA-F]{50}`
- **L-addresses**: 41 characters, format `L[0-9a-fA-F]{40}`

### Transaction Options
```typescript
interface SendTransactionOptions {
  to: string;           // Recipient address
  amount: number;       // Amount in SHADOW
  fee?: number;         // Fee in SHADOW (default: 0.011)
  token?: string;       // Token ID (default: 'SHADOW')
}
```

## Demo Application

Run the demo to see the API in action:

```bash
npm run dev
```

Open `http://localhost:8080/demo` in your browser.

The demo includes:
- Wallet creation and management
- Balance checking
- Transaction sending
- Node connection testing
- Offline mode demonstration

## Security Considerations

### Browser Storage
- Private keys are stored encrypted in browser storage
- Use `sessionStorage` for temporary wallets
- Use `memory` storage for maximum security (no persistence)

### WASM Security
- All cryptographic operations happen in WebAssembly
- Private keys never leave the browser sandbox
- Post-quantum signatures provide future-proof security

### Network Security
- All API calls use HTTPS in production
- Optional API key authentication
- Request timeout protection

## Development

### Building

```bash
npm run build        # Build for production
npm run build:dev    # Build for development
npm run dev          # Start development server
```

### Testing

```bash
npm test            # Run tests
npm run test:watch  # Run tests in watch mode
npm run coverage    # Generate coverage report
```

### Linting

```bash
npm run lint        # Check code style
npm run lint:fix    # Fix linting issues
```

## WebAssembly Integration

The API requires `shadowy.wasm` and `wasm_exec.js` files:

1. Build the Shadowy WASM module from the Go code
2. Place `shadowy.wasm` in your web server's public directory
3. Include `wasm_exec.js` before loading the Web3 library

Example HTML setup:
```html
<script src="wasm_exec.js"></script>
<script src="shadowy-web3.js"></script>
<script>
  const web3 = new ShadowyWeb3();
  web3.initialize().then(() => {
    console.log('Ready!');
  });
</script>
```

## Architecture

```
┌─────────────────────────────────────────────────────────────┐
│                    ShadowyWeb3 API                          │
├─────────────────────────────────────────────────────────────┤
│  Wallet Management  │  Network Provider  │  WASM Bridge    │
│  - Key Storage      │  - HTTP Client     │  - Crypto Ops   │
│  - Address Gen      │  - API Endpoints   │  - Tx Signing    │
│  - Transaction      │  - Balance/UTXOs   │  - Address Gen   │
├─────────────────────────────────────────────────────────────┤
│  Browser Storage    │  Fetch API         │  WebAssembly     │
│  - localStorage     │  - HTTP Requests   │  - Go Runtime    │
│  - sessionStorage   │  - JSON Parsing    │  - ML-DSA-87     │
│  - Encryption       │  - Error Handling  │  - UTXO Logic    │
└─────────────────────────────────────────────────────────────┘
```

## License

This project is part of the Shadowy blockchain ecosystem.