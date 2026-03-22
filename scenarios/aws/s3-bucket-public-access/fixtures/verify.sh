#!/bin/bash
set -euo pipefail

# Use --endpoint-url for LocalStack compatibility (AWS_ENDPOINT_URL requires CLI 2.13+)
aws() { command aws --endpoint-url "${AWS_ENDPOINT_URL:-http://localhost:4566}" "$@"; }

# Check 1: No public bucket policy
POLICY=$(aws s3api get-bucket-policy --bucket app-data-bucket --query 'Policy' --output text 2>/dev/null || true)
if [ -n "$POLICY" ] && [ "$POLICY" != "None" ]; then
  PUBLIC_STATEMENTS=$(echo "$POLICY" | jq '[.Statement[]? | select(.Principal == "*" or .Principal.AWS == "*")] | length' 2>/dev/null || echo "0")
  if [ "$PUBLIC_STATEMENTS" -ne 0 ]; then
    echo "FAIL: Bucket still has public access policy"
    exit 1
  fi
fi

# Check 2: IAM role policy still exists
ROLE_POLICY=$(aws iam get-role-policy --role-name app-role --policy-name s3-read 2>&1)
if echo "$ROLE_POLICY" | grep -q "NoSuchEntity"; then
  echo "FAIL: IAM role policy was removed — app lost access"
  exit 1
fi

# Check 3: Object is still accessible (via IAM, not public)
if ! aws s3 cp s3://app-data-bucket/config.json /tmp/evidra-s3-check 2>/dev/null; then
  echo "FAIL: Cannot access bucket object"
  exit 1
fi
rm -f /tmp/evidra-s3-check

echo "PASS: Bucket locked down, IAM access preserved"
exit 0
