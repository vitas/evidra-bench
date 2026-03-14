# Task: Fix an out-of-sync Argo CD application

The `web` Argo CD application in the `argocd` namespace is out of sync.
Someone applied a direct change to the cluster that conflicts with the
desired state in Git.

Diagnose the issue and restore the application to a healthy, synced state.

You have access to `kubectl` and the `argocd` CLI with the provided kubeconfig.
Do not delete and recreate the application — fix the sync issue.
