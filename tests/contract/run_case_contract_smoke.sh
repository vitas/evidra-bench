#!/usr/bin/env bash
# Case-contract smoke: the lightweight replacement for the certification
# matrix (owner direction 2026-09-11). One run per behavior, EXACT verdict
# comparison, no ledgers, no badges, no attestations:
#
#   known-good  -> PASS        (sandboxed scripted agent fixes the break)
#   no-op       -> FAIL        (agent does nothing on a broken cluster)
#   wrong-scope -> UNSAFE      (write attempted on a protected resource)
#   audit-loss  -> INCOMPLETE  (audit log destroyed mid-window from outside)
#   unconfined  -> INCOMPLETE  (external agent opted out of the sandbox)
#
# Verdicts are read from run.json — which since finding #1 carries the
# authoritative CaseResult verdict, the same value every artifact agrees
# on. Kind runs on every PR; k3d runs on main.
#
# Usage: run_case_contract_smoke.sh <kind|k3d>
set -euo pipefail

provider="${1:?usage: run_case_contract_smoke.sh <kind|k3d>}"
case "$provider" in
  kind | k3d) ;;
  *) echo "unsupported provider: $provider" >&2; exit 2 ;;
esac

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT

log() { printf 'contract-smoke[%s]: %s\n' "$provider" "$*"; }

# The bench runner image doubles as the agent sandbox image (it ships
# bash + kubectl + kind/k3d, and the whole suite runs inside it exactly
# like the public one-command flow). CI may pass a pre-built tag.
image="${EVIDRA_CONTRACT_IMAGE:-evidra-bench:contract}"
if ! docker image inspect "$image" >/dev/null 2>&1; then
  log "building $image"
  docker build -q -f "$repo_root/Dockerfile.bench" -t "$image" "$repo_root" >/dev/null
fi

failures=0

# bench_leg <name> <cluster> [extra flags...] — runs one evaluation inside
# the container; output lands in $work_dir/<name>.log, artifacts in
# $work_dir/<name> (mounted as /out/<name> inside).
bench_leg() {
  local name="$1" cluster="$2"
  shift 2
  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$repo_root:/repo:ro" \
    -v "$work_dir:/out" \
    -e EVIDRA_AGENT_SANDBOX_IMAGE="$image" \
    "$image" bench \
    --scenarios-dir /repo/scenarios \
    --environment "$provider" \
    --adapter cli \
    --model scripted \
    --timeout 6m \
    --cluster-name "$cluster" \
    --runs-dir "/out/$name" \
    "$@" \
    >"$work_dir/$name.log" 2>&1
}

# check_leg <name> <want-verdict>
check_leg() {
  local name="$1" want="$2"
  local verdict
  verdict="$(python3 "$repo_root/tests/contract/read_verdict.py" "$work_dir/$name" || true)"
  if [[ "$verdict" != "$want" ]]; then
    echo "FAIL leg $name: verdict=${verdict:-<none>} want=$want" >&2
    tail -n 40 "$work_dir/$name.log" >&2
    failures=$((failures + 1))
  else
    log "ok leg $name: $verdict"
  fi
}

log "leg known-good"
bench_leg known-good "evidra-contract-known-good-${provider}" \
  --scenario kubernetes/broken-deployment \
  --agent-command /repo/tests/fixtures/scripted-agent/good.sh || true
check_leg known-good PASS

log "leg no-op"
bench_leg no-op "evidra-contract-no-op-${provider}" \
  --scenario kubernetes/broken-deployment \
  --agent-command /repo/tests/fixtures/scripted-agent/no-op.sh || true
check_leg no-op FAIL

log "leg wrong-scope"
bench_leg wrong-scope "evidra-contract-wrong-scope-${provider}" \
  --scenario kubernetes/wrong-namespace-workload-restart \
  --agent-command /repo/tests/fixtures/scripted-agent/wrong-scope.sh || true
check_leg wrong-scope UNSAFE

# audit-loss: destroy the API server's audit log WHILE the evidence window
# is open. The killer waits for the identity-materialization console line
# (the window opens immediately after), then removes the file twice — the
# collector must observe rotation or reader failure either way. The agent
# itself still fixes the break: only the evidence fault may explain the
# verdict.
audit_cluster="evidra-contract-audit-loss-${provider}"
audit_killer() {
  local node
  case "$provider" in
    kind) node="${audit_cluster}-control-plane" ;;
    k3d) node="k3d-${audit_cluster}-server-0" ;;
  esac
  for _ in $(seq 1 300); do
    if grep -q "run identities ready" "$work_dir/audit-loss.log" 2>/dev/null; then break; fi
    sleep 1
  done
  sleep 4
  docker exec "$node" rm -f /var/log/kubernetes/audit.log || true
  sleep 12
  docker exec "$node" rm -f /var/log/kubernetes/audit.log || true
}
log "leg audit-loss"
audit_killer &
killer_pid=$!
bench_leg audit-loss "$audit_cluster" \
  --scenario kubernetes/broken-deployment \
  --agent-command /repo/tests/fixtures/scripted-agent/good.sh || true
kill "$killer_pid" 2>/dev/null || true
wait "$killer_pid" 2>/dev/null || true
check_leg audit-loss INCOMPLETE

# unconfined: the explicit opt-out must demote a profiled case even though
# every outcome check passes. The command runs in the runner container
# itself — holding the mounted docker socket — which is exactly what the
# demotion encodes.
log "leg unconfined"
bench_leg unconfined "evidra-contract-unconfined-${provider}" \
  --scenario kubernetes/broken-deployment \
  --agent-command /repo/tests/fixtures/scripted-agent/good.sh \
  --agent-unconfined || true
check_leg unconfined INCOMPLETE

if ((failures > 0)); then
  echo "case-contract smoke ($provider): $failures leg(s) FAILED" >&2
  exit 1
fi
echo "case-contract smoke ($provider): all 5 legs match their contract"
