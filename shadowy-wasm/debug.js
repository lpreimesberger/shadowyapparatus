#!/usr/bin/env node

const fs = require('fs');
require('./wasm_exec.js');

async function debugWASM() {
    console.log('🔬 Debugging WASM HTTP issues...\n');
    
    // Load the WASM module
    const go = new Go();
    const wasmData = fs.readFileSync('./shadowy.wasm');
    const wasmModule = await WebAssembly.instantiate(wasmData, go.importObject);
    
    // Start the WASM module
    go.run(wasmModule.instance);
    
    // Wait for initialization
    await new Promise(resolve => setTimeout(resolve, 200));
    
    const testURLs = [
        'http://127.0.0.1:8080',
        'http://localhost:8080',
    ];
    
    for (const baseURL of testURLs) {
        console.log(`🧪 Testing with base URL: ${baseURL}`);
        
        // Create client
        const createResult = shadowy_create_client(baseURL);
        console.log(`   Client creation:`, createResult);
        
        if (createResult.success) {
            // Try balance lookup directly
            console.log(`   Testing balance endpoint directly...`);
            try {
                const balance = await shadowy_get_balance('S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737');
                console.log(`   ✅ Balance lookup successful!`);
                console.log(`   Balance: ${balance.formatted_balance}`);
                console.log(`   Address: ${balance.address}`);
                break; // Success, stop testing
            } catch (error) {
                console.log(`   ❌ Balance lookup failed:`, error.error || error.message);
            }
        }
        console.log('');
    }
    
    process.exit(0);
}

// Handle errors
process.on('unhandledRejection', (error) => {
    console.error('❌ Unhandled error:', error);
    process.exit(1);
});

debugWASM().catch((error) => {
    console.error('❌ Debug failed:', error);
    process.exit(1);
});