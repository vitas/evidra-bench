#!/bin/bash
set -euo pipefail
KUBECONFIG_PATH="${1:-${KUBECONFIG:-$HOME/.kube/config}}"
export KUBECONFIG="$KUBECONFIG_PATH"
kubectl patch deployment traffic-shaper -n bench --type=json -p='[
  {"op": "replace", "path": "/spec/template/spec/containers/0/securityContext", "value": {"privileged": true}}
]'
