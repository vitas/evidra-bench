# Evidence Modes

`bench-cli` has two run-record evidence modes:

| Mode | How to select it | Meaning |
|---|---|---|
| `none` | no MCP server, or `evidence_mode: "none"` in API requests | Baseline run. The harness executes its built-in tools and stores normal benchmark artifacts. |
| `mcp` | `--mcp-server "..."`, or `evidence_mode: "mcp"` in API requests | The agent uses an external MCP server. The server may produce its own evidence, but the harness treats it as a generic tool backend. |

There are no bench-specific evidra submodes. To test `evidra-mcp`, run it the
same way as any other MCP server:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "evidra-mcp --signing-mode optional"
```

To compare tool backends, keep the model, provider, scenario set, timeout, and
cluster settings fixed, then change only `--mcp-server`.

```bash
# Baseline
bench-cli bench --scenario kubernetes --model sonnet --provider bifrost --reuse-cluster

# Same benchmark through evidra-mcp
bench-cli bench --scenario kubernetes --model sonnet --provider bifrost \
  --mcp-server "evidra-mcp --signing-mode optional" \
  --reuse-cluster

# Same benchmark through another MCP server
bench-cli bench --scenario kubernetes --model sonnet --provider bifrost \
  --mcp-server "npx -y @anthropic/mcp-server-kubernetes" \
  --reuse-cluster
```

Infrastructure verification is independent of evidence mode. Scenario checks
always run; MCP-specific artifacts are interpreted only when a scenario or a
separate verifier explicitly points at an evidence directory.
