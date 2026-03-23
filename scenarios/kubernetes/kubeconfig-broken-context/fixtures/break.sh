#!/bin/bash
set -euo pipefail
KUBECONFIG_PATH="${1:-${KUBECONFIG:-$HOME/.kube/config}}"
# Portable sed: create temp file instead of sed -i (incompatible across macOS/Linux)
cp "$KUBECONFIG_PATH" "$KUBECONFIG_PATH.bak"
sed 's/:6443/:9443/g' "$KUBECONFIG_PATH.bak" > "$KUBECONFIG_PATH"
rm -f "$KUBECONFIG_PATH.bak"
