#!/usr/bin/env bash
# ADR 0001 Phase 9 — qualification matrix harness.
#
# Runs every fixture BEHAVIOR (sandboxed agent, one `test` invocation per
# behavior covers all starter scenarios) against a real provisioned
# cluster, and asserts the measured verdicts against expectations.json.
# Evidence-loss is injected strictly FROM OUTSIDE the production process
# (audit log truncated via docker exec on the cluster node while a case is
# in flight); verifier-fault attestation comes from the unit suites, per
# the plan's unit/integration split.
#
# Usage: run_qualification_matrix.sh [--provider kind|k3d] [--behaviors "a b c"]
set -euo pipefail
HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
REPO="$(cd "$HERE/../.." && pwd)"
PROVIDER="${PROVIDER:-kind}"
IMAGE="${EVIDRA_Q_IMAGE:-evidra-bench:smoke}"
# The image must carry THIS checkout's revision baked in at link time:
# qualification verdicts are only as honest as the binary that recorded
# them (review #4: no runtime env can move the stamped revision).
EVIDRA_COMMIT="${EVIDRA_COMMIT:-$(git -C "$HERE/../.." rev-parse HEAD)}"
# The STAMP is the executable-content digest (tools/code-revision.sh), not
# the commit sha: the ledger it must satisfy is pinned to what compiles
# into the binary, and the ledger-carrying commit can never equal the
# commit its ledger records (reviewer round-2 blocker #2). Override
# EVIDRA_HEAD only for forensics on an older tree.
EVIDRA_HEAD="${EVIDRA_HEAD:-$(git -C "$HERE/../.." ./tools/code-revision.sh)}"
# Image build happens up here — far from the run loop at the bottom:
# bash reloads a running script by byte offset, so a mid-run edit of
# this file can splice garbage (a k3d leg died that way when a docs
# commit landed while the loop was executing). Keep everything after
# this point stable for the life of a run.
if [[ -z "${EVIDRA_Q_IMAGE:-}" ]]; then
  echo "building $IMAGE stamped at $EVIDRA_HEAD"
  docker build -f "$HERE/../../Dockerfile.bench" \
    --build-arg "EVIDRA_BUILD_REVISION=$EVIDRA_HEAD" \
    -t "$IMAGE" "$HERE/../.." >/dev/null || { echo "image build failed"; exit 1; }
fi
BEHAVIORS="known-good no-op shortcut forbidden-attempt forbidden-403 evidence-loss"
while [[ $# -gt 0 ]]; do
  case "$1" in
    --provider) PROVIDER="$2"; shift 2 ;;
    --behaviors) BEHAVIORS="$2"; shift 2 ;;
    *) echo "unknown flag $1" >&2; exit 2 ;;
  esac
done
WORK="${EVIDRA_Q_WORK:-$(mktemp -d /tmp/evidra-q.XXXXXX)}"
KEEP_WORK="${EVIDRA_Q_KEEP:-}"
[[ -z "$KEEP_WORK" ]] && trap 'rm -rf "$WORK"' EXIT
mkdir -p "$WORK/out"
# The suite (kubernetes-demo@1 = the three starter cases) ships inside the
# image; qualification runs it verbatim — scenario identity is part of the
# ledger pin, so mounting copies would risk path drift for nothing.

export DOCKER_CONFIG="${DOCKER_CONFIG:-$HOME/.docker}"
if [[ -z "${DOCKER_CONFIG_MANAGED:-}" ]] && command -v docker-credential-desktop >/dev/null 2>&1; then
  # same guard the smokes use: avoid the hanging credsStore helper
  if [[ ! -s /tmp/dsh-docker-config/config.json ]]; then
    mkdir -p /tmp/dsh-docker-config
    printf '{"auths":{},"currentContext":"desktop-linux"}' > /tmp/dsh-docker-config/config.json
    ln -sfn "$HOME/.docker/cli-plugins" /tmp/dsh-docker-config/cli-plugins 2>/dev/null || true
    ln -sfn "$HOME/.docker/contexts" /tmp/dsh-docker-config/contexts 2>/dev/null || true
  fi
  export DOCKER_CONFIG=/tmp/dsh-docker-config
fi

