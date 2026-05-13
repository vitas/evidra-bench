#!/bin/bash
set -euo pipefail
KUBECONFIG_PATH="${1:-${KUBECONFIG:-$HOME/.kube/config}}"
export KUBECONFIG="$KUBECONFIG_PATH"
# Find and cordon a worker node
NODE=$(kubectl get nodes --no-headers -o custom-columns=NAME:.metadata.name | grep -v control-plane | head -1)
if [ -z "$NODE" ]; then
  echo "No worker node found, cordoning control-plane"
  NODE=$(kubectl get nodes --no-headers -o custom-columns=NAME:.metadata.name | head -1)
fi
kubectl cordon "$NODE"
