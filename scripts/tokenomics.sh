#!/bin/bash
# Tokenomics and Rewards API Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Shadowy Tokenomics Overview ==="
curl -s "$BASE_URL/api/v1/tokenomics" | jq '.'

echo -e "\n=== Block Reward Examples ==="
echo "Current block (height 0):"
curl -s "$BASE_URL/api/v1/tokenomics/reward/0" | jq '.'

echo -e "\nBlock 100,000 (era 1):"
curl -s "$BASE_URL/api/v1/tokenomics/reward/100000" | jq '.'

echo -e "\nBlock 210,000 (first halving):"
curl -s "$BASE_URL/api/v1/tokenomics/reward/210000" | jq '.'

echo -e "\nBlock 420,000 (second halving):"
curl -s "$BASE_URL/api/v1/tokenomics/reward/420000" | jq '.'

echo -e "\n=== Reward Schedule ==="
curl -s "$BASE_URL/api/v1/tokenomics/schedule" | jq '{
  max_supply: .max_supply,
  initial_reward: .initial_reward,
  halving_interval: .halving_interval,
  first_5_eras: .schedule[0:5] | map({
    era: .era,
    reward_shadow: .reward_shadow,
    total_reward: (.total_reward_satoshi / 100000000),
    years: .years
  })
}'

echo -e "\n=== Supply Analysis ==="
echo "Genesis supply:"
curl -s "$BASE_URL/api/v1/tokenomics/supply/0" | jq '{
  height: .height,
  supply_shadow: .supply_shadow,
  percent_mined: .percent_mined
}'

echo -e "\nSupply after 1st era (210k blocks):"
curl -s "$BASE_URL/api/v1/tokenomics/supply/209999" | jq '{
  height: .height,
  supply_shadow: .supply_shadow,
  percent_mined: .percent_mined,
  inflation_rate: .inflation_rate
}'

echo -e "\nSupply after 2nd era (420k blocks):"
curl -s "$BASE_URL/api/v1/tokenomics/supply/419999" | jq '{
  height: .height,
  supply_shadow: .supply_shadow,
  percent_mined: .percent_mined,
  inflation_rate: .inflation_rate
}'

echo -e "\n=== Halving History ==="
curl -s "$BASE_URL/api/v1/tokenomics/halvings" | jq '{
  current_halving: .current_halving,
  halving_interval: .halving_interval,
  blocks_per_year: .blocks_per_year,
  halving_events: .halving_history[0:4] | map({
    halving: .halving_number,
    block_height: .block_height,
    reward_before: .reward_before,
    reward_after: .reward_after,
    estimated_year: .estimated_year
  })
}'

echo -e "\n=== Economic Projections ==="
echo "Key milestones:"
echo "• Era 1 (0-210k blocks): 50 SHADOW/block → 10.5M SHADOW total"
echo "• Era 2 (210k-420k): 25 SHADOW/block → 5.25M SHADOW added"  
echo "• Era 3 (420k-630k): 12.5 SHADOW/block → 2.625M SHADOW added"
echo "• Era 4 (630k-840k): 6.25 SHADOW/block → 1.3125M SHADOW added"
echo
echo "Total after 4 eras: ~19.7M SHADOW (93.75% of max supply)"
echo "Remaining 1.3M SHADOW mined over next ~124 years"
echo
echo "No premine! Fair launch! 🚀"