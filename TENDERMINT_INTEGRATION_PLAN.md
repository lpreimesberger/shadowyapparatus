# Tendermint Integration Plan for Shadowy Blockchain

## Overview
Replace the custom consensus engine with Tendermint Core to solve persistent sync and consensus issues. This will provide battle-tested BFT consensus while preserving ML-DSA-87 post-quantum cryptography.

## Current Architecture Analysis

### Issues with Custom Consensus
1. **Complex Sync Logic**: Multiple sync modes (SyncFirst, performSync, sequential sync)
2. **Fork Resolution**: Complex chain reorganization and block trimming
3. **P2P Management**: Manual peer discovery and connection handling
4. **Consensus Failures**: Nodes spinning without reaching consensus (HTTP 503)
5. **State Management**: Complex nextExpectedHeight tracking and pending block buffers

### Components to Replace
- `ConsensusEngine` (cmd/consensus.go)
- `consensus_handlers.go` - P2P message handling
- Custom sync logic and fork resolution
- Peer discovery and management
- Block propagation

### Components to Preserve
- `Blockchain` storage layer
- `Mempool` transaction management  
- `Miner` (converted to block proposer)
- ML-DSA-87 signature validation
- Transaction and block structures
- WASM client architecture

## Integration Architecture

### 1. ABCI Application Layer
Create `ShadowyABCIApp` implementing Tendermint's ABCI interface:

```go
type ShadowyABCIApp struct {
    blockchain *Blockchain
    mempool    *Mempool  
    state      *AppState
    logger     log.Logger
}

// ABCI Methods
func (app *ShadowyABCIApp) BeginBlock(req types.RequestBeginBlock) types.ResponseBeginBlock
func (app *ShadowyABCIApp) DeliverTx(req types.RequestDeliverTx) types.ResponseDeliverTx  
func (app *ShadowyABCIApp) EndBlock(req types.RequestEndBlock) types.ResponseEndBlock
func (app *ShadowyABCIApp) Commit() types.ResponseCommit
func (app *ShadowyABCIApp) CheckTx(req types.RequestCheckTx) types.ResponseCheckTx
```

### 2. Transaction Handling
- **CheckTx**: Validate transactions before mempool (ML-DSA-87 signature verification)
- **DeliverTx**: Execute transactions and update state
- **Mempool Integration**: Bridge Tendermint mempool with existing Shadowy mempool

### 3. Block Processing
- **BeginBlock**: Initialize block processing, handle validator updates
- **EndBlock**: Finalize block, emit events
- **Commit**: Apply state changes and return app hash

### 4. State Management
Replace complex sync state with Tendermint's built-in state machine:
- App state hash for consensus
- Deterministic state transitions
- Automatic rollback on consensus failures

## Implementation Steps

### Phase 1: ABCI Application Foundation
1. Create `abci/` directory structure
2. Implement basic ABCI application
3. Integrate with existing Blockchain storage
4. Add ML-DSA-87 transaction validation

### Phase 2: Transaction Bridge
1. Convert Shadowy transactions to ABCI format
2. Implement CheckTx with ML-DSA-87 validation
3. Bridge mempool operations
4. Add transaction indexing

### Phase 3: Block Integration  
1. Convert Shadowy blocks to Tendermint blocks
2. Implement block execution logic
3. Add state hash computation
4. Integrate with existing storage

### Phase 4: Configuration & Deployment
1. Create Tendermint configuration
2. Set up validator key management
3. Configure P2P networking
4. Add monitoring and logging

### Phase 5: Migration & Testing
1. Create migration tools
2. Test with multiple validators
3. Benchmark performance
4. Deploy to testnet

## Key Benefits

### Immediate Fixes
- **Consensus Reliability**: No more spinning nodes or 503 errors
- **Fork Resolution**: Automatic through BFT consensus
- **Sync Simplicity**: Built-in state sync and block replay
- **P2P Robustness**: Battle-tested networking layer

### Long-term Advantages  
- **Scalability**: Proven to handle high transaction throughput
- **Security**: BFT consensus with 1/3 Byzantine fault tolerance
- **Ecosystem**: Access to Cosmos ecosystem tools
- **Upgrades**: Built-in governance and upgrade mechanisms

## Post-Quantum Compatibility

### ML-DSA-87 Integration
- Preserve existing signature validation in CheckTx
- Use ML-DSA-87 for validator keys if needed
- Maintain transaction format compatibility
- Keep WASM client architecture unchanged

### Client Impact
- WASM bridge continues to work unchanged
- Python CLI maintains same interface  
- HTTP API endpoints remain the same
- No changes to wallet or transaction signing

## Configuration Example

```toml
# config/config.toml
[consensus]
timeout_propose = "3s"
timeout_propose_delta = "500ms"
timeout_prevote = "1s"
timeout_prevote_delta = "500ms"
timeout_precommit = "1s"
timeout_precommit_delta = "500ms"
timeout_commit = "5s"
create_empty_blocks = true
create_empty_blocks_interval = "30s"

[p2p]
laddr = "tcp://0.0.0.0:26656"
external_address = ""
seeds = ""
persistent_peers = ""
max_num_inbound_peers = 40
max_num_outbound_peers = 10

[mempool]
size = 5000
cache_size = 10000
max_txs_bytes = 1073741824
max_tx_bytes = 1048576
```

## File Structure

```
tendermint/
├── abci/
│   ├── app.go              # Main ABCI application
│   ├── transactions.go     # Transaction processing
│   ├── blocks.go          # Block processing  
│   ├── state.go           # State management
│   └── validators.go      # Validator management
├── config/
│   ├── config.toml        # Tendermint configuration
│   └── genesis.json       # Genesis state
├── node/
│   ├── node.go           # Tendermint node wrapper
│   └── service.go        # Node service management
└── migration/
    ├── migrate.go        # Migration utilities
    └── export.go         # State export tools
```

## Migration Strategy

### 1. Parallel Development
- Develop Tendermint integration alongside existing system
- Use feature flags to switch between implementations
- Gradual rollout to test nodes first

### 2. State Migration
- Export current blockchain state
- Create Tendermint genesis with existing state
- Verify state consistency across implementations

### 3. Network Transition
- Start with single Tendermint validator
- Add validators gradually
- Maintain backward compatibility during transition

## Testing Plan

### 1. Unit Tests
- ABCI method implementations
- Transaction validation
- State transitions

### 2. Integration Tests  
- Multi-validator consensus
- Network partitions
- Byzantine fault scenarios

### 3. Performance Tests
- Transaction throughput
- Block time consistency
- Memory and CPU usage

### 4. Migration Tests
- State export/import
- Network transition
- Rollback procedures

## Rollback Plan

### Risk Mitigation
- Keep custom consensus as fallback
- Staged deployment with monitoring
- Automated health checks
- Quick rollback procedures

### Monitoring
- Consensus participation
- Block production timing
- Network connectivity
- State consistency

This integration will solve the persistent consensus issues while maintaining all existing functionality and post-quantum security features.