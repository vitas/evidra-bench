import { usePageTitle } from "../hooks/usePageTitle";
import { useCallback, useEffect, useMemo, useRef, useState } from "react";
import { Link } from "react-router";
import { useApi } from "../hooks/useApi";
import { useAppInfo } from "../hooks/useAppInfo";
import {
  DEFAULT_RUN_SELECTION,
  RUN_PROVIDERS,
  SCENARIO_CATEGORIES,
  getModelsForProvider,
  normalizeRunSelection,
} from "../lib/runOptions.mts";

interface Scenario {
  id: string;
  title: string;
  category: string;
  tags: string[];
  chaos: boolean;
  evidra: boolean;
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

interface JobStatus {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  status: string;
  started_at: string;
  ended_at?: string;
  run_id?: string;
  exit_code?: number;
  passed?: boolean;
  error?: string;
}

const FEATURES = ["All", "Chaos enabled", "Evidra enabled"] as const;
type ViewMode = "cards" | "list";

export function Scenarios() {
  usePageTitle("Scenarios");
  const { request } = useApi();
  const { readonly } = useAppInfo();
  const [data, setData] = useState<ScenariosResponse | null>(null);
  const [stats, setStats] = useState<Map<string, ScenarioStat>>(new Map());
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  const [search, setSearch] = useState("");
  const [category, setCategory] = useState<string>("All");
  const [feature, setFeature] = useState<string>("All");
  const [view, setView] = useState<ViewMode>("list");

  // Run trigger state
  const [runModal, setRunModal] = useState<string | null>(null); // scenario id
  const [runModel, setRunModel] = useState<string>(DEFAULT_RUN_SELECTION.model);
  const [runProvider, setRunProvider] = useState<string>(DEFAULT_RUN_SELECTION.provider);
  const [runDryRun, setRunDryRun] = useState(false);
  const [runSubmitting, setRunSubmitting] = useState(false);
  const [runJob, setRunJob] = useState<JobStatus | null>(null);
  const [runError, setRunError] = useState<string | null>(null);
  const pollTimeoutRef = useRef<number | null>(null);
  const pollTokenRef = useRef(0);

  useEffect(() => {
    Promise.all([
      request<ScenariosResponse>("/v1/bench/scenarios"),
      request<Stats>("/v1/bench/stats"),
    ])
      .then(([scenarios, st]) => {
        setData(scenarios);
        const map = new Map<string, ScenarioStat>();
        for (const s of st.by_scenario ?? []) {
          map.set(s.scenario_id, s);
        }
        setStats(map);
      })
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [request]);

  const filtered = useMemo(() => {
    if (!data) return [];
    return data.items.filter((s) => {
      if (search) {
        const q = search.toLowerCase();
        if (!s.id.toLowerCase().includes(q) && !s.title.toLowerCase().includes(q)) return false;
      }
      if (category !== "All" && s.category !== category) return false;
      if (feature === "Chaos enabled" && !s.chaos) return false;
      if (feature === "Evidra enabled" && !s.evidra) return false;
      return true;
    });
  }, [data, search, category, feature]);

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
    cancelPolling();
    setRunSubmitting(true);
    setRunError(null);
    setRunJob(null);
    try {
      const res = await request<{ job_id: string }>("/v1/bench/execute", {
        method: "POST",
        body: JSON.stringify({
          scenario_id: runModal,
          model: selection.model,
          provider: selection.provider,
          dry_run: runDryRun,
        }),
      });
      const jobId = res.job_id;
      const pendingJob: JobStatus = {
        id: jobId,
        scenario_id: runModal,
        model: selection.model,
        provider: selection.provider,
        status: "pending",
        started_at: new Date().toISOString(),
      };
      setRunJob(pendingJob);
      const pollToken = pollTokenRef.current + 1;
      pollTokenRef.current = pollToken;
      const poll = async () => {
        try {
          const status = await request<JobStatus>(`/v1/bench/execute/${jobId}/status`);
          if (pollTokenRef.current !== pollToken) return;
          setRunJob(status);
          if (status.status === "pending" || status.status === "running") {
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
  }, [cancelPolling, runDryRun, runModal, runModel, runProvider, request]);

  const closeModal = () => {
    cancelPolling();
    setRunModal(null);
    setRunJob(null);
    setRunError(null);
    setRunSubmitting(false);
  };

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
        <h1 className="text-xl font-bold text-fg">Scenarios</h1>
        <p className="text-[0.83rem] text-fg-muted mt-1">
          {data?.total ?? 0} scenarios across Kubernetes, Helm, Argo CD, and Terraform
        </p>
      </div>

      {/* Filters */}
      <div className="flex flex-wrap items-center gap-3">
        <input
          type="text"
          placeholder="Search by ID or title..."
          value={search}
          onChange={(e) => setSearch(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg bg-bg-elevated border border-border-subtle rounded-lg placeholder:text-fg-muted/50 focus:outline-none focus:border-accent transition-colors w-64"
        />

        <select
          value={category}
          onChange={(e) => setCategory(e.target.value)}
          className="h-9 px-3 text-[0.83rem] text-fg bg-bg-elevated border border-border-subtle rounded-lg focus:outline-none focus:border-accent transition-colors cursor-pointer"
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
          className="h-9 px-3 text-[0.83rem] text-fg bg-bg-elevated border border-border-subtle rounded-lg focus:outline-none focus:border-accent transition-colors cursor-pointer"
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
        <div className="bg-bg-elevated border border-border-subtle rounded-[10px] overflow-hidden">
          <table className="w-full text-[0.82rem]">
            <thead>
              <tr className="border-b border-border bg-bg-alt">
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
                    onClick={() => (window.location.href = `/runs?scenario=${s.id}`)}
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
                            className="bg-bg-alt text-fg-muted text-[0.68rem] px-1.5 py-0.5 rounded"
                          >
                            {tag}
                          </span>
                        ))}
                        {s.chaos && (
                          <span className="bg-warning-tint text-warning text-[0.68rem] px-1.5 py-0.5 rounded">
                            chaos
                          </span>
                        )}
                        {s.evidra && (
                          <span className="bg-info-tint text-info text-[0.68rem] px-1.5 py-0.5 rounded">
                            evidra
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
            className="bg-bg-elevated border border-border-subtle rounded-[12px] shadow-[var(--shadow-card-lg)] p-6 w-full max-w-md"
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
                      className="h-9 px-3 text-[0.83rem] text-fg bg-bg-alt border border-border-subtle rounded-lg cursor-pointer"
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
                      className="h-9 px-3 text-[0.83rem] text-fg bg-bg-alt border border-border-subtle rounded-lg cursor-pointer"
                    >
                      {RUN_PROVIDERS.map((p) => (
                        <option key={p} value={p}>{p}</option>
                      ))}
                    </select>
                  </label>

                  <label className="flex items-center gap-2 cursor-pointer">
                    <input
                      type="checkbox"
                      checked={runDryRun}
                      onChange={(e) => setRunDryRun(e.target.checked)}
                      className="accent-accent"
                    />
                    <span className="text-[0.82rem] text-fg-muted">Dry run (validate only)</span>
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
                          : runJob.status === "failed"
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
                  <p>Provider: <span className="font-mono text-fg">{runJob.provider}</span></p>
                  {runJob.passed !== undefined && (
                    <p>Result: <span className={`font-semibold ${runJob.passed ? "text-accent" : "text-danger"}`}>{runJob.passed ? "PASS" : "FAIL"}</span></p>
                  )}
                  {runJob.error && (
                    <p className="text-danger">{runJob.error}</p>
                  )}
                  {runJob.run_id && (
                    <p>
                      Run:{" "}
                      <Link
                        to={`/runs/${runJob.run_id}`}
                        className="text-accent hover:text-accent-bright"
                        onClick={closeModal}
                      >
                        {runJob.run_id}
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
    <div className="bg-bg-elevated border border-border-subtle rounded-[10px] p-4 hover:border-accent hover:shadow-[var(--shadow-card-lg)] hover:-translate-y-px transition-all flex flex-col gap-2">
      <Link
        to={`/runs?scenario=${scenario.id}`}
        className="flex flex-col gap-1"
        style={{ textDecoration: "none", color: "inherit" }}
      >
        <span className="text-[0.85rem] font-bold text-fg">{scenario.title}</span>
        <span className="font-mono text-[0.73rem] text-fg-muted">{scenario.id}</span>
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
          {scenario.evidra && (
            <span className="bg-info-tint text-info font-medium text-[0.72rem] px-2 py-0.5 rounded">
              evidra
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
