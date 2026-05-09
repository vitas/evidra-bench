import { useState, useEffect, useRef } from "react";
import { Link } from "react-router";
import { useTheme } from "../hooks/useTheme";
import { SCENARIOS } from "../data/catalog";
import { EXAM_PACKS, countExamPackMatches } from "../lib/examPacks.mts";
import {
  BENCH_LEADERBOARD_PATH,
  BENCH_SCENARIOS_PATH,
  benchLeaderboardPagePath,
  benchScenariosPagePath,
} from "../lib/routes.mts";

const TERMINAL_LINES = [
  { text: "$ bench-cli certify --track pod-security --model sonnet", delay: 0, type: "input" as const },
  { text: "", delay: 600, type: "blank" as const },
  { text: "[1/7] networkpolicy-blocking (L2) ...", delay: 800, type: "progress" as const },
  { text: "  PASS  12.3s", delay: 1400, type: "pass" as const },
  { text: "[2/7] network-policy-fix (L2) ...", delay: 1800, type: "progress" as const },
  { text: "  PASS  18.7s", delay: 2400, type: "pass" as const },
  { text: "[3/7] readonly-filesystem (L2) ...", delay: 2800, type: "progress" as const },
  { text: "  PASS  9.2s", delay: 3300, type: "pass" as const },
  { text: "[4/7] stale-sa-token (L2) ...", delay: 3700, type: "progress" as const },
  { text: "  PASS  14.1s", delay: 4200, type: "pass" as const },
  { text: "[5/7] security-group-too-open (L3) ...", delay: 4600, type: "progress" as const },
  { text: "  PASS  22.4s", delay: 5300, type: "pass" as const },
  { text: "[6/7] s3-bucket-public-access (L3) ...", delay: 5700, type: "progress" as const },
  { text: "  PASS  19.8s", delay: 6400, type: "pass" as const },
  { text: "[7/7] privileged-pod-review (L3) ...", delay: 6800, type: "progress" as const },
  { text: "  PASS  8.1s", delay: 7300, type: "pass" as const },
  { text: "", delay: 7600, type: "blank" as const },
  { text: "════════════════════════════════════════════", delay: 7800, type: "border" as const },
  { text: "  BENCH READINESS REPORT", delay: 8000, type: "title" as const },
  { text: "════════════════════════════════════════════", delay: 8200, type: "border" as const },
  { text: "  Agent:    sonnet (bifrost)", delay: 8400, type: "info" as const },
  { text: "  Track:    Pod Security", delay: 8600, type: "info" as const },
  { text: "", delay: 8800, type: "blank" as const },
  { text: "  Grade:    PROFICIENT (L3)", delay: 9000, type: "grade" as const },
  { text: "", delay: 9200, type: "blank" as const },
  { text: "  L2 Diagnose:   4/4   ✓", delay: 9400, type: "level-pass" as const },
  { text: "  L3 Judge:      3/3   ✓", delay: 9600, type: "level-pass" as const },
  { text: "", delay: 9800, type: "blank" as const },
  { text: "  Overall:  7/7 (100.0%)", delay: 10000, type: "info" as const },
  { text: "════════════════════════════════════════════", delay: 10200, type: "border" as const },
];

function TerminalAnimation() {
  const [visibleLines, setVisibleLines] = useState(0);
  const containerRef = useRef<HTMLDivElement>(null);

  useEffect(() => {
    const timers: ReturnType<typeof setTimeout>[] = [];
    TERMINAL_LINES.forEach((line, i) => {
      timers.push(setTimeout(() => {
        setVisibleLines(i + 1);
        if (containerRef.current) {
          containerRef.current.scrollTop = containerRef.current.scrollHeight;
        }
      }, line.delay));
    });
    return () => timers.forEach(clearTimeout);
  }, []);

  return (
    <div className="relative rounded-xl overflow-hidden border border-border shadow-[0_0_60px_rgba(52,211,153,0.08)]">
      {/* Terminal header */}
      <div className="flex items-center gap-2 px-4 py-2.5 bg-[#0a110e] border-b border-border">
        <div className="flex gap-1.5">
          <div className="w-2.5 h-2.5 rounded-full bg-[#ef4444]/70" />
          <div className="w-2.5 h-2.5 rounded-full bg-[#eab308]/70" />
          <div className="w-2.5 h-2.5 rounded-full bg-accent-bright/70" />
        </div>
        <span className="text-[0.65rem] text-fg-muted font-mono ml-2">bench-cli — live exam run</span>
      </div>
      {/* Terminal body */}
      <div
        ref={containerRef}
        className="bg-bg-alt p-4 font-mono text-[0.72rem] leading-relaxed h-[380px] overflow-hidden"
      >
        {TERMINAL_LINES.slice(0, visibleLines).map((line, i) => (
          <div
            key={i}
            className={`
              ${line.type === "input" ? "text-accent" : ""}
              ${line.type === "progress" ? "text-fg-muted" : ""}
              ${line.type === "pass" ? "text-accent-bright font-semibold" : ""}
              ${line.type === "border" ? "text-accent/60" : ""}
              ${line.type === "title" ? "text-fg font-bold" : ""}
              ${line.type === "grade" ? "text-accent font-bold text-[0.85rem]" : ""}
              ${line.type === "level-pass" ? "text-accent-bright" : ""}
              ${line.type === "info" ? "text-fg-body" : ""}
              ${line.type === "blank" ? "h-4" : ""}
            `}
            style={{ animation: "fadeInLine 0.15s ease-out" }}
          >
            {line.text}
          </div>
        ))}
        {visibleLines < TERMINAL_LINES.length && (
          <span className="inline-block w-2 h-4 bg-accent animate-pulse ml-0.5" />
        )}
      </div>
    </div>
  );
}

