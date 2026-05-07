#!/bin/sh
set -eu

# Write a cleanup marker so the test can verify cleanup ran.
echo "cleaned" > "${BENCH_WORK_DIR}/cleanup.marker"

echo "cleanup done"
