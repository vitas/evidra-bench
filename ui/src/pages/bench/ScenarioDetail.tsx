import { useEffect, useMemo, useState } from "react";
import { useParams, Link } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { buildBenchApiURL } from "../../lib/apiBase.mts";
import { usePageTitle } from "../../hooks/usePageTitle";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

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
  checks_passed: number;
  checks_total: number;
  created_at: string;
}

interface RunsResponse {
  runs: Run[];
  total: number;
}

interface Scenario {
  id: string;
  title: string;
  description?: string;
  autopsy_description?: string;
  category: string;
  tags: string[];
  chaos: boolean;
}

interface ScenariosResponse {
  scenarios: Scenario[];
}

interface ModelGroup {
  model: string;
  runs: Run[];
  passed: number;
  failed: number;
  rate: number;
  avgDuration: number;
  avgCost: number;
  avgTokens: number;
}

/* ── Helpers ── */

function formatDuration(s: number): string {
  if (s < 60) return `${s.toFixed(1)}s`;
  return `${Math.floor(s / 60)}m ${Math.round(s % 60)}s`;
}

function formatCost(usd: number): string {
  if (usd === 0) return "$0.00";
  if (usd < 0.01) return `$${usd.toFixed(3)}`;
  return `$${usd.toFixed(2)}`;
}

