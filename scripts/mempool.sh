#!/bin/bash
# Mempool Management Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Mempool Statistics ==="
curl -s "$BASE_URL/api/v1/mempool" | jq '.'

echo -e "\n=== List Transactions (top 5) ==="
curl -s "$BASE_URL/api/v1/mempool/transactions?limit=5" | jq '.'

echo -e "\n=== Submit Test Transaction ==="
# Create a test transaction
TEST_TX='{
  "tx_hash": "test_'$(date +%s)'",
  "algorithm": "ML-DSA-87",
  "signature": "test_signature_data",
  "transaction": {
    "version": 1,
    "inputs": [],
    "outputs": [
      {
        "value": 100,
        "address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
      }
    ],
    "timestamp": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "not_until": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'",
    "nonce": '$(date +%s)'
  }
}'

echo "Submitting transaction..."
SUBMIT_RESULT=$(curl -s -X POST "$BASE_URL/api/v1/mempool/transactions" \
  -H "Content-Type: application/json" \
  -d "$TEST_TX")

echo "$SUBMIT_RESULT" | jq '.'

# Get the transaction hash if submission was successful
TX_HASH=$(echo "$SUBMIT_RESULT" | jq -r '.tx_hash // empty')

if [ ! -z "$TX_HASH" ]; then
  echo -e "\n=== Get Specific Transaction ==="
  curl -s "$BASE_URL/api/v1/mempool/transactions/$TX_HASH" | jq '.'
fi

echo -e "\n=== Updated Mempool Stats ==="
curl -s "$BASE_URL/api/v1/mempool" | jq '{
  transaction_count: .transaction_count,
  total_size: .total_size,
  average_fee: .average_fee,
  validation_stats: .validation_stats
}'