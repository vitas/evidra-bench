import { usePageTitle } from "../../hooks/usePageTitle";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { evidenceModeParam } from "../../lib/catalogData.mts";
import { useEvidenceMode } from "../../hooks/useEvidenceMode";

/* ── Types ── */

interface Run {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  passed: boolean;
  duration_seconds: number;
  turns: number;
  prompt_tokens: number;
  completion_tokens: number;
  estimated_cost_usd: number;
  checks_json: string;
}

interface RunsResponse {
  items: Run[];
  total: number;
}

interface CheckEntry {
  name: string;
  type: string;
  verdict: string;
}

interface ChecksPayload {
  checks: CheckEntry[];
}

function parseProtocolCompliance(checksJson: string): { hasProtocol: boolean; infraPass: boolean; protocolPass: boolean } {
  if (!checksJson) return { hasProtocol: false, infraPass: false, protocolPass: false };
  try {
    const parsed: ChecksPayload = JSON.parse(checksJson);
    const checks = parsed.checks ?? [];
    const infra = checks.filter((c) => c.type !== "evidra-protocol");
    const protocol = checks.filter((c) => c.type === "evidra-protocol");
    return {
      hasProtocol: protocol.length > 0,
      infraPass: infra.length === 0 || infra.every((c) => c.verdict === "pass"),
      protocolPass: protocol.length > 0 && protocol.every((c) => c.verdict === "pass"),
    };
  } catch {
    return { hasProtocol: false, infraPass: false, protocolPass: false };
  }
}

interface ModelStats {
  model: string;
  runs: number;
  passed: number;
  failed: number;
  rate: number;
  infraRate: number;
  avgTurns: number;
  scenarios: number;
  avgDuration: number;
  totalCost: number;
  costPerRun: number;
  costPerPass: number;
  avgTokens: number;
  totalTokens: number;
}

type SortKey = keyof Pick<
  ModelStats,
  "rate" | "infraRate" | "avgTurns" | "runs" | "avgDuration" | "costPerRun" | "costPerPass" | "avgTokens" | "scenarios"
>;

const SORT_OPTIONS: { key: SortKey; label: string; desc: boolean }[] = [
  { key: "rate", label: "Overall", desc: true },
  { key: "infraRate", label: "Infra Fix", desc: true },
  { key: "avgTurns", label: "Avg Turns", desc: false },
  { key: "costPerPass", label: "Cost/Pass", desc: false },
  { key: "avgDuration", label: "Duration", desc: false },
  { key: "runs", label: "Runs", desc: true },
];

function formatDuration(s: number): string {
  if (s < 60) return `${s.toFixed(1)}s`;
  return `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`;
}

