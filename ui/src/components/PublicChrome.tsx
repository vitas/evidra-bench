import { Link } from "react-router";
import { useTheme } from "../hooks/useTheme";
import {
  BENCH_KUBERNETES_CORE_PATH,
  BENCH_LEADERBOARD_PATH,
  BENCH_ONLINE_PATH,
  BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH,
  BENCH_SCENARIOS_PATH,
} from "../lib/routes.mts";

export const FOCUS_RING =
  "focus-visible:outline-none focus-visible:ring-2 focus-visible:ring-accent focus-visible:ring-offset-2 focus-visible:ring-offset-bg";

export const BENCH_GITHUB_URL = "https://github.com/vitas/evidra-bench";

export type PublicCta = {
  href: string;
  kind: "primary" | "secondary" | "quiet";
  label: string;
};

export function ArrowIcon({ className = "h-4 w-4" }: { className?: string }) {
  return (
    <svg className={className} viewBox="0 0 16 16" fill="none" aria-hidden="true">
      <path
        d="M3 8h9M8.5 4.5 12 8l-3.5 3.5"
        stroke="currentColor"
        strokeWidth="1.6"
        strokeLinecap="round"
        strokeLinejoin="round"
      />
    </svg>
  );
}

export function GitHubIcon() {
  return (
    <svg className="h-4 w-4" viewBox="0 0 16 16" fill="currentColor" aria-hidden="true">
      <path d="M8 0C3.58 0 0 3.58 0 8c0 3.54 2.29 6.53 5.47 7.59.4.07.55-.17.55-.38 0-.19-.01-.82-.01-1.49-2.01.37-2.53-.49-2.69-.94-.09-.23-.48-.94-.82-1.13-.28-.15-.68-.52-.01-.53.63-.01 1.08.58 1.23.82.72 1.21 1.87.87 2.33.66.07-.52.28-.87.51-1.07-1.78-.2-3.64-.89-3.64-3.95 0-.87.31-1.59.82-2.15-.08-.2-.36-1.02.08-2.12 0 0 .67-.21 2.2.82a7.4 7.4 0 0 1 2-.27c.68 0 1.36.09 2 .27 1.53-1.04 2.2-.82 2.2-.82.44 1.1.16 1.92.08 2.12.51.56.82 1.27.82 2.15 0 3.07-1.87 3.75-3.65 3.95.29.25.54.73.54 1.48 0 1.07-.01 1.93-.01 2.2 0 .21.15.46.55.38A8.01 8.01 0 0 0 16 8c0-4.42-3.58-8-8-8Z" />
    </svg>
  );
}

export function CtaLink({ cta }: { cta: PublicCta }) {
  const base = `inline-flex min-h-11 items-center justify-center gap-2 rounded-lg px-5 py-3 text-sm font-semibold transition-colors ${FOCUS_RING}`;

  if (cta.kind === "primary") {
    return (
      <Link to={cta.href} className={`${base} bg-accent text-white hover:bg-accent-bright hover:text-white`}>
        {cta.label}
        <ArrowIcon />
      </Link>
    );
  }

  if (cta.kind === "secondary") {
    return (
      <Link
        to={cta.href}
        className={`${base} border border-border bg-bg-elevated text-fg hover:border-accent hover:text-accent`}
      >
        {cta.label}
      </Link>
    );
  }

  return (
    <Link to={cta.href} className={`${base} text-fg-muted hover:text-accent`}>
      {cta.label}
    </Link>
  );
}

export function ThemeToggle({ theme, toggle }: { theme: "light" | "dark"; toggle: () => void }) {
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

export function PublicHeader() {
  const { theme, toggle } = useTheme();

  return (
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
          <Link to={BENCH_KUBERNETES_CORE_PATH} className={`text-sm font-medium text-fg-muted hover:text-accent ${FOCUS_RING}`}>
            Core pack
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
            href={BENCH_GITHUB_URL}
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
  );
}

export function PublicFooter() {
  return (
    <footer className="border-t border-border">
      <div className="mx-auto flex max-w-7xl flex-col gap-5 px-5 py-8 text-sm text-fg-muted sm:px-6 md:flex-row md:items-center md:justify-between">
        <span>Evidra Bench — path-aware evaluation for AI infrastructure agents</span>
        <nav aria-label="Footer navigation" className="flex flex-wrap items-center gap-x-5 gap-y-3">
          <Link to={BENCH_KUBERNETES_CORE_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
            Core pack
          </Link>
          <Link to={BENCH_SCENARIOS_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
            Scenarios
          </Link>
          <Link to={BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
            Reports
          </Link>
          <Link to={BENCH_LEADERBOARD_PATH} className={`hover:text-accent ${FOCUS_RING}`}>
            Leaderboard
          </Link>
          <a
            href={BENCH_GITHUB_URL}
            target="_blank"
            rel="noopener noreferrer"
            className={`hover:text-accent ${FOCUS_RING}`}
          >
            GitHub
          </a>
        </nav>
      </div>
    </footer>
  );
}
