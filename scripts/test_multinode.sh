#!/bin/bash

# Multi-Node Testing Script for Shadowy Blockchain
# Tests communication and synchronization between nodes

set -e

# Configuration
LOCAL_HOST="192.168.68.90"
REMOTE_HOST="192.168.68.62"
REMOTE_USER="nanocat"
LOCAL_API="http://${LOCAL_HOST}:8080/api/v1"
REMOTE_API="http://${REMOTE_HOST}:8080/api/v1"

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

log_test() {
    echo -e "${YELLOW}[TEST]${NC} $1"
}

# Test functions
test_api_connectivity() {
    log_test "Testing API connectivity..."
    
    log_info "Checking local node (${LOCAL_HOST})..."
    if curl -s --connect-timeout 5 "${LOCAL_API}/health" > /dev/null; then
        log_success "Local node API is reachable"
    else
        log_error "Local node API is not reachable"
        return 1
    fi
    
    log_info "Checking remote node (${REMOTE_HOST})..."
    if curl -s --connect-timeout 5 "${REMOTE_API}/health" > /dev/null; then
        log_success "Remote node API is reachable"
    else
        log_error "Remote node API is not reachable"
        return 1
    fi
}

test_node_health() {
    log_test "Testing node health status..."
    
    log_info "Local node health:"
    local_health=$(curl -s --connect-timeout 5 "${LOCAL_API}/health" | jq -r '.healthy // false')
    if [[ "$local_health" == "true" ]]; then
        log_success "Local node is healthy"
    else
        log_warning "Local node reports unhealthy status"
    fi
    
    log_info "Remote node health:"
    remote_health=$(curl -s --connect-timeout 5 "${REMOTE_API}/health" | jq -r '.healthy // false')
    if [[ "$remote_health" == "true" ]]; then
        log_success "Remote node is healthy"
    else
        log_warning "Remote node reports unhealthy status"
    fi
}

test_blockchain_sync() {
    log_test "Testing blockchain synchronization..."
    
    log_info "Getting blockchain heights..."
    local_height=$(curl -s "${LOCAL_API}/blockchain" | jq -r '.tip_height // 0')
    remote_height=$(curl -s "${REMOTE_API}/blockchain" | jq -r '.tip_height // 0')
    
    log_info "Local height: ${local_height}"
    log_info "Remote height: ${remote_height}"
    
    height_diff=$((local_height - remote_height))
    if [[ $height_diff -lt 0 ]]; then
        height_diff=$((-height_diff))
    fi
    
    if [[ $height_diff -le 1 ]]; then
        log_success "Blockchain heights are synchronized (difference: ${height_diff})"
    else
        log_warning "Blockchain heights differ by ${height_diff} blocks"
    fi
    
    # Get tip hashes
    local_hash=$(curl -s "${LOCAL_API}/blockchain" | jq -r '.tip_hash // "unknown"')
    remote_hash=$(curl -s "${REMOTE_API}/blockchain" | jq -r '.tip_hash // "unknown"')
    
    log_info "Local tip hash: ${local_hash}"
    log_info "Remote tip hash: ${remote_hash}"
    
    if [[ "$local_hash" == "$remote_hash" && "$local_hash" != "unknown" ]]; then
        log_success "Tip hashes match - nodes are fully synchronized"
    else
        log_warning "Tip hashes don't match - nodes may be out of sync"
    fi
}

