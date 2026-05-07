#!/usr/bin/env bash
# Run full scenario matrix with a primary provider, then retry ERRORs with a fallback.
# Usage:
#   ./scripts/run-matrix-with-fallback.sh
#
# Environment variables:
#   PRIMARY_PROVIDER    Primary provider (default: claude)
#   PRIMARY_MODEL       Primary model (default: sonnet)
#   FALLBACK_PROVIDER   Fallback provider for ERROR retries (default: anthropic)
#   FALLBACK_MODEL      Fallback model (default: claude-sonnet-4-20250514)
#   ANTHROPIC_API_KEY   Required for anthropic fallback
#   INFRA_BENCH_BIFROST_URL / EVIDRA_BIFROST_AUTH_BEARER  Required for bifrost
#   CLUSTER_NAME        Kind cluster name (default: evidra)
#   RUNS_DIR            Output directory (default: runs/matrix-fallback-<timestamp>)
set -uo pipefail

PRIMARY_PROVIDER="${PRIMARY_PROVIDER:-claude}"
PRIMARY_MODEL="${PRIMARY_MODEL:-sonnet}"
FALLBACK_PROVIDER="${FALLBACK_PROVIDER:-anthropic}"
FALLBACK_MODEL="${FALLBACK_MODEL:-claude-sonnet-4-20250514}"
CLUSTER_NAME="${CLUSTER_NAME:-evidra}"
KUBECONFIG_PATH="${KUBECONFIG_PATH:-/tmp/kind-evidra.kubeconfig}"
RUNS_DIR="${RUNS_DIR:-runs/matrix-fallback-$(date +%Y%m%d-%H%M%S)}"

mkdir -p "$RUNS_DIR"

PASS=0; FAIL=0; ERROR=0; RETRIED=0; RETRY_PASS=0

declare -a ERROR_SCENARIOS=()

echo "=== Primary: $PRIMARY_PROVIDER/$PRIMARY_MODEL ==="

while IFS= read -r scenario; do
  case "$scenario" in *resource-quota-exceeded*) continue ;; esac

  kubectl --kubeconfig "$KUBECONFIG_PATH" delete deployment,svc,configmap,networkpolicy,pdb,secret,job,resourcequota,ingress --all -n bench --ignore-not-found 2>/dev/null
  kubectl --kubeconfig "$KUBECONFIG_PATH" delete ns bench-staging --ignore-not-found 2>/dev/null
  sleep 2

  RESULT=$(bin/bench-cli run \
    --scenario "$scenario" \
    --provider "$PRIMARY_PROVIDER" \
    --model "$PRIMARY_MODEL" \
    --cluster-name "$CLUSTER_NAME" \
    --reuse-cluster \
    --timeout 5m \
    --runs-dir "$RUNS_DIR/primary" 2>&1)

  if echo "$RESULT" | grep -q "^\[PASS\]"; then
    PASS=$((PASS + 1))
    echo "  $scenario: PASS"
  elif echo "$RESULT" | grep -q "^\[FAIL\]"; then
    FAIL=$((FAIL + 1))
    echo "  $scenario: FAIL"
  else
    ERROR=$((ERROR + 1))
    ERROR_SCENARIOS+=("$scenario")
    echo "  $scenario: ERROR (will retry with $FALLBACK_PROVIDER)"
  fi
done < <(bin/bench-cli scenario list 2>&1 | awk '{print $1}')

echo ""
echo "=== Primary results: PASS=$PASS FAIL=$FAIL ERROR=$ERROR ==="

if [ ${#ERROR_SCENARIOS[@]} -gt 0 ]; then
  echo ""
  echo "=== Retrying ${#ERROR_SCENARIOS[@]} errors with $FALLBACK_PROVIDER/$FALLBACK_MODEL ==="

  for scenario in "${ERROR_SCENARIOS[@]}"; do
    # Skip ArgoCD errors (infra, not model)
    case "$scenario" in argocd/*) echo "  $scenario: SKIP (ArgoCD infra issue)"; continue ;; esac

    kubectl --kubeconfig "$KUBECONFIG_PATH" delete deployment,svc,configmap,networkpolicy,pdb,secret,job,resourcequota,ingress --all -n bench --ignore-not-found 2>/dev/null
    kubectl --kubeconfig "$KUBECONFIG_PATH" delete ns bench-staging --ignore-not-found 2>/dev/null
    sleep 2

    RETRIED=$((RETRIED + 1))
    RESULT=$(bin/bench-cli run \
      --scenario "$scenario" \
      --provider "$FALLBACK_PROVIDER" \
      --model "$FALLBACK_MODEL" \
      --cluster-name "$CLUSTER_NAME" \
      --reuse-cluster \
      --timeout 5m \
      --runs-dir "$RUNS_DIR/fallback" 2>&1)

    if echo "$RESULT" | grep -q "^\[PASS\]"; then
      RETRY_PASS=$((RETRY_PASS + 1))
      echo "  $scenario: PASS (fallback)"
    elif echo "$RESULT" | grep -q "^\[FAIL\]"; then
      echo "  $scenario: FAIL (fallback)"
    else
      echo "  $scenario: ERROR (fallback too)"
    fi
  done
fi

echo ""
echo "=== Final Summary ==="
echo "  Primary ($PRIMARY_PROVIDER/$PRIMARY_MODEL): PASS=$PASS FAIL=$FAIL ERROR=$ERROR"
echo "  Fallback retries: $RETRIED attempted, $RETRY_PASS recovered"
echo "  Effective pass: $((PASS + RETRY_PASS))"
echo "  Results: $RUNS_DIR"
