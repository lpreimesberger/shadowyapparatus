#!/bin/bash
# Manual Release Script for Shadowy Blockchain
# This script helps create manual releases with proper versioning and signing

set -euo pipefail

# Configuration
BASE_VERSION="0.1"
BINARY_NAME="shadowy"
PLATFORMS=("linux/amd64")

# Colors for output
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m' # No Color

# Logging functions
log_info() {
    echo -e "${BLUE}[INFO]${NC} $1"
}

log_success() {
    echo -e "${GREEN}[SUCCESS]${NC} $1"
}

log_warning() {
    echo -e "${YELLOW}[WARNING]${NC} $1"
}

log_error() {
    echo -e "${RED}[ERROR]${NC} $1"
}

# Help function
show_help() {
    cat << EOF
Shadowy Blockchain Release Script

Usage: $0 [OPTIONS]

Options:
    -h, --help              Show this help message
    -v, --version VERSION   Override version (default: auto-calculated)
    -t, --tag               Create and push git tag
    -s, --sign              Sign binaries with cosign
    -d, --dry-run           Show what would be done without executing
    -c, --clean             Clean build artifacts before building
    --skip-tests            Skip running tests before build
    --skip-lint             Skip linting before build

Examples:
    $0                      # Build with auto-calculated version
    $0 -v 0.2+456          # Build with specific version
    $0 -t -s               # Build, tag, and sign
    $0 --dry-run           # Show what would be done

EOF
}

# Parse command line arguments
VERSION=""
CREATE_TAG=false
SIGN_BINARIES=false
DRY_RUN=false
CLEAN_BUILD=false
SKIP_TESTS=false
SKIP_LINT=false

while [[ $# -gt 0 ]]; do
    case $1 in
        -h|--help)
            show_help
            exit 0
            ;;
        -v|--version)
            VERSION="$2"
            shift 2
            ;;
        -t|--tag)
            CREATE_TAG=true
            shift
            ;;
        -s|--sign)
            SIGN_BINARIES=true
            shift
            ;;
        -d|--dry-run)
            DRY_RUN=true
            shift
            ;;
        -c|--clean)
            CLEAN_BUILD=true
            shift
            ;;
        --skip-tests)
            SKIP_TESTS=true
            shift
            ;;
        --skip-lint)
            SKIP_LINT=true
            shift
            ;;
        *)
            log_error "Unknown option: $1"
            show_help
            exit 1
            ;;
    esac
done

# Check required tools
check_dependencies() {
    local missing_tools=()
    
    # Required tools
    command -v go >/dev/null 2>&1 || missing_tools+=("go")
    command -v git >/dev/null 2>&1 || missing_tools+=("git")
    command -v sha256sum >/dev/null 2>&1 || missing_tools+=("sha256sum")
    
    # Optional tools
    if [ "$SKIP_LINT" = false ]; then
        command -v golangci-lint >/dev/null 2>&1 || log_warning "golangci-lint not found, skipping linting"
    fi
    
    if [ "$SIGN_BINARIES" = true ]; then
        command -v cosign >/dev/null 2>&1 || missing_tools+=("cosign")
    fi
    
    if [ ${#missing_tools[@]} -ne 0 ]; then
        log_error "Missing required tools: ${missing_tools[*]}"
        log_info "Install missing tools:"
        for tool in "${missing_tools[@]}"; do
            case $tool in
                go)
                    echo "  Go: https://golang.org/doc/install"
                    ;;
                git)
                    echo "  Git: https://git-scm.com/downloads"
                    ;;
                sha256sum)
                    echo "  sha256sum: usually part of coreutils"
                    ;;
                cosign)
                    echo "  Cosign: go install github.com/sigstore/cosign/v2/cmd/cosign@latest"
                    ;;
            esac
        done
        exit 1
    fi
}

# Calculate version
calculate_version() {
    if [ -n "$VERSION" ]; then
        echo "$VERSION"
        return
    fi
    
    # Auto-calculate version
    local build_number
    build_number=$(git rev-list --count HEAD 2>/dev/null || echo "0")
    echo "${BASE_VERSION}+${build_number}"
}

