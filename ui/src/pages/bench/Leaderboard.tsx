import { usePageTitle } from "../../hooks/usePageTitle";
import { useEffect, useMemo, useState } from "react";
import { Link } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { evidenceModeParam } from "../../lib/catalogData.mts";
import { useEvidenceMode } from "../../hooks/useEvidenceMode";

/* ── Types ── */

interface LeaderboardEntry {
  model: string;
  scenarios: number;
  runs: number;
  pass_rate: number;
  avg_duration: number;
  avg_cost: number;
  total_cost: number;
  pass_k: number;
  pass_k_trials: number;
  sufficient_scenarios: number;
}

interface LeaderboardResponse {
  models: LeaderboardEntry[];
  evidence_mode: string;
}

type SortKey = "pass_rate" | "pass_k" | "runs" | "avg_duration" | "avg_cost" | "scenarios";

const SORT_OPTIONS: { key: SortKey; label: string; desc: boolean }[] = [
  { key: "pass_rate", label: "Pass Rate", desc: true },
  { key: "pass_k", label: "Reliability", desc: true },
  { key: "scenarios", label: "Scenarios", desc: true },
  { key: "avg_duration", label: "Duration", desc: false },
  { key: "avg_cost", label: "Avg Cost", desc: false },
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

function rateColor(rate: number): string {
  if (rate >= 70) return "text-accent";
  if (rate >= 50) return "text-warning";
  return "text-danger";
}

function rateBg(rate: number): string {
  if (rate >= 70) return "bg-accent";
  if (rate >= 50) return "bg-warning";
  if (rate > 0) return "bg-danger";
  return "bg-fg-muted";
}

function passKColor(k: number): string {
  if (k >= 70) return "text-green-400";
  if (k >= 40) return "text-accent";
  if (k >= 20) return "text-yellow-400";
  return "text-red-400";
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
  const [entries, setEntries] = useState<LeaderboardEntry[]>([]);
  const [loading, setLoading] = useState(true);
  const [sortKey, setSortKey] = useState<SortKey>("pass_rate");
  const [sortDesc, setSortDesc] = useState(true);

  useEffect(() => {
    const modeParam = evidenceModeParam("&", mode);
    request<LeaderboardResponse>(`/v1/bench/leaderboard?k=3${modeParam}`)
      .then((res) => setEntries(res.models ?? []))
      .catch(() => setEntries([]))
      .finally(() => setLoading(false));
  }, [request, mode]);

  const sorted = useMemo(() => {
    const arr = [...entries];
    arr.sort((a, b) => {
      const va = a[sortKey] ?? 0;
      const vb = b[sortKey] ?? 0;
      return sortDesc ? vb - va : va - vb;
    });
    return arr;
  }, [entries, sortKey, sortDesc]);

  const totalRuns = entries.reduce((s, e) => s + e.runs, 0);
  const totalCost = entries.reduce((s, e) => s + e.total_cost, 0);
  const totalModels = entries.length;
  const overallPassRate =
    totalRuns > 0
      ? entries.reduce((s, e) => s + (e.pass_rate / 100) * e.runs, 0) / totalRuns * 100
      : 0;

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
          value={`${overallPassRate.toFixed(1)}%`}
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
                        className={`h-full rounded-full ${rateBg(m.pass_rate)}`}
                        style={{ width: `${m.pass_rate}%` }}
                      />
                    </div>
                    <span className={`font-mono text-[0.7rem] font-semibold ${rateColor(m.pass_rate)}`}>
                      {m.runs} runs
                    </span>
                  </div>
                </td>

                {/* Pass Rate */}
                <td className="px-4 py-3 text-right">
                  <span className={`font-mono text-[0.85rem] font-bold ${rateColor(m.pass_rate)}`}>
                    {m.pass_rate.toFixed(1)}%
                  </span>
                </td>

                {/* Reliability (pass^k) */}
                <td className="px-4 py-3 text-right">
                  <span className={`font-mono text-[0.82rem] font-semibold ${passKColor(m.pass_k)}`}>
                    {m.pass_k.toFixed(1)}%
                  </span>
                  <div className="text-[0.62rem] text-fg-muted">
                    k={m.pass_k_trials}, {m.sufficient_scenarios}/{m.scenarios}
                  </div>
                </td>

                {/* Scenarios */}
                <td className="px-4 py-3 text-right font-mono text-[0.82rem] text-fg-body">
                  {m.scenarios}
                </td>

                {/* Avg Duration */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {formatDuration(m.avg_duration)}
                </td>

                {/* Avg Cost */}
                <td className="px-4 py-3 text-right font-mono text-[0.78rem] text-fg-muted">
                  {formatCost(m.avg_cost)}
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
          model={sorted[0]?.model ?? "\u2014"}
          stat={`${sorted[0]?.pass_rate.toFixed(1) ?? 0}% pass rate`}
          detail={`${sorted[0]?.runs ?? 0} runs across ${sorted[0]?.scenarios ?? 0} scenarios`}
          accent="accent"
        />
        <InsightCard
          title="Most Consistent"
          model={(() => {
            const viable = entries.filter((m) => m.pass_k > 0);
            viable.sort((a, b) => b.pass_k - a.pass_k);
            return viable[0]?.model ?? "\u2014";
          })()}
          stat={(() => {
            const viable = entries.filter((m) => m.pass_k > 0);
            viable.sort((a, b) => b.pass_k - a.pass_k);
            return viable[0] ? `${viable[0].pass_k.toFixed(1)}% pass^${viable[0].pass_k_trials}` : "\u2014";
          })()}
          detail={(() => {
            const viable = entries.filter((m) => m.pass_k > 0);
            viable.sort((a, b) => b.pass_k - a.pass_k);
            return viable[0]
              ? `${viable[0].sufficient_scenarios}/${viable[0].scenarios} scenarios with ${viable[0].pass_k_trials}+ trials`
              : "";
          })()}
          accent="info"
        />
        <InsightCard
          title="Fastest"
          model={(() => {
            const viable = entries.filter((m) => m.pass_rate >= 50);
            viable.sort((a, b) => a.avg_duration - b.avg_duration);
            return viable[0]?.model ?? "\u2014";
          })()}
          stat={(() => {
            const viable = entries.filter((m) => m.pass_rate >= 50);
            viable.sort((a, b) => a.avg_duration - b.avg_duration);
            return viable[0] ? formatDuration(viable[0].avg_duration) : "\u2014";
          })()}
          detail={(() => {
            const viable = entries.filter((m) => m.pass_rate >= 50);
            viable.sort((a, b) => a.avg_duration - b.avg_duration);
            return viable[0]
              ? `${viable[0].pass_rate.toFixed(0)}% rate across ${viable[0].scenarios} scenarios`
              : "";
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
