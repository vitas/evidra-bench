#!/bin/bash
# Verify that a Falco rule exists for detecting sensitive file access.
set -euo pipefail
KUBECONFIG="${1:-$KUBECONFIG}"

# Check that Falco has custom rules loaded (via ConfigMap or rules file).
# Look for rules mentioning /etc/shadow or /etc/passwd.
RULES=$(kubectl --kubeconfig "$KUBECONFIG" exec -n falco \
  "$(kubectl --kubeconfig "$KUBECONFIG" get pod -n falco -l app.kubernetes.io/name=falco -o name | head -1)" \
  -- cat /etc/falco/falco_rules.local.yaml 2>/dev/null || true)

# Also check ConfigMap-based rules
CM_RULES=$(kubectl --kubeconfig "$KUBECONFIG" get configmap -n falco -o yaml 2>/dev/null | grep -c "shadow\|passwd" || true)

if echo "$RULES" | grep -qi "shadow\|passwd"; then
  echo "PASS: Falco custom rule found for sensitive file access"
  exit 0
fi

if [[ "$CM_RULES" -gt 0 ]]; then
  echo "PASS: Falco ConfigMap rules reference sensitive files"
  exit 0
fi

echo "FAIL: No Falco rule found for /etc/shadow or /etc/passwd detection"
exit 1
