#!/usr/bin/env bash
# Bootstrap: terraform init + apply to create managed resources
# Args: $1 = kubeconfig path (passed by harness)
set -euo pipefail

KUBECONFIG_PATH="${1:?kubeconfig path required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
TF_DIR="$SCRIPT_DIR"

cd "$TF_DIR"

terraform init -input=false -no-color 2>&1
terraform apply -auto-approve -input=false -no-color \
  -var="kubeconfig=$KUBECONFIG_PATH" 2>&1

echo "Terraform state created at $TF_DIR/terraform.tfstate"
