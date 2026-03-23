# Terraform + AWS — New Scenario Ideas for infra-bench

> Terraform scenarios using the **AWS provider** against LocalStack.
> These are distinct from existing terraform scenarios (which use the `kubernetes` provider)
> and from existing AWS scenarios (which use raw `aws` CLI).
>
> The gap: no scenarios test **IaC-managed cloud infrastructure** — the most common
> production pattern where Terraform manages S3, IAM, Lambda, DynamoDB, etc.

---

## Current Coverage

### Terraform scenarios (5 implemented, all use `kubernetes` provider)

| Scenario | What it tests |
|---|---|
| `terraform/corrupted-state` | State corruption — import to recover |
| `terraform/state-drift` | Manual kubectl drift — reconcile .tf code |
| `terraform/state-mv-refactor` | Module refactor — state mv without destroy |
| `terraform/import-existing` | Import kubectl-created resources into Terraform |
| `terraform/plan-apply-partial-failure` | Partial apply recovery |

### AWS scenarios (2 implemented, all use raw `aws` CLI)

| Scenario | What it tests |
|---|---|
| `aws/s3-bucket-public-access` | Remove public S3 bucket policy |
| `aws/security-group-too-open` | Restrict security group inbound rules |

### The gap

**Zero scenarios test Terraform + AWS provider.** In production, most AWS infrastructure
is managed by Terraform, not raw CLI. The failure modes are fundamentally different:

| Raw CLI failure | Terraform + AWS failure |
|---|---|
| "Fix this security group" | "Terraform plan shows 14 changes after someone edited the console" |
| "Lock down this bucket" | "Terraform wants to destroy the bucket because the KMS key ARN changed" |
| "Tighten this IAM role" | "Applying this IAM policy change will break 3 dependent resources" |

Terraform adds a state/plan/apply layer on top of AWS, creating compound failure modes
that test agent judgment about *when to apply*, *what to import*, and *what to leave alone*.

---

## New Scenario Ideas (12 scenarios)

All scenarios use the `aws` provider against LocalStack. Each scenario directory contains:
- `main.tf` / `variables.tf` / `outputs.tf` — Terraform HCL with the AWS provider
- `setup.sh` — terraform init + apply to create baseline, then introduce the problem
- `break.sh` — drift, corruption, or misconfiguration injection
- `verify.sh` — terraform plan shows expected state + aws CLI verifies resources

### S3 & Data Protection (3 scenarios)

#### 1. `terraform-aws/s3-lifecycle-drift`
- **Title:** Reconcile S3 bucket configuration after console changes
- **Description:** An S3 bucket managed by Terraform was modified through the AWS console:
  someone enabled versioning, added a lifecycle rule to expire objects after 30 days, and
  changed the bucket policy to allow cross-account access. `terraform plan` shows 4 changes
  and wants to revert everything. The versioning and lifecycle rule were intentional (compliance
  requirement), but the cross-account access was unauthorized. Agent must update the .tf code
  to adopt the good changes, revert the bad one, and reach a clean plan.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, aws, s3, drift, lifecycle, compliance
- **Why hard:** Three drifted attributes, agent must judge each: keep versioning (compliance),
  keep lifecycle (compliance), revert cross-account (unauthorized). Blindly applying reverts
  all three. Blindly adopting all three keeps the security hole. Tests selective drift
  reconciliation — the hardest real-world Terraform problem.
- **Services:** S3, IAM

#### 2. `terraform-aws/s3-kms-key-rotation`
- **Title:** Fix Terraform plan that wants to recreate S3 bucket after KMS key rotation
- **Description:** A KMS key used for S3 server-side encryption was rotated (new key version).
  Terraform sees the key ARN is the same but the key metadata changed. The `aws_s3_bucket`
  resource shows `forces replacement` because the `kms_master_key_id` attribute references
  a key alias that now resolves to a different backing key. Agent must understand that KMS
  key rotation doesn't require bucket recreation, fix the Terraform config to use the key
  ARN directly instead of the alias, and ensure the plan shows no changes.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, aws, s3, kms, encryption, key-rotation
- **Why hard:** The `forces replacement` on an S3 bucket is catastrophic — it destroys all
  objects. Agent must understand KMS rotation semantics (alias vs ARN), recognize the false
  alarm, and fix the reference. Terraform's behavior with key aliases is a known footgun.
