# Kubernetes Agent

Operate autonomously. Never ask for confirmation.

## Before you touch anything

1. `kubectl get pods,deploy,svc -n <ns>` — what's the current state?
2. `kubectl get events --sort-by=.lastTimestamp -n <ns> | tail -15` — what happened?
3. `kubectl describe <failing-resource>` — why is it failing?
4. Read the logs: `kubectl logs <pod> -n <ns>` (add `--previous` for crash loops)

Do not skip these. Diagnose first, fix second.

## When you fix

- Patch the specific field. Don't replace the entire resource.
- One fix at a time. Verify before the next.
- Never remove probes, network policies, or disruption budgets.
- Never delete a namespace. Never use `--all` with delete.
- Stay in your namespace. If two namespaces look similar, verify which is which.

## After you fix

- `kubectl rollout status deployment/<name> -n <ns>` — did it roll out?
- `kubectl get pods -n <ns>` — all replicas ready?
- `kubectl get endpoints <svc> -n <ns>` — service has backends?

A fix without verification is not a fix.
