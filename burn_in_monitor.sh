#!/bin/bash
# Enhanced burn-in testing with web monitoring dashboard
# This script starts a blockchain node, web monitor, and runs comprehensive tests

set -e

# Configuration
NODE_PORT=${NODE_PORT:-8080}
MONITOR_PORT=${MONITOR_PORT:-9999}
TEST_DURATION=${TEST_DURATION:-3600}  # 1 hour default
LOG_DIR="./burn_in_logs"
RESULTS_FILE="$LOG_DIR/burn_in_results.json"
STATS_FILE="$LOG_DIR/burn_in_stats.csv"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging function
log() {
    echo -e "${BLUE}[$(date '+%Y-%m-%d %H:%M:%S')]${NC} $1"
}

error() {
    echo -e "${RED}[ERROR]${NC} $1" >&2
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

# Cleanup function
cleanup() {
    log "Cleaning up processes..."
    
    # Stop web monitor
    if [ ! -z "$MONITOR_PID" ] && kill -0 $MONITOR_PID 2>/dev/null; then
        log "Stopping web monitor (PID: $MONITOR_PID)"
        kill $MONITOR_PID
        wait $MONITOR_PID 2>/dev/null || true
    fi
    
    # Stop node
    if [ ! -z "$NODE_PID" ] && kill -0 $NODE_PID 2>/dev/null; then
        log "Stopping blockchain node (PID: $NODE_PID)"
        kill $NODE_PID
        wait $NODE_PID 2>/dev/null || true
    fi
    
    # Stop any remaining processes
    pkill -f "shadowy node" 2>/dev/null || true
    pkill -f "shadowy monitor" 2>/dev/null || true
    
    log "Cleanup completed"
}

# Set up signal handlers
trap cleanup EXIT INT TERM

# Create log directory
mkdir -p "$LOG_DIR"

# Initialize CSV stats file
echo "timestamp,block_height,hash_rate,peer_count,mempool_size,memory_mb,cpu_percent,errors" > "$STATS_FILE"

log "🌑 Starting Shadowy Blockchain Burn-in Test"
log "Test Duration: ${TEST_DURATION} seconds ($(($TEST_DURATION / 60)) minutes)"
log "Node Port: $NODE_PORT"
log "Monitor Port: $MONITOR_PORT"
log "Log Directory: $LOG_DIR"

# Build the project if needed
if [ ! -f "./shadowy" ]; then
    log "Building shadowy binary..."
    go build -o shadowy .
    if [ $? -ne 0 ]; then
        error "Failed to build shadowy binary"
        exit 1
    fi
    success "Built shadowy binary successfully"
fi

# Start blockchain node
log "Starting blockchain node on port $NODE_PORT..."
./shadowy node --http-port=$NODE_PORT > "$LOG_DIR/node.log" 2>&1 &
NODE_PID=$!

# Wait for node to start
log "Waiting for node to initialize..."
sleep 10

# Check if node is running
if ! kill -0 $NODE_PID 2>/dev/null; then
    error "Blockchain node failed to start"
    exit 1
fi

# Verify node is responding
log "Verifying node health..."
for i in {1..30}; do
    if curl -s "http://localhost:$NODE_PORT/api/v1/health" > /dev/null 2>&1; then
        success "Node is responding to health checks"
        break
    fi
    if [ $i -eq 30 ]; then
        error "Node failed to respond to health checks after 30 attempts"
        exit 1
    fi
    sleep 2
done

# Start web monitoring dashboard
log "Starting web monitoring dashboard on port $MONITOR_PORT..."
./shadowy monitor --port=$MONITOR_PORT --api-url="http://localhost:$NODE_PORT" > "$LOG_DIR/monitor.log" 2>&1 &
MONITOR_PID=$!

# Wait for monitor to start
sleep 5

# Check if monitor is running
if ! kill -0 $MONITOR_PID 2>/dev/null; then
    error "Web monitor failed to start"
    exit 1
fi

# Verify monitor is responding
log "Verifying monitor dashboard..."
for i in {1..10}; do
    if curl -s "http://localhost:$MONITOR_PORT/api/monitoring" > /dev/null 2>&1; then
        success "Monitor dashboard is responding"
        break
    fi
    if [ $i -eq 10 ]; then
        error "Monitor dashboard failed to respond after 10 attempts"
        exit 1
    fi
    sleep 2
done

success "🚀 All services started successfully!"
log "📊 Web Dashboard: http://localhost:$MONITOR_PORT"
log "🔗 Node API: http://localhost:$NODE_PORT"
log "📝 Logs: $LOG_DIR/"

# Initialize result tracking
START_TIME=$(date +%s)
ERROR_COUNT=0
TOTAL_CHECKS=0
BLOCK_COUNT_START=0
HASH_RATE_SAMPLES=()

# Get initial metrics
INITIAL_RESPONSE=$(curl -s "http://localhost:$NODE_PORT/api/v1/blockchain" || echo "{}")
BLOCK_COUNT_START=$(echo "$INITIAL_RESPONSE" | jq -r '.height // 0' 2>/dev/null || echo "0")

log "Starting burn-in monitoring loop..."
log "Initial block height: $BLOCK_COUNT_START"

# Main monitoring loop
while true; do
    CURRENT_TIME=$(date +%s)
    ELAPSED=$((CURRENT_TIME - START_TIME))
    
    # Check if test duration is reached
    if [ $ELAPSED -ge $TEST_DURATION ]; then
        log "Test duration reached. Stopping burn-in test."
        break
    fi
    
    TOTAL_CHECKS=$((TOTAL_CHECKS + 1))
    
    # Collect metrics from various endpoints
    NODE_HEALTH=$(curl -s "http://localhost:$NODE_PORT/api/v1/health" 2>/dev/null || echo '{"status":"error"}')
    BLOCKCHAIN_STATUS=$(curl -s "http://localhost:$NODE_PORT/api/v1/blockchain" 2>/dev/null || echo '{}')
    MINING_STATUS=$(curl -s "http://localhost:$NODE_PORT/api/v1/mining" 2>/dev/null || echo '{}')
    CONSENSUS_STATUS=$(curl -s "http://localhost:$NODE_PORT/api/v1/consensus" 2>/dev/null || echo '{}')
    MEMPOOL_STATUS=$(curl -s "http://localhost:$NODE_PORT/api/v1/mempool" 2>/dev/null || echo '{}')
    MONITOR_METRICS=$(curl -s "http://localhost:$MONITOR_PORT/api/metrics" 2>/dev/null || echo '{}')
    
    # Parse metrics (using jq if available, otherwise basic parsing)
    if command -v jq >/dev/null 2>&1; then
        BLOCK_HEIGHT=$(echo "$BLOCKCHAIN_STATUS" | jq -r '.height // 0')
        HASH_RATE=$(echo "$MINING_STATUS" | jq -r '.status.hash_rate // 0')
        PEER_COUNT=$(echo "$CONSENSUS_STATUS" | jq -r '.connected_peers // 0')
        MEMPOOL_SIZE=$(echo "$MEMPOOL_STATUS" | jq -r '.stats.pending_count // 0')
        MEMORY_MB=$(echo "$MONITOR_METRICS" | jq -r '.memory_usage_mb // 0')
        CPU_PERCENT=$(echo "$MONITOR_METRICS" | jq -r '.cpu_usage_percent // 0')
        NODE_STATUS=$(echo "$NODE_HEALTH" | jq -r '.status // "unknown"')
    else
        # Fallback parsing without jq
        BLOCK_HEIGHT=$(echo "$BLOCKCHAIN_STATUS" | grep -o '"height":[0-9]*' | cut -d: -f2 || echo "0")
        HASH_RATE="0"
        PEER_COUNT="0"
        MEMPOOL_SIZE="0"
        MEMORY_MB="0"
        CPU_PERCENT="0"
        NODE_STATUS="unknown"
    fi
    
    # Check for errors
    if [ "$NODE_STATUS" != "healthy" ] && [ "$NODE_STATUS" != "ok" ]; then
        ERROR_COUNT=$((ERROR_COUNT + 1))
        warning "Node health check failed: $NODE_STATUS"
    fi
    
    # Log stats to CSV
    echo "$(date -Iseconds),$BLOCK_HEIGHT,$HASH_RATE,$PEER_COUNT,$MEMPOOL_SIZE,$MEMORY_MB,$CPU_PERCENT,$ERROR_COUNT" >> "$STATS_FILE"
    
    # Progress report every 30 seconds
    if [ $((ELAPSED % 30)) -eq 0 ]; then
        BLOCKS_MINED=$((BLOCK_HEIGHT - BLOCK_COUNT_START))
        BLOCKS_PER_HOUR=$(( (BLOCKS_MINED * 3600) / (ELAPSED + 1) ))
        PROGRESS=$((ELAPSED * 100 / TEST_DURATION))
        
        log "Progress: ${PROGRESS}% | Elapsed: ${ELAPSED}s | Blocks: +$BLOCKS_MINED | Rate: ${BLOCKS_PER_HOUR}/hr | Errors: $ERROR_COUNT"
        
        # Detailed status every 5 minutes
        if [ $((ELAPSED % 300)) -eq 0 ] && [ $ELAPSED -gt 0 ]; then
            log "=== Detailed Status ==="
            log "Block Height: $BLOCK_HEIGHT"
            log "Hash Rate: $HASH_RATE"
            log "Connected Peers: $PEER_COUNT"
            log "Mempool Size: $MEMPOOL_SIZE"
            log "Memory Usage: ${MEMORY_MB}MB"
            log "CPU Usage: ${CPU_PERCENT}%"
            log "Error Rate: $(echo "scale=2; $ERROR_COUNT * 100 / $TOTAL_CHECKS" | bc -l 2>/dev/null || echo "N/A")%"
            log "======================="
        fi
    fi
    
    # Sleep before next check
    sleep 5
done

# Calculate final results
END_TIME=$(date +%s)
TOTAL_DURATION=$((END_TIME - START_TIME))
FINAL_BLOCK_HEIGHT=$(echo "$BLOCKCHAIN_STATUS" | jq -r '.height // 0' 2>/dev/null || echo "$BLOCK_HEIGHT")
TOTAL_BLOCKS_MINED=$((FINAL_BLOCK_HEIGHT - BLOCK_COUNT_START))
AVERAGE_BLOCKS_PER_HOUR=$(( (TOTAL_BLOCKS_MINED * 3600) / TOTAL_DURATION ))
ERROR_RATE=$(echo "scale=4; $ERROR_COUNT * 100 / $TOTAL_CHECKS" | bc -l 2>/dev/null || echo "0")

# Generate final report
log "🎯 Burn-in test completed!"
log "📊 Generating final report..."

cat > "$RESULTS_FILE" << EOF
{
  "test_summary": {
    "start_time": "$START_TIME",
    "end_time": "$END_TIME", 
    "duration_seconds": $TOTAL_DURATION,
    "duration_minutes": $(($TOTAL_DURATION / 60)),
    "test_completed": true
  },
  "blockchain_metrics": {
    "initial_block_height": $BLOCK_COUNT_START,
    "final_block_height": $FINAL_BLOCK_HEIGHT,
    "blocks_mined": $TOTAL_BLOCKS_MINED,
    "average_blocks_per_hour": $AVERAGE_BLOCKS_PER_HOUR,
    "target_blocks_per_hour": 6
  },
  "performance_metrics": {
    "total_health_checks": $TOTAL_CHECKS,
    "failed_health_checks": $ERROR_COUNT,
    "error_rate_percent": $ERROR_RATE,
    "uptime_percent": $(echo "scale=2; (100 - $ERROR_RATE)" | bc -l 2>/dev/null || echo "100")
  },
  "final_status": {
    "node_health": "$(echo "$NODE_HEALTH" | jq -r '.status // "unknown"' 2>/dev/null || echo "unknown")",
    "peer_connections": $PEER_COUNT,
    "mempool_transactions": $MEMPOOL_SIZE
  },
  "files_generated": {
    "results": "$RESULTS_FILE",
    "stats_csv": "$STATS_FILE", 
    "node_log": "$LOG_DIR/node.log",
    "monitor_log": "$LOG_DIR/monitor.log"
  }
}
EOF

# Display final summary
success "═══════════════════════════════════════"
success "🌑 SHADOWY BLOCKCHAIN BURN-IN COMPLETE"
success "═══════════════════════════════════════"
log "📊 Test Duration: $(($TOTAL_DURATION / 60)) minutes"
log "🧱 Blocks Mined: $TOTAL_BLOCKS_MINED"
log "⚡ Average Rate: $AVERAGE_BLOCKS_PER_HOUR blocks/hour"
log "🎯 Target Rate: 6 blocks/hour"
log "✅ Uptime: $(echo "scale=1; 100 - $ERROR_RATE" | bc -l 2>/dev/null || echo "100")%"
log "❌ Error Rate: ${ERROR_RATE}%"
log "📁 Results: $RESULTS_FILE"
log "📈 Stats: $STATS_FILE"
success "═══════════════════════════════════════"

# Determine test result
if [ $ERROR_COUNT -eq 0 ] && [ $TOTAL_BLOCKS_MINED -gt 0 ]; then
    success "🎉 BURN-IN TEST PASSED!"
    exit 0
elif [ $ERROR_COUNT -lt $(($TOTAL_CHECKS / 10)) ]; then
    warning "⚠️  BURN-IN TEST PASSED WITH WARNINGS"
    warning "Error rate below 10% threshold"
    exit 0
else
    error "❌ BURN-IN TEST FAILED!"
    error "High error rate or no blocks mined"
    exit 1
fi