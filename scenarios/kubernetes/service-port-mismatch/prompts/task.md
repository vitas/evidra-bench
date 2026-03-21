# Task: Fix a Service that cannot be reached

The `api` service in the `bench` namespace has endpoints but connections
to it fail. The pods are running and healthy, but traffic through the
Service does not reach them.

Diagnose the networking issue and fix the Service configuration.

You have access to `kubectl` with the provided kubeconfig.
