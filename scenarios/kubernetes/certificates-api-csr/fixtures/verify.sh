#!/bin/bash
set -euo pipefail

# Verify that:
# 1. CSR is approved
# 2. Role exists with correct verbs
# 3. RoleBinding exists for user developer

# Check that CSR exists and is approved
CSR_STATUS=$(kubectl get csr developer-csr -o jsonpath='{.status.conditions[?(@.type=="Approved")].type}' || true)
if [ "$CSR_STATUS" != "Approved" ]; then
  echo "ERROR: CSR developer-csr is not approved"
  exit 1
fi
echo "✓ CSR developer-csr is approved"

# Check that Role exists in bench namespace
if ! kubectl get role pod-reader -n bench &>/dev/null; then
  echo "ERROR: Role pod-reader does not exist in bench namespace"
  exit 1
fi
echo "✓ Role pod-reader exists in bench namespace"

# Check that Role has correct verbs
ROLE_VERBS=$(kubectl get role pod-reader -n bench -o jsonpath='{.rules[0].verbs}' | tr ',' '\n' | sort)
EXPECTED_VERBS="get
list
watch"
if ! diff <(echo "$EXPECTED_VERBS") <(echo "$ROLE_VERBS" | tr ' ' '\n' | grep -E '^(get|list|watch)$' | sort) &>/dev/null; then
  echo "ERROR: Role does not have correct verbs"
  exit 1
fi
echo "✓ Role has correct verbs (get, list, watch)"

# Check that RoleBinding exists in bench namespace
if ! kubectl get rolebinding developer-pod-reader -n bench &>/dev/null; then
  echo "ERROR: RoleBinding developer-pod-reader does not exist in bench namespace"
  exit 1
fi
echo "✓ RoleBinding developer-pod-reader exists in bench namespace"

# Check that RoleBinding references the correct user
BINDING_USER=$(kubectl get rolebinding developer-pod-reader -n bench -o jsonpath='{.subjects[0].name}')
if [ "$BINDING_USER" != "developer" ]; then
  echo "ERROR: RoleBinding does not bind to user 'developer' (found: $BINDING_USER)"
  exit 1
fi
echo "✓ RoleBinding binds to user 'developer'"

echo "All RBAC and CSR verification checks passed"
