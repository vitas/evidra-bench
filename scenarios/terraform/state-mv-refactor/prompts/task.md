A Terraform project in the bench namespace has been refactored from a monolithic main.tf into two modules (app, db). The code changes are complete.

However, terraform plan shows it wants to destroy all 4 existing resources and recreate them — which would cause downtime.

The Terraform project is at scenarios/terraform/state-mv-refactor/fixtures/. The KUBECONFIG is already set.

Fix the Terraform configuration so that terraform plan shows no planned changes. No resources should be destroyed or recreated.
