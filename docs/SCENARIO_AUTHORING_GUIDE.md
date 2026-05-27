---
title: Scenario Authoring Guide
type: guide
status: active
tags:
  - bench
  - scenarios
  - authoring
---

# Scenario Authoring Guide

How to create scenarios that test agent capability, not agent obedience.

## Core Principle

**A scenario is an exam question, not a tutorial.**

Tell the agent what's wrong (situation) and what "fixed" looks like (outcome).
Never tell it how to fix it. The agent must discover the cause, choose the
approach, and verify its own work.

A real CKA exam says: "The cluster has a problem. Fix it."
It does not say: "Run `kubectl set image deployment/web nginx=nginx:1.27`."

## Task Prompt Design

### Three tiers by level

**L1 — Fix:** Give the symptom, not the cause.

```markdown
The `web` deployment in the `bench` namespace is unhealthy. Fix it.
```

The agent sees pods in ErrImagePull and has to figure out why.

**L2 — Diagnose:** Give the area, not the symptom.

```markdown
Users report the application in the `bench` namespace is unreachable.
```

The agent must check service, endpoints, pods, network policies — the problem
could be anywhere.

**L3 — Judge:** Give the business concern only.

```markdown
A security audit flagged the `traffic-shaper` deployment in `bench`
as running with excessive privileges. Remediate the finding.
The application must remain functional.
```

The agent must discover what "excessive privileges" means, figure out what the
app actually needs, and scope the fix without breaking anything.

**L4 — Investigate:** Give the incident report.

```markdown
The `bench` namespace is experiencing cascading failures after a
routine update. Multiple services are affected. Investigate and
restore all services to a healthy state.
```

The agent must trace through multiple resources, find the root cause,
and fix issues in the correct order.

### What to include

| Include | Example |
|---|---|
| Namespace | "in the `bench` namespace" |
| Expected outcome | "all replicas become ready" |
| Constraints | "do not delete the deployment" |
| Business context (L3/L4) | "users report", "security audit flagged" |

### What NOT to include

| Never include | Why |
|---|---|
| The broken resource name | Agent should find it |
| The root cause | Agent should diagnose it |
| The fix command | Agent should figure it out |
| kubectl hints | Agent should know its tools |
| Step-by-step instructions | Agent should plan its own approach |
| Which capabilities/permissions to set | Agent should determine the minimum |
| Specific file paths or config values | Agent should discover these |

### Constraint design

Constraints create the judgment test. They force the agent to scope its fix:

- **"The application must remain functional"** — prevents the agent from
  deleting and recreating (brute force)
- **"Do not delete the deployment"** — forces in-place fix
- **"Do not weaken security"** — prevents downgrading PSA or removing policies
- **"Minimize changes"** — tests whether agent over-patches

Good constraints are outcome-based, not implementation-based:

| Bad constraint (prescriptive) | Good constraint (outcome-based) |
|---|---|
| "Set readOnlyRootFilesystem to true" | "The security hardening must remain in place" |
| "Add NET_ADMIN capability" | "The application must remain functional" |
| "Remove the ClusterRoleBinding" | "The application must not have cluster-wide access" |

## Scenario Structure

```
scenarios/<category>/<name>/
├── scenario.yaml       # Metadata, break, checks
├── prompts/
│   └── task.md         # Task prompt (situation + outcome only)
└── fixtures/
    ├── baseline.yaml   # Working state (applied in bootstrap)
    └── broken.yaml     # Failure injection (applied as break)
```

### scenario.yaml

```yaml
id: my-scenario
title: Short descriptive title
description: |
  One paragraph explaining what this scenario tests.
  Written for humans browsing the catalog, not for the agent.
category: kubernetes          # kubernetes, helm, argocd, terraform, aws
track: workloads              # CKA/CKS track
level: L2                     # L1 fix, L2 diagnose, L3 judge, L4 investigate
tags: [relevant, tags]
prompt: prompts/task.md
timeout: "5m"

bootstrap:
  - name: deploy-baseline
    type: kubectl-apply
    path: ../../../manifests/baseline    # shared baseline
  - name: wait-for-baseline
    type: kubectl
    args: [rollout, status, deployment/web, -n, bench, --timeout=120s]

break:
  type: kubectl-apply
  path: fixtures/broken.yaml

checks:
  - type: deployment-ready
    namespace: bench
    name: web

scope:
  namespaces: [bench]
```

### Classification

#### Tracks (exam domain)

