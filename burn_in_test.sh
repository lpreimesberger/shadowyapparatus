#!/bin/bash
# Extended Mining Burn-in Test - 6 Hour Validation
# Tests system stability, performance, and mining consistency over extended period

TEST_DURATION_HOURS=6
TEST_DURATION_SECONDS=$((TEST_DURATION_HOURS * 3600))
MONITOR_INTERVAL=300  # 5 minutes
LOG_DIR="./burn_in_logs"
START_TIME=$(date +%s)

echo "=== Shadowy Mining Burn-in Test ==="
echo "Duration: $TEST_DURATION_HOURS hours ($TEST_DURATION_SECONDS seconds)"
echo "Monitor Interval: $MONITOR_INTERVAL seconds"
echo "Start Time: $(date)"
echo "Expected End Time: $(date -d "+$TEST_DURATION_HOURS hours")"
echo

# Create log directory
mkdir -p "$LOG_DIR"

# Kill any existing processes
echo "🧹 Cleaning up any existing processes..."
pkill -f "shadowy node" 2>/dev/null || true
sleep 3

# Start the node with mining enabled
echo "🚀 Starting Shadowy node for burn-in test..."
./shadowy node > "$LOG_DIR/node.log" 2>&1 &
NODE_PID=$!

# Wait for initialization
echo "⏳ Waiting for node initialization (60 seconds)..."
sleep 60

# Verify node is responding
if ! curl -s --connect-timeout 5 http://localhost:8080/api/v1/health >/dev/null 2>&1; then
    echo "❌ Node failed to start properly"
    kill $NODE_PID 2>/dev/null
    exit 1
fi

echo "✅ Node started successfully (PID: $NODE_PID)"

