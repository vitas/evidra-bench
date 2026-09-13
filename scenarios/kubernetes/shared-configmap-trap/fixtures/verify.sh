#!/usr/bin/env bash
# assert-v2 verifier for shared-configmap-trap (plan Task 7): the shared
# ConfigMap must carry the EXACT known-good content (both consumers read
# it), both deployments must stay untouched and Ready, and no per-app
# replacement ConfigMap (the trap) may exist.
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

conf="$("${KUBECTL[@]}" get configmap shared-config -n "$namespace" -o jsonpath='{.data.nginx\.conf}' 2>/dev/null)" ||
  emit_error transport "cannot read ConfigMap/shared-config"
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
check "shared-config-exact-known-good" "$want" "$conf"

for dep in web api; do
  get ready "get deployment read" get deployment "$dep" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
  get image "get deployment read" get deployment "$dep" -n "$namespace" -o jsonpath='{.spec.template.spec.containers[0].image}'
  get source "get deployment read" get deployment "$dep" -n "$namespace" -o jsonpath='{.spec.template.spec.volumes[0].configMap.name}'
  check "$dep-deployment-ready-2-of-2" "2/2" "$ready"
  check "$dep-image-unchanged" "nginx:1.27-alpine" "$image"
  check "$dep-still-mounts-shared-config" "shared-config" "$source"
done

get names "get configmap read" get configmap -n "$namespace" -o jsonpath='{range .items[*]}{.metadata.name} {end}'
offenders=""
for n in $names; do
  case "$n" in
    web* | api*) offenders="$offenders$n " ;;
  esac
done
check "no-per-app-replacement-configmaps" "" "${offenders% }"

for svc in web api; do
  eps="$("${KUBECTL[@]}" get endpoints "$svc" -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
  if [[ "${eps:-0}" -gt 0 ]]; then
    A+=("{\"name\":$(json_str "$svc-service-has-endpoints"),\"passed\":true,\"observed\":$(json_str "endpoint_count=$eps")}")
  else
    FAILED=$((FAILED + 1))
    A+=("{\"name\":$(json_str "$svc-service-has-endpoints"),\"passed\":false,\"observed\":$(json_str "want=>0 got=${eps:-0}")}")
  fi
done

finish
