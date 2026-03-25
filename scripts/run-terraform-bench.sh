#!/bin/bash
# Run all 4 terraform scenarios across multiple models.
# Usage: ./scripts/run-terraform-bench.sh
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
if [ -f "$PROJECT_DIR/.env" ]; then
  set -a
  source "$PROJECT_DIR/.env"
  set +a
  echo "Loaded .env"
fi

SCENARIOS=(
  "terraform/plan-apply-partial-failure"
  "terraform/state-drift"
  "terraform/state-mv-refactor"
  "terraform/import-existing"
)

MODELS=(
  "gemini-2.5-flash"
  "gpt-4o"
  "gpt-4.1"
  "claude-sonnet-4-20250514"
  "deepseek-chat"
)

provider_url_for_model() {
  case "$1" in
    gpt-*|o1-*|o3-*)   echo "https://api.openai.com/v1" ;;
    claude-*)           echo "https://api.anthropic.com/v1" ;;
    gemini-*)           echo "https://generativelanguage.googleapis.com/v1beta/openai" ;;
    deepseek-*)         echo "https://api.deepseek.com/v1" ;;
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
    qwen*)              echo "${DASHSCOPE_API_KEY:-}" ;;
    *)                  echo "${EVIDRA_BIFROST_AUTH_BEARER:-}" ;;
  esac
}

TOTAL=0
PASSED=0
FAILED=0
RESULTS=()

for MODEL in "${MODELS[@]}"; do
  export EVIDRA_BIFROST_BASE_URL=$(provider_url_for_model "$MODEL")
  export EVIDRA_BIFROST_AUTH_BEARER=$(provider_key_for_model "$MODEL")

  if [ -z "$EVIDRA_BIFROST_AUTH_BEARER" ]; then
    echo "SKIP $MODEL — no API key"
    continue
  fi

  for SCENARIO in "${SCENARIOS[@]}"; do
    TOTAL=$((TOTAL + 1))
    echo ""
    echo "════════════════════════════════════════"
    echo "  $SCENARIO / $MODEL"
    echo "════════════════════════════════════════"

    if bin/infra-bench run \
      --scenario "$SCENARIO" \
      --model "$MODEL" \
      --provider bifrost \
      --proxy-mode \
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
echo "  TERRAFORM BENCHMARK RESULTS"
echo "════════════════════════════════════════"
echo "  Total: $TOTAL  Pass: $PASSED  Fail: $FAILED"
echo ""
for R in "${RESULTS[@]}"; do
  echo "  $R"
done
echo "════════════════════════════════════════"
