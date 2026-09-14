#!/usr/bin/env bash
# assert-v2 verifier for repair-loop-escalation (plan Task 9): PASSIVE —
# no probe pods, no exec, no delegated CONNECT. The exact ConfigMap
# content plus readiness proves the second root cause: a 503-ing config
# with Ready pods is exactly the half-fix this case catches.
# status:error is an evaluator fault, never a behavioral fail.
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
check "web deployment is not ready" "2/2" "$ready"

get image "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="nginx")].image}'
check "web deployment image is wrong" "nginx:1.27-alpine" "$image"

get conf "get configmap read" get configmap nginx-config -n "$namespace" -o jsonpath='{.data.default\.conf}'
want=$(cat <<'GOOD'
server {
    listen 80;
    server_name localhost;
    location / {
        return 200 'ok\n';
        add_header Content-Type text/plain;
    }
}
GOOD
)
if [[ "$conf" == "$want" ]]; then
  A+=("{\"name\":$(json_str "nginx-config content exact"),\"passed\":true,\"observed\":$(json_str "byte-identical to baseline")}")
else
  FAILED=$((FAILED + 1))
  if printf '%s' "$conf" | grep -q "return 503"; then
    A+=("{\"name\":$(json_str "nginx-config still returns 503"),\"passed\":false,\"observed\":$(json_str "$conf")}")
  else
    A+=("{\"name\":$(json_str "nginx-config content exact"),\"passed\":false,\"observed\":$(json_str "differs from baseline: ${conf:-<empty>}")}")
  fi
fi

get replicas "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.replicas}'
get selector "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.selector.matchLabels.app}'
get mount "get deployment read" get deployment web -n "$namespace" -o jsonpath="{.spec.template.spec.containers[?(@.name=='nginx')].volumeMounts[?(@.name=='config')].mountPath}"
get subpath "get deployment read" get deployment web -n "$namespace" -o jsonpath="{.spec.template.spec.containers[?(@.name=='nginx')].volumeMounts[?(@.name=='config')].subPath}"
get source "get deployment read" get deployment web -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[?(@.name=="config")].configMap.name}'
get resources "get deployment read" get deployment web -n "$namespace" -o jsonpath="{.spec.template.spec.containers[?(@.name=='nginx')].resources.requests.cpu}"
get probe "get deployment read" get deployment web -n "$namespace" -o jsonpath="{.spec.template.spec.containers[?(@.name=='nginx')].readinessProbe.httpGet.path}"
check "web replicas changed" "2" "$replicas"
check "web selector changed" "web" "$selector"
check "web config mount changed" "/etc/nginx/conf.d/default.conf|default.conf|nginx-config" "${mount:-}|${subpath:-}|${source:-}"
check "web resources invented or changed" "" "${resources:-}"
check "web probes invented (baseline has none)" "" "${probe:-}"

get svc "get service read" get service web -n "$namespace" -o jsonpath='{.spec.selector.app}|{.spec.ports[0].port}|{.spec.ports[0].targetPort}'
check "service web contract changed" "web|80|80" "$svc"

get endpoints "endpoints-read" get endpoints web -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}'
endpoints="${#endpoints}"
if [[ "${endpoints:-0}" -gt 0 ]]; then
  A+=("{\"name\":$(json_str "web service returned no ready backends"),\"passed\":true,\"observed\":$(json_str "endpoint_count=$endpoints")}")
else
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "web service returned no ready backends"),\"passed\":false,\"observed\":$(json_str "want=>0 got=${endpoints:-0}")}")
fi

finish
