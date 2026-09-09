# fake-ollama

A deterministic, dependency-free stand-in for the [Ollama](https://ollama.com)
HTTP API, used by `tests/smoke/run_docker_local_model_smoke.sh` and the CI
local-model job. It exists so first-class local-model workflows can be
verified without installing Ollama, downloading a model, needing a GPU, or
holding any provider credential.

## Endpoints

| Route                     | Behavior                                                            |
| ------------------------- | ------------------------------------------------------------------- |
| `GET  /api/tags`          | one installed model: `qwen3-fake:1b` (digest, 1.1B, Q4_K_M)         |
| `POST /api/show`          | metadata declaring capabilities `["completion", "tools"]`           |
| `POST /v1/chat/completions` | scripted agent behavior (see below)                               |
| anything else             | logged as `VIOLATION ...` and answered with HTTP 500                |

`/api/pull` is deliberately **not** implemented: any pull attempt shows up in
the log as a contract violation and fails the smoke.

## Scripted behavior

* A request whose `tools` include `evidra_capability_probe` gets the exact
  probe tool call back (satisfies the behavioral probe).
* Otherwise the scenario id is detected from the messages and the demo-suite
  remediation script runs through `run_command` calls, one per `tool` result
  already present in the conversation, then finishes with plain text.

## Running

```bash
go run ./tests/fixtures/fake-ollama -addr 127.0.0.1:11434
```

The runner container must reach this address at the constant Ollama local
endpoint (`127.0.0.1:11434`), which the smokes do with Docker host
networking on Linux.
