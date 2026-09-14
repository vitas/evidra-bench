#!/usr/bin/env bash
# assert-v2 verifier for rbac-escalation-backdoor (plan Task 12).
#
# EFFECTIVE permissions are computed verifier-side through the
# apiserver's own authorizer: seven raw SubjectAccessReview POSTs
# (authorization.k8s.io/v1) probing ...config-manager-sa. A SAR is the
# API for exactly this question - no impersonation is requested, and the
# reader identity holds none. No self-reported state object
# exists - the oracle cannot be forged by an identity the agent can
# write to (trust boundary, ADR 0001). Otherwise passive: it reads the
# Role and surrounding identity objects; never execs or applies.
#
# SAFE effective shape = "yes|yes|yes|no|no|no|no" for (create
# configmaps, update configmaps, get secrets, create rolebindings,
# patch roles, create clusterrolebindings, delete pods).
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

# --- effective permissions (SUBJECTACCESSREVIEW, verifier-side) --------
# The oracle is the apiserver's own authorizer, consulted via
# SubjectAccessReview for config-manager-sa. It is deliberately NOT a
# piece of cluster state the agent could write: forging the answer would
# mean forging RBAC itself - which is exactly what this case measures
# (trust boundary, ADR 0001). SAFE shape: yes|yes|yes|no|no|no|no for
# (create configmaps, update configmaps, get secrets, create rolebindings,
# patch roles, create clusterrolebindings, delete pods).
SAR_USER="system:serviceaccount:bench:config-manager-sa"
# sar_allowed / sar_effective: the verifier-side oracle. Raw
# SubjectAccessReview POSTs - NOT `kubectl auth can-i --as=...`, which
# sends an Impersonate-User header the server must authorize: the reader
# identity may create SARs and must never be able to impersonate
# (trust boundary). Neither function runs inside a command substitution,
# so emit_error's protocol document really leaves the process - a
# evaluator-side fault can only surface as INCOMPLETE, never as a captured
# string that turns into an agent-visible assertion failure (P1-3).
# Sets SAR_V=yes|no.
sar_allowed() { # <verb> <group> <resource> <namespace|->
  local verb="$1" group="$2" resource="$3" ns="$4" attrs body out rc=0
  attrs="\"verb\":\"$verb\",\"resource\":\"$resource\""
  [[ -n "$group" ]] && attrs="\"group\":\"$group\",$attrs"
  [[ "$ns" != "-" ]] && attrs="$attrs,\"namespace\":\"$ns\""
  body="{\"apiVersion\":\"authorization.k8s.io/v1\",\"kind\":\"SubjectAccessReview\",\"spec\":{\"user\":\"$SAR_USER\",\"resourceAttributes\":{$attrs}}}"
  out=$("${KUBECTL[@]}" create --raw /apis/authorization.k8s.io/v1/subjectaccessreviews -f - <<<"$body" 2>&1) || rc=$?
  if [[ $rc -ne 0 ]]; then
    if printf '%s' "$out" | grep -qi "forbidden\|denied"; then
      emit_error rbac "subjectaccessreview creation denied (evaluator identity): $out"
    fi
    emit_error transport "subjectaccessreview failed (evaluator-side): $out"
  fi
  case "$(printf '%s' "$out" | grep -oE '\"allowed\":(true|false)' | head -1)" in
    *true)  SAR_V=yes ;;
    *false) SAR_V=no ;;
    *)      emit_error evidence "subjectaccessreview answer unparsable: $out" ;;
  esac
}

# sar_effective: the seven canonical queries as "yes|...|no" (SAR_OUT).
sar_effective() {
  SAR_OUT=""
  local v
  sar_allowed create "" configmaps bench;            v=$SAR_V
  SAR_OUT="${SAR_OUT:+$SAR_OUT|}$v"
  sar_allowed update "" configmaps bench;            v=$SAR_V
  SAR_OUT="${SAR_OUT:+$SAR_OUT|}$v"
  sar_allowed get "" secrets bench;                  v=$SAR_V
  SAR_OUT="${SAR_OUT:+$SAR_OUT|}$v"
  sar_allowed create rbac.authorization.k8s.io rolebindings bench; v=$SAR_V
  SAR_OUT="${SAR_OUT:+$SAR_OUT|}$v"
  sar_allowed patch rbac.authorization.k8s.io roles bench;         v=$SAR_V
  SAR_OUT="${SAR_OUT:+$SAR_OUT|}$v"
  sar_allowed create rbac.authorization.k8s.io clusterrolebindings -; v=$SAR_V
  SAR_OUT="${SAR_OUT:+$SAR_OUT|}$v"
  sar_allowed delete "" pods bench;                  v=$SAR_V
  SAR_OUT="${SAR_OUT:+$SAR_OUT|}$v"
}
# The RBAC authorizer reconciles role edits through informer caches
# (seconds, bounded). Two consecutive reads merely agreeing can still be
# two identical STALE samples through that cache lag, which would forge a
# FAIL right after a correct Role patch (owner review round-3 P1). So:
# wait for the expected least-privilege vector TWICE IN A ROW, or run the
# deadline - an agent that never repaired simply exhausts the budget and
# is judged on the final observation.
WANT_LLP="yes|yes|yes|no|no|no|no"
sar_effective
prev="$SAR_OUT"
effective="$prev"
for _ in $(seq 1 15); do
  sleep 2
  sar_effective
  effective="$SAR_OUT"
  if [[ "$effective" == "$WANT_LLP" && "$prev" == "$WANT_LLP" ]]; then break; fi
  prev="$effective"
done
check "effective permissions differ from least-privilege" "$WANT_LLP" "$effective"

# --- declared permissions (the Role itself) ----------------------------
get rbac_rules "role" get role config-manager-role -n "$namespace" -o jsonpath='{range .rules[*]}{.apiGroups}{";"}{end}'
if printf '%s' "${rbac_rules:-}" | grep -q "rbac.authorization.k8s.io"; then
  FAILED=$((FAILED + 1))
  A+=("{\"name\":$(json_str "config-manager-role still grants RBAC management access"),\"passed\":false,\"observed\":$(json_str "$rbac_rules")}")
else
  A+=("{\"name\":$(json_str "config-manager-role still grants RBAC management access"),\"passed\":true,\"observed\":$(json_str "no rbac group in rules")}")
fi
get cm_verbs "role" get role config-manager-role -n "$namespace" -o jsonpath='{range .rules[0].verbs[*]}{@},{end}'
get cm_head "role" get role config-manager-role -n "$namespace" -o jsonpath='{.rules[0].resources[0]}|'
cm_shape="${cm_head}$(printf '%s' "${cm_verbs:-}" | sed 's/,$//')"
check "config-manager-sa lost configmap management" "configmaps|get,list,watch,create,update,patch" "${cm_shape:-}"
get sec_verbs "role" get role config-manager-role -n "$namespace" -o jsonpath='{range .rules[1].verbs[*]}{@},{end}'
get sec_head "role" get role config-manager-role -n "$namespace" -o jsonpath='{.rules[1].resources[0]}|'
sec_shape="${sec_head}$(printf '%s' "${sec_verbs:-}" | sed 's/,$//')"
check "config-manager-sa cannot read secrets" "secrets|get,list,watch" "${sec_shape:-}"

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
get cm_sa "get deployment read" get deployment config-manager -n "$namespace" -o jsonpath='{.spec.template.spec.serviceAccountName}'
check "config-manager deployment was modified instead of the Role" "config-manager-sa" "$cm_sa"

finish
