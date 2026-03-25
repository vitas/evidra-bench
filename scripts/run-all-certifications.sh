#!/bin/bash
# Run all certification exams across all models.
# Usage: ./scripts/run-all-certifications.sh
#
# Prerequisites:
#   - source .env && export $(grep -v '^#' .env | grep -v '^$' | xargs)
#   - Bifrost running or EVIDRA_BIFROST_BASE_URL pointing at provider
#   - kind or k3d installed
set -euo pipefail

# ── Load .env if present ──
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
if [ -f "$PROJECT_DIR/.env" ]; then
  set -a
  source "$PROJECT_DIR/.env"
  set +a
  echo "Loaded .env from $PROJECT_DIR/.env"
fi

# ── Configuration ──
EXAMS=("cka" "cks")
MODELS=(
  "gpt-4o"
  "gpt-4.1"
  "gpt-5.2"
  "claude-sonnet-4-20250514"
  "gemini-2.5-flash"
  "gemini-2.5-pro"
  "qwen-plus"
  "deepseek-chat"
)

PROVIDER="${PROVIDER:-bifrost}"
EVIDRA_URL="${EVIDRA_URL:-https://api.evidra.cc}"
EVIDRA_API_KEY="${EVIDRA_API_KEY:?EVIDRA_API_KEY must be set}"
EXTRA_FLAGS="${EXTRA_FLAGS:---proxy-mode --reuse-cluster}"

# ── Provider URL per model (when not using Bifrost gateway) ──
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

# ── Build ──
echo "Building infra-bench..."
make build

# ── Run ──
TOTAL=0
PASSED=0
FAILED=0
RESULTS=()

for EXAM in "${EXAMS[@]}"; do
  for MODEL in "${MODELS[@]}"; do
    TOTAL=$((TOTAL + 1))

    # Set provider URL and key for this model
    export EVIDRA_BIFROST_BASE_URL=$(provider_url_for_model "$MODEL")
    export EVIDRA_BIFROST_AUTH_BEARER=$(provider_key_for_model "$MODEL")

    if [ -z "$EVIDRA_BIFROST_AUTH_BEARER" ]; then
      echo "⏭️  SKIP ${EXAM} / ${MODEL} — no API key"
      RESULTS+=("SKIP  ${EXAM}  ${MODEL}  (no key)")
      continue
    fi

    echo ""
    echo "════════════════════════════════════════"
    echo "  ${EXAM} / ${MODEL}"
    echo "════════════════════════════════════════"

    if bin/infra-bench certify \
      --track "$EXAM" \
      --model "$MODEL" \
      --provider "$PROVIDER" \
      --evidra-url "$EVIDRA_URL" \
      --evidra-api-key "$EVIDRA_API_KEY" \
      $EXTRA_FLAGS 2>&1; then
      PASSED=$((PASSED + 1))
      RESULTS+=("PASS  ${EXAM}  ${MODEL}")
    else
      FAILED=$((FAILED + 1))
      RESULTS+=("FAIL  ${EXAM}  ${MODEL}")
    fi
  done
done

# ── Summary ──
echo ""
echo "════════════════════════════════════════════════════"
echo "  ALL CERTIFICATIONS COMPLETE"
echo "════════════════════════════════════════════════════"
echo ""
for R in "${RESULTS[@]}"; do
  echo "  $R"
done
echo ""
echo "  Total: $TOTAL  Passed: $PASSED  Failed: $FAILED"
echo "════════════════════════════════════════════════════"
