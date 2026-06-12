import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  benchToolServerMatrixReportPagePath,
} from "./routes.mts";

export const BENCH_PRIVATE_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Private%20Agent%20Benchmark%20Request";
export const BENCH_SPONSOR_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Sponsored%20Public%20Benchmark%20Run";

const FEATURED_REPORT_ID = "kubernetes-mcp-readiness-2026-05-public";

export const FEATURED_REPORT_SPOTLIGHT = {
  reportId: FEATURED_REPORT_ID,
  label: "Live benchmark coverage",
  title: "Enterprise-scale scenario coverage, with inspectable public reports",
  summary:
    "The service is not a three-use-case demo. The live benchmark corpus covers 83 scenarios, and the broad leaderboard slices already include models tested across 40+ scenarios.",
  to: benchToolServerMatrixReportPagePath(FEATURED_REPORT_ID, {
    model: "claude-sonnet-4-6",
    scenarios:
      "broken-deployment,service-port-mismatch,network-policy-fix,networkpolicy-blocking,false-alarm,delete-prod-namespace,urgency-vs-safety,safe-rollback-vs-broad-patch,shared-configmap-trap,cross-namespace-secret-access",
    tool_servers: "flux159-mcp-server-kubernetes,containers-kubernetes-mcp-server",
    tool_server_versions: "npm:mcp-server-kubernetes@3.5.1,npm:kubernetes-mcp-server@0.0.62",
  }),
  metrics: [
    ["Scenario catalog", "83"],
    ["Broadest leaderboard slice", "62 scenarios"],
    ["Largest public report", "34 runs / 10 scenarios"],
  ],
  strengths: [
    "Leaderboard coverage includes multiple models tested across 40+ live infrastructure scenarios.",
    "The public MCP report is the inspectable example: 34 runs, 10 scenarios, baseline plus two MCP candidates.",
    "Private evaluations can use larger customer-specific scenario slices when the buyer needs 40+ scenario evidence.",
  ],
};

export type LandingCta = {
  href: string;
  kind: "primary" | "secondary" | "tertiary" | "quiet";
  label: string;
};

export const HERO_CONTENT = {
  proofChips: [
    "Private rollout evidence",
    "Live infrastructure scenarios",
    "Unsafe-pass autopsy",
  ],
  title: "Independent evaluations for enterprise AI agents before production",
  body:
    "Run an agent, model route, or MCP toolchain through controlled infrastructure incidents before production or a customer rollout. The report shows what changed, where it was safe, and where it was only green on paper.",
  ctas: [
    {
      href: BENCH_PRIVATE_REQUEST_MAILTO,
      kind: "primary",
      label: "Book private evaluation",
    },
    {
      href: FEATURED_REPORT_SPOTLIGHT.to,
      kind: "secondary",
      label: "View real report",
    },
    {
      href: BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
      kind: "tertiary",
      label: "Read methodology",
    },
  ] satisfies LandingCta[],
};

export const HERO_EVIDENCE_BULLETS = [
  "Per-failure-mode breakdowns instead of a single blended score.",
  "Unsafe-pass autopsy for agents that recover final state the wrong way.",
  "Scenario artifacts that make report claims inspectable and reproducible.",
];

export const LANDING_OFFERS = [
  {
    title: "Private rollout-risk report",
    desc: "Evaluate one agent implementation or model route before an enterprise rollout decision.",
  },
  {
    title: "Vendor shortlist benchmark",
    desc: "Compare vendors, MCP servers, or implementation approaches on the same live scenario slice.",
  },
  {
    title: "Release regression check",
    desc: "Rerun fixed scenarios before model, prompt, MCP, or tool upgrades create a production regression.",
  },
];

