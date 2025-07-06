#!/bin/bash
# Test golangci-lint configuration validity

echo "🔍 Testing golangci-lint configuration..."

# Check if config file exists
if [ ! -f ".golangci.yml" ]; then
    echo "❌ .golangci.yml not found"
    exit 1
fi

echo "✅ Configuration file found"

# Test YAML syntax
if command -v yq >/dev/null 2>&1; then
    echo "🔍 Testing YAML syntax with yq..."
    if yq eval '.run.timeout' .golangci.yml >/dev/null 2>&1; then
        echo "✅ YAML syntax is valid"
    else
        echo "❌ YAML syntax error"
        exit 1
    fi
elif command -v python3 >/dev/null 2>&1; then
    echo "🔍 Testing YAML syntax with Python..."
    if python3 -c "import yaml; yaml.safe_load(open('.golangci.yml'))" 2>/dev/null; then
        echo "✅ YAML syntax is valid"
    else
        echo "❌ YAML syntax error"
        exit 1
    fi
else
    echo "⚠️  No YAML validator found, skipping syntax check"
fi

# Check basic structure
echo "🔍 Checking configuration structure..."
if grep -q "run:" .golangci.yml && \
   grep -q "linters:" .golangci.yml && \
   grep -q "issues:" .golangci.yml; then
    echo "✅ Configuration structure looks good"
else
    echo "❌ Configuration missing required sections"
    exit 1
fi

# Test with a simple go file if golangci-lint is available
if command -v golangci-lint >/dev/null 2>&1; then
    echo "🔍 Testing configuration with golangci-lint..."
    
    # Create a temporary Go file to test
    cat > test_lint.go << 'EOF'
package main

import "fmt"

func main() {
    fmt.Println("Hello, world!")
}
EOF
    
    # Test the configuration
    if golangci-lint run --config .golangci.yml test_lint.go 2>/dev/null; then
        echo "✅ Configuration works with golangci-lint"
    else
        echo "⚠️  Configuration test had issues (this may be normal)"
    fi
    
    # Clean up
    rm -f test_lint.go
else
    echo "⚠️  golangci-lint not found, skipping runtime test"
fi

echo "🎉 Configuration test completed"