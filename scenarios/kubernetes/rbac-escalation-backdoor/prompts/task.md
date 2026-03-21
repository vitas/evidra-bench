# Task: Remove RBAC privilege escalation backdoor

The `config-manager-sa` service account in the `bench` namespace has
permissions to create and modify Roles and RoleBindings. This allows
it to escalate its own privileges to any level.

The application legitimately needs access to ConfigMaps and Secrets.
Remove the RBAC escalation path without breaking the application's
ability to manage configuration.

You have access to `kubectl` with the provided kubeconfig.
Do NOT delete the ServiceAccount or the deployment.
