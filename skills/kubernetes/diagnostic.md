# Kubernetes Diagnostic Methodology

Always diagnose before acting. Never apply a fix without understanding the root cause.

## Diagnostic Sequence

1. **Observe** — get the current state before anything else
   - `kubectl get pods -n <ns>` — check pod status
   - `kubectl get events -n <ns> --sort-by=.lastTimestamp` — recent events
   - `kubectl describe <resource>` — detailed state

2. **Investigate** — dig into the failure
   - Check logs: `kubectl logs <pod> -n <ns>` (and `--previous` for crash loops)
   - Check container state: look for OOMKilled, CrashLoopBackOff, ImagePullBackOff
   - Check resource specs: wrong image, missing volume mounts, bad env vars
   - Check dependencies: ConfigMaps, Secrets, Services referenced but missing

3. **Understand** — identify the root cause, not just the symptom
   - A pod in CrashLoopBackOff is a symptom. The root cause might be a bad config, missing secret, or wrong command.
   - A deployment not ready could be a scheduling issue, not a container issue.
   - Error messages in events are more reliable than pod status alone.

4. **Fix** — apply the minimal change that addresses the root cause
   - Patch the specific field, don't replace the entire resource
   - One fix at a time — verify each before moving to the next

5. **Verify** — confirm the fix worked
   - `kubectl rollout status deployment/<name> -n <ns>`
   - `kubectl get pods -n <ns>` — all replicas ready
   - Check the service still has endpoints

## Rules

- Never restart pods as a first action. Diagnose first.
- Never delete and recreate when a patch would work.
- Never guess the fix from the pod name alone — always read the spec.
- If multiple things are broken, fix them in dependency order.
