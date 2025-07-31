# 📊 Shadowy Blockchain Monitoring

The Shadowy blockchain includes comprehensive monitoring capabilities for development, testing, and production environments.

## 🌐 Web Monitoring Dashboard

### Features

The web monitoring dashboard provides real-time insights into:

- **Node Health**: System status, uptime, resource usage
- **Blockchain Status**: Current height, recent blocks, transaction activity  
- **Mining Performance**: Hash rates, block production rates, difficulty
- **Consensus Network**: Peer connections, synchronization status
- **Mempool Activity**: Pending transactions, processing queues
- **System Metrics**: Memory usage, CPU utilization, goroutines

### Quick Start

```bash
# Start a blockchain node
./shadowy node --http-port=8080

# Start monitoring dashboard (in another terminal)
./shadowy monitor --port=9999

# Open browser to http://localhost:9999
```

### Advanced Usage

```bash
# Monitor remote node
./shadowy monitor --api-url http://remote-node:8080 --port 9999

# Custom refresh rate
./shadowy monitor --refresh 10  # 10 second intervals

# Custom port
./shadowy monitor --port 8888
```

## 🔧 Automated Testing Scripts

### Quick Monitor Setup

For development and quick testing:

```bash
# Starts node + monitor with health checks
./quick_monitor.sh

# Services:
# - Node: http://localhost:8080
# - Monitor: http://localhost:9999
```

### Comprehensive Burn-in Testing

For extensive validation and stress testing:

```bash
# Default 1-hour burn-in test
./burn_in_monitor.sh

# Custom duration (in seconds)
TEST_DURATION=7200 ./burn_in_monitor.sh  # 2 hours

# Custom ports
NODE_PORT=8081 MONITOR_PORT=9998 ./burn_in_monitor.sh
```

#### Burn-in Test Features

- **Automated Setup**: Starts node and monitoring automatically
- **Health Monitoring**: Continuous health checks every 5 seconds
- **Metrics Collection**: CSV logging of all key metrics
- **Progress Reporting**: Real-time status updates
- **Error Tracking**: Counts and reports failures
- **Final Report**: JSON summary with test results
- **Graceful Cleanup**: Proper shutdown on completion or interruption

#### Generated Files

The burn-in test creates:

```
burn_in_logs/
├── burn_in_results.json     # Final test summary
├── burn_in_stats.csv        # Time-series metrics
├── node.log                 # Blockchain node logs
└── monitor.log              # Web monitor logs
```

## 📈 Dashboard Pages

### Main Dashboard
- **URL**: `http://localhost:9999/`
- **Features**: Overview metrics, real-time charts, recent activity

### Health Monitor
- **URL**: `http://localhost:9999/health`
- **Features**: Detailed node health status, system diagnostics

### Mining Monitor  
- **URL**: `http://localhost:9999/mining`
- **Features**: Mining statistics, hash rate charts, difficulty tracking

### Consensus Monitor
- **URL**: `http://localhost:9999/consensus`
- **Features**: Network status, peer connections, sync progress

### Blocks Explorer
- **URL**: `http://localhost:9999/blocks`
- **Features**: Recent blocks list, block details, transaction counts

### Transactions Monitor
- **URL**: `http://localhost:9999/transactions`
- **Features**: Mempool status, pending transactions, fee analysis

## 🔌 API Endpoints

The monitoring dashboard exposes several API endpoints:

### Monitoring Data
```http
GET /api/monitoring
```
Returns comprehensive monitoring data aggregated from all sources.

### Individual Components
```http
GET /api/health         # Node health status
GET /api/mining         # Mining statistics  
GET /api/consensus      # Consensus and peer status
GET /api/blocks         # Recent blocks data
GET /api/transactions   # Mempool and transaction data
GET /api/metrics        # System resource metrics
```

## 🎯 Use Cases

### Development
```bash
# Start quick monitoring during development
./quick_monitor.sh

# Monitor while testing features
curl http://localhost:8080/api/v1/health
```

### Testing & Validation
```bash
# Run comprehensive burn-in test
./burn_in_monitor.sh

# Check results
cat burn_in_logs/burn_in_results.json
```

### Production Monitoring
```bash
# Long-running monitoring with external node
./shadowy monitor \
  --api-url http://production-node:8080 \
  --port 9999 \
  --refresh 30
```

