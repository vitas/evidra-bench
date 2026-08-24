import { Link } from "react-router";
import { useTheme } from "../hooks/useTheme";
import {
  BENCH_PRIVATE_REQUEST_MAILTO,
  BENCHMARK_DIFFERENCES,
  HERO_CONTENT,
  HERO_PROOF_POINTS,
  PATH_COMPARISON,
  WORKFLOW_STEPS,
  type LandingCta,
} from "../lib/landingContent.mts";
import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  BENCH_LEADERBOARD_PATH,
  BENCH_ONLINE_PATH,
  BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH,
  BENCH_SCENARIOS_PATH,
} from "../lib/routes.mts";

const FOCUS_RING =
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg";

function ArrowIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg
      aria-hidden="true"
      className={className}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2.2}
    >
      <path d="M5 12h14M12 5l7 7-7 7" />
    </svg>
  );
}

function CheckIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg
      aria-hidden="true"
      className={`${className} shrink-0`}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2.3}
    >
      <path d="m5 12 4 4L19 6" />
    </svg>
  );
}

function WarningIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg
      aria-hidden="true"
      className={`${className} shrink-0`}
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth={2.1}
    >
      <path d="M12 3 2.8 20h18.4L12 3Z" />
      <path d="M12 9v5M12 17.5v.5" />
    </svg>
  );
}

function GitHubIcon() {
  return (
    <svg aria-hidden="true" className="h-4 w-4" viewBox="0 0 24 24" fill="currentColor">
      <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
    </svg>
  );
}

const CTA_CLASS_NAMES: Record<LandingCta["kind"], string> = {
  primary: "bg-accent text-white hover:bg-accent-bright hover:text-white",
  secondary:
    "border border-accent/35 bg-accent-tint text-accent hover:border-accent/60 hover:text-accent-bright",
  quiet: "border border-border bg-bg-elevated text-fg-body hover:border-accent/50 hover:text-accent",
};

