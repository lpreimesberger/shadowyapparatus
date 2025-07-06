#!/bin/bash
# Blockchain API Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Blockchain Statistics ==="
curl -s "$BASE_URL/api/v1/blockchain" | jq '.'

echo -e "\n=== Get Tip Block ==="
curl -s "$BASE_URL/api/v1/blockchain/tip" | jq '{
  height: .header.height,
  hash: (.header | tostring | length),
  timestamp: .header.timestamp,
  previous_hash: .header.previous_block_hash,
  merkle_root: .header.merkle_root,
  tx_count: .body.tx_count
}'

echo -e "\n=== Get Genesis Block ==="
curl -s "$BASE_URL/api/v1/blockchain/block/height/0" | jq '{
  height: .header.height,
  timestamp: .header.timestamp,
  genesis_transaction: .body.transactions[0].tx_hash,
  initial_output_value: .body.transactions[0].transaction | fromjson | .outputs[0].value
}'

echo -e "\n=== Get Recent Blocks ==="
curl -s "$BASE_URL/api/v1/blockchain/recent?limit=3" | jq '{
  count: .count,
  blocks: .blocks | map({
    height: .header.height,
    timestamp: .header.timestamp,
    tx_count: .body.tx_count
  })
}'

echo -e "\n=== Blockchain Health ==="
curl -s "$BASE_URL/api/v1/health" | jq '.services.blockchain // "Blockchain service not found"'