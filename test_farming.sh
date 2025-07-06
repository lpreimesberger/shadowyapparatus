#!/bin/bash

echo "=== Shadowy Farming Service Test ==="
echo

# Start the node in background
echo "Starting Shadowy node..."
./shadowy node > node.log 2>&1 &
NODE_PID=$!

# Wait for node to start and complete indexing
echo "Waiting for farming service to start and index plots..."
sleep 10

echo "=== Testing Farming API ==="

echo "1. Health check:"
curl -s http://localhost:8080/api/v1/health | jq '.services.farming // "Service not ready"'
echo

echo "2. Farming status:"
curl -s http://localhost:8080/api/v1/farming/status | jq '{running: .running, plot_files: .stats.plot_files_indexed, total_keys: .stats.total_keys}'
echo

echo "3. Plot files list:"
curl -s http://localhost:8080/api/v1/farming/plots | jq '{count: .count, files: [.plots[] | {name: .file_name, keys: .key_count, size: .file_size}]}'
echo

echo "4. Submitting test challenge:"
CHALLENGE_RESPONSE=$(curl -s -X POST http://localhost:8080/api/v1/farming/challenge \
  -H "Content-Type: application/json" \
  -d '{"challenge": "dGVzdCBjaGFsbGVuZ2UgZGF0YQ==", "difficulty": 1}')

echo "$CHALLENGE_RESPONSE" | jq '{challenge_id: .challenge_id, valid: .valid, response_time_ns: .response_time}'
echo

echo "5. Updated farming stats after challenge:"
curl -s http://localhost:8080/api/v1/farming | jq '{challenges_handled: .challenges_handled, avg_response_time: .average_response_time, error_count: .error_count}'
echo

echo "=== Test Complete ==="
echo "Node log output:"
tail -5 node.log
echo

# Cleanup
echo "Stopping node..."
kill $NODE_PID 2>/dev/null
wait $NODE_PID 2>/dev/null
rm -f node.log

echo "✓ Farming service interaction test completed successfully!"
echo
echo "To interact with farming service manually:"
echo "  1. Start node: ./shadowy node"
echo "  2. Check status: curl http://localhost:8080/api/v1/farming/status"
echo "  3. List plots: curl http://localhost:8080/api/v1/farming/plots"
echo "  4. Submit challenge: curl -X POST http://localhost:8080/api/v1/farming/challenge -H 'Content-Type: application/json' -d '{\"challenge\": \"dGVzdA==\"}'"