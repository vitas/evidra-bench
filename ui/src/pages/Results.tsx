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
  let color = "text-gray-400 bg-gray-500/15";
  if (rate >= 0.8) {
    grade = "Expert";
    color = "text-emerald-400 bg-emerald-500/15";
  } else if (rate >= 0.85 * 0.75 + 0.15) {
    // simplified — use pass_rate thresholds
    grade = "Proficient";
    color = "text-amber-400 bg-amber-500/15";
  } else if (rate >= 0.6) {
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
      <div className="min-h-screen bg-[#0a0d0c] text-[#d1fae5] flex items-center justify-center">
        <div className="text-center max-w-md">
          <h1 className="text-xl font-bold mb-3">Results Not Configured</h1>
          <p className="text-[0.85rem] text-[#6b8f7b] mb-4">
            Set <code className="text-[#34d399]">VITE_EVIDRA_API_KEY</code> at build time to connect to the evidra API.
          </p>
          <code className="text-[0.75rem] text-[#34d399] bg-[#0c0f0e] border border-[#1e3a2c] rounded px-3 py-2 block">
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
    <div className="min-h-screen bg-[#0a0d0c] text-[#d1fae5]">
      {/* Subtle grid */}
      <div
        className="fixed inset-0 opacity-[0.03] pointer-events-none"
        style={{
          backgroundImage: `linear-gradient(#34d399 1px, transparent 1px), linear-gradient(90deg, #34d399 1px, transparent 1px)`,
          backgroundSize: "60px 60px",
        }}
      />

      <div className="relative max-w-6xl mx-auto px-6 py-12">
        {/* Header */}
        <div className="flex items-center justify-between mb-8">
          <div>
            <h1 className="text-2xl font-bold">Certification Results</h1>
            <p className="text-[0.85rem] text-[#6b8f7b] mt-1">
              {runs.length} runs across {leaderboard.length} models
            </p>
          </div>
          <a
            href="/"
            className="text-[0.78rem] text-[#6b8f7b] hover:text-[#34d399] transition-colors"
          >
            ← Back to Lab
          </a>
        </div>

        {/* Tabs */}
        <div className="flex gap-1 mb-6 bg-[#0c0f0e] border border-[#1e3a2c] rounded-lg p-1 w-fit">
          {(["leaderboard", "runs"] as Tab[]).map((t) => (
            <button
              key={t}
              onClick={() => setTab(t)}
              className={`px-4 py-1.5 text-[0.78rem] font-medium rounded-md transition-colors ${
                tab === t
                  ? "bg-[#34d399]/15 text-[#34d399]"
                  : "text-[#6b8f7b] hover:text-[#d1fae5]"
              }`}
            >
              {t === "leaderboard" ? "Leaderboard" : "Recent Runs"}
            </button>
          ))}
        </div>

        {loading && (
          <div className="text-center py-20 text-[#6b8f7b]">Loading results...</div>
        )}

        {error && (
          <div className="text-center py-20 text-red-400">
            Failed to load: {error}
          </div>
        )}

        {!loading && !error && tab === "leaderboard" && (
          <div className="border border-[#1e3a2c] rounded-xl overflow-hidden">
            <table className="w-full text-[0.78rem]">
              <thead>
                <tr className="bg-[#0c0f0e] border-b border-[#1e3a2c]">
                  <th className="text-left px-4 py-3 font-semibold text-[#6b8f7b]">#</th>
                  <th className="text-left px-4 py-3 font-semibold text-[#6b8f7b]">Model</th>
                  <th className="text-left px-4 py-3 font-semibold text-[#6b8f7b]">Grade</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Pass Rate</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Runs</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Avg Duration</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Avg Cost</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Avg Turns</th>
                </tr>
              </thead>
              <tbody>
                {leaderboard.map((entry, i) => (
                  <tr
                    key={entry.model}
                    className="border-b border-[#1e3a2c]/50 hover:bg-[#111916] transition-colors"
                  >
                    <td className="px-4 py-3 text-[#6b8f7b]">
                      {i === 0 ? "🥇" : i === 1 ? "🥈" : i === 2 ? "🥉" : `${i + 1}`}
                    </td>
                    <td className="px-4 py-3 font-semibold text-[#d1fae5]">{entry.model}</td>
                    <td className="px-4 py-3"><GradeBadge rate={entry.pass_rate} /></td>
                    <td className="px-4 py-3 text-right">
                      <span className={entry.pass_rate >= 0.8 ? "text-emerald-400" : entry.pass_rate >= 0.6 ? "text-amber-400" : "text-red-400"}>
                        {(entry.pass_rate * 100).toFixed(1)}%
                      </span>
                      <span className="text-[#4a6b5a] ml-1">
                        ({entry.passed}/{entry.runs})
                      </span>
                    </td>
                    <td className="px-4 py-3 text-right text-[#a7cdb8]">{entry.runs}</td>
                    <td className="px-4 py-3 text-right text-[#a7cdb8]">{entry.avg_duration.toFixed(1)}s</td>
                    <td className="px-4 py-3 text-right text-[#a7cdb8]">${entry.avg_cost.toFixed(3)}</td>
                    <td className="px-4 py-3 text-right text-[#a7cdb8]">{entry.avg_turns.toFixed(1)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {!loading && !error && tab === "runs" && (
          <div className="border border-[#1e3a2c] rounded-xl overflow-hidden">
            <table className="w-full text-[0.78rem]">
              <thead>
                <tr className="bg-[#0c0f0e] border-b border-[#1e3a2c]">
                  <th className="text-left px-4 py-3 font-semibold text-[#6b8f7b]">Status</th>
                  <th className="text-left px-4 py-3 font-semibold text-[#6b8f7b]">Scenario</th>
                  <th className="text-left px-4 py-3 font-semibold text-[#6b8f7b]">Model</th>
                  <th className="text-left px-4 py-3 font-semibold text-[#6b8f7b]">Mode</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Duration</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Turns</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Checks</th>
                  <th className="text-right px-4 py-3 font-semibold text-[#6b8f7b]">Cost</th>
                </tr>
              </thead>
              <tbody>
                {recentRuns.map((run) => (
                  <tr
                    key={run.id}
                    className="border-b border-[#1e3a2c]/50 hover:bg-[#111916] transition-colors"
                  >
                    <td className="px-4 py-2.5"><PassBadge passed={run.passed} /></td>
                    <td className="px-4 py-2.5 font-mono text-[0.72rem] text-[#a7cdb8]">{run.scenario_id}</td>
                    <td className="px-4 py-2.5 text-[#d1fae5]">{run.model}</td>
                    <td className="px-4 py-2.5 text-[#6b8f7b]">{run.evidence_mode}</td>
                    <td className="px-4 py-2.5 text-right text-[#a7cdb8]">{run.duration_seconds.toFixed(1)}s</td>
                    <td className="px-4 py-2.5 text-right text-[#a7cdb8]">{run.turns}</td>
                    <td className="px-4 py-2.5 text-right text-[#a7cdb8]">{run.checks_passed}/{run.checks_total}</td>
                    <td className="px-4 py-2.5 text-right text-[#a7cdb8]">${(run.estimated_cost_usd || 0).toFixed(3)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}

        {/* Footer */}
        <div className="mt-12 text-center text-[0.72rem] text-[#4a6b5a]">
          Data from exam-demo tenant · Exam runs report here automatically via{" "}
          <code className="text-[#34d399]">--evidra-url</code>
        </div>
      </div>
    </div>
  );
}
