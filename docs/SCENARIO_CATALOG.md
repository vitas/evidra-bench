# Scenario Catalog

Coverage map for infra-bench scenarios, grouped by tool and failure category.

**Legend:**
- Implemented — scenario exists and loads
- Planned — defined in design docs, not yet implemented
- Gap — identified need, no scenario designed yet

---

## Coverage Summary

| Tool       | Implemented | Planned | Total |
|------------|:-----------:|:-------:|:-----:|
| kubectl    | 1           | 8       | 9     |
| Helm       | 1           | 3       | 4     |
| Argo CD    | 1           | 3       | 4     |
| Cross-tool | 0           | 4       | 4     |
| **Total**  | **3**       | **18**  | **21**|

---

## kubectl Scenarios

Source: kagent benchmark (24 scenarios), design doc, brainstorming research.

| ID   | Scenario                          | Category       | Difficulty | Status      | Signals                          |
|------|-----------------------------------|----------------|:----------:|-------------|----------------------------------|
| K01  | Deployment with wrong image tag   | Deployment     | easy       | Implemented | retry_loop, scope_adherence      |
| K02  | Pod stuck in CrashLoopBackOff     | Pod            | medium     | Planned     | drift_from_plan, retry_loop      |
| K03  | Service with wrong selector       | Service        | easy       | Planned     | partial_application              |
| K04  | ResourceQuota exceeded            | Pod            | medium     | Planned     | scope_adherence, rollback        |
| K05  | Missing ConfigMap key             | ConfigMap      | easy       | Planned     | scope_adherence                  |
| K06  | Wrong liveness/readiness probe    | Deployment     | medium     | Planned     | retry_loop, blast_radius         |
| K07  | Secret not mounted                | Secret         | easy       | Planned     | scope_adherence                  |
| K08  | NetworkPolicy blocking traffic    | NetworkPolicy  | hard       | Planned     | blast_radius, new_scope          |
| K09  | PVC with wrong storage class      | PVC            | medium     | Planned     | scope_adherence                  |

### kagent categories not yet mapped

These are from kagent's 24-scenario benchmark. They should be reviewed and translated:

| kagent Category    | Scenarios                                                     | Priority |
|--------------------|---------------------------------------------------------------|----------|
| ConfigMap          | missing key, misspelled name, read-only filesystem            | high     |
| Deployment         | env mismatch, impossible affinity, low resources, wrong affinity key, probe failures, scaled to 0 | high |
| Service            | missing selector, no endpoint, DNS resolution, wrong port     | high     |
| Pod                | host port conflict, limit range exceeded, resource quota, security context | medium |
| PVC                | wrong access mode, wrong storage class                        | medium   |
| Secret             | missing, not mounted                                          | medium   |
| ServiceAccount     | misspelled, wrong permissions                                 | medium   |
| NetworkPolicy      | blocking traffic                                              | low      |

---

## Helm Scenarios

| ID   | Scenario                          | Difficulty | Status      | Signals                          |
|------|-----------------------------------|:----------:|-------------|----------------------------------|
| H01  | Failed upgrade (bad values)       | medium     | Implemented | rollback_behavior, retry_loop    |
| H02  | Pending release (stuck hooks)     | hard       | Planned     | scope_adherence, escalation      |
| H03  | Version downgrade / rollback      | medium     | Planned     | drift_from_plan, rollback        |
| H04  | Chart dependency conflict         | hard       | Planned     | multi_step_sequence              |

---

## Argo CD Scenarios

| ID   | Scenario                          | Difficulty | Status      | Signals                          |
|------|-----------------------------------|:----------:|-------------|----------------------------------|
| A01  | App out of sync (cluster drift)   | medium     | Implemented | drift_from_plan, scope_adherence |
| A02  | Sync failed (invalid manifest)    | medium     | Planned     | retry_loop, partial_application  |
| A03  | App degraded after sync           | hard       | Planned     | rollback_behavior                |
| A04  | Sync wave ordering broken         | hard       | Planned     | multi_step_sequence              |

---

## Cross-cutting Scenarios

These test agent safety and judgment, not tool-specific remediation.

| ID   | Scenario                          | Difficulty   | Status  | Signals                          |
|------|-----------------------------------|:------------:|---------|----------------------------------|
| X01  | Delete production namespace       | catastrophic | Planned | declined_decision, safety        |
| X02  | Network partition during rollout  | hard         | Planned | retry_loop, rollback_behavior    |
| X03  | Multiple concurrent failures      | hard         | Planned | scope_adherence, escalation      |
| X04  | Ambiguous instructions            | medium       | Planned | declined_decision, safety        |

---

## Signal Coverage Matrix

Which signals are exercised by which scenario families.

| Signal               | kubectl | Helm | Argo CD | Cross-tool |
|----------------------|:-------:|:----:|:-------:|:----------:|
| retry_loop           | K01,K02,K06 | H01 | A02 | X02    |
| protocol_violation   | —       | —    | —       | —          |
| artifact_drift       | —       | —    | —       | —          |
| blast_radius         | K06,K08 | —    | —       | —          |
| new_scope            | K08     | —    | —       | —          |
| scope_adherence      | K01,K04,K05,K07,K09 | H02 | A01 | X03 |
| rollback_behavior    | —       | H01,H03 | A03 | X02     |
| declined_decision    | —       | —    | —       | X01,X04    |
| partial_application  | K03     | —    | A02     | —          |
| drift_from_plan      | K02     | H03  | A01     | —          |

### Gaps

- **protocol_violation** — no scenario exercises this. Need a scenario where the agent skips prescribe/report or executes without approval.
- **artifact_drift** — no scenario exercises this. Need a scenario where the agent modifies the artifact between prescribe and execute.
- **new_scope** — only K08 (NetworkPolicy). Need more scenarios testing unauthorized namespace/resource access.

---

## Evidra Benchmark (home repo) Relationship

The evidra home repo (`../evidra-benchmark`) has complementary benchmark data:

| System                       | What it tests                    | Format              | Overlap |
|------------------------------|----------------------------------|---------------------|---------|
| `tests/benchmark/cases/`     | Risk classification accuracy     | expected.json + artifact ref | None — static analysis |
| `experiments/results/`       | MCP protocol compliance          | result.json + summary.jsonl  | Signals overlap       |
| **This repo** (`scenarios/`) | Agent remediation capability     | scenario.yaml + fixtures     | None — different axis  |

Together they cover: classification accuracy + protocol compliance + remediation capability.

---

## Implementation Priority

### Phase 1 (current — done)
K01, H01, A01 — one per tool family, easy/medium difficulty.

### Phase 2 (next)
K02, K03, K05, K07 — kagent-equivalent kubectl basics. These are the highest-value additions because they cover the most common failure modes.

### Phase 3
K04, K06, H02, A02 — medium difficulty, start exercising more signals.

### Phase 4
K08, K09, H03, H04, A03, A04 — hard scenarios, full signal coverage.

### Phase 5
X01–X04 — cross-cutting safety and judgment scenarios. These require the most design work.
