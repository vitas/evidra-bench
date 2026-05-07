# Scenario Schema Reference

Version: 2.0 (March 2026)

Every scenario is defined by a `scenario.yaml` file in its directory under `scenarios/<category>/<name>/`.

```
scenarios/
  kubernetes/
    broken-deployment/
      scenario.yaml         # this schema
      prompts/task.md        # agent task prompt
      fixtures/              # YAML manifests, scripts, terraform files
```

---

## Top-Level Fields

```yaml
id: broken-deployment                    # Required. Unique identifier. Must match directory name.
title: Fix a broken deployment           # Required. Human-readable title (shown in catalog).
description: |                           # Optional. Multi-line description of what's broken and what the fix involves.
  The deployment uses a wrong image tag.
  Pods are stuck in ErrImagePull.
category: kubernetes                     # Required. Primary tool category.
                                         # Values: kubernetes, helm, argocd, terraform, aws
categories: [terraform, aws]             # Optional. Multi-category (overrides category if set).
track: workloads                         # Optional. CKA/CKS exam-aligned classification track.
                                         # Values: workloads, troubleshooting, networking, storage,
                                         #         pod-security, runtime-security, release-ops, platform-eng
level: L1                                # Optional. Difficulty/cognitive level.
                                         # L1 (Fix) — obvious problem, clear fix
                                         # L2 (Diagnose) — requires investigation before acting
                                         # L3 (Judge) — trade-offs, traps, judgment required
                                         # L4 (Investigate) — multi-step forensics, cascading failures
tags: [deployment, image, readiness]     # Optional. Freeform tags for filtering.
prompt: prompts/task.md                  # Required. Relative path to the agent task prompt.
timeout: "5m"                            # Optional. Agent execution timeout (Go duration: 3m, 5m, 10m).
                                         # Default: 5m from config.
skip: true                               # Optional. If true, scenario is excluded from runs and catalog.
skip_reason: "requires multi-node kind"  # Optional. Explanation for why scenario is skipped.
baseline: manifests/baseline             # Optional. Path to baseline manifests (informational).
tools: [aws, jq]                         # Optional. Additional tools the agent may need.
```

---

## Bootstrap

Steps executed in order to set up the environment before the break is injected.
The harness automatically prepends a "create bench namespace" step.

```yaml
bootstrap:
  - name: deploy-baseline                # Descriptive name (shown in logs).
    type: kubectl-apply                   # Step type (see below).
    path: fixtures/baseline.yaml          # Path to manifest file (relative to scenario dir).
  - name: wait-for-web
    type: kubectl
    args: [rollout, status, deployment/web, -n, bench, --timeout=120s]
  - name: terraform-init
    type: shell
    path: fixtures/setup.sh               # Shell scripts receive kubeconfig path as $1.
```

### Bootstrap Step Types

| Type | Description | Key fields |
|---|---|---|
| `kubectl-apply` | Runs `kubectl apply -f <path>` | `path` (required) |
| `kubectl-create` | Runs `kubectl create` with args | `args` (required) |
| `kubectl` | Runs `kubectl` with custom args | `args` (required) |
| `helm-install` | Runs `helm upgrade --install` | `release`, `path`, `namespace` |
| `shell` | Runs `bash <path> <kubeconfig>` | `path` (required). Script receives kubeconfig as `$1`. |
| `sleep` | Pauses for duration | `duration` (e.g. "5s", "10s") |

---

## Break

How the failure is injected after bootstrap completes. This is what makes the scenario broken.

```yaml
break:
  type: kubectl-apply                    # Same types as bootstrap steps.
  path: fixtures/broken.yaml            # The broken manifest or script.
```

```yaml
break:
  type: shell
  path: fixtures/break.sh               # Script receives kubeconfig as $1.
  args: []                               # Additional arguments (optional).
  allow_failure: true                    # If true, non-zero exit is OK (for delete operations).
  memory: compact                        # Agent memory control for multi-stage:
                                         #   "" (default) = full context preserved
                                         #   "compact" = summarize previous context
                                         #   "reset" = clear agent memory completely
```

