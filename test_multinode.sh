#!/bin/bash
# Multi-node Consensus and Synchronization Test
# Tests blockchain sync between two nodes on the same machine

echo "=== Shadowy Multi-Node Consensus Test ==="
echo "Testing blockchain synchronization between two nodes"
echo

# Configuration
NODE1_HTTP_PORT=8080
NODE1_CONSENSUS_PORT=8888
NODE1_DATA_DIR="./node1_data"

NODE2_HTTP_PORT=8081
NODE2_CONSENSUS_PORT=8889
NODE2_DATA_DIR="./node2_data"

LOG_DIR="./multinode_logs"
TEST_DURATION=300  # 5 minutes

# Create directories
mkdir -p "$LOG_DIR"
mkdir -p "$NODE1_DATA_DIR"
mkdir -p "$NODE2_DATA_DIR"

# Kill any existing processes
echo "🧹 Cleaning up existing processes..."
pkill -f "shadowy node" 2>/dev/null || true
sleep 3

echo "📁 Setting up node data directories..."
echo "  Node 1: $NODE1_DATA_DIR (HTTP: $NODE1_HTTP_PORT, P2P: $NODE1_CONSENSUS_PORT)"
echo "  Node 2: $NODE2_DATA_DIR (HTTP: $NODE2_HTTP_PORT, P2P: $NODE2_CONSENSUS_PORT)"
echo

# Function to wait for node to be ready
wait_for_node() {
    local port=$1
    local name=$2
    local timeout=60
    local count=0
    
    echo "⏳ Waiting for $name to be ready on port $port..."
    while [ $count -lt $timeout ]; do
        if curl -s --connect-timeout 2 "http://localhost:$port/api/v1/health" >/dev/null 2>&1; then
            echo "✅ $name is ready"
            return 0
        fi
        sleep 1
        count=$((count + 1))
    done
    echo "❌ $name failed to start within $timeout seconds"
    return 1
}

# Start Node 1 (Primary node with existing blockchain)
echo "🚀 Starting Node 1 (Primary)..."
cd "$NODE1_DATA_DIR"
../shadowy node \
    --http-port="$NODE1_HTTP_PORT" \
    --consensus-port="$NODE1_CONSENSUS_PORT" \
    > "../$LOG_DIR/node1.log" 2>&1 &
NODE1_PID=$!
cd ..

# Wait for Node 1 to be ready
if ! wait_for_node $NODE1_HTTP_PORT "Node 1"; then
    echo "❌ Failed to start Node 1"
    kill $NODE1_PID 2>/dev/null
    exit 1
fi

# Let Node 1 mine a few blocks to establish a blockchain
echo "⛏️  Let Node 1 mine initial blocks..."
sleep 10

# Force some blocks to be mined on Node 1
echo "🎯 Forcing initial blocks on Node 1..."
for i in {1..3}; do
    curl -s -X POST "http://localhost:$NODE1_HTTP_PORT/api/v1/mining/force" >/dev/null
    sleep 5
done

# Check Node 1 initial state
echo "📊 Node 1 initial state:"
NODE1_INITIAL_STATE=$(curl -s "http://localhost:$NODE1_HTTP_PORT/api/v1/blockchain")
NODE1_INITIAL_HEIGHT=$(echo "$NODE1_INITIAL_STATE" | jq -r '.tip_height // 0')
NODE1_INITIAL_BLOCKS=$(echo "$NODE1_INITIAL_STATE" | jq -r '.total_blocks // 0')
echo "  • Height: $NODE1_INITIAL_HEIGHT"
echo "  • Total Blocks: $NODE1_INITIAL_BLOCKS"

# Start Node 2 (Secondary node that will sync)
echo
echo "🚀 Starting Node 2 (Secondary) with bootstrap peer..."
cd "$NODE2_DATA_DIR"
../shadowy node \
    --http-port="$NODE2_HTTP_PORT" \
    --consensus-port="$NODE2_CONSENSUS_PORT" \
    --bootstrap-peers="localhost:$NODE1_CONSENSUS_PORT" \
    > "../$LOG_DIR/node2.log" 2>&1 &
NODE2_PID=$!
cd ..

# Wait for Node 2 to be ready
if ! wait_for_node $NODE2_HTTP_PORT "Node 2"; then
    echo "❌ Failed to start Node 2"
    kill $NODE1_PID $NODE2_PID 2>/dev/null
    exit 1
fi

echo "✅ Both nodes started successfully"
echo "  • Node 1 PID: $NODE1_PID"
echo "  • Node 2 PID: $NODE2_PID"
echo

# Check initial peer connections
echo "🔗 Checking peer connections..."
sleep 5

NODE1_PEERS=$(curl -s "http://localhost:$NODE1_HTTP_PORT/api/v1/consensus/peers" | jq -r '.peer_count // 0')
NODE2_PEERS=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/consensus/peers" | jq -r '.peer_count // 0')

