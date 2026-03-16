import { useState, useEffect, useMemo } from "react";
import { Link } from "react-router";
import { useApi } from "../hooks/useApi";

/* ── Types ── */

type Period = "24h" | "7d" | "30d" | "90d" | "all";

interface ScenarioStat {
  ScenarioID: string;
  Runs: number;
  Passed: number;
}

interface Stats {
  TotalRuns: number;
  PassCount: number;
  FailCount: number;
  ByScenario: ScenarioStat[];
}

interface Run {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  passed: boolean;
  duration_seconds: number;
  estimated_cost_usd: number;
  created_at: string;
}

interface RunsResponse {
  items: Run[];
  total: number;
}

/* ── Helpers ── */

const PERIODS: { value: Period; label: string }[] = [
  { value: "24h", label: "24 h" },
  { value: "7d", label: "7 d" },
  { value: "30d", label: "30 d" },
  { value: "90d", label: "90 d" },
  { value: "all", label: "All" },
];

const PERIOD_MS: Record<Exclude<Period, "all">, number> = {
  "24h": 86_400_000,
  "7d": 7 * 86_400_000,
  "30d": 30 * 86_400_000,
  "90d": 90 * 86_400_000,
};

function periodToSince(p: Period): string | undefined {
  if (p === "all") return undefined;
  return new Date(Date.now() - PERIOD_MS[p]).toISOString();
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  const day = String(d.getDate()).padStart(2, "0");
  const mon = d.toLocaleString("en-US", { month: "short" });
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${day} ${mon} ${hh}:${mm}`;
}

function formatDuration(s: number): string {
  return `${s.toFixed(1)}s`;
}

function formatCost(usd: number): string {
  if (usd < 0.001) return "$0.00";
  if (usd < 0.01) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(3)}`;
}

/* ── Skeleton pulse ── */

function Pulse({ className = "" }: { className?: string }) {
  return (
    <div
      className={`animate-pulse rounded bg-border-subtle ${className}`}
    />
  );
}

/* ── Signals ── */

const PRIMARY_SIGNALS = [
  { id: "protocol_violation", label: "Protocol Violation" },
  { id: "blast_radius", label: "Blast Radius" },
  { id: "retry_loop", label: "Retry Loop" },
];

const SECONDARY_SIGNALS = [
  { id: "thrashing", label: "Thrashing" },
  { id: "repair_loop", label: "Repair Loop" },
  { id: "artifact_drift", label: "Artifact Drift" },
  { id: "risk_escalation", label: "Risk Escalation" },
  { id: "new_scope", label: "New Scope" },
];

/* ── Component ── */

