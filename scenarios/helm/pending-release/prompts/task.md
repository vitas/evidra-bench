# Task: Fix a Helm release stuck in pending state

The `web` Helm release in the `bench` namespace is stuck in a pending-install state.
A pre-install hook Job never completed, which prevents the release from finishing.

Diagnose and recover the release so that it reaches a deployed state.

You have access to `kubectl` and `helm` with the provided kubeconfig.
