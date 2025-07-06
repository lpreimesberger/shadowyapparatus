#!/bin/bash
# Comprehensive API Test Suite

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Shadowy API Test Suite ==="
echo "Base URL: $BASE_URL"
echo "Scripts Directory: $SCRIPT_DIR"
echo

# Function to check if node is running
check_node() {
  curl -s --connect-timeout 2 "$BASE_URL/api/v1/health" >/dev/null 2>&1
  return $?
}

# Function to run a test script
run_test() {
  local script="$1"
  local name="$2"
  
  echo "=== $name ==="
  if [ -f "$SCRIPT_DIR/$script" ]; then
    chmod +x "$SCRIPT_DIR/$script"
    "$SCRIPT_DIR/$script"
    echo
  else
    echo "❌ Script $script not found"
    echo
  fi
}

# Check if node is running
echo "Checking node availability..."
if ! check_node; then
  echo "❌ Node not responding at $BASE_URL"
  echo "Please start the node first: ./shadowy node"
  exit 1
fi

echo "✅ Node is running"
echo

# Run all test scripts in order
run_test "health.sh" "Health & Status Tests"
run_test "blockchain.sh" "Blockchain Tests"
run_test "tokenomics.sh" "Tokenomics & Rewards Tests"
run_test "wallet.sh" "Wallet Management Tests"
run_test "mempool.sh" "Mempool Tests"
run_test "transactions.sh" "Transaction Utility Tests"

# Check if farming is enabled
if curl -s "$BASE_URL/api/v1/farming" >/dev/null 2>&1; then
  run_test "farming.sh" "Farming Service Tests"
else
  echo "=== Farming Service Tests ==="
  echo "⚠️  Farming service not available (not enabled or not ready)"
  echo
fi

# Check if mining is enabled
if curl -s "$BASE_URL/api/v1/mining" >/dev/null 2>&1; then
  run_test "mining.sh" "Mining Service Tests"
else
  echo "=== Mining Service Tests ==="
  echo "⚠️  Mining service not available (not enabled or not ready)"
  echo
fi

# Check if timelord is enabled
if curl -s "$BASE_URL/api/v1/timelord" >/dev/null 2>&1; then
  run_test "timelord.sh" "Timelord/VDF Tests"
else
  echo "=== Timelord/VDF Tests ==="
  echo "⚠️  Timelord service not available (not enabled)"
  echo "   Enable with: ./shadowy node --enable-timelord"
  echo
fi

echo "=== Test Summary ==="
echo "✅ All available API endpoints tested"
echo "📊 Final system state:"

curl -s "$BASE_URL/api/v1/health" | jq '{
  overall_healthy: .healthy,
  timestamp: .timestamp,
  services: .services | to_entries | map({
    name: .key,
    status: .value.status,
    last_check: .value.last_check
  })
}'

echo
echo "🔍 Available test scripts:"
echo "  health.sh      - Health and status endpoints"
echo "  wallet.sh      - Wallet management"
echo "  mempool.sh     - Mempool operations"
echo "  transactions.sh - Transaction utilities"
echo "  farming.sh     - Farming service (if enabled)"
echo "  timelord.sh    - Timelord/VDF (if enabled)"
echo "  monitor.sh     - Continuous monitoring"
echo "  stress_test.sh - Load testing"
echo
echo "🚀 Usage examples:"
echo "  ./scripts/monitor.sh                    # Monitor node continuously"
echo "  STRESS_REQUESTS=50 ./scripts/stress_test.sh  # Load test with 50 requests"
echo "  SHADOWY_API_URL=http://remote:8080 ./scripts/health.sh  # Test remote node"