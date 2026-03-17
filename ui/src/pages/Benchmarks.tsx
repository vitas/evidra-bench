import { useState, useEffect, useMemo } from "react";
import { useApi } from "../hooks/useApi";
import { resolveRunsLimit } from "../lib/benchmarkData.mts";

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
  created_at: string;
}

interface RunsResponse {
  items: Run[];
  total: number;
}

interface DayGroup {
  date: string;
  runs: Run[];
  passed: number;
  failed: number;
  passRate: number;
  totalCost: number;
  models: string[];
}

interface CategoryRow {
  category: string;
  total: number;
  passed: number;
  failed: number;
  passRate: number;
  avgDuration: number;
  avgCost: number;
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

function toDateKey(iso: string): string {
  // Try ISO slice first (avoids timezone issues)
  if (iso && iso.length >= 10) {
    return iso.slice(0, 10);
  }
  const d = new Date(iso);
  if (isNaN(d.getTime())) return "unknown";
  const y = d.getFullYear();
  const m = String(d.getMonth() + 1).padStart(2, "0");
  const day = String(d.getDate()).padStart(2, "0");
  return `${y}-${m}-${day}`;
}

function formatDateLabel(dateKey: string): string {
  if (!dateKey || dateKey === "unknown") return dateKey;
  const d = new Date(dateKey + "T12:00:00");
  if (isNaN(d.getTime())) return dateKey;
  const day = d.getDate();
  const mon = d.toLocaleString("en-US", { month: "short" });
  const year = d.getFullYear();
  return `${day} ${mon} ${year}`;
}

function formatCost(usd: number): string {
  if (usd < 0.001) return "$0.00";
  return `$${usd.toFixed(3)}`;
}

function formatDuration(s: number): string {
  return `${s.toFixed(1)}s`;
}

function extractCategory(scenarioId: string): string {
  const idx = scenarioId.indexOf("-");
  if (idx === -1) return scenarioId;
  return scenarioId.substring(0, idx);
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

export function Benchmarks() {
  const { request } = useApi();
  const [period, setPeriod] = useState<Period>("all");
  const [stats, setStats] = useState<Stats | null>(null);
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [modelFilter, setModelFilter] = useState<string>("all");

  useEffect(() => {
    let cancelled = false;
    setLoading(true);

    const since = periodToSince(period);
    const sinceParam = since ? `since=${encodeURIComponent(since)}` : "";

    request<Stats>(`/v1/bench/stats${sinceParam ? `?${sinceParam}` : ""}`)
      .then(async (s) => {
        if (cancelled) return;
        setStats(s);
        const runsLimit = resolveRunsLimit(s.total_runs);
        const runsPath = `/v1/bench/runs?limit=${runsLimit}${sinceParam ? `&${sinceParam}` : ""}`;
        const r = await request<RunsResponse>(runsPath);
        if (cancelled) return;
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

  /* Distinct models for the filter */
  const models = useMemo(
    () => [...new Set(runs.map((r) => r.model))].sort(),
    [runs],
  );

  /* Filtered runs */
  const filteredRuns = useMemo(
    () => modelFilter === "all" ? runs : runs.filter((r) => r.model === modelFilter),
    [runs, modelFilter],
  );

  /* Group runs by day */
  const dayGroups = useMemo(() => {
    const map = new Map<string, Run[]>();
    for (const run of filteredRuns) {
      const key = toDateKey(run.created_at);
      const arr = map.get(key) ?? [];
      arr.push(run);
      map.set(key, arr);
    }

    const groups: DayGroup[] = [];
    for (const [date, dayRuns] of map) {
      const passed = dayRuns.filter((r) => r.passed).length;
      const failed = dayRuns.length - passed;
      groups.push({
        date,
        runs: dayRuns,
        passed,
        failed,
        passRate: dayRuns.length > 0 ? (passed / dayRuns.length) * 100 : 0,
        totalCost: dayRuns.reduce((sum, r) => sum + r.estimated_cost_usd, 0),
        models: [...new Set(dayRuns.map((r) => r.model))],
      });
    }

    groups.sort((a, b) => b.date.localeCompare(a.date));
    return groups;
  }, [filteredRuns]);

  /* Category breakdown */
  const categories = useMemo(() => {
    const scenarioStats = stats?.by_scenario ?? [];
    const map = new Map<string, { total: number; passed: number; failed: number; durations: number[]; costs: number[] }>();

    for (const sc of scenarioStats) {
      const cat = extractCategory(sc.scenario_id);
      const entry = map.get(cat) ?? { total: 0, passed: 0, failed: 0, durations: [], costs: [] };
      entry.total += sc.runs;
      entry.passed += sc.passed;
      entry.failed += sc.runs - sc.passed;
      map.set(cat, entry);
    }

    // Enrich with run-level data for avg duration and cost
    for (const run of filteredRuns) {
      const cat = extractCategory(run.scenario_id);
      const entry = map.get(cat);
      if (entry) {
        entry.durations.push(run.duration_seconds);
        entry.costs.push(run.estimated_cost_usd);
      }
    }

    const rows: CategoryRow[] = [];
    for (const [category, data] of map) {
      const avgDuration = data.durations.length > 0
        ? data.durations.reduce((a, b) => a + b, 0) / data.durations.length
        : 0;
      const avgCost = data.costs.length > 0
        ? data.costs.reduce((a, b) => a + b, 0) / data.costs.length
        : 0;
      rows.push({
        category,
        total: data.total,
        passed: data.passed,
        failed: data.failed,
        passRate: data.total > 0 ? (data.passed / data.total) * 100 : 0,
        avgDuration,
        avgCost,
      });
    }

    rows.sort((a, b) => b.total - a.total);
    return rows;
  }, [stats, filteredRuns]);

  /* ── Render ── */

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
          Benchmarks
        </h1>
        <p className="text-[0.85rem] text-fg-muted mt-0.5">
          Benchmark timeline and category breakdown
        </p>
      </div>

      {/* Filters bar */}
      <div className="flex items-center gap-4 flex-wrap">
        {/* Model select */}
        <select
          value={modelFilter}
          onChange={(e) => setModelFilter(e.target.value)}
          className="font-mono text-[0.8rem] px-3 py-1.5 rounded-md border border-border bg-bg-elevated text-fg cursor-pointer"
        >
          <option value="all">All models</option>
          {models.map((m) => (
            <option key={m} value={m}>{m}</option>
          ))}
        </select>

        {/* Period toggle */}
        <div className="flex border border-border rounded-md overflow-hidden">
          {PERIODS.map(({ value, label }, i) => (
            <button
              key={value}
              onClick={() => setPeriod(value)}
              className={`font-mono text-[0.74rem] px-3 py-1.5 bg-bg-elevated cursor-pointer transition-all ${
                i < PERIODS.length - 1 ? "border-r border-border" : ""
              } ${
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

      {/* Benchmark timeline */}
      <div>
        <h2 className="text-[0.95rem] font-semibold text-fg mb-3">
          Timeline
        </h2>

        {loading ? (
          <div className="space-y-3">
            {Array.from({ length: 5 }).map((_, i) => (
              <div
                key={i}
                className="bg-bg-elevated border border-border-subtle rounded-[10px] p-4 flex items-center gap-6"
              >
                <Pulse className="h-4 w-24 shrink-0" />
                <Pulse className="h-5 flex-1" />
                <Pulse className="h-4 w-40 shrink-0" />
              </div>
            ))}
          </div>
        ) : dayGroups.length === 0 ? (
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] p-8 text-center">
            <p className="text-fg-muted text-[0.85rem]">
              No benchmark runs found for this period.
            </p>
          </div>
        ) : (
          <div className="space-y-2">
            {dayGroups.map((day) => {
              const total = day.passed + day.failed;
              const passPct = total > 0 ? (day.passed / total) * 100 : 0;
              const failPct = total > 0 ? (day.failed / total) * 100 : 0;

              return (
                <div
                  key={day.date}
                  className="bg-bg-elevated border border-border-subtle rounded-[10px] p-4 flex items-center gap-6 cursor-pointer hover:border-accent transition-colors"
                >
                  {/* Date */}
                  <span className="font-mono text-[0.78rem] text-fg-muted whitespace-nowrap shrink-0 w-28">
                    {formatDateLabel(day.date)}
                  </span>

                  {/* Stacked bar */}
                  <div className="flex-1 h-5 rounded bg-bg-alt overflow-hidden flex">
                    {passPct > 0 && (
                      <div
                        className="bg-accent h-full"
                        style={{ width: `${passPct}%` }}
                      />
                    )}
                    {failPct > 0 && (
                      <div
                        className="bg-danger h-full"
                        style={{ width: `${failPct}%` }}
                      />
                    )}
                  </div>

                  {/* Stats text */}
                  <div className="flex items-center gap-4 shrink-0">
                    <span className="font-mono text-[0.72rem] text-fg-muted whitespace-nowrap">
                      {day.passRate.toFixed(0)}% pass
                    </span>
                    <span className="font-mono text-[0.72rem] text-fg-muted whitespace-nowrap">
                      {total} runs
                    </span>
                    <span className="font-mono text-[0.72rem] text-fg-muted whitespace-nowrap">
                      {day.models.join(", ")}
                    </span>
                    <span className="font-mono text-[0.72rem] text-fg-muted whitespace-nowrap">
                      {formatCost(day.totalCost)}
                    </span>
                  </div>
                </div>
              );
            })}
          </div>
        )}
      </div>

      {/* Category breakdown */}
      <div>
        <h2 className="text-[0.95rem] font-semibold text-fg mb-3">
          Category Breakdown
        </h2>

        {loading ? (
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] p-5">
            <div className="space-y-3">
              {Array.from({ length: 4 }).map((_, i) => (
                <Pulse key={i} className="h-8 w-full" />
              ))}
            </div>
          </div>
        ) : categories.length === 0 ? (
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] p-8 text-center">
            <p className="text-fg-muted text-[0.85rem]">
              No category data available.
            </p>
          </div>
        ) : (
          <div className="bg-bg-elevated border border-border-subtle rounded-[10px] shadow-[var(--shadow-card)] overflow-hidden">
            <div className="overflow-x-auto">
              <table className="w-full text-[0.82rem]">
                <thead>
                  <tr className="border-b border-border-subtle">
                    {["Category", "Total", "Passed", "Failed", "Pass Rate", "Avg Duration", "Avg Cost"].map(
                      (h) => (
                        <th
                          key={h}
                          className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-5 py-3 first:pl-5"
                        >
                          {h}
                        </th>
                      ),
                    )}
                  </tr>
                </thead>
                <tbody>
                  {categories.map((row) => (
                    <tr
                      key={row.category}
                      className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors"
                    >
                      <td className="px-5 py-3">
                        <span className="bg-accent-subtle text-fg-muted font-medium text-[0.72rem] px-2 py-0.5 rounded">
                          {row.category}
                        </span>
                      </td>
                      <td className="px-5 py-3 font-mono text-fg">
                        {row.total}
                      </td>
                      <td className="px-5 py-3 font-mono text-accent">
                        {row.passed}
                      </td>
                      <td className="px-5 py-3 font-mono text-danger">
                        {row.failed}
                      </td>
                      <td className="px-5 py-3 font-mono text-fg">
                        {row.passRate.toFixed(1)}%
                      </td>
                      <td className="px-5 py-3 font-mono text-fg-muted">
                        {formatDuration(row.avgDuration)}
                      </td>
                      <td className="px-5 py-3 font-mono text-fg-muted">
                        {formatCost(row.avgCost)}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          </div>
        )}
      </div>
    </div>
  );
}
