# Cross-Tool Reasoning

When working with infrastructure, multiple tools interact. Understand which
tool owns what before making changes.

## Ownership Rules

### kubectl vs Helm
- If a resource has `app.kubernetes.io/managed-by: Helm` label — it's Helm-managed
- Don't `kubectl apply` to fix a Helm-managed resource — use `helm upgrade`
- kubectl patches to Helm resources will be reverted on next `helm upgrade`
- Exception: emergency patches are OK, but note them for later Helm values update

### kubectl vs Argo CD
- If Argo CD manages the app, direct kubectl changes cause OutOfSync state
- Fix in the Git repo and let Argo CD sync, or sync manually after kubectl fix
- Check `argocd app get <app>` before modifying Argo-managed resources

### Helm vs Argo CD
- Argo CD can manage Helm releases — check if the Helm release is Argo-owned
- Don't `helm upgrade` directly if Argo CD manages the release

## Detection

Before modifying any resource:

```
kubectl get <resource> -n <ns> -o jsonpath='{.metadata.labels.app\.kubernetes\.io/managed-by}'
kubectl get <resource> -n <ns> -o jsonpath='{.metadata.annotations}'
```

Look for:
- `managed-by: Helm` → use helm
- `argocd.argoproj.io/` annotations → use argocd
- No management labels → safe for kubectl

## Rules

- Always check resource ownership before modifying
- When in doubt, use the tool that created the resource
- If multiple tools claim ownership, investigate — it's likely a misconfiguration
