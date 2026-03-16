import { useCallback, useEffect, useMemo, useState } from "react";
import { useApi } from "../hooks/useApi";

/* ------------------------------------------------------------------ */
/*  Types                                                              */
/* ------------------------------------------------------------------ */

interface CellData {
  runs: number;
  passed: number;
  pass_rate: number;
  avg_cost: number;
  avg_tokens: number;
  avg_duration: number;
}

interface ModelMatrixResponse {
  models: string[];
  scenarios: string[];
  cells: Record<string, Record<string, CellData>>;
}

interface RunRecord {
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
  checks_passed: number;
  checks_total: number;
}

interface CheckDiff {
  name: string;
  type: string;
  run_a_verdict: string;
  run_b_verdict: string;
  change: string;
}

interface RunDiffResponse {
  run_a: RunRecord;
  run_b: RunRecord;
  check_diffs: CheckDiff[];
}

/* ------------------------------------------------------------------ */
/*  Helpers                                                            */
/* ------------------------------------------------------------------ */

type Tab = "matrix" | "diff";

function passRateColor(rate: number): string {
  if (rate >= 100) return "rgba(16,185,129,0.35)";
  if (rate >= 75) return "rgba(16,185,129,0.2)";
  if (rate >= 50) return "rgba(245,158,11,0.22)";
  if (rate >= 25) return "rgba(239,68,68,0.18)";
  return "rgba(239,68,68,0.3)";
}

function changeColor(change: string): string {
  const c = change.toLowerCase();
  if (c === "improved" || c === "fixed") return "var(--color-accent)";
  if (c === "regressed" || c === "broken") return "var(--color-danger)";
  return "var(--color-fg-muted)";
}

function verdictBadge(verdict: string) {
  const pass = verdict === "pass" || verdict === "passed";
  return (
    <span
      className={`inline-block px-2 py-0.5 rounded text-[0.72rem] font-semibold ${
        pass
          ? "bg-accent-tint text-accent"
          : "bg-danger-tint text-danger"
      }`}
    >
      {verdict}
    </span>
  );
}

function formatCost(v: number): string {
  return v < 0.01 ? `$${v.toFixed(4)}` : `$${v.toFixed(2)}`;
}

function formatDuration(s: number): string {
  return `${s.toFixed(1)}s`;
}

function categoryOf(scenario: string): string {
  const prefix = scenario.split("-")[0]?.toUpperCase() ?? "";
  if (prefix.startsWith("K")) return "kubectl";
  if (prefix.startsWith("H")) return "helm";
  if (prefix.startsWith("A")) return "argocd";
  if (prefix.startsWith("T")) return "terraform";
  return "other";
}

/* ------------------------------------------------------------------ */
/*  Model Matrix                                                       */
/* ------------------------------------------------------------------ */

