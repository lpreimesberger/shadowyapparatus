#!/bin/bash

echo "🔨 Building Shadowy WASM Library..."

# Set WASM build environment
export GOOS=js
export GOARCH=wasm

# Build the WASM module
go build -o shadowy.wasm main.go

if [ $? -eq 0 ]; then
    echo "✅ WASM build successful: shadowy.wasm"
    ls -la shadowy.wasm
else
    echo "❌ WASM build failed"
    exit 1
fi

# Copy the Go WASM support file
GOROOT_VAL=$(go env GOROOT)
cp "$GOROOT_VAL/misc/wasm/wasm_exec.js" .

echo "📦 Files created:"
echo "  - shadowy.wasm (WASM module)"
echo "  - wasm_exec.js (Go WASM runtime)"
echo ""
echo "🚀 Ready to use! See example usage in test.js"