# Get build information
get_build_info() {
    local commit_sha
    local build_time
    
    commit_sha=$(git rev-parse --short HEAD 2>/dev/null || echo "unknown")
    build_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    echo "$commit_sha" "$build_time"
}

# Run tests
run_tests() {
    if [ "$SKIP_TESTS" = true ]; then
        log_warning "Skipping tests"
        return
    fi
    
    log_info "Running tests..."
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: Would run: go test -v ./..."
    else
        go test -v ./...
        log_success "Tests passed"
    fi
}

# Run linting
run_lint() {
    if [ "$SKIP_LINT" = true ]; then
        log_warning "Skipping linting"
        return
    fi
    
    if ! command -v golangci-lint >/dev/null 2>&1; then
        log_warning "golangci-lint not available, skipping"
        return
    fi
    
    log_info "Running linting..."
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: Would run: golangci-lint run"
    else
        golangci-lint run
        log_success "Linting passed"
    fi
}

# Clean build artifacts
clean_build() {
    if [ "$CLEAN_BUILD" = false ]; then
        return
    fi
    
    log_info "Cleaning build artifacts..."
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: Would remove: ${BINARY_NAME}-* *.sha256 *.cosign.bundle"
    else
        rm -f ${BINARY_NAME}-*
        rm -f *.sha256 *.sha512 *.md5
        rm -f *.cosign.bundle
        log_success "Build artifacts cleaned"
    fi
}

# Build binary for specific platform
build_binary() {
    local platform=$1
    local version=$2
    local commit_sha=$3
    local build_time=$4
    
    local goos goarch
    IFS='/' read -r goos goarch <<< "$platform"
    
    local output_name="${BINARY_NAME}-${goos}-${goarch}"
    
    log_info "Building for ${platform}..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: Would build ${output_name} with version ${version}"
        return
    fi
    
    CGO_ENABLED=0 GOOS="$goos" GOARCH="$goarch" go build \
        -ldflags="-s -w -X main.Version=${version} -X main.GitCommit=${commit_sha} -X main.BuildTime=${build_time}" \
        -o "$output_name" \
        .
    
    # Make executable
    chmod +x "$output_name"
    
    # Get file info
    local file_size
    file_size=$(du -h "$output_name" | cut -f1)
    log_success "Built ${output_name} (${file_size})"
    
    # Generate checksums
    sha256sum "$output_name" > "${output_name}.sha256"
    sha512sum "$output_name" > "${output_name}.sha512"
    md5sum "$output_name" > "${output_name}.md5"
    
    log_info "Generated checksums for ${output_name}"
}

# Sign binary with cosign
sign_binary() {
    local binary_path=$1
    
    if [ "$SIGN_BINARIES" = false ]; then
        return
    fi
    
    log_info "Signing ${binary_path} with cosign..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: Would sign ${binary_path} with cosign"
        return
    fi
    
    # Sign the binary
    COSIGN_EXPERIMENTAL=1 cosign sign-blob \
        --bundle "${binary_path}.cosign.bundle" \
        "$binary_path"
    
    # Sign the checksum
    COSIGN_EXPERIMENTAL=1 cosign sign-blob \
        --bundle "${binary_path}.sha256.cosign.bundle" \
        "${binary_path}.sha256"
    
    log_success "Signed ${binary_path}"
}

# Create git tag
create_git_tag() {
    local version=$1
    
    if [ "$CREATE_TAG" = false ]; then
        return
    fi
    
    local tag_name="v${version}"
    
    log_info "Creating git tag ${tag_name}..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: Would create and push tag ${tag_name}"
        return
    fi
    
    # Check if tag already exists
    if git tag -l | grep -q "^${tag_name}$"; then
        log_warning "Tag ${tag_name} already exists"
        return
    fi
    
    # Create annotated tag
    git tag -a "$tag_name" -m "Release ${version}"
    
    # Push tag
    if git remote | grep -q origin; then
        git push origin "$tag_name"
        log_success "Created and pushed tag ${tag_name}"
    else
        log_warning "No origin remote found, tag created locally only"
    fi
}

