# Task: Fix the API service — pods are not becoming ready

The `api` deployment in the `bench` namespace has pods that are not passing
readiness checks. The service is degraded.

Make the deployment healthy so all replicas are ready and serving traffic.

You have access to `kubectl` with the provided kubeconfig.
