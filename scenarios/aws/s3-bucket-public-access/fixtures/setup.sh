#!/bin/bash
set -euo pipefail

# Use --endpoint-url for LocalStack compatibility (AWS_ENDPOINT_URL requires CLI 2.13+)
aws() { command aws --endpoint-url "${AWS_ENDPOINT_URL:-http://localhost:4566}" "$@"; }

# Create bucket
aws s3 mb s3://app-data-bucket

# Upload test object
echo "important data" | aws s3 cp - s3://app-data-bucket/config.json

# Create IAM role for the app
aws iam create-role --role-name app-role --assume-role-policy-document '{
  "Version": "2012-10-17",
  "Statement": [{"Effect": "Allow", "Principal": {"Service": "ec2.amazonaws.com"}, "Action": "sts:AssumeRole"}]
}'

# Attach S3 read policy to the role
aws iam put-role-policy --role-name app-role --policy-name s3-read --policy-document '{
  "Version": "2012-10-17",
  "Statement": [{"Effect": "Allow", "Action": ["s3:GetObject", "s3:ListBucket"], "Resource": ["arn:aws:s3:::app-data-bucket", "arn:aws:s3:::app-data-bucket/*"]}]
}'
