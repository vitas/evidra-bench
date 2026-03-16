import { type ReactNode } from "react";
import { NavLink } from "react-router";
import { useTheme } from "./hooks/useTheme";

const navItems = [
  { to: "/", label: "Dashboard" },
  { to: "/runs", label: "Runs" },
  { to: "/scenarios", label: "Scenarios" },
  { to: "/compare", label: "Compare" },
  { to: "/benchmarks", label: "Benchmarks" },
];

export function Layout({ children }: { children: ReactNode }) {
  const { theme, toggle } = useTheme();

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
          <span
            className="inline-block w-2 h-2 bg-accent rounded-sm"
            style={{ transform: "rotate(45deg)" }}
          />
          Evidra Bench
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
    </>
  );
}
