#!/bin/bash

echo "=== Shadowy Blockchain Test ==="
echo

# Kill any existing shadowy processes
pkill -f "shadowy node" 2>/dev/null || true
sleep 2

# Start the node in background
echo "Starting Shadowy node..."
./shadowy node > blockchain_test.log 2>&1 &
NODE_PID=$!

# Wait for node to fully initialize
echo "Waiting for node to initialize..."
sleep 15

# Test blockchain endpoints
echo "=== Testing Blockchain API ==="

echo "1. Blockchain Statistics:"
curl -s http://localhost:8080/api/v1/blockchain | jq '.' || echo "Failed to get blockchain stats"
echo

echo "2. Node Health (Blockchain Service):"
curl -s http://localhost:8080/api/v1/health | jq '.services.blockchain' || echo "Failed to get health"
echo

echo "3. Genesis Block:"
curl -s http://localhost:8080/api/v1/blockchain/block/height/0 | jq '.header | {height, timestamp, previous_block_hash}' || echo "Failed to get genesis"
echo

echo "4. Tip Block:"
curl -s http://localhost:8080/api/v1/blockchain/tip | jq '.header | {height, timestamp}' || echo "Failed to get tip"
echo

echo "=== Test Complete ==="
echo "Node log output:"
tail -10 blockchain_test.log
echo

# Cleanup
echo "Stopping node..."
kill $NODE_PID 2>/dev/null
wait $NODE_PID 2>/dev/null
rm -f blockchain_test.log

echo "✓ Blockchain test completed!"
echo
echo "Blockchain directory contents:"
ls -la ./blockchain/