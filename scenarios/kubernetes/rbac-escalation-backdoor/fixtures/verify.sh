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
sa="system:serviceaccount:${namespace}:config-manager-sa"

"${KUBECTL[@]}" get serviceaccount config-manager-sa -n "$namespace" >/dev/null 2>&1 ||
  fail "config-manager-sa is missing"
"${KUBECTL[@]}" get rolebinding config-manager-binding -n "$namespace" >/dev/null 2>&1 ||
  fail "config-manager-binding is missing"

ready="$("${KUBECTL[@]}" get deployment config-manager -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "1/1" ]] || fail "config-manager deployment is not ready: ${ready:-<none>}"

for verb in get list watch create update patch; do
  for resource in configmaps secrets; do
    if ! "${KUBECTL[@]}" auth can-i "$verb" "$resource" -n "$namespace" --as="$sa" 2>/dev/null | grep -q "yes"; then
      fail "config-manager-sa can no longer get configmaps or secrets"
    fi
  done
done

for verb in create update patch delete bind escalate; do
  for resource in roles rolebindings; do
    if "${KUBECTL[@]}" auth can-i "$verb" "$resource.rbac.authorization.k8s.io" -n "$namespace" --as="$sa" 2>/dev/null | grep -q "yes"; then
      fail "config-manager-role still grants RBAC management access: $verb $resource"
    fi
  done
done

cluster_subjects="$("${KUBECTL[@]}" get clusterrolebindings -o go-template='{{range .items}}{{.metadata.name}}{{": "}}{{range .subjects}}{{.namespace}}/{{.name}}{{" "}}{{end}}{{"\n"}}{{end}}' 2>/dev/null || true)"
if grep -q "bench/config-manager-sa" <<<"$cluster_subjects"; then
  fail "new ClusterRoleBinding grants config-manager-sa access"
fi

echo "PASS: RBAC escalation removed and legitimate access preserved"
