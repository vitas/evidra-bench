#!/bin/sh
set -eu

# Install ArgoCD v2.13.3 into the cluster.
# Idempotent: safe to run multiple times.
#
# Required env:
#   KUBECONFIG          — path to the cluster kubeconfig
#   EVIDRA_CLUSTER_NAME — name of the cluster (informational)
#
# Optional env:
#   EVIDRA_WORK_DIR     — working directory for state files
#   EVIDRA_ASSETS_DIR   — path to the profile assets directory

ARGOCD_VERSION="v2.13.3"
ARGOCD_NAMESPACE="argocd"
ARGOCD_MANIFEST_URL="https://raw.githubusercontent.com/argoproj/argo-cd/${ARGOCD_VERSION}/manifests/install.yaml"

echo "Installing ArgoCD ${ARGOCD_VERSION} into cluster ${EVIDRA_CLUSTER_NAME:-unknown}..."

# Create namespace (idempotent).
kubectl --kubeconfig="${KUBECONFIG}" create namespace "${ARGOCD_NAMESPACE}" --dry-run=client -o yaml \
  | kubectl --kubeconfig="${KUBECONFIG}" apply -f -

# Apply ArgoCD manifests from upstream.
kubectl --kubeconfig="${KUBECONFIG}" apply -n "${ARGOCD_NAMESPACE}" -f "${ARGOCD_MANIFEST_URL}"

# Wait for the ArgoCD CRDs to be established.
echo "Waiting for ArgoCD CRDs..."
kubectl --kubeconfig="${KUBECONFIG}" wait --for=condition=Established \
  crd/applications.argoproj.io --timeout=60s

# Wait for core components to be available.
for deploy in argocd-server argocd-repo-server; do
  echo "Waiting for deployment/${deploy}..."
  kubectl --kubeconfig="${KUBECONFIG}" -n "${ARGOCD_NAMESPACE}" \
    rollout status deployment/"${deploy}" --timeout=300s
done

# application-controller is a StatefulSet, not a Deployment.
echo "Waiting for statefulset/argocd-application-controller..."
kubectl --kubeconfig="${KUBECONFIG}" -n "${ARGOCD_NAMESPACE}" \
  rollout status statefulset/argocd-application-controller --timeout=300s

echo "ArgoCD ${ARGOCD_VERSION} installed successfully."