### Multi-Node Testing
```bash
# Terminal 1: Start first node
./shadowy node --http-port=8080 --data-dir=node1_data

# Terminal 2: Start second node  
./shadowy node --http-port=8081 --data-dir=node2_data --peer=localhost:8080

# Terminal 3: Monitor first node
./shadowy monitor --api-url http://localhost:8080 --port=9999

# Terminal 4: Monitor second node
./shadowy monitor --api-url http://localhost:8081 --port=9998
```

## 📊 Metrics and Alerting

### Key Metrics Tracked

1. **Blockchain Health**
   - Block height progression
   - Block production rate (target: 6 blocks/hour)
   - Chain synchronization status

2. **Mining Performance**  
   - Hash rate and difficulty
   - Block discovery times
   - Mining efficiency

3. **Network Health**
   - Peer connection count
   - Consensus synchronization
   - Network partition detection

4. **System Resources**
   - Memory usage and garbage collection
   - CPU utilization
   - Goroutine counts
   - Disk I/O patterns

5. **Transaction Processing**
   - Mempool size and age
   - Transaction throughput
   - Fee distribution

### Performance Targets

| Metric | Target | Warning Threshold | Critical Threshold |
|--------|--------|------------------|-------------------|
| Block Rate | 6 blocks/hour | < 4 blocks/hour | < 2 blocks/hour |
| Peer Count | ≥ 1 peer | 0 peers | N/A |
| Memory Usage | < 500MB | > 1GB | > 2GB |
| Error Rate | 0% | > 1% | > 5% |
| API Response | < 100ms | > 500ms | > 2s |

## 🔧 Configuration

### Environment Variables

```bash
# Node configuration
export NODE_PORT=8080
export NODE_DATA_DIR="./data"

# Monitor configuration  
export MONITOR_PORT=9999
export MONITOR_REFRESH=5

# Test configuration
export TEST_DURATION=3600  # 1 hour
export LOG_LEVEL=info
```

### Command Line Options

```bash
# Node options
./shadowy node \
  --http-port=8080 \
  --grpc-port=8888 \
  --data-dir=./data \
  --log-level=debug

# Monitor options
./shadowy monitor \
  --port=9999 \
  --api-url=http://localhost:8080 \
  --refresh=5
```

## 🚨 Troubleshooting

### Common Issues

**Monitor can't connect to node**
```bash
# Check if node is running
curl http://localhost:8080/api/v1/health

# Check node logs  
tail -f node.log

# Restart with explicit ports
./shadowy node --http-port=8080
./shadowy monitor --api-url=http://localhost:8080
```

**Dashboard shows no data**
```bash
# Verify API endpoints
curl http://localhost:8080/api/v1/blockchain
curl http://localhost:8080/api/v1/mining

# Check monitor logs
tail -f monitor.log
```

**Burn-in test fails**
```bash
# Check detailed results
cat burn_in_logs/burn_in_results.json

# Review error patterns
grep -i error burn_in_logs/burn_in_stats.csv

# Analyze node logs
tail -f burn_in_logs/node.log
```

### Performance Tuning

**High memory usage**
- Increase cleanup frequency
- Reduce block retention
- Monitor goroutine counts

**Slow API responses**
- Check database performance
- Reduce concurrent requests
- Optimize query patterns

**Network connectivity issues**
- Verify firewall settings
- Check peer discovery
- Monitor network latency

## 🔮 Future Enhancements

### Planned Features

1. **Real-time Alerts**
   - Email/Slack notifications
   - Threshold-based alerting
   - Custom alert rules

2. **Advanced Analytics**
   - Historical trend analysis
   - Performance predictions
   - Anomaly detection

3. **Integration Options**
   - Prometheus metrics export
   - Grafana dashboard templates
   - ELK stack compatibility

4. **Mobile Support**
   - Responsive design improvements
   - Mobile-optimized views
   - Push notifications

### Contributing

To add new monitoring features:

1. **Backend Metrics**: Add to `monitor_api.go`
2. **Dashboard Pages**: Add to `monitor_pages.go`  
3. **API Endpoints**: Add to `monitor_web.go`
4. **Test Scripts**: Update burn-in test scripts

## 📚 Related Documentation

- [API Documentation](./docs/README.md)
- [Development Guide](./DEVELOPMENT.md)
- [Build and Release](./BUILD_AND_RELEASE.md)
- [Farming Guide](./FARMING_API.md)