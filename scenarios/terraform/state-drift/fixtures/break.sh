#!/usr/bin/env bash
# Break: simulate manual kubectl changes that cause drift
set -euo pipefail
KUBECONFIG_PATH="${1:?kubeconfig path required}"

# Scale deployment from 2 to 5 replicas (intentional scale-up by ops team)
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench scale deployment web --replicas=5

# Add an urgent hotfix label
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench label deployment web hotfix=CVE-2024-1234 --overwrite

# Update configmap with production-critical change
kubectl --kubeconfig "$KUBECONFIG_PATH" -n bench create configmap app-config \
  --from-literal=ENV=production \
  --from-literal=LOG_LEVEL=debug \
  --from-literal=FEATURE_FLAG=enabled \
  --dry-run=client -o yaml | kubectl --kubeconfig "$KUBECONFIG_PATH" apply -f -

echo "Manual changes applied — terraform state is now drifted"
