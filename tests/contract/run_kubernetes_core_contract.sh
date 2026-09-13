#!/usr/bin/env bash
# kubernetes-core@1 exact-verdict contract matrix (plan Task 18).
#
# For each repetition this runs exactly three evaluations against the
# suite — safe, noop, unsafe controls — where each evaluation provisions
# ONE disposable cluster and runs all twelve isolated cases with the
# scripted control agent. The CLI exit code is never the gate: noop and
# unsafe exit non-zero BY DESIGN. The gate is the canonical result
# artifact, checked case-by-case by check_suite_verdicts.py.
#
# usage: run_kubernetes_core_contract.sh <kind|k3d>
# env:   EVIDRA_CORE_REPEAT        repetitions (default 1; calibration 3)
#        EVIDRA_CORE_SANDBOX_IMAGE agent sandbox image (default
#                             evidra-bench:contract — build with
#                             docker build -f Dockerfile.bench -t <tag> .)
#        EVIDRA_CORE_DRYRUN=1     print the evaluation matrix and exit
set -euo pipefail

provider="${1:?usage: run_kubernetes_core_contract.sh <kind|k3d>}"
case "$provider" in
  kind | k3d) ;;
  *) echo "provider must be kind or k3d" >&2; exit 2 ;;
esac
repeat="${EVIDRA_CORE_REPEAT:-1}"
[[ "$repeat" =~ ^[1-9][0-9]*$ ]] || { echo "EVIDRA_CORE_REPEAT must be a positive integer" >&2; exit 2; }

root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
sandbox_image="${EVIDRA_CORE_SANDBOX_IMAGE:-evidra-bench:contract}"
agent="$root/tests/fixtures/scripted-agent/kubernetes-core.sh"

command -v "$provider" >/dev/null 2>&1 || { echo "$provider CLI not found on PATH" >&2; exit 2; }

if [[ "${EVIDRA_CORE_DRYRUN:-0}" == "1" ]]; then
  for r in $(seq 1 "$repeat"); do
    for mode in safe noop unsafe; do
      echo "repeat=$r provider=$provider control=$mode suite=kubernetes-core@1 cluster=disposable-per-evaluation"
    done
  done
  exit 0
fi

docker image inspect "$sandbox_image" >/dev/null 2>&1 || {
  echo "sandbox image $sandbox_image is missing; build it first:" >&2
  echo "  docker build -f Dockerfile.bench -t $sandbox_image ." >&2
  exit 2
}

work="$(mktemp -d)"
cleanup() {
  local leaked
  case "$provider" in
    kind) leaked="$(kind get clusters 2>/dev/null | grep '^evidra' || true)" ;;
    k3d) leaked="$(k3d cluster list 2>/dev/null | awk 'NR>1 {print $1}' | grep '^evidra' || true)" ;;
  esac
  if [[ -n "$leaked" ]]; then
    echo "LEAKED clusters after contract run: $leaked" >&2
    exit 1
  fi
  # Forensic insurance: when an evaluation failed, salvage every run's
  # machine-readable truth (run.json/verifier.json/failure-autopsy.json)
  # to a fixed directory BEFORE deleting the workspace. The suites
  # themselves stay lean; this only fires on the path that needed
  # debugging anyway. EVIDRA_CORE_KEEP=1 skips the delete entirely.
  if [[ "${EVIDRA_CORE_KEEP:-0}" == "1" ]]; then
    echo "EVIDRA_CORE_KEEP=1: workspace preserved at $work" >&2
    return
  fi
  if [[ -n "${eval_failed:-}" ]]; then
    forensics=/tmp/core-gate-forensics-$(basename "$work")
    mkdir -p "$forensics"
    find "$work" \( -name run.json -o -name verifier.json -o -name failure-autopsy.json \) \
      -exec cp {} --parents "$forensics" \; 2>/dev/null || true
    echo "failed evaluation artifacts salvaged to $forensics" >&2
  fi
  rm -rf "$work"
}
trap cleanup EXIT

bin="$work/bench"
go build -C "$root" -o "$bin" ./cmd/bench-cli

total=0
for r in $(seq 1 "$repeat"); do
  for mode in safe noop unsafe; do
    out="$work/run-$r-$mode"
    echo "== evaluation: repeat=$r control=$mode provider=$provider"
    # Non-zero exits for noop/unsafe are expected; the artifact decides.
    if EVIDRA_CORE_CONTROL="$mode" "$bin" test \
      --suite kubernetes-core@1 \
      --environment "$provider" --timeout 15m \
      --agent "$agent" \
      --agent-env EVIDRA_CORE_CONTROL \
      --agent-image "$sandbox_image" \
      --output "$out"; then
      echo "note: $mode control exited 0"
    fi
    [[ -f "$out/result.json" ]] || { echo "no result.json for $mode" >&2; eval_failed=1; exit 1; }
    python3 "$root/tests/contract/check_suite_verdicts.py" "$out/result.json" "$mode" \
      || { eval_failed=1; exit 1; }
    total=$((total + 12))
  done
done

echo "kubernetes-core contract ($provider): $total exact case verdicts, no leaked clusters"
