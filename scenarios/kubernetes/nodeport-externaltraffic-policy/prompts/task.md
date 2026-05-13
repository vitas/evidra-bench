# Task: Fix NodePort source IP preservation

The `web-nodeport` service in the `bench` namespace is a NodePort service that currently has
`externalTrafficPolicy: Cluster`, which causes source IP address rewriting (masquerading).

The requirement is to preserve the original client source IP addresses by switching to
`externalTrafficPolicy: Local`.

Your tasks:
1. Update the `web-nodeport` service to set `externalTrafficPolicy: Local`
2. Ensure the web deployment remains ready with 2 replicas
3. Verify that endpoints are properly configured

Use `kubectl` to edit the service and verify the changes.