echo "  • Node 1 peers: $NODE1_PEERS"
echo "  • Node 2 peers: $NODE2_PEERS"

# Manual peer connection if bootstrap didn't work
if [ "$NODE2_PEERS" -eq 0 ]; then
    echo "🔗 Bootstrap didn't work, manually connecting Node 2 to Node 1..."
    curl -s -X POST "http://localhost:$NODE2_HTTP_PORT/api/v1/consensus/peers/connect" \
        -H "Content-Type: application/json" \
        -d "{\"address\": \"localhost:$NODE1_CONSENSUS_PORT\"}" | jq '.'
    sleep 5
    
    NODE2_PEERS=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/consensus/peers" | jq -r '.peer_count // 0')
    echo "  • Node 2 peers after manual connect: $NODE2_PEERS"
fi

# Check Node 2 initial state (should be genesis or very low)
echo
echo "📊 Node 2 initial state:"
NODE2_INITIAL_STATE=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/blockchain")
NODE2_INITIAL_HEIGHT=$(echo "$NODE2_INITIAL_STATE" | jq -r '.tip_height // 0')
NODE2_INITIAL_BLOCKS=$(echo "$NODE2_INITIAL_STATE" | jq -r '.total_blocks // 0')
echo "  • Height: $NODE2_INITIAL_HEIGHT"
echo "  • Total Blocks: $NODE2_INITIAL_BLOCKS"

# Force synchronization
echo
echo "🔄 Forcing synchronization on Node 2..."
curl -s -X POST "http://localhost:$NODE2_HTTP_PORT/api/v1/consensus/sync/force" | jq '.'

# Monitor synchronization progress
echo
echo "⏱️  Monitoring synchronization for up to 2 minutes..."
SYNC_START_TIME=$(date +%s)
SYNC_TIMEOUT=120

while true; do
    current_time=$(date +%s)
    elapsed=$((current_time - SYNC_START_TIME))
    
    if [ $elapsed -ge $SYNC_TIMEOUT ]; then
        echo "⏰ Sync monitoring timeout reached"
        break
    fi
    
    # Get current sync status
    NODE2_SYNC_STATUS=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/consensus/sync")
    NODE2_CURRENT_HEIGHT=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/blockchain" | jq -r '.tip_height // 0')
    
    NODE2_IS_SYNCING=$(echo "$NODE2_SYNC_STATUS" | jq -r '.is_syncing // false')
    NODE2_SYNC_PROGRESS=$(echo "$NODE2_SYNC_STATUS" | jq -r '.sync_progress // 0')
    
    echo "$(date): Node 2 height: $NODE2_CURRENT_HEIGHT, syncing: $NODE2_IS_SYNCING, progress: $NODE2_SYNC_PROGRESS"
    
    # Check if sync is complete
    if [ "$NODE2_CURRENT_HEIGHT" -ge "$NODE1_INITIAL_HEIGHT" ]; then
        echo "✅ Synchronization appears complete!"
        break
    fi
    
    sleep 10
done

# Test block propagation
echo
echo "🧪 Testing block propagation..."
echo "Mining new block on Node 1 and checking if it propagates to Node 2..."

# Get current heights
NODE1_PRE_HEIGHT=$(curl -s "http://localhost:$NODE1_HTTP_PORT/api/v1/blockchain" | jq -r '.tip_height')
NODE2_PRE_HEIGHT=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/blockchain" | jq -r '.tip_height')

echo "  • Node 1 height before: $NODE1_PRE_HEIGHT"
echo "  • Node 2 height before: $NODE2_PRE_HEIGHT"

# Mine a new block on Node 1
curl -s -X POST "http://localhost:$NODE1_HTTP_PORT/api/v1/mining/force" >/dev/null
sleep 5

# Check if block propagated
NODE1_POST_HEIGHT=$(curl -s "http://localhost:$NODE1_HTTP_PORT/api/v1/blockchain" | jq -r '.tip_height')
NODE2_POST_HEIGHT=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/blockchain" | jq -r '.tip_height')

echo "  • Node 1 height after: $NODE1_POST_HEIGHT"
echo "  • Node 2 height after: $NODE2_POST_HEIGHT"

# Test mempool synchronization
echo
echo "🧪 Testing mempool synchronization..."
echo "Submitting transaction to Node 1 and checking Node 2 mempool..."

# Create a test transaction (simplified)
# Note: This would need actual transaction creation logic
echo "  (Mempool sync test would require transaction creation - skipping for now)"

# Final status check
echo
echo "🏁 Final Multi-Node Test Results:"

# Get final states
NODE1_FINAL_STATE=$(curl -s "http://localhost:$NODE1_HTTP_PORT/api/v1/blockchain")
NODE2_FINAL_STATE=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/blockchain")

