# Execution Modes Reference

## Two Dimensions

Bench runs have two independent dimensions:

1. **Adapter** — how the agent executes (built-in provider loop, remote A2A, CI process)
2. **Evidence mode** — how mutations are recorded (none, proxy, smart, evidra-mcp)

### Adapter × Evidence Mode Matrix

|  | none | proxy | smart | evidra-mcp |
|--|------|-------|-------|------------|
| **Provider (built-in)** | Raw baseline | Auto-evidence | Agent calls prescribe_smart | Agent uses evidra-mcp server |
| **A2A (remote agent)** | Remote agent, no evidence | Remote agent + auto-evidence | N/A — remote agent owns tools | N/A — remote agent owns tools |
| **CLI (CI/packaged)** | Spawn process | Spawn + auto-evidence | N/A | N/A |

### Adapter flags

| Adapter | Flag | What it does |
|---------|------|-------------|
| **Provider** | `--provider bifrost` (or `claude`) | Built-in multi-turn tool-use agent loop |
| **A2A** | `--adapter a2a --a2a-agent-url URL` | Sends task to remote A2A agent, waits for result |
| **CLI** | `--adapter cli --agent-command "cmd"` | Spawns a packaged agent binary/script with env vars (designed for CI pipelines and pre-built agents) |

Dispatch order: `adapter=a2a` → remote A2A, `provider!=empty` → built-in loop, `adapter=cli` → external process.

---

## Evidence Modes

Every bench run has an evidence mode that determines how (and whether) infrastructure mutations are recorded.

### Modes

| Mode | Flag | Agent behavior | Evidra involvement | Evidence captured |
|------|------|---------------|-------------------|-------------------|
| **none** | (no flags) | Agent runs kubectl, helm, etc. normally | None | No |
| **proxy** | `--proxy-mode` | Identical to none — agent is unaware | Harness silently intercepts and records every mutation | Yes (auto) |
| **smart** | `--smart-prescribe` | Agent calls `prescribe_smart` before each mutation — one lightweight tool call | Agent-driven, minimal overhead (~80% fewer tokens than full protocol) | Yes (agent-driven) |
| **evidra-mcp** | `--mcp-server "evidra-mcp ..."` | Agent gets evidra's full MCP tool suite (prescribe, report, risk classification) | Full evidra protocol — agent manages the entire evidence lifecycle | Yes (full protocol) |
| **direct** | `--evidra-bin path` | Agent calls `evidra prescribe` / `evidra report` as shell commands | Same protocol as evidra-mcp, different transport (CLI binary instead of MCP stdio) | Yes (full protocol) |

### Important: evidra-mcp is not standard MCP

The `--mcp-server` flag is an **evidra-specific integration**, not a generic MCP client. The harness:

- Starts `evidra-mcp` as a sidecar with evidra-specific config (evidence dir, signing mode)
- Provides evidra protocol tools (prescribe, report, risk classification) to the built-in agent loop
- Tests the **evidra product experience** — how an agent behaves when it has evidra tools available

A standard MCP test (generic MCP server, agent discovers tools independently) would use the A2A adapter with an MCP-capable remote agent.

### What changes between modes

| Transition | Agent changes? | New tools? | Performance impact | Evidence quality |
|-----------|---------------|-----------|-------------------|-----------------|
| none → proxy | No | No | None | Auto-recorded, basic |
| proxy → smart | Yes — adds one tool call per mutation | Yes — `prescribe_smart` | Minimal (~1 extra call) | Agent-driven, lightweight |
| smart → evidra-mcp | Yes — full tool suite | Yes — prescribe, report, risk tools | Significant — some models drop 10-15% pass rate | Full protocol compliance |
| none → evidra-mcp | Yes — full tool suite | Yes — all tools | Significant | Full protocol compliance |

### evidra-mcp vs direct

Same evidra protocol, different transport:

| | evidra-mcp | direct |
|---|---|---|
| Transport | MCP stdio (JSON-RPC) | CLI binary (shell exec) |
| Flag | `--mcp-server "evidra-mcp ..."` | `--evidra-bin /path/to/evidra` |
| Tools | MCP tool discovery | `evidra prescribe`, `evidra report` commands |
| Process management | Harness starts/stops MCP server | Agent calls binary directly |
| Status | **Current** | **Legacy** — pre-MCP path, backward compatibility only |

---

## Benchmark Data (2026-03-29)

| Mode | Runs | Notes |
|------|------|-------|
| none | 895 | Largest dataset — baseline for all models |
| proxy | 194 | Growing — identical pass rates to none |
| smart | 295 | Good coverage — shows protocol-aware models |
| evidra-mcp | 371 | Full compliance — some models drop 10-15% |
| direct | 142 | Legacy — no new runs planned |

## Which Mode to Use

| Use case | Mode | Why |
|----------|------|-----|
| Baseline benchmarking | **none** | Cheapest, fastest, measures raw agent ability |
| Benchmarking + evidence | **proxy** | Same as none but captures evidence for free |
| Protocol compliance | **smart** | Lightweight — tests if agent can prescribe before acting |
| Evidra product experience | **evidra-mcp** | Tests agent with real evidra MCP server and full protocol |
| CI / packaged agents | **none** or **proxy** via CLI adapter | External agent binary, harness verifies outcome |

## Demo Story

| Step | Mode | What judges see |
|------|------|----------------|
| 1 | **none** | Agent fixes the problem. No evidence. Raw ability. |
| 2 | **proxy** | Same agent, same fix, zero code changes. Evidence page now shows full audit trail with risk classification. "Flip a switch." |
| 3 | (leaderboard filter) | Filter by **smart** — models that call `prescribe_smart` score higher on reliability. "The next level — agents that think before acting." |
