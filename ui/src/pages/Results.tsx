import { useState, useEffect, useMemo } from "react";
import { SCENARIOS, TRACK_LABELS } from "../data/catalog";

const API_URL = import.meta.env.VITE_EVIDRA_API_URL || "https://api.evidra.cc";
const API_KEY = import.meta.env.VITE_EVIDRA_API_KEY || "";

// Build lookup from scenario ID → track/level from our catalog
const SCENARIO_META = new Map(SCENARIOS.map((s) => [s.id, { track: s.track, level: s.level }]));

interface RunResult {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  evidence_mode: string;
  passed: boolean;
  duration_seconds: number;
  turns: number;
  estimated_cost_usd: number;
  checks_passed: number;
  checks_total: number;
  created_at: string;
}

interface TrackGrade {
  track: string;
  grade: string;
  gradeColor: string;
  total: number;
  passed: number;
  byLevel: Record<string, { total: number; passed: number }>;
}

interface ModelEntry {
  model: string;
  runs: number;
  passed: number;
  pass_rate: number;
  avg_duration: number;
  avg_cost: number;
  tracks: TrackGrade[];
  overallGrade: string;
  overallGradeColor: string;
}

type Tab = "leaderboard" | "runs";

async function fetchAPI<T>(path: string): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    headers: { Authorization: `Bearer ${API_KEY}` },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

function calculateGrade(byLevel: Record<string, { total: number; passed: number }>): { grade: string; color: string } {
  const cumulative = (through: string[]) => {
    let total = 0, passed = 0;
    for (const l of through) {
      if (byLevel[l]) { total += byLevel[l].total; passed += byLevel[l].passed; }
    }
    return total > 0 ? passed / total : 0;
  };

  if (cumulative(["L1", "L2", "L3", "L4"]) >= 0.80 && byLevel["L4"]?.total) return { grade: "Expert", color: "text-emerald-400 bg-emerald-500/15" };
  if (cumulative(["L1", "L2", "L3"]) >= 0.85 && byLevel["L3"]?.total) return { grade: "Proficient", color: "text-amber-400 bg-amber-500/15" };
  if (cumulative(["L1", "L2"]) >= 0.90) return { grade: "Competent", color: "text-blue-400 bg-blue-500/15" };
  if (byLevel["L1"]?.passed) return { grade: "Novice", color: "text-fg-muted bg-bg-elevated" };
  return { grade: "—", color: "text-fg-muted bg-bg-elevated" };
}

function PassBadge({ passed }: { passed: boolean }) {
  return (
    <span className={`text-[0.68rem] font-semibold px-2 py-0.5 rounded-full ${
      passed ? "bg-emerald-500/15 text-emerald-400" : "bg-red-500/15 text-red-400"
    }`}>
      {passed ? "PASS" : "FAIL"}
    </span>
  );
}

