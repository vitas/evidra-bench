# Task: Fix overly permissive NetworkPolicy

The `bench` namespace has a default-deny NetworkPolicy. An allow policy was
added to let the `frontend` reach the `backend`, but it accidentally also
allows `frontend` to reach the `database` directly.

Fix the NetworkPolicy configuration so that:
- `frontend` can reach `backend` (port 80)
- `backend` can reach `database` (port 80)
- `frontend` CANNOT reach `database` directly

You have access to `kubectl` with the provided kubeconfig.
Do not remove the default-deny policy. Do not remove the backend-to-database policy.
