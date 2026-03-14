# Task: Fix a deployment with misconfigured probes

The `app` deployment in the `bench` namespace has pods that keep restarting and never become ready.
Check the probes — something is wrong with the readiness and liveness probe configuration.

Diagnose the issue and fix the deployment so that all replicas become ready and stay running.

You have access to `kubectl` with the provided kubeconfig.
Do not create new deployments — fix the existing one.
