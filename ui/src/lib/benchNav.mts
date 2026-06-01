export type BenchNavItem = {
  to: string;
  label: string;
};

export const BENCH_PRIMARY_NAV: BenchNavItem[] = [
  { to: "/bench", label: "Leaderboard" },
  { to: "/bench/runs", label: "Runs" },
  { to: "/bench/scenarios", label: "Scenarios" },
  { to: "/bench/compare", label: "Compare" },
  { to: "/bench/reports/tool-server", label: "Reports" },
];

export const BENCH_SECONDARY_NAV: BenchNavItem[] = [
  { to: "/bench/dashboard", label: "Dashboard" },
  { to: "/bench/skill-impact", label: "Skill Impact" },
  { to: "/bench/regressions", label: "Regressions" },
  { to: "/bench/insights", label: "Insights" },
  { to: "/bench/reviews", label: "Reviews" },
  { to: "/bench/session", label: "Session" },
  { to: "/bench/benchmarks", label: "Benchmarks" },
];
