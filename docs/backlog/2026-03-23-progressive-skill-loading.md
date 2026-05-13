# Progressive Skill Loading for Infrastructure Agent Skills

Research date: 2026-03-23

## Problem

Today we load role-based skills (~300 tokens) into the agent's system prompt
upfront via `--role k8s-admin`. With 4 roles this is fine. With 20+ specialized
skills (k8s-networking, security-rbac, terraform-state, etc.) it becomes:

- **Token waste**: all skill text in context even when irrelevant
- **Context pollution**: generic instructions may override the agent's own judgment
- **No self-selection signal**: we don't know if the agent can pick the right skill

## Three-Level Progressive Loading

Inspired by DeerFlow (ByteDance) and Claude Code's skill system.

### Level 1: Metadata (always loaded)

~50 tokens per skill. Name + one-line description in the system prompt.

```
<available_skills>
  <skill name="k8s-admin" description="Kubernetes admin: diagnosis-first, check before fix, blast radius awareness"/>
  <skill name="security-ops" description="Security: deny-by-default, PSA/RBAC analysis, CVE response"/>
  <skill name="platform-eng" description="Terraform: state safety, plan-before-apply, drift reconciliation"/>
</available_skills>
```

Agent always knows what skills exist. Zero context overhead for unused skills.

### Level 2: Body (on-demand)

~300 tokens per skill. Full instructions. Loaded when agent decides it's relevant.

Agent calls `load_skill("k8s-admin")` → full skill text injected into conversation.

### Level 3: Resources (during execution)

Unlimited. Reference docs, examples, command syntax. Loaded via tool calls
only when needed mid-task.

Example: agent loaded k8s-networking skill, now needs NetworkPolicy YAML syntax →
calls `read_file("skills/k8s-networking/references/networkpolicy-spec.md")`.

## Research: How Others Implement This

### DeerFlow (ByteDance)

- Skills are `SKILL.md` files with YAML frontmatter
- Three-level loading documented in their skill-creator skill
- **LLM-driven matching**: model reads L1 descriptions, decides which to load via `read_file`
- No keyword/embedding search — pure model judgment
- Also has deferred tool loading (`tool_search`) for MCP tools: names always in prompt,
  schemas fetched on demand. Saves 55,000+ tokens when many MCP tools available.

Source: `github.com/bytedance/deer-flow`, `skills/public/skill-creator/SKILL.md`

### Claude Code

- Skills are `SKILL.md` files in `.claude/skills/` directories
- `Skill` tool is a first-class tool call (not ad-hoc `read_file`)
- Context budget: 2% of context window (~16,000 chars fallback)
- `disable-model-invocation: true` removes skill from context entirely
- Hierarchy: enterprise > personal > project > plugin

### Academic (arXiv 2602.12430)

"Agent Skills for Large Language Models" formalizes the pattern:
- L1 Table of Contents (~20-50 tokens/skill)
- L2 Chapter Content (~500-2000 tokens)
- L3 Technical Appendices (unlimited)

Key insight: "Skills modify the agent's preparation, not its output directly."
They are meta-instructions that reshape capabilities before response generation.

## Comparison of Approaches

| Approach | Token cost (20 skills) | Agent decides? | Latency |
|---|---|---|---|
| **Load all upfront** | ~6000 tokens | No (harness decides) | 0 turns |
| **L1 metadata only** | ~1000 tokens | Yes (model selects) | +1 turn |
| **Deferred tool search** | ~400 tokens (names only) | Yes | +1 turn per tool group |
| **No skills** | 0 tokens | N/A | 0 turns |

## What This Means for evidra-bench

### Current State (Phase 1) — Done

4 role skills loaded via `--role` flag. Full skill in system prompt.
300 extra tokens. Simple, deterministic, works.

### Future State (Phase 2) — Backlogged

When we have 15+ skills:

```bash
# Agent self-selects the right skill
infra-bench certify --track cka --model sonnet --role auto
```

Implementation:
1. `skills/registry.yaml` — L1 metadata for all skills
2. `load_skill` tool — agent calls to load L2 body
3. New benchmark signal: **skill selection accuracy**
   - Did the agent pick the right skill?
   - Did self-selection improve or hurt pass rate vs manual `--role`?

### Future State (Phase 3) — Backlogged

L3 reference resources per skill:

```
skills/
  k8s-admin/
    SKILL.md              # L2 body
    references/
      diagnosis-flowchart.md   # L3
      common-errors.md         # L3
  security-ops/
    SKILL.md
    references/
      psa-levels.md
      networkpolicy-spec.md
```

### For MCP server Server

Progressive loading is more valuable here than in the benchmark:
- MCP server may have 10+ roles configured
- Agent connects and sees L1 metadata for all roles
- On first infrastructure command, agent loads the relevant L2 skill
- Saves tokens on every session where only 1-2 roles are needed

## Key Insights

1. **Progressive loading is a scale optimization, not a feature for 4 skills.**
   Don't build it until we have 15+ skills.

2. **Self-selection is the interesting benchmark signal.** Can the agent pick
   the right skill? That's a new axis of measurement nobody has data on.

3. **LLM-driven matching (DeerFlow) vs tool-driven (Claude Code):**
   DeerFlow uses `read_file` on skill paths. Claude Code uses a `Skill` tool.
   The tool approach is cleaner — explicit invocation vs implicit file read.

4. **Skills that hurt performance (our finding) make self-selection harder.**
   If the wrong skill breaks L2 scenarios, the agent must not only pick correctly
   but also know when NOT to load any skill.

5. **Deferred tool loading is separately valuable.** When MCP server has many
   tools, listing only names and letting agents search for schemas saves massive
   context. This is independent of skill loading.

## Implementation Priority

1. Ship 4 role skills as-is (done)
2. Collect benchmark data on skill impact (in progress)
3. Write "Why Your Skill Sucks" post with real data
4. Build more specialized skills (k8s-networking, security-rbac, etc.)
5. Implement `--role auto` with L1/L2 loading when skill count > 10
6. Add skill selection accuracy as a benchmark signal
7. Implement deferred tool loading for MCP server
