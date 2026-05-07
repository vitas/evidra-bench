#!/bin/sh
set -u

# Overnight lite — kubernetes + helm only, 2 parallel clusters max.
# Baseline + MCP per model, models sequential.
#
# Usage: source .env && nohup ./scripts/overnight-lite.sh &

BINARY="${BINARY:-bin/bench-cli}"
ENVIRONMENT="${ENVIRONMENT:-k3d}"
BENCH_API_URL="${BENCH_API_URL:-https://api.evidra.cc}"
REPEATS="${REPEATS:-1}"

STAMP=$(date +%Y%m%d-%H%M%S)
LOG_DIR="runs/overnight-${STAMP}"
mkdir -p "$LOG_DIR"

echo "=== Overnight Lite ${STAMP} ==="
echo "  Scenarios:   kubernetes + helm (51 active)"
echo "  Models:      gemini-2.5-flash, deepseek-chat, qwen-plus"
echo "  Modes:       baseline + mcp (2 clusters max)"
echo "  Repeats:     ${REPEATS}"
echo "  Log dir:     ${LOG_DIR}"
echo ""

run_model() {
  local model="$1" base_url="$2" key_var="$3" prefix="$4"
  local key_val
  eval "key_val=\${${key_var}:-}"

  if [ -z "$key_val" ]; then
    echo "SKIP $model — $key_var not set"
    return
  fi

  echo ""
  echo "════ $model ════"

  # Track 1: baseline
  (
    export INFRA_BENCH_BIFROST_URL="$base_url"
    export INFRA_BENCH_BIFROST_AUTH_BEARER="$key_val"
    "$BINARY" bench \
      --scenario kubernetes --scenario helm \
      --model "$model" --provider bifrost \
      --repeats "$REPEATS" \
      --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "${prefix}-b" \
      --bench-url "$BENCH_API_URL" --bench-api-key "$BENCH_API_KEY" \
      2>&1
    echo "DONE $model/baseline"
  ) > "${LOG_DIR}/${model}-baseline.log" 2>&1 &
  local pid_baseline=$!

  # Track 2: mcp
  (
    export INFRA_BENCH_BIFROST_URL="$base_url"
    export INFRA_BENCH_BIFROST_AUTH_BEARER="$key_val"
    "$BINARY" bench \
      --scenario kubernetes --scenario helm \
      --model "$model" --provider bifrost \
      --repeats "$REPEATS" \
      --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "${prefix}-m" \
      --mcp-server "evidra-mcp --signing-mode optional" \
      --bench-url "$BENCH_API_URL" --bench-api-key "$BENCH_API_KEY" \
      2>&1
    echo "DONE $model/mcp"
  ) > "${LOG_DIR}/${model}-mcp.log" 2>&1 &
  local pid_mcp=$!

  echo "  baseline PID=$pid_baseline  mcp PID=$pid_mcp"
  echo "  Waiting for $model..."
  wait $pid_baseline $pid_mcp
  echo "  $model complete."

  # Clean clusters before next model.
  k3d cluster delete "${prefix}-b" "${prefix}-m" 2>/dev/null || true
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
  echo "  ALL DONE — $(date)"
  echo "════════════════════════════════════════"
  echo "  Logs: ${LOG_DIR}/"

  # Summary
  for f in "${LOG_DIR}"/*.log; do
    name=$(basename "$f" .log)
    total=$(grep -c 'PASS\|FAIL' "$f" 2>/dev/null || echo 0)
    passed=$(grep -c 'PASS' "$f" 2>/dev/null || echo 0)
    echo "  $name: $passed/$total"
  done
} 2>&1 | tee "${LOG_DIR}/main.log"
