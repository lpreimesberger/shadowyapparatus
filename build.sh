#!/usr/bin/env bash
set -e

# Determine build number
if [[ -n "$GITHUB_RUN_NUMBER" ]]; then
  BUILD_NUMBER="$GITHUB_RUN_NUMBER"
elif [[ -n "$GITHUB_RUN_ID" ]]; then
  BUILD_NUMBER="$GITHUB_RUN_ID"
else
  BUILD_NUMBER=$(date +"%y%m%d%H%M")
fi

echo "Using BUILD_NUMBER=$BUILD_NUMBER"

# Build with build number embedded
GO_LDFLAGS="-X 'shadowyapparatus/cmd.BuildNumber=$BUILD_NUMBER'"
go build -ldflags="$GO_LDFLAGS" -o shadowyapparatus . 
ssh nanocat@192.168.68.62 'killall shadowyapparatus || true'
scp shadowyapparatus nanocat@192.168.68.62:/home/nanocat/shadowy/

