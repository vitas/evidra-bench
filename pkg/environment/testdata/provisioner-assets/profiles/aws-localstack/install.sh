#!/bin/sh
set -eu
# Write lease.env with AWS credentials (simulating real LocalStack startup).
cat > "${BENCH_WORK_DIR}/lease.env" <<ENVEOF
AWS_ENDPOINT_URL=http://localhost:4566
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
AWS_DEFAULT_REGION=us-east-1
ENVEOF

# Record services for verification.
echo "SERVICES=${BENCH_LOCALSTACK_SERVICES:-}" > "${BENCH_WORK_DIR}/marker.env"
