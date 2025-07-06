#!/bin/bash
# Farming Service Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Farming Statistics ==="
curl -s "$BASE_URL/api/v1/farming" | jq '.'

echo -e "\n=== Farming Status ==="
curl -s "$BASE_URL/api/v1/farming/status" | jq '.'

echo -e "\n=== List Plot Files ==="
curl -s "$BASE_URL/api/v1/farming/plots" | jq '.'

echo -e "\n=== Plot Files Summary ==="
curl -s "$BASE_URL/api/v1/farming/plots" | jq '{
  total_plots: .count,
  plots: .plots | map({
    name: .file_name,
    keys: .key_count,
    size_mb: (.file_size / 1024 / 1024 | floor),
    modified: .mod_time
  })
}'

echo -e "\n=== Submit Storage Challenge ==="
# Create test challenges with different data
CHALLENGES=(
  "$(echo -n "test challenge 1" | base64)"
  "$(echo -n "storage proof test" | base64)"
  "$(echo -n "random data: $(date +%s)" | base64)"
)

for i in "${!CHALLENGES[@]}"; do
  echo "Challenge $((i+1)):"
  CHALLENGE_DATA='{
    "challenge": "'${CHALLENGES[$i]}'",
    "difficulty": '$((i+1))'
  }'
  
  curl -s -X POST "$BASE_URL/api/v1/farming/challenge" \
    -H "Content-Type: application/json" \
    -d "$CHALLENGE_DATA" | jq '{
    challenge_id: .challenge_id,
    valid: .valid,
    response_time_ns: .response_time,
    error: .error
  }'
  
  # Small delay between challenges
  sleep 1
done

echo -e "\n=== Updated Farming Stats After Challenges ==="
curl -s "$BASE_URL/api/v1/farming" | jq '{
  challenges_handled: .challenges_handled,
  average_response_time: .average_response_time,
  error_count: .error_count,
  database_size_mb: (.database_size / 1024 / 1024 | floor)
}'