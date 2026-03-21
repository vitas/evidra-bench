#!/bin/bash
set -euo pipefail
SG_ID=$(cat /tmp/evidra-sg-id)

# Remove the correct rule
aws ec2 revoke-security-group-ingress --group-id "$SG_ID" --protocol tcp --port 80 --cidr 10.0.0.0/16

# Add overly permissive rule
aws ec2 authorize-security-group-ingress --group-id "$SG_ID" --protocol tcp --port 0-65535 --cidr 0.0.0.0/0
