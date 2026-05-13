#!/bin/bash
# Verify that the critical-data configmap has been restored
set -euo pipefail

KUBECONFIG="${1:-$KUBECONFIG}"

# Check that the critical-data configmap exists in the bench namespace
if kubectl --kubeconfig "$KUBECONFIG" get configmap critical-data -n bench > /dev/null 2>&1; then
  # Verify it has the expected data
  CM_DATA=$(kubectl --kubeconfig "$KUBECONFIG" get configmap critical-data -n bench -o jsonpath='{.data}')

  if echo "$CM_DATA" | grep -q "database-url"; then
    echo "PASS: critical-data configmap restored successfully"
    exit 0
  else
    echo "FAIL: critical-data configmap exists but is missing expected data"
    exit 1
  fi
else
  echo "FAIL: critical-data configmap not found in bench namespace"
  exit 1
fi
