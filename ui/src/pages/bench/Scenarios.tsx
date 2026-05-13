import { usePageTitle } from "../../hooks/usePageTitle";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link, useNavigate, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { useAppInfo } from "../../hooks/useAppInfo";
import { evidenceModeParam } from "../../lib/catalogData.mts";
import {
  EXAM_PACKS,
  countExamPackMatches,
  resolveExamPackFilter,
  scenarioMatchesExamPack,
  type ExamPackFilter,
} from "../../lib/examPacks.mts";
import { useEvidenceMode } from "../../hooks/useEvidenceMode";
import {
  DEFAULT_RUN_SELECTION,
  RUN_PROVIDERS,
  SCENARIO_CATEGORIES,
  getModelsForProvider,
  normalizeRunSelection,
} from "../../lib/runOptions.mts";
import { benchRunPath, benchRunsPagePath, benchScenarioPath } from "../../lib/routes.mts";

interface Scenario {
  id: string;
  title: string;
  description?: string;
  category: string;
  track?: string;
  level?: string;
  tags: string[];
  chaos: boolean;
}

interface ScenariosResponse {
  items: Scenario[];
  total: number;
}

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

interface ScenarioProgress {
  scenario: string;
  status: string;
  run_id?: string;
}

interface TriggerJob {
  id: string;
  model: string;
  status: string;
  provider?: string;
  evidence_mode?: string;
  execution_mode?: string;
  total: number;
  completed: number;
  passed: number;
  failed: number;
  current_scenario?: string;
  run_ids?: string[];
  progress: ScenarioProgress[];
  created_at?: string;
  error?: string;
}

interface TriggerResponse {
  id: string;
  status: string;
  mode?: string;
}

function isTerminalTriggerStatus(status: string) {
  return status === "completed" || status === "failed" || status === "error";
}

function firstTriggerRunID(job: TriggerJob) {
  return job.run_ids?.[0] ?? job.progress.find((item) => item.run_id)?.run_id;
}

const FEATURES = ["All", "Chaos enabled"] as const;
type ViewMode = "cards" | "list";