test_peer_connectivity() {
    log_test "Testing peer-to-peer connectivity..."
    
    log_info "Checking local node peers..."
    local_peers=$(curl -s "${LOCAL_API}/consensus/peers" | jq -r 'length // 0')
    log_info "Local node has ${local_peers} connected peers"
    
    log_info "Checking remote node peers..."
    remote_peers=$(curl -s "${REMOTE_API}/consensus/peers" | jq -r 'length // 0')
    log_info "Remote node has ${remote_peers} connected peers"
    
    # Check if nodes are connected to each other
    log_info "Checking if nodes are connected to each other..."
    local_peer_list=$(curl -s "${LOCAL_API}/consensus/peers" | jq -r '.[].address // empty')
    remote_peer_list=$(curl -s "${REMOTE_API}/consensus/peers" | jq -r '.[].address // empty')
    
    local_has_remote=false
    remote_has_local=false
    
    while IFS= read -r peer; do
        if [[ "$peer" == *"${REMOTE_HOST}"* ]]; then
            local_has_remote=true
            break
        fi
    done <<< "$local_peer_list"
    
    while IFS= read -r peer; do
        if [[ "$peer" == *"${LOCAL_HOST}"* ]]; then
            remote_has_local=true
            break
        fi
    done <<< "$remote_peer_list"
    
    if [[ "$local_has_remote" == "true" && "$remote_has_local" == "true" ]]; then
        log_success "Nodes are connected to each other"
    elif [[ "$local_has_remote" == "true" ]]; then
        log_warning "Local node is connected to remote, but not vice versa"
    elif [[ "$remote_has_local" == "true" ]]; then
        log_warning "Remote node is connected to local, but not vice versa"
    else
        log_error "Nodes are not connected to each other"
    fi
}

test_mempool_sync() {
    log_test "Testing mempool synchronization..."
    
    local_mempool=$(curl -s "${LOCAL_API}/mempool" | jq -r '.transaction_count // 0')
    remote_mempool=$(curl -s "${REMOTE_API}/mempool" | jq -r '.transaction_count // 0')
    
    log_info "Local mempool transactions: ${local_mempool}"
    log_info "Remote mempool transactions: ${remote_mempool}"
    
    if [[ "$local_mempool" == "$remote_mempool" ]]; then
        log_success "Mempool transaction counts match"
    else
        log_warning "Mempool transaction counts differ (${local_mempool} vs ${remote_mempool})"
    fi
}

test_mining_status() {
    log_test "Testing mining status..."
    
    log_info "Local mining status:"
    local_mining=$(curl -s "${LOCAL_API}/mining/status" 2>/dev/null | jq -r '.running // false')
    if [[ "$local_mining" == "true" ]]; then
        log_success "Local node is mining"
    else
        log_info "Local node is not mining"
    fi
    
    log_info "Remote mining status:"
    remote_mining=$(curl -s "${REMOTE_API}/mining/status" 2>/dev/null | jq -r '.running // false')
    if [[ "$remote_mining" == "true" ]]; then
        log_success "Remote node is mining"
    else
        log_info "Remote node is not mining"
    fi
}

test_block_propagation() {
    log_test "Testing block propagation..."
    
    log_info "Recording initial heights..."
    initial_local=$(curl -s "${LOCAL_API}/blockchain" | jq -r '.tip_height // 0')
    initial_remote=$(curl -s "${REMOTE_API}/blockchain" | jq -r '.tip_height // 0')
    
    log_info "Initial local height: ${initial_local}"
    log_info "Initial remote height: ${initial_remote}"
    
    log_info "Waiting for new blocks (up to 60 seconds)..."
    timeout=60
    elapsed=0
    
    while [[ $elapsed -lt $timeout ]]; do
        sleep 5
        elapsed=$((elapsed + 5))
        
        current_local=$(curl -s "${LOCAL_API}/blockchain" | jq -r '.tip_height // 0')
        current_remote=$(curl -s "${REMOTE_API}/blockchain" | jq -r '.tip_height // 0')
        
        if [[ $current_local -gt $initial_local || $current_remote -gt $initial_remote ]]; then
            log_info "New block detected after ${elapsed} seconds"
            log_info "Local height: ${initial_local} -> ${current_local}"
            log_info "Remote height: ${initial_remote} -> ${current_remote}"
            
            # Wait a bit more for propagation
            sleep 10
            final_local=$(curl -s "${LOCAL_API}/blockchain" | jq -r '.tip_height // 0')
            final_remote=$(curl -s "${REMOTE_API}/blockchain" | jq -r '.tip_height // 0')
            
            if [[ $final_local -eq $final_remote ]]; then
                log_success "Block propagated successfully between nodes"
            else
                log_warning "Block propagation may be incomplete (${final_local} vs ${final_remote})"
            fi
            return 0
        fi
        
        log_info "Waiting... (${elapsed}/${timeout}s)"
    done
    
    log_warning "No new blocks detected within timeout period"
}