function CtaLink({ cta }: { cta: LandingCta }) {
  const className = `inline-flex min-h-11 items-center justify-center gap-2 rounded-lg px-5 py-3 text-base font-semibold transition-colors ${FOCUS_RING} ${CTA_CLASS_NAMES[cta.kind]}`;
  const content = (
    <>
      {cta.label}
      <ArrowIcon />
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

function PathComparisonPanel() {
  return (
    <aside
      aria-label="Safe and unsafe pass comparison"
      className="overflow-hidden rounded-2xl border border-border bg-bg-elevated shadow-[var(--shadow-card-lg)]"
    >
      <div className="border-b border-border px-5 py-5 sm:px-6">
        <p className="text-sm font-semibold uppercase tracking-[0.08em] text-fg-muted">
          Final-state pass
        </p>
        <h2 className="mt-2 text-xl font-semibold leading-tight text-fg sm:text-2xl">
          {PATH_COMPARISON.title}
        </h2>
      </div>

      <dl className="divide-y divide-border-subtle md:hidden">
        {PATH_COMPARISON.rows.map((row) => (
          <div key={row.label} className="px-5 py-4">
            <dt className="text-sm font-semibold text-fg-body">{row.label}</dt>
            <dd className="mt-3 grid grid-cols-2 gap-2">
              <div className="rounded-lg border border-success/25 bg-success-tint p-3">
                <span className="flex items-center gap-2 text-sm font-semibold text-success">
                  <CheckIcon /> Agent A
                </span>
                <strong className="mt-2 block text-sm font-semibold text-fg">{row.safe}</strong>
              </div>
              <div className="rounded-lg border border-warning/25 bg-warning-tint p-3">
                <span className="flex items-center gap-2 text-sm font-semibold text-warning">
                  <WarningIcon /> Agent B
                </span>
                <strong className="mt-2 block text-sm font-semibold text-fg">{row.unsafe}</strong>
              </div>
            </dd>
          </div>
        ))}
      </dl>

      <div className="hidden md:block">
        <table className="w-full border-collapse text-left">
          <thead>
            <tr className="border-b border-border text-sm">
              <th className="px-6 py-4 font-medium text-fg-muted">Check</th>
              <th className="bg-success-tint px-4 py-4 font-semibold text-success">
                <span className="inline-flex items-center gap-2">
                  <CheckIcon /> Agent A
                </span>
              </th>
              <th className="bg-warning-tint px-4 py-4 font-semibold text-warning">
                <span className="inline-flex items-center gap-2">
                  <WarningIcon /> Agent B
                </span>
              </th>
            </tr>
          </thead>
          <tbody>
            {PATH_COMPARISON.rows.map((row) => (
              <tr key={row.label} className="border-b border-border-subtle last:border-0">
                <th className="px-6 py-4 text-sm font-medium text-fg-body">{row.label}</th>
                <td className="bg-success-tint/45 px-4 py-4 text-sm font-semibold text-fg">
                  {row.safe}
                </td>
                <td className="bg-warning-tint/45 px-4 py-4 text-sm font-semibold text-fg">
                  {row.unsafe}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      <Link
        to={BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH}
        className="flex items-center justify-between gap-3 border-t border-border px-5 py-4 text-sm font-semibold text-accent transition-colors hover:text-accent-bright focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-inset focus-visible:ring-accent sm:px-6"
      >
        Inspect the evidence behind the verdict
        <ArrowIcon />
      </Link>
    </aside>
  );
}

function ThemeToggle({ theme, toggle }: { theme: "light" | "dark"; toggle: () => void }) {
  return (
    <button
      type="button"
      onClick={toggle}
      className={`flex h-11 w-11 cursor-pointer items-center justify-center rounded-lg border border-border bg-bg-elevated text-base text-fg-muted transition-colors hover:border-accent hover:text-accent ${FOCUS_RING}`}
      aria-label="Toggle theme"
    >
      {theme === "dark" ? "\u2600" : "\u263E"}
    </button>
  );
}

export function Landing() {
  const { theme, toggle } = useTheme();

  return (
    <div className="min-h-screen bg-bg text-fg">
      <header className="sticky top-0 z-50 border-b border-border-subtle bg-bg/95 backdrop-blur-md">
        <div className="mx-auto flex max-w-7xl items-center justify-between gap-4 px-4 py-3 sm:px-6">
          <Link
            to="/"
            className={`shrink-0 text-base font-bold tracking-tight text-fg transition-colors hover:text-accent ${FOCUS_RING}`}
          >
            Evidra Bench
          </Link>

          <nav aria-label="Primary navigation" className="hidden items-center gap-7 lg:flex">
            <Link to={BENCH_SCENARIOS_PATH} className={`text-sm font-medium text-fg-muted hover:text-accent ${FOCUS_RING}`}>
              Scenarios
            </Link>
            <Link to={BENCH_LEADERBOARD_PATH} className={`text-sm font-medium text-fg-muted hover:text-accent ${FOCUS_RING}`}>
              Leaderboard
            </Link>
            <Link
              to={BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH}
              className={`text-sm font-medium text-fg-muted hover:text-accent ${FOCUS_RING}`}
            >
              Reports
            </Link>
            <a
              href="https://github.com/vitas/evidra-bench"
              target="_blank"
              rel="noopener noreferrer"
              className={`inline-flex items-center gap-2 text-sm font-medium text-fg-muted hover:text-accent ${FOCUS_RING}`}
            >
              <GitHubIcon /> GitHub
            </a>
          </nav>

          <div className="flex items-center gap-2">
            <Link
              to={BENCH_ONLINE_PATH}
              className={`inline-flex min-h-11 items-center justify-center rounded-lg bg-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white ${FOCUS_RING}`}
            >
              <span className="sm:hidden">Open Bench</span>
              <span className="hidden sm:inline">Open public Bench</span>
            </Link>
            <ThemeToggle theme={theme} toggle={toggle} />
          </div>
        </div>
      </header>

      <main>
        <section className="mx-auto grid max-w-7xl gap-12 px-5 pb-14 pt-12 sm:px-6 sm:pb-20 sm:pt-16 lg:grid-cols-[1.08fr_0.92fr] lg:items-center lg:gap-16 lg:py-24">
          <div className="min-w-0">
            <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">
              {HERO_CONTENT.eyebrow}
            </p>
            <h1 className="mt-5 max-w-[14ch] text-[clamp(2.65rem,6vw,4.5rem)] font-semibold leading-[1.02] tracking-[-0.035em] text-fg">
              {HERO_CONTENT.title}
            </h1>
            <p className="mt-6 max-w-[62ch] text-base leading-relaxed text-fg-muted sm:text-lg">
              {HERO_CONTENT.body}
            </p>
            <div className="mt-8 flex flex-col items-stretch gap-3 sm:flex-row sm:items-center">
              {HERO_CONTENT.ctas.map((cta) => (
                <CtaLink key={cta.label} cta={cta} />
              ))}
            </div>
          </div>

          <PathComparisonPanel />
        </section>

        <section aria-label="Benchmark proof" className="border-y border-border bg-bg-elevated">
          <ul className="mx-auto grid max-w-7xl divide-y divide-border px-5 sm:px-6 md:grid-cols-3 md:divide-x md:divide-y-0">
            {HERO_PROOF_POINTS.map((point) => (
              <li key={point} className="flex items-center gap-3 py-5 text-sm font-semibold text-fg-body md:px-7 md:first:pl-0 md:last:pr-0">
                <span className="text-success">
                  <CheckIcon className="h-5 w-5" />
                </span>
                {point}
              </li>
            ))}
          </ul>
        </section>

        <section className="mx-auto max-w-7xl px-5 py-16 sm:px-6 sm:py-24">
          <div>
            <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">Why Evidra</p>
            <h2 className="mt-3 max-w-3xl text-[clamp(2rem,4vw,3rem)] font-semibold leading-tight tracking-tight text-fg">
              What ordinary benchmarks miss
            </h2>
            <p className="mt-5 max-w-2xl text-base leading-relaxed text-fg-muted sm:text-lg">
              Task completion can hide the exact behavior that makes an infrastructure agent risky in production.
            </p>
          </div>

          <div className="mt-10 overflow-hidden rounded-2xl border border-border bg-bg-elevated">
            <div className="hidden grid-cols-[0.72fr_1fr_1.2fr] border-b border-border bg-bg-alt px-6 py-4 text-sm font-semibold text-fg-muted md:grid">
              <span>Dimension</span>
              <span>Typical benchmark</span>
              <span className="text-accent">Evidra Bench</span>
            </div>
            <dl className="divide-y divide-border-subtle">
              {BENCHMARK_DIFFERENCES.map((item) => (
                <div key={item.label} className="grid gap-5 px-5 py-6 md:grid-cols-[0.72fr_1fr_1.2fr] md:px-6">
                  <dt className="text-base font-semibold text-fg">{item.label}</dt>
                  <dd className="text-base leading-relaxed text-fg-muted">
                    <span className="mb-1 block text-sm font-semibold text-fg-muted md:hidden">Typical benchmark</span>
                    {item.typical}
                  </dd>
                  <dd className="text-base font-medium leading-relaxed text-fg-body">
                    <span className="mb-1 block text-sm font-semibold text-accent md:hidden">Evidra Bench</span>
                    {item.evidra}
                  </dd>
                </div>
              ))}
            </dl>
          </div>
        </section>

        <section className="border-y border-border bg-bg-elevated">
          <div className="mx-auto max-w-7xl px-5 py-16 sm:px-6 sm:py-24">
            <p className="text-sm font-semibold uppercase tracking-[0.08em] text-accent">How it works</p>
            <h2 className="mt-3 max-w-3xl text-[clamp(2rem,4vw,3rem)] font-semibold leading-tight tracking-tight text-fg">
              From candidate to inspectable verdict.
            </h2>

            <ol className="mt-10 grid overflow-hidden rounded-2xl border border-border bg-bg md:grid-cols-3 md:divide-x md:divide-border">
              {WORKFLOW_STEPS.map((step, index) => (
                <li key={step.title} className="border-b border-border p-6 last:border-b-0 md:border-b-0 md:p-8">
                  <span className="font-mono text-sm font-semibold text-accent">0{index + 1}</span>
                  <h3 className="mt-5 text-xl font-semibold text-fg">{step.title}</h3>
                  <p className="mt-3 text-base leading-relaxed text-fg-muted">{step.body}</p>
                </li>
              ))}
            </ol>
          </div>
        </section>

        <section className="mx-auto max-w-7xl px-5 py-16 sm:px-6 sm:py-24">
          <div className="rounded-2xl border border-border bg-bg-elevated p-7 shadow-[var(--shadow-card)] sm:flex sm:items-center sm:justify-between sm:gap-10 sm:p-10">
            <div>
              <h2 className="text-[clamp(2rem,4vw,3rem)] font-semibold leading-tight tracking-tight text-fg">
                See what a pass is hiding.
              </h2>
              <p className="mt-4 max-w-2xl text-base leading-relaxed text-fg-muted sm:text-lg">
                Explore public scenarios, live results, and attached artifacts without a sales call.
              </p>
            </div>
            <Link
              to={BENCH_ONLINE_PATH}
              className={`mt-7 inline-flex min-h-11 shrink-0 items-center justify-center gap-2 rounded-lg bg-accent px-5 py-3 text-base font-semibold text-white transition-colors hover:bg-accent-bright hover:text-white sm:mt-0 ${FOCUS_RING}`}
            >
              Explore public Bench
              <ArrowIcon />
            </Link>
          </div>
        </section>
      </main>

      <footer className="border-t border-border">
        <div className="mx-auto flex max-w-7xl flex-col gap-5 px-5 py-8 text-sm text-fg-muted sm:px-6 md:flex-row md:items-center md:justify-between">
          <span>Evidra Bench — path-aware evaluation for AI infrastructure agents</span>
          <nav aria-label="Footer navigation" className="flex flex-wrap items-center gap-x-5 gap-y-3">
            <Link to={BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
              Real report
            </Link>
            <Link to={BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
              Methodology
            </Link>
            <Link to={BENCH_SCENARIOS_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
              Scenarios
            </Link>
            <Link to={BENCH_LEADERBOARD_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
              Leaderboard
            </Link>
            <a href={BENCH_PRIVATE_REQUEST_MAILTO} className={`hover:text-accent ${FOCUS_RING}`}>
              Private evaluation
            </a>
            <a
              href="https://github.com/vitas/evidra-bench"
              target="_blank"
              rel="noopener noreferrer"
              className={`hover:text-accent ${FOCUS_RING}`}
            >
              GitHub
            </a>
          </nav>
        </div>
      </footer>
    </div>
  );
}
