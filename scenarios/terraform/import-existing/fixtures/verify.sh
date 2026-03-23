#!/usr/bin/env bash
set -euo pipefail
KUBECONFIG_PATH="${1:-$KUBECONFIG}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Check that terraform state has all 3 resources
STATE_COUNT=$(terraform state list 2>/dev/null | wc -l | tr -d ' ')
if [ "$STATE_COUNT" -lt 3 ]; then
  echo "FAIL: terraform state has only $STATE_COUNT resources, expected at least 3"
  terraform state list 2>/dev/null || true
  exit 1
fi

# terraform plan must show NO changes (zero diff = clean import)
# Do NOT pass extra -var flags — the agent's main.tf must work as-is
PLAN_OUTPUT=$(terraform plan -no-color -detailed-exitcode 2>&1) || {
  EXIT_CODE=$?
  if [ "$EXIT_CODE" -eq 2 ]; then
    echo "FAIL: terraform plan shows changes — import incomplete or HCL doesn't match"
    echo "$PLAN_OUTPUT" | head -50
    exit 1
  fi
  echo "FAIL: terraform plan errored"
  echo "$PLAN_OUTPUT" | head -30
  exit 1
}

# Verify resources still running
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment api -o jsonpath='{.status.readyReplicas}' | grep -q '[0-9]' || {
  echo "FAIL: api deployment not running"
  exit 1
}

echo "PASS: all resources imported, terraform plan clean, deployment running"
