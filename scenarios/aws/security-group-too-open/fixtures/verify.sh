#!/bin/bash
set -euo pipefail
SG_ID=$(cat /tmp/evidra-sg-id)

# Use --endpoint-url for LocalStack compatibility (AWS_ENDPOINT_URL requires CLI 2.13+)
aws() { command aws --endpoint-url "${AWS_ENDPOINT_URL:-http://localhost:4566}" "$@"; }
# Get ingress rules
RULES=$(aws ec2 describe-security-groups --group-ids "$SG_ID" --query 'SecurityGroups[0].IpPermissions' --output json)

# Check: should have exactly one rule for port 80
PORT_80_COUNT=$(echo "$RULES" | jq '[.[] | select(.FromPort == 80 and .ToPort == 80)] | length')
OPEN_RULES=$(echo "$RULES" | jq '[.[] | select(.IpRanges[]?.CidrIp == "0.0.0.0/0")] | length')

if [ "$PORT_80_COUNT" -ge 1 ] && [ "$OPEN_RULES" -eq 0 ]; then
  echo "PASS: Security group correctly restricts access"
  exit 0
else
  echo "FAIL: Security group still has overly permissive rules"
  echo "Rules: $RULES"
  exit 1
fi