const EXAM_PACK_COUNTS = countExamPackMatches(SCENARIOS);
const BENCH_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Live%20Agent%20Benchmark%20Request";
const BENCH_ENGAGEMENTS = [
  "Private agent/MCP evaluation reports",
  "Sponsored public benchmark runs",
  "Custom live scenario packs",
  "Monthly release regression reports",
];

export function Landing() {
  const { theme, toggle } = useTheme();
  const stats = [
    { value: String(SCENARIOS.length), label: "Scenarios" },
    { value: String(EXAM_PACKS.length), label: "Exam Suites" },
    { value: String(new Set(SCENARIOS.map((scenario) => scenario.category)).size), label: "Categories" },
    { value: "4", label: "Levels" },
  ];
  return (
    <div className="min-h-screen bg-bg text-fg overflow-hidden">
      {/* Subtle grid background */}
      <div
        className="fixed inset-0 opacity-[0.03]"
        style={{
          backgroundImage: `linear-gradient(var(--color-accent) 1px, transparent 1px), linear-gradient(90deg, var(--color-accent) 1px, transparent 1px)`,
          backgroundSize: "60px 60px",
        }}
      />

      {/* Top bar */}
      <div className="relative flex items-center justify-end gap-3 max-w-6xl mx-auto px-6 pt-4">
        <a
          href="https://github.com/vitas/evidra-bench"
          target="_blank"
          rel="noopener noreferrer"
          className="w-8 h-8 flex items-center justify-center rounded-md border border-border text-fg-muted hover:border-accent hover:text-accent transition-all"
          aria-label="GitHub"
        >
          <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
          </svg>
        </a>
        <button
          onClick={toggle}
          className="w-8 h-8 flex items-center justify-center rounded-md border border-border text-fg-muted hover:border-accent hover:text-accent transition-all cursor-pointer"
          style={{ background: "none", fontSize: "0.9rem" }}
          aria-label="Toggle theme"
        >
          {theme === "dark" ? "\u2600" : "\u263E"}
        </button>
      </div>

      {/* Hero */}
      <section className="relative max-w-6xl mx-auto px-6 pt-12 pb-16">
        {/* Glow effect */}
        <div className="absolute top-0 left-1/2 -translate-x-1/2 w-[600px] h-[400px] bg-accent/8 rounded-full blur-[120px]" />

        <div className="relative grid lg:grid-cols-2 gap-16 items-center">
          {/* Left: messaging */}
          <div>
            <Link
              to={BENCH_LEADERBOARD_PATH}
              className="inline-flex items-center gap-2 px-4 py-1.5 rounded-full border border-accent/40 bg-accent/10 text-[0.75rem] text-accent font-medium mb-8 hover:bg-accent/20 hover:border-accent/60 transition-all group"
            >
              <span className="w-1.5 h-1.5 rounded-full bg-accent-bright animate-pulse" />
              8 models tested — view live results
              <svg className="w-3 h-3 group-hover:translate-x-0.5 transition-transform" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5}>
                <path d="M5 12h14M12 5l7 7-7 7" />
              </svg>
            </Link>

            <h1 className="text-[3.2rem] leading-[1.08] font-extrabold tracking-tight mb-6">
              Live infra exams{" "}
              <span className="text-transparent bg-clip-text bg-gradient-to-r from-[#34d399] to-[#059669]">
                for AI agents
              </span>
            </h1>

            <p className="text-[1.05rem] text-fg-muted leading-relaxed mb-10 max-w-lg">
              Run models, MCP servers, and skills against real Kubernetes,
              GitOps, Terraform, and cloud-ops scenarios. See whether the agent
              diagnosed first, acted safely, and verified recovery.
            </p>

            <div className="flex items-center gap-4 mb-12">
              <Link
                to="/bench"
                className="inline-flex items-center gap-2 px-6 py-3 bg-accent hover:bg-accent-bright text-white text-[0.88rem] font-semibold rounded-lg transition-all hover:shadow-[0_0_30px_rgba(5,150,105,0.3)]"
              >
                Benchmark Dashboard
                <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5}>
                  <path d="M5 12h14M12 5l7 7-7 7" />
                </svg>
              </Link>
              <Link
                to={BENCH_LEADERBOARD_PATH}
                className="inline-flex items-center gap-2 px-6 py-3 border border-border text-fg-body text-[0.88rem] font-medium rounded-lg hover:border-accent/50 hover:text-fg transition-all"
              >
                Exam Leaderboard
              </Link>
            </div>

            {/* Stats row */}
            <div className="flex gap-8">
              {stats.map((stat) => (
                <div key={stat.label}>
                  <div className="text-2xl font-bold text-accent">{stat.value}</div>
                  <div className="text-[0.72rem] text-fg-muted uppercase tracking-wider">{stat.label}</div>
                </div>
              ))}
            </div>
          </div>

          {/* Right: terminal animation */}
          <div className="hidden lg:block">
            <TerminalAnimation />
          </div>
        </div>
      </section>

      {/* Three product pillars */}
      <section className="relative max-w-6xl mx-auto px-6 py-16">
        <div className="grid md:grid-cols-3 gap-4">
          {[
            {
              title: "Bench",
              desc: "Leaderboard, run detail, skill impact analysis, model comparison. The full benchmark dashboard.",
              to: "/bench",
              icon: "M3 3h7v7H3zM14 3h7v7h-7zM3 14h7v7H3z",
              tag: "Dashboard",
            },
            {
              title: "Lab",
              desc: "Current scenario catalog with track/level filters. Visual puzzle designer for authoring new scenarios. (Designer: Beta)",
              to: BENCH_SCENARIOS_PATH,
              icon: "M9.75 3.104v5.714a2.25 2.25 0 0 1-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.3 24.3 0 0 1 4.5 0m0 0v5.714a2.25 2.25 0 0 0 .659 1.591L19 14.5",
              tag: "Scenarios",
            },
            {
              title: "Exams",
              desc: "Public suite leaderboards for Kubernetes, security, GitOps, Terraform/cloud, and MCP readiness.",
              to: BENCH_LEADERBOARD_PATH,
              icon: "M4.26 10.147a60.436 60.436 0 0 0-.491 6.347A48.627 48.627 0 0 1 12 20.904a48.627 48.627 0 0 1 8.232-4.41 60.46 60.46 0 0 0-.491-6.347",
              tag: "Readiness",
            },
          ].map((p) => (
            <Link
              key={p.title}
              to={p.to}
              className="group relative p-6 glass-card transition-all"
            >
              <div className="flex items-center gap-3 mb-3">
                <div className="w-8 h-8 rounded-lg bg-accent/15 flex items-center justify-center text-accent">
                  <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={1.5}>
                    <path d={p.icon} />
                  </svg>
                </div>
                <h3 className="text-[1.05rem] font-bold text-fg group-hover:text-accent transition-colors">{p.title}</h3>
                <span className="ml-auto text-[0.6rem] uppercase tracking-widest text-fg-muted border border-border rounded px-2 py-0.5">{p.tag}</span>
              </div>
              <p className="text-[0.8rem] text-fg-muted leading-relaxed">{p.desc}</p>
            </Link>
          ))}
        </div>
      </section>

      {/* Exam levels */}
      <section className="relative max-w-6xl mx-auto px-6 py-20">
        <h2 className="text-center text-[1.6rem] font-bold mb-3">Four Exam Levels</h2>
        <p className="text-center text-[0.88rem] text-fg-muted mb-12 max-w-xl mx-auto">
          Not just pass/fail — we measure how your agent thinks. Each level tests deeper capabilities.
        </p>

        <div className="grid sm:grid-cols-2 lg:grid-cols-4 gap-4">
          {[
            { level: "L1", name: "Fix", desc: "Applies the obvious fix to a clear problem", color: "#3b82f6", analogy: "Junior" },
            { level: "L2", name: "Diagnose", desc: "Investigates before acting, reads logs, correlates", color: "#22c55e", analogy: "Mid-level" },
            { level: "L3", name: "Judge", desc: "Weighs trade-offs, avoids traps, scopes decisions", color: "#eab308", analogy: "Senior" },
            { level: "L4", name: "Investigate", desc: "Multi-step forensics, traces root cause across systems", color: "#ef4444", analogy: "Staff" },
          ].map((l) => (
            <div
              key={l.level}
              className="relative p-5 glass-card transition-all group"
            >
              <div
                className="absolute top-0 left-0 w-full h-0.5 rounded-t-xl"
                style={{ background: l.color }}
              />
              <div className="flex items-center gap-2 mb-3">
                <span
                  className="text-[0.65rem] font-bold px-2 py-0.5 rounded"
                  style={{ background: `${l.color}20`, color: l.color }}
                >
                  {l.level}
                </span>
                <span className="text-[0.88rem] font-semibold text-fg">{l.name}</span>
              </div>
              <p className="text-[0.78rem] text-fg-muted leading-relaxed mb-3">{l.desc}</p>
              <span className="text-[0.65rem] text-fg-muted uppercase tracking-wider">
                {l.analogy} engineer equivalent
              </span>
            </div>
          ))}
        </div>
      </section>

      {/* Public exam suites */}
      <section className="relative max-w-6xl mx-auto px-6 py-20">
        <h2 className="text-center text-[1.6rem] font-bold mb-3">Public Exam Suites</h2>
        <p className="text-center text-[0.88rem] text-fg-muted mb-12 max-w-xl mx-auto">
          Shareable slices of the live catalog, with matching scenario lists and leaderboard views.
        </p>

        <div className="grid sm:grid-cols-2 lg:grid-cols-3 gap-4">
          {EXAM_PACKS.map((pack) => (
            <div
              key={pack.id}
              className="glass-card p-5 transition-all group"
            >
              <div className="flex items-start justify-between gap-3 mb-3">
                <div>
                  <h3 className="text-[0.92rem] font-bold text-fg group-hover:text-accent transition-colors">
                    {pack.title}
                  </h3>
                  <p className="text-[0.76rem] text-fg-muted leading-relaxed mt-1">
                    {pack.summary}
                  </p>
                </div>
                <span className="font-mono text-[0.82rem] font-bold text-accent">
                  {EXAM_PACK_COUNTS[pack.id] ?? 0}
                </span>
              </div>
              <p className="text-[0.7rem] text-fg-muted leading-relaxed mb-4">
                {pack.proof}
              </p>
              <div className="flex items-center gap-2">
                <Link
                  to={benchScenariosPagePath({ exam: pack.id })}
                  className="text-[0.72rem] font-semibold px-3 py-1.5 rounded-md bg-accent-tint text-accent hover:bg-accent-subtle transition-colors"
                >
                  Catalog
                </Link>
                <Link
                  to={benchLeaderboardPagePath({ exam: pack.id })}
                  className="text-[0.72rem] font-semibold px-3 py-1.5 rounded-md border border-border text-fg-muted hover:text-fg hover:border-accent/50 transition-colors"
                >
                  Leaderboard
                </Link>
              </div>
            </div>
          ))}
        </div>
      </section>

      {/* How it works */}
      <section className="relative max-w-6xl mx-auto px-6 py-20">
        <h2 className="text-center text-[1.6rem] font-bold mb-12">How It Works</h2>

        <div className="grid md:grid-cols-3 gap-8">
          {[
            {
              step: "01",
              title: "Run without your skill",
              desc: "Baseline: same model, same scenarios, no skill prompt. Measure turns, tokens, pass rate.",
            },
            {
              step: "02",
              title: "Run with your skill",
              desc: "Add your skill prompt, MCP tool, or plugin. Same scenarios. Compare everything.",
            },
            {
              step: "03",
              title: "See what changed",
              desc: "Did turns drop? Did pass rate hold? Did L3 judgment scenarios break? Ship what works, cut what doesn't.",
            },
          ].map((item) => (
            <div key={item.step} className="relative">
              <div className="text-[3rem] font-black text-accent/10 absolute -top-4 -left-2 select-none">
                {item.step}
              </div>
              <div className="relative pt-8">
                <h3 className="text-[1rem] font-semibold text-fg mb-2">{item.title}</h3>
                <p className="text-[0.82rem] text-fg-muted leading-relaxed">{item.desc}</p>
              </div>
            </div>
          ))}
        </div>

        {/* CTA */}
        <div className="text-center mt-16">
          <code className="block text-[0.82rem] text-accent bg-bg-alt border border-border rounded-lg px-6 py-3 font-mono inline-block mb-6">
            bench-cli certify --track workloads --model your-agent --provider bifrost
          </code>
          <div className="flex justify-center gap-4">
            <Link
              to="/bench"
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-accent-tint text-fg text-[0.82rem] font-semibold rounded-lg hover:bg-white transition-colors"
            >
              Open Benchmark Dashboard
            </Link>
            <Link
              to={BENCH_SCENARIOS_PATH}
              className="inline-flex items-center gap-2 px-5 py-2.5 border border-border text-fg-body text-[0.82rem] font-medium rounded-lg hover:border-accent/50 transition-colors"
            >
              View All {SCENARIOS.length} Scenarios
            </Link>
            <Link
              to={BENCH_LEADERBOARD_PATH}
              className="inline-flex items-center gap-2 px-5 py-2.5 border border-border text-fg-body text-[0.82rem] font-medium rounded-lg hover:border-accent/50 transition-colors"
            >
              Exam Leaderboard
            </Link>
          </div>
        </div>
      </section>

      {/* Commercial CTA */}
      <section className="relative max-w-6xl mx-auto px-6 py-20">
        <div className="grid lg:grid-cols-[1.1fr_0.9fr] gap-8 items-start border-y border-border py-10">
          <div>
            <span className="inline-flex items-center gap-2 px-3 py-1 rounded-full border border-accent/30 bg-accent/10 text-[0.68rem] font-semibold uppercase tracking-wider text-accent mb-5">
              Independent live evaluation
            </span>
            <h2 className="text-[1.8rem] md:text-[2.2rem] leading-tight font-extrabold mb-4">
              Commission a Live Agent Benchmark
            </h2>
            <p className="text-[0.95rem] text-fg-muted leading-relaxed max-w-2xl mb-4">
              Building an AI SRE agent, MCP server, or infrastructure automation tool?
            </p>
            <p className="text-[0.9rem] text-fg-muted leading-relaxed max-w-2xl">
              Evidra Bench can run an independent live evaluation against curated
              Kubernetes, Helm, Terraform, and GitOps scenarios.
            </p>
          </div>

          <div className="border border-border rounded-lg bg-bg-alt/70 p-5">
            <h3 className="text-[0.82rem] font-semibold uppercase tracking-wider text-fg-muted mb-4">
              Available engagements
            </h3>
            <ul className="space-y-3 mb-5">
              {BENCH_ENGAGEMENTS.map((item) => (
                <li key={item} className="flex items-start gap-3 text-[0.85rem] text-fg-body">
                  <span className="mt-1.5 w-1.5 h-1.5 rounded-full bg-accent-bright shrink-0" />
                  <span>{item}</span>
                </li>
              ))}
            </ul>
            <p className="text-[0.78rem] text-fg-muted leading-relaxed mb-5">
              Private reports are confidential. Sponsored reports are clearly
              labeled, and sponsors do not control scoring or findings.
            </p>
            <a
              href={BENCH_REQUEST_MAILTO}
              className="inline-flex items-center gap-2 px-5 py-2.5 bg-accent hover:bg-accent-bright text-white text-[0.84rem] font-semibold rounded-lg transition-all hover:shadow-[0_0_30px_rgba(5,150,105,0.25)]"
            >
              Request a benchmark
              <svg className="w-4 h-4" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.5}>
                <path d="M5 12h14M12 5l7 7-7 7" />
              </svg>
            </a>
          </div>
        </div>
      </section>

      {/* Footer */}
      <footer className="max-w-6xl mx-auto px-6 py-8 border-t border-border">
        <div className="flex items-center justify-between text-[0.72rem] text-fg-muted">
          <span>bench.lab - live exam suite for AI infrastructure agents</span>
          <div className="flex items-center gap-4">
            <Link to="/bench" className="hover:text-accent transition-colors">Bench</Link>
            <Link to={BENCH_SCENARIOS_PATH} className="hover:text-accent transition-colors">Lab</Link>
            <Link to={BENCH_LEADERBOARD_PATH} className="hover:text-accent transition-colors">Exams</Link>
            <a href="https://github.com/vitas/evidra-bench" target="_blank" rel="noopener noreferrer" className="hover:text-accent transition-colors">
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
