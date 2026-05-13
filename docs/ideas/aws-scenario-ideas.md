# AWS — New Scenario Ideas for infra-bench

> Professional-level troubleshooting scenarios — not beginner security fixes.
> Cross-referenced against Udemy professional certification courses and real-world AWS incident patterns.
> No overlap with existing scenarios (only `aws/s3-bucket-public-access` and `aws/security-group-too-open` exist today).
>
> Sources: [Stephane Maarek — AWS Security Specialty SCS-C03 (Udemy)](https://www.udemy.com/course/ultimate-aws-certified-security-specialty/),
> [Stephane Maarek — AWS DevOps Professional DOP-C02 (Udemy)](https://www.udemy.com/course/aws-certified-devops-engineer-professional-hands-on/)

---

## Current Coverage

| Existing Scenario | What it tests |
|---|---|
| `aws/s3-bucket-public-access` | S3 bucket policy — remove public read access |
| `aws/security-group-too-open` | EC2 security group — restrict inbound rules |

**Two beginner-level L2 scenarios.** Everything below is professional-level territory.

### AWS Certification Domain Coverage

#### Security Specialty (SCS-C03) Domains

| Domain | Weight | Covered? |
|---|---|---|
| 1 — Threat Detection & Incident Response | ~22% | **None** |
| 2 — Security Logging & Monitoring | ~20% | **None** |
| 3 — Infrastructure Security | ~18% | Partial (SG only) |
| 4 — Identity & Access Management | ~16% | **None** |
| 5 — Data Protection | ~14% | Partial (S3 only) |
| 6 — Security Governance | ~10% | **None** |

#### DevOps Professional (DOP-C02) Domains

| Domain | Weight | Covered? |
|---|---|---|
| 1 — SDLC Automation | ~22% | **None** |
| 2 — Configuration Management & IaC | ~17% | **None** (Terraform scenarios are separate) |
| 3 — Resilient Cloud Solutions | ~15% | **None** |
| 4 — Monitoring & Logging | ~15% | **None** |
| 5 — Incident & Event Response | ~16% | **None** |
| 6 — Security & Compliance | ~15% | **None** |

---

## New Scenario Ideas (15 professional-level troubleshooting scenarios)

All scenarios use LocalStack (same pattern as existing AWS scenarios) so they run locally
with no cloud credentials. Each tests a specific failure mode drawn from professional-level
certification material and real-world AWS incidents.

### Threat Detection & Incident Response (3 scenarios)

#### 1. `aws-guardduty-findings-triage`
- **Title:** Triage GuardDuty findings and remediate a compromised IAM credential
- **Description:** GuardDuty has generated 5 findings: a `UnauthorizedAccess:IAMUser/InstanceCredentialExfiltration`, two `Recon:IAMUser/` findings, a `Trojan:EC2/` finding, and a low-severity `Policy:IAMUser/RootCredentialUsage`. One IAM access key has been exfiltrated and is making API calls from an external IP. Agent must: triage findings by severity, disable the compromised access key, revoke active sessions via an inline deny policy with a date condition, identify what resources the attacker accessed via CloudTrail, and create an SNS-backed EventBridge rule to alert on future high-severity findings.
- **Track:** runtime-security | **Level:** L3 (Judge)
- **Tags:** aws, guardduty, incident-response, iam, cloudtrail, security-specialty
- **Why hard:** Multi-phase incident response. Agent must know the correct remediation sequence (disable key before revoking sessions, not the reverse). Must filter CloudTrail by access key ID and time range. Must write the session revocation policy with `aws:TokenIssueTime` condition — a nuanced IAM pattern. Triaging 5 findings requires judgment about severity and which finding is the actual compromise vs. reconnaissance.

#### 2. `aws-cloudtrail-investigation`
- **Title:** Investigate unauthorized activity using CloudTrail logs
- **Description:** CloudTrail logs show suspicious API activity over the past hour. Someone created new IAM users, attached AdministratorAccess policies, launched EC2 instances in an unusual region, and attempted to disable CloudTrail logging. The trail itself has been partially misconfigured (S3 bucket delivery was changed). Agent must: query CloudTrail events to reconstruct the attack timeline, identify the compromised principal, revert the trail configuration, remove unauthorized IAM users and their policies, terminate rogue EC2 instances, and add a Config rule to detect future trail modifications.
- **Track:** runtime-security | **Level:** L4 (Investigate)
- **Tags:** aws, cloudtrail, investigation, incident-response, config, security-specialty
- **Why hard:** Full forensic investigation. Agent must correlate multiple event types across services (IAM, EC2, CloudTrail, S3) to reconstruct what happened. Must use `aws cloudtrail lookup-events` with correct filters. Attack cleanup has ordering constraints: fix the trail first (to log the remediation), then remove unauthorized access. Highest difficulty because the agent must determine "what happened" from raw data.

#### 3. `aws-eventbridge-incident-automation`
- **Title:** Build an automated incident response pipeline using EventBridge and Lambda
- **Description:** The security team wants automated responses to common events: auto-quarantine EC2 instances that trigger specific GuardDuty finding types, auto-revoke public S3 bucket policies, and send formatted alerts to SNS. Currently no automation exists. Agent must: create EventBridge rules with correct event patterns for each scenario, write Lambda functions for remediation actions, configure proper IAM execution roles with least privilege, wire SNS for notifications, and test the pipeline by triggering a simulated event.
- **Track:** runtime-security | **Level:** L3 (Judge)
- **Tags:** aws, eventbridge, lambda, guardduty, automation, security-specialty, devops-pro
- **Why hard:** Multi-service orchestration. Agent must write correct EventBridge event patterns (JSON matching syntax is finicky), create Lambda code that handles each event type, build least-privilege IAM roles for Lambda execution (not `*` permissions), and wire everything together. Tests the intersection of security automation (SCS-C03 Domain 2) and event-driven architecture (DOP-C02 Domain 5).

### IAM & Access Control (3 scenarios)

#### 4. `aws-iam-policy-evaluation`
- **Title:** Debug an IAM permission denial caused by conflicting policies
- **Description:** A Lambda function can't read from an S3 bucket despite having an IAM role with `s3:GetObject` permission. The actual cause is a combination of: an SCP on the organizational unit denying `s3:*` except from a VPC endpoint, an S3 bucket policy requiring a specific condition key, a permission boundary on the role limiting actions, and a missing VPC endpoint policy. Agent must trace the IAM policy evaluation chain, identify which layer is blocking access, and fix the configuration while maintaining the security intent of each layer.
- **Track:** pod-security | **Level:** L4 (Investigate)
- **Tags:** aws, iam, policy-evaluation, scp, permission-boundary, vpc-endpoint, security-specialty
- **Why hard:** IAM policy evaluation is the hardest topic in AWS security. The agent must understand the evaluation order: SCP → resource policy → permission boundary → identity policy → session policy. Four different policy types interact to create the denial. Fixing one layer without understanding the others will break security. Tests deep IAM expertise that separates professional from beginner level.

#### 5. `aws-cross-account-access-broken`
- **Title:** Fix broken cross-account access between a trust policy and assume-role chain
- **Description:** Account B's Lambda needs to read DynamoDB in Account A. An IAM role in Account A has a trust policy for Account B, and Account B's Lambda has a role with `sts:AssumeRole`. But it fails with `AccessDenied`. Three problems: the trust policy condition requires `ExternalId` but the calling code doesn't pass it, the assumed role's session policy limits `dynamodb:GetItem` but the code uses `BatchGetItem`, and Account A has an SCP that denies all actions from principals that haven't authenticated with MFA (which Lambda execution roles can't use). Agent must fix all three issues.
- **Track:** pod-security | **Level:** L3 (Judge)
- **Tags:** aws, iam, cross-account, sts, assume-role, security-specialty
- **Why hard:** Cross-account access has multiple trust boundaries. Agent must fix trust policy conditions, update session policy, and solve the SCP/MFA conflict (by adding a condition exception for service-linked roles or specific role ARNs). Each fix alone doesn't resolve the issue — all three must be addressed. Tests the most common source of cross-account AWS bugs.

#### 6. `aws-secrets-rotation-broken`
- **Title:** Fix a broken Secrets Manager automatic rotation that's causing application outages
- **Description:** Secrets Manager rotation Lambda runs but the application crashes every rotation cycle. The rotation function creates a new secret version (`AWSPENDING`) but fails to finalize it (never moves to `AWSCURRENT`). Meanwhile the application caches the old secret indefinitely. Three issues: the rotation Lambda has insufficient IAM permissions to call `FinishSecret`, the secret's resource policy blocks the Lambda's execution role, and the RDS password change succeeds but the `AWSPENDING` stage is never promoted. Agent must fix the Lambda IAM role, update the secret resource policy, test the rotation step-by-step, and verify the application can read the new credentials.
- **Track:** pod-security | **Level:** L3 (Judge)
- **Tags:** aws, secrets-manager, rotation, lambda, iam, security-specialty
- **Why hard:** Secrets rotation has a 4-step lifecycle (create → set → test → finish) that most practitioners don't fully understand. The agent must know the rotation Lambda contract, debug each step, and understand the interaction between IAM permissions, resource policies, and the rotation workflow. A common cause of production outages when rotation is first enabled.

### Infrastructure Security & Networking (3 scenarios)

#### 7. `aws-vpc-flow-log-analysis`
- **Title:** Diagnose a connectivity failure using VPC Flow Logs and network ACL analysis
- **Description:** An EC2 application can't reach an RDS instance in a different subnet. Security groups look correct. VPC Flow Logs show `REJECT` entries. The actual causes: a network ACL on the RDS subnet has an explicit deny for the application subnet's CIDR (left from a previous incident response), and the route table for the application subnet is missing a route to the RDS subnet's CIDR (someone deleted it while cleaning up a VPC peering). Agent must: analyze flow logs to identify the rejection point, trace the network path, fix the NACL rules, restore the route table entry, and verify connectivity.
- **Track:** networking | **Level:** L3 (Judge)
- **Tags:** aws, vpc, flow-logs, nacl, route-table, troubleshooting, security-specialty
- **Why hard:** Network troubleshooting with multiple failure points. Agent must read flow log output format (srcaddr, dstaddr, action fields), understand NACL statelessness (must fix both inbound and outbound rules), and distinguish between SG denials (no flow log entry) vs. NACL denials (REJECT in flow logs). Two independent issues compound the failure.

#### 8. `aws-waf-rules-misconfigured`
- **Title:** Fix WAF rules that are blocking legitimate traffic while allowing malicious requests
- **Description:** A WAF WebACL attached to an ALB is misconfigured. Legitimate API requests containing JSON bodies with the word "select" are being blocked by an overly broad SQL injection rule. Meanwhile, actual malicious requests with encoded payloads (`%27%20OR%201%3D1`) are passing through because the rule doesn't inspect URL-decoded bodies. Agent must: analyze WAF logs to identify false positives, reconfigure the SQL injection rule with proper text transformations (URL_DECODE, HTML_ENTITY_DECODE), add scope-down statements to exclude the legitimate API path, create a rate-based rule to catch the actual attack pattern, and test both legitimate and malicious requests.
- **Track:** networking | **Level:** L3 (Judge)
- **Tags:** aws, waf, alb, security, web-application, security-specialty
- **Why hard:** WAF rule authoring requires understanding text transformations, rule priority, and scope-down logic. Agent must balance security (blocking real attacks) against availability (allowing legitimate traffic). The SQL injection false positive is a notorious real-world WAF problem. Tests the judgment to tune rules rather than just enable/disable them.

#### 9. `aws-kms-key-policy-lockout`
- **Title:** Recover from a KMS key policy that locked out all principals
- **Description:** Someone updated a KMS key policy and accidentally removed the root account's access. Now no IAM user or role can encrypt, decrypt, or manage the key. S3 objects encrypted with this key are inaccessible, Lambda functions using KMS for environment variable encryption are failing, and Secrets Manager secrets encrypted with this key can't be rotated. Agent must: use the account root (simulated) to restore the key policy, fix the policy to allow the required principals while following least privilege, re-enable any disabled KMS key aliases, verify S3 objects are accessible again, and add a Config rule to prevent future key policy lockouts.
- **Track:** pod-security | **Level:** L3 (Judge)
- **Tags:** aws, kms, key-policy, encryption, data-protection, security-specialty
- **Why hard:** KMS key policy lockout is one of the most dangerous AWS misconfigurations. Unlike IAM policies, KMS key policies are the *primary* authorization mechanism — IAM policies alone can't grant KMS access if the key policy doesn't allow it. Agent must understand this fundamental difference. Recovery requires root account access. Tests understanding of KMS grant model which is distinct from standard IAM.

### Configuration Management & IaC (3 scenarios)

#### 10. `aws-cloudformation-stack-drift`
- **Title:** Detect and remediate CloudFormation stack drift without recreating resources
- **Description:** A CloudFormation stack managing 6 resources has drifted. Manual console changes modified: a Lambda function's runtime and memory, an SQS queue's visibility timeout, and a DynamoDB table's provisioned capacity. The stack is in `UPDATE_ROLLBACK_COMPLETE` state from a failed previous update. Agent must: detect drift on all resources, review each drift to decide whether to adopt manual changes into the template or revert to the template values, update the template accordingly, fix the stack state (may need to continue rollback or skip failed resources), and successfully update the stack to match the corrected template.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** aws, cloudformation, drift, iac, remediation, devops-pro
- **Why hard:** Stack drift + rollback failure is a compound problem. Agent must handle the failed state before drift remediation. Each drifted resource requires a judgment call (keep manual change vs. revert). The `UPDATE_ROLLBACK_COMPLETE` state limits what operations are available. Tests real-world CloudFormation operational recovery.

#### 11. `aws-cloudformation-nested-stack-failure`
- **Title:** Debug a nested CloudFormation stack deployment that failed mid-creation
- **Description:** A parent stack creates 3 nested stacks (networking, compute, database). The compute nested stack failed during creation because it references an output from the networking stack that doesn't exist (typo in the output name). The parent stack is in `CREATE_FAILED` state with the networking stack created, compute partially created (1 of 3 resources), and database never started. Agent must: identify the root cause from the nested stack events, fix the cross-stack reference, delete the failed stack (handling the partial resources), and redeploy successfully.
- **Track:** platform-eng | **Level:** L2 (Diagnose)
- **Tags:** aws, cloudformation, nested-stacks, cross-stack-references, troubleshooting, devops-pro
- **Why hard:** Nested stack errors are notoriously hard to debug — the parent stack shows a generic "embedded stack failed" error. Agent must drill into nested stack events. Deletion of partially-created stacks can fail if resources have dependencies. Cross-stack reference debugging requires understanding `Fn::GetAtt` and `Outputs` across templates.

#### 12. `aws-config-compliance-remediation`
- **Title:** Set up AWS Config rules with auto-remediation for compliance violations
- **Description:** An account has several compliance violations: unencrypted EBS volumes, S3 buckets without versioning, Lambda functions using deprecated runtimes, and IAM users with access keys older than 90 days. Agent must: deploy Config rules for each violation type, create SSM Automation documents for auto-remediation, wire Config rules to remediation actions, handle the IAM key rotation carefully (create new key → update applications → deactivate old key — not just delete), and verify all resources become compliant.
- **Track:** platform-eng | **Level:** L3 (Judge)
- **Tags:** aws, config, compliance, ssm, auto-remediation, devops-pro, security-specialty
- **Why hard:** Multi-service compliance orchestration. Each violation type needs a different remediation strategy. IAM key rotation has ordering constraints (deleting an active key breaks applications). SSM Automation documents have their own syntax and execution model. Tests both security knowledge (what's compliant) and DevOps automation (how to enforce it).

### Monitoring, Logging & Resilience (3 scenarios)

#### 13. `aws-cloudwatch-alarm-cascade`
- **Title:** Debug a CloudWatch alarm cascade that triggered a false auto-scaling event
- **Description:** A CloudWatch alarm for CPU utilization triggered an Auto Scaling policy that scaled the fleet from 3 to 12 instances unnecessarily. Investigation reveals: the alarm's evaluation period was 1 minute with 1 datapoint (too sensitive), a scheduled Lambda that processes batch jobs spiked CPU to 95% for 30 seconds, the scaling policy has no cooldown period, and there's no scale-in alarm to bring instances back down. Agent must: fix the alarm to use a more appropriate evaluation period and datapoints-to-alarm, add a cooldown to the scaling policy, create a scale-in alarm, terminate the excess instances, and set up a composite alarm that requires both high CPU and high network traffic before scaling.
- **Track:** troubleshooting | **Level:** L3 (Judge)
- **Tags:** aws, cloudwatch, auto-scaling, alarms, composite-alarm, devops-pro
- **Why hard:** Alarm tuning is a judgment call — too sensitive causes false triggers, too lenient misses real issues. Agent must understand CloudWatch statistics (Average vs. Maximum), evaluation periods, datapoints-to-alarm, and composite alarm logic. The scaling policy configuration requires understanding cooldown periods and step scaling vs. target tracking. Common production incident pattern.

#### 14. `aws-lambda-dlq-backlog`
- **Title:** Diagnose and recover a Lambda function with a growing dead-letter queue backlog
- **Description:** An SQS-triggered Lambda function has been failing silently. The DLQ has 2,500 messages accumulated over 3 days. The function itself throws intermittent errors: sometimes it's a timeout (15 seconds isn't enough for large messages), sometimes it's a permissions error (the function can't write to a DynamoDB table), and sometimes it's an OOM kill (128MB isn't enough). Agent must: analyze the DLQ messages to categorize failure types, fix each root cause (increase timeout, fix IAM permissions, increase memory), reprocess the DLQ messages back to the main queue, set up a CloudWatch alarm for DLQ depth, and add proper error handling in the Lambda code to distinguish retriable vs. non-retriable errors.
- **Track:** troubleshooting | **Level:** L3 (Judge)
- **Tags:** aws, lambda, sqs, dlq, troubleshooting, error-handling, devops-pro
- **Why hard:** Three different failure modes require three different fixes. Agent must categorize DLQ messages by error type, prioritize fixes (permissions first, then timeout, then memory — applying them in wrong order means reprocessed messages fail again). DLQ reprocessing must be done after all fixes are applied. Tests diagnostic skill across Lambda, SQS, DynamoDB, and IAM simultaneously.