force_peer_connection() {
    log_test "Attempting to force peer connections..."
    
    log_info "Connecting local node to remote node..."
    curl -s -X POST "${LOCAL_API}/consensus/peers/connect" \
         -H "Content-Type: application/json" \
         -d "{\"address\": \"${REMOTE_HOST}:8888\"}" || log_warning "Failed to force connection"
    
    sleep 5
    
    log_info "Connecting remote node to local node..."
    curl -s -X POST "${REMOTE_API}/consensus/peers/connect" \
         -H "Content-Type: application/json" \
         -d "{\"address\": \"${LOCAL_HOST}:8888\"}" || log_warning "Failed to force connection"
    
    sleep 5
}

show_summary() {
    log_info "=== Multi-Node Test Summary ==="
    echo ""
    
    # API connectivity
    if curl -s --connect-timeout 3 "${LOCAL_API}/health" > /dev/null && \
       curl -s --connect-timeout 3 "${REMOTE_API}/health" > /dev/null; then
        echo "✅ API Connectivity: PASS"
    else
        echo "❌ API Connectivity: FAIL"
    fi
    
    # Health status
    local_healthy=$(curl -s "${LOCAL_API}/health" | jq -r '.healthy // false')
    remote_healthy=$(curl -s "${REMOTE_API}/health" | jq -r '.healthy // false')
    if [[ "$local_healthy" == "true" && "$remote_healthy" == "true" ]]; then
        echo "✅ Node Health: PASS"
    else
        echo "⚠️  Node Health: PARTIAL (local: ${local_healthy}, remote: ${remote_healthy})"
    fi
    
    # Blockchain sync
    local_height=$(curl -s "${LOCAL_API}/blockchain" | jq -r '.tip_height // 0')
    remote_height=$(curl -s "${REMOTE_API}/blockchain" | jq -r '.tip_height // 0')
    height_diff=$((local_height - remote_height))
    if [[ $height_diff -lt 0 ]]; then height_diff=$((-height_diff)); fi
    
    if [[ $height_diff -le 1 ]]; then
        echo "✅ Blockchain Sync: PASS (heights: ${local_height}, ${remote_height})"
    else
        echo "⚠️  Blockchain Sync: PARTIAL (height difference: ${height_diff})"
    fi
    
    # Peer connectivity
    local_peers=$(curl -s "${LOCAL_API}/consensus/peers" | jq -r 'length // 0')
    remote_peers=$(curl -s "${REMOTE_API}/consensus/peers" | jq -r 'length // 0')
    echo "ℹ️  Peer Count: Local(${local_peers}), Remote(${remote_peers})"
    
    echo ""
    log_info "Test completed at $(date)"
}

# Main execution
main() {
    echo "=================================="
    echo "Shadowy Multi-Node Test Suite"
    echo "=================================="
    echo "Local Node: ${LOCAL_HOST}:8080"
    echo "Remote Node: ${REMOTE_HOST}:8080"
    echo "Test Started: $(date)"
    echo ""
    
    # Run tests
    test_api_connectivity || exit 1
    echo ""
    
    test_node_health
    echo ""
    
    test_peer_connectivity
    echo ""
    
    # If nodes aren't connected, try to force connection
    local_peer_list=$(curl -s "${LOCAL_API}/consensus/peers" | jq -r '.[].address // empty')
    if ! echo "$local_peer_list" | grep -q "${REMOTE_HOST}"; then
        log_warning "Nodes not connected, attempting to force connection..."
        force_peer_connection
        echo ""
        test_peer_connectivity
        echo ""
    fi
    
    test_blockchain_sync
    echo ""
    
    test_mempool_sync
    echo ""
    
    test_mining_status
    echo ""
    
    if [[ "${1:-}" != "--no-block-test" ]]; then
        test_block_propagation
        echo ""
    fi
    
    show_summary
}

# Check dependencies
if ! command -v jq &> /dev/null; then
    log_error "jq is required but not installed. Please install jq first."
    exit 1
fi

if ! command -v curl &> /dev/null; then
    log_error "curl is required but not installed. Please install curl first."
    exit 1
fi

# Run main function
main "$@"