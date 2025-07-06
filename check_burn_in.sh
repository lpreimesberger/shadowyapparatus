#!/bin/bash
# Check status of running burn-in test

LOG_DIR="./burn_in_logs"

if [ ! -d "$LOG_DIR" ]; then
    echo "❌ No burn-in test detected (no log directory found)"
    exit 1
fi

if [ ! -f "$LOG_DIR/stats.csv" ]; then
    echo "❌ No burn-in test statistics found"
    exit 1
fi

echo "=== Burn-in Test Status Check ==="
echo "🕐 Current Time: $(date)"
echo

# Check if node is running
if pgrep -f "shadowy node" >/dev/null; then
    NODE_PID=$(pgrep -f "shadowy node")
    echo "✅ Node Status: Running (PID: $NODE_PID)"
else
    echo "❌ Node Status: Not running"
fi

# Check API responsiveness
if curl -s --connect-timeout 5 http://localhost:8080/api/v1/health >/dev/null 2>&1; then
    echo "✅ API Status: Responsive"
else
    echo "❌ API Status: Not responding"
fi

echo

# Get latest stats from CSV
if [ -f "$LOG_DIR/stats.csv" ]; then
    LATEST_STATS=$(tail -1 "$LOG_DIR/stats.csv")
    if [ -n "$LATEST_STATS" ] && [ "$LATEST_STATS" != "timestamp,elapsed_hours,total_blocks,blocks_mined,rewards_shadow,mempool_txs,proof_success_rate,avg_block_time,heap_mb,goroutines" ]; then
        
        ELAPSED_HOURS=$(echo "$LATEST_STATS" | cut -d',' -f2)
        TOTAL_BLOCKS=$(echo "$LATEST_STATS" | cut -d',' -f3)
        BLOCKS_MINED=$(echo "$LATEST_STATS" | cut -d',' -f4)
        REWARDS_SHADOW=$(echo "$LATEST_STATS" | cut -d',' -f5)
        MEMPOOL_TXS=$(echo "$LATEST_STATS" | cut -d',' -f6)
        PROOF_SUCCESS_RATE=$(echo "$LATEST_STATS" | cut -d',' -f7)
        AVG_BLOCK_TIME=$(echo "$LATEST_STATS" | cut -d',' -f8)
        HEAP_MB=$(echo "$LATEST_STATS" | cut -d',' -f9)
        GOROUTINES=$(echo "$LATEST_STATS" | cut -d',' -f10)
        
        echo "📊 Latest Statistics (${ELAPSED_HOURS}h elapsed):"
        echo "  • Total Blocks: $TOTAL_BLOCKS"
        echo "  • Blocks Mined: $BLOCKS_MINED"
        echo "  • Total Rewards: $REWARDS_SHADOW SHADOW"
        echo "  • Mempool Transactions: $MEMPOOL_TXS"
        echo "  • Proof Success Rate: $PROOF_SUCCESS_RATE"
        echo "  • Average Block Time: $AVG_BLOCK_TIME"
        echo "  • Memory Usage: ${HEAP_MB}MB"
        echo "  • Goroutines: $GOROUTINES"
        
        # Calculate mining rate
        if [ "$ELAPSED_HOURS" != "0" ]; then
            MINING_RATE=$(echo "scale=2; $BLOCKS_MINED / $ELAPSED_HOURS" | bc -l)
            echo "  • Mining Rate: $MINING_RATE blocks/hour"
        fi
        
        echo
        
        # Check for issues
        if [ "$HEAP_MB" -gt 1024 ]; then
            echo "⚠️  High memory usage: ${HEAP_MB}MB"
        fi
        
        if [ "$GOROUTINES" -gt 1000 ]; then
            echo "⚠️  High goroutine count: $GOROUTINES"
        fi
        
        # Mining efficiency check
        EXPECTED_BLOCKS=$(echo "scale=0; $ELAPSED_HOURS * 6" | bc -l)
        if [ "$EXPECTED_BLOCKS" -gt 0 ]; then
            EFFICIENCY=$(echo "scale=1; $BLOCKS_MINED * 100 / $EXPECTED_BLOCKS" | bc -l)
            echo "🎯 Mining Efficiency: ${EFFICIENCY}% (target: 100%)"
        fi
    else
        echo "⏳ No statistics available yet (test may be starting up)"
    fi
fi

# Show recent log entries
echo
echo "📋 Recent Activity (last 10 lines from monitor log):"
if [ -f "$LOG_DIR/monitor.log" ]; then
    tail -10 "$LOG_DIR/monitor.log"
else
    echo "No monitor log available yet"
fi

# Show errors if any
echo
if [ -f "$LOG_DIR/errors.log" ] && [ -s "$LOG_DIR/errors.log" ]; then
    ERROR_COUNT=$(wc -l < "$LOG_DIR/errors.log")
    echo "⚠️  Errors/Warnings: $ERROR_COUNT"
    echo "Recent errors:"
    tail -5 "$LOG_DIR/errors.log"
else
    echo "✅ No errors or warnings detected"
fi

echo
echo "📁 Log files location: $LOG_DIR/"
echo "🔄 To continuously monitor: watch -n 30 ./check_burn_in.sh"