#!/bin/bash
# Quick monitoring setup for development and testing

set -e

# Configuration
NODE_PORT=${NODE_PORT:-8080}
MONITOR_PORT=${MONITOR_PORT:-9999}

# Colors
GREEN='\033[0;32m'
BLUE='\033[0;34m'
NC='\033[0m'

log() {
    echo -e "${BLUE}[$(date '+%H:%M:%S')]${NC} $1"
}

success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

# Cleanup function
cleanup() {
    log "Stopping services..."
    pkill -f "shadowy node" 2>/dev/null || true
    pkill -f "shadowy monitor" 2>/dev/null || true
    log "Services stopped"
}

trap cleanup EXIT INT TERM

log "🌑 Starting Shadowy Quick Monitor"

# Build if needed
if [ ! -f "./shadowy" ]; then
    log "Building shadowy..."
    go build -o shadowy .
fi

# Start node in background
log "Starting blockchain node on port $NODE_PORT..."
./shadowy node --http-port=$NODE_PORT > node.log 2>&1 &

# Wait for node
sleep 8

# Check node health
log "Checking node health..."
if curl -s "http://localhost:$NODE_PORT/api/v1/health" > /dev/null; then
    success "Node is healthy"
else
    echo "❌ Node health check failed"
    exit 1
fi

# Start monitor
log "Starting web monitor on port $MONITOR_PORT..."
./shadowy monitor --port=$MONITOR_PORT --api-url="http://localhost:$NODE_PORT" > monitor.log 2>&1 &

# Wait for monitor
sleep 5

# Check monitor
if curl -s "http://localhost:$MONITOR_PORT/" > /dev/null; then
    success "Monitor is running"
else
    echo "❌ Monitor failed to start"
    exit 1
fi

success "🚀 Quick monitor setup complete!"
echo ""
echo "📊 Web Dashboard: http://localhost:$MONITOR_PORT"
echo "🔗 Node API:      http://localhost:$NODE_PORT"
echo "📝 Logs:          node.log, monitor.log"
echo ""
echo "Press Ctrl+C to stop all services"

# Keep running until interrupted
while true; do
    sleep 30
    
    # Quick health check
    if ! curl -s "http://localhost:$NODE_PORT/api/v1/health" > /dev/null; then
        echo "⚠️  Node health check failed at $(date)"
    fi
    
    if ! curl -s "http://localhost:$MONITOR_PORT/api/monitoring" > /dev/null; then
        echo "⚠️  Monitor health check failed at $(date)"
    fi
done