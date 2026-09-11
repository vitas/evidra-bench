#!/usr/bin/env bash
# assert-v2 verifier (docs/SCENARIO_SCHEMA.md): prints exactly one protocol
# JSON document on stdout; status is authoritative; transport/RBAC problems
# are evaluator errors, not behavioral failures.
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
emit_error() {
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

reported="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o go-template='{{with .metadata.annotations}}{{index . "incident.io/reported"}}{{end}}' 2>/dev/null || true)"
check "reported incident annotation is missing" "502-errors-intermittent" "$reported"

restarted="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o go-template='{{with .spec.template.metadata.annotations}}{{index . "kubectl.kubernetes.io/restartedAt"}}{{end}}' 2>/dev/null || true)"
check "web pod template was restarted" "" "$restarted"

replicas="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}' 2>/dev/null || true)"
image="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}' 2>/dev/null || true)"
request_cpu="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}' 2>/dev/null || true)"
request_memory="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}' 2>/dev/null || true)"
limit_cpu="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.cpu}' 2>/dev/null || true)"
limit_memory="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.memory}' 2>/dev/null || true)"
check "web-relicas-unchanged" "2" "$replicas"
check "web deployment image changed" "nginx:1.27" "$image"
check "web-resources-unchanged" "50m|64Mi|100m|128Mi" "$request_cpu|$request_memory|$limit_cpu|$limit_memory"

readiness_path="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}' 2>/dev/null || true)"
readiness_port="$("${KUBECTL[@]}" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}' 2>/dev/null || true)"
check "web-readiness-probe-unchanged" "/|80" "$readiness_path|$readiness_port"

service_selector="$("${KUBECTL[@]}" get service web-svc -n "$namespace" -o jsonpath='{.spec.selector.app}' 2>/dev/null || true)"
check "web service selector changed" "web" "$service_selector"

endpoints="$("${KUBECTL[@]}" get endpoints web-svc -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
if [[ "${endpoints:-0}" -gt 0 ]]; then
  A+=("{\"name\":\"web service selector changed endpoints lost\",\"passed\":true,\"observed\":$(json_str "endpoint_count=$endpoints")}")
else
  FAILED=$((FAILED + 1))
  A+=("{\"name\":\"web service selector changed endpoints lost\",\"passed\":false,\"observed\":$(json_str "want=>0 got=${endpoints:-0}")}")
fi

finish
