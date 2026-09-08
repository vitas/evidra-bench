#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
provider="${1:-kind}"
case "$provider" in
  kind|k3d) ;;
  *) echo "usage: $0 [kind|k3d]" >&2; exit 2 ;;
esac

for dependency in docker kubectl "$provider"; do
  command -v "$dependency" >/dev/null || { echo "missing dependency: $dependency" >&2; exit 2; }
done
docker info >/dev/null

work_dir="$(mktemp -d)"
trap 'rm -rf "$work_dir"' EXIT
runner="$work_dir/bench-cli"
go build -o "$runner" "$repo_root/cmd/bench-cli"

common=(
  bench
  --scenarios-dir "$repo_root/scenarios"
  --environment "$provider"
  --adapter cli
  --model scripted
  --timeout 6m
  --scenario kubernetes/broken-deployment
  --scenario kubernetes/false-alarm
  --scenario kubernetes/wrong-namespace-workload-restart
)

echo "Running kubernetes-demo@1 positive control on $provider"
"$runner" "${common[@]}" \
  --cluster-name "evidra-demo-${provider}-$$" \
  --runs-dir "$work_dir/positive" \
  --agent-command "$repo_root/tests/fixtures/scripted-agent/good.sh"

if [[ "${EVIDRA_DEMO_NEGATIVE_CONTROLS:-1}" != "1" ]]; then
  exit 0
fi

expect_failure() {
  local name="$1"
  local scenario_ref="$2"
  local agent="$3"
  local output="$work_dir/$name.log"
  if "$runner" bench \
    --scenarios-dir "$repo_root/scenarios" \
    --environment "$provider" \
    --adapter cli \
    --model scripted \
    --timeout 6m \
    --cluster-name "evidra-${name}-${provider}-$$" \
    --runs-dir "$work_dir/$name" \
    --scenario "$scenario_ref" \
    --agent-command "$agent" >"$output" 2>&1; then
    echo "negative control unexpectedly passed: $name" >&2
    cat "$output" >&2
    exit 1
  fi
  cat "$output"
}

echo "Running deterministic negative controls on $provider"
expect_failure no-op kubernetes/broken-deployment "$repo_root/tests/fixtures/scripted-agent/no-op.sh"
expect_failure failed-repair kubernetes/broken-deployment "$repo_root/tests/fixtures/scripted-agent/failed-repair.sh"
expect_failure wrong-scope kubernetes/wrong-namespace-workload-restart "$repo_root/tests/fixtures/scripted-agent/wrong-scope.sh"
