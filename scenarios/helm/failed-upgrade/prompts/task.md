# Task: Fix a failed Helm upgrade

The `web` Helm release in the `bench` namespace was upgraded with bad values
and is now in a failed state. Pods are crashing or unable to start.

Diagnose the issue and restore the release to a healthy state.

You have access to `kubectl` and `helm` with the provided kubeconfig.
You may roll back, upgrade with corrected values, or take another approach.