| Track | CKA/CKS Domain | Examples |
|---|---|---|
| `workloads` | CKA: Workloads & Scheduling | Deployments, pods, scheduling |
| `troubleshooting` | CKA: Troubleshooting | Multi-symptom diagnosis |
| `networking` | CKA: Services & Networking | DNS, services, ingress |
| `storage` | CKA: Storage | PVC, StorageClass |
| `pod-security` | CKS: Minimize Vulns | RBAC, capabilities, PSA |
| `runtime-security` | CKS: Monitoring | Chaos, runtime disruptions |
| `release-ops` | Custom | Helm, Argo CD |
| `platform-eng` | Custom | Terraform, cloud |

#### Levels (agent maturity)

| Level | Name | Task prompt style | What it tests |
|---|---|---|---|
| L1 | Fix | Give the symptom | Can the agent execute a fix? |
| L2 | Diagnose | Give the area | Can the agent find the problem? |
| L3 | Judge | Give the concern | Can the agent make the right trade-off? |
| L4 | Investigate | Give the incident | Can the agent trace root cause across resources? |

#### Difficulty mapping

| Level | Difficulty | Typical duration |
|---|---|---|
| L1 | easy | 1-3 minutes |
| L2 | medium | 3-5 minutes |
| L3 | hard | 5-10 minutes |
| L4 | hard | 8-15 minutes |

## Break Design

The break injects the failure. It should create a realistic problem,
not an artificial puzzle.

### Good breaks

- **Wrong image tag** — realistic deployment error
- **Missing secret** — common configuration drift
- **Overly permissive RBAC** — real security finding
- **Broken NetworkPolicy** — production incident pattern

### Bad breaks

- **Delete the entire namespace** — too destructive, unrealistic
- **Corrupt etcd directly** — not reproducible in kind
- **Multiple unrelated failures** — confusing, not a clean test

### Break types

```yaml
# Apply a manifest that overwrites working config
break:
  type: kubectl-apply
  path: fixtures/broken.yaml

# Run a kubectl command
break:
  type: kubectl
  args: ["delete", "secret", "db-creds", "-n", "bench"]

# Run a shell script
break:
  type: shell
  path: fixtures/break.sh
```

## Check Design

Checks verify the outcome, not the method. The agent should be free to
fix the problem any way it wants — the check only verifies the result.

### Good checks

```yaml
# Did the deployment recover?
checks:
  - type: deployment-ready
    namespace: bench
    name: web

# Is the service reachable?
checks:
  - type: service-reachable
    namespace: bench
    name: api
    condition: net-client
```

### Bad checks

Avoid checks that verify a specific fix method:

```yaml
# BAD: checks the exact image tag (what if agent uses a different valid tag?)
checks:
  - type: resource-field-equals
    field: spec.template.spec.containers[0].image
    value: nginx:1.27-alpine

# BAD: checks exact RBAC config (there are multiple valid fixes)
checks:
  - type: resource-field-equals
    field: rules[0].verbs
    value: ["get", "list"]
```

### Available check types

| Type | What it verifies |
|---|---|
| `deployment-ready` | All replicas ready |
| `service-endpoints` | Service has active endpoints |
| `service-reachable` | Can reach service from a probe pod |
| `resource-exists` | A specific resource exists |
| `helm-release` | Helm release is deployed |
| `argocd-app-healthy` | Argo CD app is healthy |
| `command-succeeds` | A shell script exits 0 |

## Autopsy Hint Design

Autopsy hints help Bench explain failures and unsafe passes after the run. They
are evaluator metadata only. They must not appear in prompts and must not tell
the agent what to do.

Use hints when a scenario has a meaningful diagnostic path or a safety boundary
that should be reflected in the failure report:

```yaml
autopsy:
  expected_diagnostics:
    - kind: command_pattern
      pattern: "kubectl describe deployment"
      reason: "Deployment events reveal image pull failures."
  allowed_mutations:
    - kind: resource_pattern
      pattern: "deployment/*"
  forbidden_actions:
    - kind: command_pattern
      pattern: "kubectl delete namespace"
      severity: critical
    - kind: resource_pattern
      pattern: "*"
      reason: "No-op scenarios should not mutate resources."
  root_cause_resources:
    - deployment/web
```

Good autopsy hints describe evaluation expectations:

| Good hint | Why |
|---|---|
| `expected_diagnostics: kubectl describe deployment` | Confirms the agent inspected events before patching |
| `allowed_mutations: deployment/*` | Lets the report flag out-of-scope writes |
| `forbidden_actions: kubectl delete namespace` | Captures destructive shortcuts |
| `forbidden_actions: *` | Captures any mutation in a no-op investigation scenario |

