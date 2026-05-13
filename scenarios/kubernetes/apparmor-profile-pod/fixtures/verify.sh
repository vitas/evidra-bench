#!/bin/bash
# Verify the writer pod has an AppArmor profile applied.
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check if the pod has the AppArmor annotation
ANNOTATION=$(kubectl --kubeconfig "$KUBECONFIG" get pod writer -n bench \
  -o jsonpath='{.metadata.annotations.container\.apparmor\.security\.beta\.kubernetes\.io/writer}' 2>/dev/null || true)

if [[ "$ANNOTATION" == *"k8s-bench-restrict-writes"* ]]; then
  echo "PASS: AppArmor profile applied to writer container"
  exit 0
fi

# Also check the newer securityContext.appArmorProfile field (K8s 1.30+)
PROFILE=$(kubectl --kubeconfig "$KUBECONFIG" get pod writer -n bench \
  -o jsonpath='{.spec.containers[0].securityContext.appArmorProfile.localhostProfile}' 2>/dev/null || true)

if [[ "$PROFILE" == *"k8s-bench-restrict-writes"* ]]; then
  echo "PASS: AppArmor profile applied via securityContext"
  exit 0
fi

echo "FAIL: No AppArmor profile found on writer pod"
exit 1
