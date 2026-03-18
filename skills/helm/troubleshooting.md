# Helm Troubleshooting

## Diagnostic Sequence

1. **Check release status**
   - `helm list -n <ns>` — see all releases and their status
   - `helm status <release> -n <ns>` — detailed release info
   - Status values: deployed, failed, pending-install, pending-upgrade

2. **Check release history**
   - `helm history <release> -n <ns>` — see all revisions
   - Compare current vs previous values if a recent upgrade broke things

3. **Check values**
   - `helm get values <release> -n <ns>` — current applied values
   - Compare against chart defaults: `helm show values <chart>`

4. **Check hooks**
   - Failed hooks prevent releases from completing
   - Look for stuck hook pods: `kubectl get pods -n <ns> -l helm.sh/hook`

## Common Fixes

### Failed Release
- `helm rollback <release> <revision> -n <ns>` — go back to working version
- Never `helm uninstall` + `helm install` when rollback would work

### Pending Release (stuck)
- Check for stuck hooks or jobs
- If truly stuck: `helm rollback <release> 0 -n <ns>` to reset to last deployed

### Dependency Conflict
- `helm dependency update <chart>` — refresh dependencies
- Check Chart.lock vs Chart.yaml for version mismatches

## Rules

- Always check `helm history` before attempting a fix
- Prefer rollback over uninstall+reinstall — it preserves release history
- Never force-delete a release secret (`sh.helm.release.*`) as a fix
- After any helm operation, verify the underlying K8s resources are healthy
