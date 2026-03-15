# Scenario Catalog

Coverage map for infra-bench scenarios, grouped by tool and failure category.

**Legend:**
- Implemented — scenario exists and loads
- Planned — defined in design docs, not yet implemented
- Gap — identified need, no scenario designed yet

---

## Coverage Summary

| Tool       | Implemented | With Evidra | Planned | Total |
|------------|:-----------:|:-----------:|:-------:|:-----:|
| kubectl    | 15          | 8           | 0       | 15    |
| Helm       | 4           | 1           | 0       | 4     |
| Argo CD    | 4           | 0           | 0       | 4     |
| Cross-tool | 0           | 0           | 4       | 4     |
| **Total**  | **23**      | **9**       | **4**   | **27**|

---

## kubectl Scenarios

Source: kagent benchmark (24 scenarios), design doc, brainstorming research.

| ID   | Scenario                             | Category       | Difficulty | Status      | Signals                                 | Evidra |
|------|--------------------------------------|----------------|:----------:|-------------|-----------------------------------------|:------:|
| K01  | Deployment with wrong image tag      | Deployment     | easy       | Implemented | retry_loop, scope_adherence             | ✓      |
| K02  | Pod stuck in CrashLoopBackOff        | Pod            | medium     | Implemented | drift_from_plan, retry_loop             | ✓      |
| K03  | Service with wrong selector          | Service        | easy       | Implemented | partial_application                     | —      |
| K04  | ResourceQuota exceeded               | Pod            | medium     | Implemented | scope_adherence, rollback               | —      |
| K05  | Missing ConfigMap reference          | ConfigMap      | easy       | Implemented | scope_adherence                         | —      |
| K06  | Wrong liveness/readiness probe       | Deployment     | medium     | Implemented | retry_loop, blast_radius                | —      |
| K07  | Secret not mounted                   | Secret         | easy       | Implemented | scope_adherence                         | —      |
| K08  | NetworkPolicy blocking traffic       | NetworkPolicy  | hard       | Implemented | blast_radius, new_scope                 | —      |
| K09  | PVC with wrong storage class         | PVC            | medium     | Implemented | scope_adherence                         | —      |
| K10  | Privileged pod review (risk/decline) | Security       | medium     | Implemented | protocol_violation, declined_decision   | ✓      |
| K11  | ConfigMap content drift               | ConfigMap      | medium     | Implemented | artifact_drift                          | ✓      |
| K12  | Cascading misconfiguration (repair)  | Deployment     | hard       | Implemented | repair_loop                             | ✓      |
| K13  | Impossible scheduling (thrashing)    | Scheduling     | hard       | Implemented | thrashing                               | ✓      |
| K14  | Pod kill during repair               | Chaos          | medium     | Implemented | retry_loop, repair_loop                 | ✓      |
| K15  | Config mutation mid-fix              | Chaos          | hard       | Implemented | artifact_drift, thrashing               | ✓      |

### kagent categories not yet mapped

These are from kagent's 24-scenario benchmark. They can extend existing categories:

| kagent Category    | Scenarios not yet covered                                     | Priority |
|--------------------|---------------------------------------------------------------|----------|
| ConfigMap          | misspelled name, read-only filesystem                         | medium   |
| Deployment         | env mismatch, impossible affinity, wrong affinity key, scaled to 0 | medium |
| Service            | no endpoint, DNS resolution, wrong port                       | medium   |
| Pod                | host port conflict, limit range exceeded, security context    | low      |
| PVC                | wrong access mode                                             | low      |
| ServiceAccount     | misspelled, wrong permissions                                 | low      |

---

## Helm Scenarios

| ID   | Scenario                          | Difficulty | Status      | Signals                          | Evidra |
|------|-----------------------------------|:----------:|-------------|----------------------------------|:------:|
| H01  | Failed upgrade (bad values)       | medium     | Implemented | rollback_behavior, retry_loop    | ✓      |
| H02  | Pending release (stuck hooks)     | hard       | Implemented | scope_adherence, escalation      | —      |
| H03  | Version downgrade / rollback      | medium     | Implemented | drift_from_plan, rollback        | —      |
| H04  | Chart dependency conflict         | hard       | Implemented | multi_step_sequence              | —      |

---

## Argo CD Scenarios

| ID   | Scenario                          | Difficulty | Status      | Signals                          |
|------|-----------------------------------|:----------:|-------------|----------------------------------|
| A01  | App out of sync (cluster drift)   | medium     | Implemented | drift_from_plan, scope_adherence |
| A02  | Sync failed (invalid manifest)    | medium     | Implemented | retry_loop, partial_application  |
| A03  | App degraded after sync           | hard       | Implemented | rollback_behavior                |
| A04  | Sync wave ordering broken         | hard       | Implemented | multi_step_sequence              |

---

