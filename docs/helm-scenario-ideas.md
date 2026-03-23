# Helm & Argo CD — New Scenario Ideas for infra-bench

> Professional-level release-ops troubleshooting — not basic install/rollback.
> Cross-referenced against Udemy advanced courses and real-world release management failures.
> No overlap with existing 8 release-ops scenarios.
>
> Sources: [Kalyan Reddy Daida — Helm Masterclass: 50 Practical Demos (Udemy)](https://www.udemy.com/course/helm-masterclass-50-practical-demos-for-kubernetes-devops/),
> [DevOps Hint — GitOps with ArgoCD Basics to Advanced (Udemy)](https://www.udemy.com/course/gitops-with-argocd-from-basics-to-advanced-hands-on-tutorial/)

---

## Current Coverage

| Existing Scenario | Category | Level | What it tests |
|---|---|---|---|
| `helm/failed-upgrade` | helm | L1 | `helm history` + rollback or re-upgrade with corrected values |
| `helm/version-rollback` | helm | L2 | Use `helm rollback` (not uninstall/reinstall) to restore previous revision |
| `helm/pending-release` | helm | L2 | Clear stuck pending-install state caused by a failed hook Job |
| `helm/dependency-conflict` | helm | L2 | Resolve a missing ConfigMap that the chart depends on |
| `argocd/out-of-sync` | argocd | L1 | Sync an app after someone modified live state (basic drift) |
| `argocd/sync-failure` | argocd | L2 | Fix broken source path in application spec |
| `argocd/sync-wave-ordering` | argocd | L2 | Correct sync wave annotations for proper resource ordering |
| `argocd/degraded-after-sync` | argocd | L3 | App synced but degraded — pod-level error, not ArgoCD config |

**Gap analysis:** All 4 Helm scenarios test basic release state problems (bad values, rollback, pending, dependency).
None test chart authoring failures, hook logic, template rendering, multi-release coordination,
or Helm internals. ArgoCD scenarios cover sync/drift basics but miss multi-source apps,
ApplicationSets, Kustomize overlays, and rollout strategies.

---

## New Scenario Ideas (10 scenarios)

All run on Kind clusters. Helm scenarios use the harness's built-in chart infrastructure.
ArgoCD scenarios use the same bootstrap pattern as existing `argocd/*` scenarios.

### Helm: Chart Authoring & Hooks (2 scenarios)

#### 1. `helm-template-rendering-broken`
- **Title:** Fix a Helm chart with broken template logic that renders invalid YAML
- **Description:** A chart upgrade fails at `helm upgrade` with a template rendering error. The chart's `_helpers.tpl` defines a named template for labels that uses `include` with the wrong template name, and a conditional block (`{{- if .Values.ingress.enabled }}`) is missing its `{{- end }}`, causing all resources after the ingress to be nested inside the conditional. The chart has 4 templates (deployment, service, ingress, configmap) but only 2 render. Agent must: run `helm template` to see the rendering errors, trace the bug to `_helpers.tpl` and the unclosed `if` block, fix both issues, and upgrade successfully.
- **Track:** release-ops | **Level:** L2 (Diagnose)
- **Tags:** helm, template, rendering, chart-development, troubleshooting
- **Distinct from:** `helm/failed-upgrade` tests bad *values* on a working chart. This tests broken *chart templates* — the chart itself is the bug.
- **Why hard:** Agent must read Go template error messages (notoriously cryptic), understand Helm's template rendering pipeline, and fix template syntax in `_helpers.tpl`. Two independent template bugs compound the failure.

#### 2. `helm-hook-job-ordering`
- **Title:** Fix Helm hooks that execute in the wrong order and leave the release broken
- **Description:** A chart has 4 hooks: a pre-install Job that creates a database schema, a pre-install Job that seeds initial data, a post-install Job that runs migrations, and a post-install Job that verifies connectivity. The hook weights are wrong: the seed Job (weight 0) runs before the schema Job (weight 5), causing it to fail on missing tables. The migration Job has `hook-delete-policy: before-hook-creation` which deletes the previous migration before the new one starts, losing its logs. Agent must: fix the hook weights so schema runs first (weight -5), seed second (weight 0), migration after install (weight 0), connectivity last (weight 5). Also change the delete policy on migration to `hook-succeeded` so logs are preserved until the next successful run.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** helm, hooks, ordering, delete-policy, lifecycle
- **Distinct from:** `helm/pending-release` tests a stuck hook state. This tests incorrect hook *logic* — hooks run but in the wrong order with wrong cleanup policies.
- **Why hard:** Hook weight ordering is counterintuitive (lower runs first). Agent must understand the full hook lifecycle, know the 3 delete policies and their implications, and reason about dependencies between 4 hooks. Judgment required on which delete policy preserves observability.

### Helm: Release Internals & Coordination (2 scenarios)

#### 3. `helm-release-secret-corrupted`
- **Title:** Recover a Helm release whose stored Secret has been tampered with
- **Description:** Helm stores release state as base64-encoded, gzipped Secrets in the release namespace. Someone accidentally edited one of these Secrets (changed a label), causing `helm list` to show the release but `helm upgrade` to fail with "release is in an invalid state." The release has 3 revisions; only the latest Secret is corrupted. Agent must: understand that Helm uses Secrets (not ConfigMaps) for release storage by default, identify the corrupted Secret (`sh.helm.release.v1.<name>.v3`), either restore the Secret from the previous revision's data or delete the corrupted revision so rollback to v2 works, and get the release operational again.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** helm, release, secret, storage, recovery
- **Distinct from:** `helm/pending-release` tests a stuck lifecycle state. This tests corrupted release *storage* — the Kubernetes Secret backing the release is damaged.
- **Why hard:** Most engineers don't know Helm stores state in Secrets. Agent must navigate the `sh.helm.release.v1.*` naming convention, understand base64+gzip encoding, and choose between restoration strategies. Deleting the wrong Secret loses release history.

#### 4. `helm-multi-release-coordination`
- **Title:** Coordinate a multi-release upgrade where ordering matters
- **Description:** Three Helm releases must be upgraded in sequence: `database` (schema migration), `backend` (new API version), `frontend` (uses new API). Upgrading in the wrong order causes: frontend→502 (backend not ready), backend→crash (schema doesn't exist yet). The current state: database is v1.0, backend v1.0, frontend v1.0. Target: all at v2.0. Agent must: determine the correct upgrade order by inspecting each chart's changes, upgrade database first and wait for the migration Job to complete, upgrade backend and wait for readiness, upgrade frontend last, and verify all three releases are healthy. A values file for each release is provided.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** helm, multi-release, coordination, ordering, upgrade-strategy
- **Distinct from:** All existing Helm scenarios operate on a single release. This tests cross-release dependency reasoning.
- **Why hard:** No single Helm command handles multi-release coordination. Agent must determine the dependency graph by reading the charts, execute upgrades in correct order with health checks between each, and handle the case where a middle upgrade fails (should it rollback all previous upgrades too?). Tests operational judgment.

### ArgoCD: Advanced Patterns (4 scenarios)

#### 5. `argocd-app-of-apps-broken`
- **Title:** Fix a broken App-of-Apps pattern where child applications can't sync
- **Description:** An ArgoCD "app of apps" parent Application manages 3 child Applications (frontend, backend, database). The parent syncs but children fail: frontend has `targetRevision: main` but the repo renamed the branch to `master`, backend's Helm values file path is wrong (points to `values-prod.yaml` but the file is `values/prod.yaml`), and database uses a private Helm repository that ArgoCD doesn't have credentials for. Agent must: identify all 3 independent failures from the parent's health status, fix the branch reference, correct the values file path, and add the Helm repo credentials to ArgoCD.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** argocd, app-of-apps, multi-application, troubleshooting
- **Distinct from:** `argocd/sync-failure` tests one app with one path error. This tests the App-of-Apps pattern with 3 different failure modes across 3 child apps.
- **Why hard:** App-of-Apps cascading failures are hard to debug — the parent shows "Degraded" but the root causes are in 3 different children. Agent must check each child independently. The repo credential issue adds an infrastructure-level problem alongside config issues.

#### 6. `argocd-applicationset-generator-failure`
- **Title:** Debug an ApplicationSet that generates wrong or missing applications
- **Description:** An ApplicationSet with a Git directory generator should create one ArgoCD Application per subdirectory in `apps/*/`. It generates 5 apps but should generate 3 — two directories contain only a README (no manifests) and generate empty apps that are permanently "Missing." Additionally, the template's `{{path.basename}}` variable creates apps named `app-app-frontend` (double prefix) because the template already prepends `app-`. Agent must: add `exclude` patterns to skip non-manifest directories, fix the naming template to avoid double prefixes, and verify exactly 3 healthy applications exist.
- **Track:** release-ops | **Level:** L2 (Diagnose)
- **Tags:** argocd, applicationset, generator, git-directory, template
- **Distinct from:** No existing scenario tests ApplicationSets at all. This is a distinct ArgoCD feature from plain Applications.
- **Why hard:** ApplicationSet generators have their own templating language separate from Helm/Kustomize. The Git directory generator's include/exclude patterns use different syntax. Agent must understand generator-level filtering vs. application-level configuration.

#### 7. `argocd-kustomize-overlay-conflict`
- **Title:** Fix an ArgoCD app using Kustomize where overlays produce conflicting patches
- **Description:** An ArgoCD Application uses a Kustomize overlay (`overlays/production/`) that applies 3 patches: a replica count patch, a resource limits patch, and a namespace patch. The app fails to sync because: the replica patch targets `Deployment/web` but the base renamed it to `Deployment/web-server`, the resource limits patch uses a strategic merge patch that accidentally nulls out the existing volume mounts, and the namespace transformer conflicts with ArgoCD's own namespace override. Agent must: fix the patch target name, switch the resource patch to a JSON patch (to avoid clobbering volume mounts), and resolve the namespace conflict by removing ArgoCD's namespace override or the Kustomize one.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** argocd, kustomize, overlay, patches, strategic-merge
- **Distinct from:** No existing scenario tests ArgoCD + Kustomize integration. All current ArgoCD scenarios use plain manifests.
- **Why hard:** Kustomize strategic merge patches vs. JSON patches behave differently — strategic merge replaces entire lists, JSON patches target specific fields. The namespace conflict between Kustomize and ArgoCD is a known footgun. Agent must understand both tools' namespace handling to resolve the conflict correctly.

#### 8. `argocd-rollout-canary-stuck`
- **Title:** Debug an Argo Rollout canary deployment that's stuck at the analysis phase
- **Description:** An Argo Rollout is performing a canary deployment. It successfully shifted 20% of traffic to the canary but is stuck at the "analysis" step. The AnalysisRun shows "Inconclusive" because: the Prometheus metric query has a typo in the metric name (`http_request_duration_seconds` vs. `http_requests_duration_seconds`), the success condition threshold is impossible to meet (error rate < 0.001% with only 50 requests — not statistically significant), and the analysis timeout is set to 1 minute which isn't enough for metrics to stabilize. Agent must: diagnose the stuck rollout from `kubectl argo rollouts get`, fix the metric query, adjust the success condition to a realistic threshold, increase the analysis timeout, restart the analysis, and let the canary either promote or abort based on real metrics.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** argocd, argo-rollouts, canary, analysis, progressive-delivery
- **Distinct from:** No existing scenario tests Argo Rollouts or progressive delivery at all. This is an entirely new tool in the Argo ecosystem.
- **Why hard:** Argo Rollouts AnalysisRun debugging requires understanding metric queries, statistical significance of thresholds, and the rollout state machine (Progressing → Paused → Promoting/Aborting). Three independent issues in the AnalysisTemplate. Tests progressive delivery expertise that goes well beyond basic ArgoCD sync operations.

### Cross-cutting: Helm + ArgoCD Integration (2 scenarios)

#### 9. `argocd-helm-values-drift`
- **Title:** Resolve drift between ArgoCD-managed Helm values and cluster state
- **Description:** An ArgoCD Application deploys a Helm chart with values specified in the Application spec (`spec.source.helm.values`). Someone ran `helm upgrade` directly on the cluster, bypassing ArgoCD, changing the replica count and adding new environment variables. ArgoCD shows "OutOfSync" but a naive sync would revert the manual Helm changes. However, some changes were intentional hotfixes that should be preserved. Agent must: compare ArgoCD's desired values with the live Helm release values, determine which manual changes to adopt (the env vars for a hotfix) vs. revert (the replica count was a mistake), update the ArgoCD Application's Helm values to include the hotfix, and sync to reconcile.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** argocd, helm, drift, values, gitops-bypass
- **Distinct from:** `argocd/out-of-sync` tests basic `kubectl` drift on plain manifests. This tests Helm-specific drift where someone bypassed ArgoCD with `helm upgrade`, requiring Helm values reconciliation.
- **Why hard:** When someone bypasses ArgoCD to run `helm upgrade`, the drift exists at the Helm values level, not just the manifest level. Agent must compare ArgoCD's Helm parameters with `helm get values` output, make a judgment about which changes to keep, and update the ArgoCD Application spec — not just click "sync."

#### 10. `argocd-helm-chart-version-pinning`
- **Title:** Fix an ArgoCD Helm application that auto-upgraded to an incompatible chart version
- **Description:** An ArgoCD Application points to a Helm chart repository with `targetRevision: 2.*` (wildcard). The chart repo published v2.5.0 which has breaking changes: a renamed values key (`database.host` → `db.hostname`), a removed template (`ingress.yaml` replaced by `gateway.yaml`), and a new required value (`db.credentials.secretName`). ArgoCD auto-synced to v2.5.0 and the app is degraded. Agent must: identify the version jump from ArgoCD app status, pin to the last working version (v2.4.3) as an immediate fix, then update the values to be compatible with v2.5.0, upgrade the pin to `2.5.0`, and verify the app is healthy.
- **Track:** release-ops | **Level:** L3 (Judge)
- **Tags:** argocd, helm, chart-version, semver, breaking-changes
- **Distinct from:** `argocd/degraded-after-sync` tests a broken image, not a chart version change. `helm/failed-upgrade` tests bad values on the same chart version.
- **Why hard:** Two-phase fix: immediate rollback pin + forward migration. Agent must read the chart's CHANGELOG or compare templates between versions to identify breaking changes. The wildcard `targetRevision` is a common production mistake. Tests the judgment to stabilize first, then migrate.

---

## Excluded Ideas (overlap or trivial)

| Excluded Idea | Reason | Details |
|---|---|---|
| Helm rollback after bad image tag | **Overlaps** `helm/failed-upgrade` + `helm/version-rollback` | Both existing scenarios already test rollback from bad values |
| Fix Helm pending-upgrade state | **Overlaps** `helm/pending-release` | Same concept: stuck Helm state machine, different trigger |
| ArgoCD fix wrong Git path | **Overlaps** `argocd/sync-failure` | That scenario already tests exactly this: broken source path |
| ArgoCD manual drift → sync | **Overlaps** `argocd/out-of-sync` | That scenario already tests kubectl drift + sync reconciliation |
| ArgoCD degraded pod troubleshooting | **Overlaps** `argocd/degraded-after-sync` | Already tests "synced but degraded → look at pod errors" |
| Helm chart missing dependency repo | **Overlaps** `helm/dependency-conflict` | Similar: chart dependency not satisfied at install time |
| ArgoCD sync wave fix | **Overlaps** `argocd/sync-wave-ordering` | Already tests annotation ordering |
| Helm upgrade with --atomic cleanup | **Overlaps** `helm/failed-upgrade` | Variation of same failed-upgrade pattern |
| `helm-values-schema-violation` | **Trivial** | Schema errors list violations directly — agent just reads errors and fixes values. Not professional-level diagnosis. |
| `helm-subchart-values-override` | **Similar to** `helm/dependency-conflict` | Both are "chart dependency plumbing broken, workload uses defaults." Different cause but same diagnostic shape. |
| `helm-chart-signature-verification` | **Niche + heavy infra** | GPG key management + OCI registry setup is a lot of fixture infrastructure for a rarely-used Helm feature. |
| `argocd-repo-credentials-expired` | **Trivial + similar to** OCI auth | "Auth failed → update Secret with new key" is a credential rotation, not deep troubleshooting. Same pattern as OCI registry auth. |
| `helm-oci-registry-auth-failure` | **Similar to** expired credentials | Both are "credentials broken → fix credentials → retry." Different tool, same shape. |

---

## Implementation Priority

**High priority (professional-level, entirely new test areas):**
1. `argocd-app-of-apps-broken` — App-of-Apps is the #1 ArgoCD production pattern, untested
2. `helm-template-rendering-broken` — chart authoring bugs are distinct from values/state bugs
3. `argocd-helm-values-drift` — GitOps bypass via direct `helm upgrade` is a top real-world incident
4. `helm-hook-job-ordering` — hook lifecycle mastery separates professional from beginner

**Medium priority (advanced patterns, strong diagnostic value):**
5. `argocd-rollout-canary-stuck` — progressive delivery is the ArgoCD growth area
6. `helm-release-secret-corrupted` — Helm internals knowledge, unique failure mode
7. `argocd-applicationset-generator-failure` — ApplicationSets are how teams scale ArgoCD
8. `helm-multi-release-coordination` — cross-release orchestration, untested pattern

**Lower priority (still professional, narrower scope):**
9. `argocd-kustomize-overlay-conflict` — Kustomize+ArgoCD is a common production combo
10. `argocd-helm-chart-version-pinning` — wildcard versions + breaking changes

---

## Kind Cluster Feasibility

All 10 scenarios run on Kind clusters, same as existing `helm/*` and `argocd/*` scenarios.

| Requirement | Feasibility | Scenarios |
|---|---|---|
| Helm 3.x CLI | Available in harness | 1–4, 9–10 |
| ArgoCD + CRDs | Bootstrapped (same as existing argocd/*) | 5–10 |
| Argo Rollouts controller | Additional bootstrap step, lightweight | 8 |
| Kustomize | Built into kubectl | 7 |
| ApplicationSet controller | Bundled with ArgoCD since v2.3 | 6 |
| Prometheus (for analysis) | Needed for rollout analysis metrics, lightweight install | 8 |

No scenarios require external cloud services, SaaS platforms, or real Git hosting.
All Git repositories can be simulated with local `gitea` or fixture-based Git servers
running in the Kind cluster.