#### 15. `aws-s3-replication-broken`
- **Title:** Fix a broken S3 cross-region replication that's silently dropping objects
- **Description:** S3 cross-region replication was configured between a source and destination bucket. New objects in the source are not appearing in the destination. No errors are visible in the S3 console. Investigation reveals multiple issues: the replication IAM role is missing `s3:GetObjectVersionForReplication` (versioning was recently enabled), the destination bucket has a bucket policy that denies writes from outside its own region, the source bucket's replication configuration filters by a prefix that was recently changed, and the destination bucket has default encryption with a KMS key the replication role can't use. Agent must fix all four issues and trigger reprocessing of failed replications.
- **Track:** troubleshooting | **Level:** L3 (Judge)
- **Tags:** aws, s3, replication, cross-region, kms, iam, security-specialty
- **Why hard:** S3 replication failures are silent — no CloudWatch alarms by default. Four independent misconfigurations compound the problem. Agent must understand replication prerequisites: versioning, IAM permissions including the replication-specific actions, destination bucket policies, KMS cross-region key access. Each fix alone doesn't resolve the issue. Common in organizations that enable replication for compliance then discover it wasn't working.

---

## Excluded Ideas (overlap with existing scenarios)

| Excluded Idea | Overlaps With | Why excluded |
|---|---|---|
| S3 bucket encryption enforcement | `s3-bucket-public-access` | Both are S3 policy fixes — too similar in approach (modify bucket configuration) |
| Security group CIDR restriction | `security-group-too-open` | Same concept: tighten overly permissive SG rules |
| Basic IAM user permission fix | `s3-bucket-public-access` | That scenario already tests IAM + S3 interaction at L2 level |

