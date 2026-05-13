#!/bin/bash
set -euo pipefail

# Check that the plaintext password is no longer in the ConfigMap
CM=$(kubectl get configmap migration-config -n bench -o json 2>/dev/null || echo '{}')
if echo "$CM" | grep -q "correcthorse"; then
  echo "FAIL: ConfigMap migration-config still contains plaintext password"
  exit 1
fi

# Check that the Job with plaintext password has been cleaned up or fixed
JOB=$(kubectl get job db-migrate -n bench -o json 2>/dev/null || echo '{}')
if echo "$JOB" | grep -q "correcthorse"; then
  echo "FAIL: Job db-migrate still contains plaintext password in command args"
  exit 1
fi

echo "PASS: No plaintext credential exposure found"
exit 0
