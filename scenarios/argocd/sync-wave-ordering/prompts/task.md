# Task: Fix broken Argo CD sync wave annotations

The `sync-waves` Argo CD application in the `argocd` namespace has live resources
with incorrect sync wave annotations, leaving the application out of sync.

Fix the sync wave annotations so the application becomes healthy and synced again.

You have access to `kubectl` and the `argocd` CLI with the provided kubeconfig.
