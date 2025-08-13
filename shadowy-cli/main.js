#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

// Import Go's WASM runtime
require('../shadowy-wasm/wasm_exec.js');

class ShadowyCLI {
    constructor() {
        this.wasmLoaded = false;
    }
    
    async loadWASM() {
        if (this.wasmLoaded) return;
        
        console.log('🔧 Loading Shadowy WASM library...');
        
        // Set up HTTP bridge for WASM (since WASM can't do native HTTP in Node.js)
        this.setupHTTPBridge();
        
        const wasmPath = path.join(__dirname, '../shadowy-wasm/shadowy.wasm');
        const go = new Go();
        const wasmData = fs.readFileSync(wasmPath);
        const wasmModule = await WebAssembly.instantiate(wasmData, go.importObject);
        
        // Start the WASM module
        go.run(wasmModule.instance);
        
        // Wait for initialization
        await new Promise(resolve => setTimeout(resolve, 200));
        
        this.wasmLoaded = true;
        console.log('✅ WASM library loaded');
    }
    
    setupHTTPBridge() {
        const http = require('http');
        const https = require('https');
        const url = require('url');
        const crypto = require('crypto');
        const fs = require('fs');
        const path = require('path');
        
        // Create HTTP bridge for WASM to use
        global.shadowy_http_bridge = (requestData) => {
            return new Promise((resolve, reject) => {
                const options = url.parse(requestData.url);
                options.method = requestData.method || 'GET';
                options.headers = requestData.headers || {};
                
                const client = options.protocol === 'https:' ? https : http;
                
                const req = client.request(options, (res) => {
                    let body = '';
                    res.on('data', (chunk) => {
                        body += chunk;
                    });
                    res.on('end', () => {
                        resolve({
                            result: {
                                status_code: res.statusCode,
                                body: body,
                                headers: res.headers
                            }
                        });
                    });
                });
                
                req.on('error', (error) => {
                    reject({
                        error: error.message
                    });
                });
                
                if (requestData.body) {
                    req.write(requestData.body);
                }
                
                req.end();
            });
        };
        
        // Create crypto bridge for wallet operations
        global.shadowy_crypto_bridge = {
            // Generate a new private key using Node.js crypto
            generatePrivateKey: () => {
                const privateKey = crypto.randomBytes(32);
                return Array.from(privateKey); // Convert to array for WASM
            },
            
            // Read wallet file from $HOME/.shadowy
            readWalletFile: (filename) => {
                const shadowyDir = path.join(require('os').homedir(), '.shadowy');
                const walletPath = path.join(shadowyDir, filename);
                try {
                    const data = fs.readFileSync(walletPath, 'utf8');
                    return data;
                } catch (error) {
                    return null; // File doesn't exist
                }
            },
            
            // Write wallet file to $HOME/.shadowy
            writeWalletFile: (filename, data) => {
                const shadowyDir = path.join(require('os').homedir(), '.shadowy');
                
                // Create directory if it doesn't exist
                try {
                    if (!fs.existsSync(shadowyDir)) {
                        fs.mkdirSync(shadowyDir, { recursive: true });
                        console.log(`📁 Created wallet directory: ${shadowyDir}`);
                    }
                } catch (error) {
                    console.error('❌ Failed to create wallet directory:', error.message);
                    return false;
                }
                
                const walletPath = path.join(shadowyDir, filename);
                try {
                    fs.writeFileSync(walletPath, data, 'utf8');
                    return true;
                } catch (error) {
                    console.error('❌ Failed to write wallet file:', error.message);
                    return false;
                }
            },
            
            // Get secure random bytes
            getRandomBytes: (length) => {
                const bytes = crypto.randomBytes(length);
                return Array.from(bytes);
            }
        };
        
        console.log('🌉 HTTP bridge set up for WASM');
        console.log('🔐 Crypto bridge set up for WASM');
    }
    
    async detectNodeURL() {
        // Use IP addresses instead of localhost to avoid WASM DNS issues
        const testPorts = [8080, 8081, 8082, 9090];
        
        console.log('🔍 Auto-detecting local Shadowy node...');
        
        for (const port of testPorts) {
            const url = `http://127.0.0.1:${port}`;
            console.log(`   Trying ${url}...`);
            
            const result = shadowy_create_client(url);
            if (result.success) {
                try {
                    await shadowy_test_connection();
                    console.log(`✅ Found node at ${url}`);
                    return url;
                } catch (error) {
                    // Continue to next port
                    continue;
                }
            }
        }
        
        return null;
    }
    