export function Scenarios() {
  usePageTitle("Scenarios");
  const { request } = useApi();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const { mode } = useEvidenceMode();
  const { readonly } = useAppInfo();
  const [data, setData] = useState<ScenariosResponse | null>(null);
  const [stats, setStats] = useState<Map<string, ScenarioStat>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<string>("All");
  const [feature, setFeature] = useState<string>("All");
  const [view, setView] = useState<ViewMode>("list");
  const examPack = resolveExamPackFilter(searchParams.get("exam"));

  // Run trigger state
  const [runModal, setRunModal] = useState<string | null>(null); // scenario id
  const [runModel, setRunModel] = useState<string>(DEFAULT_RUN_SELECTION.model);
  const [runProvider, setRunProvider] = useState<string>(DEFAULT_RUN_SELECTION.provider);
  const [runSubmitting, setRunSubmitting] = useState(false);
  const [runJob, setRunJob] = useState<TriggerJob | null>(null);
  const [runError, setRunError] = useState<string | null>(null);
  const pollTimeoutRef = useRef<number | null>(null);
  const pollTokenRef = useRef(0);

  useEffect(() => {
    Promise.all([
      request<{ scenarios?: Scenario[]; items?: Scenario[] }>(`/v1/bench/scenarios${evidenceModeParam("?", mode)}`),
      request<Stats>(`/v1/bench/stats${evidenceModeParam("?", mode)}`),
    ])
      .then(([raw, st]) => {
        const items = raw.items ?? raw.scenarios ?? [];
        setData({ items, total: items.length });
        const map = new Map<string, ScenarioStat>();
        for (const s of st.by_scenario ?? []) {
          map.set(s.scenario_id, s);
        }
        setStats(map);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [request, mode]);

  const filtered = useMemo(() => {
    if (!data) return [];
    return data.items.filter((s) => {
      if (examPack !== "all" && !scenarioMatchesExamPack(s, examPack)) return false;
      if (search) {
        const q = search.toLowerCase();
        if (!s.id.toLowerCase().includes(q) && !s.title.toLowerCase().includes(q)) return false;
      }
      if (category !== "All" && s.category !== category) return false;
      if (feature === "Chaos enabled" && !s.chaos) return false;
      return true;
    });
  }, [data, examPack, search, category, feature]);

  const examPackCounts = useMemo(
    () => countExamPackMatches(data?.items ?? []),
    [data],
  );

  const grouped = useMemo(() => {
    const groups = new Map<string, Scenario[]>();
    for (const s of filtered) {
      const list = groups.get(s.category) ?? [];
      list.push(s);
      groups.set(s.category, list);
    }
    return groups;
  }, [filtered]);

  const runModels = useMemo(() => getModelsForProvider(runProvider), [runProvider]);

  const selectExamPack = useCallback(
    (next: ExamPackFilter) => {
      const nextParams = new URLSearchParams(searchParams);
      if (next === "all") {
        nextParams.delete("exam");
      } else {
        nextParams.set("exam", next);
      }
      setSearchParams(nextParams);
    },
    [searchParams, setSearchParams],
  );

  const cancelPolling = useCallback(() => {
    pollTokenRef.current += 1;
    if (pollTimeoutRef.current !== null) {
      window.clearTimeout(pollTimeoutRef.current);
      pollTimeoutRef.current = null;
    }
  }, []);

  useEffect(() => () => cancelPolling(), [cancelPolling]);

  // Submit run
  const submitRun = useCallback(async () => {
    if (!runModal) return;
    const selection = normalizeRunSelection(runProvider, runModel);
    const evidenceMode = mode === "mcp" ? "mcp" : "none";
    cancelPolling();
    setRunSubmitting(true);
    setRunError(null);
    setRunJob(null);
    try {
      const res = await request<TriggerResponse>("/v1/bench/trigger", {
        method: "POST",
        body: JSON.stringify({
          model: selection.model,
          provider: selection.provider,
          evidence_mode: evidenceMode,
          execution_mode: "provider",
          scenarios: [runModal],
        }),
      });
      const jobId = res.id;
      const pendingJob: TriggerJob = {
        id: jobId,
        model: selection.model,
        provider: selection.provider,
        status: res.status || "pending",
        evidence_mode: evidenceMode,
        execution_mode: "provider",
        total: 1,
        completed: 0,
        passed: 0,
        failed: 0,
        progress: [{ scenario: runModal, status: "pending" }],
        created_at: new Date().toISOString(),
      };
      setRunJob(pendingJob);
      const pollToken = pollTokenRef.current + 1;
      pollTokenRef.current = pollToken;
      const poll = async () => {
        try {
          const status = await request<TriggerJob>(`/v1/bench/trigger/${jobId}`);
          if (pollTokenRef.current !== pollToken) return;
          setRunJob(status);
          if (!isTerminalTriggerStatus(status.status)) {
            pollTimeoutRef.current = window.setTimeout(() => {
              void poll();
            }, 2000);
            return;
          }
          pollTimeoutRef.current = null;
        } catch (err) {
          if (pollTokenRef.current !== pollToken) return;
          const message = err instanceof Error ? err.message : String(err);
          setRunJob((current) =>
            current
              ? { ...current, status: "failed", error: message }
              : { ...pendingJob, status: "failed", error: message },
          );
          pollTimeoutRef.current = null;
        }
      };
      void poll();
    } catch (err) {
      setRunError(err instanceof Error ? err.message : String(err));
    } finally {
      setRunSubmitting(false);
    }
  }, [cancelPolling, mode, runModal, runModel, runProvider, request]);

  const closeModal = () => {
    cancelPolling();
    setRunModal(null);
    setRunJob(null);
    setRunError(null);
    setRunSubmitting(false);
  };
  const runJobRunID = runJob ? firstTriggerRunID(runJob) : undefined;

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Loading scenarios...
      </div>
    );
  }

  if (error) {
    return (
      <div className="flex items-center justify-center py-20 text-danger text-[0.85rem]">
        Failed to load scenarios: {error}
      </div>
    );
  }

  return (
    <div className="flex flex-col gap-6">
      {/* Header */}
      <div>
        <h1 className="text-xl font-bold text-fg">Live Agent Exam Catalog</h1>
        <p className="text-[0.83rem] text-fg-muted mt-1">
          {data?.total ?? 0} real scenarios packaged as public exams for AI infrastructure agents
        </p>
      </div>

      {/* Exam packs */}
      <section className="grid grid-cols-[repeat(auto-fit,minmax(220px,1fr))] gap-3">
        <button
          onClick={() => selectExamPack("all")}
          className={`glass-card p-4 text-left transition-all ${
            examPack === "all"
              ? "border-accent shadow-[0_0_0_1px_var(--color-accent)]"
              : "hover:border-accent/50"
          }`}
        >
          <div className="flex items-start justify-between gap-3">
            <div>
              <h2 className="text-[0.9rem] font-bold text-fg">All Exam Scenarios</h2>
              <p className="text-[0.74rem] text-fg-muted leading-relaxed mt-1">
                Full live catalog across Kubernetes, GitOps, Terraform, AWS, and MCP readiness work.
              </p>
            </div>
            <span className="font-mono text-[0.82rem] font-bold text-accent">{data?.total ?? 0}</span>
          </div>
        </button>

        {EXAM_PACKS.map((pack) => {
          const active = examPack === pack.id;
          return (
            <button
              key={pack.id}
              onClick={() => selectExamPack(pack.id)}
              className={`glass-card p-4 text-left transition-all ${
                active
                  ? "border-accent shadow-[0_0_0_1px_var(--color-accent)]"
                  : "hover:border-accent/50"
              }`}
            >
              <div className="flex items-start justify-between gap-3">
                <div>
                  <h2 className="text-[0.9rem] font-bold text-fg">{pack.shortTitle}</h2>
                  <p className="text-[0.74rem] text-fg-muted leading-relaxed mt-1">
                    {pack.summary}
                  </p>
                </div>
                <span className="font-mono text-[0.82rem] font-bold text-accent">
                  {examPackCounts[pack.id] ?? 0}
                </span>
              </div>
              <p className="text-[0.68rem] text-fg-muted/80 leading-relaxed mt-3">
                {pack.proof}
              </p>
            </button>
          );
        })}
      </section>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder="Search by ID or title..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg glass-card placeholder:text-fg-muted/50 focus:outline-none focus:border-accent transition-colors w-64"
        />

        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg glass-card focus:outline-none focus:border-accent transition-colors cursor-pointer"
        >
          {SCENARIO_CATEGORIES.map((c) => (
            <option key={c} value={c}>
              {c === "All" ? "All categories" : c}
            </option>
          ))}
        </select>

        <select
          value={feature}
          onChange={(e) => setFeature(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg glass-card focus:outline-none focus:border-accent transition-colors cursor-pointer"
        >
          {FEATURES.map((f) => (
            <option key={f} value={f}>
              {f === "All" ? "All features" : f}
            </option>
          ))}
        </select>

        <div className="ml-auto flex gap-0 border border-border rounded-md overflow-hidden">
          <button
            onClick={() => setView("cards")}
            className={`px-2.5 py-1.5 text-[0.78rem] border-r border-border cursor-pointer transition-all ${
              view === "cards"
                ? "bg-accent-tint text-accent font-semibold"
                : "bg-bg-elevated text-fg-muted hover:text-fg"
            }`}
            title="Card view"
          >
            {"\u25A6"}
          </button>
          <button
            onClick={() => setView("list")}
            className={`px-2.5 py-1.5 text-[0.78rem] cursor-pointer transition-all ${
              view === "list"
                ? "bg-accent-tint text-accent font-semibold"
                : "bg-bg-elevated text-fg-muted hover:text-fg"
            }`}
            title="List view"
          >
            {"\u2630"}
          </button>
        </div>
      </div>

      {/* Empty state */}
      {filtered.length === 0 && (
        <div className="flex items-center justify-center py-16 text-fg-muted text-[0.85rem]">
          No scenarios match the current filters.
        </div>
      )}

      {/* Card view */}
      {view === "cards" &&
        Array.from(grouped.entries()).map(([cat, scenarios]) => (
          <section key={cat} className="flex flex-col gap-3">
            <h2 className="text-[0.85rem] font-semibold text-fg-muted uppercase tracking-wide">
              {cat}
              <span className="ml-2 font-normal normal-case tracking-normal">
                ({scenarios.length})
              </span>
            </h2>

            <div className="grid grid-cols-[repeat(auto-fill,minmax(320px,1fr))] gap-4">
              {scenarios.map((s) => (
                <ScenarioCard
                  key={s.id}
                  scenario={s}
                  stat={stats.get(s.id)}
                  readonly={readonly}
                  onRun={() => setRunModal(s.id)}
                />
              ))}
            </div>
          </section>
        ))}

      {/* List view */}
      {view === "list" && filtered.length > 0 && (
        <div className="glass-card overflow-hidden">
          <table className="w-full text-[0.82rem]">
            <thead>
              <tr className="border-b border-border bg-bg-alt/80">
                {["Scenario", "Title", "Category", "Tags", "Runs", "Passed", "Rate", ...(readonly ? [] : [""])].map((h) => (
                  <th
                    key={h || "actions"}
                    className="text-left text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted px-4 py-2"
                  >
                    {h}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {filtered.map((s) => {
                const stat = stats.get(s.id);
                const passRate =
                  stat && stat.runs > 0
                    ? Math.round((stat.passed / stat.runs) * 100)
                    : null;
                return (
                  <tr
                    key={s.id}
                    className="border-b border-border-subtle last:border-0 hover:bg-accent-subtle transition-colors cursor-pointer"
                    onClick={() => navigate(benchScenarioPath(s.id))}
                  >
                    <td className="py-2.5 px-4 font-mono text-[0.78rem] text-accent whitespace-nowrap">
                      {s.id}
                    </td>
                    <td className="py-2.5 px-4 text-fg font-medium">
                      {s.title}
                    </td>
                    <td className="py-2.5 px-4">
                      <span className="bg-accent-subtle text-fg-muted font-medium text-[0.72rem] px-2 py-0.5 rounded">
                        {s.category}
                      </span>
                    </td>
                    <td className="py-2.5 px-4">
                      <div className="flex flex-wrap gap-1">
                        {s.tags.slice(0, 3).map((tag) => (
                          <span
                            key={tag}
                            className="bg-bg-alt/80 text-fg-muted text-[0.68rem] px-1.5 py-0.5 rounded"
                          >
                            {tag}
                          </span>
                        ))}
                        {s.chaos && (
                          <span className="bg-warning-tint text-warning text-[0.68rem] px-1.5 py-0.5 rounded">
                            chaos
                          </span>
                        )}
                      </div>
                    </td>
                    <td className="py-2.5 px-4 font-mono text-[0.78rem] text-fg-muted">
                      {stat?.runs ?? "\u2014"}
                    </td>
                    <td className="py-2.5 px-4 font-mono text-[0.78rem] text-fg-muted">
                      {stat ? `${stat.passed}/${stat.runs}` : "\u2014"}
                    </td>
                    <td className="py-2.5 px-4">
                      {passRate !== null ? (
                        <span
                          className={`font-mono text-[0.78rem] font-semibold ${
                            passRate >= 70
                              ? "text-accent"
                              : passRate >= 40
                                ? "text-warning"
                                : "text-danger"
                          }`}
                        >
                          {passRate}%
                        </span>
                      ) : (
                        <span className="font-mono text-[0.78rem] text-fg-muted">{"\u2014"}</span>
                      )}
                    </td>
                    {!readonly && (
                      <td className="py-2.5 px-4">
                        <button
                          onClick={(e) => {
                            e.stopPropagation();
                            setRunModal(s.id);
                          }}
                          className="text-[0.72rem] font-semibold px-2.5 py-1 rounded bg-accent text-[#064e3b] hover:bg-accent-bright transition-colors cursor-pointer"
                        >
                          Run
                        </button>
                      </td>
                    )}
                  </tr>
                );
              })}
            </tbody>
          </table>
        </div>
      )}

      {/* Run modal */}
      {runModal && (
        <div
          className="fixed inset-0 z-50 flex items-center justify-center"
          style={{ background: "rgba(0,0,0,0.5)", backdropFilter: "blur(4px)" }}
          onClick={closeModal}
        >
          <div
            className="glass-card p-6 w-full max-w-md"
            onClick={(e) => e.stopPropagation()}
          >
            <h3 className="text-[1rem] font-bold text-fg mb-1">Run Scenario</h3>
            <p className="font-mono text-[0.78rem] text-accent mb-4">{runModal}</p>

            {!runJob ? (
              <>
                <div className="space-y-3 mb-5">
                  <label className="flex flex-col gap-1">
                    <span className="text-[0.72rem] font-semibold uppercase tracking-wide text-fg-muted">
                      Model
                    </span>
                    <select
                      value={runModel}
                      onChange={(e) => setRunModel(e.target.value)}
                      className="h-9 px-3 text-[0.83rem] text-fg bg-bg-alt/80 border border-border-subtle rounded-lg cursor-pointer"
                    >
                      {runModels.map((m) => (
                        <option key={m} value={m}>{m}</option>
                      ))}
                    </select>
                  </label>

                  <label className="flex flex-col gap-1">
                    <span className="text-[0.72rem] font-semibold uppercase tracking-wide text-fg-muted">
                      Provider
                    </span>
                    <select
                      value={runProvider}
                      onChange={(e) => {
                        const selection = normalizeRunSelection(e.target.value, runModel);
                        setRunProvider(selection.provider);
                        setRunModel(selection.model);
                      }}
                      className="h-9 px-3 text-[0.83rem] text-fg bg-bg-alt/80 border border-border-subtle rounded-lg cursor-pointer"
                    >
                      {RUN_PROVIDERS.map((p) => (
                        <option key={p} value={p}>{p}</option>
                      ))}
                    </select>
                  </label>

                </div>

                {runError && (
                  <p className="text-danger text-[0.78rem] mb-3">{runError}</p>
                )}

                <div className="flex gap-2 justify-end">
                  <button
                    onClick={closeModal}
                    className="px-4 py-2 text-[0.82rem] font-medium rounded-lg border border-border text-fg-muted hover:text-fg cursor-pointer transition-colors"
                  >
                    Cancel
                  </button>
                  <button
                    onClick={submitRun}
                    disabled={runSubmitting}
                    className="px-4 py-2 text-[0.82rem] font-semibold rounded-lg bg-accent text-[#064e3b] hover:bg-accent-bright disabled:opacity-50 cursor-pointer transition-colors"
                  >
                    {runSubmitting ? "Starting..." : "Start Run"}
                  </button>
                </div>
              </>
            ) : (
              <div className="space-y-3">
                <div className="flex items-center gap-2">
                  <span
                    className={`inline-block w-2 h-2 rounded-full ${
                      runJob.status === "running"
                        ? "bg-info animate-pulse"
                        : runJob.status === "completed"
                          ? "bg-accent"
                          : runJob.status === "failed" || runJob.status === "error"
                            ? "bg-danger"
                            : "bg-fg-muted"
                    }`}
                  />
                  <span className="text-[0.85rem] font-semibold text-fg capitalize">
                    {runJob.status}
                  </span>
                </div>

                <div className="text-[0.78rem] text-fg-muted space-y-1">
                  <p>Model: <span className="font-mono text-fg">{runJob.model}</span></p>
                  {runJob.provider && <p>Provider: <span className="font-mono text-fg">{runJob.provider}</span></p>}
                  <p>
                    Progress:{" "}
                    <span className="font-mono text-fg">
                      {runJob.completed}/{runJob.total}
                    </span>
                  </p>
                  {runJob.current_scenario && (
                    <p>Current: <span className="font-mono text-fg">{runJob.current_scenario}</span></p>
                  )}
                  {isTerminalTriggerStatus(runJob.status) && (
                    <p>
                      Result:{" "}
                      <span className={`font-semibold ${runJob.failed === 0 ? "text-accent" : "text-danger"}`}>
                        {runJob.failed === 0 ? "PASS" : "FAIL"}
                      </span>
                    </p>
                  )}
                  {(runJob.error || runJob.failed > 0) && (
                    <p className="text-danger">
                      {runJob.error || `${runJob.failed} scenario${runJob.failed === 1 ? "" : "s"} failed`}
                    </p>
                  )}
                  {runJobRunID && (
                    <p>
                      Run:{" "}
                      <Link
                        to={benchRunPath(runJobRunID)}
                        className="text-accent hover:text-accent-bright"
                        onClick={closeModal}
                      >
                        {runJobRunID}
                      </Link>
                    </p>
                  )}
                </div>

                <div className="flex justify-end pt-2">
                  <button
                    onClick={closeModal}
                    className="px-4 py-2 text-[0.82rem] font-medium rounded-lg bg-accent text-[#064e3b] hover:bg-accent-bright cursor-pointer transition-colors"
                  >
                    Close
                  </button>
                </div>
              </div>
            )}
          </div>
        </div>
      )}
    </div>
  );
}

function ScenarioCard({
  scenario,
  stat,
  readonly,
  onRun,
}: {
  scenario: Scenario;
  stat?: ScenarioStat;
  readonly: boolean;
  onRun: () => void;
}) {
  const passRate = stat && stat.runs > 0
    ? Math.round((stat.passed / stat.runs) * 100)
    : null;

  return (
    <div className="glass-card p-4 hover:border-accent hover:shadow-[var(--shadow-card-lg)] hover:-translate-y-px transition-all flex flex-col gap-2">
      <Link
        to={benchRunsPagePath({ scenario: scenario.id })}
        className="flex flex-col gap-1"
        style={{ textDecoration: "none", color: "inherit" }}
      >
        <span className="text-[0.85rem] font-bold text-fg">{scenario.title}</span>
        <span className="font-mono text-[0.73rem] text-fg-muted">{scenario.id}</span>
        {scenario.description && (
          <span className="text-[0.73rem] text-fg-muted leading-snug line-clamp-2">{scenario.description.trim().split('\n')[0]}</span>
        )}
      </Link>

      {scenario.tags.length > 0 && (
        <div className="flex flex-wrap gap-1.5">
          {scenario.tags.map((tag) => (
            <span
              key={tag}
              className="bg-accent-subtle text-fg-muted font-medium text-[0.72rem] px-2 py-0.5 rounded"
            >
              {tag}
            </span>
          ))}
          {scenario.chaos && (
            <span className="bg-warning-tint text-warning font-medium text-[0.72rem] px-2 py-0.5 rounded">
              chaos
            </span>
          )}
        </div>
      )}

      <div className="mt-auto pt-3 border-t border-border-subtle flex items-center gap-4 font-mono text-[0.73rem] text-fg-muted">
        {stat ? (
          <>
            <span>
              <strong className="text-fg">{stat.runs}</strong> runs
            </span>
            <span>
              <strong className="text-fg">{stat.passed}</strong>/{stat.runs} passed
            </span>
            <span
              className={`font-semibold ${
                passRate !== null && passRate >= 70
                  ? "text-accent"
                  : passRate !== null && passRate >= 40
                    ? "text-warning"
                    : "text-danger"
              }`}
            >
              {passRate}%
            </span>
          </>
        ) : (
          <span className="text-fg-muted">No runs yet</span>
        )}
        {!readonly && (
          <button
            onClick={(e) => {
              e.preventDefault();
              onRun();
            }}
            className="ml-auto text-[0.72rem] font-semibold px-2.5 py-1 rounded bg-accent text-[#064e3b] hover:bg-accent-bright transition-colors cursor-pointer"
          >
            Run
          </button>
        )}
      </div>
    </div>
  );
}
