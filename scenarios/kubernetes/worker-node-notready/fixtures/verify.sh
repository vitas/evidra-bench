#!/bin/bash
# Verify that all nodes are Ready
set -euo pipefail

KUBECONFIG="${1:-$KUBECONFIG}"

# Get all nodes and check their status
NOTREADY=$(kubectl --kubeconfig "$KUBECONFIG" get nodes -o jsonpath='{.items[?(@.status.conditions[?(@.type=="Ready")].status!="True")].metadata.name}')

if [[ -z "$NOTREADY" ]]; then
  echo "PASS: All nodes are Ready"
  exit 0
else
  echo "FAIL: Nodes still NotReady: $NOTREADY"
  exit 1
fi
