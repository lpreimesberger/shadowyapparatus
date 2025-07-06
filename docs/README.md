# Shadowy Documentation

Welcome to the Shadowy proof-of-storage cryptocurrency documentation!

## Quick Navigation

### Core Documentation
- **[Plot Format Specification](plot-format.md)** - Technical details of the umbra plot file format
- **[Challenge/Response System](challenge-response.md)** - How the proof-of-storage mechanism works
- **[Usage Examples](examples.md)** - Practical examples and scripts

### Getting Started

1. **Generate your first plot:**
   ```bash
   shadowy plot ./plots -k 5
   ```

2. **Verify plot integrity:**
   ```bash
   shadowy verifyplot ./plots/umbra_v1_k5_*.dat
   ```

3. **Test challenge/response:**
   ```bash
   shadowy challenge 4
   shadowy prove ./plots/umbra_v1_k5_*.dat "4:deadbeef..."
   ```

## Key Features

### 🔐 Post-Quantum Security
- **ML-DSA-87**: NIST-standardized post-quantum signatures
- **CNSA 2.0 Compliant**: 256-bit equivalent security
- **Quantum-Resistant**: Secure against both classical and quantum attacks

### 🌑 Umbra Plot Format
- **Efficient Storage**: Packed binary format with minimal overhead
- **Fast Searching**: SHAKE128 identifiers for quick challenge matching
- **Scalable**: Supports plots from 32 keys (K=5) to 1M+ keys (K=20+)

### ⚡ Performance Optimized
- **Linear Search**: O(n) challenge response time
- **Early Termination**: Stops on first valid proof found
- **Progress Tracking**: Real-time feedback for long operations

### 🛠️ Production Ready
- **Automatic Naming**: Timestamp-based unique filenames
- **Integrity Checking**: Built-in plot verification
- **Error Handling**: Comprehensive error reporting

## File Format Overview

### Umbra Plot Files
```
umbra_v1_k{K}_{TIMESTAMP}_{RANDOM}.dat
```
- **Binary format** with header + packed private keys
- **Self-contained** - no external dependencies
- **Version tracked** for future compatibility

### Challenge Format
```
{difficulty}:{challenge_data_hex}
```
- **Difficulty**: Number of required leading zero bits
- **Challenge data**: 32 bytes of random data to sign

### Proof Format
```
{challenge}|{public_key}|{address}|{identifier}|{signature}
```
- **Complete proof** with all verification data
- **Self-validating** - can be verified independently

## Architecture

```
[Challenge Generator] → [Storage Node] → [Proof Verifier]
                          ↓
                      [Umbra Plots]
                    (ML-DSA-87 Keys)
```

1. **Challenge Generation**: Random difficulty-based challenges
2. **Plot Storage**: Large collections of post-quantum key pairs
3. **Proof Generation**: Find matching key and sign challenge
4. **Proof Verification**: Validate signatures and difficulty

## Use Cases

### Storage Providers
- Generate large plots to prove storage capacity
- Respond to network challenges with cryptographic proofs
- Earn rewards based on proven storage

### Network Validators
- Issue challenges to storage providers
- Verify incoming proofs for correctness
- Maintain network consensus on storage claims

### Researchers
- Study post-quantum cryptography in practice
- Analyze proof-of-storage economics
- Develop improved storage protocols

## Technical Specifications

### Cryptographic Primitives
- **Signatures**: ML-DSA-87 (FIPS 204)
- **Hashing**: SHAKE128 (FIPS 202)
- **Key Generation**: FIPS 140-2 Level 3 entropy

### File Formats
- **Encoding**: Little-endian binary
- **Alignment**: Natural alignment for all fields
- **Platform**: Cross-platform compatible

### Performance Characteristics
- **Plot Generation**: ~1000-5000 keys/second
- **Challenge Response**: Linear in plot size
- **Proof Verification**: <1ms per proof

## Security Model

### Threat Model
- **Quantum Adversary**: Resistant to quantum attacks
- **Storage Cheating**: Cannot fake proofs without actual storage
- **Replay Attacks**: Challenge randomness prevents replay

### Assumptions
- **Secure Random Generation**: High-quality entropy source
- **Honest Majority**: Network validators act honestly
- **Network Timing**: Reasonable bounds on challenge/response timing

## Development Status

- ✅ **Core Implementation**: Complete
- ✅ **Post-Quantum Migration**: ML-DSA-87 integrated
- ✅ **File Format**: Stable v1 specification
- ✅ **Documentation**: Comprehensive coverage
- 🚧 **Network Protocol**: Future work
- 🚧 **Economic Model**: Future work

## Contributing

This is a research/educational project demonstrating post-quantum proof-of-storage concepts. The codebase serves as a reference implementation for:

- Post-quantum cryptography integration
- Efficient binary file formats
- Challenge/response protocols
- Storage verification mechanisms

## License

See the main repository for license information.

---

*Shadowy: Post-quantum proof-of-storage for the shadow economy* 🌑