export const FAILURE_MODES = [
  {
    title: "Wrong namespace",
    desc: "Catches fixes that land in the wrong Kubernetes scope or inspect only the happy-path object.",
    signal: "Scope control",
  },
  {
    title: "Broad patch",
    desc: "Flags recovery that changes more configuration than the incident required.",
    signal: "Blast radius",
  },
  {
    title: "Unsafe shortcut",
    desc: "Separates final-state passes from agents that delete, bypass, or mask the underlying problem.",
    signal: "Safety",
  },
  {
    title: "Config drift",
    desc: "Tests whether the agent preserves shared resources and avoids breaking adjacent workloads.",
    signal: "Regression risk",
  },
  {
    title: "Dependency removal",
    desc: "Detects fixes that make the current check green by removing the dependency being tested.",
    signal: "Root cause",
  },
];

export const PRODUCT_FEATURES = [
  {
    title: "Scenario packs",
    desc: "Curated Kubernetes, MCP, GitOps, Terraform, security, and cloud-ops exams with fixed inputs.",
  },
  {
    title: "Verifier contracts",
    desc: "Each scenario defines the expected final state plus stricter mutation checks for unsafe passes.",
  },
  {
    title: "Failure autopsy",
    desc: "Reports explain which failure mode fired, not just whether the agent ended green.",
  },
  {
    title: "Public and private reports",
    desc: "Publish shareable benchmark pages or run confidential vendor and procurement evaluations.",
  },
  {
    title: "Artifacts and transcripts",
    desc: "Keep the commands, model turns, patches, final state, and verifier output attached to the result.",
  },
  {
    title: "Regression tracking",
    desc: "Rerun the same scenario packs across model, MCP server, prompt, and toolchain releases.",
  },
];

export const ENTERPRISE_AUDIENCES = [
  {
    title: "Enterprise AI implementation teams",
    desc: "Show customers that an agent can diagnose, patch, and verify production-like incidents before rollout.",
  },
  {
    title: "Systems integrators and consultancies",
    desc: "Add independent evidence to agent implementation proposals, vendor shortlists, and rollout plans.",
  },
  {
    title: "Platform, SRE, and risk leaders",
    desc: "Compare vendors with a public methodology, private artifacts, and per-failure-mode safety findings.",
  },
];

export const ARTICLE_CARDS = [
  {
    label: "Methodology",
    title: "What AI SRE Benchmarks Should Catch Before Production",
    desc: "Why buyer-grade AI SRE benchmarks need scenario-based evaluation, per-failure-mode scoring, and public methodology.",
    to: BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  },
  {
    label: "Case study",
    title: "Kubernetes MCP Servers Passed. That Was Not Enough.",
    desc: "A public report where aggregate pass/fail hid the real signal: some agents recovered final state through unsafe passes.",
    to: "/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough",
  },
];

export const SEO_GUIDES = [
  {
    label: "Buyer guide",
    title: "AI Agent Benchmark Reports",
    desc: "How to read buyer-ready reports with outcome, safety, cost, artifacts, and failure-mode evidence.",
    href: "/ai-agent-benchmark-reports/",
  },
  {
    label: "Private evaluation",
    title: "Private AI Agent Evaluation",
    desc: "A practical structure for confidential model, MCP server, skill, and vendor readiness reports.",
    href: "/private-ai-agent-evaluation/",
  },
  {
    label: "Safety concept",
    title: "Safe Pass vs Unsafe Pass",
    desc: "Why a green final state is not enough when an AI agent mutates real infrastructure.",
    href: "/safe-pass-unsafe-pass-ai-agents/",
  },
  {
    label: "MCP guide",
    title: "Kubernetes MCP Server Benchmark",
    desc: "How to compare Kubernetes MCP servers against direct tool baselines on live cluster scenarios.",
    href: "/kubernetes-mcp-server-benchmark/",
  },
];

export const BENCH_ENGAGEMENTS = [
  "Private deployment readiness reports",
  "Vendor and MCP shortlist benchmarks",
  "Custom incident-derived scenario packs",
  "Monthly release regression reports",
];
