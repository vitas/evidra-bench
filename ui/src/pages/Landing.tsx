import { Link } from "react-router";
import { useTheme } from "../hooks/useTheme";
import {
  BENCH_PRIVATE_REQUEST_MAILTO,
  BENCH_SPONSOR_REQUEST_MAILTO,
  ENTERPRISE_AUDIENCES,
  FEATURED_REPORT_SPOTLIGHT,
  HERO_CONTENT,
  LANDING_OFFERS,
  type LandingCta,
} from "../lib/landingContent.mts";
import { LANDING_PUBLIC_REPORTS } from "../lib/publicReports.mts";
import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  BENCH_LEADERBOARD_PATH,
  BENCH_ONLINE_PATH,
  BENCH_SCENARIOS_PATH,
} from "../lib/routes.mts";

function ArrowIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.2}>
      <path d="M5 12h14M12 5l7 7-7 7" />
    </svg>
  );
}

function CheckIcon() {
  return (
    <svg className="h-4 w-4 shrink-0" viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2.3}>
      <path d="m5 12 4 4L19 6" />
    </svg>
  );
}

const CTA_CLASS_NAMES: Record<LandingCta["kind"], string> = {
  primary: "bg-accent text-white hover:bg-accent-bright hover:text-white",
  secondary: "border border-accent/35 bg-accent/10 text-accent hover:border-accent/60 hover:text-accent-bright",
  tertiary: "border border-border text-fg-body hover:border-accent/50 hover:text-fg",
  quiet: "border border-border text-fg-muted hover:border-accent/50 hover:text-fg",
};

