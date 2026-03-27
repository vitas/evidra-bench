#!/bin/sh
set -eu

# Verify ArgoCD is healthy and ready to accept Applications.
#
# Required env:
#   KUBECONFIG — path to the cluster kubeconfig

ARGOCD_NAMESPACE="argocd"

echo "Checking ArgoCD health..."

# Verify the CRD exists.
kubectl --kubeconfig="${KUBECONFIG}" get crd applications.argoproj.io >/dev/null

# Verify core deployments are available.
for deploy in argocd-server argocd-repo-server argocd-application-controller; do
  available=$(kubectl --kubeconfig="${KUBECONFIG}" -n "${ARGOCD_NAMESPACE}" \
    get deployment/"${deploy}" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
  if [ "${available:-0}" -lt 1 ]; then
    echo "FAIL: ${deploy} has no available replicas"
    exit 1
  fi
done

echo "ArgoCD is healthy."
