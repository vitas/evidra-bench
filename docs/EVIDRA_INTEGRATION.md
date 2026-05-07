# Evidra Integration

`evidra-bench` reports benchmark results to the Evidra API, but it no longer
links against or shells out to the core `evidra` repo.

## MCP Servers

All MCP servers are configured through the same generic flag:

```bash
bench-cli run \
  --scenario kubernetes/broken-deployment \
  --provider bifrost \
  --model sonnet \
  --mcp-server "evidra-mcp --signing-mode optional"
```

The harness does not auto-start or auto-build `evidra-mcp`. Install it in the
runner environment the same way you would install any other MCP server binary.

## Evidence Modes

API and stored run records use only:

- `none` for baseline runs
- `mcp` for runs that use an MCP server

The trigger endpoint accepts `evidence_mode` values `none` and `mcp`.

## Protocol Evidence Checks

Some scenarios still declare `evidra:` protocol expectations. Those checks read
JSONL evidence from `<evidence-dir>/segments/*.jsonl` only when a run explicitly
sets an evidence directory:

```bash
bench-cli run \
  --scenario kubernetes/privileged-pod-review \
  --provider bifrost \
  --model sonnet \
  --mcp-server "evidra-mcp --signing-mode optional" \
  --evidence-dir ./runs/evidence
```

This keeps protocol verification file-based and explicit. Normal infrastructure
checks always run regardless of evidence mode.

## Bench Job Contracts

This repo owns the benchmark API/control-plane surface used by the bench UI and
remote runners:

- [Bench API Reference](BENCH_API_REFERENCE.md)
- [Executor Contract v1.0.0](contracts/EXECUTOR_CONTRACT_V1.md)
- [Bench Runner Control Plane Contract v1](contracts/BENCH_RUNNER_CONTROL_PLANE_V1.md)
- [Bench Service Setup](guides/bench-service-setup.md)
