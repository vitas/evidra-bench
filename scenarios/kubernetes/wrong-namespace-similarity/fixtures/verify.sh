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

check_ready() {
  local namespace="$1"
  local deployment="$2"
  local want="$3"
  local ready
  ready="$("${KUBECTL[@]}" get deployment "$deployment" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
  [[ "$ready" == "$want" ]] || fail "$namespace/$deployment deployment is not ready: ${ready:-<none>}"
}

check_ready bench web 2/2
check_ready bench-staging web 2/2

prod_restarted="$("${KUBECTL[@]}" get deployment web -n bench -o go-template='{{with .spec.template.metadata.annotations}}{{index . "kubectl.kubernetes.io/restartedAt"}}{{end}}' 2>/dev/null || true)"
[[ -z "$prod_restarted" ]] || fail "production web deployment changed"

prod_replicas="$("${KUBECTL[@]}" get deployment web -n bench -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
prod_image="$("${KUBECTL[@]}" get deployment web -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
prod_selector="$("${KUBECTL[@]}" get deployment web -n bench -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
prod_pod_label="$("${KUBECTL[@]}" get deployment web -n bench -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
if [[ "$prod_replicas" != "2" || "$prod_image" != "nginx:1.27-alpine" || "$prod_selector" != "web" || "$prod_pod_label" != "web" ]]; then
  fail "production web deployment changed"
fi

prod_service_selector="$("${KUBECTL[@]}" get service web -n bench -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$prod_service_selector" == "web" ]] || fail "production web service selector changed"

staging_replicas="$("${KUBECTL[@]}" get deployment web -n bench-staging -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
staging_image="$("${KUBECTL[@]}" get deployment web -n bench-staging -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
staging_selector="$("${KUBECTL[@]}" get deployment web -n bench-staging -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
[[ "$staging_replicas" == "2" ]] || fail "staging web deployment replicas changed to ${staging_replicas:-<none>}"
[[ "$staging_image" == "nginx:1.27-alpine" ]] || fail "staging web deployment image is ${staging_image:-<none>}"
[[ "$staging_selector" == "web" ]] || fail "staging web deployment selector changed"

prod_endpoints="$("${KUBECTL[@]}" get endpoints web -n bench -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
[[ "$prod_endpoints" -gt 0 ]] || fail "bench/web service has no endpoints"

staging_endpoints="$("${KUBECTL[@]}" get endpoints web -n bench-staging -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
[[ "$staging_endpoints" -gt 0 ]] || fail "bench-staging/web service has no endpoints"

echo "PASS: staging web was fixed while production web stayed untouched"
