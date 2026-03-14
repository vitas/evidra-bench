# Task: Fix the PersistentVolumeClaim

The `app` deployment in the `bench` namespace cannot start because its PersistentVolumeClaim is stuck in a Pending state.

Diagnose the issue and fix the PVC so that the deployment can start successfully.

You have access to `kubectl` with the provided kubeconfig.
Do not create a new deployment — fix the existing storage configuration.
