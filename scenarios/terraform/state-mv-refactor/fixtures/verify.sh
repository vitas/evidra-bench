#!/usr/bin/env bash
set -euo pipefail
KUBECONFIG_PATH="${1:-$KUBECONFIG}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# State should have module-prefixed addresses
STATE_LIST=$(terraform state list 2>/dev/null)
for ADDR in "module.app.kubernetes_deployment_v1.web" "module.app.kubernetes_service_v1.web" \
            "module.db.kubernetes_deployment_v1.db" "module.db.kubernetes_config_map_v1.db_config"; do
  if ! echo "$STATE_LIST" | grep -q "$ADDR"; then
    echo "FAIL: missing expected state address: $ADDR"
    echo "Current state:"
    echo "$STATE_LIST"
    exit 1
  fi
done

# No old (non-module) addresses should remain
if echo "$STATE_LIST" | grep -v "^module\." | grep -q "kubernetes_"; then
  echo "FAIL: old non-module addresses still in state"
  echo "$STATE_LIST"
  exit 1
fi

# terraform plan must show no changes
# Do NOT pass extra -var flags — agent's main.tf + tfvars must work as-is
PLAN_OUTPUT=$(terraform plan -no-color -detailed-exitcode 2>&1) || {
  EXIT_CODE=$?
  if [ "$EXIT_CODE" -eq 2 ]; then
    echo "FAIL: terraform plan still shows changes"
    echo "$PLAN_OUTPUT" | head -40
    exit 1
  fi
  echo "FAIL: terraform plan errored"
  echo "$PLAN_OUTPUT" | head -30
  exit 1
}

# Resources still running
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.status.readyReplicas}' | grep -q '[0-9]' || {
  echo "FAIL: web deployment not ready"
  exit 1
}
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment db -o jsonpath='{.status.readyReplicas}' | grep -q '[0-9]' || {
  echo "FAIL: db deployment not ready"
  exit 1
}

echo "PASS: all resources moved to module addresses, plan clean, deployments running"
