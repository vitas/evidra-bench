# Task: Fix the service selector

The `app` service in the `bench` namespace has no endpoints.
Traffic sent to the service is not reaching any pods.

Diagnose the issue and fix the service so that it correctly routes to the running pods.

You have access to `kubectl` with the provided kubeconfig.
Do not create new services or deployments — fix the existing service.