    async initializeClient(nodeUrl, apiKey) {
        const result = shadowy_create_client(nodeUrl);
        if (!result.success) {
            throw new Error(result.error || 'Failed to create client');
        }
        
        if (apiKey) {
            const keyResult = shadowy_set_api_key(apiKey);
            if (!keyResult.success) {
                throw new Error(keyResult.error || 'Failed to set API key');
            }
        }
        
        // Test the connection
        try {
            await shadowy_test_connection();
            console.log('✅ Connected to Shadowy node');
        } catch (error) {
            throw new Error(`Connection failed: ${error.error || error.message}`);
        }
    }
    
    async getBalance(address) {
        if (!address) {
            throw new Error('Address is required');
        }
        
        try {
            const balance = await shadowy_get_balance(address);
            return balance;
        } catch (error) {
            throw new Error(`Balance lookup failed: ${error.error || error.message}`);
        }
    }
    
    async getNodeInfo() {
        try {
            const info = await shadowy_get_node_info();
            return info;
        } catch (error) {
            throw new Error(`Node info failed: ${error.error || error.message}`);
        }
    }
}

// Parse command line arguments for flags
function parseArgs(args) {
    const parsed = {
        positional: [],
        flags: {}
    };
    
    for (let i = 0; i < args.length; i++) {
        const arg = args[i];
        if (arg.startsWith('--')) {
            const flagName = arg.substring(2);
            const nextArg = args[i + 1];
            if (nextArg && !nextArg.startsWith('-')) {
                parsed.flags[flagName] = nextArg;
                i++; // Skip the next arg since it's the flag value
            } else {
                parsed.flags[flagName] = true;
            }
        } else if (arg.startsWith('-') && arg.length === 2) {
            // Handle single-letter flags
            const flagChar = arg.substring(1);
            const nextArg = args[i + 1];
            
            // Map single-letter flags to full names
            const flagMap = {
                's': 'source',
                'd': 'destination', 
                't': 'token',
                'a': 'amount',
                'f': 'fee',
                'h': 'help'
            };
            
            const fullFlagName = flagMap[flagChar] || flagChar;
            
            if (nextArg && !nextArg.startsWith('-')) {
                parsed.flags[fullFlagName] = nextArg;
                i++; // Skip the next arg since it's the flag value
            } else {
                parsed.flags[fullFlagName] = true;
            }
        } else {
            parsed.positional.push(arg);
        }
    }
    
    return parsed;
}