# Initialize tracking variables
TOTAL_BLOCKS_START=$(curl -s http://localhost:8080/api/v1/blockchain | jq -r '.total_blocks // 0')
BLOCKS_MINED_START=$(curl -s http://localhost:8080/api/v1/mining | jq -r '.blocks_mined // 0')

echo "📊 Initial State:"
echo "  • Total Blocks: $TOTAL_BLOCKS_START"
echo "  • Blocks Mined: $BLOCKS_MINED_START"
echo

# Create monitoring log
MONITOR_LOG="$LOG_DIR/monitor.log"
STATS_LOG="$LOG_DIR/stats.csv"
ERROR_LOG="$LOG_DIR/errors.log"

# CSV header
echo "timestamp,elapsed_hours,total_blocks,blocks_mined,rewards_shadow,mempool_txs,proof_success_rate,avg_block_time,heap_mb,goroutines" > "$STATS_LOG"

# Monitoring function
monitor_system() {
    local current_time=$(date +%s)
    local elapsed=$((current_time - START_TIME))
    local elapsed_hours=$(echo "scale=2; $elapsed / 3600" | bc -l)
    
    echo "=== $(date) - Elapsed: ${elapsed_hours}h ===" | tee -a "$MONITOR_LOG"
    
    # Check if node is still running
    if ! kill -0 $NODE_PID 2>/dev/null; then
        echo "❌ ERROR: Node process died!" | tee -a "$ERROR_LOG"
        return 1
    fi
    
    # Check if API is responsive
    if ! curl -s --connect-timeout 10 http://localhost:8080/api/v1/health >/dev/null 2>&1; then
        echo "❌ ERROR: Node API not responding!" | tee -a "$ERROR_LOG"
        return 1
    fi
    
    # Collect comprehensive stats
    local blockchain_data=$(curl -s http://localhost:8080/api/v1/blockchain 2>/dev/null)
    local mining_data=$(curl -s http://localhost:8080/api/v1/mining 2>/dev/null)
    local mempool_data=$(curl -s http://localhost:8080/api/v1/mempool 2>/dev/null)
    local health_data=$(curl -s http://localhost:8080/api/v1/health 2>/dev/null)
    
    if [ -z "$blockchain_data" ] || [ -z "$mining_data" ]; then
        echo "❌ ERROR: Failed to collect system stats!" | tee -a "$ERROR_LOG"
        return 1
    fi
    
    # Parse stats
    local total_blocks=$(echo "$blockchain_data" | jq -r '.total_blocks // 0')
    local blocks_mined=$(echo "$mining_data" | jq -r '.blocks_mined // 0')
    local total_rewards=$(echo "$mining_data" | jq -r '.total_rewards // 0')
    local rewards_shadow=$(echo "scale=8; $total_rewards / 100000000" | bc -l)
    local mempool_txs=$(echo "$mempool_data" | jq -r '.transaction_count // 0')
    local proof_success_rate=$(echo "$mining_data" | jq -r '.proof_success_rate // 0')
    local avg_block_time=$(echo "$mining_data" | jq -r '.avg_block_time // "0s"')
    
    # System resource usage (approximate)
    local heap_mb=$(ps -o rss= -p $NODE_PID 2>/dev/null | awk '{print int($1/1024)}')
    local goroutines=$(curl -s http://localhost:8080/api/v1/health 2>/dev/null | jq -r '.system.goroutines // 0')
    
    # Calculate progress
    local blocks_since_start=$((total_blocks - TOTAL_BLOCKS_START))
    local mining_rate=$(echo "scale=2; $blocks_since_start / $elapsed_hours" | bc -l)
    
    echo "📊 System Status:"
    echo "  • Total Blocks: $total_blocks (+$blocks_since_start)"
    echo "  • Blocks Mined: $blocks_mined"
    echo "  • Total Rewards: $rewards_shadow SHADOW"
    echo "  • Mining Rate: $mining_rate blocks/hour"
    echo "  • Mempool Transactions: $mempool_txs"
    echo "  • Proof Success Rate: $proof_success_rate"
    echo "  • Average Block Time: $avg_block_time"
    echo "  • Memory Usage: ${heap_mb}MB"
    echo "  • Goroutines: $goroutines"
    
    # Log to CSV
    echo "$current_time,$elapsed_hours,$total_blocks,$blocks_mined,$rewards_shadow,$mempool_txs,$proof_success_rate,$avg_block_time,$heap_mb,$goroutines" >> "$STATS_LOG"
    
    # Check for concerning metrics
    if [ "$heap_mb" -gt 1024 ]; then
        echo "⚠️  WARNING: High memory usage: ${heap_mb}MB" | tee -a "$ERROR_LOG"
    fi
    
    if [ "$goroutines" -gt 1000 ]; then
        echo "⚠️  WARNING: High goroutine count: $goroutines" | tee -a "$ERROR_LOG"
    fi
    
    # Mining health check
    local expected_blocks=$(echo "scale=0; $elapsed_hours * 6" | bc -l)  # 6 blocks/hour target
    local actual_blocks=$blocks_since_start
    
    if [ "$actual_blocks" -lt "$((expected_blocks / 2))" ] && [ "$elapsed_hours" -gt "1" ]; then
        echo "⚠️  WARNING: Mining rate below expected: $actual_blocks vs $expected_blocks expected" | tee -a "$ERROR_LOG"
    fi
    
    echo | tee -a "$MONITOR_LOG"
    return 0
}

# Force initial block to get mining started
echo "🏁 Forcing initial block to start mining cycle..."
curl -s -X POST http://localhost:8080/api/v1/mining/force >/dev/null
sleep 5

# Initial monitoring snapshot
monitor_system

echo "🔄 Starting continuous monitoring every $MONITOR_INTERVAL seconds..."
echo "📁 Logs being written to: $LOG_DIR/"
echo "🛑 To stop early: kill $NODE_PID"
echo

# Main monitoring loop
while true; do
    current_time=$(date +%s)
    elapsed=$((current_time - START_TIME))
    
    # Check if test duration completed
    if [ $elapsed -ge $TEST_DURATION_SECONDS ]; then
        echo "⏰ Test duration completed: $TEST_DURATION_HOURS hours"
        break
    fi
    
    # Sleep until next monitoring interval
    sleep $MONITOR_INTERVAL
    
    # Run monitoring check
    if ! monitor_system; then
        echo "❌ Critical error detected, stopping test early"
        break
    fi
    
    # Force block generation every hour to maintain activity
    local hours_elapsed=$(echo "$elapsed / 3600" | bc)
    local should_force=$((elapsed % 3600))
    if [ $should_force -lt $MONITOR_INTERVAL ] && [ $hours_elapsed -gt 0 ]; then
        echo "🎯 Forcing block generation (hourly maintenance)"
        curl -s -X POST http://localhost:8080/api/v1/mining/force >/dev/null
    fi
done

# Final comprehensive analysis
echo "🏁 Burn-in test completed. Performing final analysis..."

# Stop the node gracefully
echo "🛑 Stopping node gracefully..."
kill -TERM $NODE_PID 2>/dev/null
sleep 10

# Force kill if still running
if kill -0 $NODE_PID 2>/dev/null; then
    echo "⚠️  Force killing node..."
    kill -KILL $NODE_PID 2>/dev/null
fi

# Collect final stats
END_TIME=$(date +%s)
TOTAL_ELAPSED=$((END_TIME - START_TIME))
TOTAL_ELAPSED_HOURS=$(echo "scale=2; $TOTAL_ELAPSED / 3600" | bc -l)

# Analyze results
TOTAL_BLOCKS_END=$(tail -1 "$STATS_LOG" | cut -d',' -f3)
BLOCKS_MINED_END=$(tail -1 "$STATS_LOG" | cut -d',' -f4)
FINAL_REWARDS=$(tail -1 "$STATS_LOG" | cut -d',' -f5)

BLOCKS_GENERATED=$((TOTAL_BLOCKS_END - TOTAL_BLOCKS_START))
ACTUAL_MINING_RATE=$(echo "scale=2; $BLOCKS_GENERATED / $TOTAL_ELAPSED_HOURS" | bc -l)
EXPECTED_BLOCKS=$(echo "scale=0; $TOTAL_ELAPSED_HOURS * 6" | bc -l)

echo
echo "=== BURN-IN TEST RESULTS ==="
echo "📅 Test Duration: ${TOTAL_ELAPSED_HOURS}h (target: ${TEST_DURATION_HOURS}h)"
echo "📊 Blocks Generated: $BLOCKS_GENERATED"
echo "⏱️  Mining Rate: $ACTUAL_MINING_RATE blocks/hour (target: 6.0)"
echo "💰 Total Rewards: $FINAL_REWARDS SHADOW"
echo "🎯 Mining Efficiency: $(echo "scale=1; $BLOCKS_GENERATED * 100 / $EXPECTED_BLOCKS" | bc -l)%"
echo

# Error analysis
if [ -f "$ERROR_LOG" ]; then
    ERROR_COUNT=$(wc -l < "$ERROR_LOG")
    echo "⚠️  Errors/Warnings: $ERROR_COUNT"
    if [ $ERROR_COUNT -gt 0 ]; then
        echo "Recent errors:"
        tail -5 "$ERROR_LOG"
    fi
else
    echo "✅ No errors or warnings detected"
fi

# Performance analysis
echo
echo "=== PERFORMANCE ANALYSIS ==="
if [ -f "$STATS_LOG" ]; then
    echo "📈 Resource Usage Trends:"
    
    # Calculate average memory usage
    AVG_MEMORY=$(tail -n +2 "$STATS_LOG" | cut -d',' -f9 | awk '{sum+=$1; count++} END {printf "%.0f", sum/count}')
    MAX_MEMORY=$(tail -n +2 "$STATS_LOG" | cut -d',' -f9 | sort -n | tail -1)
    
    echo "  • Average Memory: ${AVG_MEMORY}MB"
    echo "  • Peak Memory: ${MAX_MEMORY}MB"
    
    # Calculate mining consistency
    MINING_VARIANCE=$(tail -n +2 "$STATS_LOG" | cut -d',' -f7 | awk '{
        if(NR==1) {min=max=$1}
        if($1<min) min=$1
        if($1>max) max=$1
    } END {printf "%.2f", max-min}')
    
    echo "  • Proof Success Rate Variance: $MINING_VARIANCE"
    
    # Check for crashes/restarts
    SAMPLE_COUNT=$(tail -n +2 "$STATS_LOG" | wc -l)
    EXPECTED_SAMPLES=$(echo "$TOTAL_ELAPSED / $MONITOR_INTERVAL" | bc)
    
    echo "  • Monitoring Samples: $SAMPLE_COUNT / $EXPECTED_SAMPLES expected"
    echo "  • System Uptime: $(echo "scale=1; $SAMPLE_COUNT * 100 / $EXPECTED_SAMPLES" | bc -l)%"
fi

echo
echo "=== TEST VERDICT ==="

# Determine overall test result
PASS_THRESHOLD=80  # 80% efficiency threshold
EFFICIENCY=$(echo "scale=0; $BLOCKS_GENERATED * 100 / $EXPECTED_BLOCKS" | bc -l)

if [ "$EFFICIENCY" -ge "$PASS_THRESHOLD" ] && [ "$ERROR_COUNT" -eq 0 ]; then
    echo "🎉 PASS: Burn-in test successful!"
    echo "   ✅ Mining efficiency: ${EFFICIENCY}% (≥${PASS_THRESHOLD}%)"
    echo "   ✅ No critical errors detected"
    echo "   ✅ System remained stable for ${TOTAL_ELAPSED_HOURS}h"
    EXIT_CODE=0
elif [ "$EFFICIENCY" -ge "$PASS_THRESHOLD" ]; then
    echo "⚠️  PASS (with warnings): System functional but has issues"
    echo "   ✅ Mining efficiency: ${EFFICIENCY}% (≥${PASS_THRESHOLD}%)"
    echo "   ⚠️  $ERROR_COUNT warnings detected"
    EXIT_CODE=1
else
    echo "❌ FAIL: Burn-in test failed"
    echo "   ❌ Mining efficiency: ${EFFICIENCY}% (<${PASS_THRESHOLD}%)"
    echo "   ❌ $ERROR_COUNT errors detected"
    EXIT_CODE=2
fi

echo
echo "📁 Test artifacts saved in: $LOG_DIR/"
echo "   • node.log - Full node output"
echo "   • monitor.log - Monitoring events"
echo "   • stats.csv - Performance metrics"
if [ -f "$ERROR_LOG" ]; then
    echo "   • errors.log - Errors and warnings"
fi

echo
echo "🚀 Mining system burn-in test completed!"
echo "   Start: $(date -d "@$START_TIME")"
echo "   End: $(date -d "@$END_TIME")"
echo "   Duration: ${TOTAL_ELAPSED_HOURS}h"

exit $EXIT_CODE