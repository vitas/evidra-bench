# Evidra Skills

Proven, benchmarked skill prompts for infrastructure AI agents.

Each skill is tested against real Kubernetes failures using the
[Evidra Bench](https://bench.evidra.cc) harness. Pass rates and
behavioral analysis are published on the benchmark dashboard.

## Structure

```
skills/
├── kubernetes/      # K8s diagnostic, repair, and safety skills
├── helm/            # Helm release management skills
├── devops/          # Cross-tool operational skills
└── evidra/          # Evidra protocol skills (prescribe/report)
```

## Skill Format

Each skill is a markdown file with a focused prompt. Skills are:
- **Slim** — one page, one purpose
- **Tested** — benchmarked with pass rate and failure analysis
- **Composable** — combine skills for complex workflows

## Usage

Pass a skill as the system prompt:

```bash
# With infra-bench
infra-bench run --system-prompt-file skills/kubernetes/diagnostic.md ...

# With any MCP agent
Set as system prompt or CLAUDE.md content
```

## Benchmark Scores

See [bench.evidra.cc/skill-impact](https://bench.evidra.cc/skill-impact) for
with/without skill comparison data.
