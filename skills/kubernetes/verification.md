# Kubernetes Verification Chain

After every fix, verify at multiple levels. A fix is not complete until verified.

## Verification Steps

### 1. Resource Level
- The resource you modified exists and has the expected spec
- `kubectl get <resource> -n <ns> -o yaml` — confirm the change applied

### 2. Pod Level
- All pods are Running and Ready (READY column shows N/N)
- No pods in CrashLoopBackOff, ImagePullBackOff, or Pending
- `kubectl get pods -n <ns> -l <selector>`

### 3. Rollout Level
- Deployment rollout completed successfully
- `kubectl rollout status deployment/<name> -n <ns> --timeout=60s`

### 4. Service Level
- Service has endpoints (pods are registered)
- `kubectl get endpoints <service-name> -n <ns>`
- Endpoints list should not be empty

### 5. Functional Level
- If possible, test the service responds correctly
- Check logs for errors after the fix: `kubectl logs -l <selector> -n <ns> --tail=10`

## Rules

- Never claim a fix is done without running at least steps 1-3
- If rollout status hangs, investigate — don't just wait
- If pods restart after your fix, the fix is wrong — investigate further
- Compare before/after state to confirm only your intended change applied
