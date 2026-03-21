import { useState, useEffect, useMemo } from "react";
// Data from exam-demo tenant via evidra API

const API_URL = import.meta.env.VITE_EVIDRA_API_URL || "https://api.evidra.cc";
const API_KEY = import.meta.env.VITE_EVIDRA_API_KEY || "";

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

interface LeaderboardEntry {
  model: string;
  runs: number;
  passed: number;
  failed: number;
  pass_rate: number;
  avg_duration: number;
  avg_cost: number;
  avg_turns: number;
}

type Tab = "leaderboard" | "runs";

async function fetchAPI<T>(path: string): Promise<T> {
  const res = await fetch(`${API_URL}${path}`, {
    headers: { Authorization: `Bearer ${API_KEY}` },
  });
  if (!res.ok) throw new Error(`HTTP ${res.status}`);
  return res.json();
}

function PassBadge({ passed }: { passed: boolean }) {
  return (
    <span
      className={`text-[0.68rem] font-semibold px-2 py-0.5 rounded-full ${
        passed
          ? "bg-emerald-500/15 text-emerald-400"
          : "bg-red-500/15 text-red-400"
      }`}
    >
      {passed ? "PASS" : "FAIL"}
    </span>
  );
}

function GradeBadge({ rate }: { rate: number }) {
  let grade = "Novice";
  let color = "text-fg-muted bg-bg-elevated";
  if (rate >= 0.8) {
    grade = "Expert";
    color = "text-emerald-400 bg-emerald-500/15";
  } else if (rate >= 0.65) {
    grade = "Proficient";
    color = "text-amber-400 bg-amber-500/15";
  } else if (rate >= 0.5) {
    grade = "Competent";
    color = "text-blue-400 bg-blue-500/15";
  }
  return (
    <span className={`text-[0.68rem] font-bold px-2 py-0.5 rounded-full ${color}`}>
      {grade}
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
          <code className="text-[0.75rem] text-accent bg-bg-elevated border border-border rounded px-3 py-2 block">
            docker build --build-arg VITE_EVIDRA_API_KEY=your-key ui/
          </code>
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
    fetchAPI<{ items: RunResult[] }>("/v1/bench/runs?limit=500")
      .then((data) => {
        setRuns(data.items || []);
        setError(null);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, []);

  const leaderboard = useMemo(() => {
    const byModel = new Map<string, RunResult[]>();
    for (const r of runs) {
      const list = byModel.get(r.model) || [];
      list.push(r);
      byModel.set(r.model, list);
    }

    const entries: LeaderboardEntry[] = [];
    for (const [model, modelRuns] of byModel) {
      const passed = modelRuns.filter((r) => r.passed).length;
      const totalDuration = modelRuns.reduce((sum, r) => sum + r.duration_seconds, 0);
      const totalCost = modelRuns.reduce((sum, r) => sum + (r.estimated_cost_usd || 0), 0);
      const totalTurns = modelRuns.reduce((sum, r) => sum + r.turns, 0);
      entries.push({
        model,
        runs: modelRuns.length,
        passed,
        failed: modelRuns.length - passed,
        pass_rate: passed / modelRuns.length,
        avg_duration: totalDuration / modelRuns.length,
        avg_cost: totalCost / modelRuns.length,
        avg_turns: totalTurns / modelRuns.length,
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

  return (
    <div className="max-w-6xl mx-auto px-6 py-10">
      {/* Header */}
      <div className="flex items-center justify-between mb-8">
        <div>
          <h1 className="text-2xl font-bold text-fg">Certification Results</h1>
          <p className="text-[0.85rem] text-fg-muted mt-1">
            {runs.length} runs across {leaderboard.length} models
          </p>
        </div>
      </div>

      {/* Tabs */}
      <div className="flex gap-1 mb-6 bg-bg-elevated border border-border rounded-lg p-1 w-fit">
        {(["leaderboard", "runs"] as Tab[]).map((t) => (
          <button
            key={t}
            onClick={() => setTab(t)}
            className={`px-4 py-1.5 text-[0.78rem] font-medium rounded-md transition-colors ${
              tab === t
                ? "bg-accent/15 text-accent"
                : "text-fg-muted hover:text-fg"
            }`}
          >
            {t === "leaderboard" ? "Leaderboard" : "Recent Runs"}
          </button>
        ))}
      </div>

      {loading && (
        <div className="text-center py-20 text-fg-muted">Loading results...</div>
      )}

      {error && (
        <div className="text-center py-20 text-red-400">
          Failed to load: {error}
        </div>
      )}

      {!loading && !error && tab === "leaderboard" && (
        <div className="border border-border rounded-xl overflow-hidden">
          <table className="w-full text-[0.78rem]">
            <thead>
              <tr className="bg-bg-elevated border-b border-border">
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">#</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Model</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Grade</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Pass Rate</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Runs</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Avg Duration</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Avg Cost</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Avg Turns</th>
              </tr>
            </thead>
            <tbody>
              {leaderboard.map((entry, i) => (
                <tr
                  key={entry.model}
                  className="border-b border-border-subtle hover:bg-bg-elevated/50 transition-colors"
                >
                  <td className="px-4 py-3 text-fg-muted">
                    {i === 0 ? "🥇" : i === 1 ? "🥈" : i === 2 ? "🥉" : `${i + 1}`}
                  </td>
                  <td className="px-4 py-3 font-semibold text-fg">{entry.model}</td>
                  <td className="px-4 py-3"><GradeBadge rate={entry.pass_rate} /></td>
                  <td className="px-4 py-3 text-right">
                    <span className={entry.pass_rate >= 0.8 ? "text-emerald-400" : entry.pass_rate >= 0.6 ? "text-amber-400" : "text-red-400"}>
                      {(entry.pass_rate * 100).toFixed(1)}%
                    </span>
                    <span className="text-fg-muted/60 ml-1">
                      ({entry.passed}/{entry.runs})
                    </span>
                  </td>
                  <td className="px-4 py-3 text-right text-fg-muted">{entry.runs}</td>
                  <td className="px-4 py-3 text-right text-fg-muted">{entry.avg_duration.toFixed(1)}s</td>
                  <td className="px-4 py-3 text-right text-fg-muted">${entry.avg_cost.toFixed(3)}</td>
                  <td className="px-4 py-3 text-right text-fg-muted">{entry.avg_turns.toFixed(1)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {!loading && !error && tab === "runs" && (
        <div className="border border-border rounded-xl overflow-hidden">
          <table className="w-full text-[0.78rem]">
            <thead>
              <tr className="bg-bg-elevated border-b border-border">
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Status</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Scenario</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Model</th>
                <th className="text-left px-4 py-3 font-semibold text-fg-muted">Mode</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Duration</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Turns</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Checks</th>
                <th className="text-right px-4 py-3 font-semibold text-fg-muted">Cost</th>
              </tr>
            </thead>
            <tbody>
              {recentRuns.map((run) => (
                <tr
                  key={run.id}
                  className="border-b border-border-subtle hover:bg-bg-elevated/50 transition-colors"
                >
                  <td className="px-4 py-2.5"><PassBadge passed={run.passed} /></td>
                  <td className="px-4 py-2.5 font-mono text-[0.72rem] text-fg-muted">{run.scenario_id}</td>
                  <td className="px-4 py-2.5 text-fg">{run.model}</td>
                  <td className="px-4 py-2.5 text-fg-muted">{run.evidence_mode}</td>
                  <td className="px-4 py-2.5 text-right text-fg-muted">{run.duration_seconds.toFixed(1)}s</td>
                  <td className="px-4 py-2.5 text-right text-fg-muted">{run.turns}</td>
                  <td className="px-4 py-2.5 text-right text-fg-muted">{run.checks_passed}/{run.checks_total}</td>
                  <td className="px-4 py-2.5 text-right text-fg-muted">${(run.estimated_cost_usd || 0).toFixed(3)}</td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}

      {/* Footer */}
      <div className="mt-12 text-center text-[0.72rem] text-fg-muted/60">
        Data from exam-demo tenant · Exam runs report here automatically via{" "}
        <code className="text-accent">--evidra-url</code>
      </div>
    </div>
  );
}
