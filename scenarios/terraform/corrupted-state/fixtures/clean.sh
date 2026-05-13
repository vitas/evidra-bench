#!/usr/bin/env bash
# Clean up previous terraform state and resources
# Args: $1 = kubeconfig path (passed by harness)
set -euo pipefail

KUBECONFIG_PATH="${1:?kubeconfig path required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"

# Clean previous terraform state
rm -f "$SCRIPT_DIR/terraform.tfstate" "$SCRIPT_DIR/terraform.tfstate.backup"

# Clean previous resources from cluster
kubectl --kubeconfig "$KUBECONFIG_PATH" delete deployment web -n bench --ignore-not-found 2>/dev/null || true
kubectl --kubeconfig "$KUBECONFIG_PATH" delete service web -n bench --ignore-not-found 2>/dev/null || true

echo "Cleaned previous state and resources"
