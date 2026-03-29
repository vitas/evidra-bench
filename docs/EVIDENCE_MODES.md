# Execution Modes Reference

## Two Paths

The bench framework has two distinct execution paths:

### Path 1: Infrastructure Testing (any agent, any MCP server)

Tests whether an agent can fix infrastructure problems. Works with any MCP server or no MCP server at all.

| Mode | Flag | What happens | Evidence |
|------|------|-------------|----------|
| **Baseline** | (no flags) | Agent runs kubectl/helm directly | None |
| **Evidra Auto** | `--proxy-mode` | Identical to baseline — harness silently records mutations | Auto-captured |

These modes work with every adapter:

| Adapter | Flag | Use case |
|---------|------|----------|
| **Provider** | `--provider bifrost --model gemini-2.5-flash` | Built-in agent loop, direct model testing |
| **Any MCP server** | `--mcp-server "npx @anthropic/mcp-server-kubernetes"` | Agent uses third-party MCP tools |
| **A2A** | `--adapter a2a --a2a-agent-url URL` | Remote agent, harness only bootstraps and verifies |
| **CLI** | `--adapter cli --agent-command "cmd"` | Packaged agent binary for CI pipelines |

### Path 2: Evidra Protocol Testing (evidra-mcp only)

Tests whether an agent follows the evidra prescribe/report protocol. Requires `evidra-mcp` as the MCP server.

| Mode | Flag | What the agent does | Evidence |
|------|------|-------------------|----------|
| **Evidra Smart** | `--smart-prescribe` | Agent calls `prescribe_smart` before each mutation — one lightweight tool call | Agent-driven, minimal overhead |
| **Evidra Full** | `--mcp-server "evidra-mcp --signing-mode optional"` | Agent uses full evidra tool suite: prescribe, report, risk classification | Full protocol compliance |

These modes only work with the **Provider** adapter (built-in agent loop) because the harness must inject evidra tools into the agent's tool set.

**A2A and CLI agents own their tool loop** — the harness cannot inject evidra tools into them. If a remote agent uses evidra internally, that's the agent's choice, not the harness's. The evidence mode filter on the leaderboard is not meaningful for A2A/CLI runs.

---

## Summary Matrix

|  | Baseline | Evidra Auto | Evidra Smart | Evidra Full |
|--|----------|-------------|-------------|-------------|
| **Provider** | Yes | Yes | Yes | Yes |
| **Any MCP server** | Yes | Yes | — | — |
| **A2A** | Yes | Yes | — | — |
| **CLI** | Yes | Yes | — | — |

---

## Benchmark Data (2026-03-29)

| Mode | Label | Runs | Notes |
|------|-------|------|-------|
| `none` | Baseline | 895 | Largest dataset — raw agent ability |
| `proxy` | Evidra Auto | 194 | Identical pass rates to baseline |
| `smart` | Evidra Smart | 295 | Lightweight protocol compliance |
| `mcp` | Evidra Full | 371 | Full protocol — some models drop 10-15% |

## Which Mode to Use

| Goal | Mode | Why |
|------|------|-----|
| Measure raw agent ability | **Baseline** | No overhead, cheapest |
| Measure + get free audit trail | **Evidra Auto** | Same pass rate, evidence for free |
| Test protocol awareness | **Evidra Smart** | Does the agent prescribe before acting? |
| Test full evidra product experience | **Evidra Full** | Full prescribe/report/risk protocol |
| Compare MCP servers | **Baseline** with different `--mcp-server` | Same agent, different tool backends |
| Test remote agents | **Baseline** or **Evidra Auto** via A2A | Harness bootstraps and verifies, agent owns execution |

## Demo Story

| Step | What | Mode |
|------|------|------|
| 1 | Agent fixes broken deployment | **Baseline** — raw ability, no evidence |
| 2 | Same agent, flip a switch | **Evidra Auto** — same result, full audit trail appears |
| 3 | Leaderboard filter | Show **Evidra Smart** — models that prescribe first score higher on reliability |
