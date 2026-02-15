#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"

echo "Starting HarborWatch backend on :8080"
(
  cd "$ROOT_DIR/backend"
  go run ./cmd/server
)
