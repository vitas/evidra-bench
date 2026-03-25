#!/bin/bash
set -euo pipefail

# Ensure probe pod exists (agent may have deleted it)
kubectl --kubeconfig "$KUBECONFIG" apply -f "$(dirname "$0")/client.yaml" 2>/dev/null || true
kubectl --kubeconfig "$KUBECONFIG" wait --for=condition=Ready pod/net-client -n bench --timeout=30s

# Test connectivity from net-client to web service
RESULT=$(kubectl --kubeconfig "$KUBECONFIG" exec -n bench net-client -- wget -q -O - -T 5 http://web.bench.svc.cluster.local 2>&1)
if [ -z "$RESULT" ]; then
  echo "FAIL: web service not reachable from net-client"
  exit 1
fi
echo "PASS: web service reachable"
