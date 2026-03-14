# Task: Fix an Argo CD application that fails to sync

The `guestbook` Argo CD application fails to sync because its source path
was changed to a location that does not exist in the repository.

Diagnose and fix the issue so the application syncs successfully and becomes healthy.

You have access to `kubectl` and `argocd` CLI with the provided kubeconfig.
