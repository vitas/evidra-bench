import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  BENCH_ONLINE_PATH,
  BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH,
} from "./routes.mts";

export const BENCH_PRIVATE_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Private%20Agent%20Benchmark%20Request";

export type LandingCta = {
  href: string;
  kind: "primary" | "secondary" | "quiet";
  label: string;
};

export const HERO_CONTENT = {
  eyebrow: "Path-aware infrastructure benchmark",
  title: "A green check doesn't mean the agent was safe.",
  body:
    "Most benchmarks measure whether the task was completed. Evidra also verifies how the agent got there—catching wrong-scope changes, unsafe shortcuts, skipped verification, and hidden regressions in live infrastructure.",
  ctas: [
    {
      href: BENCH_ONLINE_PATH,
      kind: "primary",
      label: "Open public Bench",
    },
    {
      href: BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH,
      kind: "quiet",
      label: "See a safe vs unsafe pass",
    },
  ] satisfies LandingCta[],
};

export const HERO_PROOF_POINTS = [
  "Final state + action path",
  "Safe pass vs unsafe pass",
  "Inspectable artifacts for every verdict",
];

export const PATH_COMPARISON = {
  title: "Same final result. Different operational risk.",
  rows: [
    { label: "Infrastructure restored", safe: "Pass", unsafe: "Pass" },
    { label: "Action path", safe: "Safe", unsafe: "Unsafe" },
    { label: "Wrong-scope changes", safe: "None", unsafe: "Production modified" },
    { label: "Verification", safe: "Complete", unsafe: "Skipped" },
  ],
};

export const BENCHMARK_DIFFERENCES = [
  {
    label: "Outcome",
    typical: "Final answer or final state",
    evidra: "Live final state and mutation path",
  },
  {
    label: "Verdict",
    typical: "Aggregate pass/fail score",
    evidra: "Safe pass, unsafe pass, fail, and error",
  },
  {
    label: "Evidence",
    typical: "Opaque or detached verdict",
    evidra: "Deterministic checks with linked artifacts",
  },
  {
    label: "Comparison target",
    typical: "Model-only comparison",
    evidra: "Models, agents, skills, MCP servers, and runtimes",
  },
];

export const WORKFLOW_STEPS = [
  {
    title: "Choose the stack",
    body: "Select an agent, model, skill, MCP server, or runtime to evaluate.",
  },
  {
    title: "Run a live incident",
    body: "Evidra injects a controlled failure into real infrastructure and lets the agent act.",
  },
  {
    title: "Inspect the evidence",
    body: "Review final state, action path, cost, findings, transcripts, and verifier artifacts.",
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
