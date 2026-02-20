#!/usr/bin/env bash
set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
export GOCACHE="${GOCACHE:-/tmp/go-build-cache}"

cd "$ROOT_DIR/backend"
go test ./... >/dev/null

if [ -d "$ROOT_DIR/web/node_modules" ]; then
  cd "$ROOT_DIR/web"
  npx tsc --noEmit >/dev/null
  npm run build >/dev/null
fi

echo "Lint/build checks completed"
