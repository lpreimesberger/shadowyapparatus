#!/bin/bash

# Build Version Script
# This script manages build numbers and version information

set -e

# Configuration
VERSION_FILE=".build_version"
DEFAULT_VERSION="0.1"

# Functions
get_next_build_number() {
    if [[ -f "$VERSION_FILE" ]]; then
        local current_build=$(cat "$VERSION_FILE" 2>/dev/null || echo "0")
        echo $((current_build + 1))
    else
        echo "1"
    fi
}

save_build_number() {
    echo "$1" > "$VERSION_FILE"
}

get_git_commit() {
    if git rev-parse --git-dir > /dev/null 2>&1; then
        git rev-parse --short HEAD 2>/dev/null || echo "unknown"
    else
        echo "unknown"
    fi
}

get_git_version() {
    if git rev-parse --git-dir > /dev/null 2>&1; then
        # Try to get version from git tag
        local tag=$(git describe --tags --exact-match 2>/dev/null || echo "")
        if [[ -n "$tag" ]]; then
            # Remove 'v' prefix if present
            echo "${tag#v}"
        else
            echo "$DEFAULT_VERSION"
        fi
    else
        echo "$DEFAULT_VERSION"
    fi
}

# Main execution
main() {
    local build_number=$(get_next_build_number)
    local git_commit=$(get_git_commit)
    local version=$(get_git_version)
    local build_time=$(date -u +"%Y-%m-%dT%H:%M:%SZ")
    
    # Save the new build number
    save_build_number "$build_number"
    
    # Output version information for use in build scripts
    if [[ "${1:-}" == "--ldflags" ]]; then
        # Output ldflags for go build
        echo "-X 'shadowy/cmd.Version=$version' \
-X 'shadowy/cmd.BuildNum=$build_number' \
-X 'shadowy/cmd.GitCommit=$git_commit' \
-X 'shadowy/cmd.BuildTime=$build_time'"
    elif [[ "${1:-}" == "--json" ]]; then
        # Output JSON format
        cat <<EOF
{
    "version": "$version",
    "build_number": "$build_number",
    "git_commit": "$git_commit", 
    "build_time": "$build_time"
}
EOF
    else
        # Human readable output
        echo "Version: $version"
        echo "Build Number: $build_number"
        echo "Git Commit: $git_commit"
        echo "Build Time: $build_time"
    fi
}

# Check for help
if [[ "${1:-}" == "--help" ]]; then
    cat <<EOF
Build Version Script

This script manages build numbers and version information for Shadowy.

Usage:
    $0                 Show version information
    $0 --ldflags       Output ldflags for go build
    $0 --json          Output JSON format
    $0 --help          Show this help

The script automatically increments the build number and saves it to $VERSION_FILE.
Version is determined from git tags, defaulting to $DEFAULT_VERSION.
EOF
    exit 0
fi

main "$@"