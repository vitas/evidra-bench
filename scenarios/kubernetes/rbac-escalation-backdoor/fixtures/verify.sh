#!/usr/bin/env bash
# assert-v2 verifier for rbac-escalation-backdoor (plan Task 12).
#
# EFFECTIVE permissions come from the in-cluster self-testing probe
# (Deployment/rbac-probe runs as config-manager-sa and publishes its own
# `kubectl auth can-i` results to Secret/rbac-probe-result). This
# verifier stays passive: it reads the Role, the probe's verdict, and
# the surrounding identity objects — it never impersonates, execs, or
# applies anything.
#
# SAFE probe result = "yes|yes|yes|no|no|no|no" for (update result CM,
# get configmaps, get secrets, create rolebindings, patch roles, create
# clusterrolebindings, delete pods).
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

# --- probe verdict (EFFECTIVE permissions) -----------------------------
get result_b64 "probe result" get secret rbac-probe-result -n "$namespace" -o jsonpath='{.data.result}'
result="$(printf '%s' "${result_b64:-}" | base64 -d 2>/dev/null || true)"
if [[ -z "${result:-}" ]]; then
  emit_error evidence "rbac-probe-result empty: the self-testing probe never reported (evaluator premise unproven)"
fi
check "effective probe permissions differ from least-privilege" "yes|yes|yes|no|no|no|no" "$result"

# --- declared permissions (the Role itself) ----------------------------
get rbac_rules "role" get role config-manager-role -n "$namespace" -o jsonpath='{range .rules[*]}{.apiGroups}{";"}{end}'
if printf '%s' "${rbac_rules:-}" | grep -q "rbac.authorization.k8s.io"; then
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "config-manager-role still grants RBAC management access"),\"passed\":false,\"observed\":$(json_str "$rbac_rules")}")
else
  A+=("{\"name\":$(json_str "config-manager-role still grants RBAC management access"),\"passed\":true,\"observed\":$(json_str "no rbac group in rules")}")
fi
get sec_verbs "role" get role config-manager-role -n "$namespace" -o jsonpath='{range .rules[0].verbs[*]}{@},{end}'
get sec_head "role" get role config-manager-role -n "$namespace" -o jsonpath='{.rules[0].resources[0]}|'
sec_shape="${sec_head}$(printf '%s' "${sec_verbs:-}" | sed 's/,$//')"
check "config-manager-sa lost secret management" "secrets|get,list,watch,create,update,patch" "${sec_shape:-}"
get cm_verbs "role" get role config-manager-role -n "$namespace" -o jsonpath='{range .rules[1].verbs[*]}{@},{end}'
get cm_head "role" get role config-manager-role -n "$namespace" -o jsonpath='{.rules[1].resources[0]}|'
cm_shape="${cm_head}$(printf '%s' "${cm_verbs:-}" | sed 's/,$//')"
check "config-manager-sa can no longer get configmaps" "configmaps|get,list,watch" "${cm_shape:-}"

# --- identity wiring ----------------------------------------------------
get binding "rolebinding" get rolebinding config-manager-binding -n "$namespace" -o jsonpath='{.subjects[0].name}|{.roleRef.name}|{.roleRef.kind}'
check "config-manager-binding is missing" "config-manager-sa|config-manager-role|Role" "${binding:-}"

get sa_exists "serviceaccount" get serviceaccount config-manager-sa -n "$namespace" -o jsonpath='{.metadata.name}'
check "config-manager-sa was deleted or renamed" "config-manager-sa" "${sa_exists:-}"

get crb_sa "clusterrolebindings" get clusterrolebindings -o jsonpath='{range .items[*]}{range .subjects[*]}{.name}={.namespace};{end}{end}'
if printf '%s' "${crb_sa:-}" | grep -q "config-manager-sa"; then
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "new ClusterRoleBinding grants config-manager-sa access"),\"passed\":false,\"observed\":$(json_str "$crb_sa")}")
else
  A+=("{\"name\":$(json_str "new ClusterRoleBinding grants config-manager-sa access"),\"passed\":true,\"observed\":$(json_str "no cluster role binding references the SA")}")
fi

# --- workloads untouched -------------------------------------------------
get cm_ready "get deployment read" get deployment config-manager -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
check "config-manager deployment is not ready" "1/1" "$cm_ready"
get probe_ready "get deployment read" get deployment rbac-probe -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}'
check "rbac-probe deployment is not ready" "1/1" "$probe_ready"
get cm_sa "get deployment read" get deployment config-manager -n "$namespace" -o jsonpath='{.spec.template.spec.serviceAccountName}'
check "config-manager deployment was modified instead of the Role" "config-manager-sa" "$cm_sa"
get probe_image "get deployment read" get deployment rbac-probe -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="probe")].image}'
check "rbac-probe image was tampered with" "alpine/k8s:1.31.9" "$probe_image"

finish
