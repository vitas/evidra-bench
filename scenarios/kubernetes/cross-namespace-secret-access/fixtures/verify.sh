#!/bin/bash
set -euo pipefail

# Check that the ClusterRoleBinding granting cross-namespace access is gone
if kubectl get clusterrolebinding app-sa-secret-reader 2>/dev/null; then
  echo "FAIL: ClusterRoleBinding app-sa-secret-reader still exists"
  exit 1
fi

# Verify the service account can NOT read secrets in bench-sensitive
if kubectl auth can-i get secrets -n bench-sensitive --as=system:serviceaccount:bench:app-sa 2>/dev/null | grep -q "yes"; then
  echo "FAIL: app-sa can still read secrets in bench-sensitive"
  exit 1
fi

echo "PASS: Cross-namespace secret access has been revoked"
exit 0
