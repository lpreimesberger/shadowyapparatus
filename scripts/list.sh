#!/bin/bash
# List all available API testing scripts

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo "=== Shadowy API Testing Scripts ==="
echo "Directory: $SCRIPT_DIR"
echo

echo "📋 Available Scripts:"
echo

# Core service scripts
echo "🔧 Core Services:"
echo "  health.sh        - Health checks and node status monitoring"
echo "  blockchain.sh    - Blockchain operations and block queries"
echo "  tokenomics.sh    - Tokenomics, rewards, and supply analysis"
echo "  mempool.sh       - Mempool operations and transaction management"
echo "  wallet.sh        - Wallet management and address validation"
echo "  transactions.sh  - Transaction creation, signing, and utilities"
echo

# Optional service scripts
echo "⚡ Optional Services:"
echo "  farming.sh       - Farming service operations (plot management, challenges)"
echo "  mining.sh        - Mining service operations (block generation, rewards)"
echo "  timelord.sh      - Timelord/VDF operations (requires --enable-timelord)"
echo

# Utility scripts
echo "🛠️  Utilities:"
echo "  monitor.sh       - Continuous monitoring dashboard"
echo "  stress_test.sh   - Load testing and performance validation"
echo "  test_all.sh      - Run complete test suite"
echo "  list.sh          - This script (list all available scripts)"
echo

echo "📖 Usage Examples:"
echo "  ./scripts/test_all.sh                    # Run all tests"
echo "  ./scripts/monitor.sh                     # Monitor continuously"
echo "  ./scripts/health.sh                      # Quick health check"
echo "  STRESS_REQUESTS=50 ./scripts/stress_test.sh  # Load test"
echo

echo "🌐 Environment Variables:"
echo "  SHADOWY_API_URL     Default: http://localhost:8080"
echo "  MONITOR_INTERVAL    Default: 5 (seconds)"
echo "  STRESS_REQUESTS     Default: 10"
echo "  STRESS_CONCURRENT   Default: 3"
echo

echo "📚 Documentation:"
echo "  scripts/README.md - Complete documentation"
echo "  FARMING_API.md    - Farming API reference"
echo

# Check if node is running
if curl -s --connect-timeout 2 "${SHADOWY_API_URL:-http://localhost:8080}/api/v1/health" >/dev/null 2>&1; then
  echo "✅ Node Status: Running at ${SHADOWY_API_URL:-http://localhost:8080}"
else
  echo "❌ Node Status: Not running"
  echo "   Start with: ./shadowy node"
fi

echo