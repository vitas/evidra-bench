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

get() { # get <var> <desc> <kubectl args...> : STRICT top-level read
  # A transport/API failure is an EVALUATOR fault (status:error => the run
  # grades INCOMPLETE), never a silently empty value that blames the agent.
  # Forbidden reads are evaluator-fault by contract too (scenario RBAC must
  # be provably sufficient). NotFound remains DATA: absent objects are what
  # assertions observe. One bounded retry absorbs transient API hiccups.
  local __v="$1" __d="$2"; shift 2
  local __out="" __rc=1 __try
  for __try in 1 2; do
    if __out=$("${KUBECTL[@]}" "$@" 2>&1); then __rc=0; break; fi
    if printf '%s' "$__out" | grep -qi forbidden; then
      emit_error rbac "$__d read denied: $__out"
    fi
    if printf '%s' "$__out" | grep -qE 'NotFound|not found'; then
      __out=""; __rc=0; break
    fi
    [[ "$__try" == 1 ]] && sleep 2
  done
  if [[ "$__rc" != 0 ]]; then
    emit_error transport "$__d read failed (evaluator-side): $__out"
  fi
  printf -v "$__v" '%s' "$__out"
}

get ready "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
check "web-deployment-ready" "2/2" "$ready"

get reported "get deployment read" get deployment web -n "$namespace" -o go-template='{{with .metadata.annotations}}{{index . "incident.io/reported"}}{{end}}'
check "reported incident annotation is missing" "502-errors-intermittent" "$reported"

get restarted "get deployment read" get deployment web -n "$namespace" -o go-template='{{with .spec.template.metadata.annotations}}{{index . "kubectl.kubernetes.io/restartedAt"}}{{end}}'
check "web pod template was restarted" "" "$restarted"

get replicas "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}'
get image "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}'
get request_cpu "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}'
get request_memory "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}'
get limit_cpu "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.cpu}'
get limit_memory "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.limits.memory}'
check "web-relicas-unchanged" "2" "$replicas"
check "web deployment image changed" "nginx:1.27" "$image"
check "web-resources-unchanged" "50m|64Mi|100m|128Mi" "$request_cpu|$request_memory|$limit_cpu|$limit_memory"

get readiness_path "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}'
get readiness_port "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}'
check "web-readiness-probe-unchanged" "/|80" "$readiness_path|$readiness_port"

get service_selector "get service read" get service web-svc -n "$namespace" -o jsonpath='{.spec.selector.app}'
check "web service selector changed" "web" "$service_selector"

get endpoints "endpoints-read" get endpoints web-svc -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}'
endpoints="${#endpoints}"
if [[ "${endpoints:-0}" -gt 0 ]]; then
  A+=("{\"name\":\"web service selector changed endpoints lost\",\"passed\":true,\"observed\":$(json_str "endpoint_count=$endpoints")}")
else
  FAILED=$((FAILED + 1))
  A+=("{\"name\":\"web service selector changed endpoints lost\",\"passed\":false,\"observed\":$(json_str "want=>0 got=${endpoints:-0}")}")
fi

finish
