# Evidence Modes Reference

## Overview

Every bench run has an evidence mode that determines how (and whether) infrastructure mutations are recorded as evidence entries.

## Modes

| Mode | Flag | Agent behavior | Evidra involvement | Evidence captured |
|------|------|---------------|-------------------|-------------------|
| **none** | (no flags) | Agent runs kubectl, helm, etc. normally | None | No |
| **proxy** | `--proxy-mode` | Identical to none — agent is unaware | Harness silently intercepts and records every mutation | Yes (auto) |
| **smart** | `--smart-prescribe` | Agent calls `prescribe_smart` before each mutation — one lightweight tool call | Agent-driven, minimal overhead (~80% fewer tokens than full protocol) | Yes (agent-driven) |
| **mcp** | `--mcp-server "evidra-mcp ..."` | Agent gets full MCP tool suite (prescribe, report, risk classification) | Full protocol — agent manages the entire evidence lifecycle | Yes (full protocol) |
| **direct** | `--evidra-bin path` | Agent calls `evidra prescribe` / `evidra report` as shell commands | Same as mcp but via CLI binary instead of MCP stdio transport | Yes (full protocol) |

## What changes between modes

| Transition | Agent code changes? | Agent needs new tools? | Performance impact | Evidence quality |
|-----------|--------------------|-----------------------|-------------------|-----------------|
| none → proxy | No | No | None | Auto-recorded, basic |
| proxy → smart | Yes — adds one tool call per mutation | Yes — `prescribe_smart` | Minimal (~1 extra call) | Agent-driven, lightweight |
| smart → mcp | Yes — full tool suite | Yes — prescribe, report, risk tools | Significant — some models drop 10-15% pass rate | Full protocol compliance |
| none → mcp | Yes — full tool suite | Yes — all tools | Significant | Full protocol compliance |

## mcp vs direct

These are functionally identical — same protocol, different transport:

| | mcp | direct |
|---|---|---|
| Transport | MCP stdio (JSON-RPC) | CLI binary (shell exec) |
| Flag | `--mcp-server "evidra-mcp ..."` | `--evidra-bin /path/to/evidra` |
| Tools | MCP tool discovery | `evidra prescribe`, `evidra report` commands |
| Process management | Harness starts/stops MCP server | Agent calls binary directly |
| Status | **Current** — modern path | **Legacy** — pre-MCP path |

For new work, use `mcp`. The `direct` mode exists for backward compatibility.

## Benchmark data (2026-03-29)

| Mode | Runs in DB | Notes |
|------|-----------|-------|
| none | 895 | Largest dataset — baseline for all models |
| proxy | 194 | Growing — identical pass rates to none |
| smart | 295 | Good coverage — shows protocol-aware models |
| mcp | 371 | Shows full compliance — some models drop 10-15% |
| direct | 142 | Legacy runs — no new runs planned |

## Which mode to use

| Use case | Mode | Why |
|----------|------|-----|
| Baseline benchmarking | **none** | Cheapest, fastest, measures raw agent ability |
| Benchmarking + evidence capture | **proxy** | Same as none but captures evidence for free |
| Testing protocol compliance | **smart** | Lightweight — tests if agent can prescribe before acting |
| Testing full evidra product experience | **mcp** | Tests agent with real evidra MCP server |
| Demo | **proxy** (live) + **smart/mcp** (leaderboard) | Live runs with proxy, pre-seeded data shows smart/mcp comparison |

## Demo story

| Step | Mode | What judges see |
|------|------|----------------|
| 1 | **none** | Agent fixes the problem. No evidence. Raw ability. |
| 2 | **proxy** | Same agent, same fix, zero code changes. Evidence page now shows full audit trail with risk classification. "Flip a switch." |
| 3 | (leaderboard filter) | Filter by **smart** — models that call `prescribe_smart` score higher on reliability. "The next level — agents that think before acting." |
