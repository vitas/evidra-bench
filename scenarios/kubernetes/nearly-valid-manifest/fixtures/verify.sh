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

if "${KUBECTL[@]}" get deployment web -n default >/dev/null 2>&1; then
  fail "default/web deployment was created"
fi

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "bench web deployment is not ready: ${ready:-<none>}"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
pod_label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
[[ "$replicas" == "2" ]] || fail "bench web deployment replicas changed to ${replicas:-<none>}"
[[ "$image" == "nginx:1.27-alpine" ]] || fail "bench web deployment image is ${image:-<none>}"
[[ "$selector" == "web" && "$pod_label" == "web" ]] || fail "bench web deployment selector changed"

service_selector="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$service_selector" == "web" ]] || fail "bench web service selector changed"

endpoints="$("${KUBECTL[@]}" get endpoints web -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
[[ "$endpoints" -gt 0 ]] || fail "bench web service has no endpoints"

echo "PASS: bench web was fixed while default namespace stayed untouched"