function ModelMatrix() {
  const { request } = useApi();
  const [allModels, setAllModels] = useState<string[]>([]);
  const [activeModels, setActiveModels] = useState<string[]>([]);
  const [category, setCategory] = useState("");
  const [data, setData] = useState<ModelMatrixResponse | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Discover available models from stats endpoint, then fetch matrix
  useEffect(() => {
    setLoading(true);
    request<{ items: Array<{ model: string }> }>("/v1/bench/runs?limit=200")
      .then((res) => {
        const models = Array.from(new Set((res.items ?? []).map((r) => r.model).filter(Boolean))).sort();
        setAllModels(models);
        setActiveModels(models);
      })
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [request]);

  // Fetch matrix when active models change
  useEffect(() => {
    if (activeModels.length === 0) return;
    setLoading(true);
    const params = new URLSearchParams();
    params.set("models", activeModels.join(","));
    if (category) params.set("scenarios", category);
    request<ModelMatrixResponse>(`/v1/bench/compare/models?${params}`)
      .then(setData)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [activeModels, category, request]);

  const toggleModel = useCallback((m: string) => {
    setActiveModels((prev) =>
      prev.includes(m) ? prev.filter((x) => x !== m) : [...prev, m],
    );
  }, []);

  const categories = useMemo(() => {
    if (!data) return [];
    const set = new Set(data.scenarios.map(categoryOf));
    return Array.from(set).sort();
  }, [data]);

  const filteredScenarios = useMemo(() => {
    if (!data) return [];
    if (!category) return data.scenarios;
    return data.scenarios.filter((s) => categoryOf(s) === category);
  }, [data, category]);

  if (error) {
    return <p className="text-danger text-sm py-6">Error: {error}</p>;
  }

  return (
    <div className="space-y-4">
      {/* Controls */}
      <div className="flex flex-wrap items-center gap-3">
        {allModels.map((m) => {
          const active = activeModels.includes(m);
          return (
            <button
              key={m}
              onClick={() => toggleModel(m)}
              className={`font-mono text-[0.76rem] px-3 py-1 rounded-full border cursor-pointer transition-all ${
                active
                  ? "bg-accent-tint border-accent text-accent font-semibold"
                  : "border-border bg-bg-elevated text-fg-muted"
              }`}
            >
              {m}
            </button>
          );
        })}

        {categories.length > 0 && (
          <select
            value={category}
            onChange={(e) => setCategory(e.target.value)}
            className="ml-auto text-[0.8rem] px-2 py-1 rounded-md border border-border bg-bg-elevated text-fg cursor-pointer"
          >
            <option value="">All categories</option>
            {categories.map((c) => (
              <option key={c} value={c}>
                {c}
              </option>
            ))}
          </select>
        )}
      </div>

      {/* Table */}
      {loading ? (
        <p className="text-fg-muted text-sm py-8 text-center">Loading matrix...</p>
      ) : !data || filteredScenarios.length === 0 ? (
        <p className="text-fg-muted text-sm py-8 text-center">
          No data available. Run some benchmarks first.
        </p>
      ) : (
        <div className="overflow-x-auto rounded-[10px] border border-border-subtle">
          <table className="w-full border-collapse text-[0.82rem]">
            <thead>
              <tr className="border-b border-border-subtle">
                <th className="text-left px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold bg-bg-elevated sticky left-0 z-10">
                  Scenario
                </th>
                {activeModels.map((m) => (
                  <th
                    key={m}
                    className="text-center px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold bg-bg-elevated font-mono"
                  >
                    {m}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filteredScenarios.map((scenario) => (
                <tr
                  key={scenario}
                  className="border-b border-border-subtle last:border-b-0"
                >
                  <td className="px-3 py-2 font-mono text-fg whitespace-nowrap bg-bg sticky left-0 z-10">
                    {scenario}
                  </td>
                  {activeModels.map((model) => {
                    const cell = data.cells[scenario]?.[model];
                    if (!cell) {
                      return (
                        <td
                          key={model}
                          className="px-3 py-2 text-center text-fg-muted"
                          style={{ background: "var(--color-bg-elevated)" }}
                        >
                          --
                        </td>
                      );
                    }
                    return (
                      <td
                        key={model}
                        className="px-3 py-2 text-center transition-all"
                        style={{
                          background: passRateColor(cell.pass_rate),
                          outline: "2px solid transparent",
                        }}
                        onMouseEnter={(e) => {
                          (e.currentTarget as HTMLElement).style.outline =
                            "2px solid var(--color-accent)";
                        }}
                        onMouseLeave={(e) => {
                          (e.currentTarget as HTMLElement).style.outline =
                            "2px solid transparent";
                        }}
                      >
                        <span className="font-semibold text-fg">
                          {cell.pass_rate.toFixed(0)}%
                        </span>
                        <br />
                        <span className="text-[0.68rem] text-fg-muted">
                          {cell.runs} runs &middot; {formatCost(cell.avg_cost)}
                        </span>
                      </td>
                    );
                  })}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Run Diff                                                           */
/* ------------------------------------------------------------------ */

function RunDiff() {
  const { request } = useApi();
  const [runs, setRuns] = useState<RunRecord[]>([]);
  const [runA, setRunA] = useState("");
  const [runB, setRunB] = useState("");
  const [diff, setDiff] = useState<RunDiffResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [loadingRuns, setLoadingRuns] = useState(true);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    request<{ items: RunRecord[] }>("/v1/bench/runs?limit=100")
      .then((res) => setRuns(res.items ?? []))
      .catch((e) => setError(e.message))
      .finally(() => setLoadingRuns(false));
  }, [request]);

  const compare = useCallback(() => {
    if (!runA || !runB) return;
    setLoading(true);
    setError(null);
    request<RunDiffResponse>(`/v1/bench/compare/runs?a=${runA}&b=${runB}`)
      .then(setDiff)
      .catch((e) => setError(e.message))
      .finally(() => setLoading(false));
  }, [runA, runB, request]);

  const runLabel = (r: RunRecord) =>
    `${r.id.slice(0, 20)} -- ${r.scenario_id} -- ${r.model} -- ${r.passed ? "PASS" : "FAIL"}`;

  function statRow(label: string, a: number | string, b: number | string, higherIsBetter = true) {
    const numA = typeof a === "number" ? a : 0;
    const numB = typeof b === "number" ? b : 0;
    const improved = higherIsBetter ? numB > numA : numB < numA;
    const regressed = higherIsBetter ? numB < numA : numB > numA;

    return (
      <div className="flex justify-between py-1.5 text-[0.82rem]" key={label}>
        <span className="text-fg-muted">{label}</span>
        <div className="flex gap-6">
          <span className="font-mono font-semibold text-fg w-24 text-right">
            {String(a)}
          </span>
          <span
            className={`font-mono font-semibold w-24 text-right ${
              improved ? "text-accent" : regressed ? "text-danger" : "text-fg"
            }`}
          >
            {String(b)}
          </span>
        </div>
      </div>
    );
  }

  return (
    <div className="space-y-4">
      {/* Selectors */}
      <div className="flex flex-wrap items-end gap-3">
        <label className="flex flex-col gap-1 flex-1 min-w-[200px]">
          <span className="text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted">
            Run A
          </span>
          <select
            value={runA}
            onChange={(e) => setRunA(e.target.value)}
            className="text-[0.8rem] px-2 py-1.5 rounded-md border border-border bg-bg-elevated text-fg cursor-pointer"
          >
            <option value="">Select run...</option>
            {runs.map((r) => (
              <option key={r.id} value={r.id}>
                {runLabel(r)}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1 flex-1 min-w-[200px]">
          <span className="text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted">
            Run B
          </span>
          <select
            value={runB}
            onChange={(e) => setRunB(e.target.value)}
            className="text-[0.8rem] px-2 py-1.5 rounded-md border border-border bg-bg-elevated text-fg cursor-pointer"
          >
            <option value="">Select run...</option>
            {runs.map((r) => (
              <option key={r.id} value={r.id}>
                {runLabel(r)}
              </option>
            ))}
          </select>
        </label>

        <button
          onClick={compare}
          disabled={!runA || !runB || loading}
          className="px-4 py-1.5 text-[0.82rem] font-semibold rounded-md bg-accent text-white disabled:opacity-40 cursor-pointer disabled:cursor-not-allowed transition-opacity"
        >
          {loading ? "Comparing..." : "Compare"}
        </button>
      </div>

      {loadingRuns && (
        <p className="text-fg-muted text-sm py-4 text-center">Loading runs...</p>
      )}

      {error && <p className="text-danger text-sm">Error: {error}</p>}

      {!diff && !loading && !loadingRuns && (
        <p className="text-fg-muted text-sm py-8 text-center">
          Select two runs and click Compare.
        </p>
      )}

      {diff && (
        <>
          {/* Side-by-side stat cards */}
          <div className="grid grid-cols-1 md:grid-cols-2 gap-4">
            {[diff.run_a, diff.run_b].map((run, idx) => (
              <div
                key={idx}
                className="bg-bg-elevated border border-border-subtle rounded-[10px] p-4"
              >
                <h3 className="text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted mb-3">
                  Run {idx === 0 ? "A" : "B"} &mdash;{" "}
                  <span className="font-mono text-fg">{run.id.slice(0, 12)}</span>
                </h3>
                <div className="space-y-0">
                  {statRow("Model", diff.run_a.model, diff.run_b.model)}
                  {statRow(
                    "Duration",
                    formatDuration(diff.run_a.duration_seconds),
                    formatDuration(diff.run_b.duration_seconds),
                    false,
                  )}
                  {statRow("Turns", diff.run_a.turns, diff.run_b.turns, false)}
                  {statRow(
                    "Tokens",
                    diff.run_a.prompt_tokens + diff.run_a.completion_tokens,
                    diff.run_b.prompt_tokens + diff.run_b.completion_tokens,
                    false,
                  )}
                  {statRow(
                    "Cost",
                    formatCost(diff.run_a.estimated_cost_usd),
                    formatCost(diff.run_b.estimated_cost_usd),
                    false,
                  )}
                  {statRow(
                    "Checks",
                    `${diff.run_a.checks_passed}/${diff.run_a.checks_total}`,
                    `${diff.run_b.checks_passed}/${diff.run_b.checks_total}`,
                  )}
                </div>
              </div>
            ))}
          </div>

          {/* Check comparison table */}
          {diff.check_diffs && diff.check_diffs.length > 0 && (
            <div className="overflow-x-auto rounded-[10px] border border-border-subtle">
              <table className="w-full border-collapse text-[0.82rem]">
                <thead>
                  <tr className="border-b border-border-subtle bg-bg-elevated">
                    <th className="text-left px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">
                      Check
                    </th>
                    <th className="text-left px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">
                      Type
                    </th>
                    <th className="text-center px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">
                      Run A
                    </th>
                    <th className="text-center px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">
                      Run B
                    </th>
                    <th className="text-center px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">
                      Change
                    </th>
                  </tr>
                </thead>
                <tbody>
                  {diff.check_diffs.map((cd) => (
                    <tr
                      key={cd.name}
                      className="border-b border-border-subtle last:border-b-0"
                    >
                      <td className="px-3 py-2 font-mono text-fg">{cd.name}</td>
                      <td className="px-3 py-2 text-fg-muted">{cd.type}</td>
                      <td className="px-3 py-2 text-center">
                        {verdictBadge(cd.run_a_verdict)}
                      </td>
                      <td className="px-3 py-2 text-center">
                        {verdictBadge(cd.run_b_verdict)}
                      </td>
                      <td
                        className="px-3 py-2 text-center font-semibold text-[0.78rem]"
                        style={{ color: changeColor(cd.change) }}
                      >
                        {cd.change}
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
          )}
        </>
      )}
    </div>
  );
}

/* ------------------------------------------------------------------ */
/*  Compare (main export)                                              */
/* ------------------------------------------------------------------ */

export function Compare() {
  const [tab, setTab] = useState<Tab>("matrix");

  return (
    <div className="space-y-5">
      <h1 className="text-xl font-bold text-fg">Compare</h1>

      {/* Tabs */}
      <div className="flex gap-1 border-b border-border-subtle">
        {(
          [
            { key: "matrix", label: "Model Matrix" },
            { key: "diff", label: "Run Diff" },
          ] as const
        ).map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setTab(key)}
            className={`px-4 py-2 text-[0.84rem] font-medium border-b-2 transition-all cursor-pointer ${
              tab === key
                ? "border-accent text-accent"
                : "border-transparent text-fg-muted hover:text-fg"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {tab === "matrix" ? <ModelMatrix /> : <RunDiff />}
    </div>
  );
}
