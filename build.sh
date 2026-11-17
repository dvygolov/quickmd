#!/usr/bin/env bash
# Build the Windows binary (runs from WSL/Linux).
set -euo pipefail
cd "$(dirname "$0")"
GOOS=windows GOARCH=amd64 GOCACHE="$PWD/.gocache" \
  go build -ldflags="-H=windowsgui -s -w" -o quickmd.exe .
echo "built quickmd.exe"
