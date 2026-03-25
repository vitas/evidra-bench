#!/bin/bash
# Run ALL active scenarios through evidra-mcp with multiple models.
# Usage: ./scripts/run-full-evidra-mcp-bench.sh
#
# Prerequisites:
#   - source .env && export $(grep -v '^#' .env | grep -v '^$' | xargs)
#   - evidra-mcp installed (brew install samebits/tap/evidra or build from source)
#   - kind cluster running (infra-bench creates one if needed)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
if [ -f "$PROJECT_DIR/.env" ]; then
  set -a
  source "$PROJECT_DIR/.env"
  set +a
  echo "Loaded .env"
fi

MCP_SERVER="evidra-mcp --signing-mode optional"

MODELS=(
  "gemini-2.5-flash"
  "gemini-2.5-pro"
  "gpt-4.1"
  "gpt-5.2"
  "claude-sonnet-4-20250514"
  "deepseek-chat"
  "qwen-plus"
)

provider_url_for_model() {
  case "$1" in
    gpt-*|o1-*|o3-*)   echo "https://api.openai.com/v1" ;;
    claude-*)           echo "https://api.anthropic.com/v1" ;;
    gemini-*)           echo "https://generativelanguage.googleapis.com/v1beta/openai" ;;
    deepseek-*)         echo "https://api.deepseek.com/v1" ;;
    llama-*|mixtral-*)  echo "https://api.groq.com/openai/v1" ;;
    qwen*)              echo "https://dashscope-intl.aliyuncs.com/compatible-mode/v1" ;;
    *)                  echo "${EVIDRA_BIFROST_BASE_URL:-http://localhost:8080/openai}" ;;
  esac
}

provider_key_for_model() {
  case "$1" in
    gpt-*|o1-*|o3-*)   echo "${OPENAI_API_KEY:-}" ;;
    claude-*)           echo "${ANTHROPIC_API_KEY:-}" ;;
    gemini-*)           echo "${GEMINI_API_KEY:-}" ;;
    deepseek-*)         echo "${DEEPSEEK_API_KEY:-}" ;;
    llama-*|mixtral-*)  echo "${GROQ_API_KEY:-}" ;;
    qwen*)              echo "${DASHSCOPE_API_KEY:-}" ;;
    *)                  echo "${EVIDRA_BIFROST_AUTH_BEARER:-}" ;;
  esac
}

# Get all active scenarios
SCENARIOS=$(find "$PROJECT_DIR/scenarios" -name scenario.yaml -exec grep -L "^skip: true" {} \; | \
  sed "s|$PROJECT_DIR/scenarios/||;s|/scenario.yaml||" | sort)

SCENARIO_COUNT=$(echo "$SCENARIOS" | wc -l | tr -d ' ')
MODEL_COUNT=${#MODELS[@]}
TOTAL_RUNS=$((SCENARIO_COUNT * MODEL_COUNT))

echo "════════════════════════════════════════"
echo "  EVIDRA-MCP FULL BENCHMARK"
echo "  $SCENARIO_COUNT scenarios × $MODEL_COUNT models = $TOTAL_RUNS runs"
echo "  MCP Server: $MCP_SERVER"
echo "════════════════════════════════════════"

# Build
make -C "$PROJECT_DIR" build

PASSED=0
FAILED=0
SKIPPED=0
RESULTS=()
RESULTS_FILE="$PROJECT_DIR/runs/results.jsonl"

# Check if a run already exists in results.jsonl
run_exists() {
  local scenario_id="$1" model="$2"
  # Extract scenario_id (last path component)
  scenario_id="${scenario_id##*/}"
  if [ -f "$RESULTS_FILE" ]; then
    grep -q "\"scenario_id\":\"${scenario_id}\".*\"model\":\"${model}\".*\"evidence_mode\":\"mcp\"" "$RESULTS_FILE" && return 0
    grep -q "\"model\":\"${model}\".*\"scenario_id\":\"${scenario_id}\".*\"evidence_mode\":\"mcp\"" "$RESULTS_FILE" && return 0
  fi
  return 1
}

for MODEL in "${MODELS[@]}"; do
  export EVIDRA_BIFROST_BASE_URL=$(provider_url_for_model "$MODEL")
  export EVIDRA_BIFROST_AUTH_BEARER=$(provider_key_for_model "$MODEL")

  if [ -z "$EVIDRA_BIFROST_AUTH_BEARER" ]; then
    echo "SKIP $MODEL — no API key"
    SKIPPED=$((SKIPPED + SCENARIO_COUNT))
    continue
  fi

  for SCENARIO in $SCENARIOS; do
    RUN_NUM=$((PASSED + FAILED + SKIPPED + 1))

    if run_exists "$SCENARIO" "$MODEL"; then
      echo "[$RUN_NUM/$TOTAL_RUNS] SKIP (already done) $SCENARIO / $MODEL"
      SKIPPED=$((SKIPPED + 1))
      continue
    fi

    echo ""
    echo "[$RUN_NUM/$TOTAL_RUNS] $SCENARIO / $MODEL (via evidra-mcp)"

    if bin/infra-bench run \
      --scenario "$SCENARIO" \
      --model "$MODEL" \
      --provider bifrost \
      --mcp-server "$MCP_SERVER" \
      --reuse-cluster \
      --timeout 10m \
      --evidra-url "https://api.evidra.cc" \
      --evidra-api-key "${EVIDRA_API_KEY:?EVIDRA_API_KEY must be set}" \
      2>&1; then
      PASSED=$((PASSED + 1))
      RESULTS+=("PASS  $SCENARIO  $MODEL")
    else
      FAILED=$((FAILED + 1))
      RESULTS+=("FAIL  $SCENARIO  $MODEL")
    fi
  done
done

echo ""
echo "════════════════════════════════════════"
echo "  EVIDRA-MCP BENCHMARK RESULTS"
echo "════════════════════════════════════════"
echo "  Total: $((PASSED + FAILED))  Pass: $PASSED  Fail: $FAILED  Skip: $SKIPPED"
echo ""
for R in "${RESULTS[@]}"; do
  echo "  $R"
done
echo "════════════════════════════════════════"