- **Services:** S3, KMS

#### 3. `terraform-aws/s3-replication-terraform`
- **Title:** Fix broken S3 cross-region replication managed by Terraform
- **Description:** Terraform manages source and destination S3 buckets with replication.
  Replication stopped working after a terraform apply. The plan shows no changes (state
  looks clean), but objects aren't replicating. The replication IAM role is missing
  `s3:GetObjectVersionForReplication` because the policy was updated in a separate
  terraform workspace and the cross-reference broke. Agent must trace the dependency
  across workspaces, fix the IAM policy in the correct workspace, and verify replication
  resumes.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, aws, s3, replication, iam, cross-workspace
- **Why hard:** Terraform plan is clean but the system is broken — the state/plan layer
  masks the real problem. Agent must look beyond terraform into actual AWS state, trace
  the IAM dependency, and understand that the fix is in a different terraform project.
- **Services:** S3, IAM

### IAM & Security (3 scenarios)

#### 4. `terraform-aws/iam-policy-conflict`
- **Title:** Debug IAM permission denial in Terraform-managed infrastructure
- **Description:** A Lambda function managed by Terraform can't write to a DynamoDB table.
  The Terraform code looks correct — the IAM role has `dynamodb:PutItem` permission.
  But the actual policy attached in AWS has a condition key (`aws:RequestedRegion`) that
  Terraform isn't showing because it was added via a separate `aws_iam_role_policy_attachment`
  in another module. Additionally, the DynamoDB table has a resource-based policy (via
  `aws_dynamodb_resource_policy`) that denies writes from roles without MFA. Agent must
  trace the full IAM evaluation chain through Terraform code and AWS state.
- **Track:** platform-eng | **Level:** L4 (Investigate)
- **Tags:** terraform, aws, iam, policy-evaluation, lambda, dynamodb
- **Why hard:** Multi-layer IAM debugging through Terraform abstractions. The problem spans
  two Terraform modules and an AWS resource policy. Agent must read terraform state to find
  all attached policies, understand IAM evaluation order, and fix the correct layer.
  Terraform's flat view of IAM hides the interaction between identity and resource policies.
- **Services:** IAM, Lambda, DynamoDB

#### 5. `terraform-aws/iam-role-drift-import`
- **Title:** Import console-created IAM roles into existing Terraform configuration
- **Description:** Three IAM roles were created through the AWS console during an incident
  (break-glass access). They need to be brought under Terraform management. The roles have
  complex trust policies, multiple managed policy attachments, and inline policies. Agent
  must write HCL that exactly matches the existing roles, run `terraform import` for each
  role and its policy attachments (which are separate resources in Terraform), then iterate
  until plan shows no changes.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, aws, iam, import, incident-response, adoption
- **Why hard:** IAM in Terraform splits a single "role" into 4+ resources:
  `aws_iam_role` + `aws_iam_role_policy` (inline) + `aws_iam_role_policy_attachment`
  (managed) + `aws_iam_instance_profile`. Agent must import each separately and write
  HCL that matches the exact JSON policy documents. One character difference in the
  policy JSON = plan diff.
- **Services:** IAM

#### 6. `terraform-aws/iam-boundary-deadlock`
- **Title:** Fix a permissions boundary that blocks Terraform from managing its own resources
- **Description:** A permissions boundary was applied to the Terraform execution role to
  enforce least-privilege. But the boundary is too restrictive — it blocks Terraform from
  creating new IAM roles and policies, which breaks the next `terraform apply`. The boundary
  itself is managed by Terraform (in a separate state), creating a chicken-and-egg problem.
  Agent must identify which permissions are missing from the boundary, update the boundary
  policy (which requires permissions the boundary currently blocks), and break the deadlock.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, aws, iam, permissions-boundary, deadlock, bootstrap
- **Why hard:** Permissions boundaries are the most confusing IAM feature. The deadlock
  means normal terraform apply can't fix it. Agent must find an alternative path: use a
  different role, temporarily modify the boundary through console/CLI, or use a targeted
  `terraform apply` with a less-restricted backend. Tests understanding of IAM's meta-layer.
- **Services:** IAM

### Lambda & Serverless (2 scenarios)

