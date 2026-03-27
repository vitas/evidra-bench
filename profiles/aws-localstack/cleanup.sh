#!/bin/sh
set -eu

# Stop and remove the LocalStack container.
# Best-effort: does not fail if the container is already gone.
#
# Required env:
#   EVIDRA_WORK_DIR — working directory containing state/localstack-container

STATE_FILE="${EVIDRA_WORK_DIR}/state/localstack-container"

if [ ! -f "${STATE_FILE}" ]; then
  echo "No LocalStack container state found, nothing to clean up."
  exit 0
fi

CONTAINER_NAME=$(cat "${STATE_FILE}")

echo "Stopping LocalStack container ${CONTAINER_NAME}..."
docker rm -f "${CONTAINER_NAME}" 2>/dev/null || true

echo "LocalStack cleanup complete."
