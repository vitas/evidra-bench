# Evidra Skills

Free, open source skill prompts for infrastructure AI agents.
Benchmarked against real Kubernetes failures at [bench.evidra.cc](https://bench.evidra.cc).

## The Three Layers

Benchmark data shows three distinct reliability tiers:

| Layer | Skill | Pass Rate | Cost |
|-------|-------|-----------|------|
| **Naked model** | None — raw model ability | ~36% | - |
| **+ Generic skill** | `k8s-agent.md` / `helm-agent.md` | ~55%* | Free |
| **+ Evidra protocol** | Generic + `evidra/protocol.md` | ~100%* | With evidra |

*Benchmarked across 9 models and 36 Kubernetes failure scenarios.

Generic skills are **free best practices** — they improve any agent without
vendor lock-in. The evidra protocol adds structured evidence recording
on top, closing the gap to full reliability.

## Skills

```
skills/
├── kubernetes/k8s-agent.md   # Generic K8s principles (free, open source)
├── helm/helm-agent.md        # Generic Helm principles (free, open source)
└── evidra/protocol.md        # Evidra prescribe/report protocol (with product)
```

### Generic Skills (free)

Short, principled prompts that teach any AI agent basic operational discipline.
No step-by-step instructions — just rules any DevOps engineer would agree with.

- **Diagnose before you act.** Understand the root cause first.
- **Smallest change.** Patch, don't replace.
- **Verify your fix.** Don't assume it worked.
- **Respect safety mechanisms.** Never remove probes or policies.

### Evidra Protocol Skill (with product)

Adds structured evidence recording: every mutation gets a prescribe/report
cycle. The agent proves what it did and why — not just that it ran a command.

## Usage

```bash
# Add to any agent as system prompt
--system-prompt-file skills/kubernetes/k8s-agent.md
```

## For Third-Party Agents

When benchmarking third-party agents (kagent, custom MCP agents), we do NOT
inject our skills. We test their agent as-is:

| Mode | Evidence | Measures |
|------|----------|----------|
| **Proxy** | Auto-recorded by evidra-mcp --proxy | "How reliable is your agent?" |
| **Direct** | Agent calls prescribe/report (evidra protocol) | "Full compliance + risk assessment" |

Choose one mode per run. They don't mix.

## Benchmark Scores

See [bench.evidra.cc/skill-impact](https://bench.evidra.cc/skill-impact) for
live comparison data across models and skills.
