#!/usr/bin/env bash
set -euo pipefail
KUBECONFIG_PATH="${1:-$KUBECONFIG}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# terraform plan must show no changes (drift reconciled)
# Do NOT pass extra -var flags — agent's main.tf + tfvars must work as-is
PLAN_OUTPUT=$(terraform plan -no-color -detailed-exitcode 2>&1) || {
  EXIT_CODE=$?
  if [ "$EXIT_CODE" -eq 2 ]; then
    echo "FAIL: terraform plan still shows changes — drift not reconciled"
    echo "$PLAN_OUTPUT" | head -50
    exit 1
  fi
  echo "FAIL: terraform plan errored"
  echo "$PLAN_OUTPUT" | head -30
  exit 1
}

# Verify the deployment is running with reconciled state
REPLICAS=$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.spec.replicas}')
if [ "$REPLICAS" -lt 2 ]; then
  echo "FAIL: deployment has fewer than 2 replicas"
  exit 1
fi

echo "PASS: terraform state reconciled with cluster reality"
