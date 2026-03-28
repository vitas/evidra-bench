#!/usr/bin/env bash
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT_DIR="$(cd "$SCRIPT_DIR/../.." && pwd)"
BINARY="${ROOT_DIR}/bin/bench-cli"

echo "=== Using built bench-cli ==="
cd "$ROOT_DIR"
if [[ ! -x "$BINARY" ]]; then
  echo "missing built binary: $BINARY" >&2
  exit 1
fi

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
echo "=== Test: dry-run broken-deployment by id ==="
"$BINARY" run \
  --scenario broken-deployment \
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
echo "=== Test: dry-run aws/s3-bucket-public-access ==="
"$BINARY" run \
  --scenario aws/s3-bucket-public-access \
  --scenarios-dir "${ROOT_DIR}/scenarios" \
  --dry-run

echo ""
echo "=== Test: dry-run terraform/state-drift ==="
"$BINARY" run \
  --scenario terraform/state-drift \
  --scenarios-dir "${ROOT_DIR}/scenarios" \
  --dry-run

echo ""
echo "=== Test: dry-run all non-skipped scenarios ==="
"$BINARY" bench \
  --scenarios-dir "${ROOT_DIR}/scenarios" \
  --dry-run 2>&1 | tail -5

echo ""
echo "=== All smoke tests passed ==="
