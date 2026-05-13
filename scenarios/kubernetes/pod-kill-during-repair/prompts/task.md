# Task: Fix the deployment even if pods restart while you work

The `web` deployment in the `bench` namespace is failing to become ready.
While you investigate, pods in this namespace may be recreated or restarted.

Diagnose the root cause and fix the deployment so that all replicas become
ready again.

You have access to `kubectl` with the provided kubeconfig.
Do not create a new deployment. Fix the existing `web` deployment.
