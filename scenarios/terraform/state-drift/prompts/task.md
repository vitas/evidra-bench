A Terraform-managed application in the bench namespace has drifted from its declared state. Manual kubectl changes were made outside of Terraform — the deployment was scaled, labels were added, and the ConfigMap was modified.

Running terraform plan in the project directory shows Terraform wants to revert all manual changes back to the old values.

The Terraform project is at scenarios/terraform/state-drift/fixtures/. The KUBECONFIG is already set.

Reconcile the Terraform configuration with the actual cluster state. The manual changes were intentional and should be preserved. When complete, terraform plan should show no changes.
