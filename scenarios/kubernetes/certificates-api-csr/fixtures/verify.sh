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

# Check that Role has correct verbs (get, list, watch on pods)
ROLE_VERBS=$(kubectl get role pod-reader -n bench -o jsonpath='{range .rules[0].verbs[*]}{@}{"\n"}{end}' | sort)
for VERB in get list watch; do
  if ! echo "$ROLE_VERBS" | grep -qx "$VERB"; then
    echo "ERROR: Role missing verb '$VERB'. Has: $(echo $ROLE_VERBS | tr '\n' ' ')"
    exit 1
  fi
done
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