function formatCost(usd: number): string {
  if (usd === 0) return "$0.00";
  if (usd < 0.001) return `$${usd.toFixed(4)}`;
  if (usd < 0.01) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(2)}`;
}

// formatTokens removed — column replaced by protocol compliance

function rateColor(rate: number): string {
  if (rate >= 90) return "text-accent";
  if (rate >= 70) return "text-accent";
  if (rate >= 50) return "text-warning";
  return "text-danger";
}

function rateBg(rate: number): string {
  if (rate >= 90) return "bg-accent";
  if (rate >= 70) return "bg-accent";
  if (rate >= 50) return "bg-warning";
  if (rate > 0) return "bg-danger";
  return "bg-fg-muted";
}

function medalEmoji(rank: number): string {
  if (rank === 0) return "\uD83E\uDD47";
  if (rank === 1) return "\uD83E\uDD48";
  if (rank === 2) return "\uD83E\uDD49";
  return "";
}

/* ── Component ── */

export function Leaderboard() {
  usePageTitle("Model Leaderboard");
  const { request } = useApi();
  const { mode } = useEvidenceMode();
  const [runs, setRuns] = useState<Run[]>([]);
  const [loading, setLoading] = useState(true);
  const [sortKey, setSortKey] = useState<SortKey>("rate");
  const [sortDesc, setSortDesc] = useState(true);

  useEffect(() => {
    request<RunsResponse>(`/v1/bench/runs?limit=1000${evidenceModeParam("&", mode)}`)
      .then((res) => setRuns(res.items ?? []))
      .catch(() => setRuns([]))
      .finally(() => setLoading(false));
  }, [request, mode]);

  const models = useMemo(() => {
    const map = new Map<
      string,
      {
        runs: number;
        passed: number;
        infraPassed: number;
        turns: number;
        scenarios: Set<string>;
        durations: number[];
        cost: number;
        tokens: number;
      }
    >();

    for (const r of runs) {
      if (!r.model) continue;
      const entry = map.get(r.model) ?? {
        runs: 0,
        passed: 0,
        infraPassed: 0,
        turns: 0,
        scenarios: new Set(),
        durations: [],
        cost: 0,
        tokens: 0,
      };
      entry.runs += 1;
      if (r.passed) entry.passed += 1;

      const compliance = parseProtocolCompliance(r.checks_json);
      if (compliance.infraPass) entry.infraPassed += 1;
      entry.turns += r.turns || 0;

      entry.scenarios.add(r.scenario_id);
      entry.durations.push(r.duration_seconds);
      entry.cost += r.estimated_cost_usd || 0;
      entry.tokens += (r.prompt_tokens || 0) + (r.completion_tokens || 0);
      map.set(r.model, entry);
    }

    const stats: ModelStats[] = [];
    for (const [model, e] of map) {
      const rate = e.runs > 0 ? (e.passed / e.runs) * 100 : 0;
      const infraRate = e.runs > 0 ? (e.infraPassed / e.runs) * 100 : 0;
      const avgTurns = e.runs > 0 ? e.turns / e.runs : 0;
      const avgDuration =
        e.durations.length > 0
          ? e.durations.reduce((a, b) => a + b, 0) / e.durations.length
          : 0;
      stats.push({
        model,
        runs: e.runs,
        passed: e.passed,
        failed: e.runs - e.passed,
        rate,
        infraRate,
        avgTurns,
        scenarios: e.scenarios.size,
        avgDuration,
        totalCost: e.cost,
        costPerRun: e.runs > 0 ? e.cost / e.runs : 0,
        costPerPass: e.passed > 0 ? e.cost / e.passed : Infinity,
        avgTokens: e.runs > 0 ? e.tokens / e.runs : 0,
        totalTokens: e.tokens,
      });
    }

    return stats;
  }, [runs]);

  const sorted = useMemo(() => {
    const arr = [...models];
    arr.sort((a, b) => {
      const va = a[sortKey] ?? -1;
      const vb = b[sortKey] ?? -1;
      if (va === Infinity && vb === Infinity) return 0;
      if (va === Infinity || va === -1) return 1;
      if (vb === Infinity || vb === -1) return -1;
      return sortDesc ? (vb as number) - (va as number) : (va as number) - (vb as number);
    });
    return arr;
  }, [models, sortKey, sortDesc]);

  const totalRuns = runs.length;
  const totalPassed = runs.filter((r) => r.passed).length;
  const totalCost = runs.reduce((s, r) => s + (r.estimated_cost_usd || 0), 0);
  const totalModels = models.length;

  const handleSort = (key: SortKey) => {
    if (sortKey === key) {
      setSortDesc(!sortDesc);
    } else {
      const opt = SORT_OPTIONS.find((o) => o.key === key);
      setSortKey(key);
      setSortDesc(opt?.desc ?? true);
    }
  };

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Loading leaderboard...
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Header */}
      <div>
        <h1 className="text-[1.4rem] font-bold text-fg tracking-tight">
          Model Leaderboard
        </h1>
        <p className="text-[0.85rem] text-fg-muted mt-0.5">
          {totalModels} models ranked across {totalRuns} benchmark runs
        </p>
      </div>

      {/* Summary cards */}
      <div className="grid grid-cols-2 sm:grid-cols-4 gap-3">
        <MiniCard label="Models" value={String(totalModels)} />
        <MiniCard label="Total Runs" value={String(totalRuns)} />
        <MiniCard
          label="Overall Pass Rate"
          value={`${totalRuns > 0 ? ((totalPassed / totalRuns) * 100).toFixed(1) : 0}%`}
        />
        <MiniCard label="Est. Cost*" value={`$${totalCost.toFixed(2)}`} />
      </div>

      {/* Leaderboard table */}
      <div className="glass-card overflow-hidden">
        <table className="w-full text-[0.82rem]">
          <thead>
            <tr className="border-b border-border bg-bg-alt/80">
              <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5 w-10">
                #
              </th>
              <th className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5">
                Model
              </th>
              {SORT_OPTIONS.map((opt) => (
                <th
                  key={opt.key}
                  className="text-right text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2.5 cursor-pointer hover:text-accent transition-colors whitespace-nowrap"
                  onClick={() => handleSort(opt.key)}
                >
                  {opt.label}{" "}
                  {sortKey === opt.key ? (
                    <span className="text-accent">{sortDesc ? "\u2193" : "\u2191"}</span>
                  ) : (
                    <span className="opacity-30">{"\u2195"}</span>
                  )}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {sorted.map((m, i) => (
              <tr
                key={m.model}
                className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors"
              >
                {/* Rank */}
                <td className="px-4 py-3 text-center">
                  {i < 3 ? (
                    <span className="text-[1.1rem]">{medalEmoji(i)}</span>
                  ) : (
                    <span className="font-mono text-fg-muted text-[0.78rem]">{i + 1}</span>
                  )}
                </td>

                {/* Model name */}
                <td className="px-4 py-3">
                  <Link
                    to={`/bench/runs?model=${m.model}`}
                    className="font-mono text-[0.85rem] font-semibold text-fg hover:text-accent transition-colors"
                    style={{ textDecoration: "none" }}
                  >
                    {m.model}
                  </Link>
                  <div className="flex items-center gap-2 mt-1">
                    <div className="flex-1 max-w-[120px] h-1.5 rounded-full bg-bg-alt/80 overflow-hidden">
                      <div
                        className={`h-full rounded-full ${rateBg(m.rate)}`}
                        style={{ width: `${m.rate}%` }}
                      />
                    </div>
                    <span className={`font-mono text-[0.7rem] font-semibold ${rateColor(m.rate)}`}>
                      {m.passed}/{m.runs}
                    </span>
                  </div>
                </td>

                {/* Overall Rate */}
                <td className="px-4 py-3 text-right">
                  <span className={`font-mono text-[0.85rem] font-bold ${rateColor(m.rate)}`}>
                    {m.rate.toFixed(1)}%
                  </span>
                </td>

                {/* Infra Fix Rate */}
                <td className="px-4 py-3 text-right">
                  <span className={`font-mono text-[0.82rem] font-semibold ${rateColor(m.infraRate)}`}>
                    {m.infraRate.toFixed(0)}%
                  </span>
                </td>

                {/* Avg Turns */}
                <td className="px-4 py-3 text-right font-mono text-[0.82rem] text-fg-body">
                  {m.avgTurns > 0 ? m.avgTurns.toFixed(1) : "\u2014"}
                </td>

                {/* Cost per Pass */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {m.costPerPass === Infinity ? "\u2014" : formatCost(m.costPerPass)}
                </td>

                {/* Avg Duration */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {formatDuration(m.avgDuration)}
                </td>

                {/* Total Runs */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {m.runs}
                </td>
              </tr>
            ))}
          </tbody>
        </table>
      </div>

      {/* Key insights */}
      <div className="grid grid-cols-1 md:grid-cols-3 gap-4">
        <InsightCard
          title="Most Reliable"
          model={sorted[0]?.model ?? "—"}
          stat={`${sorted[0]?.rate.toFixed(1) ?? 0}% pass rate`}
          detail={`${sorted[0]?.passed ?? 0}/${sorted[0]?.runs ?? 0} runs passed`}
          accent="accent"
        />
        <InsightCard
          title="Best Value"
          model={(() => {
            const viable = models.filter((m) => m.rate >= 50 && m.costPerRun > 0);
            viable.sort((a, b) => a.costPerPass - b.costPerPass);
            return viable[0]?.model ?? "—";
          })()}
          stat={(() => {
            const viable = models.filter((m) => m.rate >= 50 && m.costPerRun > 0);
            viable.sort((a, b) => a.costPerPass - b.costPerPass);
            return viable[0] ? `${formatCost(viable[0].costPerPass)} per pass` : "—";
          })()}
          detail={(() => {
            const viable = models.filter((m) => m.rate >= 50 && m.costPerRun > 0);
            viable.sort((a, b) => a.costPerPass - b.costPerPass);
            return viable[0] ? `${viable[0].rate.toFixed(0)}% rate, ${formatCost(viable[0].costPerRun)}/run` : "";
          })()}
          accent="info"
        />
        <InsightCard
          title="Fastest"
          model={(() => {
            const viable = models.filter((m) => m.rate >= 50);
            viable.sort((a, b) => a.avgDuration - b.avgDuration);
            return viable[0]?.model ?? "—";
          })()}
          stat={(() => {
            const viable = models.filter((m) => m.rate >= 50);
            viable.sort((a, b) => a.avgDuration - b.avgDuration);
            return viable[0] ? formatDuration(viable[0].avgDuration) : "—";
          })()}
          detail={(() => {
            const viable = models.filter((m) => m.rate >= 50);
            viable.sort((a, b) => a.avgDuration - b.avgDuration);
            return viable[0] ? `${viable[0].rate.toFixed(0)}% rate across ${viable[0].scenarios} scenarios` : "";
          })()}
          accent="warning"
        />
      </div>

      {/* Cost disclaimer */}
      <p className="text-[0.7rem] text-fg-muted mt-4">
        * Cost estimates are approximate — based on listed API prices and reported token counts.
        Actual billing may differ due to prompt caching, proxy overhead, and incomplete token reporting.
        Do not use for budgeting.
      </p>
    </div>
  );
}

/* ── Sub-components ── */

function MiniCard({ label, value }: { label: string; value: string }) {
  return (
    <div className="glass-card p-4">
      <p className="text-[0.68rem] font-semibold uppercase tracking-widest text-fg-muted">
        {label}
      </p>
      <p className="font-mono text-[1.15rem] font-bold text-fg mt-1">{value}</p>
    </div>
  );
}

function InsightCard({
  title,
  model,
  stat,
  detail,
  accent,
}: {
  title: string;
  model: string;
  stat: string;
  detail: string;
  accent: string;
}) {
  return (
    <div className={`glass-card p-4 border-l-[3px] border-l-${accent}`}>
      <p className="text-[0.72rem] font-semibold uppercase tracking-widest text-fg-muted mb-1">
        {title}
      </p>
      <p className="font-mono text-[0.92rem] font-bold text-fg">{model}</p>
      <p className={`font-mono text-[0.78rem] font-semibold text-${accent} mt-0.5`}>{stat}</p>
      {detail && <p className="text-[0.7rem] text-fg-muted mt-0.5">{detail}</p>}
    </div>
  );
}
