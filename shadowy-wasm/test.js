#!/usr/bin/env node

const fs = require('fs');
require('./wasm_exec.js'); // Go's WASM runtime

async function runShadowyWASM() {
    console.log('🚀 Testing Shadowy WASM Library...\n');
    
    // Load the WASM module
    const go = new Go();
    const wasmData = fs.readFileSync('./shadowy.wasm');
    const wasmModule = await WebAssembly.instantiate(wasmData, go.importObject);
    
    // Start the WASM module
    go.run(wasmModule.instance);
    
    // Wait a bit for initialization
    await new Promise(resolve => setTimeout(resolve, 100));
    
    console.log('📡 Creating client for local node...');
    
    // Test 1: Create client for local node
    let result = shadowy_create_client('http://localhost:8080');
    console.log('Client creation:', result);
    
    if (result.error) {
        console.log('❌ Failed to create client, trying different ports...');
        
        // Try different common ports
        const ports = [8081, 8082, 9090];
        let connected = false;
        
        for (const port of ports) {
            console.log(`🔍 Trying port ${port}...`);
            result = shadowy_create_client(`http://localhost:${port}`);
            if (result.success) {
                try {
                    const testResult = await shadowy_test_connection();
                    console.log('✅ Connected successfully on port', port);
                    connected = true;
                    break;
                } catch (error) {
                    console.log(`❌ Port ${port} not responding:`, error.error || error.message);
                    continue;
                }
            }
        }
        
        if (!connected) {
            console.log('⚠️  No local node found, testing with mock remote...');
            result = shadowy_create_client('https://api.shadowy.network'); // Mock URL
        }
    }
    
    // Test 2: Set API key (for remote nodes)
    console.log('\n🔐 Setting API key for remote access...');
    const apiResult = shadowy_set_api_key('sk-test-key-123456789abcdef');
    console.log('API key result:', apiResult);
    
    // Test 3: Test connection
    console.log('\n📡 Testing node connection...');
    try {
        const connectionTest = await shadowy_test_connection();
        console.log('✅ Node connection successful:');
        console.log('   Version:', connectionTest.node_info?.version || 'Unknown');
        console.log('   Height:', connectionTest.node_info?.chain_height || 0);
        console.log('   Chain ID:', connectionTest.node_info?.chain_id || 'Unknown');
    } catch (error) {
        console.log('❌ Connection test failed:', error.error || error.message);
        console.log('   This is expected if no node is running locally');
    }
    
    // Test 4: Get node info
    console.log('\n📊 Getting detailed node info...');
    try {
        const nodeInfo = await shadowy_get_node_info();
        console.log('Node information:');
        console.log('   Status:', nodeInfo.status || 'Unknown');
        console.log('   Chain Height:', nodeInfo.tip_height || 0);
        console.log('   Total Blocks:', nodeInfo.total_blocks || 0);
        console.log('   Total Transactions:', nodeInfo.total_transactions || 0);
    } catch (error) {
        console.log('❌ Failed to get node info:', error.error || error.message);
    }
    
    // Test 5: Get wallet balance (using a test address)
    console.log('\n💰 Testing wallet balance lookup...');
    const testAddresses = [
        'S427a724d41e3a5a03d1f83553134239813272bc2c4b2d50737', // Known test address
        'S123456789abcdef123456789abcdef123456789abcdef123', // Invalid address for testing
    ];
    
    for (const address of testAddresses) {
        console.log(`\n🔍 Checking balance for: ${address.substring(0, 20)}...`);
        
        try {
            const balance = await shadowy_get_balance(address);
            console.log('✅ Balance retrieved:');
            console.log(`   Address: ${balance.address?.substring(0, 20)}...`);
            console.log(`   Balance: ${balance.formatted_balance || '0.00000000 SHADOW'}`);
            console.log(`   Confirmed: ${balance.confirmed_balance_satoshi || 0} satoshi`);
            console.log(`   Pending: ${balance.pending_balance_satoshi || 0} satoshi`);
            console.log(`   Transactions: ${balance.transaction_count || 0}`);
            
            if (balance.last_activity) {
                console.log(`   Last Activity: ${balance.last_activity}`);
            }
        } catch (error) {
            console.log('❌ Balance lookup failed:', error.error || error.message);
        }
    }
    
    console.log('\n🎉 WASM library test completed!');
    console.log('\n💡 Next steps:');
    console.log('   1. Run a Shadowy node locally: go run . node');
    console.log('   2. Try the balance lookup with a real address');
    console.log('   3. Use this WASM in a browser or CLI application');
    
    process.exit(0);
}

// Handle unhandled promise rejections
process.on('unhandledRejection', (error) => {
    console.error('❌ Unhandled promise rejection:', error);
    process.exit(1);
});

// Run the test
runShadowyWASM().catch((error) => {
    console.error('❌ Failed to run WASM test:', error);
    process.exit(1);
});