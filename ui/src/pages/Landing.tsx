import { Link } from "react-router";
import { useTheme } from "../hooks/useTheme";
import { SCENARIOS } from "../data/catalog";
import { EXAM_PACKS, countExamPackMatches } from "../lib/examPacks.mts";
import {
  ARTICLE_CARDS,
  BENCH_ENGAGEMENTS,
  BENCH_PRIVATE_REQUEST_MAILTO,
  BENCH_SPONSOR_REQUEST_MAILTO,
  ENTERPRISE_AUDIENCES,
  FAILURE_MODES,
  HERO_CONTENT,
  HERO_EVIDENCE_BULLETS,
  LANDING_OFFERS,
  PRODUCT_FEATURES,
  SEO_GUIDES,
  type LandingCta,
} from "../lib/landingContent.mts";
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

const CTA_CLASS_NAMES: Record<LandingCta["kind"], string> = {
  primary:
    "bg-accent text-white hover:bg-accent-bright hover:text-white hover:shadow-[0_0_28px_rgba(14,165,233,0.24)]",
  secondary:
    "border border-accent/35 bg-accent/10 text-accent hover:border-accent/60 hover:text-accent-bright",
  tertiary:
    "border border-border text-fg-body hover:border-accent/50 hover:text-fg",
  quiet:
    "border border-border text-fg-muted hover:border-accent/50 hover:text-fg",
};

