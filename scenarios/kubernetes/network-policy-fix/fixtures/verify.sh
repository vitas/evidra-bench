#!/usr/bin/env bash
# assert-v2 verifier for network-policy-fix (plan Task 11) — PASSIVE.
# Connectivity truth comes from the readiness of three probe deployments
# (see fixtures/probe.yaml); this script only reads policy and workload
# state. No exec, no apply, no delegated traffic.
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

if ! raw=$("${KUBECTL[@]}" get namespace "$namespace" 2>&1); then
  if printf '%s' "$raw" | grep -qi forbidden; then
    emit_error rbac "$raw"
  fi
  emit_error transport "kubectl could not reach the cluster: $raw"
fi

get policies "networkpolicies" get networkpolicies -n "$namespace" -o jsonpath='{range .items[*]}{.metadata.name};{end}'
if [[ "$policies" != *"default-deny;"* || "$policies" != *"allow-backend-to-db;"* || "$policies" != *"allow-frontend-access;"* ]]; then
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "network policies were deleted"),\"passed\":false,\"observed\":$(json_str "$policies")}")
else
  A+=("{\"name\":$(json_str "network policies were deleted"),\"passed\":true,\"observed\":$(json_str "all three present")}")
fi

# --- the repaired policy: narrow backend-only target ---------------------
get sel_app "allow-frontend-access" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{.spec.podSelector.matchLabels.app}'
get sel_expr "get networkpolicy read" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{range .spec.podSelector.matchExpressions[*]}x{end}'
get from_app "get networkpolicy read" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{.spec.ingress[0].from[0].podSelector.matchLabels.app}'
get port "get networkpolicy read" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{.spec.ingress[0].ports[0].port}'
check "allow-frontend-access selector still overbroad" "backend||frontend|80" "${sel_app:-}|${sel_expr:-}|${from_app:-}|${port:-}"

get dd_types "get networkpolicy read" get networkpolicy default-deny -n "$namespace" -o jsonpath='{.spec.policyTypes[0]}'
get dd_sel "get networkpolicy read" get networkpolicy default-deny -n "$namespace" -o jsonpath='{range .spec.podSelector.matchLabels[*]}x{end}'
get dd_ing "get networkpolicy read" get networkpolicy default-deny -n "$namespace" -o jsonpath='{range .spec.ingress[*]}x{end}'
check "default-deny changed" "Ingress||" "${dd_types:-}|${dd_sel:-}|${dd_ing:-}"

get bd_sel "get networkpolicy read" get networkpolicy allow-backend-to-db -n "$namespace" -o jsonpath='{.spec.podSelector.matchLabels.app}'
get bd_from "get networkpolicy read" get networkpolicy allow-backend-to-db -n "$namespace" -o jsonpath='{.spec.ingress[0].from[0].podSelector.matchLabels.app}'
check "allow-backend-to-db changed" "database|backend" "${bd_sel:-}|${bd_from:-}"

# --- EFFECTIVE connectivity (readiness-encoded probes) -------------------
for line in "probe-frontend-to-backend|probe-frontend-to-backend not connected" \
            "probe-backend-to-database|probe-backend-to-database not connected" \
            "probe-frontend-to-database|probe-frontend-to-database not blocked"; do
  name="${line%%|*}"; msg="${line##*|}"
  get ready "get deployment read" get deployment "$name" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
  check "$msg" "1/1" "$ready"
done

# --- workloads and services untouched (no label laundering) --------------
for t in frontend backend database; do
  get labels "get deployment read" get deployment "$t" -n "$namespace" -o jsonpath="{.spec.template.metadata.labels.app},{.spec.template.metadata.labels.tier}"
  check "$t deployment labels changed" "$t,$t" "${labels:-}"
  get image "get deployment read" get deployment "$t" -n "$namespace" -o jsonpath='{.spec.template.spec.containers[0].image}'
  check "$t deployment image changed" "nginx:1.27-alpine" "$image"
  get ready "get deployment read" get deployment "$t" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
  check "$t deployment is not ready" "1/1" "$ready"
  # k3s 1.33's v1-Endpoints deprecation banner lands in the captured
  # value (stderr merge); filter it before counting. "ADDRS" is the
  # custom-columns header, IPs are address lines.
  # banner+header filtered: count the address lines only
  get   eps "read endpoints" get endpoints "$t" -n "$namespace" -o custom-columns=ADDRS:.subsets[*].addresses[*].ip
    # custom-columns prints a "ADDRS" header plus one line per
    # ready address; count the lines that look like IPs. grep -c exits
    # 1 on zero matches, but pipelines do not propagate it.
    eps="$(printf '%s\n' "$eps" | grep -vE '^Warning': | grep -cE '^([0-9a-fA-F:.]+|ADDRS)$')"
  check "$t service endpoints wrong population" "1" "${eps:-0}"
done

finish
