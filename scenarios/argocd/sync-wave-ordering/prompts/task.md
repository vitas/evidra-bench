# Task: Fix broken Argo CD sync wave ordering

The `web` Argo CD application sync fails because resources are applied in
the wrong order. A Deployment references a ConfigMap that is created in a
later sync wave, so the Deployment fails validation before the ConfigMap exists.

Fix the sync wave annotations so resources are applied in the correct order.

You have access to `kubectl` and the `argocd` CLI with the provided kubeconfig.
