#!/bin/bash
# Wallet Management Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== List Wallets ==="
curl -s "$BASE_URL/api/v1/wallet" | jq '.'

echo -e "\n=== Wallet Details ==="
# Get list of wallets and show details for each
WALLETS=$(curl -s "$BASE_URL/api/v1/wallet" | jq -r '.wallets[]?.name // empty')

if [ ! -z "$WALLETS" ]; then
  echo "Found wallets:"
  for wallet in $WALLETS; do
    echo "--- Wallet: $wallet ---"
    curl -s "$BASE_URL/api/v1/wallet/$wallet" | jq '.'
    
    echo "Balance:"
    curl -s "$BASE_URL/api/v1/wallet/$wallet/balance" | jq '.'
    echo
  done
else
  echo "No wallets found. Create one with: ./shadowy wallet create <name>"
fi

echo -e "\n=== Validate Test Addresses ==="
TEST_ADDRESSES=(
  "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
  "invalid_address_test"
  "S" 
  ""
)

for addr in "${TEST_ADDRESSES[@]}"; do
  echo "Testing address: '$addr'"
  curl -s -X POST "$BASE_URL/api/v1/utils/validate-address" \
    -H "Content-Type: application/json" \
    -d '{"address": "'$addr'"}' | jq '{address: .address, valid: .valid}'
done