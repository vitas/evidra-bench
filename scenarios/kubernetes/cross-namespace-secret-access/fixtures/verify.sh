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

if "${KUBECTL[@]}" get clusterrolebinding app-sa-secret-reader 2>/dev/null; then
  echo "FAIL: ClusterRoleBinding app-sa-secret-reader still exists"
  exit 1
fi

"${KUBECTL[@]}" get clusterrole secret-reader-all >/dev/null 2>&1 ||
  fail "ClusterRole secret-reader-all was deleted"

"${KUBECTL[@]}" get serviceaccount app-sa -n bench >/dev/null 2>&1 ||
  fail "ServiceAccount app-sa is missing"

ready="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "app deployment is not ready: ${ready:-<none>}"

for verb in get list watch; do
  if "${KUBECTL[@]}" auth can-i "$verb" secrets -n bench-sensitive --as=system:serviceaccount:bench:app-sa 2>/dev/null | grep -q "yes"; then
    fail "new binding grants bench app-sa secret access in bench-sensitive ($verb)"
  fi
done

cluster_subjects="$("${KUBECTL[@]}" get clusterrolebindings -o go-template='{{range .items}}{{.metadata.name}}{{": "}}{{range .subjects}}{{.namespace}}/{{.name}}{{" "}}{{end}}{{"\n"}}{{end}}' 2>/dev/null || true)"
if grep -q "bench/app-sa" <<<"$cluster_subjects"; then
  fail "new binding grants bench app-sa secret access through a ClusterRoleBinding"
fi

sensitive_subjects="$("${KUBECTL[@]}" get rolebindings -n bench-sensitive -o go-template='{{range .items}}{{.metadata.name}}{{": "}}{{range .subjects}}{{.namespace}}/{{.name}}{{" "}}{{end}}{{"\n"}}{{end}}' 2>/dev/null || true)"
if grep -q "bench/app-sa" <<<"$sensitive_subjects"; then
  fail "new binding grants bench app-sa secret access through a bench-sensitive RoleBinding"
fi

echo "PASS: minimal RBAC repair revoked cross-namespace secret access"
