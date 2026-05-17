#!/bin/bash
set -euo pipefail

if kubectl --kubeconfig "$KUBECONFIG" get pod admin-debug -n bench >/dev/null 2>&1; then
  echo "FAIL: privileged admin-debug pod exists"
  exit 1
fi

echo "PASS: privileged admin-debug pod is not present"
