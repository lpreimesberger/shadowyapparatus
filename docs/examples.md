# Shadowy Usage Examples

## Quick Start

### 1. Generate Your First Plot

```bash
# Create a small test plot (32 keys)
shadowy plot ./plots -k 5

# Output:
# Generating 32 ML-DSA-87 keys...
# Creating plot with k=5 (32 keys) in ./plots/umbra_v1_k5_20250701-150430_a1b2c3d4.dat
# Plot created successfully (total size: 157968 bytes)
```

### 2. Verify Plot Integrity

```bash
# Verify the plot was created correctly
shadowy verifyplot ./plots/umbra_v1_k5_20250701-150430_a1b2c3d4.dat

# Output:
# Header validation passed: version=1, k=5, entries=32
# Validated 32/32 keys (100.0%)
# Key consistency validation passed
# File size validation passed
# Plot file ./plots/umbra_v1_k5_20250701-150430_a1b2c3d4.dat is valid
```

### 3. Test Challenge/Response

```bash
# Generate a challenge
shadowy challenge 4

# Output:
# Challenge: 4:deadbeef1234567890abcdef0123456789abcdef0123456789abcdef01234567
# Difficulty: 4 bits
# Target: identifier must start with 4 zero bits

# Generate proof for the challenge
shadowy prove ./plots/umbra_v1_k5_20250701-150430_a1b2c3d4.dat \
  "4:deadbeef1234567890abcdef0123456789abcdef0123456789abcdef01234567"

# Output:
# Searching 32 keys for difficulty 4 challenge...
# Found matching key! Identifier: 0012ab34567890cd...
# Proof: 4:deadbeef...|03a1b2c3d4e5f6...|0x789abc123def...|0012ab34567890cd...|1a2b3c4d5e6f...

# Verify the proof
shadowy verify "4:deadbeef...|03a1b2c3d4e5f6...|0x789abc123def...|0012ab34567890cd...|1a2b3c4d5e6f..."

# Output:
# Verifying proof with difficulty 4
# ✓ Address verification passed
# ✓ Identifier verification passed
# ✓ Difficulty requirement met (4 zero bits)
# ✓ Signature verification passed
# Proof is valid!
```

## Production Scenarios

### Storage Farm Setup

```bash
#!/bin/bash
# create-farm.sh - Generate multiple plots for storage farming

PLOT_DIR="/storage/shadowy/plots"
mkdir -p "$PLOT_DIR"

# Generate 10 medium-sized plots (1024 keys each)
for i in {1..10}; do
    echo "Creating plot $i/10..."
    shadowy plot "$PLOT_DIR" -k 10
    
    # Verify immediately after creation
    LATEST_PLOT=$(ls -t "$PLOT_DIR"/umbra_v1_k10_*.dat | head -1)
    shadowy verifyplot "$LATEST_PLOT" || {
        echo "ERROR: Plot $LATEST_PLOT failed verification!"
        exit 1
    }
    
    echo "Plot $i completed and verified"
done

echo "Farm setup complete: $(ls "$PLOT_DIR"/*.dat | wc -l) plots created"
```

### Mining Node Integration

```bash
#!/bin/bash
# mine.sh - Simplified mining loop

PLOT_DIR="/storage/shadowy/plots"
DIFFICULTY=12

while true; do
    # Generate new challenge
    CHALLENGE=$(shadowy challenge $DIFFICULTY)
    echo "New challenge: $CHALLENGE"
    
    # Try each plot until we find a proof
    PROOF_FOUND=false
    for PLOT in "$PLOT_DIR"/umbra_v1_k*.dat; do
        echo "Trying plot: $(basename "$PLOT")"
        
        if PROOF=$(shadowy prove "$PLOT" "$CHALLENGE" 2>/dev/null); then
            echo "✓ Proof found in $(basename "$PLOT")!"
            echo "Proof: $PROOF"
            
            # Verify our own proof
            if shadowy verify "$PROOF" >/dev/null 2>&1; then
                echo "✓ Proof verified successfully"
                PROOF_FOUND=true
                break
            else
                echo "✗ Proof verification failed"
            fi
        fi
    done
    
    if [ "$PROOF_FOUND" = false ]; then
        echo "✗ No proof found in any plot for difficulty $DIFFICULTY"
    fi
    
    echo "---"
    sleep 1
done
```

### Plot Management

