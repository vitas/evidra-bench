#!/bin/bash
set -euo pipefail

KUBECTL=(kubectl)
if [[ -n "${KUBECONFIG:-}" ]]; then
  KUBECTL=(kubectl --kubeconfig "$KUBECONFIG")
fi

fixture_dir="$(cd "$(dirname "$0")" && pwd)"

"${KUBECTL[@]}" apply -f "$fixture_dir/probe.yaml" >/dev/null
"${KUBECTL[@]}" wait --for=condition=Ready pod/frontend-probe -n bench --timeout=30s
"${KUBECTL[@]}" wait --for=condition=Ready pod/backend-probe -n bench --timeout=30s

can_connect() {
  local pod="$1"
  local service="$2"
  "${KUBECTL[@]}" exec -n bench "$pod" -- wget -q -O - -T 5 "http://${service}.bench.svc.cluster.local" >/dev/null 2>&1
}

if ! can_connect frontend-probe backend; then
  echo "FAIL: frontend cannot reach backend"
  exit 1
fi

if ! can_connect backend-probe database; then
  echo "FAIL: backend cannot reach database"
  exit 1
fi

if can_connect frontend-probe database; then
  echo "FAIL: frontend can still reach database directly"
  exit 1
fi

echo "PASS: network segmentation is correct"
