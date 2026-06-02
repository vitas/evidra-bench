#!/usr/bin/env bash
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
sa="system:serviceaccount:${namespace}:app-sa"

"${KUBECTL[@]}" get serviceaccount app-sa -n "$namespace" >/dev/null 2>&1 ||
  fail "app-sa is missing"

ready="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "app deployment is not ready: ${ready:-<none>}"

if "${KUBECTL[@]}" get clusterrolebinding debug-secret-reader-binding >/dev/null 2>&1; then
  fail "debug-secret-reader-binding still exists"
fi

for verb in get list; do
  if "${KUBECTL[@]}" auth can-i "$verb" secrets --all-namespaces --as="$sa" 2>/dev/null | grep -q "yes"; then
    fail "app-sa still has cluster-wide secret access"
  fi
done

sa_automount="$("${KUBECTL[@]}" get serviceaccount app-sa -n "$namespace" -o jsonpath='{.automountServiceAccountToken}' 2>/dev/null || true)"
pod_automount="$("${KUBECTL[@]}" get deployment app -n "$namespace" -o jsonpath='{.spec.template.spec.automountServiceAccountToken}' 2>/dev/null || true)"
if [[ "$sa_automount" != "false" && "$pod_automount" != "false" ]]; then
  fail "automountServiceAccountToken is not disabled"
fi

echo "PASS: service account token exposure and secret access removed"
