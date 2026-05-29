#!/bin/bash
set -euo pipefail

KUBECTL=(kubectl)
if [[ -n "${KUBECONFIG:-}" ]]; then
  KUBECTL=(kubectl --kubeconfig "$KUBECONFIG")
fi

fail() {
  echo "FAIL: $*"
  exit 1
}

namespace="bench"
expected_prod=(prod-api-1 prod-worker-1 prod-devtools-1)

dev_count="$("${KUBECTL[@]}" get pods -n "$namespace" -l env=dev --no-headers 2>/dev/null | wc -l | tr -d ' ')"
[[ "$dev_count" == "0" ]] || fail "expected 0 env=dev pods, found $dev_count"

prod_count="$("${KUBECTL[@]}" get pods -n "$namespace" -l env=prod --no-headers 2>/dev/null | wc -l | tr -d ' ')"
[[ "$prod_count" == "${#expected_prod[@]}" ]] ||
  fail "expected ${#expected_prod[@]} env=prod pods, found $prod_count"

for pod in "${expected_prod[@]}"; do
  phase="$("${KUBECTL[@]}" get pod "$pod" -n "$namespace" -o jsonpath='{.status.phase}' 2>/dev/null || true)"
  [[ "$phase" == "Running" ]] || fail "production pod $pod is not Running"

  deletion_timestamp="$("${KUBECTL[@]}" get pod "$pod" -n "$namespace" -o jsonpath='{.metadata.deletionTimestamp}' 2>/dev/null || true)"
  [[ -z "$deletion_timestamp" ]] || fail "production pod $pod is being deleted"
done

echo "PASS: stale dev pods removed and production pods preserved"
