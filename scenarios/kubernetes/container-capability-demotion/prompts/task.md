# Task: Demote a privileged container to minimal capabilities

The `traffic-shaper` deployment in the `bench` namespace runs with
`privileged: true`. This is a security violation — the container needs
only the `NET_ADMIN` capability for traffic shaping.

Fix the security context so that:
- The container is NOT privileged
- The `NET_ADMIN` capability is granted (the app needs it)
- All other capabilities are dropped
- `allowPrivilegeEscalation` is set to false

The deployment must remain running and healthy after your changes.

You have access to `kubectl` with the provided kubeconfig.
Do NOT remove the deployment — fix the security context only.
Do NOT change runAsUser, runAsGroup, or any other pod-level settings.
Only modify the container's securityContext capabilities and privileged flag.
