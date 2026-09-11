#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
image="${EVIDRA_DOCKER_SMOKE_IMAGE:-evidra-bench:smoke}"
provider="${1:-kind}"

# shellcheck source=assert_evaluation_artifacts.sh
source "$repo_root/tests/smoke/assert_evaluation_artifacts.sh"

case "$provider" in
  kind|k3d) ;;
  *) echo "unsupported smoke-test environment: $provider" >&2; exit 2 ;;
esac

command -v docker >/dev/null || { echo "missing dependency: docker" >&2; exit 2; }
docker info >/dev/null

if [[ -z "${EVIDRA_DOCKER_SMOKE_IMAGE:-}" ]]; then
  docker build -f "$repo_root/Dockerfile.bench" --build-arg "EVIDRA_BUILD_REVISION=$(git -C "$repo_root" rev-parse HEAD)" -t "$image" "$repo_root"
fi

docker run --rm "$image" --version | grep -q '^evidra version '
docker run --rm --entrypoint bench-cli "$image" --version | grep -q '^evidra version '

result_dir="$(mktemp -d)"
chmod 0777 "$result_dir"
trap 'rm -rf "$result_dir"' EXIT

if [[ "$provider" == "kind" ]]; then
  cluster_label="io.x-k8s.kind.cluster"
else
  cluster_label="k3d.cluster"
fi
before="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"

# ADR 0001 review #9: running as root inside the container made every
# result file root-owned on the host, so the trap's rm -rf failed with
# "Permission denied" on Linux CI — the smoke could not clean up after a
# PASSING evaluation. Run as the invoking UID/GID; group-add the socket
# group so the docker CLI (kind/k3d) still works. Locally (Docker
# Desktop) the mapping is cosmetic; on CI it is the difference between a
# green cleanup and a red one.
user_args=()
if [[ "$(uname)" == "Linux" ]]; then
  sock_gid="$(stat -c %g /var/run/docker.sock 2>/dev/null || echo 0)"
  user_args=(--user "$(id -u):$(id -g)" --group-add "$sock_gid")
fi

docker run --rm \
  "${user_args[@]}" \
  -e HOME=/workspace/evidra-results \
  -w /workspace \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$result_dir:/workspace/evidra-results" \
  -v "$repo_root/tests/fixtures/scripted-agent/good.sh:/fixtures/good.sh:ro" \
  "$image" \
  test --agent /fixtures/good.sh --environment "$provider" --ci --timeout 6m

assert_evaluation_artifacts "$result_dir"

after="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"
if [[ "$after" != "$before" ]]; then
  echo "$provider containers changed across smoke test" >&2
  diff -u <(printf '%s\n' "$before") <(printf '%s\n' "$after") || true
  exit 1
fi

echo "Docker one-command smoke ($provider): PASS"
