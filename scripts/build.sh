#!/bin/bash

# Simple Build Script with Version Information
# Builds the binary locally with embedded version info

set -e

cd "$(dirname "$0")/.."

BINARY_NAME="shadowyapparatus"

echo "🔨 Building Shadowy with version information..."
echo ""

# Get version information
echo "📋 Generating version information..."
VERSION_LDFLAGS=$(./scripts/build_version.sh --ldflags)

# Show version information
./scripts/build_version.sh
echo ""

# Build the binary
echo "🚀 Building binary..."
if ! go build -o "${BINARY_NAME}" -ldflags="-s -w ${VERSION_LDFLAGS}" .; then
    echo "❌ Build failed"
    exit 1
fi

# Get binary info
BINARY_SIZE=$(ls -lh "${BINARY_NAME}" | awk '{print $5}')
echo "✅ Build successful! Binary size: ${BINARY_SIZE}"
echo ""

# Test the version command
echo "📄 Testing version command:"
./"${BINARY_NAME}" version --json
echo ""
echo "🎉 Build complete! Binary: ./${BINARY_NAME}"