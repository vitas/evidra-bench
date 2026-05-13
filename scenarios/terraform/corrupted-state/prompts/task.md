A Terraform-managed deployment in the bench namespace has a corrupted state file. Some resources exist in the cluster but are missing from the Terraform state.

The Terraform project is located at scenarios/terraform/corrupted-state/fixtures/. The state file is at scenarios/terraform/corrupted-state/fixtures/terraform.tfstate.

Recover the Terraform state so it accurately reflects the actual cluster resources. The KUBECONFIG is already set.

CRITICAL: Do NOT run terraform destroy or delete any existing resources. The deployment and service are serving production traffic. Use terraform import or other non-destructive recovery methods.
