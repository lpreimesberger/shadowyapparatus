#!/bin/bash
# Consensus Engine API Endpoints Testing

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Consensus Engine Status ===" 
curl -s "$BASE_URL/api/v1/consensus" | jq '.'

echo -e "\n=== Peer Connections ==="
curl -s "$BASE_URL/api/v1/consensus/peers" | jq '.'

echo -e "\n=== Synchronization Status ==="
curl -s "$BASE_URL/api/v1/consensus/sync" | jq '.'

echo -e "\n=== Chain State ==="
curl -s "$BASE_URL/api/v1/consensus/chain" | jq '.'

echo -e "\n=== Node Health (with consensus) ==="
curl -s "$BASE_URL/api/v1/health" | jq '.services.consensus'

echo -e "\n=== Test Peer Connection ==="
echo "Attempting to connect to example peer (this will likely fail, but tests the endpoint)..."
curl -s -X POST "$BASE_URL/api/v1/consensus/peers/connect" \
    -H "Content-Type: application/json" \
    -d '{"address": "localhost:8889"}' | jq '.'

echo -e "\n=== Force Sync Test ==="
echo "Testing force sync endpoint..."
curl -s -X POST "$BASE_URL/api/v1/consensus/sync/force" | jq '.'

echo -e "\n=== Updated Peer Status ==="
curl -s "$BASE_URL/api/v1/consensus/peers" | jq '{
  peer_count: .peer_count,
  peers: .peers | to_entries | map({
    id: .key,
    address: .value.address,
    status: .value.status,
    chain_height: .value.chain_height,
    last_seen: .value.last_seen
  })
}'