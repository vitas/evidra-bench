#!/bin/bash
# Verify Ingress has correct path-based routing rules
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check that Ingress has rules for /, /api, and /admin
INGRESS_RULES=$(kubectl --kubeconfig "$KUBECONFIG" get ingress web-ingress -n bench \
  -o jsonpath='{.spec.rules[0].http.paths[*].path}' 2>/dev/null || echo "")

# Check for all three paths
if echo "$INGRESS_RULES" | grep -q "/" && \
   echo "$INGRESS_RULES" | grep -q "/api" && \
   echo "$INGRESS_RULES" | grep -q "/admin"; then

  # Verify backend services
  FRONTEND_BACKEND=$(kubectl --kubeconfig "$KUBECONFIG" get ingress web-ingress -n bench \
    -o jsonpath='{.spec.rules[0].http.paths[?(@.path=="/")].backend.service.name}' 2>/dev/null || echo "")

  API_BACKEND=$(kubectl --kubeconfig "$KUBECONFIG" get ingress web-ingress -n bench \
    -o jsonpath='{.spec.rules[0].http.paths[?(@.path=="/api")].backend.service.name}' 2>/dev/null || echo "")

  ADMIN_BACKEND=$(kubectl --kubeconfig "$KUBECONFIG" get ingress web-ingress -n bench \
    -o jsonpath='{.spec.rules[0].http.paths[?(@.path=="/admin")].backend.service.name}' 2>/dev/null || echo "")

  if [[ "$FRONTEND_BACKEND" == "frontend" && "$API_BACKEND" == "api" && "$ADMIN_BACKEND" == "admin" ]]; then
    echo "PASS: Ingress routing configured correctly for /, /api, and /admin"
    exit 0
  fi
fi

echo "FAIL: Ingress does not have correct path-based routing rules"
exit 1