---

## Implementation Priority

**High priority (professional-level multi-service troubleshooting, distinct from existing L2 scenarios):**
1. `aws-iam-policy-evaluation` — the hardest AWS topic, tests deep IAM expertise
2. `aws-cloudtrail-investigation` — full forensic investigation, professional-only skill
3. `aws-cloudwatch-alarm-cascade` — common production incident, judgment-heavy
4. `aws-guardduty-findings-triage` — incident response workflow, multi-phase

**Medium priority (complex multi-service scenarios):**
5. `aws-lambda-dlq-backlog` — multi-failure diagnosis across Lambda/SQS/DynamoDB
6. `aws-kms-key-policy-lockout` — dangerous misconfiguration, unique authorization model
7. `aws-cloudformation-stack-drift` — compound IaC failure, judgment required
8. `aws-vpc-flow-log-analysis` — network troubleshooting with multiple failure points
9. `aws-cross-account-access-broken` — multi-layered trust boundary debugging
10. `aws-secrets-rotation-broken` — rotation lifecycle debugging

**Lower priority (still professional, narrower scope):**
11. `aws-config-compliance-remediation` — compliance automation orchestration
12. `aws-eventbridge-incident-automation` — security automation pipeline
13. `aws-waf-rules-misconfigured` — WAF rule tuning
14. `aws-cloudformation-nested-stack-failure` — cross-stack reference debugging
15. `aws-s3-replication-broken` — silent replication failure diagnosis

