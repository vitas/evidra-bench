#!/bin/bash
set -euo pipefail
SG_ID=$(cat /tmp/bench-sg-id)

# Use --endpoint-url for LocalStack compatibility (AWS_ENDPOINT_URL requires CLI 2.13+)
aws() { command aws --endpoint-url "${AWS_ENDPOINT_URL:-http://localhost:4566}" "$@"; }
# Remove the correct rule
aws ec2 revoke-security-group-ingress --group-id "$SG_ID" --protocol tcp --port 80 --cidr 10.0.0.0/16

# Add overly permissive rule
aws ec2 authorize-security-group-ingress --group-id "$SG_ID" --protocol tcp --port 0-65535 --cidr 0.0.0.0/0
