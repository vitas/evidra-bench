#!/bin/bash
set -euo pipefail

# Verify that:
# 1. kubectl cluster-info works
# 2. The kubeconfig contains port 6443 (not 9443)

# Check that cluster-info succeeds
kubectl cluster-info

# Verify the server URL in kubeconfig contains 6443
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/.kube/config}"
if ! grep -q ':6443' "$KUBECONFIG_PATH"; then
  echo "ERROR: Kubeconfig does not contain :6443"
  exit 1
fi

if grep -q ':9443' "$KUBECONFIG_PATH"; then
  echo "ERROR: Kubeconfig still contains broken port 9443"
  exit 1
fi

echo "✓ Kubeconfig is correctly configured with port 6443"
