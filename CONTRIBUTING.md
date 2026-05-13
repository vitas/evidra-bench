# Contributing

Evidra Bench is a live regression testing system for infrastructure agents,
MCP servers, skills, and external agent protocols. Contributions are welcome
when they improve repeatable evaluation, scenario quality, reporting, or
operator safety.

## Development Setup

Install the local prerequisites used by the scenario runner:

- Go 1.25+
- `kubectl`
- `kind` or `k3d`
- `helm`
- Node.js 22+ for the UI

Common commands:

```bash
make build
make test
make lint
make ui-build
```

Before opening a pull request, run the relevant tests for the area you touched
and always run `make lint`.

## Commit Sign-Off

Use Developer Certificate of Origin sign-off for commits:

```bash
git commit -s -m "area: concise change summary"
```

The sign-off means you have the right to submit the contribution under the
project license.

## Scenario Contributions

Good scenarios test capability, not command memorization.

- Prefer real infrastructure state over mocked transcripts.
- Keep the task open-ended: the agent may choose any valid remediation path.
- Verify final state declaratively.
- Add failure-analysis hints only for post-run scoring; do not leak the answer
  into the agent prompt.
- Avoid credentials, private customer data, and transcripts from private runs.

For larger additions, open a scenario proposal issue first.

## Data Hygiene

Do not commit private run artifacts, transcripts, API keys, database dumps, or
customer-specific incident details. Use sanitized fixtures and public scenario
data only.
