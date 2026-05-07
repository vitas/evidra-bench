#!/bin/bash
set -euo pipefail

# Overnight benchmark — seeds the Bench dashboard with real data.
#
# Runs scenarios in two parallel tracks per model:
#   Track 1: baseline
#   Track 2: --mcp-server "$MCP_SERVER"
#
# Each track gets its own k3d cluster to avoid conflicts.
#
# Prerequisites:
#   - source .env && export $(grep -v '^#' .env | grep -v '^$' | xargs)
#   - Docker running (for k3d clusters)
#   - bin/bench-cli built (make build)
#   - MCP server installed
#
# Usage:
#   ./scripts/overnight-bench.sh
#
# With repeats for pass^k:
#   REPEATS=3 ./scripts/overnight-bench.sh
#
# Expected: ~3-4 hours for 1 repeat, ~10 hours for 3 repeats

REPEATS="${REPEATS:-1}"
BINARY="${BINARY:-bin/bench-cli}"
ENVIRONMENT="${ENVIRONMENT:-k3d}"
BENCH_API_URL="${BENCH_API_URL:?BENCH_API_URL must be set}"
MCP_SERVER="${MCP_SERVER:-}"

if [ ! -x "$BINARY" ]; then
  echo "ERROR: $BINARY not found. Run 'make build' first."
  exit 1
fi
if [ -z "${BENCH_API_KEY:-}" ]; then
  echo "ERROR: BENCH_API_KEY not set."
  exit 1
fi
if [ -z "$MCP_SERVER" ]; then
  echo "WARN: MCP_SERVER not set — MCP track will be skipped."
fi

STAMP=$(date +%Y%m%d-%H%M%S)
LOG_DIR="runs/overnight-${STAMP}"
mkdir -p "$LOG_DIR"

echo "=== Overnight Benchmark ${STAMP} ==="
echo "  Repeats:     ${REPEATS}"
echo "  Environment: ${ENVIRONMENT}"
echo "  Log dir:     ${LOG_DIR}"
echo ""

# run_bench MODEL BASE_URL KEY_VAR CLUSTER MODE [MCP_CMD]
run_bench() {
  local model="$1" base_url="$2" key_var="$3" cluster="$4" mode="$5" mcp_cmd="${6:-}"
  local key_val log_file
  local mode_args=()

  eval "key_val=\${${key_var}:-}"
  if [ -z "$key_val" ]; then
    echo "SKIP $model/$mode — $key_var not set"
    return
  fi

  log_file="${LOG_DIR}/${model}-${mode}.log"

  if [ "$mode" = "mcp" ]; then
    mode_args=(--mcp-server "$mcp_cmd")
  fi

  echo "  START $model/$mode → $log_file"

  (
    export INFRA_BENCH_BIFROST_URL="$base_url"
    export INFRA_BENCH_BIFROST_AUTH_BEARER="$key_val"

    # Kubernetes + Helm (51 active scenarios, default profile).
    "$BINARY" bench \
      --scenario kubernetes --scenario helm \
      --model "$model" --provider bifrost \
      --repeats "$REPEATS" \
      --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "$cluster" \
      "${mode_args[@]}" \
      --bench-url "$BENCH_API_URL" --bench-api-key "$BENCH_API_KEY" \
      2>&1 || echo "WARN: $model/$mode k8s+helm exited $?"

    # ArgoCD (4 active, argocd profile — separate cluster).
    "$BINARY" bench \
      --scenario argocd \
      --model "$model" --provider bifrost \
      --repeats "$REPEATS" \
      --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "${cluster}-argo" \
      "${mode_args[@]}" \
      --bench-url "$BENCH_API_URL" --bench-api-key "$BENCH_API_KEY" \
      2>&1 || echo "WARN: $model/$mode argocd exited $?"

    # Terraform (5 active).
    "$BINARY" bench \
      --scenario terraform \
      --model "$model" --provider bifrost \
      --repeats "$REPEATS" \
      --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "$cluster" \
      "${mode_args[@]}" \
      --bench-url "$BENCH_API_URL" --bench-api-key "$BENCH_API_KEY" \
      2>&1 || echo "WARN: $model/$mode terraform exited $?"

    echo "DONE $model/$mode"
  ) > "$log_file" 2>&1 &
}

# Launch all tracks. Two parallel per model (baseline + mcp).
# Models run sequentially to avoid overloading the machine with clusters.

run_model() {
  local model="$1" base_url="$2" key_var="$3" prefix="$4"

  echo ""
  echo "════ $model ════"

  # Track 1: baseline mode
  run_bench "$model" "$base_url" "$key_var" "${prefix}-baseline" "baseline"

  # Track 2: MCP mode.
  if [ -n "$MCP_SERVER" ]; then
    run_bench "$model" "$base_url" "$key_var" "${prefix}-mcp" "mcp" \
      "$MCP_SERVER"
  fi

  # Wait for both tracks to finish before moving to next model.
  echo "  Waiting for $model tracks..."
  wait
  echo "  $model complete."

  # Clean up this model's clusters.
  k3d cluster delete "${prefix}-baseline" "${prefix}-baseline-argo" \
    "${prefix}-mcp" "${prefix}-mcp-argo" 2>/dev/null || true
}

{
  run_model "gemini-2.5-flash" \
    "https://generativelanguage.googleapis.com/v1beta/openai" \
    "GEMINI_API_KEY" "bn-gem"

  run_model "deepseek-chat" \
    "https://api.deepseek.com/v1" \
    "DEEPSEEK_API_KEY" "bn-ds"

  run_model "qwen-plus" \
    "https://dashscope-intl.aliyuncs.com/compatible-mode/v1" \
    "DASHSCOPE_API_KEY" "bn-qw"

  echo ""
  echo "════════════════════════════════════════"
  echo "  ALL DONE"
  echo "════════════════════════════════════════"
  echo "  Logs: ${LOG_DIR}/"
  echo "  Results: ${BENCH_API_URL}"
} 2>&1 | tee "${LOG_DIR}/main.log"
