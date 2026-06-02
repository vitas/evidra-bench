import { Link } from "react-router";
import { useTheme } from "../hooks/useTheme";
import { SCENARIOS } from "../data/catalog";
import { EXAM_PACKS, countExamPackMatches } from "../lib/examPacks.mts";
import { LANDING_PUBLIC_REPORTS } from "../lib/publicReports.mts";
import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  BENCH_ARTICLE_PASS_FAIL_PATH,
  BENCH_LEADERBOARD_PATH,
  BENCH_ONLINE_PATH,
  BENCH_SAMPLE_REPORT_PATH,
  BENCH_SCENARIOS_PATH,
  LANDING_ARTICLES_ANCHOR,
  benchLeaderboardPagePath,
  benchScenariosPagePath,
} from "../lib/routes.mts";

const EXAM_PACK_COUNTS = countExamPackMatches(SCENARIOS);
const BENCH_PRIVATE_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Private%20Agent%20Benchmark%20Request";
const BENCH_SPONSOR_REQUEST_MAILTO =
  "mailto:bench@evidra.cc?subject=Sponsored%20Public%20Benchmark%20Run";

const PROOF_CHIPS = [
  "Live scenarios",
  "Failure-mode breakdowns",
  "Unsafe-pass autopsy",
  "MCP/server comparisons",
  "Artifacts and transcripts",
];

const FAILURE_MODES = [
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

const PRODUCT_FEATURES = [
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

const USE_CASES = [
  {
    title: "AI SRE teams",
    desc: "Prove that an agent diagnoses, patches, and verifies production-like incidents before it touches real clusters.",
  },
  {
    title: "MCP builders",
    desc: "Show which server capabilities help agents act safely, and where tool access creates unsafe shortcuts.",
  },
  {
    title: "Platform buyers",
    desc: "Compare vendors with a public methodology, per-failure-mode breakdowns, and reproducible evidence.",
  },
];

const ARTICLE_CARDS = [
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
    to: BENCH_ARTICLE_PASS_FAIL_PATH,
  },
];

const BENCH_ENGAGEMENTS = [
  "Private agent and MCP evaluation reports",
  "Sponsored public benchmark runs",
  "Custom live scenario packs",
  "Monthly release regression reports",
];

function ArrowIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.4}>
      <path d="M5 12h14M12 5l7 7-7 7" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.4}>
      <path d="m5 12 4 4L19 6" />
    </svg>
  );
}

function SectionHeader({
  eyebrow,
  title,
  body,
}: {
  eyebrow: string;
  title: string;
  body: string;
}) {
  return (
    <div className="mb-8 flex flex-col gap-3 md:flex-row md:items-end md:justify-between">
      <div className="max-w-2xl">
        <span className="mb-3 inline-flex rounded border border-accent/25 bg-accent/10 px-2.5 py-1 text-[0.66rem] font-semibold uppercase tracking-wider text-accent">
          {eyebrow}
        </span>
        <h2 className="text-[1.65rem] font-extrabold leading-tight text-fg md:text-[2rem]">{title}</h2>
      </div>
      <p className="max-w-xl text-[0.88rem] leading-relaxed text-fg-muted">{body}</p>
    </div>
  );
}

