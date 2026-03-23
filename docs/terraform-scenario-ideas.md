# Terraform — New Scenario Ideas for infra-bench

> Hard, practical troubleshooting scenarios — not beginner tutorials.
> Cross-referenced against Udemy Terraform courses and real-world IaC failures.
> No overlap with existing scenarios (only `terraform/corrupted-state` exists today).
>
> Sources: [Zeal Vora — Terraform Associate 2026 (Udemy)](https://www.udemy.com/course/terraform-beginner-to-advanced/),
> [Bryan Krausen — Terraform Associate 004 (Udemy)](https://www.udemy.com/course/hashicorp-certified-terraform-associate-step-by-step/),
> [HashiCorp Terraform Associate 004 Exam Objectives](https://www.hashicorp.com/en/certification/terraform-associate)

---

## Current Coverage

| Existing Scenario | What it tests |
|---|---|
| `terraform/corrupted-state` | State file corruption — `terraform import` to recover missing resources |

**One scenario.** Everything below is new territory.

### Terraform Associate 004 Exam Domains

| Domain | Weight | Covered? |
|---|---|---|
| 1 — Understand IaC concepts | ~8% | — |
| 2 — Terraform purpose & basics | ~8% | — |
| 3 — Terraform CLI (init, plan, apply, destroy, fmt, validate) | ~12% | Partial (import via corrupted-state) |
| 4 — Use Terraform outside core workflow (import, state mv/rm, taint, replace) | ~10% | Partial (import only) |
| 5 — Interact with modules | ~10% | **None** |
| 6 — Use core workflow (write, plan, apply) | ~12% | — |
| 7 — Manage state | ~12% | Partial (corruption only) |
| 8 — Read/generate configs (variables, outputs, data sources, expressions) | ~14% | **None** |
| 9 — Terraform Cloud/Enterprise | ~14% | **None** (out of scope — requires SaaS) |

---

## New Scenario Ideas (15 hard troubleshooting scenarios)

All scenarios use the `kubernetes` provider against a Kind cluster (same pattern as
`corrupted-state`) so they run locally with no cloud dependency. Each tests a specific
Terraform failure mode that agents encounter in real production environments.

### State Management Failures (4 scenarios)

#### 1. `terraform-state-drift`
- **Title:** Reconcile Terraform state after manual kubectl changes
- **Description:** Someone bypassed Terraform and used `kubectl` to scale the deployment from 2→5 replicas, add labels, and change the image tag. `terraform plan` now shows it wants to revert all manual changes. Agent must decide which changes to keep: adopt the manual changes into Terraform code (update `.tf` files to match reality), or selectively revert some while keeping others. Simply running `terraform apply` would destroy the manual work.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, state, drift, reconciliation, troubleshooting
- **Why hard:** Judgment call — not all drift is bad. Agent must read the plan output, understand which diffs are intentional (scale-up) vs. accidental, and update the `.tf` files accordingly rather than blindly applying. Tests the most common real-world Terraform failure.

#### 2. `terraform-state-mv-refactor`
- **Title:** Refactor Terraform code without destroying resources
- **Description:** A monolithic `main.tf` with 6 resources needs to be split into modules (`modules/app`, `modules/db`). Naive approach: delete old code, write modules, apply — which destroys and recreates everything. Agent must use `terraform state mv` to move resources into the new module paths (e.g., `kubernetes_deployment.web` → `module.app.kubernetes_deployment.web`) so `terraform plan` shows no changes after refactor.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, state, refactor, modules, state-mv
- **Why hard:** Agent must plan the state moves before touching code. Wrong order or wrong address = resource destruction. Must understand Terraform's resource addressing with module prefixes. Tests real-world code maintenance skill.

#### 3. `terraform-state-lock-stuck`
- **Title:** Recover from a stuck Terraform state lock
- **Description:** A previous `terraform apply` crashed mid-execution, leaving the state locked. The lock file exists but the process is gone. Agent cannot run any Terraform command. Must identify the lock, safely force-unlock it (verify no other process is running first), then assess the partial apply: some resources were created, others weren't. Agent must bring state and reality back in sync.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, state, lock, crash-recovery, troubleshooting
- **Why hard:** Two-phase problem: unlock first, then handle partial apply. Agent must not blindly `terraform apply` after unlocking — the partial state may cause duplicates or conflicts. Requires inspecting state vs. cluster reality.

#### 4. `terraform-state-rm-orphan`
- **Title:** Clean up Terraform-managed resources that were deleted externally
- **Description:** Three resources in the state file reference Kubernetes objects that were deleted by another team. `terraform plan` fails with "resource not found" errors during refresh. Agent must use `terraform state rm` to remove the orphaned entries, then decide whether to recreate the resources or update the code to remove them.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, state, orphan, cleanup, troubleshooting
- **Why hard:** `terraform plan` crashes before showing useful output. Agent must read the error, identify which resources are orphaned, remove them from state, and then handle the code (delete resource blocks or recreate). Common in teams with mixed kubectl/terraform workflows.

### Configuration & Expression Failures (4 scenarios)

#### 5. `terraform-variable-precedence`
- **Title:** Debug unexpected variable values from conflicting sources
- **Description:** A deployment uses the wrong instance count. Variables are set in 4 places: `variables.tf` (default=2), `terraform.tfvars` (replicas=3), `dev.auto.tfvars` (replicas=1), and `TF_VAR_replicas=5` environment variable. Agent must understand the precedence chain, identify which source wins, and fix the configuration so the correct value (from `dev.auto.tfvars`) takes effect by removing the conflicting higher-precedence sources.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, variables, precedence, debugging, configuration
- **Why hard:** Terraform variable precedence is: defaults < `terraform.tfvars` < `*.auto.tfvars` < `-var` / `TF_VAR_` env. Counter-intuitive: env var beats the file. Agent must trace the value through the chain and know the exact ordering. A common source of production bugs.

#### 6. `terraform-count-index-splat`
- **Title:** Fix a broken Terraform config using count with dynamic references
- **Description:** A config uses `count = 3` on a deployment resource. An output references `kubernetes_deployment.web.metadata[0].name` (wrong — should use splat `[*]` or index). A service references `kubernetes_deployment.web.id` (wrong — count resources need index). `terraform plan` fails with multiple errors. Agent must fix all references to work with count-indexed resources.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, count, splat, expressions, troubleshooting
- **Why hard:** Count changes resource addresses from `resource.name` to `resource.name[N]`. Every reference must be updated. Agent must understand splat expressions (`[*]`), `count.index`, and when to use `toset` vs. `tolist`. Multiple cascading errors from one root cause.

#### 7. `terraform-for-each-key-change`
- **Title:** Fix a for_each map change that wants to destroy and recreate resources
- **Description:** Resources were created with `for_each = toset(["web", "api", "worker"])`. Someone changed the map keys to `["frontend", "backend", "worker"]`. `terraform plan` shows destroy `web` + `api`, create `frontend` + `backend` — losing the existing deployments. Agent must use `terraform state mv` to rename the state keys to match the new code, so plan shows no changes.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, for-each, state, rename, troubleshooting
- **Why hard:** for_each keys are part of the resource address (`resource.name["web"]`). Changing keys = destroy+create in Terraform's eyes. Agent must combine `state mv` with code changes to achieve zero-downtime refactor. Tests deep understanding of Terraform's resource identity model.

#### 8. `terraform-cycle-dependency`
- **Title:** Break a circular dependency between Terraform resources
- **Description:** Two resources reference each other: a NetworkPolicy references the Service's cluster IP, and the Service has an annotation referencing the NetworkPolicy name. `terraform plan` fails with "Cycle" error. Agent must break the cycle by restructuring the dependency — either using `depends_on` with a data source, splitting into two applies, or removing the circular reference.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, cycle, dependency, graph, troubleshooting
- **Why hard:** Multiple valid solutions: break the cycle by removing one reference, use a `null_resource` with `local-exec` to set the annotation after creation, or use `terraform apply -target` for staged deployment. Tests understanding of Terraform's dependency graph.

### Module & Provider Failures (3 scenarios)

#### 9. `terraform-module-version-conflict`
- **Title:** Fix a module upgrade that breaks existing infrastructure
- **Description:** A module was upgraded from v1 to v2. The new version changed variable names (`name` → `app_name`), removed an output (`service_ip`), and changed a resource type. `terraform plan` shows destroy-and-recreate for everything. Agent must update the calling code to use new variable names, fix output references, and use `moved` blocks or `state mv` to prevent resource destruction.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, modules, upgrade, version, breaking-changes
- **Why hard:** Multiple simultaneous breakages from one version bump. Agent must read the plan carefully, distinguish between cosmetic changes (renames) and real changes (new resource types), and use the right tool for each: `moved` block for renames, code updates for API changes. Tests real-world module maintenance.

#### 10. `terraform-provider-version-pin`
- **Title:** Fix provider version constraint conflict blocking terraform init
- **Description:** `terraform init` fails: the root module requires `kubernetes >= 2.25`, but a child module pins `kubernetes ~> 2.20` (which excludes 2.25+). The `.terraform.lock.hcl` has stale hashes from a different platform. Agent must resolve the version constraint conflict and fix the lock file — without downgrading security patches.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, provider, version, constraint, lock-file
- **Why hard:** Two problems: version conflict + stale lock file. Agent must understand version constraint syntax (`>=`, `~>`, `!=`), update the child module constraint, and run `terraform init -upgrade` to regenerate the lock file. Common after team member uses different OS.

#### 11. `terraform-module-source-broken`
- **Title:** Fix a module that references a moved Git repository
- **Description:** A Terraform module sources from a Git URL that no longer exists (repo was renamed/moved). `terraform init` fails with "repository not found". There are 3 module blocks using this source. Agent must find the new location, update all source references, handle the `.terraform/modules/` cache that still has stale data, and verify the module interface hasn't changed.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, modules, source, git, troubleshooting
- **Why hard:** Agent must clean the module cache (`rm -rf .terraform/modules`), find the correct new source, update all references, and re-init. If the module API changed, further code fixes needed. Tests operational recovery skills.

### Import & Migration Failures (2 scenarios)

#### 12. `terraform-import-existing`
- **Title:** Import an entire manually-created application stack into Terraform
- **Description:** A 4-resource stack (Namespace, Deployment, Service, ConfigMap) was created with kubectl and needs to be brought under Terraform management. Agent must write `.tf` resource blocks that match the existing resources, run `terraform import` for each, then iterate until `terraform plan` shows no changes. Any mismatch means the import "succeeded" but plan will destroy/recreate.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, import, migration, adoption, troubleshooting
- **Why hard:** Import only brings state — it doesn't generate code. Agent must write HCL that exactly matches reality: every label, annotation, selector, port. One mismatch and `terraform plan` wants to "update" (often means destroy+create). Iterative refinement until plan is clean. Much harder than corrupted-state scenario.
- **Distinct from `corrupted-state`:** That scenario has existing `.tf` code and just needs `import` for 2 known resources. This starts from zero code — agent must write all HCL from scratch by inspecting live resources.

#### 13. `terraform-moved-block-rename`
- **Title:** Rename Terraform resources without destroying them using moved blocks
- **Description:** Resources have bad names: `kubernetes_deployment.foo`, `kubernetes_service.bar`. They need renaming to `kubernetes_deployment.web`, `kubernetes_service.web_svc`. Simply renaming in code = destroy+create. Agent must add `moved` blocks to tell Terraform these are renames, verify plan shows "moved" not "destroyed", and clean up the moved blocks after apply.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, moved, rename, refactor, troubleshooting
- **Why hard:** `moved` block syntax is specific and easy to get wrong (from/to addresses must be exact). Agent must handle multiple moves without circular references. Tests knowledge of Terraform 1.1+ features for safe refactoring.

### Multi-Resource Interaction Failures (2 scenarios)

#### 14. `terraform-taint-replace-targeted`
- **Title:** Force recreation of one resource without affecting dependents
- **Description:** A Kubernetes deployment's pods are in a bad state due to a cached image. The deployment spec in Terraform hasn't changed, so `terraform apply` does nothing. Agent must force recreation of just the deployment (using `terraform apply -replace=`) without touching the Service, ConfigMap, or other resources. Must also handle the case where dependents would be affected.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, taint, replace, targeted, troubleshooting
- **Why hard:** `terraform taint` (deprecated) vs. `terraform apply -replace=` (current). Agent must know the exact resource address. If the deployment is referenced by other resources, `-replace` may cascade. Agent must check the plan before applying. Tests operational "surgical fix" capability.

#### 15. `terraform-plan-apply-partial-failure`
- **Title:** Recover from a partial terraform apply that created some resources but failed mid-way
- **Description:** A `terraform apply` was creating 5 resources. It succeeded on 3 (namespace, configmap, service) but failed on the 4th (deployment — invalid image) and never reached the 5th (ingress). State has 3 resources. Running `apply` again will try to create the deployment (which will fail again) and the ingress. Agent must fix the deployment image in the `.tf` file, then apply — and verify the state is fully consistent afterward.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, partial-apply, recovery, error-handling, troubleshooting
- **Why hard:** Terraform state is partially applied — some resources exist, others don't. Agent must not destroy the working resources. Must read the error (invalid image), fix the root cause in HCL, and re-apply. Trap: running `terraform destroy` first would kill the 3 working resources.

---

## Implementation Priority

**High priority (practical troubleshooting, distinct from corrupted-state):**
1. `terraform-state-drift` — #1 real-world Terraform problem, judgment required
2. `terraform-import-existing` — much harder than corrupted-state, builds from zero
3. `terraform-plan-apply-partial-failure` — common crash recovery, tests careful approach
4. `terraform-state-mv-refactor` — code maintenance skill, zero-downtime refactor

**Medium priority (expression/config debugging):**
5. `terraform-for-each-key-change` — subtle identity model, state mv + code change
6. `terraform-variable-precedence` — debugging a very common source of bugs
7. `terraform-state-lock-stuck` — operational recovery
8. `terraform-cycle-dependency` — graph understanding
9. `terraform-module-version-conflict` — real-world module upgrade pain

**Lower priority (narrower scenarios):**
10. `terraform-state-rm-orphan` — state cleanup
11. `terraform-count-index-splat` — expression syntax
12. `terraform-provider-version-pin` — init failure debugging
13. `terraform-module-source-broken` — source resolution
14. `terraform-taint-replace-targeted` — surgical fix
15. `terraform-moved-block-rename` — refactoring technique

---

## Kind Cluster Feasibility

All 15 scenarios use the `kubernetes` provider against a Kind cluster, same as
the existing `corrupted-state` scenario. No cloud credentials needed.

Each scenario's fixtures include:
- `main.tf` (and optionally `modules/`) — the Terraform code with embedded bugs
- `setup.sh` — creates the initial state (terraform init + apply of the "good" version)
- `break.sh` — introduces the failure (corrupt state, manual kubectl changes, code changes)
- `verify.sh` — checks that `terraform plan` shows no changes and resources are healthy

### Terraform binary requirement

Scenarios require `terraform` (>= 1.5) on the agent's PATH. The harness can pre-install
it during bootstrap, similar to how Helm scenarios expect `helm`.

### No overlap with existing scenarios

| This doc | Existing | Why distinct |
|---|---|---|
| All 15 ideas | `corrupted-state` | That scenario tests `terraform import` for 2 known resources with existing HCL. None of these 15 duplicate that pattern. |
| State scenarios | Kubernetes troubleshooting scenarios | Those test kubectl/pod-level issues. These test IaC-layer state management — completely different abstraction level. |
| Import-existing | corrupted-state | corrupted-state has code, just missing state. import-existing has no code — agent writes HCL from scratch. |
