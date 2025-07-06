#!/bin/bash
# Stress Testing Script

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"
NUM_REQUESTS="${STRESS_REQUESTS:-10}"
CONCURRENT="${STRESS_CONCURRENT:-3}"

echo "=== Shadowy API Stress Test ==="
echo "Requests: $NUM_REQUESTS per endpoint"
echo "Concurrent: $CONCURRENT requests"
echo "Base URL: $BASE_URL"
echo

# Function to run concurrent requests
run_concurrent() {
  local endpoint="$1"
  local method="$2"
  local data="$3"
  local name="$4"
  
  echo "Testing $name ($method $endpoint)..."
  
  local pids=()
  local start_time=$(date +%s.%N)
  
  for ((i=1; i<=NUM_REQUESTS; i++)); do
    (
      if [ "$method" = "POST" ]; then
        curl -s -X POST "$BASE_URL$endpoint" \
          -H "Content-Type: application/json" \
          -d "$data" >/dev/null 2>&1
      else
        curl -s "$BASE_URL$endpoint" >/dev/null 2>&1
      fi
      echo $? > /tmp/stress_result_$$_$i
    ) &
    
    pids+=($!)
    
    # Limit concurrent requests
    if [ $((i % CONCURRENT)) -eq 0 ]; then
      wait
    fi
  done
  
  # Wait for remaining requests
  wait
  
  local end_time=$(date +%s.%N)
  local duration=$(echo "$end_time - $start_time" | bc -l)
  
  # Count successes
  local success=0
  for ((i=1; i<=NUM_REQUESTS; i++)); do
    if [ -f "/tmp/stress_result_$$_$i" ]; then
      if [ "$(cat /tmp/stress_result_$$_$i)" = "0" ]; then
        ((success++))
      fi
      rm -f "/tmp/stress_result_$$_$i"
    fi
  done
  
  local rps=$(echo "scale=2; $NUM_REQUESTS / $duration" | bc -l)
  
  printf "  ✓ %d/%d successful (%.1f%%) in %.2fs (%.2f req/s)\n" \
    $success $NUM_REQUESTS $((success * 100 / NUM_REQUESTS)) $duration $rps
}

# Test health endpoints
run_concurrent "/api/v1/health" "GET" "" "Health Check"
run_concurrent "/api/v1/status" "GET" "" "Node Status"

# Test mempool endpoints
run_concurrent "/api/v1/mempool" "GET" "" "Mempool Stats"
run_concurrent "/api/v1/mempool/transactions?limit=1" "GET" "" "List Transactions"

# Test farming endpoints (if available)
if curl -s "$BASE_URL/api/v1/farming" >/dev/null 2>&1; then
  run_concurrent "/api/v1/farming" "GET" "" "Farming Stats"
  run_concurrent "/api/v1/farming/status" "GET" "" "Farming Status"
  run_concurrent "/api/v1/farming/plots" "GET" "" "Plot List"
  
  # Test challenge submission
  CHALLENGE_DATA='{"challenge": "'$(echo -n "stress test" | base64)'", "difficulty": 1}'
  run_concurrent "/api/v1/farming/challenge" "POST" "$CHALLENGE_DATA" "Challenge Submission"
fi

# Test timelord endpoints (if available)
if curl -s "$BASE_URL/api/v1/timelord" >/dev/null 2>&1; then
  run_concurrent "/api/v1/timelord" "GET" "" "Timelord Stats"
  
  # Test VDF job submission
  VDF_DATA='{"data": "'$(echo -n "stress test vdf" | base64)'", "priority": 1}'
  run_concurrent "/api/v1/timelord/jobs" "POST" "$VDF_DATA" "VDF Job Submission"
fi

# Test wallet endpoints
run_concurrent "/api/v1/wallet" "GET" "" "Wallet List"

# Test utility endpoints
ADDR_DATA='{"address": "S42618a7524a82df51c8a2406321e161de65073008806f042f0"}'
run_concurrent "/api/v1/utils/validate-address" "POST" "$ADDR_DATA" "Address Validation"

echo
echo "=== Final System State ==="
curl -s "$BASE_URL/api/v1/health" | jq '{
  healthy: .healthy,
  services: .services | to_entries | map({name: .key, status: .value.status})
}'

echo
echo "✓ Stress test completed!"