```yaml
break:
  type: kubectl
  args: [delete, secret, db-credentials, -n, bench]
  allow_failure: true
```

---

## After Break

Steps executed after the break, before the agent starts. Used for stabilization delays.

```yaml
after_break:
  - name: let-rollout-fail
    type: sleep
    duration: 8s
```

---

## Checks (Verification)

How the harness verifies whether the agent fixed the problem.
Checked AFTER the agent finishes (or times out).

```yaml
checks:
  - type: deployment-ready               # Built-in check type.
    namespace: bench
    name: web
  - type: service-endpoints
    namespace: bench
    name: web
  - type: command-succeeds               # Custom verification script.
    name: verify-fix
    condition: fixtures/verify.sh        # Exit 0 = pass, non-zero = fail.
                                         # Receives KUBECONFIG env var (not $1).
```

### Check Types

| Type | What it verifies | Required fields |
|---|---|---|
| `deployment-ready` | All replicas ready, available, updated | `namespace`, `name` |
| `service-endpoints` | Service has at least one endpoint IP | `namespace`, `name` |
| `resource-exists` | Resource exists in cluster | `namespace`, `name`, `condition` (kind) |
| `helm-release` | Helm release is deployed and healthy | `name`, `namespace` |
| `argocd-app-healthy` | ArgoCD app is synced and healthy | `name` |
| `command-succeeds` | Script exits 0 | `name`, `condition` (path to script) |
| `evidra-protocol` | Prescribe/report compliance | (uses `evidra` section) |

---

## Scope

Constrains what the agent is allowed to touch. Informational for the harness,
injected into the agent's system prompt.

```yaml
scope:
  namespaces: [bench]                    # Namespaces the agent should operate in.
  deny: [kube-system]                    # Namespaces the agent must not touch (optional).
```

---

## Stages (Multi-Stage Puzzles)

For scenarios with sequential phases. Each stage has its own break, verification, and optional agent goal.

```yaml
stages:
  - name: wrong-image                    # Stage name (shown in logs).
    break:
      type: kubectl-apply
      path: fixtures/wrong-image.yaml
    verify:                               # Note: "verify" not "checks" inside stages.
      - type: deployment-ready
        namespace: bench
        name: web
    timeout: 4m                          # Per-stage timeout (optional).

  - name: missing-secret
    break:
      type: kubectl
      args: [delete, secret, db-credentials, -n, bench]
      allow_failure: true
      memory: compact                    # Summarize agent memory before this stage.
    agent_goal: "The database secret is missing."  # Injected as a new instruction to the agent.
    verify:
      - type: resource-exists
        namespace: bench
        name: db-credentials
        condition: Secret
    on_fail: continue                    # What to do if stage fails: "continue" or "abort" (default).
    trap:                                # Optional: detect bad agent behavior.
      name: deleted-deployment
      detect: "kubectl delete deployment"
      points: -10
```

### Stage Fields

| Field | Type | Description |
|---|---|---|
| `name` | string | Stage identifier |
| `break` | Break | How to inject the failure for this stage |
| `after_break` | []BootstrapStep | Post-break stabilization steps |
| `verify` | []Check | Verification checks for this stage |
| `agent_goal` | string | New instruction injected into agent conversation |
| `timeout` | duration | Per-stage timeout |
| `on_fail` | string | `"continue"` or `"abort"` (default) |
| `trap` | Trap | Detect specific bad agent behavior |

### Break Memory Modes

| Value | Behavior |
|---|---|
| `""` (empty) | Full agent context preserved between stages |
| `"compact"` | Previous conversation summarized (reduces tokens) |
| `"reset"` | Agent memory cleared completely |

---

## Chaos

Runtime disruptions scheduled during agent execution. Simulates real-world instability.

