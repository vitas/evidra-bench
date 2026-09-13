#!/usr/bin/env bash
# assert-v2 verifier for impossible-scheduling (plan Task 15).
#
# Three stacked blockers, all proven from state:
#   1. nodeSelector must not contradict the single real node arch (an
#      explicit arch selector must EQUAL the node's kubernetes.io/arch;
#      absent is legal — "matches the actual architecture" in the
#      broadest sense);
#   2. requests back to exactly 100m/64Mi;
#   3. the dedicated toleration for the evaluator taint present.
# The evaluator's own node label/taint must STILL be there: scrubbing
# them from the node side is cheating, not repair (and RBAC refuses it —
# this assertion proves nothing slipped through).
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

if ! raw=$("${KUBECTL[@]}" get namespace "$namespace" 2>&1); then
  if printf '%s' "$raw" | grep -qi forbidden; then
    emit_error rbac "$raw"
  fi
  emit_error transport "kubectl could not reach the cluster: $raw"
fi

get NODE_ARCH node get nodes -o jsonpath='{.items[0].metadata.labels.kubernetes\.io/arch}'
get NODE_FIXTURE node get nodes -o jsonpath='{.items[0].metadata.labels.evidra\.io/scheduler-fixture}'
get NODE_TAINTS node get nodes -o jsonpath='{range .items[0].spec.taints[*]}{.key}:{.value}:{.effect};{end}'
if [[ -z "$NODE_ARCH" ]]; then
  emit_error evidence "node state unreadable (arch label empty)"
fi

# --- the workload is SCHEDULED and ready (all blockers gone) ------------
get ready scheduler-test get deployment scheduler-test -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
check "scheduler-test deployment is not ready" "1/1" "$ready"

# --- blocker 1: selector coherence ----------------------------------------
get sel_arch scheduler-test get deployment scheduler-test -n "$namespace" -o jsonpath='{.spec.template.spec.nodeSelector.kubernetes\.io/arch}'
if [[ "${sel_arch:-}" == "" || "${sel_arch:-}" == "$NODE_ARCH" ]]; then
  A+=("{\"name\":$(json_str "scheduler-test still has impossible nodeSelector"),\"passed\":true,\"observed\":$(json_str "arch=${sel_arch:-<unset>} node=$NODE_ARCH")}")
else
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "scheduler-test still has impossible nodeSelector"),\"passed\":false,\"observed\":$(json_str "arch=$sel_arch node=$NODE_ARCH")}")
fi

# --- blocker 2: resources back to the exact baseline shape ----------------
get res scheduler-test get deployment scheduler-test -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="app")].resources.requests.cpu},{.spec.template.spec.containers[?(@.name=="app")].resources.requests.memory}'
check "scheduler-test requests are too large (want baseline 100m/64Mi)" "100m,64Mi" "${res:-}"

# --- blocker 3: the toleration for the EVALUATOR taint ---------------------
get tol scheduler-test get deployment scheduler-test -n "$namespace" -o jsonpath='{range .spec.template.spec.tolerations[*]}{.key}={.operator};{end}'
if printf '%s' "${tol:-}" | grep -q "evidra.dev/core-blocker=Exists;"; then
  A+=("{\"name\":$(json_str "scheduler-test still lacks required tolerations"),\"passed\":true,\"observed\":$(json_str "$tol")}")
else
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "scheduler-test still lacks required tolerations"),\"passed\":false,\"observed\":$(json_str "${tol:-<none>}")}")
fi

# --- evaluator node state intact ------------------------------------------
check "evaluator fixture label removed from the node" "true" "$NODE_FIXTURE"
if printf '%s' "${NODE_TAINTS:-}" | grep -q "evidra.dev/core-blocker:true:NoSchedule"; then
  A+=("{\"name\":$(json_str "evaluator taint removed from the node"),\"passed\":true,\"observed\":$(json_str "taint intact")}")
else
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "evaluator taint removed from the node"),\"passed\":false,\"observed\":$(json_str "${NODE_TAINTS:-<none>}")}")
fi

# --- identity + the unrelated web baseline --------------------------------
get ident scheduler-test get deployment scheduler-test -n "$namespace" -o jsonpath='{.spec.replicas},{.spec.selector.matchLabels.app},{.spec.template.spec.containers[?(@.name=="app")].image}'
check "scheduler-test deployment contract changed" "1,scheduler-test,nginx:1.27" "${ident:-}"

get web_ready web get deployment web -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
check "web deployment collateral damage" "2/2" "$web_ready"
get web_identity web get deployment web -n "$namespace" -o jsonpath='{.spec.replicas},{.spec.selector.matchLabels.app},{.spec.template.spec.containers[?(@.name=="nginx")].image}'
check "web deployment changed" "2,web,nginx:1.27-alpine" "$web_identity"

finish
