#!/bin/bash
set -euo pipefail
KUBECONFIG_PATH="${1:-${KUBECONFIG:-$HOME/.kube/config}}"

# Get the actual server port from kubeconfig (kind uses random ports, not always 6443)
ACTUAL_PORT=$(grep -oE 'server: https://[^:]+:([0-9]+)' "$KUBECONFIG_PATH" | grep -oE '[0-9]+$' | head -1)
if [ -z "$ACTUAL_PORT" ] || [ "$ACTUAL_PORT" = "9443" ]; then
  echo "ERROR: could not find server port in kubeconfig or already broken"
  exit 1
fi

# Replace the actual port with 9443 (broken port)
cp "$KUBECONFIG_PATH" "$KUBECONFIG_PATH.bak"
sed "s/:${ACTUAL_PORT}/:9443/g" "$KUBECONFIG_PATH.bak" > "$KUBECONFIG_PATH"
rm -f "$KUBECONFIG_PATH.bak"

echo "Broke kubeconfig: replaced port $ACTUAL_PORT with 9443"
