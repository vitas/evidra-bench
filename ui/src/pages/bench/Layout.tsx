import { type ReactNode } from "react";
import { NavLink } from "react-router";
import { useTheme } from "../../hooks/useTheme";
import { useAppInfo } from "../../hooks/useAppInfo";
import { SCENARIOS } from "../../data/catalog";
import { BENCH_FEATURE_NAV, BENCH_PRIMARY_NAV, type BenchNavItem } from "../../lib/benchNav.mts";

function BenchNavLink({ item, variant = "primary" }: { item: BenchNavItem; variant?: "primary" | "feature" }) {
  return (
    <NavLink
      to={item.to}
      end={item.to === "/bench"}
      className={({ isActive }) =>
        `${variant === "primary" ? "text-[0.8rem] px-2.5 py-1.5" : "text-[0.72rem] px-2 py-1"} font-semibold rounded-md transition-all whitespace-nowrap ${
          isActive
            ? "text-accent bg-accent-tint"
            : "text-fg-muted hover:text-fg hover:bg-accent-subtle"
        }`
      }
    >
      {item.label}
    </NavLink>
  );
}

export function Layout({ children }: { children: ReactNode }) {
  const { theme, toggle } = useTheme();
  const { readonly, version } = useAppInfo();
  const activeScenarioCount = SCENARIOS.length;

  return (
    <>
      <header
        className="sticky top-0 z-50 glass border-b-0"
        style={{ borderBottom: "1px solid var(--glass-border)" }}
      >
        <div className="flex min-h-14 items-center gap-3 px-4 sm:px-6 py-2">
          <div className="flex items-center gap-3 min-w-0">
            <NavLink
              to="/"
              className="inline-flex items-center gap-2 text-fg hover:text-accent transition-colors whitespace-nowrap"
              style={{ textDecoration: "none", color: "inherit" }}
            >
              <span
                className="inline-block w-2.5 h-2.5 bg-accent rounded-sm"
                style={{ transform: "rotate(45deg)" }}
              />
              <span className="text-base font-extrabold tracking-tight hidden sm:inline">Bench Lab</span>
              <span className="text-base font-extrabold tracking-tight sm:hidden">Bench</span>
            </NavLink>
            <span className="hidden lg:inline text-[0.72rem] font-semibold text-fg-muted whitespace-nowrap">
              Live AI SRE benchmark workspace
            </span>
          </div>

          {readonly && (
            <span className="text-[0.68rem] font-semibold uppercase tracking-wide px-2 py-0.5 rounded bg-warning-tint text-warning border border-warning">
              Demo
            </span>
          )}

          <span className="hidden md:inline-flex items-center gap-1.5 text-[0.68rem] font-semibold text-fg-muted px-2 py-1 rounded-md border border-border bg-bg-elevated">
            <span className="w-1.5 h-1.5 rounded-full bg-accent" />
            {activeScenarioCount} active scenarios
          </span>

          <div className="flex-1" />

          <NavLink
            to="/bench/session"
            className="hidden md:inline-flex items-center justify-center rounded-md bg-accent px-3 py-1.5 text-[0.75rem] font-bold text-white hover:text-white hover:bg-accent-bright transition-colors whitespace-nowrap"
          >
            API Session
          </NavLink>

          <a
            href="https://github.com/vitas/evidra-bench"
            target="_blank"
            rel="noopener noreferrer"
            className="hidden sm:flex w-[30px] h-[30px] items-center justify-center rounded-md border border-border text-fg-muted hover:border-accent hover:text-accent transition-all"
            aria-label="GitHub"
          >
            <svg width="16" height="16" viewBox="0 0 24 24" fill="currentColor">
              <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
            </svg>
          </a>

          <button
            onClick={toggle}
            className="w-[30px] h-[30px] flex items-center justify-center rounded-md border border-border text-fg-muted hover:border-accent hover:text-accent transition-all cursor-pointer"
            style={{ background: "none", fontSize: "0.9rem" }}
            aria-label="Toggle theme"
          >
            {theme === "dark" ? "\u2600" : "\u263E"}
          </button>
        </div>

        <div className="border-t border-border-subtle">
          <nav className="flex gap-1 px-4 sm:px-6 py-2 overflow-x-auto scrollbar-none">
            {BENCH_PRIMARY_NAV.map((item) => (
              <BenchNavLink key={item.to} item={item} />
            ))}
          </nav>
          <div className="flex items-center gap-1 px-4 sm:px-6 pb-2 overflow-x-auto scrollbar-none">
            <span className="text-[0.65rem] font-bold uppercase tracking-wide text-fg-muted whitespace-nowrap mr-1">
              Online bench
            </span>
            {BENCH_FEATURE_NAV.map((item) => (
              <BenchNavLink key={item.to} item={item} variant="feature" />
            ))}
          </div>
        </div>
      </header>

      <main className="max-w-[1280px] mx-auto px-6 py-5 pb-12">
        {children}
      </main>

      <footer className="border-t border-border-subtle mt-8">
        <div className="max-w-[1280px] mx-auto px-6 py-6 flex flex-wrap items-center justify-between gap-4 text-[0.75rem] text-fg-muted">
          <div className="flex items-center gap-4">
            <a
              href="/"
              className="hover:text-accent transition-colors"
            >
              Bench
            </a>
            <a
              href="https://github.com/vitas/evidra-bench"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-accent transition-colors"
            >
              GitHub
            </a>
          </div>
          {version && (
            <span className="font-mono text-[0.7rem]">{version}</span>
          )}
        </div>
      </footer>
    </>
  );
}
