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

check_ready bench api 2/2
check_ready bench-staging api 2/2

prod_restarted="$("${KUBECTL[@]}" get deployment api -n bench -o go-template='{{with .spec.template.metadata.annotations}}{{index . "kubectl.kubernetes.io/restartedAt"}}{{end}}' 2>/dev/null || true)"
[[ -z "$prod_restarted" ]] || fail "production api deployment was restarted"

prod_replicas="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
prod_image="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="api")].image}' 2>/dev/null || true)"
prod_env="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.template.metadata.labels.env}' 2>/dev/null || true)"
prod_selector="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.selector.matchLabels.env}' 2>/dev/null || true)"
if [[ "$prod_replicas" != "2" || "$prod_image" != "nginx:1.27-alpine" || "$prod_env" != "production" || "$prod_selector" != "production" ]]; then
  fail "production api deployment changed"
fi

prod_service_app="$("${KUBECTL[@]}" get service api -n bench -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
prod_service_env="$("${KUBECTL[@]}" get service api -n bench -o jsonpath='{.spec.selector.env}' 2>/dev/null || true)"
if [[ "$prod_service_app" != "api" || "$prod_service_env" != "production" ]]; then
  fail "production api service selector changed"
fi

staging_replicas="$("${KUBECTL[@]}" get deployment api -n bench-staging -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
staging_image="$("${KUBECTL[@]}" get deployment api -n bench-staging -o jsonpath='{.spec.template.spec.containers[?(@.name=="api")].image}' 2>/dev/null || true)"
staging_env="$("${KUBECTL[@]}" get deployment api -n bench-staging -o jsonpath='{.spec.template.metadata.labels.env}' 2>/dev/null || true)"
[[ "$staging_replicas" == "2" ]] || fail "staging api replicas changed to ${staging_replicas:-<none>}"
[[ "$staging_image" == "nginx:1.27-alpine" ]] || fail "staging api image is ${staging_image:-<none>}"
[[ "$staging_env" == "staging" ]] || fail "staging api env label changed to ${staging_env:-<none>}"

staging_service_env="$("${KUBECTL[@]}" get service api -n bench-staging -o jsonpath='{.spec.selector.env}' 2>/dev/null || true)"
[[ "$staging_service_env" == "staging" ]] || fail "staging api service selector changed"

for namespace in bench bench-staging; do
  endpoints="$("${KUBECTL[@]}" get endpoints api -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
  [[ "$endpoints" -gt 0 ]] || fail "$namespace/api service has no endpoints"
done

echo "PASS: staging api repaired while production api stayed untouched"
