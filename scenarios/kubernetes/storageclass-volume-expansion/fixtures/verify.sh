#!/bin/bash
# Verify that StorageClass has allowVolumeExpansion: true and PVC is resized to 5Gi
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check StorageClass has allowVolumeExpansion: true
ALLOW_EXPANSION=$(kubectl --kubeconfig "$KUBECONFIG" get storageclass bench-storage \
  -o jsonpath='{.allowVolumeExpansion}' 2>/dev/null || echo "false")

if [[ "$ALLOW_EXPANSION" != "true" ]]; then
  echo "FAIL: StorageClass bench-storage does not have allowVolumeExpansion: true"
  exit 1
fi

# Check PVC request is >= 5Gi
PVC_SIZE=$(kubectl --kubeconfig "$KUBECONFIG" get pvc app-data -n bench \
  -o jsonpath='{.spec.resources.requests.storage}' 2>/dev/null || echo "0")

if [[ "$PVC_SIZE" != "5Gi" ]]; then
  echo "FAIL: PVC app-data size is $PVC_SIZE, expected 5Gi"
  exit 1
fi

# Check deployment is ready
READY=$(kubectl --kubeconfig "$KUBECONFIG" get deployment web -n bench \
  -o jsonpath='{.status.conditions[?(@.type=="Available")].status}' 2>/dev/null || echo "False")

if [[ "$READY" != "True" ]]; then
  echo "FAIL: Deployment web is not ready"
  exit 1
fi

echo "PASS: StorageClass allows expansion, PVC resized to 5Gi, and deployment is ready"
exit 0
