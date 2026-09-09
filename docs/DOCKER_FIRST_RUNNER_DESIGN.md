# Docker-First One-Command Runner Design

**Date:** 2026-09-09  
**Status:** Approved implementation design  
**Related direction:** Evidra Bench Relaunch Focus: One-Command Testing

## Goal

A developer with Docker installed can run the supported Evidra runner image,
test a model or external agent against the bundled Kubernetes starter suite,
receive local terminal, HTML, and JSON results, and leave no test cluster
behind.

The container is a thin distribution of the existing CLI and evaluation
engine. It must not introduce a Docker-specific runner, scenario format,
provider client, result model, or cleanup path.

## Selected approach

Evolve the existing `Dockerfile.bench` into the one public runner image.

The alternatives were rejected:

- A second simplified image would create a parallel product and packaging
  stack.
- Docker-in-Docker would be heavier and would diverge from the existing
  kind/k3d lifecycle.

The runner uses the host Docker socket to create kind or k3d sibling
containers. This preserves the same environment providers used by native CLI
runs.

## Image contract

The image contains:

- the CLI installed as `evidra`;
- a `bench-cli` compatibility link for existing automation;
- Docker CLI, `kubectl`, `kind`, and `k3d`;
- versioned suites and the scenario, manifest, cluster, and profile assets
  referenced by them.

The container has a stable internal asset root. Simple commands resolve that
root automatically; users do not provide or learn `--project-root`.

The default entry point is `evidra`. With no command, the container prints CLI
help. The hosted `serve` command remains available explicitly but is no longer
the default behavior of the public runner image.

Typical execution:

```bash
docker run --rm \
  -v /var/run/docker.sock:/var/run/docker.sock \
  -v "$PWD/evidra-results:/workspace/evidra-results" \
  -e OPENAI_API_KEY \
  ghcr.io/vitas/evidra-bench:latest \
  test --model openai/gpt-5
```

## Asset resolution

Asset discovery belongs above the evaluation service. Resolution order for the
simple command is:

1. an explicit internal/test override;
2. an `EVIDRA_ASSETS_DIR` runtime override;
3. the image root `/opt/evidra` when its suite manifest exists;
4. the current directory for native source-tree development.

The resolved path is normalized before suite loading and disappears from the
canonical evaluation plan. Suite digesting and traversal protection remain in
the existing shared loader.

## Execution and data flow

```text
docker invocation
  -> evidra test argument/config resolution
  -> bundled suite resolution
  -> shared evaluation service
  -> existing kind or k3d provider via host Docker socket
  -> existing harness, adapter/provider, verifier, and cleanup
  -> canonical EvaluationResult
  -> terminal + standalone HTML + JSON
  -> bind-mounted /workspace/evidra-results
```

Provider credentials and raw endpoints stay in runtime-only provider
configuration. They are not written into the plan or report.

## Failure behavior

Before cluster creation, preflight verifies:

- Docker CLI exists and the mounted daemon is reachable;
- the selected kind/k3d executable exists;
- bundled suite assets resolve and validate;
- model configuration and credentials are sufficient.

Behavioral failures and unsafe behavior exit `1`. Setup, execution, cleanup,
and report-generation failures exit `2`. Cleanup continues through the shared
evaluation lease path.

## Compatibility

Advanced commands remain in the same binary. Existing users may continue to
invoke `bench-cli`; new documentation and image examples use `evidra`.

The existing image repository may retain compatibility tags during migration,
but the intended relaunch name is `ghcr.io/vitas/evidra-bench`.

## Verification

Automated verification covers:

- deterministic asset-root selection without exposing it as a public flag;
- root command naming and no-command help behavior;
- Dockerfile contents and installed tool contract;
- both `evidra` and `bench-cli` executable names;
- repository tests, image build, and repository linter;
- a container smoke test that uses a scripted external agent and no paid
  inference when the local Docker environment permits it;
- successful report persistence and absence of leaked kind/k3d clusters.
