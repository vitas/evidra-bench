#!/bin/sh
set -eu

# Prove env propagation by writing env vars to a marker file.
echo "KUBECONFIG=${KUBECONFIG}" > "${BENCH_WORK_DIR}/marker.env"
echo "BENCH_PROFILE=${BENCH_PROFILE}" >> "${BENCH_WORK_DIR}/marker.env"
echo "BENCH_PROVIDER=${BENCH_PROVIDER}" >> "${BENCH_WORK_DIR}/marker.env"
echo "BENCH_CLUSTER_NAME=${BENCH_CLUSTER_NAME}" >> "${BENCH_WORK_DIR}/marker.env"
echo "BENCH_ASSETS_DIR=${BENCH_ASSETS_DIR}" >> "${BENCH_WORK_DIR}/marker.env"

# Write a lease.env that downstream code should parse.
echo "LOCALSTACK_ENDPOINT=http://localhost:4566" > "${BENCH_WORK_DIR}/lease.env"
echo "AWS_DEFAULT_REGION=us-east-1" >> "${BENCH_WORK_DIR}/lease.env"
echo "" >> "${BENCH_WORK_DIR}/lease.env"
echo "# comment line" >> "${BENCH_WORK_DIR}/lease.env"

echo "install done"
