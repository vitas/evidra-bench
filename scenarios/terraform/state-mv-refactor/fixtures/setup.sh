#!/usr/bin/env bash
# Bootstrap: apply monolithic version, then swap in refactored code with modules
set -euo pipefail
KUBECONFIG_PATH="${1:?kubeconfig path required}"
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
cd "$SCRIPT_DIR"

# Save the refactored main.tf (with modules) for later
cp main.tf /tmp/bench-tf-refactored-main.tf

# Use the monolithic version for initial apply (from .bak so terraform ignores it)
cp monolithic.tf.bak main.tf

# Write kubeconfig path to tfvars
echo "kubeconfig = \"$KUBECONFIG_PATH\"" > terraform.tfvars

# Init and apply monolithic version
terraform init -input=false -no-color 2>&1
terraform apply -auto-approve -input=false -no-color 2>&1

# Restore the refactored main.tf (with modules)
cp /tmp/bench-tf-refactored-main.tf main.tf

# Re-init to pick up module sources
terraform init -input=false -no-color 2>&1

echo "Monolithic resources created. Refactored code with modules is now in place."
echo "State has old addresses: kubernetes_deployment_v1.web, kubernetes_deployment_v1.db, etc."
echo "Code expects module addresses: module.app.kubernetes_deployment_v1.web, module.db.kubernetes_deployment_v1.db, etc."