// CLI Command handlers
async function handleSend(args) {
    const parsed = parseArgs(args);
    
    // Handle help flag
    if (parsed.flags.help) {
        console.log(`
🚀 Shadowy Send Command

Usage:
  shadowy-cli send [flags]

Required Flags:
  -s, --source <wallet>        Source wallet name (must be loaded/created locally)
  -d, --destination <address>  Destination S-address or L-address
  -a, --amount <amount>        Amount to send (in token units, not satoshis)

Optional Flags:
  -t, --token <token_id>       Token ID (defaults to SHADOW)
  -f, --fee <fee>              Transaction fee in SHADOW (defaults to 0.011)
  --node <url>                 Node URL (default: auto-detect local)
  --api-key <key>              API key for remote nodes

Examples:
  shadowy-cli send -s my-wallet -d S427a724... -a 10.5
  shadowy-cli send --source test-wallet --destination L123... --amount 5.0 --token TOKEN1
  shadowy-cli send -s wallet1 -d S123... -a 100 -f 0.02
        `);
        return;
    }
    
    // Validate required flags
    const source = parsed.flags.source;
    const destination = parsed.flags.destination; 
    const amount = parsed.flags.amount;
    
    if (!source || !destination || !amount) {
        console.error('❌ Missing required flags. Use --help for usage information.');
        console.error('');
        console.error('Required: -s/--source, -d/--destination, -a/--amount');
        console.error('Example: shadowy-cli send -s my-wallet -d S427a724... -a 10.5');
        process.exit(1);
    }
    
    // Parse and validate amount
    const amountNum = parseFloat(amount);
    if (isNaN(amountNum) || amountNum <= 0) {
        console.error('❌ Invalid amount. Please specify a positive number.');
        process.exit(1);
    }
    
    // Set defaults
    const token = parsed.flags.token || 'SHADOW';
    const fee = parsed.flags.fee ? parseFloat(parsed.flags.fee) : 0.011;
    
    if (isNaN(fee) || fee < 0) {
        console.error('❌ Invalid fee. Please specify a non-negative number.');
        process.exit(1);
    }
    
    // Validate destination address format
    if (!isValidAddressFormat(destination)) {
        console.error('❌ Invalid destination address format.');
        console.error('   Must be S-address (51 chars) or L-address (41 chars)');
        process.exit(1);
    }
    
    console.log('💸 Preparing transaction:');
    console.log(`   From wallet: ${source}`);
    console.log(`   To address: ${destination}`);
    console.log(`   Amount: ${amountNum} ${token}`);
    console.log(`   Fee: ${fee} SHADOW`);
    console.log('');
    
    const cli = new ShadowyCLI();
    await cli.loadWASM();
    
    // Set up node connection
    let nodeUrl = parsed.flags.node;
    if (!nodeUrl) {
        nodeUrl = await cli.detectNodeURL();
        if (!nodeUrl) {
            console.log('⚠️  No local node detected, specify remote node with --node flag');
            process.exit(1);
        }
    }
    
    await cli.initializeClient(nodeUrl, parsed.flags['api-key']);
    
    try {
        // Load the source wallet
        console.log(`🔓 Loading wallet: ${source}`);
        await shadowy_load_wallet(source);
        
        // Get wallet info to confirm it's loaded
        const walletInfo = shadowy_get_wallet_address();
        if (walletInfo.error) {
            throw new Error(`Failed to load wallet: ${source}`);
        }
        
        console.log(`✅ Wallet loaded: ${walletInfo.address}`);
        
        // Convert amount to satoshis (assuming 8 decimal places like Bitcoin)
        const amountSatoshis = Math.round(amountNum * 100000000);
        const feeSatoshis = Math.round(fee * 100000000);
        
        console.log('🔏 Creating and signing transaction...');
        
        // Create transaction data
        const transactionData = {
            destination: destination,
            amount: amountSatoshis,
            token: token,
            fee: feeSatoshis,
            from_address: walletInfo.address
        };
        
        // Sign the transaction using WASM
        const signedTx = await shadowy_sign_transaction(transactionData);
        
        console.log('✅ Transaction signed successfully!');
        console.log('');
        console.log('📋 Transaction Details:');
        console.log(`   Transaction ID: ${signedTx.txid}`);
        console.log(`   Signature: ${signedTx.signature.substring(0, 32)}...`);
        console.log(`   Signature Length: ${signedTx.signature.length} chars (base64)`);
        console.log(`   Raw Transaction Size: ${signedTx.raw_tx.length} bytes`);
        
        // Calculate complete transaction size (JSON)
        const fullTxSize = JSON.stringify(signedTx).length;
        console.log(`   Complete Transaction Size: ${fullTxSize} bytes`);
        console.log('');
        console.log('📡 Broadcasting transaction to network...');
        
        try {
            // Broadcast the signed transaction
            const broadcastResult = await shadowy_broadcast_transaction(signedTx);
            
            console.log('✅ Transaction successfully broadcast to network!');
            console.log('');
            console.log('🌐 Network Response:');
            console.log(`   Status: ${broadcastResult.status}`);
            console.log(`   Message: ${broadcastResult.message}`);
            console.log(`   Network TX Hash: ${broadcastResult.tx_hash}`);
            console.log('');
            console.log('🎉 Transaction sent successfully! Check the blockchain explorer for confirmation.');
            
        } catch (broadcastError) {
            console.log('⚠️  Transaction signed but broadcast failed:');
            console.log(`   Error: ${broadcastError.error || broadcastError.message}`);
            console.log('');
            console.log('💡 Your transaction was signed successfully but could not be broadcast.');
            console.log('💡 This could mean the node is not available or there are network issues.');
            console.log('');
            console.log('🔄 You can try again later or check your node connection.');
        }
        
    } catch (error) {
        console.error(`❌ Send failed: ${error.error || error.message}`);
        process.exit(1);
    }
}

// Validate address format helper
function isValidAddressFormat(address) {
    if (!address || typeof address !== 'string') return false;
    
    // S-address: 51 characters starting with 'S'
    if (address.startsWith('S') && address.length === 51) {
        return /^S[0-9a-fA-F]{50}$/.test(address);
    }
    
    // L-address: 41 characters starting with 'L'  
    if (address.startsWith('L') && address.length === 41) {
        return /^L[0-9a-fA-F]{40}$/.test(address);
    }
    
    return false;
}

