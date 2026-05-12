#!/usr/bin/env bash
set -euo pipefail

KUBECONFIG_PATH="${KUBECONFIG:?KUBECONFIG is required}"

status_json="$(helm --kubeconfig "$KUBECONFIG_PATH" -n bench status web -o json)"
if ! printf '%s' "$status_json" | grep -Eq '"status"[[:space:]]*:[[:space:]]*"deployed"'; then
  echo "FAIL: helm release is not deployed"
  helm --kubeconfig "$KUBECONFIG_PATH" -n bench status web || true
  exit 1
fi

revision_count="$(helm --kubeconfig "$KUBECONFIG_PATH" -n bench history web -o json | grep -o '"revision"' | wc -l | tr -d ' ')"
if [ "$revision_count" -lt 1 ]; then
  echo "FAIL: helm history is missing"
  exit 1
fi

ready="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.status.readyReplicas}')"
desired="$(kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench get deployment web -o jsonpath='{.spec.replicas}')"
if [ -z "$ready" ] || [ "$ready" != "$desired" ]; then
  echo "FAIL: web deployment ready replicas $ready do not match desired $desired"
  exit 1
fi

echo "PASS: helm release recovered with deployed history"
