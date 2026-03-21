# Task: Fix broken DNS resolution

Pods in the `bench` namespace cannot resolve Kubernetes service names.
DNS lookups for services like `web.bench.svc.cluster.local` are failing.

Investigate the DNS configuration and fix the issue so that service
name resolution works correctly.

You have access to `kubectl` with the provided kubeconfig.
The DNS service runs as CoreDNS in the `kube-system` namespace.