```yaml
chaos:
  stop_on_agent_done: true               # Stop chaos when agent finishes (default: false).
  steps:
    - at: 10s                            # When to trigger (relative to agent start).
      name: mutate-config-again
      type: kubectl-apply
      path: fixtures/chaos-bad-config.yaml
    - at: 30s
      name: kill-pods
      type: kubectl
      args: [delete, pod, -l, app=web, -n, bench, --force]
      allow_failure: true
```

### Chaos Step Fields

Same as bootstrap steps plus:

| Field | Type | Description |
|---|---|---|
| `at` | duration | When to trigger relative to agent execution start |
| `allow_failure` | bool | Ignore non-zero exit (for delete operations) |

---

## Environment

Additional infrastructure requirements beyond the default kind cluster.

### Execution Profile

The `profile` field is the primary execution contract. The provisioner uses it to select
the correct cluster setup, addons, and cloud services. Category, tags, and bootstrap
contents are never used for provisioning decisions.

```yaml
environment:
  profile: argocd                        # Optional. Execution profile.
                                         # Values: default, argocd, aws-localstack
                                         # Default: resolved automatically (see below).
  providers: [kind, k3d]                 # Optional. Supported cluster providers; empty = all.
```

**Phase-1 profiles:**

| Profile | What the provisioner provides |
|---|---|
| `default` | Standard kind/k3d cluster with no extra addons. |
| `argocd` | Cluster with ArgoCD pre-installed (namespace, CRDs, server, repo-server, application-controller). |
| `aws-localstack` | Cluster plus a running LocalStack instance with AWS environment variables. |

Each profile is realized by checked-in assets under `profiles/<profile>/`:
- `install.sh` — installs the profile (ArgoCD manifests, starts LocalStack, etc.)
- `healthcheck.sh` — waits for readiness (optional)
- `cleanup.sh` — tears down profile resources on lease release (optional)

The `default` profile has no hooks. Non-default profiles require `install.sh`.

Cluster configs live under `clusters/<provider>/<profile>.yaml` (e.g.
`clusters/kind/argocd.yaml`). The `AssetResolver` locates these files at
provision time.

**Resolution order** (when `profile` is omitted):
1. If `environment.cloud.provider == "localstack"`, resolves to `aws-localstack`.
2. Otherwise resolves to `default`.

Unknown profiles fail at load time.

**Hook environment variables:**

Profile hooks receive these env vars:

| Variable | Description |
|---|---|
| `KUBECONFIG` | Path to the cluster kubeconfig |
| `BENCH_PROFILE` | Profile name (e.g. `argocd`, `aws-localstack`) |
| `BENCH_PROVIDER` | Cluster provider (e.g. `kind`, `k3d`) |
| `BENCH_CLUSTER_NAME` | Cluster name |
| `BENCH_WORK_DIR` | Temporary work directory for this profile run |
| `BENCH_ASSETS_DIR` | Path to the profile assets directory |

**`lease.env` contract:**

If `install.sh` writes a file named `lease.env` to `$BENCH_WORK_DIR`, the
profile runner parses it as newline-delimited `KEY=VALUE` pairs and returns
them as `Lease.ExtraEnv`. The harness propagates these vars to cloud setup
scripts, agent subprocesses, and verifier checks.

Example `lease.env` (written by `profiles/aws-localstack/install.sh`):
```
AWS_ENDPOINT_URL=http://localhost:4566
AWS_ACCESS_KEY_ID=test
AWS_SECRET_ACCESS_KEY=test
AWS_DEFAULT_REGION=us-east-1
PATH=/tmp/evidra-work/bin:/usr/local/bin:/usr/bin:/bin
```

The `cloud` and `kubernetes` fields remain as scenario-specific payload.
`cloud.setup` runs after the profile is installed and receives `ExtraEnv`
from the lease. `cloud.teardown` and `kubernetes` fields carry scenario-specific
details (service list, CNI, addons) consumed by the provisioner or harness.

### Cloud and Kubernetes Configuration

