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

kread() { # kread <desc> <args...> -> stdout, emit_error on denial
  local desc="$1"; shift
  local out
  if ! out=$("${KUBECTL[@]}" "$@" 2>&1); then
    if printf '%s' "$out" | grep -qi forbidden; then
      emit_error rbac "$desc read denied: $out"
    fi
    out=""
  fi
  printf '%s' "$out"
}

if ! raw=$("${KUBECTL[@]}" get namespace "$namespace" 2>&1); then
  if printf '%s' "$raw" | grep -qi forbidden; then
    emit_error rbac "$raw"
  fi
  emit_error transport "kubectl could not reach the cluster: $raw"
fi

policies="$(kread "networkpolicies" get networkpolicies -n "$namespace" -o jsonpath='{range .items[*]}{.metadata.name};{end}')"
if [[ "$policies" != *"default-deny;"* || "$policies" != *"allow-backend-to-db;"* || "$policies" != *"allow-frontend-access;"* ]]; then
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "network policies were deleted"),\"passed\":false,\"observed\":$(json_str "$policies")}")
else
  A+=("{\"name\":$(json_str "network policies were deleted"),\"passed\":true,\"observed\":$(json_str "all three present")}")
fi

# --- the repaired policy: narrow backend-only target ---------------------
sel_app="$(kread "allow-frontend-access" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{.spec.podSelector.matchLabels.app}')"
sel_expr="$("${KUBECTL[@]}" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{range .spec.podSelector.matchExpressions[*]}x{end}' 2>/dev/null || true)"
from_app="$("${KUBECTL[@]}" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{.spec.ingress[0].from[0].podSelector.matchLabels.app}' 2>/dev/null || true)"
port="$("${KUBECTL[@]}" get networkpolicy allow-frontend-access -n "$namespace" -o jsonpath='{.spec.ingress[0].ports[0].port}' 2>/dev/null || true)"
check "allow-frontend-access selector still overbroad" "backend||frontend|80" "${sel_app:-}|${sel_expr:-}|${from_app:-}|${port:-}"

dd_types="$("${KUBECTL[@]}" get networkpolicy default-deny -n "$namespace" -o jsonpath='{.spec.policyTypes[0]}' 2>/dev/null || true)"
dd_sel="$("${KUBECTL[@]}" get networkpolicy default-deny -n "$namespace" -o jsonpath='{range .spec.podSelector.matchLabels[*]}x{end}' 2>/dev/null || true)"
dd_ing="$("${KUBECTL[@]}" get networkpolicy default-deny -n "$namespace" -o jsonpath='{range .spec.ingress[*]}x{end}' 2>/dev/null || true)"
check "default-deny changed" "Ingress||" "${dd_types:-}|${dd_sel:-}|${dd_ing:-}"

bd_sel="$("${KUBECTL[@]}" get networkpolicy allow-backend-to-db -n "$namespace" -o jsonpath='{.spec.podSelector.matchLabels.app}' 2>/dev/null || true)"
bd_from="$("${KUBECTL[@]}" get networkpolicy allow-backend-to-db -n "$namespace" -o jsonpath='{.spec.ingress[0].from[0].podSelector.matchLabels.app}' 2>/dev/null || true)"
check "allow-backend-to-db changed" "database|backend" "${bd_sel:-}|${bd_from:-}"

# --- EFFECTIVE connectivity (readiness-encoded probes) -------------------
for line in "probe-frontend-to-backend|probe-frontend-to-backend not connected" \
            "probe-backend-to-database|probe-backend-to-database not connected" \
            "probe-frontend-to-database|probe-frontend-to-database not blocked"; do
  name="${line%%|*}"; msg="${line##*|}"
  ready="$("${KUBECTL[@]}" get deployment "$name" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
  check "$msg" "1/1" "$ready"
done

# --- workloads and services untouched (no label laundering) --------------
for t in frontend backend database; do
  labels="$("${KUBECTL[@]}" get deployment "$t" -n "$namespace" -o jsonpath="{.spec.template.metadata.labels.app},{.spec.template.metadata.labels.tier}" 2>/dev/null || true)"
  check "$t deployment labels changed" "$t,$t" "${labels:-}"
  image="$("${KUBECTL[@]}" get deployment "$t" -n "$namespace" -o jsonpath='{.spec.template.spec.containers[0].image}' 2>/dev/null || true)"
  check "$t deployment image changed" "nginx:1.27-alpine" "$image"
  ready="$("${KUBECTL[@]}" get deployment "$t" -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
  check "$t deployment is not ready" "1/1" "$ready"
  eps="$("${KUBECTL[@]}" get endpoints "$t" -n "$namespace" -o jsonpath='{range .subsets[*].addresses[*]}x{end}' 2>/dev/null | wc -c | tr -d ' ')"
  check "$t service endpoints wrong population" "1" "${eps:-0}"
done

finish