async function handleBalance(args) {
    const parsed = parseArgs(args);
    const address = parsed.positional[0];
    
    if (!address) {
        console.error('❌ Usage: shadowy-cli balance <address> [--node <url>] [--api-key <key>]');
        process.exit(1);
    }
    
    const cli = new ShadowyCLI();
    await cli.loadWASM();
    
    let nodeUrl = parsed.flags.node;
    
    // If no node specified, try to auto-detect
    if (!nodeUrl) {
        nodeUrl = await cli.detectNodeURL();
        if (!nodeUrl) {
            console.log('⚠️  No local node detected, specify remote node with --node flag');
            console.log('   Example: ./main.js balance <address> --node https://api.shadowy.network');
            process.exit(1);
        }
    }
    
    await cli.initializeClient(nodeUrl, parsed.flags['api-key']);
    
    console.log(`\n💰 Getting balance for address: ${address.substring(0, 20)}...`);
    
    const balance = await cli.getBalance(address);
    
    console.log('\n📊 Wallet Balance:');
    console.log(`   Address: ${balance.address}`);
    console.log(`   Balance: ${balance.balance.toFixed(8)} SHADOW`);
    
    // Use the correct field names from node API
    const confirmedSatoshi = balance.confirmed_satoshis || 0;
    const pendingSatoshi = balance.unconfirmed_satoshis || 0;
    const totalReceivedSatoshi = balance.total_received_satoshis || 0;
    const totalSentSatoshi = balance.total_sent_satoshis || 0;
    
    console.log(`   Confirmed: ${(confirmedSatoshi / 100000000).toFixed(8)} SHADOW`);
    console.log(`   Pending: ${(pendingSatoshi / 100000000).toFixed(8)} SHADOW`);
    console.log(`   Total Received: ${(totalReceivedSatoshi / 100000000).toFixed(8)} SHADOW`);
    console.log(`   Total Sent: ${(totalSentSatoshi / 100000000).toFixed(8)} SHADOW`);
    console.log(`   Transactions: ${balance.transaction_count || 0}`);
    if (balance.last_activity) {
        console.log(`   Last Activity: ${balance.last_activity}`);
    }
}

async function handleWallet(args) {
    const parsed = parseArgs(args);
    const action = parsed.positional[0];
    
    if (!action) {
        console.error('❌ Usage: shadowy-cli wallet <action> [arguments]');
        console.error('');
        console.error('Actions:');
        console.error('  create <name>    Create a new wallet');
        console.error('  load <name>      Load an existing wallet');
        console.error('  list             List all available wallets');
        console.error('  address          Show current wallet address');
        console.error('  sign <to> <amount>  Sign a transaction (requires loaded wallet)');
        process.exit(1);
    }
    
    const cli = new ShadowyCLI();
    await cli.loadWASM();
    
    try {
        switch (action) {
            case 'create':
                await handleWalletCreate(parsed.positional[1]);
                break;
            case 'load':
                await handleWalletLoad(parsed.positional[1]);
                break;
            case 'list':
                await handleWalletList();
                break;
            case 'address':
                await handleWalletAddress();
                break;
            case 'sign':
                await handleWalletSign(parsed.positional[1], parsed.positional[2]);
                break;
            default:
                console.error(`❌ Unknown wallet action: ${action}`);
                process.exit(1);
        }
    } catch (error) {
        console.error(`❌ Wallet error: ${error.message}`);
        process.exit(1);
    }
}

async function handleWalletCreate(walletName) {
    if (!walletName) {
        console.error('❌ Usage: shadowy-cli wallet create <name>');
        process.exit(1);
    }
    
    console.log(`🔑 Creating wallet: ${walletName}`);
    console.log('🔒 Generating secure Ed25519 key pair...');
    
    try {
        const result = await shadowy_create_wallet(walletName);
        console.log('✅ Wallet created successfully!');
        console.log('');
        console.log('📋 Wallet Details:');
        console.log(`   Name: ${result.name}`);
        console.log(`   Address: ${result.address}`);
        console.log(`   File: ~/.shadowy/shadowy-wallet-${walletName}.json`);
        console.log('');
        console.log('💡 Your wallet is saved in ~/.shadowy directory.');
        console.log('💰 Send SHADOW to this address to test transactions!');
        
        // Mark first todo as completed
        await updateTodoProgress(200, 'completed');
        
    } catch (error) {
        throw new Error(`Failed to create wallet: ${error.error || error.message}`);
    }
}

