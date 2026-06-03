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

readiness_port="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}' 2>/dev/null || true)"
liveness_port="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].livenessProbe.httpGet.port}' 2>/dev/null || true)"
[[ -n "$readiness_port" ]] || fail "readinessProbe was removed"
[[ -n "$liveness_port" ]] || fail "livenessProbe was removed"
[[ "$readiness_port" == "80" ]] || fail "readinessProbe port is ${readiness_port:-<none>}, want 80"
[[ "$liveness_port" == "80" ]] || fail "livenessProbe port is ${liveness_port:-<none>}, want 80"

readiness_path="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
liveness_path="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].livenessProbe.httpGet.path}' 2>/dev/null || true)"
[[ "$readiness_path" == "/" ]] || fail "readinessProbe path changed to ${readiness_path:-<none>}"
[[ "$liveness_path" == "/" ]] || fail "livenessProbe path changed to ${liveness_path:-<none>}"

readiness_threshold="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.failureThreshold}' 2>/dev/null || true)"
liveness_threshold="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].livenessProbe.failureThreshold}' 2>/dev/null || true)"
[[ "$readiness_threshold" == "3" ]] || fail "readinessProbe failureThreshold changed to ${readiness_threshold:-<none>}"
[[ "$liveness_threshold" == "1" ]] || fail "livenessProbe failureThreshold changed to ${liveness_threshold:-<none>}"

image="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
replicas="$("${KUBECTL[@]}" get deployment app -n bench -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
[[ "$image" == "nginx:1.27-alpine" ]] || fail "app deployment image changed to ${image:-<none>}"
[[ "$replicas" == "1" ]] || fail "app deployment replicas changed to ${replicas:-<none>}"

echo "PASS: app probes repaired without removing or masking health checks"
