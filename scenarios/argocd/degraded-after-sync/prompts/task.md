# Task: Fix a Degraded Argo CD application

The `web` Argo CD application synced successfully but is now showing a
Degraded health status. The deployment pods are failing to start.

Diagnose the root cause and restore the application to a Healthy state.
The issue is in the deployment spec that was synced, not in Argo CD itself.

You have access to `kubectl` and the `argocd` CLI with the provided kubeconfig.
