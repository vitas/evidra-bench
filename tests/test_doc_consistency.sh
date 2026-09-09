#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")/.."

fail() {
  echo "FAIL: $*" >&2
  exit 1
}

grep -q 'bin/bench-cli scenario list' README.md ||
  fail "README Quick Start should use bin/bench-cli after make build"

if grep -qE '(^|[[:space:]])bench-cli (scenario list|run |bench |certify |lab)' README.md; then
  fail "README Quick Start and provider examples should not show bare bench-cli commands after make build"
fi

grep -q '"stop" or "continue"' docs/SCENARIO_SCHEMA.md ||
  fail "Scenario schema should document on_fail as stop or continue"

if grep -q '"continue" or "abort"' docs/SCENARIO_SCHEMA.md; then
  fail "Scenario schema still documents unsupported on_fail abort value"
fi

if grep -qi 'committable to git' docs/LAB_TUI_GUIDE.md; then
  fail "Lab TUI guide must not say raw runs/results.jsonl is committable"
fi

grep -q '^runs/$' .gitignore ||
  fail ".gitignore should continue ignoring generated runs/"

grep -q 'Local vs Hosted Work Queues' docs/ARCHITECTURE.md ||
  fail "Architecture docs should explicitly separate local and hosted queues"

grep -q 'This package is not the hosted Bench runner control-plane queue' pkg/jobqueue/doc.go ||
  fail "pkg/jobqueue should document that it is local CLI parallel execution"

grep -q '^## Try It In One Command' README.md ||
  fail "README should lead with the one-command Docker runner"

grep -q 'ghcr.io/vitas/evidra-bench:latest' README.md ||
  fail "README should use the public Evidra runner image"

grep -q '/var/run/docker.sock:/var/run/docker.sock' README.md ||
  fail "README one-command example should mount the Docker socket"

grep -qi 'host-level control' README.md ||
  fail "README should warn that the Docker socket grants host-level control"

if grep -q -- '--group-add keep-groups' README.md; then
  fail "README should not recommend Podman-only keep-groups to Docker users"
fi

grep -q 'ghcr.io/vitas/evidra-bench:latest' docs/QUICKSTART.md ||
  fail "Quickstart should start with the Docker runner image"

grep -q -- '--environment k3d' docs/QUICKSTART.md ||
  fail "Quickstart should document optional k3d support"

grep -q 'PASS / FAIL / UNSAFE' docs/QUICKSTART.md ||
  fail "Quickstart should explain the primary verdicts"

grep -q 'evidra-results/report.html' docs/QUICKSTART.md ||
  fail "Quickstart should point to the standalone HTML report"

grep -q 'defaults to CLI help' docs/RUNNER_ARCHITECTURE.md ||
  fail "Runner architecture should distinguish the CLI default from explicit serve"

grep -q 'internal kind Docker network' docs/RUNNER_ARCHITECTURE.md ||
  fail "Runner architecture should document container-to-cluster networking"

grep -q 'cluster-specific Docker network' docs/RUNNER_ARCHITECTURE.md ||
  fail "Runner architecture should document containerized k3d networking"

grep -q '`evidra test` plus infrastructure tooling and versioned suite assets' docs/ARCHITECTURE.md ||
  fail "Architecture image table should describe the one-command runner"

grep -q -- '--model ollama/qwen3:8b' README.md ||
  fail "README should test local Ollama models with evidra test, not only demo"

grep -q '^  demo$' README.md ||
  fail "README should offer demo as the thin local-model entry point"

grep -q -- '--network host' README.md ||
  fail "README should document host networking for reaching the local runtime"

grep -q 'docker run --rm -it --network host' README.md ||
  fail "README demo command should allocate an interactive terminal for model selection"

grep -q -- '--model ollama/qwen3:8b' docs/QUICKSTART.md ||
  fail "Quickstart should include a first-class local-model test path"

grep -q 'docker run --rm -it --network host' docs/QUICKSTART.md ||
  fail "Quickstart demo command should allocate an interactive terminal for model selection"

grep -qi 'never downloaded automatically\|never downloads' README.md docs/QUICKSTART.md ||
  true

grep -qi 'before creating any cluster' docs/QUICKSTART.md ||
  fail "Quickstart should state capability validation happens before cluster creation"

grep -q 'HostConfig.NetworkMode' docs/RUNNER_ARCHITECTURE.md ||
  fail "Runner architecture should document network-mode detection for containers"

grep -q 'EVIDRA_RUNNER_CONTAINER' docs/RUNNER_ARCHITECTURE.md ||
  fail "Runner architecture should document the runner container identity override"

grep -qi 'before cluster provisioning' docs/ARCHITECTURE.md ||
  fail "Architecture should place local-model validation before cluster provisioning"

echo "PASS: test_doc_consistency"
