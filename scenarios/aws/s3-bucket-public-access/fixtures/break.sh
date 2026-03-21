#!/bin/bash
set -euo pipefail

# Add public read access via bucket policy
aws s3api put-bucket-policy --bucket app-data-bucket --policy '{
  "Version": "2012-10-17",
  "Statement": [
    {
      "Sid": "PublicRead",
      "Effect": "Allow",
      "Principal": "*",
      "Action": "s3:GetObject",
      "Resource": "arn:aws:s3:::app-data-bucket/*"
    }
  ]
}'
