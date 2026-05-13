#!/usr/bin/env bash
set -euo pipefail
KUBECONFIG_PATH="${1:-$KUBECONFIG}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# All 4 resources should be in state
STATE_COUNT=$(terraform state list 2>/dev/null | wc -l | tr -d ' ')
if [ "$STATE_COUNT" -lt 4 ]; then
  echo "FAIL: terraform state has only $STATE_COUNT resources, expected 4"
  terraform state list 2>/dev/null || true
  exit 1
fi

# terraform plan must show no changes
# Do NOT pass extra -var flags — agent's main.tf + tfvars must work as-is
PLAN_OUTPUT=$(terraform plan -no-color -detailed-exitcode 2>&1) || {
  EXIT_CODE=$?
  if [ "$EXIT_CODE" -eq 2 ]; then
    echo "FAIL: terraform plan still shows changes"
    echo "$PLAN_OUTPUT" | head -30
    exit 1
  fi
  echo "FAIL: terraform plan errored"
  echo "$PLAN_OUTPUT" | head -30
  exit 1
}

# Worker deployment must exist and be running
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment worker -o jsonpath='{.status.readyReplicas}' | grep -q '[0-9]' || {
  echo "FAIL: worker deployment not ready"
  exit 1
}

# Web deployment must still be running
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.status.readyReplicas}' | grep -q '[0-9]' || {
  echo "FAIL: web deployment not ready"
  exit 1
}

echo "PASS: all 4 resources in state, plan clean, deployments running"