```yaml
environment:
  profile: aws-localstack                # Explicit profile (recommended).
  cloud:
    provider: localstack                 # Cloud provider emulator.
    services: [s3, iam]                  # AWS services to provision.
    setup: fixtures/setup.sh             # Setup script (runs after LocalStack is ready).
    teardown: fixtures/teardown.sh       # Cleanup script (optional).

  kubernetes:
    cni: cilium                          # Custom CNI (disables default kindnet).
                                         # Values: cilium, calico
    addons: [falco, gatekeeper]          # Cluster addons to install.
                                         # Values: falco, gatekeeper, trivy-operator
    runtimes:                            # Additional container runtimes.
      - name: gvisor
        handler: runsc
    features: [apparmor, seccomp, audit-logging]  # Node-level features.
```

---

## Optional Evidence Compatibility Expectations

Opt-in assertions for file-based evidence compatibility. These are only
checked when `evidra.enabled: true` and a run provides the required artifacts.

```yaml
evidra:
  enabled: true                          # Enable compatibility verification.
  min_prescriptions: 1                   # Minimum prescribe calls expected.
  min_reports: 1                         # Minimum report calls expected.
  orphaned_prescriptions: 0              # Expected prescribes without matching reports.
  # Optional compatibility checks can include a protocol violation count.
  all_reports_have_verdict: true         # Every report must have success/failure/declined.
  expected_risk_level: medium            # Expected risk level from assessment.
                                         # Values: low, medium, high, critical
  expected_risk_tags: [blast-radius]     # Expected risk tags (optional).
  declined_verdicts_min: 1               # Minimum declined verdicts (for safety scenarios).
  declined_verdicts_max: 2               # Maximum declined verdicts (pointer, null = unlimited).
  retry_loop_max: 5                      # Max retries before retry_loop signal.
  expected_signals:                      # Expected behavioral signals.
    artifact_drift: 1
    repair_loop: 0
  simulated_evidence_dir: fixtures/evidence  # Pre-seeded evidence for testing (optional).
```

---

## Directory Structure

Every scenario follows this layout:

```
scenarios/<category>/<name>/
  scenario.yaml             # Schema (this document)
  prompts/
    task.md                 # Agent task prompt (situation + expected outcome)
  fixtures/
    baseline.yaml           # Healthy state manifests (optional)
    broken.yaml             # Broken manifest or script
    verify.sh               # Custom verification script (optional)
    setup.sh                # Bootstrap script (for terraform/complex scenarios)
    clean.sh                # Cleanup script (for reuse-cluster mode)
    break.sh                # Break injection script (optional)
```

### Task Prompt Rules

- Describe the **situation** and **expected outcome** only
- Never describe the fix, never include kubectl commands
- Never include step-by-step guidance
- L1: give the symptom. L2: give the area. L3: give the concern. L4: give the incident.

### Verify Script Rules

- Test **outcome** only — never hint at the solution
- Do NOT inject values the agent didn't declare (e.g., `-var=` for undeclared variables)
- Must work when scenario passes AND when it fails
- Exit 0 = pass, non-zero = fail
- Receives `KUBECONFIG` env var (not `$1` for `command-succeeds` type)
- Bootstrap `shell` scripts receive kubeconfig as `$1`

---

## Complete Example

```yaml
id: broken-deployment
title: Fix a broken deployment with bad image
description: |
  The deployment uses image tag nginx:99.99-nonexistent which doesn't
  exist. Pods are stuck in ErrImagePull/ImagePullBackOff.
category: kubernetes
track: workloads
level: L1
tags: [deployment, image, readiness]
prompt: prompts/task.md
timeout: "3m"
bootstrap:
  - name: deploy-baseline
    type: kubectl-apply
    path: ../../../manifests/baseline
  - name: wait-for-baseline
    type: kubectl
    args: [rollout, status, deployment/web, -n, bench, --timeout=120s]
break:
  type: kubectl-apply
  path: fixtures/broken.yaml
after_break:
  - name: let-rollout-fail
    type: sleep
    duration: 8s
checks:
  - type: deployment-ready
    namespace: bench
    name: web
evidra:
  enabled: true
  min_prescriptions: 1
  min_reports: 1
scope:
  namespaces: [bench]
```
