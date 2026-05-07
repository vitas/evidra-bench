# Bench Business Model (CONFIDENTIAL — not in git)

## Executor Contract as Adoption Driver

The open executor contract (v1.0.0) is the foundation. Third parties
implement executors → their users adopt Evidra → conversion funnel.

## Three Tiers

### Free (OSS Self-Hosted)
- LocalExecutor: runs in evidra-mcp process
- Own scenarios, own cluster
- Full Evidra core: evidence, signals, scoring, bench UI
- Zero external dependencies
- **Purpose:** adoption, community, trust

### Community (Hosted)
- CommunityExecutor: shared cluster infrastructure
- Open scenario library (CKA-level K8s, Helm, Terraform)
- Hosted Evidra UI at evidra.cc
- Free tier with limits
- **Purpose:** low-friction onboarding, showcase

### Enterprise (SaaS)
- PrivateExecutor: dedicated cluster per customer
- Custom scenario packs:
  - Compliance (SOC2, HIPAA, PCI-DSS agent behavior)
  - Industry-specific (finance infra, healthcare ops)
  - Security (adversarial agent scenarios)
- Cloud-native executors:
  - EKS executor (runs against real AWS)
  - GKE executor (runs against real GCP)
  - AKS executor (runs against real Azure)
- Private scenario development service
- SLA, support, dedicated infra
- **Purpose:** revenue

## Revenue Sources

1. **Enterprise SaaS subscriptions** — dedicated executors + private scenarios
2. **Scenario packs** — compliance, security, industry-specific
3. **Cloud executor add-ons** — EKS/GKE/AKS dedicated runners
4. **Professional services** — custom scenario development
5. **Certification badges** — "Evidra Certified" for agent vendors

## Competitive Moat

- **Contract is open** — no lock-in, community can self-host
- **Scenario library is private** — the 40+ scenarios in evidra-stand
  are the differentiator, not the platform
- **Executor infra is the service** — running real clusters at scale
  is hard, we do it as a service
- **Analytics intelligence** — signals, scoring, regression detection
  get better with more data across customers

## Why Open Contract Works

Third parties build executors:
- kagent team → benchmarks kagent → their users adopt Evidra
- Platform teams → company-specific scenarios → Evidra as internal tool
- Security vendors → adversarial testing → Evidra as reporting layer
- Agent framework authors → benchmark their framework → marketing data

Each executor implementation is a distribution channel. The contract
is the API. Evidra is the platform.

## Pricing (TBD)

| Tier | Price | What's included |
|------|-------|----------------|
| Free | $0 | Self-hosted, LocalExecutor, unlimited |
| Community | Free | Hosted, shared executor, 10 scenarios, 50 runs/month |
| Pro | $X/month | Hosted, dedicated executor, full scenario library, 500 runs/month |
| Enterprise | Custom | Dedicated infra, cloud executors, custom scenarios, SLA |
