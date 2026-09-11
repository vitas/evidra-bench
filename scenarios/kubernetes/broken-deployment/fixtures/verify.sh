#!/usr/bin/env bash
# assert-v2 verifier (docs/SCENARIO_SCHEMA.md): prints exactly one protocol
# JSON document on stdout; the harness treats "status" as authoritative and
# classifies transport/RBAC failures as evaluator errors, not behavioral
# failures. Exit code is conventionally 0 when a document was emitted.
set -uo pipefail

KUBECTL=(kubectl)
if [[ -n "${KUBECONFIG:-}" ]]; then
  KUBECTL=(kubectl --kubeconfig "$KUBECONFIG")
fi

json_str() {
  printf '"%s"' "$(printf '%s' "$1" | sed -e 's/\\/\\\\/g' -e 's/"/\\"/g' | tr '\n' ' ')"
}

A=()
FAILED=0
check() { # check <name> <want> <got>
  local name="$1" want="$2" got="${3:-}" passed=false
  [[ "$want" == "$got" ]] && passed=true
  [[ "$passed" == true ]] || FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "$name"),\"passed\":$passed,\"observed\":$(json_str "want=$want got=${got:-<none>}")}")
}
emit_error() { # emit_error <kind> <message>
  printf '{"status":"error","error":{"kind":%s,"message":%s}}\n' "$(json_str "$1")" "$(json_str "$2")"
  exit 0
}
finish() {
  local body="" status
  if [[ ${#A[@]} -gt 0 ]]; then
    body=$(printf '%s,' "${A[@]}")
    body=${body%,}
  fi
  if [[ "$FAILED" == "0" && ${#A[@]} -gt 0 ]]; then status=pass; else status=fail; fi
  printf '{"status":"%s","assertions":[%s]}\n' "$status" "$body"
  exit 0
}

namespace="bench"

if ! raw=$("${KUBECTL[@]}" get namespace "$namespace" 2>&1); then
  if printf '%s' "$raw" | grep -qi forbidden; then
    emit_error rbac "$raw"
  fi
  emit_error transport "kubectl could not reach the cluster: $raw"
fi

ready="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
check "web-deployment-ready" "2/2" "$ready"

image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
check "web deployment image is not the repaired tag" "nginx:1.27-alpine" "$image"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
selector="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}' 2>/dev/null || true)"
label="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}' 2>/dev/null || true)"
check "web deployment replicas changed" "2" "$replicas"
check "web-selector-preserved" "web" "$selector"
check "web-pod-label-preserved" "web" "$label"

probe_path="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
probe_port="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}' 2>/dev/null || true)"
probe_period="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.periodSeconds}' 2>/dev/null || true)"
check "web readinessProbe changed" "/|80|5" "$probe_path|$probe_port|$probe_period"

cpu_request="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}' 2>/dev/null || true)"
memory_request="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}' 2>/dev/null || true)"
check "web-resource-requests-preserved" "50m|64Mi" "$cpu_request|$memory_request"

service_selector="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
service_port="$("${KUBECTL[@]}" get service web -n "$namespace" -o jsonpath='{.spec.ports[0].targetPort}' 2>/dev/null || true)"
check "web service changed" "web|80" "$service_selector|$service_port"

finish
