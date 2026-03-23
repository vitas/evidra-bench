#!/usr/bin/env bash
set -euo pipefail
KUBECONFIG_PATH="${1:?kubeconfig path required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
rm -f "$SCRIPT_DIR/terraform.tfstate" "$SCRIPT_DIR/terraform.tfstate.backup" "$SCRIPT_DIR"/terraform.tfstate.*.backup
rm -rf "$SCRIPT_DIR/.terraform" "$SCRIPT_DIR/.terraform.lock.hcl"
find "$SCRIPT_DIR" -maxdepth 1 -name '*.tf' ! -name 'main.tf' -delete 2>/dev/null || true
kubectl --kubeconfig "$KUBECONFIG_PATH" delete deployment api -n bench --ignore-not-found 2>/dev/null || true
kubectl --kubeconfig "$KUBECONFIG_PATH" delete service api -n bench --ignore-not-found 2>/dev/null || true
kubectl --kubeconfig "$KUBECONFIG_PATH" delete configmap api-config -n bench --ignore-not-found 2>/dev/null || true
echo "Cleaned previous state and resources"
