#!/bin/bash
set -euo pipefail

# Verify that:
# 1. kubectl cluster-info works (connectivity restored)
# 2. The broken port 9443 is no longer in kubeconfig

# Check that cluster-info succeeds
if ! kubectl cluster-info >/dev/null 2>&1; then
  echo "ERROR: kubectl cluster-info failed — connectivity not restored"
  exit 1
fi

# Verify the broken port is gone
KUBECONFIG_PATH="${KUBECONFIG:-$HOME/.kube/config}"
if grep -q ':9443' "$KUBECONFIG_PATH"; then
  echo "ERROR: Kubeconfig still contains broken port 9443"
  exit 1
fi

echo "PASS: Kubeconfig connectivity restored, broken port removed"
