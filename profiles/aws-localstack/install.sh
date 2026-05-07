#!/bin/sh
set -eu

# Start a LocalStack container and create an AWS CLI wrapper.
# Idempotent: stops any existing container before starting.
#
# Required env:
#   KUBECONFIG          — path to the cluster kubeconfig
#   BENCH_CLUSTER_NAME — name of the cluster
#   BENCH_WORK_DIR     — working directory for state and binaries
#
# Optional env:
#   BENCH_LOCALSTACK_SERVICES — comma-separated list of AWS services (default: s3,iam,sts)
#   BENCH_ASSETS_DIR          — path to the profile assets directory

LOCALSTACK_IMAGE="localstack/localstack:3.8"
LOCALSTACK_SERVICES="${BENCH_LOCALSTACK_SERVICES:-s3,iam,sts}"
CONTAINER_NAME="bench-localstack-${BENCH_CLUSTER_NAME}"
LOCALSTACK_PORT=4566

STATE_DIR="${BENCH_WORK_DIR}/state"
BIN_DIR="${BENCH_WORK_DIR}/bin"

mkdir -p "${STATE_DIR}" "${BIN_DIR}"

# Stop any existing container (idempotent).
docker rm -f "${CONTAINER_NAME}" 2>/dev/null || true

echo "Starting LocalStack (services: ${LOCALSTACK_SERVICES})..."

docker run -d \
  --name "${CONTAINER_NAME}" \
  -p "${LOCALSTACK_PORT}:4566" \
  -e "SERVICES=${LOCALSTACK_SERVICES}" \
  -e "EAGER_SERVICE_LOADING=1" \
  "${LOCALSTACK_IMAGE}"

# Record container name for cleanup.
echo "${CONTAINER_NAME}" > "${STATE_DIR}/localstack-container"

# Determine the LocalStack endpoint.
# Use host.docker.internal on macOS/Windows, localhost on Linux.
LOCALSTACK_HOST="localhost"
LOCALSTACK_ENDPOINT="http://${LOCALSTACK_HOST}:${LOCALSTACK_PORT}"

# Wait for LocalStack to be ready.
echo "Waiting for LocalStack to be ready..."
retries=30
while [ "${retries}" -gt 0 ]; do
  if curl -sf "${LOCALSTACK_ENDPOINT}/_localstack/health" >/dev/null 2>&1; then
    break
  fi
  retries=$((retries - 1))
  sleep 2
done

if [ "${retries}" -eq 0 ]; then
  echo "FAIL: LocalStack did not become ready"
  exit 1
fi

echo "LocalStack is ready at ${LOCALSTACK_ENDPOINT}"

# Find the real aws binary before we prepend our wrapper dir to PATH.
REAL_AWS="$(command -v aws 2>/dev/null || echo /usr/local/bin/aws)"

# Create the AWS CLI wrapper that delegates to the real binary.
cat > "${BIN_DIR}/aws" <<WRAPPER
#!/bin/sh
exec "${REAL_AWS}" --endpoint-url "${LOCALSTACK_ENDPOINT}" "\$@"
WRAPPER
chmod +x "${BIN_DIR}/aws"

# Build the concrete PATH with our wrapper dir prepended.
CONCRETE_PATH="${BIN_DIR}:${PATH}"

# Write lease.env with concrete values only — no shell variable placeholders.
cat > "${BENCH_WORK_DIR}/lease.env" <<EOF
AWS_ENDPOINT_URL=${LOCALSTACK_ENDPOINT}
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
AWS_DEFAULT_REGION=us-east-1
PATH=${CONCRETE_PATH}
LOCALSTACK_CONTAINER=${CONTAINER_NAME}
EOF

echo "LocalStack install complete. lease.env written to ${BENCH_WORK_DIR}/lease.env"
