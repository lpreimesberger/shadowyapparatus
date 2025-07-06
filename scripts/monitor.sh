#!/bin/bash
# Continuous Monitoring Script

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"
INTERVAL="${MONITOR_INTERVAL:-5}"

echo "=== Shadowy Node Monitor ==="
echo "Monitoring every $INTERVAL seconds. Press Ctrl+C to stop."
echo "Base URL: $BASE_URL"
echo

# Function to get timestamp
timestamp() {
  date '+%Y-%m-%d %H:%M:%S'
}

# Function to check if node is running
check_node() {
  curl -s --connect-timeout 2 "$BASE_URL/api/v1/health" >/dev/null 2>&1
  return $?
}

# Main monitoring loop
while true; do
  echo "=== $(timestamp) ==="
  
  if ! check_node; then
    echo "❌ Node not responding at $BASE_URL"
    echo "   Try starting with: ./shadowy node"
    echo
    sleep $INTERVAL
    continue
  fi
  
  # Get overall health
  HEALTH=$(curl -s "$BASE_URL/api/v1/health")
  OVERALL_HEALTHY=$(echo "$HEALTH" | jq -r '.healthy')
  
  if [ "$OVERALL_HEALTHY" = "true" ]; then
    echo "✅ Node Status: HEALTHY"
  else
    echo "⚠️  Node Status: UNHEALTHY"
  fi
  
  # Service status summary
  echo "Services:"
  echo "$HEALTH" | jq -r '.services | to_entries[] | "  \(.key): \(.value.status)"'
  
  # Mempool stats
  MEMPOOL=$(curl -s "$BASE_URL/api/v1/mempool" 2>/dev/null)
  if [ $? -eq 0 ] && [ "$(echo "$MEMPOOL" | jq -r 'type')" = "object" ]; then
    echo "Mempool: $(echo "$MEMPOOL" | jq -r '.transaction_count // 0') transactions, $(echo "$MEMPOOL" | jq -r '.total_size // 0') bytes"
  fi
  
  # Farming stats (if enabled)
  FARMING=$(curl -s "$BASE_URL/api/v1/farming" 2>/dev/null)
  if [ $? -eq 0 ] && [ "$(echo "$FARMING" | jq -r 'type')" = "object" ]; then
    echo "Farming: $(echo "$FARMING" | jq -r '.plot_files_indexed // 0') plots, $(echo "$FARMING" | jq -r '.total_keys // 0') keys, $(echo "$FARMING" | jq -r '.challenges_handled // 0') challenges"
  fi
  
  # Timelord stats (if enabled)
  TIMELORD=$(curl -s "$BASE_URL/api/v1/timelord" 2>/dev/null)
  if [ $? -eq 0 ] && [ "$(echo "$TIMELORD" | jq -r 'type')" = "object" ]; then
    echo "Timelord: $(echo "$TIMELORD" | jq -r '.total_jobs // 0') total jobs, $(echo "$TIMELORD" | jq -r '.pending_jobs // 0') pending"
  fi
  
  echo
  sleep $INTERVAL
done