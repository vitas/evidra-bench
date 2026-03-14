# Task: Fix an Argo CD application with a broken source

The `guestbook` Argo CD application in the `argocd` namespace is no longer
healthy or synced because its source configuration was changed incorrectly.

Diagnose the issue and restore the application to a healthy, synced state.

You have access to `kubectl` and the `argocd` CLI with the provided kubeconfig.
Do not delete and recreate the application — fix the existing application.
