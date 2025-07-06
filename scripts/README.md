# Shadowy API Testing Scripts

This directory contains bash scripts to exercise all HTTP API endpoints of the Shadowy blockchain node.

## Prerequisites

- Node running: `./shadowy node`
- `jq` installed for JSON parsing
- `curl` for HTTP requests
- `bc` for calculations (stress test only)

## Quick Start

```bash
# Start the node
./shadowy node

# Run all tests
./scripts/test_all.sh

# Monitor continuously
./scripts/monitor.sh
```

## Individual Scripts

### Core Services

- **`health.sh`** - Health checks and node status
- **`blockchain.sh`** - Blockchain operations and block queries
- **`tokenomics.sh`** - Tokenomics, rewards, and supply analysis
- **`mempool.sh`** - Mempool management and transaction submission
- **`wallet.sh`** - Wallet operations and address validation
- **`transactions.sh`** - Transaction creation, signing, and utilities

### Optional Services

- **`farming.sh`** - Farming service (plot management, challenges)
- **`timelord.sh`** - Timelord/VDF operations (requires --enable-timelord)

### Utilities

- **`monitor.sh`** - Continuous monitoring dashboard
- **`stress_test.sh`** - Load testing and performance validation
- **`test_all.sh`** - Run complete test suite

## Environment Variables

| Variable | Default | Description |
|----------|---------|-------------|
| `SHADOWY_API_URL` | `http://localhost:8080` | Base API URL |
| `MONITOR_INTERVAL` | `5` | Monitoring refresh interval (seconds) |
| `STRESS_REQUESTS` | `10` | Number of requests per endpoint |
| `STRESS_CONCURRENT` | `3` | Concurrent request limit |

## Usage Examples

### Basic Testing
```bash
# Test health endpoints
./scripts/health.sh

# Test farming service
./scripts/farming.sh

# Test with remote node
SHADOWY_API_URL=http://remote-node:8080 ./scripts/health.sh
```

### Monitoring
```bash
# Monitor every 10 seconds
MONITOR_INTERVAL=10 ./scripts/monitor.sh

# Monitor remote node
SHADOWY_API_URL=http://remote:8080 ./scripts/monitor.sh
```

### Load Testing
```bash
# Light load test
./scripts/stress_test.sh

# Heavy load test
STRESS_REQUESTS=100 STRESS_CONCURRENT=10 ./scripts/stress_test.sh
```

### Custom Scenarios
```bash
# Test mempool with multiple transactions
for i in {1..5}; do ./scripts/mempool.sh; sleep 2; done

# Submit multiple farming challenges
for i in {1..10}; do 
  echo "Challenge $i"
  ./scripts/farming.sh | grep -A5 "Submit Storage Challenge"
done
```

## API Endpoints Covered

### Health & Status
- `GET /api/v1/health` - Overall health check
- `GET /api/v1/status` - Detailed node status

### Blockchain
- `GET /api/v1/blockchain` - Blockchain statistics
- `GET /api/v1/blockchain/tip` - Get tip (latest) block
- `GET /api/v1/blockchain/block/{hash}` - Get block by hash
- `GET /api/v1/blockchain/block/height/{height}` - Get block by height
- `GET /api/v1/blockchain/recent` - Get recent blocks

### Tokenomics
- `GET /api/v1/tokenomics` - Network stats and current rewards
- `GET /api/v1/tokenomics/reward/{height}` - Block reward at height
- `GET /api/v1/tokenomics/schedule` - Complete reward schedule
- `GET /api/v1/tokenomics/supply/{height}` - Supply analysis at height
- `GET /api/v1/tokenomics/halvings` - Halving history and projections

### Mempool
- `GET /api/v1/mempool` - Mempool statistics
- `GET /api/v1/mempool/transactions` - List transactions
- `POST /api/v1/mempool/transactions` - Submit transaction
- `GET /api/v1/mempool/transactions/{hash}` - Get specific transaction

### Farming (if enabled)
- `GET /api/v1/farming` - Farming statistics
- `GET /api/v1/farming/status` - Service status
- `GET /api/v1/farming/plots` - List plot files
- `POST /api/v1/farming/challenge` - Submit storage challenge

### Timelord (if enabled)
- `GET /api/v1/timelord` - Timelord statistics
- `POST /api/v1/timelord/jobs` - Submit VDF job
- `GET /api/v1/timelord/jobs/{id}` - Get job status

### Wallet
- `GET /api/v1/wallet` - List wallets
- `GET /api/v1/wallet/{name}` - Get wallet info
- `GET /api/v1/wallet/{name}/balance` - Get wallet balance

### Utilities
- `POST /api/v1/utils/validate-address` - Validate address
- `POST /api/v1/utils/transaction/create` - Create transaction
- `POST /api/v1/utils/transaction/sign` - Sign transaction

## Output Examples

### Health Check
```json
{
  "status": "ok",
  "healthy": true,
  "services": {
    "farming": {
      "name": "farming",
      "status": "healthy",
      "last_check": "2025-07-04T10:33:39Z"
    }
  }
}
```

### Farming Challenge Response
```json
{
  "challenge_id": "api_1751625453796816151",
  "plot_file": "umbra_v1_k32_20250704-100311_4e655f84.dat",
  "offset": 0,
  "private_key": "placeholder_key",
  "signature": "placeholder_signature",
  "valid": true,
  "response_time": 80
}
```

## Error Handling

Scripts include error handling for:
- Node not running
- Services not enabled
- Network timeouts
- Invalid responses
- Missing dependencies

## Contributing

To add new test scripts:
1. Create script in `scripts/` directory
2. Make it executable: `chmod +x script.sh`
3. Use consistent environment variables
4. Include error handling and JSON formatting
5. Add to `test_all.sh` if appropriate