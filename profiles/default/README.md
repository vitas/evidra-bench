# Default Profile

The default profile creates a plain Kubernetes cluster with no additional
addons or hooks. No install, healthcheck, or cleanup scripts are required.

Scenarios using this profile get a single control-plane node cluster via
kind or k3d, with no extra setup beyond what the cluster provider creates.
