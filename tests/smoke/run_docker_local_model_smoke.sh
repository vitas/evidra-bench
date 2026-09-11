#!/usr/bin/env bash
# Docker local-model smoke: proves the first-class Ollama workflow end to end
# against the deterministic fake-ollama fixture. No real Ollama, no model
# download, no GPU, no provider credential.
#
# The fake Ollama server and the runner container both use Docker host
# networking, so the runner sees the fixture at the fixed local endpoint
# 127.0.0.1:11434 exactly like a real local Ollama runtime.
#
# The smoke proves:
#   * discovery (/api/tags, /api/show) happens before any cluster exists;
#   * inference uses the shared OpenAI-compatible /v1/chat/completions path;
#   * no pull or unapproved endpoint is touched (fixture logs VIOLATION);
#   * a missing model aborts with exit 2 and creates zero cluster resources;
#   * canonical reports and signed evidence bundles are produced;
#   * kind/k3d leave no containers or k3d networks behind.
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
image="${EVIDRA_DOCKER_SMOKE_IMAGE:-evidra-bench:local-model-smoke}"
provider="${1:-kind}"

# shellcheck source=assert_evaluation_artifacts.sh
source "$repo_root/tests/smoke/assert_evaluation_artifacts.sh"

case "$provider" in
  kind|k3d) ;;
  *) echo "unsupported smoke-test environment: $provider" >&2; exit 2 ;;
esac

command -v docker >/dev/null || { echo "missing dependency: docker" >&2; exit 2; }
command -v go >/dev/null || { echo "missing dependency: go" >&2; exit 2; }
docker info >/dev/null

work_dir="$(mktemp -d)"
fixture_name="evidra-fake-ollama-$$"
runner_name="evidra-local-model-runner-$$"
fixture_ready=0

cleanup() {
  docker rm -f "$fixture_name" "$runner_name" >/dev/null 2>&1 || true
  # The runner process writes root-owned artifacts into the bind-mounted
  # results dir; delete them with root inside a throwaway container so the
  # cleanup cannot fail the job on Linux (CI runs as an unprivileged uid).
  results_dir="$work_dir/results"
  if [[ -d "$results_dir" ]]; then
    docker run --rm -v "$results_dir:/mnt" alpine:3.22 \
      find /mnt -mindepth 1 -delete >/dev/null 2>&1 || true
  fi
  rm -rf "$work_dir" 2>/dev/null || true
}
trap cleanup EXIT

if [[ -z "${EVIDRA_DOCKER_SMOKE_IMAGE:-}" ]]; then
  docker build -f "$repo_root/Dockerfile.bench" -t "$image" "$repo_root"
fi

# Build the static Linux fixture binary and run it in the host network
# namespace of the Docker VM/host, reachable at 127.0.0.1:11434 there.
case "$(uname -m)" in
  arm64|aarch64) fixture_arch=arm64 ;;
  x86_64|amd64) fixture_arch=amd64 ;;
  *) echo "unsupported fixture architecture: $(uname -m)" >&2; exit 2 ;;
esac
CGO_ENABLED=0 GOOS=linux GOARCH="$fixture_arch" \
  go build -o "$work_dir/fake-ollama" "$repo_root/tests/fixtures/fake-ollama"

docker run -d --rm --network host --name "$fixture_name" \
  -v "$work_dir/fake-ollama:/usr/local/bin/fake-ollama:ro" \
  alpine:3.22 \
  /usr/local/bin/fake-ollama -addr 127.0.0.1:11434 >/dev/null

for _ in $(seq 1 40); do
  if docker exec "$fixture_name" wget -q -O - http://127.0.0.1:11434/api/tags >/dev/null 2>&1; then
    fixture_ready=1
    break
  fi
  sleep 0.5
done
if [[ "$fixture_ready" -ne 1 ]]; then
  echo "fake-ollama fixture did not become ready" >&2
  docker logs "$fixture_name" >&2 || true
  exit 1
fi

if [[ "$provider" == "kind" ]]; then
  cluster_label="io.x-k8s.kind.cluster"
else
  cluster_label="k3d.cluster"
fi
before="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"

result_dir="$work_dir/results"
mkdir -p "$result_dir"
chmod 0777 "$result_dir"

# Preflight boundary: a model that the runtime does not report must fail the
# command with zero cluster activity — the discovery endpoint answers, no
# pull or create endpoint is ever called, and no control-plane container is
# created.
set +e
docker run --rm --network host --name "$runner_name" \
  -e EVIDRA_RUNNER_CONTAINER="$runner_name" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$result_dir:/workspace/evidra-results" \
  "$image" \
  test --model ollama/absent-model:0x --environment "$provider" --ci --timeout 6m >"$work_dir/missing.log" 2>&1
missing_exit=$?
set -e
if [[ "$missing_exit" -eq 0 ]]; then
  echo "missing model must not pass preflight" >&2
  cat "$work_dir/missing.log" >&2
  exit 1
fi
grep -qi 'not installed' "$work_dir/missing.log" || {
  echo "missing-model error must name the boundary; got:" >&2
  cat "$work_dir/missing.log" >&2
  exit 1
}
missing_after="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"
if [[ "$missing_after" != "$before" ]]; then
  echo "preflight failure must never create clusters" >&2
  diff -u <(printf '%s\n' "$before") <(printf '%s\n' "$missing_after") || true
  exit 1
fi

# Full first-class local-model run through plain `evidra test`.
docker run --rm --network host --name "$runner_name" \
  -e EVIDRA_RUNNER_CONTAINER="$runner_name" \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$result_dir:/workspace/evidra-results" \
  "$image" \
  test --model ollama/qwen3-fake:1b --environment "$provider" --ci --timeout 8m

assert_evaluation_artifacts "$result_dir"

fixture_log="$work_dir/fixture.log"
docker logs "$fixture_name" >"$fixture_log" 2>&1

if grep -q 'VIOLATION' "$fixture_log"; then
  echo "runner touched a forbidden Ollama endpoint (pull/credentials):" >&2
  grep 'VIOLATION' "$fixture_log" >&2
  exit 1
fi
grep -q '^tags' "$fixture_log" || { echo "discovery via /api tags never happened" >&2; exit 1; }
grep -q '^show' "$fixture_log" || { echo "capability metadata via /api show never happened" >&2; exit 1; }
grep -q '^chat ' "$fixture_log" || { echo "inference never used the shared /v1 chat endpoint" >&2; exit 1; }
if [[ "$(grep -n '^chat ' "$fixture_log" | head -1 | cut -d: -f1)" -lt "$(grep -n '^tags' "$fixture_log" | head -1 | cut -d: -f1)" ]]; then
  echo "chat inference preceded model discovery" >&2
  exit 1
fi

after="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"
if [[ "$after" != "$before" ]]; then
  echo "$provider containers changed across local-model smoke" >&2
  diff -u <(printf '%s\n' "$before") <(printf '%s\n' "$after") || true
  exit 1
fi
if [[ "$provider" == "k3d" ]]; then
  leftover_networks="$(docker network ls --format '{{.Name}}' | grep -c '^k3d-' || true)"
  if [[ "$leftover_networks" -ne 0 ]]; then
    echo "k3d left $leftover_networks network(s) behind" >&2
    exit 1
  fi
fi

echo "Docker local-model smoke ($provider): PASS"
