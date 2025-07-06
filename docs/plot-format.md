# Umbra Plot File Format Specification

## Overview

Umbra plots are the core storage format for the Shadowy proof-of-storage cryptocurrency. Each plot contains a collection of ML-DSA-87 (post-quantum) key pairs with their corresponding addresses and SHAKE128 identifiers for efficient searching.

## File Naming Convention

```
umbra_v1_k{K}_{TIMESTAMP}_{RANDOM}.dat
```

### Components

- **umbra**: File type identifier
- **v1**: Format version (currently 1)
- **k{K}**: Plot size parameter (e.g., k10 = 2^10 = 1024 keys)
- **{TIMESTAMP}**: UTC timestamp in format `YYYYMMDD-HHMMSS`
- **{RANDOM}**: 8-character hex string for uniqueness
- **.dat**: Binary data file extension

### Examples

- `umbra_v1_k10_20250701-150430_a1b2c3d4.dat` - 1024 keys created at 15:04:30 UTC
- `umbra_v1_k5_20250701-150431_e5f6a7b8.dat` - 32 keys created at 15:04:31 UTC

## Binary File Structure

### File Layout

```
[Header][Private Keys]
```

### Header Format

| Field | Type | Size | Description |
|-------|------|------|-------------|
| Version | int64 | 8 bytes | Format version (currently 1) |
| K | int32 | 4 bytes | Plot size parameter |
| Entry Count | int32 | 4 bytes | Number of key entries |
| Entries | AddressOffsetPair[] | Variable | Array of address/offset pairs |

### AddressOffsetPair Format

| Field | Type | Size | Description |
|-------|------|------|-------------|
| Address | byte[20] | 20 bytes | Ethereum-style address (Keccak256 hash) |
| Identifier | byte[16] | 16 bytes | SHAKE128 identifier for matching |
| Offset | int32 | 4 bytes | Byte offset to private key in file |

### Private Key Section

- **Location**: Immediately after header
- **Format**: Packed ML-DSA-87 private keys
- **Size**: 4896 bytes per key
- **Order**: Sequential, referenced by offset in header entries

## Key Generation

### ML-DSA-87 Parameters

- **Private Key Size**: 4896 bytes
- **Public Key Size**: 2592 bytes  
- **Signature Size**: 4627 bytes
- **Security Level**: 256-bit equivalent (CNSA 2.0 compliant)

### Address Generation

1. Generate ML-DSA-87 key pair
2. Hash public key with Keccak256
3. Take last 20 bytes as Ethereum-style address

### Identifier Generation

1. Hash public key with SHAKE128
2. Extract first 16 bytes as identifier
3. Used for fast challenge matching

## Plot Sizes

| K Value | Key Count | Private Key Data | Typical File Size |
|---------|-----------|------------------|-------------------|
| 5 | 32 | ~154 KB | ~158 KB |
| 10 | 1024 | ~4.8 MB | ~5.0 MB |
| 15 | 32768 | ~157 MB | ~158 MB |
| 20 | 1048576 | ~5.0 GB | ~5.0 GB |

## File Validation

Use the `verifyplot` command to validate plot file integrity:

```bash
shadowy verifyplot umbra_v1_k10_20250701-150430_a1b2c3d4.dat
```

### Validation Checks

1. **Header integrity**: Version, K value, entry count
2. **Offset consistency**: All offsets point to valid locations
3. **Key consistency**: Private keys generate claimed addresses/identifiers
4. **File size**: Total size matches expected header + key data

## Technical Notes

- All integers use little-endian encoding
- File format is platform-independent
- Version 1 is the current stable format
- Future versions may extend the header for new features

## Security Considerations

- Private keys use cryptographically secure random generation
- SHAKE128 identifiers provide 128-bit collision resistance
- ML-DSA-87 provides quantum-resistant signatures
- File format includes no sensitive metadata beyond key material