# Task: Fix crash-looping pods

The `app` deployment in the `bench` namespace has pods stuck in CrashLoopBackOff.
The containers keep restarting because they exit immediately after starting.

Diagnose the issue and fix the deployment so that all replicas become ready.

You have access to `kubectl` with the provided kubeconfig.
Do not create new deployments — fix the existing one.
