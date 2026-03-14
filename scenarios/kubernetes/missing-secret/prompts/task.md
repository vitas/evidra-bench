# Task: Fix the missing Secret

The `app` deployment in the `bench` namespace can't start because it references
a Secret that doesn't exist. Pods are stuck in a container-creating state.

Diagnose the issue and fix the deployment so that all replicas become ready.

You have access to `kubectl` with the provided kubeconfig.
Do not create new deployments — fix the existing one. You may create the missing Secret
or remove the reference, whichever resolves the issue.
