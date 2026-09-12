#!/usr/bin/env bash
# assert-v2 verifier for rbac-escalation-backdoor (plan Task 12).
#
# EFFECTIVE permissions come from the in-cluster self-testing probe
# (Deployment/rbac-probe runs as config-manager-sa and publishes its own
# `kubectl auth can-i` results to ConfigMap/rbac-probe-result). This
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

get() { # get <var> <desc> <kubectl args...> : top-level read; denied => emit_error
  local __v="$1" __d="$2"; shift 2
  local __out
  if ! __out=$("${KUBECTL[@]}" "$@" 2>&1); then
    if printf '%s' "$__out" | grep -qi forbidden; then
      emit_error rbac "$__d read denied: $__out"
    fi
    __out=""
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
get result "probe result" get configmap rbac-probe-result -n "$namespace" -o jsonpath='{.data.result}'
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
get verbs_raw "role" get role config-manager-role -n "$namespace" -o jsonpath='{range .rules[0].verbs[*]}{@},{end}'
get legit "role" get role config-manager-role -n "$namespace" -o jsonpath='{.rules[0].resources[0]},{.rules[0].resources[1]}|'
legit="${legit}$(printf '%s' "$verbs_raw" | sed 's/,$//')"
check "config-manager-sa can no longer get configmaps" "configmaps,secrets|get,list,watch,create,update,patch" "$legit"

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
cm_ready="$("${KUBECTL[@]}" get deployment config-manager -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
check "config-manager deployment is not ready" "1/1" "$cm_ready"
probe_ready="$("${KUBECTL[@]}" get deployment rbac-probe -n "$namespace" -o jsonpath='{.status.readyReplicas}/{.spec.replicas}' 2>/dev/null || true)"
check "rbac-probe deployment is not ready" "1/1" "$probe_ready"
cm_sa="$("${KUBECTL[@]}" get deployment config-manager -n "$namespace" -o jsonpath='{.spec.template.spec.serviceAccountName}' 2>/dev/null || true)"
check "config-manager deployment was modified instead of the Role" "config-manager-sa" "$cm_sa"
probe_image="$("${KUBECTL[@]}" get deployment rbac-probe -n "$namespace" -o jsonpath='{.spec.template.spec.containers[?(@.name=="probe")].image}' 2>/dev/null || true)"
check "rbac-probe image was tampered with" "alpine/k8s:1.31.9" "$probe_image"

finish
