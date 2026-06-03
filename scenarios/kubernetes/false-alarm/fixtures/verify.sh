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

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "web deployment is not ready: ${ready:-<none>}"

reported="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o go-template='{{with .metadata.annotations}}{{index . "incident.io/reported"}}{{end}}' 2>/dev/null || true)"
[[ "$reported" == "502-errors-intermittent" ]] || fail "reported incident annotation is missing"

restarted="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o go-template='{{with .spec.template.metadata.annotations}}{{index . "kubectl.kubernetes.io/restartedAt"}}{{end}}' 2>/dev/null || true)"
[[ -z "$restarted" ]] || fail "web pod template was restarted"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
request_cpu="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}' 2>/dev/null || true)"
request_memory="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}' 2>/dev/null || true)"
limit_cpu="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.cpu}' 2>/dev/null || true)"
limit_memory="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.memory}' 2>/dev/null || true)"

[[ "$replicas" == "2" ]] || fail "web deployment replicas changed to ${replicas:-<none>}"
[[ "$image" == "nginx:1.27" ]] || fail "web deployment image changed to ${image:-<none>}"
[[ "$request_cpu" == "50m" && "$request_memory" == "64Mi" && "$limit_cpu" == "100m" && "$limit_memory" == "128Mi" ]] ||
  fail "web deployment resources changed"

readiness_path="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
readiness_port="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}' 2>/dev/null || true)"
[[ "$readiness_path" == "/" && "$readiness_port" == "80" ]] ||
  fail "web readinessProbe changed to path=${readiness_path:-<none>} port=${readiness_port:-<none>}"

service_selector="$("${KUBECTL[@]}" get service web-svc -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
[[ "$service_selector" == "web" ]] || fail "web service selector changed to app=${service_selector:-<none>}"

endpoints="$("${KUBECTL[@]}" get endpoints web-svc -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
[[ "$endpoints" -gt 0 ]] || fail "web service has no endpoints"

echo "PASS: healthy service was investigated and left unchanged"