async function handleWalletLoad(walletName) {
    if (!walletName) {
        console.error('❌ Usage: shadowy-cli wallet load <name>');
        process.exit(1);
    }
    
    console.log(`🔓 Loading wallet: ${walletName}`);
    
    try {
        const result = await shadowy_load_wallet(walletName);
        console.log('✅ Wallet loaded successfully!');
        console.log('');
        console.log('📋 Wallet Details:');
        console.log(`   Name: ${result.name}`);
        console.log(`   Address: ${result.address}`);
        console.log(`   File: ~/.shadowy/shadowy-wallet-${walletName}.json`);
        
    } catch (error) {
        throw new Error(`Failed to load wallet: ${error.error || error.message}`);
    }
}

async function handleWalletList() {
    console.log('📋 Available wallets in ~/.shadowy:');
    console.log('');
    
    try {
        const os = require('os');
        const shadowyDir = path.join(os.homedir(), '.shadowy');
        
        if (!fs.existsSync(shadowyDir)) {
            console.log('⚠️  No wallet directory found.');
            console.log('💡 Create your first wallet: shadowy-cli wallet create <name>');
            return;
        }
        
        const files = fs.readdirSync(shadowyDir);
        const walletFiles = files.filter(file => file.startsWith('shadowy-wallet-') && file.endsWith('.json'));
        
        if (walletFiles.length === 0) {
            console.log('⚠️  No wallets found.');
            console.log('💡 Create your first wallet: shadowy-cli wallet create <name>');
            return;
        }
        
        console.log(`Found ${walletFiles.length} wallet(s):\n`);
        
        for (const walletFile of walletFiles) {
            try {
                const walletPath = path.join(shadowyDir, walletFile);
                const walletData = fs.readFileSync(walletPath, 'utf8');
                const wallet = JSON.parse(walletData);
                
                const walletName = walletFile.replace('shadowy-wallet-', '').replace('.json', '');
                const version = wallet.version === 3 ? 'Post-Quantum' : `v${wallet.version || 1}`;
                const createdDate = wallet.created_at ? new Date(wallet.created_at).toLocaleDateString() : 'Unknown';
                
                console.log(`📁 ${walletName}`);
                console.log(`   Address: ${wallet.address}`);
                console.log(`   Type: ${version}`);
                console.log(`   Created: ${createdDate}`);
                console.log('');
                
            } catch (error) {
                console.log(`❌ Error reading ${walletFile}: ${error.message}`);
            }
        }
        
        console.log('💡 Load a wallet: shadowy-cli wallet load <name>');
        
    } catch (error) {
        throw new Error(`Failed to list wallets: ${error.message}`);
    }
}

async function handleWalletAddress() {
    console.log('📍 Current wallet address:');
    
    try {
        const result = shadowy_get_wallet_address();
        if (result.error) {
            console.log('⚠️  No wallet loaded.');
            console.log('💡 Create a wallet: shadowy-cli wallet create <name>');
            console.log('💡 Load a wallet: shadowy-cli wallet load <name>');
            return;
        }
        
        console.log('');
        console.log(`   Name: ${result.name}`);
        console.log(`   Address: ${result.address}`);
        console.log('');
        console.log('💰 Send SHADOW to this address to test transactions!');
        
        // Mark todo as completed
        await updateTodoProgress(203, 'completed');
        
    } catch (error) {
        throw new Error(`Failed to get address: ${error.error || error.message}`);
    }
}

