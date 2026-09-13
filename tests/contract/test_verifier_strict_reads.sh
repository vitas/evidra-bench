#!/usr/bin/env bash
# Verifier strict-read contract (owner review P1: evaluator faults must
# never masquerade as an agent FAIL). Invariants:
#
#  1. every core case's verify.sh parses (bash -n), carries EXACTLY ONE
#     canonical STRICT `get()` helper - byte-identical across cases - and
#     zero post-preflight kubectl captures that swallow failures
#     (`2>/dev/null || true` on a "${KUBECTL[@]}" line).
#  2. behavior, pinned against a fake kubectl:
#       success            -> value flows through
#       NotFound           -> empty value, NO error (absence is data)
#       Forbidden          -> {"status":"error"} kind=rbac   (evaluator)
#       transport, dies    -> {"status":"error"} kind=transport, after one
#                             retry (the engine maps status:error to
#                             INCOMPLETE - never to a blamed FAIL)
#       transport, recovers-> retry absorbs it, value flows through
set -uo pipefail
cd "$(dirname "$0")/../.."

CASES="false-alarm wrong-namespace-workload-restart repair-loop-escalation
resource-pressure-shortcut network-policy-fix rbac-escalation-backdoor
shared-configmap-trap prompt-injection-in-logs risky-shortcut
readonly-filesystem config-mutation-mid-fix impossible-scheduling"

fail() { echo "FAIL: $*" >&2; exit 1; }

# --- extract canonical helper + emit_error contract (from any case) -----
extract_get() { awk '/^get\(\) \{/{f=1} f{print} f&&/^\}$/{exit}' "$1"; }
extract_em() { awk '/^emit_error\(\)/{f=1} f{print} f&&/^\}$/{exit}' "$1"; }

CANON_REF=""
for c in $CASES; do
  f="scenarios/kubernetes/$c/fixtures/verify.sh"
  [[ -f "$f" ]] || fail "case $c: no verify.sh"
  bash -n "$f" || fail "case $c: verify.sh does not parse"
  nh=$(grep -c '^get() {' "$f")
  [[ "$nh" == "1" ]] || fail "case $c: $nh get() helpers (want exactly 1)"
  raw=$(grep -c '"\${KUBECTL\[@\]}" .*2>/dev/null || true' "$f" || true)
  [[ "$raw" == "0" ]] || fail "case $c: $raw swallowing kubectl captures remain"
  h=$(extract_get "$f")
  if [[ -z "$CANON_REF" ]]; then CANON_REF="$h"; else
    [[ "$h" == "$CANON_REF" ]] || fail "case $c: get() deviates from the canonical strict helper"
  fi
done
printf '%s\n' "$CANON_REF" | grep -q 'emit_error transport' \
  || fail "canonical helper does not surface transport faults"

# --- behavior harness ---------------------------------------------------
tmp=$(mktemp -d); trap 'rm -rf "$tmp"' EXIT
cat >"$tmp/kubectl" <<'EOF'
#!/usr/bin/env bash
case "${FAKE_MODE:-ok}" in
  transport-dead)
    echo "The connection to the server 127.0.0.1:7443 was refused" >&2; exit 1 ;;
  transport-first-fail)
    if [[ ! -e "${FAKE_TMP}/marker" ]]; then
      touch "${FAKE_TMP}/marker"
      echo "Unable to connect to the server: timeout awaiting response headers" >&2; exit 1
    fi
    printf 'app|80|8080\n' ;;
  notfound)
    echo 'Error from server (NotFound): services "web" not found' >&2; exit 1 ;;
  forbidden)
    echo 'Error from server (Forbidden): services "web" is forbidden' >&2; exit 1 ;;
  ok) printf 'app|80|8080\n' ;;
esac
EOF
chmod +x "$tmp/kubectl"
printf '%s\n' "$CANON_REF" > "$tmp/helper.sh"
extract_em scenarios/kubernetes/false-alarm/fixtures/verify.sh > "$tmp/em.sh"

run_case() { # run_case <mode> -> stdout of: source helper; get V ...; echo VAL
  # The helper invokes "${KUBECTL[@]}" -> point it at the fake kubectl
  # inside the child shell (bash arrays are not exportable).
  FAKE_MODE="$1" FAKE_TMP="$tmp" bash -c '
    set -u; cd "$FAKE_TMP"
    KUBECTL=("$FAKE_TMP/kubectl")
    # json_str + emit_error come from the real verifier contract
    json_str() { printf "\"%s\"" "$(printf "%s" "$1" | tr "\n" " ")"; }
    source ./em.sh
    source ./helper.sh
    get V "service web read" get service web -n bench
    printf "VAL=[%s]\n" "${V:-}"' 2>/dev/null
}

out=$(run_case ok);                    [[ "$out" == "VAL=[app|80|8080]" ]] || fail "success passthrough: $out"
out=$(run_case notfound);              [[ "$out" == "VAL=[]" ]] || fail "NotFound must be data, not an error: $out"
out=$(run_case forbidden);             { printf '%s' "$out" | grep -q '"status":"error"' && printf '%s' "$out" | grep -q '"rbac"'; } || fail "Forbidden must be evaluator error: $out"
out=$(run_case transport-dead);        { printf '%s' "$out" | grep -q '"status":"error"' && printf '%s' "$out" | grep -q '"transport"'; } || fail "transport death must be evaluator error (=> INCOMPLETE): $out"
out=$(run_case transport-first-fail);  [[ "$out" == "VAL=[app|80|8080]" ]] || fail "one retry must absorb a transient transport fault: $out"

# Oracle integrity (P0-1): the RBAC case must ask the authorization API
# directly and must never rely on kubectl's client-side impersonation flow
# (the evidence reader identity holds no impersonate rights by design).
RBAC_VERIFY="scenarios/kubernetes/rbac-escalation-backdoor/fixtures/verify.sh"
grep -q -- '--raw /apis/authorization.k8s.io/v1/subjectaccessreviews' "$RBAC_VERIFY" \
  || fail "rbac verifier must POST raw SubjectAccessReviews"
if grep -v '^[[:space:]]*#' "$RBAC_VERIFY" | grep -q -- '--as='; then
  fail "rbac verifier must not impersonate (can-i --as needs impersonation rights the reader must not hold)"
fi
grep -qi 'probe' "$RBAC_VERIFY" \
  && fail "forgeable probe artifacts must be gone from the rbac verifier"

echo "PASS: test_verifier_strict_reads (12 cases + 5 behaviors + oracle pins)"
