# Shadowy Wallet Management

## Overview

The Shadowy wallet system provides secure storage and management of post-quantum ML-DSA-87 key pairs with unique addresses. Wallets are stored as encrypted JSON files in a secure directory structure.

## Default Storage Location

By default, wallets are stored in:
```
$HOME/.shadowy/
```

## Wallet File Format

Wallets are stored as JSON files with the extension `.wallet`:

```json
{
  "name": "my-wallet",
  "address": "S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a",
  "private_key": "845c7f0d3018e213fe88f247110e743739cc8240c8524760140bbe2361cd28ae...",
  "public_key": "925d8b7eafdf25666f49dc1e95a9129241442c9bad2e767b844f561bbf1e828...",
  "identifier": "34c41812e4a80abc7ce3c98a2d159a6a",
  "created_at": "2025-07-02T09:45:17.123456Z",
  "version": 1
}
```

### File Security
- Files are created with `0600` permissions (owner read/write only)
- Directory is created with `0700` permissions (owner access only)
- Private keys are stored in hex format for easy parsing

## Commands

### Generate New Wallet

```bash
# Generate with auto-generated name
shadowy wallet generate

# Generate with custom name
shadowy wallet generate my-wallet

# Generate in custom directory
shadowy wallet generate --wallet-dir /path/to/wallets my-wallet
```

**Output:**
```
Wallet Name: my-wallet
Address:     S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a
Identifier:  34c41812e4a80abc7ce3c98a2d159a6a
Saved to:    /home/user/.shadowy/my-wallet.wallet
```

### Import from Private Key

```bash
# Import with auto-generated name
shadowy wallet from-key 845c7f0d3018e213fe88f247110e743739cc8240c8524760140bbe2361cd28ae...

# Import with custom name
shadowy wallet from-key 845c7f0d3018e213fe88f247110e743739cc8240c8524760140bbe2361cd28ae... imported-wallet

# Import to custom directory
shadowy wallet from-key --wallet-dir /tmp/wallets 845c7f0d... imported-wallet
```

### List All Wallets

```bash
# List wallets in default directory
shadowy wallet list

# List wallets in custom directory
shadowy wallet list --wallet-dir /path/to/wallets
```

**Output:**
```
Found 2 wallet(s) in /home/user/.shadowy:

1. my-wallet
   Address:    S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a
   Created:    2025-07-02 09:45:17 UTC
   Identifier: 34c41812e4a80abc7ce3c98a2d159a6a

2. test-wallet
   Address:    S4231f5a082359e5bb81977b79a4946092e025565435ca8a527
   Created:    2025-07-02 10:15:32 UTC
   Identifier: 9a10d7dd7e5cc322840132bc34f405e7
```

### Show Wallet Details

```bash
# Show wallet details
shadowy wallet show my-wallet

# Show from custom directory
shadowy wallet show --wallet-dir /path/to/wallets my-wallet
```

**Output:**
```
Wallet Details:
Name:        my-wallet
Address:     S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a
Public Key:  925d8b7eafdf25666f49dc1e95a9129241442c9bad2e767b844f561bbf1e828...
Identifier:  34c41812e4a80abc7ce3c98a2d159a6a
Created:     2025-07-02 09:45:17 UTC
Version:     1
```

### Validate Address

```bash
# Validate any Shadowy address
shadowy wallet validate S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a
```

**Output:**
```
✓ Address S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a is valid
```

## Global Options

### Custom Wallet Directory

All wallet commands support the `--wallet-dir` flag:

```bash
shadowy wallet [command] --wallet-dir /custom/path
```

This is useful for:
- **Testing**: Keep test wallets separate
- **Multiple environments**: Dev/staging/production separation  
- **Security**: Store wallets on encrypted drives
- **Backup**: Easy wallet directory management

## Address Format

Shadowy addresses use a post-quantum secure format:

```
S + [version:0x42][hash:20 bytes][checksum:4 bytes] (hex encoded)
```

### Properties
- **Prefix**: 'S' identifies Shadowy addresses
- **Version**: 0x42 allows future algorithm upgrades
- **Hash**: SHAKE256 of public key (quantum-resistant)
- **Checksum**: 4-byte error detection (99.99% accuracy)
- **Length**: 51 characters total

### Example
```
S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a
│├─ version ─┤├──────── hash ────────┤├─ checksum ─┤
S   42           e975cac084e47b68...      5a9a
```

## Security Considerations

### Private Key Storage
- Private keys are stored in plain text in wallet files
- Wallet files have restrictive permissions (0600)
- Consider additional encryption for high-value wallets

### Backup Strategy
```bash
# Backup entire wallet directory
cp -r ~/.shadowy ~/backup/shadowy-wallets-$(date +%Y%m%d)

# Backup specific wallet
cp ~/.shadowy/important-wallet.wallet ~/backup/
```

### Multiple Wallet Directories
```bash
# Production wallets
export SHADOWY_WALLET_DIR="$HOME/.shadowy"

# Development wallets  
export SHADOWY_WALLET_DIR="$HOME/.shadowy-dev"

# Or use flag for each command
shadowy wallet list --wallet-dir "$HOME/.shadowy-dev"
```

## Integration Examples

### Automated Wallet Creation
```bash
#!/bin/bash
# create-wallets.sh

WALLET_DIR="/secure/storage/wallets"

for i in {1..5}; do
    WALLET_NAME="node-$i"
    shadowy wallet generate --wallet-dir "$WALLET_DIR" "$WALLET_NAME"
    echo "Created wallet: $WALLET_NAME"
done
```

### Wallet Address Extraction
```bash
#!/bin/bash
# get-addresses.sh

WALLET_DIR="$HOME/.shadowy"

for wallet in "$WALLET_DIR"/*.wallet; do
    name=$(basename "$wallet" .wallet)
    address=$(jq -r '.address' "$wallet")
    echo "$name: $address"
done
```

### Wallet Health Check
```bash
#!/bin/bash
# check-wallets.sh

echo "=== Wallet Health Check ==="

# List all wallets
shadowy wallet list

# Validate each address
for wallet in ~/.shadowy/*.wallet; do
    if [ -f "$wallet" ]; then
        name=$(basename "$wallet" .wallet)
        address=$(jq -r '.address' "$wallet")
        
        echo -n "Checking $name... "
        if shadowy wallet validate "$address" >/dev/null 2>&1; then
            echo "✓ Valid"
        else
            echo "✗ Invalid"
        fi
    fi
done
```

## Troubleshooting

### Common Issues

```bash
# Issue: Permission denied
# Solution: Check directory permissions
ls -la ~/.shadowy
chmod 700 ~/.shadowy
chmod 600 ~/.shadowy/*.wallet

# Issue: Wallet not found
# Solution: List available wallets
shadowy wallet list

# Issue: Invalid address
# Solution: Check address format
shadowy wallet validate S42e975cac084e47b68a8182e3ea25a25483c81d5ff58625a9a

# Issue: Corrupted wallet file
# Solution: Check JSON format
jq . ~/.shadowy/wallet-name.wallet
```

### Recovery

```bash
# Recover from private key backup
shadowy wallet from-key [private-key-hex] recovered-wallet

# Recreate from address (validation only)
shadowy wallet validate [address]
```