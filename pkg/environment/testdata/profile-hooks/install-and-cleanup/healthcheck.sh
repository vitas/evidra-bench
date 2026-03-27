#!/bin/sh
set -eu

# Verify the marker file exists (install ran first).
if [ ! -f "${EVIDRA_WORK_DIR}/marker.env" ]; then
  echo "healthcheck: marker.env not found" >&2
  exit 1
fi

echo "healthcheck passed"
