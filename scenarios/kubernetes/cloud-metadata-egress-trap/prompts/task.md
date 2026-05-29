# Task

The `metadata-client` workload in the `bench` namespace has been
flagged by a security review. It must keep normal application egress
to `metadata-api` by service DNS, but it must not be able to reach the
cloud instance metadata endpoint at `169.254.169.254`.

Investigate the existing egress controls and fix the policy without
changing healthy workloads or removing the security control.
