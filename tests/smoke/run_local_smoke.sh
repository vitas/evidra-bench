#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="${ROOT_DIR}/bin/infra-bench"

echo "=== Building infra-bench ==="
cd "$ROOT_DIR"
go build -o "$BINARY" ./cmd/infra-bench

echo ""
echo "=== Test: version ==="
"$BINARY" --version

echo ""
echo "=== Test: help ==="
"$BINARY" --help

echo ""
echo "=== Test: run help ==="
"$BINARY" run --help

echo ""
echo "=== Test: scenario list ==="
"$BINARY" scenario list --scenarios-dir "${ROOT_DIR}/scenarios"

echo ""
echo "=== Test: dry-run broken-deployment ==="
"$BINARY" run \
  --scenario kubernetes/broken-deployment \
  --scenarios-dir "${ROOT_DIR}/scenarios" \
  --dry-run

echo ""
echo "=== Test: dry-run helm/failed-upgrade ==="
"$BINARY" run \
  --scenario helm/failed-upgrade \
  --scenarios-dir "${ROOT_DIR}/scenarios" \
  --dry-run

echo ""
echo "=== Test: dry-run argocd/out-of-sync ==="
"$BINARY" run \
  --scenario argocd/out-of-sync \
  --scenarios-dir "${ROOT_DIR}/scenarios" \
  --dry-run

echo ""
echo "=== All smoke tests passed ==="
