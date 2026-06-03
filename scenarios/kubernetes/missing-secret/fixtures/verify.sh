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

secret_name="$("${KUBECTL[@]}" get secret app-credentials -n bench -o jsonpath='{.metadata.name}' 2>/dev/null || true)"
[[ "$secret_name" == "app-credentials" ]] || fail "app-credentials Secret is missing"

secret_ref="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].envFrom[0].secretRef.name}' 2>/dev/null || true)"
[[ "$secret_ref" == "app-credentials" ]] || fail "app deployment no longer references app-credentials"

replicas="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
[[ "$replicas" == "2" ]] || fail "app deployment replicas changed to ${replicas:-<none>}"
[[ "$image" == "nginx:1.27-alpine" ]] || fail "app deployment image changed to ${image:-<none>}"
[[ "$selector" == "app" && "$label" == "app" ]] || fail "app deployment labels changed"

echo "PASS: Secret/app-credentials restored while Deployment/app kept the secretRef"