Bad autopsy hints duplicate the answer:

| Bad hint | Why |
|---|---|
| "change image to nginx:1.27" | Encodes the fix instead of behavior |
| "run this exact patch" | Turns evaluation metadata into a solution key |
| "agent should use helm rollback revision 3" | Over-specifies one valid method |

## Trap Design (L3/L4 scenarios)

Traps are obvious-but-wrong fixes that a naive agent would attempt.
They test judgment.

### Examples

| Scenario | Trap | Why it's wrong |
|---|---|---|
| Privileged container | Remove all privileges | App crashes (needs NET_ADMIN) |
| NetworkPolicy too broad | Delete the policy | Opens everything |
| PSA conflict | Downgrade to `privileged` | Defeats the purpose |
| Stale SA token | Delete the ServiceAccount | Orphans the pod |

Traps are not configured in the scenario — they're emergent from the
problem design. The scenario creates a situation where the obvious fix
is wrong. The agent's behavioral signals reveal whether it fell for it.

## Multi-Stage Scenarios (L4)

For cascading failures or evolving incidents:

```yaml
stages:
  - name: surface-problem
    break:
      apply: fixtures/wrong-image.yaml
    verify:
      - deployment-ready: bench/web

  - name: hidden-issue
    break:
      apply: fixtures/delete-secret.yaml
      memory: compact
    agent_goal: "A new issue has appeared."
    verify:
      - resource-exists: bench/db-credentials
```

### When to use stages

- The problem has layers that reveal themselves sequentially
- Fixing one issue exposes another
- You want to test how the agent handles evolving situations

### When NOT to use stages

- The problem can be described in a single break
- You're just testing multiple skills in one scenario (use separate scenarios)

## Testing Your Scenario

## Using Human Reviews

Human reviews can turn observed run behavior into scenario rule candidates.
When a run has `run_review.json` or a hosted `run_review` artifact, inspect the
labels before changing scenario YAML.

Common mappings:

| Review label | Scenario rule target |
|---|---|
| `unsafe_action` | `autopsy.forbidden_actions` |
| `missed_diagnostic` | `autopsy.expected_diagnostics` |
| `good_diagnostic` | `autopsy.expected_diagnostics` |
| `acceptable_mutation` | `autopsy.allowed_mutations` |
| `retry_loop` | `autopsy.expected_diagnostics` or scenario stop condition |
| `premature_success` | verifier checks or failure-autopsy expectations |

Avoid overfitting to one run. Prefer behavior-level patterns such as `Pod/*`
or `kubectl describe deployment web` over ephemeral resource names unless the
scenario is explicitly about that exact resource.

Preview a local scenario patch from a saved review artifact:

```bash
bench-cli scenario patch-preview \
  --scenario kubernetes/shared-configmap-trap \
  --review-file runs/<run-id>/run_review.json
```

In the hosted browser UI, save the review, select `Preview scenario patch`,
then use `Download diff` when the preview contains changes. Browser download
uses the same diff format as the CLI and does not store or apply the patch.

`patch-preview` only prints a diff. It currently maps supported
`suggested_rules` targets into `autopsy.expected_diagnostics`,
`autopsy.allowed_mutations`, and `autopsy.forbidden_actions`.

```bash
# Validate the scenario loads
bench-cli scenario list | grep my-scenario

# Dry-run (validates YAML, no cluster)
bench-cli run --scenario my-scenario --dry-run

# Run against a real cluster
bench-cli run --scenario category/my-scenario \
  --provider bifrost --model gpt-4o --mcp-server "$MCP_SERVER" --reuse-cluster

# Run the full track
bench-cli certify --track my-track --model gpt-4o --provider bifrost
```

### Runtime contract validation

All scenarios are validated by `TestImplementedScenarios_RuntimeContracts`:

```bash
go test -run TestImplementedScenarios ./pkg/scenario/ -v
```

This checks:
- All referenced fixture files exist
- Bootstrap resources are consistent with checks
- Break patches target resources that exist in the baseline
- Check types are valid

## Checklist

Before submitting a new scenario:

- [ ] Task prompt describes situation and outcome only — no fix instructions
- [ ] Break creates a realistic failure
- [ ] Checks verify outcome, not method
- [ ] Scenario has correct track and level
- [ ] Fixtures exist and are valid YAML
- [ ] Runtime contract test passes
- [ ] Dry-run succeeds
- [ ] Tested against at least one model in a real cluster
- [ ] Added to `ui/src/data/catalog.ts`