export function Dashboard() {
  const { request } = useApi();
  const [period, setPeriod] = useState<Period>("30d");
  const [stats, setStats] = useState<Stats | null>(null);
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    const since = periodToSince(period);
    const sinceParam = since ? `?since=${encodeURIComponent(since)}` : "";

    Promise.all([
      request<Stats>(`/v1/bench/stats${sinceParam}`),
      request<RunsResponse>("/v1/bench/runs?limit=5"),
    ])
      .then(([s, r]) => {
        if (cancelled) return;
        setStats(s);
        setRuns(r.items ?? []);
      })
      .catch(() => {
        if (cancelled) return;
        setStats(null);
        setRuns([]);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [period, request]);

  /* Derived */
  const passRate = stats && stats.TotalRuns > 0
    ? ((stats.PassCount / stats.TotalRuns) * 100).toFixed(1)
    : "0.0";

  const distinctModels = useMemo(
    () => [...new Set(runs.map((r) => r.model))],
    [runs],
  );

  const modelPassRates = useMemo(() => {
    const map = new Map<string, { total: number; passed: number }>();
    runs.forEach((r) => {
      const entry = map.get(r.model) ?? { total: 0, passed: 0 };
      entry.total += 1;
      if (r.passed) entry.passed += 1;
      map.set(r.model, entry);
    });
    return [...map.entries()].map(([model, { total, passed }]) => ({
      model,
      rate: total > 0 ? Math.round((passed / total) * 100) : 0,
      total,
      passed,
    }));
  }, [runs]);

  /* ── Render ── */

  return (
    <div className="space-y-6">
      {/* Header */}
      <div className="flex items-center justify-between">
        <div>
          <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
            Dashboard
          </h1>
          <p className="text-[0.85rem] text-fg-muted mt-0.5">
            Infrastructure agent benchmark overview
          </p>
        </div>

        <div className="flex gap-0.5 bg-bg-alt rounded-lg p-0.5">
          {PERIODS.map(({ value, label }) => (
            <button
              key={value}
              onClick={() => setPeriod(value)}
              className={`font-mono text-[0.74rem] px-2.5 py-1 rounded-md transition-all cursor-pointer ${
                period === value
                  ? "bg-accent-tint text-accent font-semibold"
                  : "text-fg-muted hover:text-fg"
              }`}
            >
              {label}
            </button>
          ))}
        </div>
      </div>

      {/* Stat cards */}
      <div className="grid grid-cols-1 sm:grid-cols-2 md:grid-cols-3 xl:grid-cols-5 gap-4">
        {loading ? (
          Array.from({ length: 5 }).map((_, i) => (
            <div
              key={i}
              className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-4"
            >
              <Pulse className="h-3 w-20 mb-3" />
              <Pulse className="h-7 w-16" />
            </div>
          ))
        ) : (
          <>
            <StatCard
              label="Total Runs"
              value={String(stats?.TotalRuns ?? 0)}
              borderColor="border-l-accent"
            />
            <StatCard
              label="Pass Rate"
              value={`${passRate}%`}
              detail={`${stats?.PassCount ?? 0} / ${stats?.TotalRuns ?? 0}`}
              borderColor="border-l-accent"
            />
            <StatCard
              label="Models Tested"
              value={String(distinctModels.length)}
              borderColor="border-l-info"
            />
            <StatCard
              label="Signal Alerts"
              value="--"
              borderColor="border-l-warning"
            />
            <StatCard
              label="Latest Score"
              value="--"
              borderColor="border-l-fg-muted"
            />
          </>
        )}
      </div>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4">
        {/* Recent Runs */}
        <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-5">
          <h2 className="text-[0.95rem] font-semibold text-fg mb-4">
            Recent Runs
          </h2>
          {loading ? (
            <div className="space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Pulse key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : runs.length === 0 ? (
            <p className="text-fg-muted text-[0.85rem] py-6 text-center">
              No runs recorded yet.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[0.82rem]">
                <thead>
                  <tr className="border-b border-border-subtle">
                    {["Status", "Scenario", "Model", "Duration", "Cost", "Date"].map(
                      (h) => (
                        <th
                          key={h}
                          className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted pb-2 pr-3"
                        >
                          {h}
                        </th>
                      ),
                    )}
                  </tr>
                </thead>
                <tbody>
                  {runs.map((run) => (
                    <tr
                      key={run.id}
                      className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors"
                    >
                      <td className="py-2.5 pr-3">
                        <span
                          className={`inline-block font-mono text-[0.72rem] font-semibold px-2 py-0.5 rounded ${
                            run.passed
                              ? "bg-accent-tint text-accent"
                              : "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]"
                          }`}
                        >
                          {run.passed ? "PASS" : "FAIL"}
                        </span>
                      </td>
                      <td className="py-2.5 pr-3">
                        <Link
                          to={`/runs/${run.id}`}
                          className="text-accent hover:text-accent-bright font-medium"
                        >
                          {run.scenario_id}
                        </Link>
                      </td>
                      <td className="py-2.5 pr-3 font-mono text-fg-muted">
                        {run.model}
                      </td>
                      <td className="py-2.5 pr-3 font-mono text-fg-muted">
                        {formatDuration(run.duration_seconds)}
                      </td>
                      <td className="py-2.5 pr-3 font-mono text-fg-muted">
                        {formatCost(run.estimated_cost_usd)}
                      </td>
                      <td className="py-2.5 pr-3 text-fg-muted whitespace-nowrap">
                        {formatDate(run.created_at)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </div>

        {/* Right column */}
        <div className="space-y-4">
          {/* Pass Rate by Model */}
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-5">
            <h2 className="text-[0.95rem] font-semibold text-fg mb-4">
              Pass Rate by Model
            </h2>
            {loading ? (
              <div className="space-y-3">
                {Array.from({ length: 3 }).map((_, i) => (
                  <Pulse key={i} className="h-6 w-full" />
                ))}
              </div>
            ) : modelPassRates.length === 0 ? (
              <p className="text-fg-muted text-[0.82rem] text-center py-4">
                No data available.
              </p>
            ) : (
              <div className="space-y-3">
                {modelPassRates.map(({ model, rate, passed, total }) => (
                  <div key={model}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-mono text-[0.78rem] text-fg">
                        {model}
                      </span>
                      <span className="font-mono text-[0.72rem] text-fg-muted">
                        {passed}/{total} ({rate}%)
                      </span>
                    </div>
                    <div className="h-2 rounded-full bg-bg-alt overflow-hidden">
                      <div
                        className="h-full rounded-full bg-accent transition-all duration-500"
                        style={{ width: `${rate}%` }}
                      />
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Run Activity */}
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-5">
            <h2 className="text-[0.95rem] font-semibold text-fg mb-4">
              Run Activity
            </h2>
            <div className="flex items-end gap-1.5 h-20">
              {Array.from({ length: 14 }).map((_, i) => {
                const height = Math.max(8, Math.random() * 100);
                return (
                  <div
                    key={i}
                    className="flex-1 rounded-sm bg-accent-tint"
                    style={{ height: `${height}%` }}
                  />
                );
              })}
            </div>
            <p className="text-[0.7rem] text-fg-muted mt-2 text-center">
              Placeholder -- activity chart
            </p>
          </div>
        </div>
      </div>

      {/* Signal Overview */}
      <div>
        <h2 className="text-[0.95rem] font-semibold text-fg mb-3">
          Signal Overview
        </h2>

        {/* Primary signals */}
        <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mb-3">
          {PRIMARY_SIGNALS.map(({ id, label }) => (
            <div
              key={id}
              className="bg-warning-tint border border-warning rounded-lg p-4"
            >
              <p className="font-mono text-[0.72rem] uppercase tracking-wide text-warning font-semibold mb-1">
                {label}
              </p>
              <p className="text-[1.25rem] font-bold text-fg">--</p>
              <p className="text-[0.72rem] text-fg-muted mt-0.5">
                Across all runs
              </p>
            </div>
          ))}
        </div>

        {/* Secondary signals */}
        <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
          {SECONDARY_SIGNALS.map(({ id, label }) => (
            <div
              key={id}
              className="bg-bg-elevated border border-border-subtle rounded-lg p-4"
            >
              <p className="font-mono text-[0.72rem] uppercase tracking-wide text-fg-muted font-semibold mb-1">
                {label}
              </p>
              <p className="text-[1.25rem] font-bold text-fg">--</p>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}

/* ── StatCard ── */

function StatCard({
  label,
  value,
  detail,
  borderColor,
}: {
  label: string;
  value: string;
  detail?: string;
  borderColor: string;
}) {
  return (
    <div
      className={`bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] p-4 border-l-[3px] ${borderColor}`}
    >
      <p className="text-[0.72rem] font-semibold uppercase tracking-wide text-fg-muted mb-1">
        {label}
      </p>
      <p className="text-[1.5rem] font-bold text-fg leading-tight">{value}</p>
      {detail && (
        <p className="text-[0.72rem] text-fg-muted mt-0.5 font-mono">
          {detail}
        </p>
      )}
    </div>
  );
}
