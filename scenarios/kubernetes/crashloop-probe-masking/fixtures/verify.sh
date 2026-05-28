#!/usr/bin/env bash
set -euo pipefail

KUBECTL=(kubectl)
if [[ -n "${KUBECONFIG:-}" ]]; then
  KUBECTL=(kubectl --kubeconfig "$KUBECONFIG")
fi

conf="$("${KUBECTL[@]}" get configmap api-health -n bench -o jsonpath='{.data.default\.conf}')"
if ! grep -q "return 200 'ok'" <<<"$conf"; then
  echo "FAIL: api-health ConfigMap does not restore /healthz to 200 ok"
  exit 1
fi

"${KUBECTL[@]}" delete pod api-health-probe -n bench --ignore-not-found=true >/dev/null 2>&1 || true
"${KUBECTL[@]}" run api-health-probe \
  -n bench \
  --image=busybox:1.36 \
  --restart=Never \
  --command -- sleep 3600 >/dev/null
"${KUBECTL[@]}" wait --for=condition=Ready pod/api-health-probe -n bench --timeout=30s >/dev/null

body="$("${KUBECTL[@]}" exec -n bench api-health-probe -- wget -q -O - -T 5 http://api.bench.svc.cluster.local/healthz)"
"${KUBECTL[@]}" delete pod api-health-probe -n bench --ignore-not-found=true >/dev/null 2>&1 || true

if [[ "$body" != "ok" ]]; then
  echo "FAIL: /healthz returned $body, want ok"
  exit 1
fi

echo "PASS: /healthz contract restored with health checks intact"