async function handleWalletSign(toAddress, amountStr) {
    if (!toAddress || !amountStr) {
        console.error('❌ Usage: shadowy-cli wallet sign <to_address> <amount>');
        console.error('   Example: shadowy-cli wallet sign S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737 1.5');
        process.exit(1);
    }
    
    const amount = parseFloat(amountStr);
    if (isNaN(amount) || amount <= 0) {
        console.error('❌ Invalid amount. Please specify a positive number.');
        process.exit(1);
    }
    
    // Convert SHADOW to satoshis (1 SHADOW = 100,000,000 satoshis)
    const amountSatoshis = Math.round(amount * 100000000);
    
    console.log(`🔐 Signing transaction:`);
    console.log(`   To: ${toAddress}`);
    console.log(`   Amount: ${amount} SHADOW (${amountSatoshis} satoshis)`);
    console.log('');
    
    try {
        // Check if wallet is loaded
        const walletCheck = shadowy_get_wallet_address();
        if (walletCheck.error) {
            console.error('❌ No wallet loaded. Load a wallet first:');
            console.error('   shadowy-cli wallet load <name>');
            process.exit(1);
        }
        
        console.log(`📝 From: ${walletCheck.address}`);
        console.log('🔏 Creating and signing transaction...');
        
        // Create transaction data
        const transactionData = {
            to_address: toAddress,
            amount: amountSatoshis
        };
        
        // Sign the transaction
        const signedTx = await shadowy_sign_transaction(transactionData);
        
        console.log('✅ Transaction signed successfully!');
        console.log('');
        console.log('📋 Signed Transaction Details:');
        console.log(`   Transaction ID: ${signedTx.txid}`);
        console.log(`   Signature: ${signedTx.signature.substring(0, 32)}...`);
        console.log(`   Raw Transaction: ${signedTx.raw_tx.substring(0, 64)}...`);
        console.log('');
        console.log('💡 This signed transaction can now be broadcast to the network');
        console.log('💡 In a real implementation, you would submit this to a Shadowy node');
        
    } catch (error) {
        throw new Error(`Failed to sign transaction: ${error.error || error.message}`);
    }
}

// Helper function for todo updates (async stub)
async function updateTodoProgress(id, status) {
    // This is a stub for now - would integrate with todo system
    return;
}

async function handleNodeInfo() {
    const cli = new ShadowyCLI();
    await cli.loadWASM();
    
    let nodeUrl = await cli.detectNodeURL();
    if (!nodeUrl) {
        console.log('⚠️  No local node detected, specify remote node with --node flag');
        process.exit(1);
    }
    
    await cli.initializeClient(nodeUrl);
    
    console.log('\n📊 Node Information:');
    const info = await cli.getNodeInfo();
    
    console.log(`   Chain Height: ${info.tip_height || 0}`);
    console.log(`   Total Blocks: ${info.total_blocks || 0}`);
    console.log(`   Total Transactions: ${info.total_transactions || 0}`);
    console.log(`   Status: ${info.status || 'Unknown'}`);
    if (info.version) {
        console.log(`   Version: ${info.version}`);
    }
}

async function handleHelp() {
    console.log(`
🌟 Shadowy CLI - Powered by WASM

Usage:
  shadowy-cli <command> [arguments] [flags]

Commands:
  balance <address>     Get balance for a Shadowy address
  send                  Send tokens (use --help for details)
  node                  Get node information
  wallet <action>       Wallet operations (create, load, address)
  help                  Show this help message

Flags:
  --node <url>          Specify remote node URL (default: auto-detect local)
  --api-key <key>       API key for remote node authentication

Examples:
  shadowy-cli wallet create my-wallet
  shadowy-cli wallet list
  shadowy-cli wallet load my-wallet
  shadowy-cli wallet address
  shadowy-cli send -s my-wallet -d S427a724... -a 10.5
  shadowy-cli send --source my-wallet --destination L123... --token TOKEN1 --amount 5.0 --fee 0.02
  shadowy-cli balance S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737
  shadowy-cli balance S123... --node https://api.shadowy.network --api-key sk-...
  shadowy-cli node

Features:
  ✅ Auto-detects local running nodes
  ✅ Supports remote nodes with API key auth  
  ✅ Uses WASM library for consistent behavior
  ✅ Same logic can be used in browser/web3 apps
  ✅ Post-quantum cryptography (Dilithium Mode3)
  ✅ Persistent wallet storage in ~/.shadowy
`);
}

// Main CLI entry point
async function main() {
    const args = process.argv.slice(2);
    
    if (args.length === 0) {
        await handleHelp();
        return;
    }
    
    const command = args[0];
    const commandArgs = args.slice(1);
    
    try {
        switch (command) {
            case 'balance':
                await handleBalance(commandArgs);
                break;
            case 'send':
                await handleSend(commandArgs);
                break;
            case 'wallet':
                await handleWallet(commandArgs);
                break;
            case 'node':
                await handleNodeInfo();
                break;
            case 'help':
            case '--help':
            case '-h':
                await handleHelp();
                break;
            default:
                console.error(`❌ Unknown command: ${command}`);
                console.error('Run "shadowy-cli help" for usage information');
                process.exit(1);
        }
    } catch (error) {
        console.error(`❌ Error: ${error.message}`);
        process.exit(1);
    }
}

// Handle unhandled promise rejections
process.on('unhandledRejection', (error) => {
    console.error('❌ Unhandled error:', error.message);
    process.exit(1);
});

// Run the CLI
main();