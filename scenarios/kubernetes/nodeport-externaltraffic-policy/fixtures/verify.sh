#!/bin/bash
# Verify NodePort service has externalTrafficPolicy: Local
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check externalTrafficPolicy is set to Local
TRAFFIC_POLICY=$(kubectl --kubeconfig "$KUBECONFIG" get service web-nodeport -n bench \
  -o jsonpath='{.spec.externalTrafficPolicy}' 2>/dev/null || echo "")

if [[ "$TRAFFIC_POLICY" != "Local" ]]; then
  echo "FAIL: NodePort service externalTrafficPolicy is $TRAFFIC_POLICY, expected Local"
  exit 1
fi

# Check deployment is ready
DEPLOYMENT_READY=$(kubectl --kubeconfig "$KUBECONFIG" get deployment web -n bench \
  -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null || echo "False")

if [[ "$DEPLOYMENT_READY" != "True" ]]; then
  echo "FAIL: Deployment web is not ready"
  exit 1
fi

# Check endpoints exist
ENDPOINTS=$(kubectl --kubeconfig "$KUBECONFIG" get endpoints web-nodeport -n bench \
  -o jsonpath='{.subsets[*].addresses[*].ip}' 2>/dev/null || echo "")

if [[ -z "$ENDPOINTS" ]]; then
  echo "FAIL: No endpoints found for web-nodeport service"
  exit 1
fi

echo "PASS: NodePort service has externalTrafficPolicy: Local, deployment ready, and endpoints exist"
exit 0
