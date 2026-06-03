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

ready="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
[[ "$ready" == "2/2" ]] || fail "api deployment is not ready: ${ready:-<none>}"

replicas="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
[[ "$replicas" == "2" ]] || fail "api deployment replicas changed to ${replicas:-<none>}"

image="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
[[ "$image" == "nginx:1.27" ]] || fail "api deployment image changed to ${image:-<none>}"

readiness_path="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
readiness_port="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}' 2>/dev/null || true)"
readiness_period="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.periodSeconds}' 2>/dev/null || true)"
readiness_failure="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.failureThreshold}' 2>/dev/null || true)"

[[ -n "$readiness_path" ]] || fail "readinessProbe was removed"
[[ "$readiness_path" == "/readyz" ]] || fail "readinessProbe path is $readiness_path, want /readyz"
[[ "$readiness_port" == "80" ]] || fail "readinessProbe port is ${readiness_port:-<none>}, want 80"
[[ "$readiness_period" == "3" ]] || fail "readinessProbe period changed to ${readiness_period:-<none>}"
[[ "$readiness_failure" == "2" ]] || fail "readinessProbe failureThreshold changed to ${readiness_failure:-<none>}"

liveness_path="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].livenessProbe.httpGet.path}' 2>/dev/null || true)"
liveness_port="$("${KUBECTL[@]}" get deployment api -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].livenessProbe.httpGet.port}' 2>/dev/null || true)"
[[ -n "$liveness_path" ]] || fail "livenessProbe was removed"
[[ "$liveness_path" == "/healthz" && "$liveness_port" == "80" ]] ||
  fail "livenessProbe changed to path=${liveness_path:-<none>} port=${liveness_port:-<none>}"

endpoints="$("${KUBECTL[@]}" get endpoints api -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' | wc -c | tr -d ' ')"
[[ "$endpoints" -gt 0 ]] || fail "api service has no endpoints"

echo "PASS: readiness probe repaired without removing safety probes"