```bash
#!/bin/bash
# manage-plots.sh - Plot maintenance utilities

PLOT_DIR="/storage/shadowy/plots"

list_plots() {
    echo "=== Plot Inventory ==="
    ls -lh "$PLOT_DIR"/umbra_v1_*.dat | while read -r perms links owner group size date time file; do
        BASENAME=$(basename "$file")
        K_VALUE=$(echo "$BASENAME" | sed 's/.*_k\([0-9]\+\)_.*/\1/')
        KEY_COUNT=$((2**K_VALUE))
        echo "$(printf "%-50s" "$BASENAME") | K=$K_VALUE ($KEY_COUNT keys) | $size"
    done
}

verify_all() {
    echo "=== Verifying All Plots ==="
    TOTAL=0
    VALID=0
    
    for PLOT in "$PLOT_DIR"/umbra_v1_*.dat; do
        ((TOTAL++))
        echo -n "Checking $(basename "$PLOT")... "
        
        if shadowy verifyplot "$PLOT" >/dev/null 2>&1; then
            echo "✓ VALID"
            ((VALID++))
        else
            echo "✗ INVALID"
        fi
    done
    
    echo "Results: $VALID/$TOTAL plots valid"
}

cleanup_old() {
    echo "=== Cleaning Up Old Plots ==="
    # Remove plots older than 30 days
    find "$PLOT_DIR" -name "umbra_v1_*.dat" -mtime +30 -exec rm -v {} \;
}

case "$1" in
    list) list_plots ;;
    verify) verify_all ;;
    cleanup) cleanup_old ;;
    *) echo "Usage: $0 {list|verify|cleanup}" ;;
esac
```

## Development Testing

### Unit Test Plot Generation

```bash
#!/bin/bash
# test-plots.sh - Automated testing suite

test_small_plot() {
    echo "Testing small plot generation..."
    PLOT=$(shadowy plot /tmp -k 3 2>&1 | grep "Creating plot" | cut -d' ' -f10)
    
    if [ -f "$PLOT" ]; then
        echo "✓ Plot created: $PLOT"
        
        if shadowy verifyplot "$PLOT" >/dev/null 2>&1; then
            echo "✓ Plot verification passed"
        else
            echo "✗ Plot verification failed"
            return 1
        fi
        
        rm "$PLOT"
    else
        echo "✗ Plot file not found"
        return 1
    fi
}

test_challenge_response() {
    echo "Testing challenge/response system..."
    
    # Create temporary plot
    PLOT=$(shadowy plot /tmp -k 4 2>&1 | grep "Creating plot" | cut -d' ' -f10)
    
    # Generate challenge
    CHALLENGE=$(shadowy challenge 2)
    
    # Generate proof
    if PROOF=$(shadowy prove "$PLOT" "$CHALLENGE" 2>/dev/null); then
        echo "✓ Proof generated"
        
        if shadowy verify "$PROOF" >/dev/null 2>&1; then
            echo "✓ Proof verification passed"
        else
            echo "✗ Proof verification failed"
            rm "$PLOT"
            return 1
        fi
    else
        echo "✗ Proof generation failed (expected for low difficulty)"
    fi
    
    rm "$PLOT"
}

test_filename_uniqueness() {
    echo "Testing filename uniqueness..."
    
    PLOT1=$(shadowy plot /tmp -k 3 2>&1 | grep "Creating plot" | cut -d' ' -f10)
    sleep 1
    PLOT2=$(shadowy plot /tmp -k 3 2>&1 | grep "Creating plot" | cut -d' ' -f10)
    
    if [ "$PLOT1" != "$PLOT2" ]; then
        echo "✓ Unique filenames generated"
        rm "$PLOT1" "$PLOT2"
    else
        echo "✗ Filename collision detected"
        rm "$PLOT1" "$PLOT2" 2>/dev/null
        return 1
    fi
}

echo "=== Shadowy Test Suite ==="
test_small_plot && \
test_challenge_response && \
test_filename_uniqueness && \
echo "✓ All tests passed!" || \
echo "✗ Some tests failed!"
```

## Performance Benchmarking

### Plot Generation Speed

```bash
#!/bin/bash
# benchmark.sh - Performance testing

echo "=== Plot Generation Benchmark ==="

for K in 3 5 8 10; do
    echo "Benchmarking K=$K..."
    
    START_TIME=$(date +%s.%N)
    PLOT=$(shadowy plot /tmp -k $K 2>&1 | grep "Creating plot" | cut -d' ' -f10)
    END_TIME=$(date +%s.%N)
    
    DURATION=$(echo "$END_TIME - $START_TIME" | bc)
    KEY_COUNT=$((2**K))
    KEYS_PER_SEC=$(echo "scale=2; $KEY_COUNT / $DURATION" | bc)
    
    echo "  K=$K: $KEY_COUNT keys in ${DURATION}s (${KEYS_PER_SEC} keys/sec)"
    
    rm "$PLOT"
done
```

## Troubleshooting

### Common Issues

```bash
# Issue: Plot verification fails
shadowy verifyplot problematic_plot.dat
# Look for specific error messages in output

# Issue: No proof found for challenge
# Try lower difficulty or larger plot
shadowy challenge 2  # Easier challenge
shadowy plot /plots -k 12  # More keys

# Issue: Signature verification fails
# Check if plot file is corrupted
file suspicious_plot.dat
hexdump -C suspicious_plot.dat | head

# Issue: Out of disk space
# Check plot sizes before generation
echo "K=10 will use ~5MB, K=15 will use ~158MB, K=20 will use ~5GB"
```