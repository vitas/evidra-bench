# Task: Fix the NetworkPolicy blocking traffic

The `web` service in the `bench` namespace is unreachable. A NetworkPolicy is blocking all ingress traffic to the service.

Diagnose the issue and fix the NetworkPolicy so that traffic can reach the `web` service again.

You have access to `kubectl` with the provided kubeconfig.
Do not delete the deployment or service — fix the networking configuration.
