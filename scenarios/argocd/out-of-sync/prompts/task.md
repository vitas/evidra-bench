# Task: Fix an Argo CD application that is out of sync

The `guestbook` Argo CD application in the `argocd` namespace is out of sync
because someone changed the live workload directly in the cluster.

Diagnose the issue and restore the application to a healthy, synced state.

You have access to `kubectl` and the `argocd` CLI with the provided kubeconfig.
Do not delete and recreate the application — fix the existing application.
