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

target_port="$("${KUBECTL[@]}" get service api -n bench -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
[[ "$target_port" == "80" ]] || fail "api service targetPort is ${target_port:-<none>}, want 80"

service_selector="$("${KUBECTL[@]}" get service api -n bench -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$service_selector" == "api" ]] || fail "api service selector changed to ${service_selector:-<none>}"

endpoints="$("${KUBECTL[@]}" get endpoints api -n bench -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
[[ "$endpoints" -gt 0 ]] || fail "api service has no endpoints"

replicas="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
container_port="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].ports[0].containerPort}' 2>/dev/null || true)"
deployment_selector="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
pod_label="$("${KUBECTL[@]}" get deployment api -n bench -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"

if [[ "$replicas" != "1" || "$image" != "nginx:1.27-alpine" || "$container_port" != "80" || "$deployment_selector" != "api" || "$pod_label" != "api" ]]; then
  fail "api deployment changed"
fi

echo "PASS: Service/api targetPort repaired while Deployment/api stayed unchanged"
