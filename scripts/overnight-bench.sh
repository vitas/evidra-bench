#!/bin/sh
set -eu

# Overnight benchmark — seeds the evidra.cc dashboard with real data.
#
# Runs all active scenarios across 3 cheap models with proxy mode
# (so evidence entries are created for the Evidence page).
#
# Prerequisites:
#   - source .env && export $(grep -v '^#' .env | grep -v '^$' | xargs)
#   - Docker running (for k3d clusters)
#   - bin/bench-cli built (make build)
#
# Usage:
#   ./scripts/overnight-bench.sh
#
# Expected duration: ~3-4 hours (62 scenarios × 3 models × 1 repeat)
# Expected cost: ~$5-10 (mostly Gemini Flash at $0.001/run)
#
# To run with repeats for pass^k data:
#   REPEATS=3 ./scripts/overnight-bench.sh
#   (adds ~8-10 hours, but fills pass^k with k=3 across all scenarios)

REPEATS="${REPEATS:-1}"
BINARY="${BINARY:-bin/bench-cli}"
ENVIRONMENT="${ENVIRONMENT:-k3d}"
EVIDRA_URL="${EVIDRA_URL:-https://api.evidra.cc}"

# Verify prerequisites.
if [ ! -x "$BINARY" ]; then
  echo "ERROR: $BINARY not found. Run 'make build' first."
  exit 1
fi
if [ -z "${EVIDRA_API_KEY:-}" ]; then
  echo "ERROR: EVIDRA_API_KEY not set. Run 'source .env && export \$(grep -v '^#' .env | grep -v '^\$' | xargs)'"
  exit 1
fi

# Timestamp for this batch.
STAMP=$(date +%Y%m%d-%H%M%S)
LOG="runs/overnight-${STAMP}.log"
mkdir -p runs

echo "=== Overnight Benchmark ${STAMP} ==="
echo "  Models:      gemini-2.5-flash, deepseek-chat, qwen-plus"
echo "  Repeats:     ${REPEATS}"
echo "  Environment: ${ENVIRONMENT}"
echo "  Evidra URL:  ${EVIDRA_URL}"
echo "  Log:         ${LOG}"
echo ""

run_model() {
  local model="$1" base_url="$2" key_var="$3" cluster="$4"
  local key_val
  eval "key_val=\${${key_var}:-}"

  if [ -z "$key_val" ]; then
    echo "SKIP $model — $key_var not set"
    return
  fi

  echo ""
  echo "════════════════════════════════════════"
  echo "  Model: $model"
  echo "  Cluster: $cluster"
  echo "════════════════════════════════════════"

  export EVIDRA_BIFROST_BASE_URL="$base_url"
  export EVIDRA_BIFROST_AUTH_BEARER="$key_val"

  # Run all kubernetes + helm scenarios (51 active).
  "$BINARY" bench \
    --scenario kubernetes --scenario helm \
    --model "$model" --provider bifrost \
    --repeats "$REPEATS" \
    --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "$cluster" \
    --proxy-mode \
    --evidra-url "$EVIDRA_URL" --evidra-api-key "$EVIDRA_API_KEY" \
    2>&1 || echo "WARN: $model kubernetes/helm batch exited $?"

  # Run ArgoCD scenarios (4 active, separate profile).
  "$BINARY" bench \
    --scenario argocd \
    --model "$model" --provider bifrost \
    --repeats "$REPEATS" \
    --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "${cluster}-argo" \
    --proxy-mode \
    --evidra-url "$EVIDRA_URL" --evidra-api-key "$EVIDRA_API_KEY" \
    2>&1 || echo "WARN: $model argocd batch exited $?"

  # Run Terraform scenarios (5 active, no cluster needed but uses default profile).
  "$BINARY" bench \
    --scenario terraform \
    --model "$model" --provider bifrost \
    --repeats "$REPEATS" \
    --environment "$ENVIRONMENT" --reuse-cluster --cluster-name "$cluster" \
    --proxy-mode \
    --evidra-url "$EVIDRA_URL" --evidra-api-key "$EVIDRA_API_KEY" \
    2>&1 || echo "WARN: $model terraform batch exited $?"

  echo ""
  echo "  $model complete."
}

# Run models sequentially — each gets its own cluster to avoid conflicts.
# ArgoCD scenarios use a separate cluster per model (different profile).
{
  run_model "gemini-2.5-flash" \
    "https://generativelanguage.googleapis.com/v1beta/openai" \
    "GEMINI_API_KEY" \
    "bench-gemini"

  run_model "deepseek-chat" \
    "https://api.deepseek.com/v1" \
    "DEEPSEEK_API_KEY" \
    "bench-deepseek"

  run_model "qwen-plus" \
    "https://dashscope-intl.aliyuncs.com/compatible-mode/v1" \
    "DASHSCOPE_API_KEY" \
    "bench-qwen"

  echo ""
  echo "════════════════════════════════════════"
  echo "  ALL MODELS COMPLETE"
  echo "════════════════════════════════════════"
  echo ""

  # Cleanup clusters.
  echo "Cleaning up clusters..."
  k3d cluster delete bench-gemini bench-gemini-argo \
    bench-deepseek bench-deepseek-argo \
    bench-qwen bench-qwen-argo 2>/dev/null || true

  echo "Done. Results reported to ${EVIDRA_URL}"
} 2>&1 | tee "$LOG"

echo "Log saved to: $LOG"