# Generate release notes
generate_release_notes() {
    local version=$1
    local output_file="RELEASE_NOTES_${version}.md"
    
    log_info "Generating release notes..."
    
    if [ "$DRY_RUN" = true ]; then
        log_info "DRY RUN: Would generate ${output_file}"
        return
    fi
    
    cat > "$output_file" << EOF
# Shadowy Blockchain Release ${version}

## 🚀 What's New

This release includes the complete Shadowy blockchain implementation.

### ✨ Features
- **Proof-of-Storage Consensus**: Energy-efficient mining using storage space
- **Bitcoin-style Tokenomics**: 21M SHADOW max supply with halving every 210,000 blocks
- **Multi-node P2P Network**: Full consensus and synchronization between peers
- **Mining & Farming**: Plot-based storage mining with 10-minute block times
- **Web3 Compatible**: Complete transaction and wallet management
- **RESTful API**: Comprehensive HTTP API for all blockchain operations

### 📊 Build Information
- **Version**: ${version}
- **Platform**: Linux x86_64
- **Go Version**: $(go version | cut -d' ' -f3)
- **Build Time**: $(date -u +"%Y-%m-%d %H:%M:%S UTC")

### 📥 Installation

\`\`\`bash
# Download binary
wget https://github.com/\$GITHUB_REPOSITORY/releases/download/v${version}/shadowy-linux-amd64

# Verify checksum
wget https://github.com/\$GITHUB_REPOSITORY/releases/download/v${version}/shadowy-linux-amd64.sha256
sha256sum -c shadowy-linux-amd64.sha256

# Make executable
chmod +x shadowy-linux-amd64

# Run
./shadowy-linux-amd64 --help
\`\`\`

### 🚀 Quick Start

\`\`\`bash
# Start a mining node
./shadowy-linux-amd64 node

# Create a wallet
./shadowy-linux-amd64 wallet create my-wallet

# Check mining status
curl http://localhost:8080/api/v1/mining
\`\`\`

EOF

    log_success "Generated ${output_file}"
}

# Main function
main() {
    log_info "Starting Shadowy Blockchain release process..."
    
    # Check dependencies
    check_dependencies
    
    # Calculate version and build info
    local version commit_sha build_time
    version=$(calculate_version)
    read -r commit_sha build_time <<< "$(get_build_info)"
    
    log_info "Release version: ${version}"
    log_info "Git commit: ${commit_sha}"
    log_info "Build time: ${build_time}"
    
    if [ "$DRY_RUN" = true ]; then
        log_warning "DRY RUN MODE - No actual changes will be made"
    fi
    
    # Clean if requested
    clean_build
    
    # Run quality checks
    run_lint
    run_tests
    
    # Build binaries for all platforms
    for platform in "${PLATFORMS[@]}"; do
        build_binary "$platform" "$version" "$commit_sha" "$build_time"
        
        # Sign if requested
        local goos goarch
        IFS='/' read -r goos goarch <<< "$platform"
        local binary_name="${BINARY_NAME}-${goos}-${goarch}"
        sign_binary "$binary_name"
    done
    
    # Create git tag if requested
    create_git_tag "$version"
    
    # Generate release notes
    generate_release_notes "$version"
    
    # Summary
    log_success "Release ${version} completed successfully!"
    
    if [ "$DRY_RUN" = false ]; then
        echo
        log_info "Generated files:"
        ls -la ${BINARY_NAME}-* *.sha* *.md5 *.cosign.bundle RELEASE_NOTES_* 2>/dev/null || true
        
        echo
        log_info "Next steps:"
        echo "1. Test the binaries"
        echo "2. Upload to GitHub releases"
        echo "3. Update documentation"
        echo "4. Announce the release"
    fi
}

# Run main function
main "$@"