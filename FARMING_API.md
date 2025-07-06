# Farming Service API

The Shadowy farming service provides runtime interaction through HTTP API endpoints when running in node mode.

## Starting the Node with Farming

```bash
# Start the full node (farming enabled by default)
./shadowy node

# Start with custom ports
./shadowy node --http-port 8080 --grpc-port 9090

# Start with timelord enabled (resource intensive)
./shadowy node --enable-timelord
```

## Farming API Endpoints

Base URL: `http://localhost:8080/api/v1/farming`

### Get Farming Statistics
```bash
curl http://localhost:8080/api/v1/farming
```

Returns farming service statistics including:
- `plot_files_indexed`: Number of indexed plot files
- `total_keys`: Total number of keys in database
- `challenges_handled`: Number of storage challenges processed
- `average_response_time`: Average challenge response time
- `database_size`: Size of the lookup database

### Get Farming Status
```bash
curl http://localhost:8080/api/v1/farming/status
```

Returns current farming service status:
- `running`: Whether the farming service is active
- `stats`: Complete statistics object

### List Indexed Plot Files
```bash
curl http://localhost:8080/api/v1/farming/plots
```

Returns information about all indexed plot files:
- `file_path`: Full path to plot file
- `file_name`: Plot file name
- `key_count`: Number of keys indexed from this file
- `file_size`: Size of plot file in bytes
- `mod_time`: Last modification time

### Submit Storage Challenge
```bash
curl -X POST http://localhost:8080/api/v1/farming/challenge \
  -H "Content-Type: application/json" \
  -d '{
    "challenge": "dGVzdCBjaGFsbGVuZ2UgZGF0YQ==",
    "difficulty": 1
  }'
```

Submits a proof-of-storage challenge and returns a storage proof:
- `challenge_id`: Unique challenge identifier
- `plot_file`: Plot file containing the proof
- `offset`: Offset in plot file
- `private_key`: Private key at that offset
- `signature`: Cryptographic signature proof
- `valid`: Whether the proof is valid
- `response_time`: Time taken to generate proof

## Health Monitoring

### Node Health Check
```bash
curl http://localhost:8080/api/v1/health
```

Returns overall node health including farming service status.

### Node Status
```bash
curl http://localhost:8080/api/v1/status
```

Returns detailed node information including all enabled services.

## Example Usage

1. **Check if farming is running:**
```bash
curl -s http://localhost:8080/api/v1/farming/status | jq '.running'
```

2. **Get plot file count:**
```bash
curl -s http://localhost:8080/api/v1/farming/plots | jq '.count'
```

3. **Submit a test challenge:**
```bash
echo -n "test challenge" | base64 | \
  curl -X POST http://localhost:8080/api/v1/farming/challenge \
    -H "Content-Type: application/json" \
    -d @- <<< '{"challenge": "'$(cat)'", "difficulty": 1}'
```

4. **Monitor farming performance:**
```bash
watch -n 5 'curl -s http://localhost:8080/api/v1/farming | jq "{challenges: .challenges_handled, avg_time: .average_response_time, errors: .error_count}"'
```

## Integration Notes

- The farming service automatically starts when the node starts
- Plot files are indexed during startup
- The service maintains persistent BadgerDB lookups in the scratch directory
- All API responses are JSON formatted
- Error responses include descriptive messages
- The service is thread-safe and handles concurrent requests

## Troubleshooting

- **Service not available**: Check that farming is enabled in node config
- **No plot files**: Ensure plot directories are configured and contain `.dat` files
- **Database errors**: Check scratch directory permissions and disk space
- **Performance issues**: Monitor database size and consider plot file optimization