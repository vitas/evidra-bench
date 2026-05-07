#!/bin/sh
set -eu

# Verify LocalStack is healthy.
#
# Required env:
#   BENCH_WORK_DIR — working directory containing lease.env

LEASE_ENV="${BENCH_WORK_DIR}/lease.env"

if [ ! -f "${LEASE_ENV}" ]; then
  echo "FAIL: lease.env not found at ${LEASE_ENV}"
  exit 1
fi

# Read the endpoint from lease.env.
ENDPOINT=$(grep '^AWS_ENDPOINT_URL=' "${LEASE_ENV}" | cut -d= -f2-)

if [ -z "${ENDPOINT}" ]; then
  echo "FAIL: AWS_ENDPOINT_URL not found in lease.env"
  exit 1
fi

echo "Checking LocalStack health at ${ENDPOINT}..."

STATUS=$(curl -sf "${ENDPOINT}/_localstack/health" 2>/dev/null) || {
  echo "FAIL: LocalStack health endpoint unreachable"
  exit 1
}

echo "LocalStack is healthy: ${STATUS}"