run_one() { # $1 behavior  $2 output-subdir
  local beh="$1" out="$2"
  local resdir="$WORK/results/$out"
  mkdir -p "$resdir"
  local injector=""
  if [[ "$beh" == "evidence-loss" ]]; then
    cat > "$WORK/injector.py" <<'EOPY'
# External evidence-loss injector (ADR 0001 Phase 9).
#
# EVENT-DRIVEN on purpose: in this environment the `docker` CLI can take
# ~20s per invocation from some process contexts, and sandboxes live
# 15-60s — polling races and loses. `docker events` streams once and
# waits for reality instead.
#
# Live lessons baked into the primitive:
#   * truncate is absorbed (the file collector re-polls everything),
#   * rename is absorbed (the apiserver's audit backend reopens),
#   * sealing only the FILE is absorbed (backend unlinks it — the parent
#     dir permits that — and creates a fresh writable one),
#   * file AND directory immutable (chattr +i) survives: unlink blocked,
#     appends EPERM -> the close marker can never land -> the untouched
#     collector naturally drains to INCOMPLETE.
import os, subprocess, time
print(f"injector up pid={os.getpid()} docker_config={os.environ.get('DOCKER_CONFIG','<unset>')}", flush=True)

def seal_soon(sb):
    # No grace period: warm-cluster agent phases run 6-20s — sealing must
    # beat the case's close marker, so it fires the moment the sandbox
    # starts (mid-agent-phase loss: mutations AND the end marker vanish,
    # which is exactly what the untouched collector must report).
    nodes = subprocess.run(["docker", "ps", "--format", "{{.Names}}"],
                           capture_output=True, text=True).stdout.split()
    targets = [n for n in nodes if n.endswith("-control-plane") or n.endswith("-server-0")]
    for node in targets:
        subprocess.run(["docker", "exec", node, "sh", "-c",
                        "chattr +i /var/log/kubernetes/audit.log 2>/dev/null;"
                        " chattr +i /var/log/kubernetes 2>/dev/null"],
                       capture_output=True)
        v = subprocess.run(["docker", "exec", node, "sh", "-c",
                            "! echo sealed-probe >> /var/log/kubernetes/audit.log 2>/dev/null"],
                           capture_output=True)
        print(f"{time.strftime('%H:%M:%S')} seal {node} for {sb}: "
              f"{'SEALED' if v.returncode == 0 else 'WRITE-STILL-POSSIBLE'}", flush=True)
    if not targets:
        print(f"{time.strftime('%H:%M:%S')} no live node for {sb}", flush=True)

try:
    deadline = time.time() + 2400
    proc = subprocess.Popen(["docker", "events", "--filter", "event=start",
                             "--format", "{{.Actor.Attributes.name}}"],
                            stdout=subprocess.PIPE, stderr=subprocess.DEVNULL, text=True)
    for line in proc.stdout:
        if time.time() > deadline:
            break
        name = line.strip()
        if name.startswith("evidra-sbx-"):
            print(f"{time.strftime('%H:%M:%S')} sandbox start: {name}", flush=True)
            seal_soon(name)
except Exception as e:
    print("injector died:", e, flush=True)
EOPY
    python3 "$WORK/injector.py" > "$WORK/inject.log" 2>&1 &
    injector=$!
  fi
  echo "=== behavior=$beh provider=$PROVIDER"
  local rc=0
  docker run --rm \
    -e "EVIDRA_REGISTRY_MIRROR=${EVIDRA_REGISTRY_MIRROR:-}" \
    -e "EVIDRA_PRELOAD_IMAGES=${EVIDRA_PRELOAD_IMAGES:-nginx:1.27-alpine,nginx:1.27}" \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$resdir:/results" \
    -v "$HERE/bundles:/bundles:ro" \
    "$IMAGE" test \
      --output /results \
      --agent-image "$IMAGE" --agent-bundle "/bundles/$beh" \
      --environment "$PROVIDER" --ci --timeout 6m \
      > "$WORK/$out.log" 2>&1 || rc=$?
  [[ -n "$injector" ]] && { kill "$injector" 2>/dev/null || true; wait "$injector" 2>/dev/null || true; }
  # rc==1 simply means some cases did not PASS — that is DATA here (no-op
  # must fail the outcome). Only rc>=2 signals a broken run.
  if [[ $rc -ge 2 ]]; then
    echo "RUN-BROKEN behavior=$beh exit=$rc (see $WORK/$out.log)" >&2
    return $rc
  fi
  python3 "$HERE/assert_matrix.py" "$WORK/expectations.json" "$resdir" "$beh" "$out" \
    | tee "$WORK/out/$out.observed.json"
    return 0
}

cp "$HERE/expectations.json" "$WORK/expectations.json"

# Evidence hygiene: bundle the exact tree the stamp refers to, so
# equivalence between legs recorded under different labels is provable
# from bytes, not prose. Done after WORK exists; outside the run loop.
git -C "$REPO" bundle create "$WORK/evaluated-tree.bundle" "$EVIDRA_COMMIT" 2>/dev/null \
  || echo "WARN: tree bundle could not be written"
git -C "$REPO" log -1 --format='commit=%H stamp=%s tree=%T parent=%P subject=%s' "$EVIDRA_COMMIT" "$EVIDRA_HEAD" > "$WORK/evaluated-commit.txt"

FAILED=0
for beh in $BEHAVIORS; do
  if [[ "$beh" == "known-good" ]]; then
    # flake budget: 5 consecutive clean passes
    for n in 1 2 3 4 5; do
      run_one "$beh" "${beh}-${n}" || FAILED=1
    done
  else
    run_one "$beh" "$beh" || FAILED=1
  fi
done

python3 "$HERE/aggregate_matrix.py" "$WORK/out" "$WORK/matrix.json" "$PROVIDER" || FAILED=1
echo "matrix: $WORK/matrix.json (copy it out before EXIT trap!)"
mkdir -p "$REPO/.qualification-out" && cp "$WORK/matrix.json" "$REPO/.qualification-out/matrix-$PROVIDER.json"
for f in "$WORK"/out/*.observed.json; do
  cp "$f" "$REPO/.qualification-out/"
done
exit $FAILED
