# Role-Based Skills

Compact, bench-tested skill prompts for infrastructure AI agents.
Each role loads ~300 tokens of domain-specific instructions.

## Usage

```bash
# Load a role skill in infra-bench
infra-bench certify --track cka --model sonnet --role k8s-admin

# Or with any agent via system prompt
--system-prompt-file skills/k8s-admin.md
```

## Roles

| Role | File | Tracks | What it teaches |
|---|---|---|---|
| `k8s-admin` | `k8s-admin.md` | workloads, troubleshooting, networking, storage | Diagnosis-first, check before fix, blast radius awareness |
| `security-ops` | `security-ops.md` | pod-security, runtime-security | Deny-by-default, PSA/RBAC analysis, least-privilege |
| `release-manager` | `release-manager.md` | release-ops | Rollback discipline, Helm/ArgoCD safety |
| `platform-eng` | `platform-eng.md` | platform-eng | Terraform state safety, plan-before-apply, drift reconciliation |

## Design Principles

1. **Compact** — ~300 tokens max. Every token competes with reasoning capacity.
2. **Principles, not procedures** — "diagnose before fix" beats "run kubectl get pods first".
3. **Safety as negative rules** — "never delete outside scope" is more powerful than listing safe commands.
4. **One concern per skill** — k8s-admin doesn't mention security. Clean separation.

## Legacy Skills

The `kubernetes/` and `helm/` subdirectories contain older generic skills
superseded by the role-based system above.

## Evidra Protocol Skills

The `evidra/` subdirectory contains protocol skills for the prescribe/report
evidence recording. Loaded separately via `--smart-prescribe` or `--system-prompt-file`.

## Benchmark Data

Skills aren't universally good — they help some scenarios and hurt others.
Test before shipping. See results at [lab.evidra.cc/results](https://lab.evidra.cc/results).
