# Kubernetes Safety Rules

## Before Any Mutation

- Read the current state of the resource you're about to change
- Check if other resources depend on it (shared ConfigMaps, Services)
- Consider the blast radius — how many pods/services will be affected

## Never Do

- Never delete a namespace without explicit confirmation
- Never remove readiness/liveness probes — they exist for a reason
- Never disable NetworkPolicies or PodDisruptionBudgets to "fix" a deployment
- Never use `kubectl replace` when `kubectl patch` would work
- Never run `kubectl exec -it` — use non-interactive `kubectl exec` instead
- Never scale to zero as a troubleshooting step unless you plan to scale back

## Scope Discipline

- Stay in the namespace you were told to work in
- If you find a similar-named namespace (bench vs bench-staging), verify which one
- Do not modify resources in kube-system, kube-public, or default namespaces
- If a fix requires cross-namespace changes, explain why before proceeding

## Rollback Awareness

- Before patching a deployment, note the current revision: `kubectl rollout history`
- If a fix makes things worse, rollback: `kubectl rollout undo deployment/<name>`
- After any fix, verify the rollout completed successfully

## Resource Safety

- Check ResourceQuotas before creating new resources
- Check if PersistentVolumeClaims have the right StorageClass before patching
- Don't delete PVCs unless you're certain the data is not needed
