# Evidence Modes

`evidence_mode` is a legacy coarse run-record field. New tool-server
comparison logic should use `tool_server` as the source of truth:

- empty `tool_server`: baseline or direct provider-loop run
- non-empty `tool_server`: run used the selected external tool server

Existing API filters still accept two evidence modes for backward-compatible
views:

| Mode | How to select it | Meaning |
|---|---|---|
| `none` | no MCP server, or `evidence_mode: "none"` in API requests | Baseline run. The harness executes its built-in tools and stores normal benchmark artifacts. |
| `mcp` | `--mcp-server "..."`, or `evidence_mode: "mcp"` in API requests | The agent uses an external MCP server. The server may produce its own evidence, but the harness treats it as a generic tool backend. |

There are no bench-specific MCP submodes and no reference MCP server. To test a
tool server, pass its command through `--mcp-server` and label the tested
server explicitly:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION"
```

To compare tool backends, keep the model, provider, scenario set, timeout, and
cluster settings fixed, then change only `--mcp-server`.

```bash
# Baseline
bench-cli bench --scenario kubernetes --model sonnet --provider bifrost --reuse-cluster

# Same benchmark through the selected MCP server
bench-cli bench --scenario kubernetes --model sonnet --provider bifrost \
  --mcp-server "$MCP_SERVER" \
  --tool-server-id "$TOOL_SERVER_ID" \
  --tool-server-version "$TOOL_SERVER_VERSION" \
  --reuse-cluster

# Same benchmark through another MCP server command
bench-cli bench --scenario kubernetes --model sonnet --provider bifrost \
  --mcp-server "$OTHER_MCP_SERVER" \
  --tool-server-id "$OTHER_TOOL_SERVER_ID" \
  --tool-server-version "$OTHER_TOOL_SERVER_VERSION" \
  --reuse-cluster
```

Infrastructure verification is independent of `evidence_mode`. Scenario checks
always run; MCP-specific artifacts are interpreted only when a scenario or a
separate verifier explicitly points at an evidence directory.
