#!/bin/bash
# Verify the syscall-tester pod has a Seccomp profile applied (not Unconfined).
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check pod-level seccomp profile
POD_PROFILE=$(kubectl --kubeconfig "$KUBECONFIG" get pod syscall-tester -n bench \
  -o jsonpath='{.spec.securityContext.seccompProfile.type}' 2>/dev/null || echo "")

# Check container-level seccomp profile
CONTAINER_PROFILE=$(kubectl --kubeconfig "$KUBECONFIG" get pod syscall-tester -n bench \
  -o jsonpath='{.spec.containers[0].securityContext.seccompProfile.type}' 2>/dev/null || echo "")

PROFILE="${POD_PROFILE:-$CONTAINER_PROFILE}"

if [[ "$PROFILE" == "Unconfined" || -z "$PROFILE" ]]; then
  echo "FAIL: Pod still has Unconfined or no Seccomp profile (got: '$PROFILE')"
  exit 1
fi

echo "PASS: Seccomp profile applied: $PROFILE"
