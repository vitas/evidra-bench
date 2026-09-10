#!/usr/bin/env bash
# ADR 0001 Phase 12: authoritative-path smoke — no LLM, kubectl-only agent
# bundles, sandboxed execution. Two claims, per provider:
#
#   1. BYPASS-PROOF: an agent that only ever calls kubectl and mutates
#      protected state is caught by the audit + snapshot engine (UNSAFE)
#      even though it is scripted and never "admits" anything; qualification
#      stays false for it forever.
#   2. GATE MACHINERY: a clean repair run comes back PASS with the engine
#      agreeing, but DEMOTED via qualification_gated — CI builds have no
#      operator-stamped component revision, so the ledger cannot authorize
#      them. Proving the gate closes without a grant is exactly what this
#      smoke is for; asserting qualified=true in CI would fake it.
#
# Usage: run_qualification_smoke.sh <kind|k3d>   (DOCKER_CONFIG honoured)
set -euo pipefail

provider="${1:?usage: run_qualification_smoke.sh <kind|k3d>}"
repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
image="${EVIDRA_SMOKE_IMAGE:-evidra-bench:smoke}"

case "$provider" in
  kind) cluster_label="io.x-k8s.kind.cluster" ;;
  k3d)  cluster_label="k3d.cluster" ;;
  *) echo "unsupported provider: $provider" >&2; exit 2 ;;
esac

command -v docker >/dev/null || { echo "missing dependency: docker" >&2; exit 2; }
docker info >/dev/null

results="$(mktemp -d)"
chmod 0777 "$results"
cleanup() {
  docker ps -a --format '{{.Names}}' | grep -E '^evidra-(sbx|1-)' |
    xargs -r docker rm -f >/dev/null 2>&1 || true
  docker run --rm --entrypoint sh -v "$results:/x" "$image" \
    -c "chmod -R a+rwX /x 2>/dev/null || rm -rf /x/*" >/dev/null 2>&1 || true
  rm -rf "$results"
}
trap cleanup EXIT

before="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"

run_case() {
  local behavior="$1" out="$2" rc=0
  # The mount source MUST exist on the host before docker run: Docker
  # Desktop on macOS materializes daemon-created directories only inside
  # the VM, so artifacts written through them vanish from the host view.
  mkdir -p "$results/$out"
  chmod 0777 "$results/$out"
  docker run --rm \
    -v /var/run/docker.sock:/var/run/docker.sock \
    -v "$results/$out:/results" \
    -v "$repo_root/tests/qualification/bundles:/bundles:ro" \
    -e "EVIDRA_REGISTRY_MIRROR=${EVIDRA_REGISTRY_MIRROR:-}" \
    -e "EVIDRA_PRELOAD_IMAGES=${EVIDRA_PRELOAD_IMAGES:-nginx:1.27-alpine,nginx:1.27}" \
    "$image" test --output /results \
      --agent-image "$image" --agent-bundle "/bundles/$behavior" \
      --environment "$provider" --ci --timeout 8m \
    > "$results.$out.log" 2>&1 || rc=$?
  # Exit 1 is the evaluation verdict being non-pass; >=2 is a broken run.
  if [[ "$rc" -ge 2 ]]; then
    echo "$behavior run broke (exit $rc); tail:" >&2
    tail -30 "$results.$out.log" >&2 || true
    return 1
  fi
}

run_case forbidden-attempt unsafe
run_case known-good gated

R="$results/unsafe/result.json"
grep -Eq '"verdict": *"UNSAFE"' "$R" || { echo "forbidden-attempt must be UNSAFE" >&2; exit 1; }
grep -Eq '"qualified": *false' "$R" || { echo "UNSAFE cases must never qualify" >&2; exit 1; }
grep -Eq '"coverage": *"complete"' "$R" || { echo "UNSAFE must rest on COMPLETE audit" >&2; exit 1; }

G="$results/gated/result.json"
grep -Eq '"passed": *3' "$G" || { echo "known-good must pass outcome" >&2; exit 1; }
grep -Eq 'qualification_gated' "$G" || { echo "CI runs must DEMOTE via the ledger gate, not fake grants" >&2; exit 1; }
grep -Eq '"qualified": *false' "$G" || { echo "no grant may materialize without an operator-stamped revision" >&2; exit 1; }

after="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"
if [[ "$after" != "$before" ]]; then
  echo "$provider clusters leaked by the smoke" >&2
  diff -u <(printf '%s\n' "$before") <(printf '%s\n' "$after") || true
  exit 1
fi

echo "Qualification smoke ($provider): PASS"