function formatDate(iso: string): string {
  const d = new Date(iso);
  if (isNaN(d.getTime())) return iso;
  const day = d.getDate();
  const mon = d.toLocaleString("en-US", { month: "short" });
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${day} ${mon} ${hh}:${mm}`;
}

function rateColor(rate: number): string {
  if (rate >= 80) return "text-accent";
  if (rate >= 50) return "text-warning";
  return "text-danger";
}

function rateBg(rate: number): string {
  if (rate >= 80) return "bg-accent";
  if (rate >= 50) return "bg-warning";
  if (rate > 0) return "bg-danger";
  return "bg-fg-muted";
}

/* ── Component ── */

export function ScenarioDetail() {
  const { id } = useParams<{ id: string }>();
  usePageTitle(id ? `Scenario: ${id}` : "Scenario");
  const { request } = useApi();

  const [runs, setRuns] = useState<Run[]>([]);
  const [scenario, setScenario] = useState<Scenario | null>(null);
  const [loading, setLoading] = useState(true);
  const [expandedRun, setExpandedRun] = useState<string | null>(null);
  const [transcripts, setTranscripts] = useState<Record<string, string>>({});
  const [transcriptLoading, setTranscriptLoading] = useState<string | null>(null);

  useEffect(() => {
    if (!id) return;
    setLoading(true);
    Promise.all([
      request<RunsResponse>(`/v1/bench/runs?scenario=${encodeURIComponent(id)}&limit=100`),
      request<ScenariosResponse>("/v1/bench/scenarios"),
    ])
      .then(([runsRes, scenariosRes]) => {
        setRuns(runsRes.runs ?? []);
        const sc = scenariosRes.scenarios?.find((s) => s.id === id) ?? null;
        setScenario(sc);
      })
      .catch(() => {})
      .finally(() => setLoading(false));
  }, [id, request]);

  // Load transcript on expand
  useEffect(() => {
    if (!expandedRun || transcripts[expandedRun] !== undefined) return;
    setTranscriptLoading(expandedRun);
    fetch(buildBenchApiURL(API_BASE, `/v1/bench/runs/${expandedRun}/transcript`))
      .then((res) => (res.ok ? res.text() : Promise.resolve("")))
      .then((text) => setTranscripts((prev) => ({ ...prev, [expandedRun]: text })))
      .catch(() => setTranscripts((prev) => ({ ...prev, [expandedRun]: "" })))
      .finally(() => setTranscriptLoading(null));
  }, [expandedRun, transcripts]);

  const modelGroups = useMemo(() => {
    const map = new Map<string, Run[]>();
    for (const r of runs) {
      const arr = map.get(r.model) ?? [];
      arr.push(r);
      map.set(r.model, arr);
    }

    const groups: ModelGroup[] = [];
    for (const [model, modelRuns] of map) {
      const passed = modelRuns.filter((r) => r.passed).length;
      const rate = modelRuns.length > 0 ? (passed / modelRuns.length) * 100 : 0;
      groups.push({
        model,
        runs: modelRuns,
        passed,
        failed: modelRuns.length - passed,
        rate,
        avgDuration: modelRuns.reduce((s, r) => s + r.duration_seconds, 0) / modelRuns.length,
        avgCost: modelRuns.reduce((s, r) => s + r.estimated_cost_usd, 0) / modelRuns.length,
        avgTokens:
          modelRuns.reduce((s, r) => s + r.prompt_tokens + r.completion_tokens, 0) /
          modelRuns.length,
      });
    }

    groups.sort((a, b) => b.rate - a.rate || b.passed - a.passed);
    return groups;
  }, [runs]);

  const totalRuns = runs.length;
  const totalPassed = runs.filter((r) => r.passed).length;
  const overallRate = totalRuns > 0 ? (totalPassed / totalRuns) * 100 : 0;

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Loading scenario...
      </div>
    );
  }

  return (
    <div className="space-y-6">
      {/* Breadcrumb */}
      <div className="flex items-center gap-3 flex-wrap">
        <Link
          to="/bench/scenarios"
          className="text-accent text-[0.82rem] font-medium hover:underline"
        >
          &larr; Scenarios
        </Link>
        <span className="text-fg-muted text-[0.75rem]">/</span>
        <span className="text-fg font-semibold text-[0.95rem]">{id}</span>
      </div>

      {/* Header */}
      {scenario && (
        <div className="glass-card p-5">
          <h1 className="text-[1.2rem] font-bold text-fg">{scenario.title}</h1>
          {scenario.description && (
            <p className="text-[0.82rem] text-fg-muted mt-2 leading-relaxed whitespace-pre-line">
              {scenario.description.trim()}
            </p>
          )}
          <div className="flex flex-wrap items-center gap-2 mt-2">
            <span className="bg-accent-subtle text-fg-muted font-medium text-[0.72rem] px-2 py-0.5 rounded">
              {scenario.category}
            </span>
            {scenario.tags.map((tag) => (
              <span
                key={tag}
                className="bg-bg-alt/80 text-fg-muted text-[0.72rem] px-2 py-0.5 rounded"
              >
                {tag}
              </span>
            ))}
            {scenario.chaos && (
              <span className="bg-warning-tint text-warning text-[0.72rem] px-2 py-0.5 rounded">
                chaos
              </span>
            )}
          </div>

          {scenario.autopsy_description && (
            <div className="mt-4 rounded-lg border border-border bg-bg-elevated p-4">
              <div className="mb-2 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">
                Autopsy rulebook
              </div>
              <p className="whitespace-pre-line text-[0.8rem] leading-relaxed text-fg-body">
                {scenario.autopsy_description.trim()}
              </p>
            </div>
          )}

          {/* Stats row */}
          <div className="flex items-center gap-6 mt-4 font-mono text-[0.82rem]">
            <span className="text-fg-muted">
              <strong className="text-fg">{totalRuns}</strong> runs
            </span>
            <span className="text-fg-muted">
              <strong className="text-fg">{totalPassed}</strong> passed
            </span>
            <span className={`font-semibold ${rateColor(overallRate)}`}>
              {overallRate.toFixed(0)}% pass rate
            </span>
            <span className="text-fg-muted">
              {modelGroups.length} models tested
            </span>
          </div>
        </div>
      )}

      {/* Model comparison cards */}
      <h2 className="text-[0.95rem] font-semibold text-fg">
        Results by Model
      </h2>

      <div className="space-y-4">
        {modelGroups.map((group) => (
          <div
            key={group.model}
            className="glass-card overflow-hidden"
          >
            {/* Model header */}
            <div className="px-5 py-4 flex items-center gap-4 flex-wrap">
              <div className="flex items-center gap-3 flex-1 min-w-0">
                <span className="font-mono text-[0.92rem] font-bold text-fg">
                  {group.model}
                </span>
                <div className="flex items-center gap-2">
                  <div className="w-20 h-2 rounded-full bg-bg-alt/80 overflow-hidden">
                    <div
                      className={`h-full rounded-full ${rateBg(group.rate)}`}
                      style={{ width: `${group.rate}%` }}
                    />
                  </div>
                  <span className={`font-mono text-[0.82rem] font-semibold ${rateColor(group.rate)}`}>
                    {group.rate.toFixed(0)}%
                  </span>
                </div>
              </div>

              <div className="flex items-center gap-4 font-mono text-[0.75rem] text-fg-muted">
                <span>
                  <strong className="text-fg">{group.passed}</strong>/{group.runs.length} passed
                </span>
                <span>{formatDuration(group.avgDuration)} avg</span>
                <span>{formatCost(group.avgCost)}/run</span>
              </div>
            </div>

            {/* Runs table */}
            <table className="w-full text-[0.8rem]">
              <thead>
                <tr className="border-t border-b border-border bg-bg-alt/80">
                  {["", "Status", "Duration", "Turns", "Tokens", "Cost", "Checks", "Date", ""].map(
                    (h, i) => (
                      <th
                        key={i}
                        className="text-left text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-1.5"
                      >
                        {h}
                      </th>
                    ),
                  )}
                </tr>
              </thead>
              <tbody>
                {group.runs.map((run) => {
                  const isExpanded = expandedRun === run.id;
                  const transcript = transcripts[run.id];
                  const isLoading = transcriptLoading === run.id;

                  return (
                    <>
                      <tr
                        key={run.id}
                        className="border-b border-border-subtle hover:bg-accent-subtle transition-colors cursor-pointer"
                        onClick={() => setExpandedRun(isExpanded ? null : run.id)}
                      >
                        <td className="px-4 py-2 w-6">
                          <span className="text-fg-muted text-[0.7rem]">
                            {isExpanded ? "\u25BC" : "\u25B6"}
                          </span>
                        </td>
                        <td className="px-4 py-2">
                          <span
                            className={`inline-block font-mono text-[0.7rem] font-semibold px-2 py-0.5 rounded ${
                              run.passed
                                ? "bg-accent-tint text-accent"
                                : "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]"
                            }`}
                          >
                            {run.passed ? "PASS" : "FAIL"}
                          </span>
                        </td>
                        <td className="px-4 py-2 font-mono text-fg-muted text-[0.76rem]">
                          {formatDuration(run.duration_seconds)}
                        </td>
                        <td className="px-4 py-2 font-mono text-fg-muted text-[0.76rem]">
                          {run.turns}
                        </td>
                        <td className="px-4 py-2 font-mono text-fg-muted text-[0.76rem]">
                          {((run.prompt_tokens + run.completion_tokens) / 1000).toFixed(1)}k
                        </td>
                        <td className="px-4 py-2 font-mono text-fg-muted text-[0.76rem]">
                          {formatCost(run.estimated_cost_usd)}
                        </td>
                        <td className="px-4 py-2 font-mono text-fg-muted text-[0.76rem]">
                          {run.checks_passed}/{run.checks_total}
                        </td>
                        <td className="px-4 py-2 text-fg-muted text-[0.76rem] whitespace-nowrap">
                          {formatDate(run.created_at)}
                        </td>
                        <td className="px-4 py-2">
                          <Link
                            to={`/bench/runs/${run.id}`}
                            className="text-accent text-[0.72rem] hover:underline"
                            onClick={(e) => e.stopPropagation()}
                          >
                            detail
                          </Link>
                        </td>
                      </tr>

                      {/* Expanded transcript */}
                      {isExpanded && (
                        <tr key={`${run.id}-transcript`}>
                          <td colSpan={9} className="px-4 py-3 bg-bg-alt/80">
                            <pre
                              className={`bg-code-bg border border-border-subtle rounded-lg p-4 font-mono text-[0.75rem] leading-relaxed max-h-[400px] overflow-y-auto whitespace-pre-wrap break-words transition-opacity duration-200 ${
                                isLoading ? "opacity-40 animate-pulse min-h-[60px]" : "opacity-100"
                              }`}
                            >
                              {transcript
                                ? highlightTranscript(transcript)
                                : isLoading
                                  ? "\u00A0"
                                  : "No transcript available."}
                            </pre>
                          </td>
                        </tr>
                      )}
                    </>
                  );
                })}
              </tbody>
            </table>
          </div>
        ))}
      </div>
    </div>
  );
}

/* ── Transcript highlighting ── */

function highlightTranscript(text: string): (React.ReactElement | string)[] {
  const parts = text.split(/(\[(?:system|user|assistant|tool)\])/g);
  return parts.map((part, i) => {
    const colors: Record<string, string> = {
      "[system]": "text-fg-muted font-semibold",
      "[user]": "text-info font-semibold",
      "[assistant]": "text-accent font-semibold",
      "[tool]": "text-warning font-semibold",
    };
    const cls = colors[part];
    if (cls) {
      return (
        <span key={i} className={cls}>
          {part}
        </span>
      );
    }
    return part;
  });
}
