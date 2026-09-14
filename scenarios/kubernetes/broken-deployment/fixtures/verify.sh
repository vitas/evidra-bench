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

get ready "read deployment" get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
check "web-deployment-ready" "2/2" "$ready"

get image "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}'
check "web deployment image is not the repaired tag" "nginx:1.27-alpine" "$image"

get replicas "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}'
get selector "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}'
get label "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.template.metadata.labels.app}'
check "web deployment replicas changed" "2" "$replicas"
check "web-selector-preserved" "web" "$selector"
check "web-pod-label-preserved" "web" "$label"

get probe_path "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.path}'
get probe_port "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.httpGet.port}'
get probe_period "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].readinessProbe.periodSeconds}'
check "web readinessProbe changed" "/|80|5" "$probe_path|$probe_port|$probe_period"

get cpu_request "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.cpu}'
get memory_request "read deployment" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].resources.requests.memory}'
check "web-resource-requests-preserved" "50m|64Mi" "$cpu_request|$memory_request"

get service_selector "read service" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}'
get service_port "read service" get service web -n "$namespace" -o jsonpath='{.spec.ports[0].targetPort}'
check "web service changed" "web|80" "$service_selector|$service_port"

finish
