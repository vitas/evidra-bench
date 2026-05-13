# Skill Prompt Examples

Compact, bench-tested skill prompts for infrastructure AI agents. These files
are examples that can be passed to Bench with `--skill-file` and labeled with a
stable `--skill-id`.

## Usage

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --model sonnet \
  --provider bifrost \
  --skill-file skills/k8s-admin.md \
  --skill-id k8s-admin
```

## Examples

| Skill ID | File | Tracks | What it teaches |
|---|---|---|---|
| `k8s-admin` | `k8s-admin.md` | workloads, troubleshooting, networking, storage | Diagnosis-first, check before fix, blast radius awareness |
| `security-ops` | `security-ops.md` | pod-security, runtime-security | Deny-by-default, PSA/RBAC analysis, least-privilege |
| `release-manager` | `release-manager.md` | release-ops | Rollback discipline, Helm/ArgoCD safety |
| `platform-eng` | `platform-eng.md` | platform-eng | Terraform state safety, plan-before-apply, drift reconciliation |

## Design Principles

1. **Compact** — ~300 tokens max. Every token competes with reasoning capacity.
2. **Principles, not procedures** — "diagnose before fix" beats "run kubectl get pods first".
3. **Safety as negative rules** — "never delete outside scope" is more powerful than listing safe commands.
4. **One concern per skill** — `k8s-admin` does not mention security. Clean separation.

## External Protocol Skills

Protocol-specific prompts should be tested with `--skill-file` and a stable
`--skill-id` so reports can compare them as first-class skill runs.
The bench harness does not inject protocol tools or special runtime modes.

## Benchmark Data

Skills aren't universally good — they help some scenarios and hurt others.
Test before shipping.
