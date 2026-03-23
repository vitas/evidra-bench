#!/bin/bash
# Verify that all nodes are running the same kubelet version
set -euo pipefail

KUBECONFIG="${1:-$KUBECONFIG}"

# Get all node kubelet versions
VERSIONS=$(kubectl --kubeconfig "$KUBECONFIG" get nodes -o jsonpath='{.items[*].status.nodeInfo.kubeletVersion}' | tr ' ' '\n' | sort -u)

# Count unique versions
VERSION_COUNT=$(echo "$VERSIONS" | wc -l)

if [[ "$VERSION_COUNT" -eq 1 ]]; then
  CURRENT_VERSION=$(echo "$VERSIONS" | head -1)
  echo "PASS: All nodes are running the same kubelet version: $CURRENT_VERSION"
  exit 0
else
  echo "FAIL: Nodes are running different kubelet versions:"
  echo "$VERSIONS"
  exit 1
fi
