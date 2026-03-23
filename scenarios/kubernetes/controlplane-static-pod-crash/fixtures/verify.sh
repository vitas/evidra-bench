#!/bin/bash
# Verify that the API server is responding and etcd-servers flag is correct
set -euo pipefail

KUBECONFIG="${1:-$KUBECONFIG}"

# Test that kubectl is working (API server is responding)
if ! kubectl --kubeconfig "$KUBECONFIG" get nodes > /dev/null 2>&1; then
  echo "FAIL: kubectl is not responding (API server still down)"
  exit 1
fi

# Verify the kube-apiserver manifest has the correct etcd-servers flag
# The control-plane node should be accessible via docker exec
CONTROL_PLANE=$(docker ps --filter "label=io.x-k8s.kind.role=control-plane" --format "{{.Names}}" | head -1)

if [[ -z "$CONTROL_PLANE" ]]; then
  echo "FAIL: Could not find control-plane container"
  exit 1
fi

# Check that the etcd-servers flag points to the correct port (2379, not 2399)
if docker exec "$CONTROL_PLANE" grep -q "etcd-servers=http://127.0.0.1:2379" /etc/kubernetes/manifests/kube-apiserver.yaml; then
  echo "PASS: API server is responding and etcd-servers flag is correct"
  exit 0
else
  echo "FAIL: etcd-servers flag is still incorrect"
  exit 1
fi
