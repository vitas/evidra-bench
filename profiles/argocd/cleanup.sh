#!/bin/sh
set -eu

# Best-effort cleanup of ArgoCD Application objects.
# Does NOT remove the ArgoCD controller itself — that is destroyed
# with the cluster.
#
# Required env:
#   KUBECONFIG — path to the cluster kubeconfig

ARGOCD_NAMESPACE="argocd"

echo "Cleaning up ArgoCD Applications..."

# Delete all Application objects (best-effort).
kubectl --kubeconfig="${KUBECONFIG}" -n "${ARGOCD_NAMESPACE}" \
  delete applications.argoproj.io --all --timeout=30s 2>/dev/null || true

# Delete all AppProject objects except 'default' (best-effort).
kubectl --kubeconfig="${KUBECONFIG}" -n "${ARGOCD_NAMESPACE}" \
  delete appprojects.argoproj.io --field-selector='metadata.name!=default' --timeout=30s 2>/dev/null || true

echo "ArgoCD cleanup complete."
