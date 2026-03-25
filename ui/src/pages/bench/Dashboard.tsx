import { usePageTitle } from "../../hooks/usePageTitle";
import { useState, useEffect, useMemo } from "react";
import { Link } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { buildRunsPath, evidenceModeParam } from "../../lib/catalogData.mts";
import { useEvidenceMode } from "../../hooks/useEvidenceMode";

/* ── Types ── */

type Period = "24h" | "7d" | "30d" | "90d" | "all";

interface ScenarioStat {
  scenario_id: string;
  runs: number;
  passed: number;
}

interface Stats {
  total_runs: number;
  pass_count: number;
  fail_count: number;
  by_scenario: ScenarioStat[];
}

interface Run {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  passed: boolean;
  duration_seconds: number;
  estimated_cost_usd: number;
  prompt_tokens: number;
  completion_tokens: number;
  created_at: string;
}

interface RunsResponse {
  items: Run[];
  total: number;
}

interface SignalCount {
  total: number;
  run_count: number;
}

interface SignalAggregation {
  total_runs: number;
  runs_with_scorecard: number;
  signals: Record<string, SignalCount>;
  avg_score: number;
}

const PRIMARY_SIGNALS = ["protocol_violation", "blast_radius", "retry_loop"] as const;
const SECONDARY_SIGNALS = ["thrashing", "repair_loop", "artifact_drift", "risk_escalation", "new_scope"] as const;

function signalLabel(id: string): string {
  return id.replace(/_/g, " ").replace(/\b\w/g, (c) => c.toUpperCase());
}

/* ── Helpers ── */