NODE1_FINAL_HEIGHT=$(echo "$NODE1_FINAL_STATE" | jq -r '.tip_height')
NODE1_FINAL_BLOCKS=$(echo "$NODE1_FINAL_STATE" | jq -r '.total_blocks')
NODE1_FINAL_HASH=$(echo "$NODE1_FINAL_STATE" | jq -r '.tip_hash')

NODE2_FINAL_HEIGHT=$(echo "$NODE2_FINAL_STATE" | jq -r '.tip_height')
NODE2_FINAL_BLOCKS=$(echo "$NODE2_FINAL_STATE" | jq -r '.total_blocks')
NODE2_FINAL_HASH=$(echo "$NODE2_FINAL_STATE" | jq -r '.tip_hash')

echo "Node 1 Final State:"
echo "  • Height: $NODE1_FINAL_HEIGHT"
echo "  • Total Blocks: $NODE1_FINAL_BLOCKS"
echo "  • Tip Hash: ${NODE1_FINAL_HASH:0:16}..."

echo "Node 2 Final State:"
echo "  • Height: $NODE2_FINAL_HEIGHT"
echo "  • Total Blocks: $NODE2_FINAL_BLOCKS"
echo "  • Tip Hash: ${NODE2_FINAL_HASH:0:16}..."

# Peer connection status
NODE1_FINAL_PEERS=$(curl -s "http://localhost:$NODE1_HTTP_PORT/api/v1/consensus/peers" | jq -r '.peer_count // 0')
NODE2_FINAL_PEERS=$(curl -s "http://localhost:$NODE2_HTTP_PORT/api/v1/consensus/peers" | jq -r '.peer_count // 0')

echo "Peer Connections:"
echo "  • Node 1 peers: $NODE1_FINAL_PEERS"
echo "  • Node 2 peers: $NODE2_FINAL_PEERS"

# Test results analysis
echo
echo "=== TEST ANALYSIS ==="

# Check if sync was successful
SYNC_SUCCESS=false
if [ "$NODE2_FINAL_HEIGHT" -ge "$NODE1_INITIAL_HEIGHT" ]; then
    SYNC_SUCCESS=true
fi

# Check if block propagation worked
PROPAGATION_SUCCESS=false
if [ "$NODE1_POST_HEIGHT" -gt "$NODE1_PRE_HEIGHT" ] && [ "$NODE2_POST_HEIGHT" -ge "$NODE1_POST_HEIGHT" ]; then
    PROPAGATION_SUCCESS=true
fi

# Check if peers are connected
PEER_CONNECTION_SUCCESS=false
if [ "$NODE1_FINAL_PEERS" -gt 0 ] && [ "$NODE2_FINAL_PEERS" -gt 0 ]; then
    PEER_CONNECTION_SUCCESS=true
fi

# Check if chains are consistent
CHAIN_CONSISTENCY=false
if [ "$NODE1_FINAL_HASH" = "$NODE2_FINAL_HASH" ] && [ "$NODE1_FINAL_HEIGHT" = "$NODE2_FINAL_HEIGHT" ]; then
    CHAIN_CONSISTENCY=true
fi

echo "Test Results:"
echo "  ✅ Peer Connection: $PEER_CONNECTION_SUCCESS"
echo "  ✅ Initial Sync: $SYNC_SUCCESS"
echo "  ✅ Block Propagation: $PROPAGATION_SUCCESS"
echo "  ✅ Chain Consistency: $CHAIN_CONSISTENCY"

# Overall verdict
if [ "$PEER_CONNECTION_SUCCESS" = true ] && [ "$SYNC_SUCCESS" = true ] && [ "$CHAIN_CONSISTENCY" = true ]; then
    echo
    echo "🎉 PASS: Multi-node consensus test successful!"
    echo "  • Nodes can connect to each other"
    echo "  • Blockchain synchronization works"
    echo "  • Chains remain consistent"
    OVERALL_RESULT=0
else
    echo
    echo "❌ FAIL: Multi-node consensus test failed"
    echo "  Check the individual test results above"
    OVERALL_RESULT=1
fi

# Cleanup
echo
echo "🛑 Stopping nodes..."
kill $NODE1_PID $NODE2_PID 2>/dev/null
sleep 3

# Force kill if still running
pkill -f "shadowy node" 2>/dev/null || true

echo "📁 Test artifacts saved in:"
echo "  • $LOG_DIR/node1.log - Node 1 logs"
echo "  • $LOG_DIR/node2.log - Node 2 logs"
echo "  • $NODE1_DATA_DIR/ - Node 1 blockchain data"
echo "  • $NODE2_DATA_DIR/ - Node 2 blockchain data"

echo
echo "🚀 Multi-node consensus test completed!"
echo "   Duration: $(date)"

exit $OVERALL_RESULT