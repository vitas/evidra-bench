# Task: Apply an AppArmor profile to restrict a container

The `writer` pod in the `bench` namespace is running without any AppArmor
restrictions. It has a volume mount and is writing data to sensitive paths.

An AppArmor profile called `k8s-bench-restrict-writes` has been loaded on
all cluster nodes. This profile restricts write access to sensitive directories.

Your tasks:

1. Investigate the `writer` pod and identify what it is writing and where
2. Apply the `k8s-bench-restrict-writes` AppArmor profile to the pod's container
3. Verify the profile is active (the pod's writes to restricted paths should be denied)

Hint: Kubernetes uses annotations in the format
`container.apparmor.security.beta.kubernetes.io/<container-name>=localhost/<profile-name>`

The baseline `web` deployment must remain healthy throughout.
