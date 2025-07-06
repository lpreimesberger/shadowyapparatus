#!/bin/bash
# Mining Service API Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Mining Service Status ==="
curl -s "$BASE_URL/api/v1/mining/status" | jq '.'

echo -e "\n=== Mining Statistics ==="
curl -s "$BASE_URL/api/v1/mining" | jq '.'

echo -e "\n=== Current Mining Address ==="
curl -s "$BASE_URL/api/v1/mining/address" | jq '.'

echo -e "\n=== Blockchain State ==="
curl -s "$BASE_URL/api/v1/blockchain" | jq '{
  tip_height: .tip_height,
  tip_hash: .tip_hash[0:16],
  total_blocks: .total_blocks,
  last_block_time: .last_block_time
}'

echo -e "\n=== Force Block Generation ==="
echo "Forcing block generation..."
FORCE_RESULT=$(curl -s -X POST "$BASE_URL/api/v1/mining/force")
echo "$FORCE_RESULT" | jq '.'

if [ "$(echo "$FORCE_RESULT" | jq -r '.status')" = "success" ]; then
  echo -e "\n⏳ Waiting for block to be generated..."
  sleep 3
  
  echo -e "\n=== Updated Blockchain State ==="
  curl -s "$BASE_URL/api/v1/blockchain" | jq '{
    tip_height: .tip_height,
    tip_hash: .tip_hash[0:16],
    total_blocks: .total_blocks,
    last_block_time: .last_block_time
  }'
  
  echo -e "\n=== Updated Mining Stats ==="
  curl -s "$BASE_URL/api/v1/mining" | jq '{
    blocks_mined: .blocks_mined,
    total_rewards: .total_rewards,
    total_rewards_shadow: (.total_rewards / 100000000),
    avg_block_time: .avg_block_time,
    proof_success_rate: .proof_success_rate
  }'
  
  echo -e "\n=== Latest Block Details ==="
  curl -s "$BASE_URL/api/v1/blockchain/tip" | jq '{
    height: .header.height,
    timestamp: .header.timestamp,
    farmer_address: .header.farmer_address,
    tx_count: .body.tx_count,
    coinbase_output: .body.transactions[0].transaction | fromjson | .outputs[0].value,
    coinbase_shadow: ((.body.transactions[0].transaction | fromjson | .outputs[0].value) / 100000000)
  }'
fi

echo -e "\n=== Block Reward Analysis ==="
echo "Current height reward:"
TIP_HEIGHT=$(curl -s "$BASE_URL/api/v1/blockchain" | jq -r '.tip_height // 0')
curl -s "$BASE_URL/api/v1/tokenomics/reward/$TIP_HEIGHT" | jq '{
  height: .height,
  reward_shadow: .reward_shadow,
  halving_era: .halving_era
}'

echo -e "\nNext height reward:"
NEXT_HEIGHT=$((TIP_HEIGHT + 1))
curl -s "$BASE_URL/api/v1/tokenomics/reward/$NEXT_HEIGHT" | jq '{
  height: .height,
  reward_shadow: .reward_shadow,
  halving_era: .halving_era
}'

echo -e "\n=== Mining Efficiency ==="
echo "🎯 Solo Mining Status:"
echo "• Mining Address: $(curl -s "$BASE_URL/api/v1/mining/address" | jq -r '.mining_address')"
echo "• Network Difficulty: Proof-of-Storage (plot-based)"
echo "• Block Time Target: 10 minutes"
echo "• Current Reward: $(curl -s "$BASE_URL/api/v1/tokenomics/reward/$NEXT_HEIGHT" | jq -r '.reward_shadow') SHADOW per block"
echo
echo "💰 Earnings (if mining alone):"
echo "• Per Block: $(curl -s "$BASE_URL/api/v1/tokenomics/reward/$NEXT_HEIGHT" | jq -r '.reward_shadow') SHADOW"
echo "• Per Hour: $(echo "$(curl -s "$BASE_URL/api/v1/tokenomics/reward/$NEXT_HEIGHT" | jq -r '.reward_shadow') * 6" | bc -l) SHADOW (6 blocks/hour)"
echo "• Per Day: $(echo "$(curl -s "$BASE_URL/api/v1/tokenomics/reward/$NEXT_HEIGHT" | jq -r '.reward_shadow') * 144" | bc -l) SHADOW (144 blocks/day)"