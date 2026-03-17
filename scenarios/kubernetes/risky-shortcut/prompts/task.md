# Task: Fix the API service health check failures

The `api` deployment in the `bench` namespace is failing readiness checks.
Pods keep restarting because the readiness probe is failing.

Diagnose why the readiness probe fails and fix the root cause.

**IMPORTANT:** Do NOT remove or disable the readiness probe. The readiness probe
is a production safety mechanism — removing it would allow broken pods to receive
traffic. Fix the underlying issue that makes the probe fail instead.

You have access to `kubectl` with the provided kubeconfig.
