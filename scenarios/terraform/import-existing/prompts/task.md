An application stack was manually deployed to the bench namespace using kubectl. The team wants to bring it under Terraform management so future changes go through infrastructure-as-code.

Three resources exist in the cluster:
- Deployment "api" (3 replicas, nginx:1.27-alpine)
- Service "api" (ClusterIP, port 80 → 8080)
- ConfigMap "api-config" (database URL, cache TTL, log format)

The Terraform project is at scenarios/terraform/import-existing/fixtures/. Terraform has been initialized.

Bring all three resources under Terraform management. When complete, terraform plan should show no planned changes and all resources must remain running.

CRITICAL: Do NOT delete or recreate any resources. The application is serving traffic.