export function Results() {
  if (!API_KEY) {
    return (
      <div className="flex items-center justify-center py-20">
        <div className="text-center max-w-md">
          <h1 className="text-xl font-bold text-fg mb-3">Results Not Configured</h1>
          <p className="text-[0.85rem] text-fg-muted mb-4">
            Set <code className="text-accent">VITE_EVIDRA_API_KEY</code> at build time to connect to the evidra API.
          </p>
        </div>
      </div>
    );
  }

  const [tab, setTab] = useState<Tab>("leaderboard");
  const [runs, setRuns] = useState<RunResult[]>([]);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoading(true);
    fetchAPI<{ items: RunResult[] }>("/v1/bench/runs?limit=2000")
      .then((data) => { setRuns(data.items || []); setError(null); })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  const leaderboard = useMemo((): ModelEntry[] => {
    const byModel = new Map<string, RunResult[]>();
    for (const r of runs) {
      const list = byModel.get(r.model) || [];
      list.push(r);
      byModel.set(r.model, list);
    }

    const entries: ModelEntry[] = [];
    for (const [model, modelRuns] of byModel) {
      // Group by track → level
      const trackMap = new Map<string, Record<string, { total: number; passed: number }>>();
      for (const r of modelRuns) {
        const meta = SCENARIO_META.get(r.scenario_id);
        const track = meta?.track || "unknown";
        const level = meta?.level || "L1";
        if (!trackMap.has(track)) trackMap.set(track, {});
        const levels = trackMap.get(track)!;
        if (!levels[level]) levels[level] = { total: 0, passed: 0 };
        levels[level].total++;
        if (r.passed) levels[level].passed++;
      }

      // Calculate grade per track
      const tracks: TrackGrade[] = [];
      for (const [track, byLevel] of trackMap) {
        if (track === "unknown") continue;
        const { grade, color } = calculateGrade(byLevel);
        let total = 0, passed = 0;
        for (const l of Object.values(byLevel)) { total += l.total; passed += l.passed; }
        tracks.push({ track, grade, gradeColor: color, total, passed, byLevel });
      }
      tracks.sort((a, b) => a.track.localeCompare(b.track));

      // Overall grade: use all levels across all tracks
      const allLevels: Record<string, { total: number; passed: number }> = {};
      for (const t of tracks) {
        for (const [level, counts] of Object.entries(t.byLevel)) {
          if (!allLevels[level]) allLevels[level] = { total: 0, passed: 0 };
          allLevels[level].total += counts.total;
          allLevels[level].passed += counts.passed;
        }
      }
      const { grade: overallGrade, color: overallGradeColor } = calculateGrade(allLevels);

      const passed = modelRuns.filter((r) => r.passed).length;
      const totalDuration = modelRuns.reduce((sum, r) => sum + r.duration_seconds, 0);
      const totalCost = modelRuns.reduce((sum, r) => sum + (r.estimated_cost_usd || 0), 0);

      entries.push({
        model,
        runs: modelRuns.length,
        passed,
        pass_rate: passed / modelRuns.length,
        avg_duration: totalDuration / modelRuns.length,
        avg_cost: totalCost / modelRuns.length,
        tracks,
        overallGrade,
        overallGradeColor,
      });
    }

    entries.sort((a, b) => b.pass_rate - a.pass_rate || a.avg_duration - b.avg_duration);
    return entries;
  }, [runs]);

  const recentRuns = useMemo(() => {
    return [...runs].sort((a, b) =>
      (b.created_at || "").localeCompare(a.created_at || "")
    ).slice(0, 50);
  }, [runs]);

  const [expandedModel, setExpandedModel] = useState<string | null>(null);

  return (
    <div className="max-w-6xl mx-auto px-6 py-10">
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-fg">Certification Results</h1>
          <p className="text-[0.85rem] text-fg-muted mt-1">
            {runs.length} runs across {leaderboard.length} models
          </p>
        </div>
      </div>

      <div className="flex gap-1 mb-6 bg-bg-elevated border border-border rounded-lg p-1 w-fit">
        {(["leaderboard", "runs"] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-1.5 text-[0.78rem] font-medium rounded-md transition-colors ${
              tab === t ? "bg-accent/15 text-accent" : "text-fg-muted hover:text-fg"
            }`}
          >
            {t === "leaderboard" ? "Leaderboard" : "Recent Runs"}
          </button>
        ))}
      </div>

      {loading && <div className="text-center py-20 text-fg-muted">Loading results...</div>}
      {error && <div className="text-center py-20 text-red-400">Failed to load: {error}</div>}

      {!loading && !error && tab === "leaderboard" && (
        <div className="space-y-2">
          {leaderboard.map((entry, i) => (
            <div key={entry.model} className="border border-border rounded-xl overflow-hidden">
              {/* Model row */}
              <button
                onClick={() => setExpandedModel(expandedModel === entry.model ? null : entry.model)}
                className="w-full flex items-center gap-4 px-4 py-3 hover:bg-bg-elevated/50 transition-colors text-left"
              >
                <span className="text-fg-muted w-8 shrink-0">
                  {i === 0 ? "🥇" : i === 1 ? "🥈" : i === 2 ? "🥉" : `${i + 1}`}
                </span>
                <span className="font-semibold text-fg text-[0.85rem] w-40 shrink-0">{entry.model}</span>
                <span className={`text-[0.68rem] font-bold px-2 py-0.5 rounded-full shrink-0 ${entry.overallGradeColor}`}>
                  {entry.overallGrade}
                </span>
                <span className="text-[0.78rem] text-right flex-1">
                  <span className={entry.pass_rate >= 0.8 ? "text-emerald-400" : entry.pass_rate >= 0.6 ? "text-amber-400" : "text-red-400"}>
                    {(entry.pass_rate * 100).toFixed(1)}%
                  </span>
                  <span className="text-fg-muted/60 ml-1">({entry.passed}/{entry.runs})</span>
                </span>
                <span className="text-[0.75rem] text-fg-muted w-20 text-right shrink-0">{entry.avg_duration.toFixed(1)}s</span>
                <span className="text-[0.75rem] text-fg-muted w-20 text-right shrink-0">${entry.avg_cost.toFixed(3)}</span>
                <svg
                  className={`w-3.5 h-3.5 text-fg-muted transition-transform shrink-0 ${expandedModel === entry.model ? "rotate-180" : ""}`}
                  viewBox="0 0 24 24" fill="none" stroke="currentColor" strokeWidth={2}
                >
                  <path d="M6 9l6 6 6-6" />
                </svg>
              </button>

              {/* Track breakdown (expanded) */}
              {expandedModel === entry.model && (
                <div className="border-t border-border-subtle bg-bg-elevated/30 px-4 py-3">
                  <div className="text-[0.7rem] font-semibold text-fg-muted uppercase tracking-wider mb-2">
                    Grade by Track
                  </div>
                  <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
                    {entry.tracks.map((t) => (
                      <div key={t.track} className="border border-border-subtle rounded-lg p-3">
                        <div className="text-[0.72rem] font-medium text-fg mb-1">
                          {TRACK_LABELS[t.track] || t.track}
                        </div>
                        <div className="flex items-center gap-2 mb-2">
                          <span className={`text-[0.65rem] font-bold px-1.5 py-0.5 rounded ${t.gradeColor}`}>
                            {t.grade}
                          </span>
                          <span className="text-[0.68rem] text-fg-muted">
                            {t.passed}/{t.total}
                          </span>
                        </div>
                        <div className="flex gap-1">
                          {(["L1", "L2", "L3", "L4"] as const).map((level) => {
                            const lv = t.byLevel[level];
                            if (!lv) return null;
                            const allPassed = lv.passed === lv.total;
                            return (
                              <span
                                key={level}
                                className={`text-[0.58rem] px-1 py-0.5 rounded ${
                                  allPassed
                                    ? "bg-emerald-500/15 text-emerald-400"
                                    : lv.passed > 0
                                      ? "bg-amber-500/15 text-amber-400"
                                      : "bg-red-500/15 text-red-400"
                                }`}
                                title={`${level}: ${lv.passed}/${lv.total}`}
                              >
                                {level} {lv.passed}/{lv.total}
                              </span>
                            );
                          })}
                        </div>
                      </div>
                    ))}
                  </div>
                </div>
              )}
            </div>
          ))}
        </div>
      )}

      {!loading && !error && tab === "runs" && (
        <div className="border border-border rounded-xl overflow-hidden">
          <table className="w-full text-[0.78rem]">
            <thead>
              <tr className="bg-bg-elevated border-b border-border">
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Status</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Scenario</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Track</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Model</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Duration</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Turns</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Cost</th>
              </tr>
            </thead>
            <tbody>
              {recentRuns.map((run) => {
                const meta = SCENARIO_META.get(run.scenario_id);
                return (
                  <tr
                    key={run.id}
                    className="border-b border-border-subtle hover:bg-bg-elevated/50 transition-colors"
                  >
                    <td className="px-4 py-2.5"><PassBadge passed={run.passed} /></td>
                    <td className="px-4 py-2.5 font-mono text-[0.72rem] text-fg-muted">{run.scenario_id}</td>
                    <td className="px-4 py-2.5 text-[0.72rem] text-fg-muted">
                      {meta ? `${TRACK_LABELS[meta.track] || meta.track} · ${meta.level}` : "—"}
                    </td>
                    <td className="px-4 py-2.5 text-fg">{run.model}</td>
                    <td className="px-4 py-2.5 text-right text-fg-muted">{run.duration_seconds.toFixed(1)}s</td>
                    <td className="px-4 py-2.5 text-right text-fg-muted">{run.turns}</td>
                    <td className="px-4 py-2.5 text-right text-fg-muted">${(run.estimated_cost_usd || 0).toFixed(3)}</td>
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      <div className="mt-12 text-center text-[0.72rem] text-fg-muted/60">
        Results update automatically after each certification run
      </div>
    </div>
  );
}