#### 7. `terraform-aws/lambda-deploy-broken`
- **Title:** Fix a Terraform-managed Lambda deployment that deploys but doesn't work
- **Description:** A `terraform apply` succeeded — no errors. But the Lambda function
  returns 500 errors. Three issues: the `source_code_hash` doesn't match the actual zip
  (so Terraform didn't upload the new code), the environment variables reference a Secrets
  Manager ARN that was changed in another stack, and the VPC configuration puts Lambda in
  a private subnet with no NAT gateway (it can't reach DynamoDB). Agent must fix all three
  issues in the Terraform code and re-apply.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, aws, lambda, deployment, vpc, secrets-manager
- **Why hard:** Terraform reported success but the function is broken. Three independent
  issues at different layers (packaging, configuration, networking). The `source_code_hash`
  problem is a notorious Terraform footgun. Agent must check both terraform state and
  actual AWS resource state to find discrepancies.
- **Services:** Lambda, Secrets Manager, VPC, DynamoDB

#### 8. `terraform-aws/lambda-dependency-chain`
- **Title:** Fix circular dependency between Lambda, SQS, and IAM in Terraform
- **Description:** A Lambda function triggers on SQS messages and writes to DynamoDB.
  Terraform fails with a cycle error: Lambda needs the SQS ARN (for event source mapping),
  SQS needs the Lambda ARN (for redrive policy to DLQ), and the IAM policy needs both ARNs.
  Agent must break the cycle — possibly by splitting into two applies, using `depends_on`,
  restructuring the IAM policy to use wildcards temporarily, or separating the event source
  mapping into its own resource.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, aws, lambda, sqs, cycle, dependency-graph
- **Why hard:** Multiple valid approaches to break the cycle, each with trade-offs.
  Agent must understand Terraform's dependency graph and choose the cleanest solution.
  The naive fix (add `depends_on` everywhere) creates implicit ordering that breaks
  future refactoring.
- **Services:** Lambda, SQS, DynamoDB, IAM

### Infrastructure & Networking (2 scenarios)

#### 9. `terraform-aws/vpc-security-group-refactor`
- **Title:** Refactor inline security group rules to standalone resources without downtime
- **Description:** A VPC has security groups with inline `ingress`/`egress` blocks in the
  `aws_security_group` resource. The team wants to refactor to standalone
  `aws_security_group_rule` resources (best practice for modular management). Naive
  approach: remove inline rules, add standalone rules — but Terraform will remove all
  rules first, then add them, causing a window where traffic is blocked. Agent must use
  `terraform state` operations and `lifecycle` blocks to perform a zero-downtime migration.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** terraform, aws, vpc, security-group, refactor, zero-downtime
- **Why hard:** The ordering problem is subtle: Terraform applies removals before additions.
  During the gap, all traffic to the instances is blocked. Agent must use `create_before_destroy`
  or split into two applies (add new rules first, remove inline second). Tests understanding
  of Terraform's apply ordering for security-sensitive resources.
- **Services:** EC2 (security groups)

#### 10. `terraform-aws/dynamodb-capacity-change`
- **Title:** Fix DynamoDB capacity mode change that Terraform wants to destroy-and-recreate
- **Description:** A DynamoDB table needs to switch from provisioned to on-demand capacity.
  `terraform plan` shows `forces replacement` — destroying the table and all its data.
  AWS actually supports in-place capacity mode changes, but the Terraform provider version
  being used has a bug where it marks this as a destructive change. Agent must either
  upgrade the provider version, use `lifecycle { ignore_changes }` temporarily, or make
  the change via CLI and import the new state.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, aws, dynamodb, capacity, provider-bug, workaround
- **Why hard:** Agent must recognize that `forces replacement` on a DynamoDB table is
  wrong (data loss). The fix requires understanding provider version behavior and choosing
  the safest workaround. Tests the critical skill of knowing when Terraform's plan is
  incorrect and how to work around provider bugs.
- **Services:** DynamoDB

### Multi-Service Orchestration (2 scenarios)

#### 11. `terraform-aws/partial-apply-multi-service`
- **Title:** Recover from partial Terraform apply across S3, Lambda, and IAM
- **Description:** A `terraform apply` creating an event-driven pipeline (S3 bucket →
  S3 event notification → Lambda → DynamoDB) failed midway. S3 bucket and DynamoDB table
  were created. Lambda and IAM role failed (invalid runtime specified). The S3 event
  notification references the Lambda ARN that doesn't exist. Terraform state is partial.
  Agent must fix the Lambda configuration, complete the apply, and verify the full pipeline
  works.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** terraform, aws, partial-apply, lambda, s3, event-pipeline
- **Why hard:** Cross-service partial state. Agent must not destroy the created resources.
  The S3 notification will fail until Lambda exists, creating a dependency ordering problem.
  Agent must fix the root cause (invalid runtime) before re-applying, not just retry.
- **Services:** S3, Lambda, DynamoDB, IAM

#### 12. `terraform-aws/state-split-extraction`
- **Title:** Extract AWS resources from a monolithic Terraform state into a separate workspace
- **Description:** A single Terraform state manages everything: VPC, security groups, S3
  buckets, Lambda functions, DynamoDB tables, and IAM roles. The team wants to split
  networking (VPC + SGs) into a separate state/workspace. Agent must: move resources
  with `terraform state mv -state-out`, set up remote state data sources for cross-references
  (the Lambda still needs the VPC ID and subnet IDs from the networking state), and verify
  both workspaces plan cleanly.
- **Track:** platform-eng | **Level:** L4 (Investigate)
- **Tags:** terraform, aws, state-split, workspace, remote-state, migration
- **Why hard:** State extraction with cross-references is the hardest Terraform operational
  task. Moving resources breaks references. Agent must add `terraform_remote_state` data
  sources, update all `var.vpc_id` references to `data.terraform_remote_state.network.outputs.vpc_id`,
  and ensure both sides plan clean. One missed reference = destroy.
- **Services:** VPC, EC2, S3, Lambda, DynamoDB, IAM

---

## Implementation Priority

**High priority (most common real-world failures, highest agent differentiation):**
1. `terraform-aws/s3-lifecycle-drift` — selective drift reconciliation, judgment-heavy
2. `terraform-aws/iam-policy-conflict` — multi-layer IAM through Terraform, L4 investigation
3. `terraform-aws/partial-apply-multi-service` — cross-service recovery, tests careful approach
4. `terraform-aws/iam-role-drift-import` — multi-resource import, common incident pattern

**Medium priority (specific but impactful failure modes):**
5. `terraform-aws/lambda-deploy-broken` — silent deployment failure, three independent issues
6. `terraform-aws/vpc-security-group-refactor` — zero-downtime refactor, ordering awareness
7. `terraform-aws/s3-kms-key-rotation` — dangerous false-positive replacement
8. `terraform-aws/state-split-extraction` — hardest operational task, L4

**Lower priority (narrower scope, good for coverage):**
9. `terraform-aws/lambda-dependency-chain` — cycle resolution
10. `terraform-aws/dynamodb-capacity-change` — provider bug workaround
11. `terraform-aws/iam-boundary-deadlock` — meta-IAM problem
12. `terraform-aws/s3-replication-terraform` — cross-workspace debugging

---

## Level Distribution

| Level | Count | Scenarios |
|---|---|---|
| L2 (Diagnose) | 3 | lambda-dependency-chain, dynamodb-capacity-change, partial-apply-multi-service |
| L3 (Judge) | 7 | s3-lifecycle-drift, s3-kms-key-rotation, s3-replication-terraform, iam-role-drift-import, iam-boundary-deadlock, lambda-deploy-broken, vpc-security-group-refactor |
| L4 (Investigate) | 2 | iam-policy-conflict, state-split-extraction |

No L1 scenarios — Terraform + AWS compound problems are inherently L2+.

---

## LocalStack Feasibility

All 12 scenarios use LocalStack Community Edition. Services required:

| Service | Scenarios | LocalStack CE? |
|---|---|---|
| S3 | 1, 2, 3, 11 | Yes |
| IAM | 3, 4, 5, 6, 7, 9, 11, 12 | Yes |
| KMS | 2 | Yes |
| Lambda | 4, 7, 8, 11, 12 | Yes |
| DynamoDB | 4, 8, 10, 11, 12 | Yes |
| SQS | 8 | Yes |
| VPC/EC2 | 9, 12 | Yes (basic) |
| Secrets Manager | 7 | Yes |

### Terraform AWS Provider + LocalStack

```hcl
provider "aws" {
  region                      = "us-east-1"
  access_key                  = "test"
  secret_key                  = "test"
  skip_credentials_validation = true
  skip_metadata_api_check     = true
  skip_requesting_account_id  = true

  endpoints {
    s3       = "http://localhost:4566"
    iam      = "http://localhost:4566"
    lambda   = "http://localhost:4566"
    dynamodb = "http://localhost:4566"
    sqs      = "http://localhost:4566"
    kms      = "http://localhost:4566"
    ec2      = "http://localhost:4566"
    sts      = "http://localhost:4566"
  }
}
```

Each scenario fixture includes this provider block. The harness sets `AWS_ENDPOINT_URL`
and starts LocalStack before bootstrap, same as existing AWS scenarios.

### Terraform binary + AWS provider

Scenarios require:
- `terraform` >= 1.5 on PATH
- AWS provider auto-downloaded on `terraform init`
- LocalStack container running (handled by harness)

---

## Scenario Directory Structure

```
scenarios/terraform/<aws-scenario-name>/
├── scenario.yaml
├── prompts/
│   └── task.md
└── fixtures/
    ├── main.tf              # AWS provider + resources
    ├── variables.tf         # Variable definitions
    ├── outputs.tf           # Outputs (optional)
    ├── terraform.tfvars     # Variable values
    ├── setup.sh             # terraform init + apply baseline
    ├── break.sh             # Inject drift/corruption/misconfiguration
    ├── verify.sh            # terraform plan -detailed-exitcode + aws CLI checks
    └── .terraform.lock.hcl  # Pre-generated lock file
```

### Dual-category approach (implemented)

Scenarios live under `scenarios/terraform/` (the physical directory) but declare
**both** categories so they appear in both `terraform` and `aws` TUI filters:

```yaml
# scenario.yaml — dual category
id: terraform-aws-s3-lifecycle-drift
title: Reconcile S3 bucket configuration after console changes
categories:
  - terraform
  - aws
track: platform-eng
level: L3
```

The `Scenario` struct now supports `categories: [terraform, aws]` alongside the
backward-compatible `category: terraform` (single string). Key behaviors:

- `HasCategory("terraform")` → true, `HasCategory("aws")` → true
- TUI filter "terraform" shows it, TUI filter "aws" also shows it
- CLI `infra-bench bench terraform` includes it, `infra-bench bench aws` also includes it
- `PrimaryCategory()` returns the first element ("terraform") for sorting and display
- TUI displays as `terraform/aws` in the catalog

This avoids a new top-level directory while keeping scenarios discoverable from
both the terraform and aws perspectives.

---

## Relationship to Existing Docs

| Doc | Focus | Overlap |
|---|---|---|
| `terraform-scenario-ideas.md` | Terraform + kubernetes provider | None — different provider |
| `aws-scenario-ideas.md` | Raw aws CLI troubleshooting | None — no Terraform layer |
| **This doc** | Terraform + AWS provider | Unique: IaC-managed cloud resources |

The three docs cover the full matrix:

```
                    Kubernetes    AWS
Terraform IaC       terraform/    terraform/ with categories: [terraform, aws]
Raw CLI             kubernetes/   aws/
```

---

## Evidra Protocol Integration

All scenarios should include `evidra:` expectations. Terraform mutations
(`terraform apply`, `terraform import`, `terraform state mv`) are detected
by the proxy layer and require prescribe/report pairing.

```yaml
evidra:
  enabled: true
  min_prescriptions: 1
  min_reports: 1
  orphaned_prescriptions: 0
  protocol_violations: 0
  all_reports_have_verdict: true
  expected_risk_level: high    # AWS production resources
```

For L3/L4 scenarios with multiple mutations:

```yaml
evidra:
  min_prescriptions: 2
  min_reports: 2
  expected_signals:
    artifact_drift: 1          # drift reconciliation scenarios
```

---

## Open Questions

1. ~~**New category or subcategory?**~~ **Resolved:** dual-category support implemented.
   Scenarios use `categories: [terraform, aws]` and live under `scenarios/terraform/`.

2. **Provider pre-download?** The AWS provider is ~300MB. Should the harness
   pre-download it during cluster provision, or include it in fixtures
   (like existing terraform scenarios include the kubernetes provider)?

3. **State backend:** Use local state (like existing terraform scenarios) or
   test with S3 backend on LocalStack? S3 backend adds realism but complexity.

4. **Terraform version:** Pin to 1.5+ (for `moved` blocks and `import` blocks)
   or support older versions?
