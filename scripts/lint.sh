#!/bin/bash
# Local linting script for development

set -euo pipefail

# Colors
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m' # No Color

echo -e "${GREEN}🔍 Running Go linting checks...${NC}"

# Check if golangci-lint is installed
if ! command -v golangci-lint >/dev/null 2>&1; then
    echo -e "${YELLOW}⚠️  golangci-lint not found${NC}"
    echo "Installing golangci-lint..."
    
    # Install golangci-lint
    if command -v go >/dev/null 2>&1; then
        echo "Installing via go install..."
        go install github.com/golangci/golangci-lint/cmd/golangci-lint@v1.59.1
    else
        echo -e "${RED}❌ Go not found. Please install Go first.${NC}"
        exit 1
    fi
fi

# Check golangci-lint version
echo "golangci-lint version:"
golangci-lint version

# Validate configuration
echo -e "\n${GREEN}📋 Validating golangci-lint configuration...${NC}"
if golangci-lint config verify .golangci.yml; then
    echo -e "${GREEN}✅ Configuration is valid${NC}"
else
    echo -e "${RED}❌ Configuration validation failed${NC}"
    exit 1
fi

# Run linting
echo -e "\n${GREEN}🔍 Running golangci-lint...${NC}"
if golangci-lint run --timeout=5m; then
    echo -e "${GREEN}✅ Linting passed!${NC}"
else
    echo -e "${RED}❌ Linting failed${NC}"
    exit 1
fi

# Run go fmt check
echo -e "\n${GREEN}📐 Checking Go formatting...${NC}"
if [ -n "$(gofmt -l .)" ]; then
    echo -e "${RED}❌ Code is not properly formatted:${NC}"
    gofmt -l .
    echo "Run 'gofmt -w .' to fix formatting issues"
    exit 1
else
    echo -e "${GREEN}✅ Code is properly formatted${NC}"
fi

# Run go vet
echo -e "\n${GREEN}🔍 Running go vet...${NC}"
if go vet ./...; then
    echo -e "${GREEN}✅ go vet passed${NC}"
else
    echo -e "${RED}❌ go vet failed${NC}"
    exit 1
fi

echo -e "\n${GREEN}🎉 All linting checks passed!${NC}"