#!/bin/bash
# Verify that cache-app pods are Running and emptyDir is properly configured
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check that cache-app pod is in Running state (not OOMKilled or CrashLoopBackOff)
POD_STATUS=$(kubectl --kubeconfig "$KUBECONFIG" get pod -n bench \
  -l app=cache-app -o jsonpath='{.items[0].status.phase}' 2>/dev/null || echo "")

if [[ "$POD_STATUS" != "Running" ]]; then
  echo "FAIL: cache-app pod is in $POD_STATUS state, expected Running"
  exit 1
fi

# Check that the pod has not been OOMKilled
LAST_STATE=$(kubectl --kubeconfig "$KUBECONFIG" get pod -n bench \
  -l app=cache-app -o jsonpath='{.items[0].status.containerStatuses[0].lastState.terminated.reason}' 2>/dev/null || echo "")

if [[ "$LAST_STATE" == "OOMKilled" ]]; then
  echo "FAIL: cache-app pod was OOMKilled, indicating fix did not work"
  exit 1
fi

# Check that emptyDir has sizeLimit OR memory limit was increased
EMPTYDIR=$(kubectl --kubeconfig "$KUBECONFIG" get deployment cache-app -n bench \
  -o jsonpath='{.spec.template.spec.volumes[?(@.name=="cache")].emptyDir}' 2>/dev/null || echo "")

MEMORY_LIMIT=$(kubectl --kubeconfig "$KUBECONFIG" get deployment cache-app -n bench \
  -o jsonpath='{.spec.template.spec.containers[0].resources.limits.memory}' 2>/dev/null || echo "")

# Check for sizeLimit in emptyDir
if echo "$EMPTYDIR" | grep -q "sizeLimit"; then
  echo "PASS: emptyDir has sizeLimit, preventing unbounded growth"
  exit 0
fi

# Check if memory limit was increased from 64Mi
if [[ "$MEMORY_LIMIT" != "64Mi" ]]; then
  echo "PASS: Memory limit increased to $MEMORY_LIMIT"
  exit 0
fi

# Check if emptyDir medium was changed from Memory to disk
MEDIUM=$(echo "$EMPTYDIR" | grep -o "medium.*" || echo "Memory")

if [[ "$MEDIUM" != "Memory" ]]; then
  echo "PASS: emptyDir medium changed from Memory to disk"
  exit 0
fi

echo "FAIL: cache-app issue not properly fixed"
exit 1
