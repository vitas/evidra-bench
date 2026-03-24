#!/bin/bash
# Run ArgoCD scenarios on a pre-provisioned cluster with ArgoCD installed.
# Usage: ./scripts/run-argocd-bench.sh
#
# This script:
# 1. Creates a dedicated kind cluster with ArgoCD pre-installed
# 2. Runs ArgoCD scenarios against it
# 3. Keeps the cluster for reuse (delete manually with: kind delete cluster --name infra-bench-argocd)
set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"
CLUSTER_NAME="infra-bench-argocd"
ARGOCD_VERSION="v2.13.3"

if [ -f "$PROJECT_DIR/.env" ]; then
  set -a
  source "$PROJECT_DIR/.env"
  set +a
fi

SCENARIOS=(
  "argocd/out-of-sync"
  "argocd/sync-wave-ordering"
  "argocd/argocd-sync-failure"
  "argocd/argocd-degraded-after-sync"
)

MODELS=(
  "gemini-2.5-flash"
  "claude-sonnet-4-20250514"
)

provider_url_for_model() {
  case "$1" in
    gpt-*) echo "https://api.openai.com/v1" ;;
    claude-*) echo "https://api.anthropic.com/v1" ;;
    gemini-*) echo "https://generativelanguage.googleapis.com/v1beta/openai" ;;
    deepseek-*) echo "https://api.deepseek.com/v1" ;;
    *) echo "${EVIDRA_BIFROST_BASE_URL:-http://localhost:8080/openai}" ;;
  esac
}

provider_key_for_model() {
  case "$1" in
    gpt-*) echo "${OPENAI_API_KEY:-}" ;;
    claude-*) echo "${ANTHROPIC_API_KEY:-}" ;;
    gemini-*) echo "${GEMINI_API_KEY:-}" ;;
    deepseek-*) echo "${DEEPSEEK_API_KEY:-}" ;;
    *) echo "${EVIDRA_BIFROST_AUTH_BEARER:-}" ;;
  esac
}

# Step 1: Ensure cluster with ArgoCD exists
if kind get clusters 2>/dev/null | grep -q "^${CLUSTER_NAME}$"; then
  echo "Cluster $CLUSTER_NAME already exists, reusing"
else
  echo "Creating cluster $CLUSTER_NAME..."
  kind create cluster --name "$CLUSTER_NAME" --wait 60s

  echo "Installing ArgoCD $ARGOCD_VERSION..."
  kubectl --context "kind-${CLUSTER_NAME}" create namespace argocd
  kubectl --context "kind-${CLUSTER_NAME}" apply -n argocd \
    -f "https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"

  echo "Waiting for ArgoCD components..."
  kubectl --context "kind-${CLUSTER_NAME}" wait --for=condition=Available \
    deployment/argocd-server -n argocd --timeout=300s
  kubectl --context "kind-${CLUSTER_NAME}" wait --for=condition=Available \
    deployment/argocd-repo-server -n argocd --timeout=300s
  kubectl --context "kind-${CLUSTER_NAME}" rollout status \
    statefulset/argocd-application-controller -n argocd --timeout=300s

  echo "ArgoCD ready on cluster $CLUSTER_NAME"
fi

# Step 2: Build
make -C "$PROJECT_DIR" build

# Step 3: Run scenarios
TOTAL=0; PASSED=0; FAILED=0; RESULTS=()

for MODEL in "${MODELS[@]}"; do
  export EVIDRA_BIFROST_BASE_URL=$(provider_url_for_model "$MODEL")
  export EVIDRA_BIFROST_AUTH_BEARER=$(provider_key_for_model "$MODEL")

  [ -z "$EVIDRA_BIFROST_AUTH_BEARER" ] && continue

  for SCENARIO in "${SCENARIOS[@]}"; do
    TOTAL=$((TOTAL + 1))
    echo ""
    echo "════ $SCENARIO / $MODEL ════"

    if bin/infra-bench run \
      --scenario "$SCENARIO" \
      --model "$MODEL" \
      --provider bifrost \
      --proxy-mode \
      --reuse-cluster \
      --cluster-name "$CLUSTER_NAME" \
      --timeout 10m \
      --evidra-url "https://api.evidra.cc" \
      --evidra-api-key "${EVIDRA_API_KEY:-REDACTED_KEY}" \
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
echo "  ARGOCD BENCHMARK: $PASSED/$TOTAL passed"
echo "════════════════════════════════════════"
for R in "${RESULTS[@]}"; do echo "  $R"; done
echo ""
echo "Cluster $CLUSTER_NAME kept for reuse. Delete with:"
echo "  kind delete cluster --name $CLUSTER_NAME"
