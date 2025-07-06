#!/bin/bash
# Health and Status Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Health Check ==="
curl -s "$BASE_URL/api/v1/health" | jq '.'

echo -e "\n=== Node Status ==="
curl -s "$BASE_URL/api/v1/status" | jq '.'

echo -e "\n=== Service Health Summary ==="
curl -s "$BASE_URL/api/v1/health" | jq '{
  overall_healthy: .healthy,
  services: .services | to_entries | map({
    name: .key,
    status: .value.status,
    last_check: .value.last_check
  })
}'