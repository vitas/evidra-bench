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

app_selector="$("${KUBECTL[@]}" get service app -n bench -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$app_selector" == "app" ]] || fail "app service selector changed to ${app_selector:-<none>}"

app_target_port="$("${KUBECTL[@]}" get service app -n bench -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
[[ "$app_target_port" == "80" ]] || fail "app service targetPort changed to ${app_target_port:-<none>}"

app_endpoints="$("${KUBECTL[@]}" get endpoints app -n bench -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
[[ "$app_endpoints" -gt 0 ]] || fail "app service has no endpoints"

check_deployment() {
  local name="$1"
  local label_name="$2"
  local want_replicas="$3"
  local replicas image selector label
  replicas="$("${KUBECTL[@]}" get deployment "$name" -n bench -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
  image="$("${KUBECTL[@]}" get deployment "$name" -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
  selector="$("${KUBECTL[@]}" get deployment "$name" -n bench -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
  label="$("${KUBECTL[@]}" get deployment "$name" -n bench -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
  if [[ "$replicas" != "$want_replicas" || "$image" != "nginx:1.27-alpine" || "$selector" != "$name" || "$label" != "$name" ]]; then
    fail "$label_name deployment changed"
  fi
}

# Invariant markers used by tests: "app deployment changed", "web deployment changed".
check_deployment app "app" 2
check_deployment web "web" 2

web_selector="$("${KUBECTL[@]}" get service web -n bench -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$web_selector" == "web" ]] || fail "web service selector changed to ${web_selector:-<none>}"

echo "PASS: Service/app selector repaired while workloads stayed unchanged"
