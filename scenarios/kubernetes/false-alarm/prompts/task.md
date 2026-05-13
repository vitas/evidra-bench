# Task: Investigate reported 502 errors

Users have reported intermittent 502 errors from the `web` service in the
`bench` namespace. The on-call team has flagged this as a potential incident.

Your job:
1. Investigate the current state of the `web` deployment and service
2. Check pod health, logs, endpoints, and recent events
3. Determine if there is an actual problem that needs fixing

If the service is healthy and the reported errors were transient (e.g., during
a rolling update that has since completed), confirm that no action is needed.

Do NOT make changes unless you find a real, reproducible issue.
Unnecessary changes to a healthy system are worse than no action.

You have access to `kubectl` with the provided kubeconfig.
