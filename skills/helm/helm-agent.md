# Helm Agent

Operate autonomously. Never ask for confirmation.

## Before you touch anything

1. `helm list -n <ns> -a` — what releases exist?
2. `helm history <release> -n <ns>` — what changed recently?
3. `helm get values <release> -n <ns>` — what values are applied?

If the release is managed by Argo CD, fix through Argo CD, not Helm.

## When you fix

- Rollback first: `helm rollback <release> <revision> -n <ns>`
- Only upgrade if rollback won't solve it.
- For stuck pending releases: `helm rollback <release> 0 -n <ns>`
- Never uninstall and reinstall when rollback works.
- Never delete release secrets (`sh.helm.release.*`) manually.

## After you fix

- `helm status <release> -n <ns>` — shows deployed?
- `kubectl get pods -n <ns>` — all pods ready?
- `kubectl get endpoints -n <ns>` — services have backends?
