#!/bin/bash
# Verify that the PV is Bound and the new PVC is bound to it
set -euo pipefail

KUBECONFIG="${1:-$KUBECONFIG}"

# Check that PV is in Bound or Available state (not Released)
PV_STATUS=$(kubectl --kubeconfig "$KUBECONFIG" get pv bench-pv -o jsonpath='{.status.phase}')

if [[ "$PV_STATUS" != "Bound" && "$PV_STATUS" != "Available" ]]; then
  echo "FAIL: PV status is $PV_STATUS (expected Bound or Available)"
  exit 1
fi

# Check that the data-new PVC is Bound
PVC_STATUS=$(kubectl --kubeconfig "$KUBECONFIG" get pvc data-new -n bench -o jsonpath='{.status.phase}')

if [[ "$PVC_STATUS" != "Bound" ]]; then
  echo "FAIL: PVC data-new status is $PVC_STATUS (expected Bound)"
  exit 1
fi

# Verify that the PVC is bound to the correct PV
BOUND_PV=$(kubectl --kubeconfig "$KUBECONFIG" get pvc data-new -n bench -o jsonpath='{.spec.volumeName}')

if [[ "$BOUND_PV" == "bench-pv" ]]; then
  echo "PASS: PV is Bound to data-new PVC"
  exit 0
else
  echo "FAIL: PVC is bound to $BOUND_PV, expected bench-pv"
  exit 1
fi