function CtaLink({ cta }: { cta: LandingCta }) {
  const className = `inline-flex items-center justify-center gap-2 rounded-md px-4 py-2.5 text-[0.86rem] font-semibold transition-colors ${CTA_CLASS_NAMES[cta.kind]}`;
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

function SectionLabel({ children }: { children: React.ReactNode }) {
  return (
    <span className="mb-4 inline-flex text-[0.68rem] font-semibold uppercase tracking-wider text-accent">
      {children}
    </span>
  );
}

function ReportSpotlight({ compact = false }: { compact?: boolean }) {
  return (
    <aside className="rounded-xl border border-border bg-bg-elevated p-5 md:p-6">
      <div className="flex flex-wrap items-center gap-2">
        <span className="rounded border border-border bg-bg-alt px-2 py-0.5 text-[0.62rem] font-semibold uppercase tracking-wider text-fg-muted">
          {FEATURED_REPORT_SPOTLIGHT.label}
        </span>
        <span className="rounded bg-warning-tint px-2 py-0.5 text-[0.62rem] font-semibold text-warning">
          40+ scenario slices
        </span>
      </div>

      <h2 className={`${compact ? "mt-4 text-[1.2rem]" : "mt-5 text-[1.35rem]"} font-extrabold leading-tight text-fg`}>
        {FEATURED_REPORT_SPOTLIGHT.title}
      </h2>
      <p className="mt-3 text-[0.9rem] leading-relaxed text-fg-muted">
        {FEATURED_REPORT_SPOTLIGHT.summary}
      </p>

      <dl className="mt-5 divide-y divide-border rounded-lg border border-border bg-bg-alt/50">
        {FEATURED_REPORT_SPOTLIGHT.metrics.map(([label, value]) => (
          <div key={label} className="flex items-center justify-between gap-4 px-4 py-3">
            <dt className="text-[0.78rem] font-medium text-fg-muted">{label}</dt>
            <dd className="font-mono text-[0.9rem] font-bold text-fg">{value}</dd>
          </div>
        ))}
      </dl>

      <div className="mt-5 space-y-3">
        {FEATURED_REPORT_SPOTLIGHT.strengths.map((item) => (
          <div key={item} className="flex gap-3 text-[0.84rem] leading-relaxed text-fg-body">
            <span className="mt-0.5 text-accent">
              <CheckIcon />
            </span>
            <span>{item}</span>
          </div>
        ))}
      </div>

      <Link
        to={FEATURED_REPORT_SPOTLIGHT.to}
        className="mt-6 inline-flex items-center gap-2 rounded-md bg-accent px-4 py-2.5 text-[0.84rem] font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white"
      >
        Open public report
        <ArrowIcon />
      </Link>
    </aside>
  );
}

export function Landing() {
  const { theme, toggle } = useTheme();
  const primaryReport = LANDING_PUBLIC_REPORTS[0];

  return (
    <div className="min-h-screen bg-bg text-fg">
      <header className="mx-auto flex max-w-6xl items-center justify-between gap-4 px-6 py-4">
        <Link to="/" className="text-[0.84rem] font-extrabold tracking-tight text-fg hover:text-accent">
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
            to={FEATURED_REPORT_SPOTLIGHT.to}
            className="hidden rounded-md px-3 py-2 text-[0.76rem] font-semibold text-fg-muted hover:text-accent sm:inline-flex"
          >
            Real report
          </Link>
          <a
            href="https://github.com/vitas/evidra-bench"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden h-8 w-8 items-center justify-center rounded-md border border-border text-fg-muted transition-colors hover:border-accent hover:text-accent sm:flex"
            aria-label="GitHub"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
          </a>
          <button
            onClick={toggle}
            className="flex h-8 w-8 cursor-pointer items-center justify-center rounded-md border border-border text-fg-muted transition-colors hover:border-accent hover:text-accent"
            style={{ background: "none", fontSize: "0.9rem" }}
            aria-label="Toggle theme"
          >
            {theme === "dark" ? "\u2600" : "\u263E"}
          </button>
        </div>
      </header>

      <main>
        <section className="mx-auto grid max-w-6xl gap-10 px-6 pb-12 pt-8 lg:grid-cols-[1.05fr_0.95fr] lg:items-start lg:pb-16 lg:pt-14">
          <div>
            <div className="mb-6 flex flex-wrap gap-2">
              {HERO_CONTENT.proofChips.map((chip) => (
                <span
                  key={chip}
                  className="rounded border border-border bg-bg-elevated px-2.5 py-1 text-[0.68rem] font-semibold text-fg-muted"
                >
                  {chip}
                </span>
              ))}
            </div>

            <h1 className="max-w-4xl text-[2.15rem] font-extrabold leading-[1.08] tracking-tight text-fg md:text-[3.25rem] lg:text-[3.55rem]">
              {HERO_CONTENT.title}
            </h1>

            <p className="mt-6 max-w-2xl text-[1rem] leading-relaxed text-fg-muted md:text-[1.06rem]">
              {HERO_CONTENT.body}
            </p>

            <div className="mt-8 flex flex-wrap items-center gap-3">
              {HERO_CONTENT.ctas.map((cta) => (
                <CtaLink key={cta.label} cta={cta} />
              ))}
            </div>

            <p className="mt-6 max-w-xl text-[0.82rem] leading-relaxed text-fg-muted">
              Built for teams selling, buying, or implementing AI agents in infrastructure-heavy enterprise environments.
            </p>
          </div>

          <ReportSpotlight />
        </section>

        <section id="private-evaluation" className="mx-auto max-w-6xl px-6 py-10">
          <div className="grid gap-8 border-y border-border py-10 lg:grid-cols-[0.86fr_1.14fr]">
            <div>
              <SectionLabel>Private evaluation</SectionLabel>
              <h2 className="max-w-2xl text-[1.75rem] font-extrabold leading-tight text-fg md:text-[2.15rem]">
                A buyer-ready report, not another benchmark wall.
              </h2>
              <p className="mt-4 max-w-xl text-[0.95rem] leading-relaxed text-fg-muted">
                The useful output is a short decision document: what was tested, what passed safely, what passed unsafely, and which rollout risks remain.
              </p>
              <div className="mt-7 flex flex-wrap gap-3">
                <a
                  href={BENCH_PRIVATE_REQUEST_MAILTO}
                  className="inline-flex items-center gap-2 rounded-md bg-accent px-5 py-2.5 text-[0.84rem] font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white"
                >
                  Book private evaluation
                  <ArrowIcon />
                </a>
                <a
                  href={BENCH_SPONSOR_REQUEST_MAILTO}
                  className="inline-flex items-center gap-2 rounded-md border border-border px-5 py-2.5 text-[0.84rem] font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
                >
                  Sponsor public proof
                </a>
              </div>
            </div>

            <div className="divide-y divide-border rounded-xl border border-border bg-bg-elevated">
              {LANDING_OFFERS.map((offer) => (
                <div key={offer.title} className="grid gap-2 px-5 py-4 md:grid-cols-[0.38fr_0.62fr] md:gap-5">
                  <h3 className="text-[0.95rem] font-bold text-fg">{offer.title}</h3>
                  <p className="text-[0.86rem] leading-relaxed text-fg-muted">{offer.desc}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section id="public-reports" className="mx-auto max-w-6xl px-6 py-10 md:py-14">
          <div className="grid gap-8 lg:grid-cols-[0.82fr_1.18fr]">
            <div>
              <SectionLabel>Why the report matters</SectionLabel>
              <h2 className="max-w-2xl text-[1.65rem] font-extrabold leading-tight text-fg md:text-[2rem]">
                The signal is path safety, not the number of green cells.
              </h2>
              <p className="mt-4 max-w-xl text-[0.92rem] leading-relaxed text-fg-muted">
                Large scenario coverage proves breadth. The public MCP report shows the deeper decision signal: final-state passes split into safe and unsafe paths with linked evidence behind the result.
              </p>
              <div className="mt-6 flex flex-wrap gap-3">
                <Link
                  to={FEATURED_REPORT_SPOTLIGHT.to}
                  className="inline-flex items-center gap-2 rounded-md bg-accent px-4 py-2.5 text-[0.84rem] font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white"
                >
                  Open public report
                  <ArrowIcon />
                </Link>
                {primaryReport ? (
                  <Link
                    to={primaryReport.to}
                    className="inline-flex items-center gap-2 rounded-md border border-border px-4 py-2.5 text-[0.84rem] font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
                  >
                    Open full report
                  </Link>
                ) : null}
              </div>
            </div>

            <div className="rounded-xl border border-border bg-bg-elevated p-5 md:p-6">
              <div className="grid gap-5 md:grid-cols-3">
                <div>
                  <h3 className="text-[0.9rem] font-bold text-fg">Same task slice</h3>
                  <p className="mt-2 text-[0.82rem] leading-relaxed text-fg-muted">
                    Candidates run against the same scenarios, model, and report filter.
                  </p>
                </div>
                <div>
                  <h3 className="text-[0.9rem] font-bold text-fg">Unsafe pass visible</h3>
                  <p className="mt-2 text-[0.82rem] leading-relaxed text-fg-muted">
                    A green final state is separated from risky mutations and shortcuts.
                  </p>
                </div>
                <div>
                  <h3 className="text-[0.9rem] font-bold text-fg">Evidence attached</h3>
                  <p className="mt-2 text-[0.82rem] leading-relaxed text-fg-muted">
                    Run pages preserve transcripts, tool calls, timelines, and autopsies.
                  </p>
                </div>
              </div>
            </div>
          </div>
        </section>

        <section className="mx-auto max-w-6xl px-6 py-10 md:py-14">
          <div className="grid gap-8 border-t border-border pt-10 lg:grid-cols-[0.82fr_1.18fr]">
            <div>
              <SectionLabel>Who it is for</SectionLabel>
              <h2 className="max-w-xl text-[1.65rem] font-extrabold leading-tight text-fg md:text-[2rem]">
                Teams that need evidence before an agent touches production.
              </h2>
            </div>

            <div className="divide-y divide-border rounded-xl border border-border bg-bg-elevated">
              {ENTERPRISE_AUDIENCES.map((audience) => (
                <div key={audience.title} className="grid gap-2 px-5 py-4 md:grid-cols-[0.34fr_0.66fr] md:gap-5">
                  <h3 className="text-[0.95rem] font-bold text-fg">{audience.title}</h3>
                  <p className="text-[0.86rem] leading-relaxed text-fg-muted">{audience.desc}</p>
                </div>
              ))}
            </div>
          </div>
        </section>

        <section className="mx-auto max-w-6xl px-6 py-12">
          <div className="rounded-xl border border-border bg-bg-elevated p-6 md:flex md:items-center md:justify-between md:gap-8 md:p-8">
            <div>
              <h2 className="text-[1.45rem] font-extrabold leading-tight text-fg">
                Need to defend an enterprise AI agent rollout?
              </h2>
              <p className="mt-3 max-w-2xl text-[0.92rem] leading-relaxed text-fg-muted">
                Send the agent, toolchain, and target scenario slice. Evidra Bench returns a private report built for buyer, SRE, and risk review.
              </p>
            </div>
            <a
              href={BENCH_PRIVATE_REQUEST_MAILTO}
              className="mt-6 inline-flex items-center gap-2 rounded-md bg-accent px-5 py-2.5 text-[0.84rem] font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white md:mt-0"
            >
              Book private evaluation
              <ArrowIcon />
            </a>
          </div>
        </section>
      </main>

      <footer className="mx-auto max-w-6xl border-t border-border px-6 py-8">
        <div className="flex flex-col gap-4 text-[0.72rem] text-fg-muted md:flex-row md:items-center md:justify-between">
          <span>Evidra Bench - independent evaluation for AI infrastructure agents</span>
          <div className="flex flex-wrap items-center gap-4">
            <Link to={FEATURED_REPORT_SPOTLIGHT.to} className="transition-colors hover:text-accent">Real report</Link>
            <Link to={BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH} className="transition-colors hover:text-accent">Methodology</Link>
            <Link to={BENCH_ONLINE_PATH} className="transition-colors hover:text-accent">Online Bench</Link>
            <Link to={BENCH_SCENARIOS_PATH} className="transition-colors hover:text-accent">Scenarios</Link>
            <Link to={BENCH_LEADERBOARD_PATH} className="transition-colors hover:text-accent">Leaderboards</Link>
            <a
              href="https://github.com/vitas/evidra-bench"
              target="_blank"
              rel="noopener noreferrer"
              className="transition-colors hover:text-accent"
            >
              GitHub
            </a>
          </div>
        </div>
      </footer>
    </div>
  );
}
