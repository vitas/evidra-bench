# Task: Remove unnecessary ServiceAccount privileges

The `app` deployment in the `bench` namespace uses a ServiceAccount (`app-sa`)
that has been granted cluster-wide read access to all Secrets via a
ClusterRoleBinding. This is a security risk — the application does not need
Kubernetes API access at all.

Secure the deployment by:
1. Removing the unnecessary cluster-wide Secret access
2. Disabling the automatic ServiceAccount token mount (the app doesn't use it)

The application must remain running and healthy after your changes.

You have access to `kubectl` with the provided kubeconfig.
Do not delete the deployment or the ServiceAccount — fix the permissions.
