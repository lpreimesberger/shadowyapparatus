#!/bin/bash
# Quick 30-minute burn-in test to validate the full test script

# Override the test duration for quick validation
export TEST_DURATION_HOURS=0.5  # 30 minutes
export MONITOR_INTERVAL=60       # 1 minute intervals

echo "=== Quick Burn-in Test (30 minutes) ==="
echo "This is a validation run for the full 6-hour test"
echo

# Run the main burn-in test with modified parameters
exec ./burn_in_test.sh