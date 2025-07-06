# Challenge/Response System

## Overview

The Shadowy proof-of-storage system uses a difficulty-based challenge/response mechanism. Challenges require finding a key with a SHAKE128 identifier that starts with a specific number of zero bits, proving computational work over the stored keyspace.

## Challenge Format

```
{difficulty}:{challenge_data_hex}
```

### Components

- **difficulty**: Number of leading zero bits required (1-64)
- **challenge_data_hex**: 64-character hex string (32 bytes) of random data

### Examples

```
8:deadbeef1234567890abcdef0123456789abcdef0123456789abcdef01234567
16:cafebabe1234567890abcdef0123456789abcdef0123456789abcdef01234567
```

## Challenge Generation

```bash
shadowy challenge 8
```

### Output
```
Challenge: 8:deadbeef1234567890abcdef0123456789abcdef0123456789abcdef01234567
Difficulty: 8 bits
Target: identifier must start with 8 zero bits
```

## Difficulty Levels

| Difficulty | Zero Bits | Probability | Expected Keys Needed |
|------------|-----------|-------------|---------------------|
| 4 | 4 | 1/16 | 16 |
| 8 | 8 | 1/256 | 256 |
| 12 | 12 | 1/4096 | 4,096 |
| 16 | 16 | 1/65536 | 65,536 |
| 20 | 20 | 1/1048576 | 1,048,576 |

## Proof Generation

```bash
shadowy prove umbra_v1_k10_20250701-150430_a1b2c3d4.dat "8:deadbeef..."
```

### Process

1. **Parse Challenge**: Extract difficulty and challenge data
2. **Search Plot**: Scan all identifiers for required zero bit pattern
3. **Load Key**: Retrieve private key for matching identifier
4. **Sign Challenge**: Create ML-DSA-87 signature of challenge data
5. **Format Proof**: Encode complete proof response

### Proof Format

```
{challenge}|{public_key_hex}|{address_hex}|{identifier_hex}|{signature_hex}
```

### Example Proof
```
8:deadbeef...|0123456789abcdef...|0x1234567890abcdef...|00ab1234567890ab...|abcdef0123456789...
```

## Proof Verification

```bash
shadowy verify "8:deadbeef...|0123456789abcdef...|..."
```

### Verification Steps

1. **Parse Proof**: Decode all components
2. **Validate Public Key**: Ensure public key generates claimed address/identifier
3. **Check Difficulty**: Verify identifier meets zero bit requirement
4. **Verify Signature**: Validate ML-DSA-87 signature against challenge data

### Verification Output
```
Verifying proof with difficulty 8
✓ Address verification passed
✓ Identifier verification passed  
✓ Difficulty requirement met (8 zero bits)
✓ Signature verification passed
Proof is valid!
```

## Algorithm Details

### Zero Bit Counting

```go
func checkDifficulty(identifier [16]byte, difficulty int) bool {
    zeroBits := 0
    for _, b := range identifier {
        if b == 0 {
            zeroBits += 8
        } else {
            // Count leading zeros in this byte
            for i := 7; i >= 0; i-- {
                if (b>>i)&1 == 0 {
                    zeroBits++
                } else {
                    break
                }
            }
            break
        }
        if zeroBits >= difficulty {
            break
        }
    }
    return zeroBits >= difficulty
}
```

### Search Strategy

1. **Linear Scan**: Check each identifier in plot sequentially
2. **Early Exit**: Stop on first match (any valid key suffices)
3. **Progress Display**: Show search progress for large plots

## Security Properties

### Proof of Work

- Finding a matching key requires scanning stored keyspace
- Cannot be precomputed without access to specific plot
- Difficulty scales exponentially with zero bit requirement

### Proof of Storage

- Must have access to actual private keys to generate valid signatures
- Cannot forge proofs without corresponding key material
- Challenge data prevents replay attacks

### Post-Quantum Security

- ML-DSA-87 signatures resist quantum attacks
- SHAKE128 identifiers provide quantum-resistant hashing
- No classical or quantum shortcuts to finding matches

## Performance Considerations

### Search Time

- **Linear in plot size**: O(n) where n = number of keys
- **Early termination**: Average case significantly better than worst case
- **I/O bound**: Limited by storage read speed for large plots

### Recommended Difficulties

| Plot Size (K) | Recommended Difficulty | Success Probability |
|---------------|----------------------|-------------------|
| 5 (32 keys) | 4-6 bits | ~50-95% |
| 10 (1024 keys) | 8-10 bits | ~75-95% |
| 15 (32K keys) | 12-15 bits | ~85-99% |
| 20 (1M keys) | 16-20 bits | ~95-99.9% |

## Integration Examples

### Mining/Validation Node

```bash
# Generate challenge
CHALLENGE=$(shadowy challenge 12)

# Request proof from storage node
# ... network communication ...

# Verify received proof
shadowy verify "$PROOF" && echo "Valid proof received"
```

### Storage Provider

```bash
# Monitor for challenges
# ... challenge received via network ...

# Generate proof
PROOF=$(shadowy prove /storage/plots/umbra_v1_k15_*.dat "$CHALLENGE")

# Submit proof
# ... network communication ...
```