function CtaLink({ cta }: { cta: LandingCta }) {
  const className = `inline-flex items-center gap-2 rounded-lg px-4 py-2.5 text-[0.86rem] font-semibold transition-all md:px-5 md:py-3 md:text-[0.88rem] ${CTA_CLASS_NAMES[cta.kind]}`;
  const content = (
    <>
      {cta.label}
      {cta.kind === "primary" ? <ArrowIcon /> : null}
    </>
  );

  if (cta.href.startsWith("/")) {
    return (
      <Link to={cta.href} className={className}>
        {content}
      </Link>
    );
  }

  return (
    <a href={cta.href} className={className}>
      {content}
    </a>
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
          <a
            href={BENCH_PRIVATE_REQUEST_MAILTO}
            className="inline-flex rounded-md bg-accent px-3 py-2 text-[0.76rem] font-bold text-white transition-colors hover:bg-accent-bright hover:text-white"
          >
            Book evaluation
          </a>
          <Link
            to={BENCH_ONLINE_PATH}
            className="hidden rounded-md px-3 py-2 text-[0.76rem] font-semibold text-fg-muted hover:text-accent md:inline-flex"
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
              {HERO_CONTENT.proofChips.map((chip) => (
                <span
                  key={chip}
                  className="rounded-md border border-border bg-bg-elevated/70 px-3 py-1.5 text-[0.72rem] font-semibold text-fg-body"
                >
                  {chip}
                </span>
              ))}
            </div>

            <h1 className="max-w-4xl text-[2.15rem] font-extrabold leading-[1.08] tracking-tight text-fg md:text-[3.45rem] lg:text-[3.75rem]">
              {HERO_CONTENT.title}
            </h1>

            <p className="mt-6 max-w-2xl text-[1rem] leading-relaxed text-fg-muted md:text-[1.08rem]">
              {HERO_CONTENT.body}
            </p>

            <div className="mt-8 flex flex-wrap items-center gap-3">
              {HERO_CONTENT.ctas.map((cta) => (
                <CtaLink key={cta.label} cta={cta} />
              ))}
            </div>
          </div>

          <div className="glass-card hidden p-5 md:p-6 lg:block">
            <div className="mb-5 flex items-start justify-between gap-4">
              <div>
                <p className="text-[0.7rem] font-semibold uppercase tracking-wider text-accent">
                  Deployment evidence
                </p>
                <h2 className="mt-2 text-[1.25rem] font-extrabold leading-tight text-fg">
                  Reports designed for enterprise rollout decisions
                </h2>
              </div>
              <span className="rounded-md bg-accent-tint px-2.5 py-1 text-[0.66rem] font-bold uppercase tracking-wider text-accent">
                Private first
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
              {HERO_EVIDENCE_BULLETS.map((item) => (
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

      <section id="private-evaluation" className="relative mx-auto max-w-6xl px-6 py-10 md:py-14">
        <div className="grid items-start gap-8 border-y border-border py-10 lg:grid-cols-[0.92fr_1.08fr]">
          <div>
            <span className="mb-5 inline-flex rounded border border-accent/25 bg-accent/10 px-3 py-1 text-[0.68rem] font-semibold uppercase tracking-wider text-accent">
              Enterprise deployment evidence
            </span>
            <h2 className="max-w-2xl text-[1.9rem] font-extrabold leading-tight text-fg md:text-[2.35rem]">
              Evaluate agents before enterprise rollout
            </h2>
            <p className="mt-4 max-w-2xl text-[0.95rem] leading-relaxed text-fg-muted">
              Run a confidential evaluation for an agent implementation, vendor
              shortlist, model upgrade, or MCP toolchain. The output is a
              buyer-ready report with scenarios, safety findings, artifacts, and
              rollout risks.
            </p>
            <div className="mt-7 flex flex-wrap gap-3">
              <a
                href={BENCH_PRIVATE_REQUEST_MAILTO}
                className="inline-flex items-center gap-2 rounded-lg bg-accent px-5 py-2.5 text-[0.84rem] font-semibold text-white transition-all hover:bg-accent-bright hover:text-white"
              >
                Book private evaluation
                <ArrowIcon />
              </a>
              <Link
                to={BENCH_SAMPLE_REPORT_PATH}
                className="inline-flex items-center gap-2 rounded-lg border border-accent/35 bg-accent/10 px-5 py-2.5 text-[0.84rem] font-semibold text-accent transition-all hover:border-accent/60 hover:text-accent-bright"
              >
                View sample report
              </Link>
              <a
                href={BENCH_SPONSOR_REQUEST_MAILTO}
                className="inline-flex items-center gap-2 rounded-lg border border-border px-5 py-2.5 text-[0.84rem] font-semibold text-fg-body transition-all hover:border-accent/50 hover:text-fg"
              >
                Sponsor public proof
              </a>
            </div>
          </div>

          <div className="grid gap-4 md:grid-cols-2">
            {LANDING_OFFERS.map((offer) => (
              <div key={offer.title} className="glass-card p-5">
                <div className="mb-3 flex h-8 w-8 items-center justify-center rounded-lg bg-accent-tint text-accent">
                  <CheckIcon />
                </div>
                <h3 className="text-[1rem] font-bold text-fg">{offer.title}</h3>
                <p className="mt-2 text-[0.82rem] leading-relaxed text-fg-muted">{offer.desc}</p>
              </div>
            ))}
          </div>

          <div className="rounded-lg border border-border bg-bg-alt/70 p-5 lg:col-span-2">
            <h3 className="mb-4 text-[0.82rem] font-semibold uppercase tracking-wider text-fg-muted">
              Available engagements
            </h3>
            <div className="grid gap-3 md:grid-cols-2 lg:grid-cols-4">
              {BENCH_ENGAGEMENTS.map((item) => (
                <div key={item} className="flex items-start gap-3 text-[0.85rem] text-fg-body">
                  <span className="mt-0.5 text-accent">
                    <CheckIcon />
                  </span>
                  <span>{item}</span>
                </div>
              ))}
            </div>
            <p className="mt-5 max-w-3xl text-[0.78rem] leading-relaxed text-fg-muted">
              Private reports stay confidential. Sponsored reports are labeled,
              and sponsors do not control scoring or findings.
            </p>
          </div>
        </div>
      </section>

      <section id="public-reports" className="relative mx-auto max-w-6xl px-6 py-10 md:py-14">
        <SectionHeader
          eyebrow="Public proof"
          title="Public reports prove the method"
          body="Buyers should not have to trust a vendor claim. Public runs show the methodology, while private evaluations apply the same scoring and artifact standards to customer decisions."
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

        <div className="mt-8">
          <div className="mb-3 flex items-center justify-between gap-3">
            <h3 className="text-[0.9rem] font-bold uppercase tracking-wider text-fg-muted">
              Buyer guides
            </h3>
            <a
              href="/ai-agent-benchmark-reports/"
              className="text-[0.74rem] font-semibold text-accent hover:text-accent-bright"
            >
              Browse SEO guides
            </a>
          </div>
          <div className="grid gap-4 md:grid-cols-2 lg:grid-cols-4">
            {SEO_GUIDES.map((guide) => (
              <a
                key={guide.href}
                href={guide.href}
                className="group rounded-lg border border-border bg-bg-alt/70 p-5 transition-all hover:border-accent/40"
              >
                <div className="mb-3 flex items-center justify-between gap-3">
                  <span className="rounded border border-accent/25 bg-accent/10 px-2 py-0.5 text-[0.62rem] font-semibold uppercase tracking-wider text-accent">
                    {guide.label}
                  </span>
                  <ArrowIcon className="h-4 w-4 shrink-0 text-accent transition-transform group-hover:translate-x-0.5" />
                </div>
                <h3 className="text-[0.98rem] font-bold leading-snug text-fg transition-colors group-hover:text-accent">
                  {guide.title}
                </h3>
                <p className="mt-2 text-[0.8rem] leading-relaxed text-fg-muted">{guide.desc}</p>
              </a>
            ))}
          </div>
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
          title="Built for private AI SRE and MCP evaluation"
          body="Evidra Bench combines scenario authoring, deterministic verification, public reporting, and regression views so enterprise benchmark claims are inspectable."
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
          eyebrow="Enterprise buyers"
          title="Who buys this evaluation"
          body="The strongest use case is not a generic open-source score. It is independent rollout evidence for teams implementing AI agents in enterprise environments."
        />

        <div className="grid gap-4 md:grid-cols-3">
          {ENTERPRISE_AUDIENCES.map((audience) => (
            <div key={audience.title} className="rounded-lg border border-border bg-bg-elevated/70 p-5">
              <h3 className="text-[1rem] font-bold text-fg">{audience.title}</h3>
              <p className="mt-2 text-[0.84rem] leading-relaxed text-fg-muted">{audience.desc}</p>
            </div>
          ))}
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
            <a href="/ai-agent-benchmark-reports/" className="transition-colors hover:text-accent">Benchmark reports</a>
            <a href="/private-ai-agent-evaluation/" className="transition-colors hover:text-accent">Private evaluation</a>
            <a href="/safe-pass-unsafe-pass-ai-agents/" className="transition-colors hover:text-accent">Safe pass</a>
            <a href="/kubernetes-mcp-server-benchmark/" className="transition-colors hover:text-accent">Kubernetes MCP</a>
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
