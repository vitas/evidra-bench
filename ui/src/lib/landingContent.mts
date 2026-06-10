import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  BENCH_ONLINE_PATH,
  BENCH_SAMPLE_REPORT_PATH,
} from "./routes.mts";

export const BENCH_PRIVATE_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Private%20Agent%20Benchmark%20Request";
export const BENCH_SPONSOR_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Sponsored%20Public%20Benchmark%20Run";

export type LandingCta = {
  href: string;
  kind: "primary" | "secondary" | "tertiary" | "quiet";
  label: string;
};

export const HERO_CONTENT = {
  proofChips: [
    "Private readiness reports",
    "Live failure scenarios",
    "Tool traces and transcripts",
    "Unsafe-pass autopsy",
    "Vendor/model comparisons",
  ],
  title: "Independent evaluations for enterprise AI agents before production",
  body:
    "Evidra Bench runs AI agents through live infrastructure incidents before production, captures the tool path, transcripts, artifacts, and safety findings, then delivers rollout evidence for buyers and implementation teams.",
  ctas: [
    {
      href: BENCH_PRIVATE_REQUEST_MAILTO,
      kind: "primary",
      label: "Book private evaluation",
    },
    {
      href: BENCH_SAMPLE_REPORT_PATH,
      kind: "secondary",
      label: "View sample report",
    },
    {
      href: BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
      kind: "tertiary",
      label: "Read methodology",
    },
    {
      href: BENCH_ONLINE_PATH,
      kind: "quiet",
      label: "Open online bench",
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
    title: "Agent Deployment Readiness Report",
    desc: "A private rollout-risk report for one agent, model route, or toolchain before it touches enterprise infrastructure.",
  },
  {
    title: "Vendor Selection Benchmark",
    desc: "Compare agent vendors, MCP servers, models, or implementation approaches against the same live scenario slice.",
  },
  {
    title: "Release Regression Gate",
    desc: "Rerun fixed agent scenarios before model, prompt, MCP, or tool upgrades create a production regression.",
  },
  {
    title: "Custom Incident Scenario Pack",
    desc: "Convert sanitized customer incidents into private scenarios with deterministic setup, break, and verifier steps.",
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
