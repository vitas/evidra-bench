import { type ReactNode } from "react";
import { NavLink } from "react-router";
import { useTheme } from "../../hooks/useTheme";
import { useAppInfo } from "../../hooks/useAppInfo";
import { useEvidenceMode, type EvidenceMode } from "../../hooks/useEvidenceMode";

const navItems = [
  { to: "/", label: "Leaderboard" },
  { to: "/dashboard", label: "Dashboard" },
  { to: "/skill-impact", label: "Skill Impact" },
  { to: "/regressions", label: "Regressions" },
  { to: "/insights", label: "Insights" },
  { to: "/runs", label: "Runs" },
  { to: "/scenarios", label: "Scenarios" },
  { to: "/compare", label: "Compare" },
  { to: "/benchmarks", label: "Benchmarks" },
];

export function Layout({ children }: { children: ReactNode }) {
  const { theme, toggle } = useTheme();
  const { readonly, version } = useAppInfo();
  const { mode, setMode } = useEvidenceMode();

  const modeOptions: { value: EvidenceMode; label: string }[] = [
    { value: "proxy", label: "Proxy" },
    { value: "smart", label: "Smart" },
  ];

  return (
    <>
      <header
        className="sticky top-0 z-50 h-14 flex items-center gap-8 px-6 border-b border-border-subtle"
        style={{
          background: "color-mix(in srgb, var(--color-bg-elevated) 85%, transparent)",
          backdropFilter: "blur(12px)",
        }}
      >
        <div className="flex items-center gap-2 font-extrabold text-fg tracking-tight whitespace-nowrap">
          <a
            href="https://evidra.cc"
            target="_blank"
            rel="noopener noreferrer"
            className="inline-flex items-center gap-2 hover:text-accent transition-colors"
            style={{ textDecoration: "none", color: "inherit" }}
          >
            <span
              className="inline-block w-2 h-2 bg-accent rounded-sm"
              style={{ transform: "rotate(45deg)" }}
            />
            Evidra Bench
          </a>
        </div>

        {readonly && (
          <span className="text-[0.68rem] font-semibold uppercase tracking-wide px-2 py-0.5 rounded bg-warning-tint text-warning border border-warning">
            Demo
          </span>
        )}

        <div className="flex items-center rounded-md border border-border overflow-hidden text-[0.72rem] font-semibold">
          {modeOptions.map((opt) => (
            <button
              key={opt.value}
              onClick={() => setMode(opt.value)}
              className={`px-2.5 py-1 cursor-pointer transition-all ${
                mode === opt.value
                  ? "bg-accent text-bg"
                  : "text-fg-muted hover:text-fg"
              }`}
              style={{ background: mode === opt.value ? "var(--color-accent)" : "none", border: "none" }}
            >
              {opt.label}
            </button>
          ))}
        </div>

        <nav className="flex gap-1 flex-1">
          {navItems.map(({ to, label }) => (
            <NavLink
              key={to}
              to={to}
              end={to === "/"}
              className={({ isActive }) =>
                `text-[0.83rem] font-medium px-3 py-1.5 rounded-md transition-all ${
                  isActive
                    ? "text-accent bg-accent-tint"
                    : "text-fg-muted hover:text-fg hover:bg-accent-subtle"
                }`
              }
            >
              {label}
            </NavLink>
          ))}
        </nav>

        <a
          href="https://github.com/vitas/evidra"
          target="_blank"
          rel="noopener noreferrer"
          className="w-[34px] h-[34px] flex items-center justify-center rounded-md border border-border text-fg-muted hover:border-accent hover:text-accent transition-all"
          aria-label="GitHub"
        >
          <svg width="18" height="18" viewBox="0 0 24 24" fill="currentColor">
            <path d="M12 0C5.37 0 0 5.37 0 12c0 5.31 3.435 9.795 8.205 11.385.6.105.825-.255.825-.57 0-.285-.015-1.23-.015-2.235-3.015.555-3.795-.735-4.035-1.41-.135-.345-.72-1.41-1.23-1.695-.42-.225-1.02-.78-.015-.795.945-.015 1.62.87 1.845 1.23 1.08 1.815 2.805 1.305 3.495.99.105-.78.42-1.305.765-1.605-2.67-.3-5.46-1.335-5.46-5.925 0-1.305.465-2.385 1.23-3.225-.12-.3-.54-1.53.12-3.18 0 0 1.005-.315 3.3 1.23.96-.27 1.98-.405 3-.405s2.04.135 3 .405c2.295-1.56 3.3-1.23 3.3-1.23.66 1.65.24 2.88.12 3.18.765.84 1.23 1.905 1.23 3.225 0 4.605-2.805 5.625-5.475 5.925.435.375.81 1.095.81 2.22 0 1.605-.015 2.895-.015 3.3 0 .315.225.69.825.57A12.02 12.02 0 0024 12c0-6.63-5.37-12-12-12z" />
          </svg>
        </a>

        <button
          onClick={toggle}
          className="w-[34px] h-[34px] flex items-center justify-center rounded-md border border-border text-fg-muted hover:border-accent hover:text-accent transition-all cursor-pointer"
          style={{ background: "none", fontSize: "1rem" }}
          aria-label="Toggle theme"
        >
          {theme === "dark" ? "\u2600" : "\u263E"}
        </button>
      </header>

      <main className="max-w-[1280px] mx-auto px-6 py-5 pb-12">
        {children}
      </main>

      <footer className="border-t border-border-subtle mt-8">
        <div className="max-w-[1280px] mx-auto px-6 py-6 flex flex-wrap items-center justify-between gap-4 text-[0.75rem] text-fg-muted">
          <div className="flex items-center gap-4">
            <a
              href="https://evidra.cc"
              target="_blank"
              rel="noopener noreferrer"
              className="hover:text-accent transition-colors"
            >
              evidra.cc
            </a>
            <a
              href="https://github.com/vitas/evidra"
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
