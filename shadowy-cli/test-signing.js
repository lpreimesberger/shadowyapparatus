#!/usr/bin/env node

const fs = require('fs');
const path = require('path');

// Import Go's WASM runtime
require('../shadowy-wasm/wasm_exec.js');

async function testTransactionSigning() {
    console.log('🧪 Testing Post-Quantum Transaction Signing Workflow\n');
    
    // Set up HTTP bridge for WASM (since WASM can't do native HTTP in Node.js)
    const http = require('http');
    const https = require('https');
    const url = require('url');
    const crypto = require('crypto');
    
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
    
    console.log('🔧 Loading Shadowy WASM library...');
    
    const wasmPath = path.join(__dirname, '../shadowy-wasm/shadowy.wasm');
    const go = new Go();
    const wasmData = fs.readFileSync(wasmPath);
    const wasmModule = await WebAssembly.instantiate(wasmData, go.importObject);
    
    // Start the WASM module
    go.run(wasmModule.instance);
    
    // Wait for initialization
    await new Promise(resolve => setTimeout(resolve, 200));
    
    console.log('✅ WASM library loaded\n');
    
    try {
        // Step 1: Load existing post-quantum wallet
        console.log('🔓 Loading post-quantum wallet...');
        const loadResult = await shadowy_load_wallet('ml-dsa87-wallet');
        console.log(`✅ Wallet loaded: ${loadResult.address}\n`);
        
        // Step 2: Verify wallet is active
        console.log('📍 Checking wallet address...');
        const addressResult = shadowy_get_wallet_address();
        if (addressResult.error) {
            throw new Error('Wallet not properly loaded');
        }
        console.log(`✅ Active wallet: ${addressResult.address}\n`);
        
        // Step 3: Create and sign a transaction
        console.log('🔐 Creating transaction...');
        const transactionData = {
            to_address: 'S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737',
            amount: 150000000  // 1.5 SHADOW in satoshis
        };
        
        console.log(`   From: ${addressResult.address}`);
        console.log(`   To: ${transactionData.to_address}`);
        console.log(`   Amount: ${transactionData.amount / 100000000} SHADOW\n`);
        
        console.log('🔏 Signing transaction...');
        const signedTx = await shadowy_sign_transaction(transactionData);
        
        console.log('✅ Transaction signed successfully!\n');
        console.log('📋 Signed Transaction Details:');
        console.log(`   Transaction ID: ${signedTx.txid}`);
        console.log(`   Signature: ${signedTx.signatures[0].substring(0, 64)}...`);
        console.log(`   Raw Transaction Length: ${signedTx.raw_tx.length} bytes`);
        console.log(`   Raw Transaction: ${signedTx.raw_tx.substring(0, 128)}...`);
        console.log('');
        console.log('🎉 Post-quantum transaction signing workflow completed successfully!');
        console.log('💡 This demonstrates secure Dilithium Mode3 (ML-DSA87) signing with the WASM library');
        
    } catch (error) {
        console.error(`❌ Test failed: ${error.error || error.message}`);
        process.exit(1);
    }
    
    process.exit(0);
}

// Handle errors
process.on('unhandledRejection', (error) => {
    console.error('❌ Unhandled error:', error);
    process.exit(1);
});

testTransactionSigning().catch((error) => {
    console.error('❌ Test failed:', error);
    process.exit(1);
});