const PERIODS: { value: Period; label: string }[] = [
  { value: "24h", label: "24h" },
  { value: "7d", label: "7d" },
  { value: "30d", label: "30d" },
  { value: "90d", label: "90d" },
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
  if (isNaN(d.getTime())) return iso;
  const day = String(d.getDate()).padStart(2, "0");
  const mon = d.toLocaleString("en-US", { month: "short" });
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${day} ${mon} ${hh}:${mm}`;
}

function formatDateShort(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "";
  return `${d.getDate()} ${d.toLocaleString("en-US", { month: "short" })}`;
}

function formatDuration(s: number): string {
  return `${s.toFixed(1)}s`;
}

function formatCost(usd: number): string {
  if (usd < 0.001) return "$0.00";
  return `$${usd.toFixed(3)}`;
}

function formatTokens(n: number): string {
  if (n >= 1000) return `${(n / 1000).toFixed(1)}k`;
  return String(n);
}

/* ── Skeleton pulse ── */

function Pulse({ className = "" }: { className?: string }) {
  return (
    <div
      className={`animate-pulse rounded bg-border-subtle ${className}`}
    />
  );
}

/* ── Component ── */

export function Dashboard() {
  usePageTitle("Dashboard");
  const { request } = useApi();
  const { mode } = useEvidenceMode();
  const [period, setPeriod] = useState<Period>("7d");
  const [stats, setStats] = useState<Stats | null>(null);
  const [recentRuns, setRecentRuns] = useState<Run[]>([]);
  const [allRuns, setAllRuns] = useState<Run[]>([]);
  const [signals, setSignals] = useState<SignalAggregation | null>(null);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    const since = periodToSince(period);
    const modeQ = evidenceModeParam("?", mode);
    const modeAmp = evidenceModeParam("&", mode);
    // When evidence mode is "all", modeQ is empty so since needs "?" prefix.
    const sinceQ = since ? `${modeQ ? "&" : "?"}since=${encodeURIComponent(since)}` : "";
    const sinceAmp = since ? `&since=${encodeURIComponent(since)}` : "";

    Promise.all([
      request<Stats>(`/v1/bench/stats${modeQ}${sinceQ}`),
      request<RunsResponse>(buildRunsPath(8, since, mode)),
      request<RunsResponse>(`/v1/bench/runs?limit=500${modeAmp}${sinceAmp}`),
      request<SignalAggregation>(`/v1/bench/signals${modeQ}${sinceQ}`).catch(() => null),
    ])
      .then(([s, recent, all, sig]) => {
        if (cancelled) return;
        setStats(s);
        setRecentRuns(recent.items ?? []);
        setAllRuns(all.items ?? []);
        setSignals(sig);
      })
      .catch(() => {
        if (cancelled) return;
        setStats(null);
        setRecentRuns([]);
        setAllRuns([]);
        setSignals(null);
      })
      .finally(() => {
        if (!cancelled) setLoading(false);
      });

    return () => {
      cancelled = true;
    };
  }, [period, request, mode]);

  /* Derived data */
  const passRate =
    stats && stats.total_runs > 0
      ? ((stats.pass_count / stats.total_runs) * 100).toFixed(1)
      : "0.0";

  const distinctModels = useMemo(
    () => [...new Set(allRuns.map((r) => r.model).filter(Boolean))],
    [allRuns],
  );

  const totalCost = useMemo(
    () => allRuns.reduce((sum, r) => sum + (r.estimated_cost_usd || 0), 0),
    [allRuns],
  );

  const totalTokens = useMemo(
    () =>
      allRuns.reduce(
        (sum, r) => sum + (r.prompt_tokens || 0) + (r.completion_tokens || 0),
        0,
      ),
    [allRuns],
  );

  const totalSignals = useMemo(() => {
    if (!signals?.signals) return 0;
    return Object.values(signals.signals).reduce((sum, s) => sum + s.total, 0);
  }, [signals]);

  const modelPassRates = useMemo(() => {
    const map = new Map<string, { total: number; passed: number; cost: number }>();
    allRuns.forEach((r) => {
      if (!r.model) return;
      const entry = map.get(r.model) ?? { total: 0, passed: 0, cost: 0 };
      entry.total += 1;
      if (r.passed) entry.passed += 1;
      entry.cost += r.estimated_cost_usd || 0;
      map.set(r.model, entry);
    });
    return [...map.entries()]
      .map(([model, { total, passed, cost }]) => ({
        model,
        rate: total > 0 ? Math.round((passed / total) * 100) : 0,
        total,
        passed,
        cost,
      }))
      .sort((a, b) => b.rate - a.rate);
  }, [allRuns]);

  // Activity: group runs by day
  const dailyActivity = useMemo(() => {
    const map = new Map<string, { pass: number; fail: number }>();
    allRuns.forEach((r) => {
      const d = new Date(r.created_at);
      if (isNaN(d.getTime())) return;
      const key = d.toISOString().slice(0, 10);
      const entry = map.get(key) ?? { pass: 0, fail: 0 };
      if (r.passed) entry.pass += 1;
      else entry.fail += 1;
      map.set(key, entry);
    });
    return [...map.entries()]
      .sort(([a], [b]) => a.localeCompare(b))
      .slice(-14)
      .map(([date, counts]) => ({ date, ...counts, total: counts.pass + counts.fail }));
  }, [allRuns]);

  const maxDailyTotal = useMemo(
    () => Math.max(1, ...dailyActivity.map((d) => d.total)),
    [dailyActivity],
  );

  // Worst scenarios (lowest pass rate with >= 2 runs)
  const worstScenarios = useMemo(() => {
    if (!stats?.by_scenario) return [];
    return [...stats.by_scenario]
      .filter((s) => s.runs >= 2)
      .map((s) => ({
        ...s,
        rate: Math.round((s.passed / s.runs) * 100),
      }))
      .sort((a, b) => a.rate - b.rate)
      .slice(0, 5);
  }, [stats]);

  // Best scenarios
  const bestScenarios = useMemo(() => {
    if (!stats?.by_scenario) return [];
    return [...stats.by_scenario]
      .filter((s) => s.runs >= 2)
      .map((s) => ({
        ...s,
        rate: Math.round((s.passed / s.runs) * 100),
      }))
      .sort((a, b) => b.rate - a.rate)
      .slice(0, 5);
  }, [stats]);

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

        <div className="flex gap-0 border border-border rounded-md overflow-hidden">
          {PERIODS.map(({ value, label }) => (
            <button
              key={value}
              onClick={() => setPeriod(value)}
              className={`font-mono text-[0.74rem] px-3 py-1.5 border-r border-border last:border-r-0 cursor-pointer transition-all ${
                period === value
                  ? "bg-accent-tint text-accent font-semibold"
                  : "bg-bg-elevated text-fg-muted hover:text-fg"
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
              className="glass-card p-4"
            >
              <Pulse className="h-3 w-20 mb-3" />
              <Pulse className="h-7 w-16" />
            </div>
          ))
        ) : (
          <>
            <StatCard
              label="Total Runs"
              value={String(stats?.total_runs ?? 0)}
              detail={`${stats?.fail_count ?? 0} failed`}
              borderColor="border-l-accent"
            />
            <StatCard
              label="Pass Rate"
              value={`${passRate}%`}
              detail={`${stats?.pass_count ?? 0} / ${stats?.total_runs ?? 0}`}
              borderColor="border-l-accent"
            />
            <StatCard
              label="Models Tested"
              value={String(distinctModels.length)}
              detail={distinctModels.join(", ")}
              borderColor="border-l-info"
            />
            <StatCard
              label="Total Cost"
              value={`$${totalCost.toFixed(2)}`}
              detail={`${formatTokens(totalTokens)} tokens`}
              borderColor="border-l-warning"
            />
            <StatCard
              label="Signal Alerts"
              value={String(totalSignals)}
              detail={signals?.runs_with_scorecard ? `${signals.runs_with_scorecard} runs scored` : "no scorecards"}
              borderColor={totalSignals > 0 ? "border-l-warning" : "border-l-fg-muted"}
            />
          </>
        )}
      </div>

      {/* Two-column layout */}
      <div className="grid grid-cols-1 lg:grid-cols-[2fr_1fr] gap-4">
        {/* Recent Runs */}
        <div className="glass-card overflow-hidden">
          <div className="flex items-center justify-between px-5 pt-4 pb-3">
            <h2 className="text-[0.95rem] font-semibold text-fg">
              Recent Runs
            </h2>
            <Link
              to="/bench/runs"
              className="text-[0.78rem] text-accent hover:text-accent-bright"
            >
              View all &rarr;
            </Link>
          </div>
          {loading ? (
            <div className="px-5 pb-4 space-y-3">
              {Array.from({ length: 5 }).map((_, i) => (
                <Pulse key={i} className="h-8 w-full" />
              ))}
            </div>
          ) : recentRuns.length === 0 ? (
            <p className="text-fg-muted text-[0.85rem] py-8 text-center">
              No runs recorded yet.
            </p>
          ) : (
            <div className="overflow-x-auto">
              <table className="w-full text-[0.82rem]">
                <thead>
                  <tr className="border-b border-border bg-bg-alt/80">
                    {["Status", "Scenario", "Model", "Duration", "Cost", "Date"].map(
                      (h) => (
                        <th
                          key={h}
                          className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2"
                        >
                          {h}
                        </th>
                      ),
                    )}
                  </tr>
                </thead>
                <tbody>
                  {recentRuns.map((run) => (
                    <tr
                      key={run.id}
                      className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors cursor-pointer"
                      onClick={() => (window.location.href = `/runs/${run.id}`)}
                    >
                      <td className="py-2.5 px-4">
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
                      <td className="py-2.5 px-4 font-medium text-fg">
                        {run.scenario_id}
                      </td>
                      <td className="py-2.5 px-4 font-mono text-fg-muted text-[0.78rem]">
                        {run.model}
                      </td>
                      <td className="py-2.5 px-4 font-mono text-fg-muted text-[0.78rem]">
                        {formatDuration(run.duration_seconds)}
                      </td>
                      <td className="py-2.5 px-4 font-mono text-fg-muted text-[0.78rem]">
                        {formatCost(run.estimated_cost_usd)}
                      </td>
                      <td className="py-2.5 px-4 text-fg-muted whitespace-nowrap text-[0.78rem]">
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
          <div className="glass-card p-5">
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
                {modelPassRates.map(({ model, rate, passed, total, cost }) => (
                  <div key={model}>
                    <div className="flex items-center justify-between mb-1">
                      <span className="font-mono text-[0.78rem] font-semibold text-fg">
                        {model}
                      </span>
                      <span className="font-mono text-[0.72rem] text-fg-muted">
                        {passed}/{total} &middot; {formatCost(cost)}
                      </span>
                    </div>
                    <div className="h-2 rounded-full bg-bg-alt/80 overflow-hidden">
                      <div
                        className={`h-full rounded-full transition-all duration-500 ${
                          rate >= 70
                            ? "bg-accent"
                            : rate >= 40
                              ? "bg-warning"
                              : "bg-danger"
                        }`}
                        style={{ width: `${rate}%` }}
                      />
                    </div>
                    <div className="text-right">
                      <span
                        className={`font-mono text-[0.72rem] font-semibold ${
                          rate >= 70
                            ? "text-accent"
                            : rate >= 40
                              ? "text-warning"
                              : "text-danger"
                        }`}
                      >
                        {rate}%
                      </span>
                    </div>
                  </div>
                ))}
              </div>
            )}
          </div>

          {/* Run Activity */}
          <div className="glass-card p-5">
            <h2 className="text-[0.95rem] font-semibold text-fg mb-4">
              Run Activity
            </h2>
            {dailyActivity.length === 0 ? (
              <p className="text-fg-muted text-[0.82rem] text-center py-4">
                No activity data.
              </p>
            ) : (
              <>
                <div className="flex items-end gap-1 h-16">
                  {dailyActivity.map((day) => {
                    const passH = (day.pass / maxDailyTotal) * 100;
                    const failH = (day.fail / maxDailyTotal) * 100;
                    return (
                      <div
                        key={day.date}
                        className="flex-1 flex flex-col justify-end gap-px"
                        title={`${day.date}: ${day.pass} pass, ${day.fail} fail`}
                        style={{ height: "100%" }}
                      >
                        {day.fail > 0 && (
                          <div
                            className="w-full rounded-t-sm bg-danger opacity-70"
                            style={{ height: `${failH}%`, minHeight: day.fail > 0 ? "2px" : 0 }}
                          />
                        )}
                        {day.pass > 0 && (
                          <div
                            className="w-full rounded-t-sm bg-accent"
                            style={{ height: `${passH}%`, minHeight: day.pass > 0 ? "2px" : 0 }}
                          />
                        )}
                      </div>
                    );
                  })}
                </div>
                <div className="flex justify-between mt-1.5">
                  <span className="text-[0.65rem] text-fg-muted">
                    {formatDateShort(dailyActivity[0].date)}
                  </span>
                  <span className="text-[0.65rem] text-fg-muted">
                    {formatDateShort(dailyActivity[dailyActivity.length - 1].date)}
                  </span>
                </div>
              </>
            )}
          </div>
        </div>
      </div>

      {/* Signal Overview */}
      {signals && signals.runs_with_scorecard > 0 && (
        <div>
          <h2 className="text-[0.95rem] font-semibold text-fg mb-3">
            Signal Overview
            <span className="text-[0.72rem] text-fg-muted font-normal ml-2">
              {signals.runs_with_scorecard} runs with scorecards
            </span>
          </h2>

          {/* Primary signals — highlighted */}
          <div className="grid grid-cols-1 sm:grid-cols-3 gap-3 mb-3">
            {PRIMARY_SIGNALS.map((id) => {
              const s = signals.signals[id];
              const count = s?.total ?? 0;
              const runCount = s?.run_count ?? 0;
              const detected = count > 0;
              return (
                <div
                  key={id}
                  className={`rounded-lg p-4 border ${
                    detected
                      ? "bg-warning-tint border-warning"
                      : "bg-bg-elevated border-border-subtle"
                  }`}
                >
                  <div className="flex items-center gap-2 mb-1">
                    <span className={`text-[0.9rem] ${detected ? "text-warning" : "text-fg-muted"}`}>
                      {detected ? "\u26A0" : "\u2713"}
                    </span>
                    <span className="font-mono text-[0.72rem] uppercase tracking-wide font-semibold text-fg">
                      {signalLabel(id)}
                    </span>
                  </div>
                  <p className="font-mono text-[1.3rem] font-bold text-fg leading-tight">
                    {count}
                  </p>
                  <p className="text-[0.7rem] text-fg-muted mt-0.5">
                    {detected ? `in ${runCount} run${runCount !== 1 ? "s" : ""}` : "clear"}
                  </p>
                </div>
              );
            })}
          </div>

          {/* Secondary signals */}
          <div className="grid grid-cols-2 sm:grid-cols-3 md:grid-cols-5 gap-3">
            {SECONDARY_SIGNALS.map((id) => {
              const s = signals.signals[id];
              const count = s?.total ?? 0;
              const runCount = s?.run_count ?? 0;
              const detected = count > 0;
              return (
                <div
                  key={id}
                  className="glass-card p-3"
                >
                  <div className="flex items-center gap-1.5 mb-1">
                    <span className={`text-[0.8rem] ${detected ? "text-warning" : "text-fg-muted"}`}>
                      {detected ? "\u26A0" : "\u2713"}
                    </span>
                    <span className="font-mono text-[0.68rem] uppercase tracking-wide font-semibold text-fg-muted">
                      {signalLabel(id)}
                    </span>
                  </div>
                  <p className="font-mono text-[1.1rem] font-bold text-fg">
                    {count}
                  </p>
                  {detected && (
                    <p className="text-[0.65rem] text-fg-muted">
                      {runCount} run{runCount !== 1 ? "s" : ""}
                    </p>
                  )}
                </div>
              );
            })}
          </div>
        </div>
      )}

      {/* Bottom row: worst + best scenarios */}
      <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
        {/* Hardest scenarios */}
        <div className="glass-card overflow-hidden">
          <div className="px-5 pt-4 pb-2">
            <h2 className="text-[0.95rem] font-semibold text-fg">
              Hardest Scenarios
            </h2>
            <p className="text-[0.72rem] text-fg-muted">Lowest pass rate (min 2 runs)</p>
          </div>
          {worstScenarios.length === 0 ? (
            <p className="text-fg-muted text-[0.82rem] text-center py-6">
              Not enough data.
            </p>
          ) : (
            <table className="w-full text-[0.82rem]">
              <tbody>
                {worstScenarios.map((s) => (
                  <tr
                    key={s.scenario_id}
                    className="border-t border-border-subtle hover:bg-accent-subtle transition-colors"
                  >
                    <td className="py-2 px-5 font-medium text-fg">
                      <Link to={`/bench/runs?scenario=${s.scenario_id}`} className="text-fg hover:text-accent">
                        {s.scenario_id}
                      </Link>
                    </td>
                    <td className="py-2 px-3 font-mono text-[0.78rem] text-fg-muted text-right">
                      {s.passed}/{s.runs}
                    </td>
                    <td className="py-2 px-5 text-right w-20">
                      <span
                        className={`font-mono text-[0.78rem] font-semibold ${
                          s.rate >= 70
                            ? "text-accent"
                            : s.rate >= 40
                              ? "text-warning"
                              : "text-danger"
                        }`}
                      >
                        {s.rate}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
        </div>

        {/* Best scenarios */}
        <div className="glass-card overflow-hidden">
          <div className="px-5 pt-4 pb-2">
            <h2 className="text-[0.95rem] font-semibold text-fg">
              Easiest Scenarios
            </h2>
            <p className="text-[0.72rem] text-fg-muted">Highest pass rate (min 2 runs)</p>
          </div>
          {bestScenarios.length === 0 ? (
            <p className="text-fg-muted text-[0.82rem] text-center py-6">
              Not enough data.
            </p>
          ) : (
            <table className="w-full text-[0.82rem]">
              <tbody>
                {bestScenarios.map((s) => (
                  <tr
                    key={s.scenario_id}
                    className="border-t border-border-subtle hover:bg-accent-subtle transition-colors"
                  >
                    <td className="py-2 px-5 font-medium text-fg">
                      <Link to={`/bench/runs?scenario=${s.scenario_id}`} className="text-fg hover:text-accent">
                        {s.scenario_id}
                      </Link>
                    </td>
                    <td className="py-2 px-3 font-mono text-[0.78rem] text-fg-muted text-right">
                      {s.passed}/{s.runs}
                    </td>
                    <td className="py-2 px-5 text-right w-20">
                      <span className="font-mono text-[0.78rem] font-semibold text-accent">
                        {s.rate}%
                      </span>
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          )}
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
      className={`glass-card p-4 border-l-[3px] ${borderColor} hover:shadow-[var(--shadow-card-lg)] hover:-translate-y-px transition-all`}
    >
      <p className="text-[0.72rem] font-semibold uppercase tracking-wide text-fg-muted mb-1">
        {label}
      </p>
      <p className="font-mono text-[1.5rem] font-bold text-fg leading-tight tracking-tight">
        {value}
      </p>
      {detail && (
        <p className="text-[0.72rem] text-fg-muted mt-1 leading-snug">
          {detail}
        </p>
      )}
    </div>
  );
}
