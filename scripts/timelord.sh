#!/bin/bash
# Timelord/VDF Endpoints

BASE_URL="${SHADOWY_API_URL:-http://localhost:8080}"

echo "=== Timelord Statistics ==="
curl -s "$BASE_URL/api/v1/timelord" | jq '.'

echo -e "\n=== Submit VDF Job ==="
# Submit a test VDF job
VDF_JOB='{
  "data": "'$(echo -n "test vdf challenge $(date)" | base64)'",
  "priority": 5
}'

echo "Submitting VDF job..."
SUBMIT_RESULT=$(curl -s -X POST "$BASE_URL/api/v1/timelord/jobs" \
  -H "Content-Type: application/json" \
  -d "$VDF_JOB")

echo "$SUBMIT_RESULT" | jq '.'

# Get the job ID if submission was successful
JOB_ID=$(echo "$SUBMIT_RESULT" | jq -r '.job_id // empty')

if [ ! -z "$JOB_ID" ]; then
  echo -e "\n=== Get VDF Job Status ==="
  # Check job status immediately
  curl -s "$BASE_URL/api/v1/timelord/jobs/$JOB_ID" | jq '.'
  
  echo -e "\n=== Waiting for job completion (10 seconds) ==="
  sleep 10
  
  echo "Final job status:"
  curl -s "$BASE_URL/api/v1/timelord/jobs/$JOB_ID" | jq '.'
fi

echo -e "\n=== Updated Timelord Stats ==="
curl -s "$BASE_URL/api/v1/timelord" | jq '{
  total_jobs: .total_jobs,
  completed_jobs: .completed_jobs,
  pending_jobs: .pending_jobs,
  failed_jobs: .failed_jobs,
  average_proof_time: .average_proof_time
}'