---

## LocalStack Feasibility

All 15 scenarios use LocalStack, same as the existing AWS scenarios. No cloud credentials needed.

### LocalStack Community Edition Coverage

| Service | Available | Scenarios Using It |
|---|---|---|
| IAM | Yes | 1, 2, 4, 5, 6, 12, 14, 15 |
| S3 | Yes | 2, 9, 15 |
| Lambda | Yes | 3, 6, 10, 13, 14 |
| CloudFormation | Yes | 10, 11 |
| SQS/SNS | Yes | 3, 14 |
| DynamoDB | Yes | 5, 14 |
| CloudWatch | Yes | 13 |
| KMS | Yes | 9, 15 |
| Secrets Manager | Yes | 6 |
| EC2 (basic) | Yes | 1, 2, 7 |
| EventBridge | Yes | 3 |
| Config | Yes (basic) | 2, 12 |
| CloudTrail | Yes (basic) | 1, 2 |

### LocalStack Pro Required

| Service | Scenarios | Notes |
|---|---|---|
| GuardDuty | 1, 3 | Findings can be simulated via fixture injection |
| WAF | 8 | Rule evaluation can be partially simulated |
| VPC Flow Logs | 7 | Can simulate via fixture log files |
| SSM Automation | 12 | Document creation may need mocking |

**Mitigation:** For Pro-required services, scenarios can use fixture-based simulation:
pre-created findings/logs/events that the agent queries and acts on, rather than
requiring real-time service behavior. This matches how the existing scenarios use
LocalStack — the harness sets up the broken state, agent diagnoses and fixes it.

### Each scenario's fixtures include:
- `setup.sh` — creates the initial AWS state (resources, policies, configurations)
- `break.sh` — introduces the failure (misconfigure policies, inject drift, corrupt state)
- `verify.sh` — checks that the agent fixed all issues and resources are functional
- Pre-created fixture data (CloudTrail events, GuardDuty findings, flow logs) where real-time service behavior isn't available

### No overlap with existing scenarios

| This doc | Existing | Why distinct |
|---|---|---|
| All 15 ideas | `s3-bucket-public-access` | That scenario tests one simple bucket policy change. These test multi-service professional scenarios. |
| All 15 ideas | `security-group-too-open` | That scenario tests one SG rule change. These involve forensic investigation, multi-layer policy debugging, and cross-service orchestration. |
| IAM scenarios | Both existing | Existing scenarios use IAM as a side effect. These test IAM as the primary problem domain (policy evaluation chains, cross-account trust, SCP interactions). |
