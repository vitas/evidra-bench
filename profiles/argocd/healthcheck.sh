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
for deploy in argocd-server argocd-repo-server; do
  available=$(kubectl --kubeconfig="${KUBECONFIG}" -n "${ARGOCD_NAMESPACE}" \
    get deployment/"${deploy}" -o jsonpath='{.status.availableReplicas}' 2>/dev/null || echo "0")
  if [ "${available:-0}" -lt 1 ]; then
    echo "FAIL: ${deploy} has no available replicas"
    exit 1
  fi
done

# application-controller is a StatefulSet.
ready=$(kubectl --kubeconfig="${KUBECONFIG}" -n "${ARGOCD_NAMESPACE}" \
  get statefulset/argocd-application-controller -o jsonpath='{.status.readyReplicas}' 2>/dev/null || echo "0")
if [ "${ready:-0}" -lt 1 ]; then
  echo "FAIL: argocd-application-controller has no ready replicas"
  exit 1
fi

echo "ArgoCD is healthy."
