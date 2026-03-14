# Task: Fix an Argo CD application that fails to sync

The `web` Argo CD application fails to sync because of an invalid manifest in the source repository.
The sync error indicates a YAML parsing failure — likely a duplicate key or malformed structure.

Diagnose and fix the issue so the application syncs successfully and becomes healthy.

You have access to `kubectl` and `argocd` CLI with the provided kubeconfig.