## Cross-cutting Scenarios (Phase 5 — planned)

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
| retry_loop           | K01,K02,K06,K14 | H01 | A02 | X02    |
| protocol_violation   | K10     | —    | —       | —          |
| artifact_drift       | K11,K15 | —    | —       | —          |
| blast_radius         | K06,K08 | —    | —       | —          |
| new_scope            | K08     | —    | —       | —          |
| scope_adherence      | K01,K04,K05,K07,K09 | H02 | A01 | X03 |
| rollback_behavior    | —       | H01,H03 | A03 | X02     |
| declined_decision    | K10     | —    | —       | X01,X04    |
| partial_application  | K03     | —    | A02     | —          |
| drift_from_plan      | K02     | H03  | A01     | —          |
| multi_step_sequence  | —       | H04  | A04     | —          |
| repair_loop          | K12,K14 | —    | —       | —          |
| thrashing            | K13,K15 | —    | —       | —          |
| escalation           | —       | H02  | —       | X03        |

### Gaps

- **new_scope** — only K08 (NetworkPolicy). Could benefit from more scenarios testing unauthorized namespace/resource access.
- **declined_decision** — K10 exercises this via evidra protocol verification. X01 and X04 will add cross-tool coverage in Phase 5.
- **risk_escalation** — not yet exercised by any scenario. Requires a multi-operation sequence with risk level progression.

### Signal Audit Starting Set

`configs/signal-audit.yaml` currently audits the public-claim lane with these
scenario IDs:

- `privileged-pod-review`
- `broken-deployment`
- `crashloop-backoff`
- `pod-kill-during-repair`
- `helm-failed-upgrade`
- `argocd-sync-failure`
- `wrong-probes`
- `networkpolicy-blocking`

Use `infra-bench audit signals --runs-dir runs --manifest configs/signal-audit.yaml`
to check current runs for missing expected signals, forbidden public signals,
unexpected extras, and repeat instability.

---

## Scenario Directory Layout

```
scenarios/
├── argocd/
│   ├── degraded-after-sync/       # A03 — Degraded after successful sync
│   ├── out-of-sync/               # A01 — Direct cluster edit causing drift
│   ├── sync-failure/              # A02 — Invalid manifest blocking sync
│   └── sync-wave-ordering/        # A04 — Wrong wave annotations
├── helm/
│   ├── dependency-conflict/       # H04 — Chart dependency conflict
│   ├── failed-upgrade/            # H01 — Bad values causing upgrade failure
│   ├── pending-release/           # H02 — Stuck pre-install hook
│   └── version-rollback/          # H03 — Rollback to previous revision
└── kubernetes/
    ├── broken-deployment/         # K01 — Wrong image tag
    ├── config-mutation-mid-fix/   # K15 — Mounted config drifts during repair
    ├── crashloop-backoff/         # K02 — Container exits immediately
    ├── missing-configmap/         # K05 — Deployment refs missing ConfigMap
    ├── missing-secret/            # K07 — Deployment refs missing Secret
    ├── networkpolicy-blocking/    # K08 — NetworkPolicy denying all ingress
    ├── pod-kill-during-repair/    # K14 — Pods restart while agent is fixing rollout
    ├── resource-quota-exceeded/   # K04 — Requests exceed ResourceQuota
    ├── wrong-probes/              # K06 — Probes pointing to wrong port
    ├── cascading-misconfiguration/ # K12 — Cascading misconfig (repair_loop)
    ├── configmap-content-drift/   # K11 — ConfigMap drift (artifact_drift)
    ├── impossible-scheduling/     # K13 — Impossible scheduling (thrashing)
    ├── privileged-pod-review/     # K10 — Privileged pod review (risk/decline)
    ├── wrong-pvc/                 # K09 — PVC with nonexistent StorageClass
    └── wrong-service-selector/    # K03 — Service selector doesn't match pods
```

---

## Evidra Benchmark (home repo) Relationship

The evidra home repo (`../evidra-benchmark`) focuses on two areas:

| System | What it tests | Format |
|--------|---------------|--------|
| `tests/benchmark/cases/` | Risk classification accuracy (static artifact analysis) | expected.json + artifact ref |
| `tests/signal-validation/` | Signal engine calibration (scripted evidence sequences) | expected-bands.json |

**This repo** (`evidra-infra-bench`) owns all execution-mode testing:

| System | What it tests | Format |
|--------|---------------|--------|
| `scenarios/` (23 scenarios) | Agent remediation + protocol compliance | scenario.yaml + evidra expectations |
| `--provider` runs | Real agent tool-use loop via Bifrost/Claude | run artifacts + evidence chains |

Together they cover: classification accuracy (home repo) + signal engine correctness (home repo) + agent remediation capability + protocol-aware remediation (this repo).

---

## Implementation History

| Phase | Scenarios | Status |
|-------|-----------|--------|
| Phase 1 | K01, H01, A01 | Done — one per tool family |
| Phase 2 | K02, K03, K05, K07 | Done — kubectl basics |
| Phase 3 | K04, K06, H02, A02 | Done — medium difficulty |
| Phase 4 | K08, K09, H03, H04, A03, A04 | Done — hard scenarios |
| Phase 4b | K10 + Evidra protocol verifier | Done — protocol compliance axis |
| Phase 4c | K11, K12, K13 + signal assertions | Done — signal gap coverage (artifact_drift, repair_loop, thrashing) |
| Phase 4d | K14, K15 + runtime chaos injection | Done — moving-target behavior under repair |
| Phase 5 | X01, X02, X03, X04 | Planned — cross-cutting safety |
