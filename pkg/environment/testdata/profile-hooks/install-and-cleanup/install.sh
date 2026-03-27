#!/bin/sh
set -eu

# Prove env propagation by writing env vars to a marker file.
echo "KUBECONFIG=${KUBECONFIG}" > "${EVIDRA_WORK_DIR}/marker.env"
echo "EVIDRA_PROFILE=${EVIDRA_PROFILE}" >> "${EVIDRA_WORK_DIR}/marker.env"
echo "EVIDRA_PROVIDER=${EVIDRA_PROVIDER}" >> "${EVIDRA_WORK_DIR}/marker.env"
echo "EVIDRA_CLUSTER_NAME=${EVIDRA_CLUSTER_NAME}" >> "${EVIDRA_WORK_DIR}/marker.env"
echo "EVIDRA_ASSETS_DIR=${EVIDRA_ASSETS_DIR}" >> "${EVIDRA_WORK_DIR}/marker.env"

# Write a lease.env that downstream code should parse.
echo "LOCALSTACK_ENDPOINT=http://localhost:4566" > "${EVIDRA_WORK_DIR}/lease.env"
echo "AWS_DEFAULT_REGION=us-east-1" >> "${EVIDRA_WORK_DIR}/lease.env"
echo "" >> "${EVIDRA_WORK_DIR}/lease.env"
echo "# comment line" >> "${EVIDRA_WORK_DIR}/lease.env"

echo "install done"
