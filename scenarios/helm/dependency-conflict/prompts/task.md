# Task: Resolve the Helm chart dependency issue

The `web` Helm release in the `bench` namespace was upgraded with a chart configuration
that enables a dependency on a ConfigMap which the chart no longer renders.
The upgrade completed, but the workload is now failing because that dependency is unresolved.

Diagnose the dependency issue and restore the Helm release to a healthy deployed state.

You have access to `kubectl` and `helm` with the provided kubeconfig.
