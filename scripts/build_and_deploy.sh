#!/bin/bash

# Quick Build and Deploy Script
# Builds the binary and deploys it to the remote node

set -e

cd "$(dirname "$0")/.."

echo "🔨 Building and deploying Shadowy node..."
echo ""

# Build and deploy
./scripts/deploy_to_node.sh --clean --install-service

echo ""
echo "🚀 Deployment complete!"
echo ""
echo "Next steps:"
echo "1. Check status: ./scripts/manage_remote_node.sh status"
echo "2. View logs: ./scripts/manage_remote_node.sh tail"
echo "3. Test connectivity: ./scripts/test_multinode.sh"
echo ""
echo "API endpoints:"
echo "- Local node: http://192.168.68.90:8080/api/v1/health"
echo "- Remote node: http://192.168.68.62:8080/api/v1/health"