# Task: Resolve Pod Security Admission conflict

The `bench` namespace enforces the `restricted` Pod Security Standard.
A monitoring DaemonSet `node-monitor` needs `hostPath` volumes to monitor
disk usage, but this violates the restricted policy.

The DaemonSet pods are failing to be admitted. Fix this so that:
- The monitoring DaemonSet can run with hostPath access
- The `web` deployment continues running under restricted PSA
- Security is not weakened for application workloads

You have access to `kubectl` with the provided kubeconfig.
Hint: Consider whether all workloads need to be in the same namespace.