export function Landing() {
  const { theme, toggle } = useTheme();
  const stats = [
    { value: String(SCENARIOS.length), label: "Active scenarios" },
    { value: String(LANDING_PUBLIC_REPORTS.length), label: "Public reports" },
    { value: String(EXAM_PACKS.length), label: "Exam suites" },
    { value: "4", label: "Exam levels" },
  ];

  return (
    <div className="min-h-screen overflow-hidden bg-bg text-fg">
      <div
        className="fixed inset-0 opacity-[0.025]"
        style={{
          backgroundImage:
            "linear-gradient(var(--color-accent) 1px, transparent 1px), linear-gradient(90deg, var(--color-accent) 1px, transparent 1px)",
          backgroundSize: "56px 56px",
        }}
      />

      <div className="relative mx-auto flex max-w-6xl items-center justify-between gap-4 px-6 pt-4">
        <Link to="/" className="text-[0.82rem] font-extrabold tracking-tight text-fg hover:text-accent">
          Evidra Bench
        </Link>
        <div className="flex items-center gap-2">
          <Link
            to={BENCH_ONLINE_PATH}
            className="hidden rounded-md bg-accent px-3 py-2 text-[0.76rem] font-bold text-white transition-colors hover:bg-accent-bright hover:text-white sm:inline-flex"
          >
            Online Bench
          </Link>
          <a
            href="#public-reports"
            className="hidden rounded-md px-3 py-2 text-[0.76rem] font-semibold text-fg-muted hover:text-accent sm:inline-flex"
          >
            Reports
          </a>
          <a
            href={LANDING_ARTICLES_ANCHOR}
            className="hidden rounded-md px-3 py-2 text-[0.76rem] font-semibold text-fg-muted hover:text-accent sm:inline-flex"
          >
            Articles
          </a>
          <a
            href="https://github.com/vitas/evidra-bench"
            target="_blank"
            rel="noopener noreferrer"
            className="flex h-8 w-8 items-center justify-center rounded-md border border-border text-fg-muted transition-all hover:border-accent hover:text-accent"
            aria-label="GitHub"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
          </a>
          <button
            onClick={toggle}
            className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border border-border text-fg-muted transition-all hover:border-accent hover:text-accent"
            style={{ background: "none", fontSize: "0.9rem" }}
            aria-label="Toggle theme"
          >
            {theme === "dark" ? "\u2600" : "\u263E"}
          </button>
        </div>
      </div>

      <section className="relative mx-auto max-w-6xl px-6 pb-8 pt-8 md:pb-14 md:pt-14 lg:pb-16 lg:pt-16">
        <div className="grid gap-10 lg:grid-cols-[1.08fr_0.92fr] lg:items-center">
          <div>
            <div className="mb-5 flex flex-wrap gap-2 md:mb-7">
              {PROOF_CHIPS.map((chip) => (
                <span
                  key={chip}
                  className="rounded-md border border-border bg-bg-elevated/70 px-3 py-1.5 text-[0.72rem] font-semibold text-fg-body"
                >
                  {chip}
                </span>
              ))}
            </div>

            <h1 className="max-w-4xl text-[2.15rem] font-extrabold leading-[1.08] tracking-tight text-fg md:text-[3.45rem] lg:text-[3.75rem]">
              AI SRE benchmarks that show what agents actually do
            </h1>

            <p className="mt-6 max-w-2xl text-[1rem] leading-relaxed text-fg-muted md:text-[1.08rem]">
              Evidra Bench runs live infrastructure scenarios for AI agents, MCP servers,
              and AI SRE tools, then shows pass/fail, unsafe passes, failure modes,
              artifacts, and reproducible evidence.
            </p>

            <div className="mt-8 flex flex-wrap items-center gap-3">
              <Link
                to={BENCH_ONLINE_PATH}
                className="inline-flex items-center gap-2 rounded-lg bg-accent px-4 py-2.5 text-[0.86rem] font-semibold text-white transition-all hover:bg-accent-bright hover:text-white hover:shadow-[0_0_28px_rgba(14,165,233,0.24)] md:px-5 md:py-3 md:text-[0.88rem]"
              >
                Open online bench
                <ArrowIcon />
              </Link>
              <a
                href="#public-reports"
                className="inline-flex items-center gap-2 rounded-lg border border-accent/35 bg-accent/10 px-4 py-2.5 text-[0.86rem] font-semibold text-accent transition-all hover:border-accent/60 hover:text-accent-bright md:px-5 md:py-3 md:text-[0.88rem]"
              >
                View public reports
              </a>
              <Link
                to={BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH}
                className="inline-flex items-center gap-2 rounded-lg border border-border px-4 py-2.5 text-[0.86rem] font-semibold text-fg-body transition-all hover:border-accent/50 hover:text-fg md:px-5 md:py-3 md:text-[0.88rem]"
              >
                Read methodology
              </Link>
              <a
                href={BENCH_PRIVATE_REQUEST_MAILTO}
                className="inline-flex items-center gap-2 rounded-lg border border-border px-4 py-2.5 text-[0.86rem] font-semibold text-fg-body transition-all hover:border-accent/50 hover:text-fg md:px-5 md:py-3 md:text-[0.88rem]"
              >
                Request benchmark
              </a>
            </div>
          </div>

          <div className="glass-card hidden p-5 md:p-6 lg:block">
            <div className="mb-5 flex items-start justify-between gap-4">
              <div>
                <p className="text-[0.7rem] font-semibold uppercase tracking-wider text-accent">
                  Product evidence
                </p>
                <h2 className="mt-2 text-[1.25rem] font-extrabold leading-tight text-fg">
                  Reports designed for platform teams and procurement
                </h2>
              </div>
              <span className="rounded-md bg-accent-tint px-2.5 py-1 text-[0.66rem] font-bold uppercase tracking-wider text-accent">
                Open source
              </span>
            </div>
            <div className="grid grid-cols-2 gap-3">
              {stats.map((stat) => (
                <div key={stat.label} className="rounded-lg border border-border bg-bg-alt/65 p-4">
                  <div className="text-[1.55rem] font-extrabold leading-none text-accent">{stat.value}</div>
                  <div className="mt-2 text-[0.68rem] font-semibold uppercase tracking-wider text-fg-muted">
                    {stat.label}
                  </div>
                </div>
              ))}
            </div>
            <div className="mt-5 space-y-3">
              {[
                "Per-failure-mode breakdowns instead of a single blended score.",
                "Unsafe-pass autopsy for agents that recover final state the wrong way.",
                "Scenario artifacts that make report claims inspectable and reproducible.",
              ].map((item) => (
                <div key={item} className="flex items-start gap-3 text-[0.84rem] leading-relaxed text-fg-body">
                  <span className="mt-0.5 text-accent">
                    <CheckIcon />
                  </span>
                  <span>{item}</span>
                </div>
              ))}
            </div>
          </div>
        </div>
      </section>

      <section id="public-reports" className="relative mx-auto max-w-6xl px-6 py-10 md:py-14">
        <SectionHeader
          eyebrow="Public proof"
          title="Reports and articles are the product surface"
          body="Buyers should not have to trust a vendor claim. Start with public runs, then inspect the methodology and the evidence behind each failure mode."
        />

        <div className="grid gap-4 md:grid-cols-2">
          {LANDING_PUBLIC_REPORTS.map((report) => (
            <Link key={report.id} to={report.to} className="glass-card group p-5 transition-all">
              <div className="mb-4 flex flex-wrap items-center gap-2">
                <span className="rounded border border-border bg-bg-alt px-2 py-0.5 text-[0.62rem] font-semibold uppercase tracking-wider text-fg-muted">
                  {report.label}
                </span>
                <span className="rounded bg-accent-tint px-2 py-0.5 text-[0.62rem] font-semibold text-accent">
                  {report.finding}
                </span>
              </div>
              <div className="mb-3 flex items-start justify-between gap-4">
                <div>
                  <h3 className="text-[1.02rem] font-bold text-fg transition-colors group-hover:text-accent">
                    {report.title}
                  </h3>
                  <p className="mt-1 font-mono text-[0.72rem] text-fg-muted">
                    {report.model} - {report.scope}
                  </p>
                </div>
                <ArrowIcon className="mt-1 h-4 w-4 shrink-0 text-accent transition-transform group-hover:translate-x-0.5" />
              </div>
              <p className="text-[0.82rem] leading-relaxed text-fg-muted">{report.result}</p>
            </Link>
          ))}
        </div>

        <div id="articles" className="mt-5 scroll-mt-24">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-[0.9rem] font-bold uppercase tracking-wider text-fg-muted">
              Articles
            </h3>
            <Link
              to={BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH}
              className="text-[0.74rem] font-semibold text-accent hover:text-accent-bright"
            >
              Start with methodology
            </Link>
          </div>
        </div>

        <div className="grid gap-4 md:grid-cols-2">
          {ARTICLE_CARDS.map((article) => (
            <Link
              key={article.to}
              to={article.to}
              className="group rounded-lg border border-border bg-bg-alt/70 p-5 transition-all hover:border-accent/40"
            >
              <div className="mb-3 flex items-center justify-between gap-3">
                <span className="rounded border border-accent/25 bg-accent/10 px-2 py-0.5 text-[0.62rem] font-semibold uppercase tracking-wider text-accent">
                  {article.label}
                </span>
                <ArrowIcon className="h-4 w-4 shrink-0 text-accent transition-transform group-hover:translate-x-0.5" />
              </div>
              <h3 className="text-[1rem] font-bold leading-snug text-fg transition-colors group-hover:text-accent">
                {article.title}
              </h3>
              <p className="mt-2 text-[0.82rem] leading-relaxed text-fg-muted">{article.desc}</p>
            </Link>
          ))}
        </div>
      </section>

      <section className="relative mx-auto max-w-6xl px-6 py-14">
        <SectionHeader
          eyebrow="Failure modes"
          title="What Evidra Bench catches"
          body="The useful signal is not only whether the workload recovered. The report should show which operational failure mode the agent avoided or triggered."
        />

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-5">
          {FAILURE_MODES.map((mode) => (
            <div key={mode.title} className="glass-card p-4">
              <span className="rounded bg-warning-tint px-2 py-0.5 text-[0.62rem] font-bold uppercase tracking-wider text-warning">
                {mode.signal}
              </span>
              <h3 className="mt-4 text-[0.95rem] font-bold text-fg">{mode.title}</h3>
              <p className="mt-2 text-[0.78rem] leading-relaxed text-fg-muted">{mode.desc}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="relative mx-auto max-w-6xl px-6 py-14">
        <SectionHeader
          eyebrow="Feature set"
          title="Built for live AI SRE and MCP evaluation"
          body="Evidra Bench combines scenario authoring, deterministic verification, public reporting, and regression views so benchmark results are inspectable."
        />

        <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-3">
          {PRODUCT_FEATURES.map((feature) => (
            <div key={feature.title} className="glass-card p-5">
              <div className="mb-3 flex h-8 w-8 items-center justify-center rounded-lg bg-accent-tint text-accent">
                <CheckIcon />
              </div>
              <h3 className="text-[1rem] font-bold text-fg">{feature.title}</h3>
              <p className="mt-2 text-[0.82rem] leading-relaxed text-fg-muted">{feature.desc}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="relative mx-auto max-w-6xl px-6 py-14">
        <SectionHeader
          eyebrow="Exam catalog"
          title="Scenario packs for repeatable evaluation"
          body="Public exam suites expose shareable slices of the live catalog, with matching scenario lists and leaderboard views."
        />

        <div className="grid gap-4 sm:grid-cols-2 lg:grid-cols-3">
          {EXAM_PACKS.map((pack) => (
            <div key={pack.id} className="glass-card p-5">
              <div className="mb-3 flex items-start justify-between gap-3">
                <div>
                  <h3 className="text-[0.92rem] font-bold text-fg">{pack.title}</h3>
                  <p className="mt-1 text-[0.76rem] leading-relaxed text-fg-muted">{pack.summary}</p>
                </div>
                <span className="font-mono text-[0.82rem] font-bold text-accent">
                  {EXAM_PACK_COUNTS[pack.id] ?? 0}
                </span>
              </div>
              <p className="mb-4 text-[0.7rem] leading-relaxed text-fg-muted">{pack.proof}</p>
              <div className="flex items-center gap-2">
                <Link
                  to={benchScenariosPagePath({ exam: pack.id })}
                  className="rounded-md bg-accent-tint px-3 py-1.5 text-[0.72rem] font-semibold text-accent transition-colors hover:bg-accent-subtle"
                >
                  Catalog
                </Link>
                <Link
                  to={benchLeaderboardPagePath({ exam: pack.id })}
                  className="rounded-md border border-border px-3 py-1.5 text-[0.72rem] font-semibold text-fg-muted transition-colors hover:border-accent/50 hover:text-fg"
                >
                  Leaderboard
                </Link>
              </div>
            </div>
          ))}
        </div>
      </section>

      <section className="relative mx-auto max-w-6xl px-6 py-14">
        <SectionHeader
          eyebrow="Users"
          title="Who this is for"
          body="The same benchmark surface works for builders who need regression signal and buyers who need comparable external evidence."
        />

        <div className="grid gap-4 md:grid-cols-3">
          {USE_CASES.map((useCase) => (
            <div key={useCase.title} className="rounded-lg border border-border bg-bg-elevated/70 p-5">
              <h3 className="text-[1rem] font-bold text-fg">{useCase.title}</h3>
              <p className="mt-2 text-[0.84rem] leading-relaxed text-fg-muted">{useCase.desc}</p>
            </div>
          ))}
        </div>
      </section>

      <section className="relative mx-auto max-w-6xl px-6 py-16">
        <div className="grid items-start gap-8 border-y border-border py-10 lg:grid-cols-[1.05fr_0.95fr]">
          <div>
            <span className="mb-5 inline-flex rounded border border-accent/25 bg-accent/10 px-3 py-1 text-[0.68rem] font-semibold uppercase tracking-wider text-accent">
              Independent live evaluation
            </span>
            <h2 className="max-w-2xl text-[1.9rem] font-extrabold leading-tight text-fg md:text-[2.35rem]">
              Commission a benchmark that buyers can inspect
            </h2>
            <p className="mt-4 max-w-2xl text-[0.95rem] leading-relaxed text-fg-muted">
              Run a private evaluation for internal decision-making, or sponsor a
              clearly labeled public report with the same scoring and artifact standards.
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <a
                href={BENCH_PRIVATE_REQUEST_MAILTO}
                className="inline-flex items-center gap-2 rounded-lg bg-accent px-5 py-2.5 text-[0.84rem] font-semibold text-white transition-all hover:bg-accent-bright hover:text-white"
              >
                Request a private benchmark
                <ArrowIcon />
              </a>
              <a
                href={BENCH_SPONSOR_REQUEST_MAILTO}
                className="inline-flex items-center gap-2 rounded-lg border border-border px-5 py-2.5 text-[0.84rem] font-semibold text-fg-body transition-all hover:border-accent/50 hover:text-fg"
              >
                Sponsor a public run
              </a>
              <Link
                to={BENCH_SAMPLE_REPORT_PATH}
                className="inline-flex items-center gap-2 rounded-lg border border-border px-5 py-2.5 text-[0.84rem] font-semibold text-fg-body transition-all hover:border-accent/50 hover:text-fg"
              >
                View sample report
              </Link>
            </div>
          </div>

          <div className="rounded-lg border border-border bg-bg-alt/70 p-5">
            <h3 className="mb-4 text-[0.82rem] font-semibold uppercase tracking-wider text-fg-muted">
              Available engagements
            </h3>
            <ul className="space-y-3">
              {BENCH_ENGAGEMENTS.map((item) => (
                <li key={item} className="flex items-start gap-3 text-[0.85rem] text-fg-body">
                  <span className="mt-0.5 text-accent">
                    <CheckIcon />
                  </span>
                  <span>{item}</span>
                </li>
              ))}
            </ul>
            <p className="mt-5 text-[0.78rem] leading-relaxed text-fg-muted">
              Private reports stay confidential. Sponsored reports are labeled, and
              sponsors do not control scoring or findings.
            </p>
          </div>
        </div>
      </section>

      <footer className="relative mx-auto max-w-6xl border-t border-border px-6 py-8">
        <div className="flex flex-col gap-4 text-[0.72rem] text-fg-muted md:flex-row md:items-center md:justify-between">
          <span>Evidra Bench - live regression testing for AI infrastructure agents</span>
          <div className="flex flex-wrap items-center gap-4">
            <Link to={BENCH_ONLINE_PATH} className="transition-colors hover:text-accent">Online Bench</Link>
            <a href="#public-reports" className="transition-colors hover:text-accent">Reports</a>
            <a href={LANDING_ARTICLES_ANCHOR} className="transition-colors hover:text-accent">Articles</a>
            <Link to={BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH} className="transition-colors hover:text-accent">Methodology</Link>
            <Link to={BENCH_ARTICLE_PASS_FAIL_PATH} className="transition-colors hover:text-accent">Case study</Link>
            <Link to={BENCH_SCENARIOS_PATH} className="transition-colors hover:text-accent">Scenarios</Link>
            <Link to={BENCH_LEADERBOARD_PATH} className="transition-colors hover:text-accent">Leaderboards</Link>
            <a href="https://github.com/vitas/evidra-bench" target="_blank" rel="noopener noreferrer" className="transition-colors hover:text-accent">
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
