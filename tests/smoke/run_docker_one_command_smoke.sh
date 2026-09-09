#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/../.." && pwd)"
image="${EVIDRA_DOCKER_SMOKE_IMAGE:-evidra-bench:smoke}"
provider="${1:-kind}"

case "$provider" in
  kind|k3d) ;;
  *) echo "unsupported smoke-test environment: $provider" >&2; exit 2 ;;
esac

command -v docker >/dev/null || { echo "missing dependency: docker" >&2; exit 2; }
docker info >/dev/null

if [[ -z "${EVIDRA_DOCKER_SMOKE_IMAGE:-}" ]]; then
  docker build -f "$repo_root/Dockerfile.bench" -t "$image" "$repo_root"
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

docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$result_dir:/workspace/evidra-results" \
  -v "$repo_root/tests/fixtures/scripted-agent/good.sh:/fixtures/good.sh:ro" \
  "$image" \
  test --agent /fixtures/good.sh --environment "$provider" --ci --timeout 6m

test -s "$result_dir/result.json"
test -s "$result_dir/report.html"
grep -Eq '"passed": 3' "$result_dir/result.json"
grep -Eq '"failed": 0' "$result_dir/result.json"
grep -Eq '"unsafe": 0' "$result_dir/result.json"
grep -Eq '"incomplete": 0' "$result_dir/result.json"

bundle_count="$(find "$result_dir/bundles" -name bundle.json 2>/dev/null | wc -l | tr -d ' ')"
if [[ "$bundle_count" -eq 0 ]]; then
  echo "expected signed evidence bundles under $result_dir/bundles after one-command test" >&2
  exit 1
fi

after="$(docker ps -a --filter "label=$cluster_label" --format '{{.Names}}' | sort)"
if [[ "$after" != "$before" ]]; then
  echo "$provider containers changed across smoke test" >&2
  diff -u <(printf '%s\n' "$before") <(printf '%s\n' "$after") || true
  exit 1
fi

echo "Docker one-command smoke ($provider): PASS"
