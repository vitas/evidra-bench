# Evidra Smart Prescribe Protocol

Every infrastructure **mutation** follows three steps:

1. `evidra_prescribe_smart` — record intent, get prescription_id
2. Execute the command
3. `evidra_report` — record outcome with prescription_id

Read-only operations (get, describe, logs, plan, show) skip the protocol.

## Rules

- Do not execute mutate commands until prescribe returns ok=true with prescription_id.
- Every prescribe must have exactly one report.
- On retry, call prescribe again for a new prescription_id.

## Prescribe Smart

Call before every kubectl apply/delete/patch, helm upgrade/install, terraform apply/destroy.

```json
{
  "tool": "kubectl",
  "operation": "apply",
  "resource": "deployment/web",
  "namespace": "bench",
  "actor": {"type": "agent", "id": "bench", "origin": "mcp-stdio", "skill_version": "1.1.0"}
}
```

Required: tool, operation, resource, actor (type, id, origin).

## Report

Call after the command completes:

```json
{
  "prescription_id": "<from prescribe>",
  "verdict": "success",
  "exit_code": 0
}
```

Verdicts: success (exit 0), failure (non-zero exit), declined (chose not to execute).

## What Requires Protocol

| Tool | Mutating operations |
|------|-------------------|
| kubectl | apply, delete, patch, create, replace, rollout restart |
| helm | install, upgrade, uninstall, rollback |
| terraform | apply, destroy, import |

When unsure, call prescribe. The cost is one extra call. Skipping triggers a protocol violation.
