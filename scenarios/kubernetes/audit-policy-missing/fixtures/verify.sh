#!/bin/bash
# Verify the API server has an audit policy configured and audit logs exist.
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Get the control-plane node name (Kind convention: <cluster>-control-plane)
CLUSTER_NAME=$(kubectl --kubeconfig "$KUBECONFIG" config view -o jsonpath='{.clusters[0].name}' | sed 's/^kind-//')
NODE="${CLUSTER_NAME}-control-plane"

# Check 1: API server manifest has --audit-policy-file flag
POLICY_FLAG=$(docker exec "$NODE" cat /etc/kubernetes/manifests/kube-apiserver.yaml 2>/dev/null | grep -c "audit-policy-file" || echo "0")
if [[ "$POLICY_FLAG" -eq 0 ]]; then
  echo "FAIL: API server manifest does not contain --audit-policy-file"
  exit 1
fi

# Check 2: The audit policy file actually exists on the node
POLICY_EXISTS=$(docker exec "$NODE" test -f /etc/kubernetes/audit/policy.yaml && echo "yes" || echo "no")
if [[ "$POLICY_EXISTS" != "yes" ]]; then
  # Try common alternative path
  POLICY_EXISTS=$(docker exec "$NODE" test -f /etc/kubernetes/audit-policy.yaml && echo "yes" || echo "no")
  if [[ "$POLICY_EXISTS" != "yes" ]]; then
    echo "FAIL: Audit policy file not found on control-plane node"
    exit 1
  fi
fi

# Check 3: API server is running and cluster is healthy
kubectl --kubeconfig "$KUBECONFIG" get nodes >/dev/null 2>&1
if [[ $? -ne 0 ]]; then
  echo "FAIL: Cannot reach API server — cluster may be unhealthy after manifest change"
  exit 1
fi

# Check 4: Audit log path is configured
LOG_FLAG=$(docker exec "$NODE" cat /etc/kubernetes/manifests/kube-apiserver.yaml 2>/dev/null | grep -c "audit-log-path" || echo "0")
if [[ "$LOG_FLAG" -eq 0 ]]; then
  echo "FAIL: API server manifest does not contain --audit-log-path"
  exit 1
fi

echo "PASS: Audit policy is configured and API server is healthy"
