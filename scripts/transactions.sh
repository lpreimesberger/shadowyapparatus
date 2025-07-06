#!/bin/bash
# Transaction Utility Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Create Transaction ==="
# Create a test transaction
TX_REQUEST='{
  "inputs": [],
  "outputs": [
    {
      "value": 100,
      "address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
    },
    {
      "value": 50,
      "address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
    }
  ],
  "not_until": "'$(date -u +%Y-%m-%dT%H:%M:%SZ)'"
}'

echo "Creating transaction..."
CREATE_RESULT=$(curl -s -X POST "$BASE_URL/api/v1/utils/transaction/create" \
  -H "Content-Type: application/json" \
  -d "$TX_REQUEST")

echo "$CREATE_RESULT" | jq '.'

# Extract transaction and hash for signing
TRANSACTION=$(echo "$CREATE_RESULT" | jq '.transaction')
TX_HASH=$(echo "$CREATE_RESULT" | jq -r '.hash')

if [ ! -z "$TX_HASH" ] && [ "$TX_HASH" != "null" ]; then
  echo -e "\n=== Sign Transaction ==="
  
  # Check if we have any wallets
  WALLETS=$(curl -s "$BASE_URL/api/v1/wallet" | jq -r '.wallets[]?.name // empty')
  
  if [ ! -z "$WALLETS" ]; then
    # Use the first available wallet
    WALLET_NAME=$(echo "$WALLETS" | head -1)
    echo "Signing with wallet: $WALLET_NAME"
    
    SIGN_REQUEST='{
      "transaction": '$TRANSACTION',
      "wallet_name": "'$WALLET_NAME'"
    }'
    
    SIGN_RESULT=$(curl -s -X POST "$BASE_URL/api/v1/utils/transaction/sign" \
      -H "Content-Type: application/json" \
      -d "$SIGN_REQUEST")
    
    echo "$SIGN_RESULT" | jq '.'
    
    echo -e "\n=== Submit Signed Transaction to Mempool ==="
    # Submit the signed transaction to mempool
    curl -s -X POST "$BASE_URL/api/v1/mempool/transactions" \
      -H "Content-Type: application/json" \
      -d "$SIGN_RESULT" | jq '.'
      
  else
    echo "No wallets available for signing. Create one with: ./shadowy wallet create test-wallet"
  fi
else
  echo "Transaction creation failed"
fi

echo -e "\n=== Complex Transaction Example ==="
# Create a more complex transaction with multiple inputs/outputs
COMPLEX_TX='{
  "inputs": [
    {
      "tx_hash": "previous_tx_hash_1",
      "output_index": 0,
      "unlock_script": "test_unlock_script"
    }
  ],
  "outputs": [
    {
      "value": 1000,
      "address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
    },
    {
      "value": 500,
      "address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
    },
    {
      "value": 250,
      "address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"
    }
  ],
  "not_until": "'$(date -u -d "+1 hour" +%Y-%m-%dT%H:%M:%SZ)'"
}'

echo "Creating complex transaction..."
curl -s -X POST "$BASE_URL/api/v1/utils/transaction/create" \
  -H "Content-Type: application/json" \
  -d "$COMPLEX_TX" | jq '{
  hash: .hash,
  inputs: .transaction.inputs | length,
  outputs: .transaction.outputs | length,
  total_output_value: (.transaction.outputs | map(.value) | add),
  not_until: .transaction.not_until
}'