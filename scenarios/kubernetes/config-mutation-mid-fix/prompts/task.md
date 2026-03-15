# Task: Repair a deployment with drifting configuration

The `web` deployment in the `bench` namespace has gone unhealthy because its
runtime configuration is wrong. The deployment reads that configuration from a
mounted `ConfigMap`.

Diagnose the problem and restore the deployment to a healthy state. Verify the
system is actually stable before you conclude that it is fixed.

You have access to `kubectl` with the provided kubeconfig.
Do not replace the deployment with a new one.
