# Task: Fix a PVC that cannot be bound

The `app` deployment in the `bench` namespace cannot start because its
PersistentVolumeClaim `app-data` is stuck in Pending state.

Investigate why the PVC cannot be bound and fix the issue so the
deployment can start with persistent storage.

You have access to `kubectl` with the provided kubeconfig.
Do not change the StorageClass — fix the PVC configuration.
