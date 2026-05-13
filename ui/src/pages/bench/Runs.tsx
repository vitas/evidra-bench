import { usePageTitle } from "../../hooks/usePageTitle";
import { useCallback, useEffect, useState } from "react";
import { useNavigate, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import {
  coerceToolServerVersion,
  normalizeCatalog,
  toolServerVersionOptions,
  type CatalogResponse,
} from "../../lib/catalogData.mts";
import {
  EXAM_PACKS,
  scenarioIDsForExamPack,
  type ExamPackFilter,
  type ExamPackScenario,
} from "../../lib/examPacks.mts";
import {
  DEFAULT_RUNS_FILTERS,
  buildRunsAPIPath,
  runsFiltersFromSearchParams,
  runsSearchParamsFromFilters,
  type RunsFilterState,
  type RunsStatus,
} from "../../lib/runFilters.mts";
import { benchRunPath } from "../../lib/routes.mts";

interface RunRecord {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  adapter: string;
  tool_server: string;
  tool_server_version: string;
  passed: boolean;
  duration_seconds: number;
  exit_code: number;
  turns: number;
  memory_window: number;
  prompt_tokens: number;
  completion_tokens: number;
  estimated_cost_usd: number;
  checks_passed: number;
  checks_total: number;
  created_at: string;
}

interface RunsResponse {
  runs: RunRecord[];
  total: number;
  limit: number;
  offset: number;
}

interface ScenariosResponse {
  scenarios?: ExamPackScenario[];
  items?: ExamPackScenario[];
}

type SortField =
  | "passed"
  | "scenario_id"
  | "model"
  | "provider"
  | "duration_seconds"
  | "turns"
  | "tokens"
  | "estimated_cost_usd"
  | "checks"
  | "created_at";

type SortDir = "asc" | "desc";

const STATUSES = ["All", "Passed", "Failed"] as const;
const PAGE_SIZE = 25;

function formatDate(iso: string): string {
  const d = new Date(iso);
  const months = ["Jan", "Feb", "Mar", "Apr", "May", "Jun", "Jul", "Aug", "Sep", "Oct", "Nov", "Dec"];
  const day = String(d.getDate()).padStart(2, "0");
  const mon = months[d.getMonth()];
  const hh = String(d.getHours()).padStart(2, "0");
  const mm = String(d.getMinutes()).padStart(2, "0");
  return `${day} ${mon} ${hh}:${mm}`;
}

function formatCost(usd: number): string {
  if (usd < 0.001) return "$0.000";
  return `$${usd.toFixed(3)}`;
}

function formatDuration(s: number): string {
  return `${s.toFixed(1)}s`;
}

function formatTokens(n: number): string {
  return n.toLocaleString("en-US");
}

function SortArrow({ field, sort }: { field: SortField; sort: { field: SortField; dir: SortDir } }) {
  if (sort.field !== field) return <span className="text-fg-muted/30 ml-0.5">{"\u2195"}</span>;
  return <span className="text-accent ml-0.5">{sort.dir === "asc" ? "\u2191" : "\u2193"}</span>;
}

export function Runs() {
  usePageTitle("Runs");
  const { request } = useApi();
  const navigate = useNavigate();
  const [searchParams, setSearchParams] = useSearchParams();
  const search = searchParams.toString();
  const initialFilters = runsFiltersFromSearchParams(searchParams);

  const [data, setData] = useState<RunsResponse | null>(null);
  const [catalog, setCatalog] = useState<CatalogResponse>({
    models: [],
    providers: [],
    tool_servers: [],
    tool_server_versions: [],
  });
  const [scenarioCatalog, setScenarioCatalog] = useState<ExamPackScenario[]>([]);
  const [scenarioCatalogLoaded, setScenarioCatalogLoaded] = useState(false);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);

  // Filters
  const [scenario, setScenario] = useState(initialFilters.scenario);
  const [exam, setExam] = useState<ExamPackFilter>(initialFilters.exam);
  const [model, setModel] = useState(initialFilters.model);
  const [provider, setProvider] = useState(initialFilters.provider);
  const [toolServer, setToolServer] = useState(initialFilters.toolServer);
  const [toolServerVersion, setToolServerVersion] = useState(initialFilters.toolServerVersion);
  const [status, setStatus] = useState<RunsStatus>(initialFilters.status);
  const [since, setSince] = useState(initialFilters.since);

  // Applied filters (only update on Apply)
  const [appliedFilters, setAppliedFilters] = useState<RunsFilterState>(initialFilters);

  // Sort & pagination
  const [sort, setSort] = useState<{ field: SortField; dir: SortDir }>({
    field: "created_at",
    dir: "desc",
  });
  const [page, setPage] = useState(0);

  const fetchRuns = useCallback(async () => {
    if (appliedFilters.exam !== "all" && !scenarioCatalogLoaded) {
      return;
    }

    setLoading(true);
    setError(null);
    try {
      const suiteScenarioIDs = scenarioIDsForExamPack(scenarioCatalog, appliedFilters.exam);
      if (!appliedFilters.scenario && appliedFilters.exam !== "all" && suiteScenarioIDs.length === 0) {
        setData({ runs: [], total: 0, limit: PAGE_SIZE, offset: page * PAGE_SIZE });
        return;
      }

      const resp = await request<RunsResponse>(
        buildRunsAPIPath(appliedFilters, page, suiteScenarioIDs, PAGE_SIZE),
      );
      setData(resp);
    } catch (err) {
      setError(err instanceof Error ? err.message : "Failed to load runs");
    } finally {
      setLoading(false);
    }
  }, [request, appliedFilters, page, scenarioCatalog, scenarioCatalogLoaded]);

  useEffect(() => {
    fetchRuns();
  }, [fetchRuns]);

  useEffect(() => {
    request<CatalogResponse>("/v1/bench/catalog")
      .then((res) => setCatalog(normalizeCatalog(res)))
      .catch(() =>
        setCatalog({
          models: [],
          providers: [],
          tool_servers: [],
          tool_server_versions: [],
        }),
      );
  }, [request]);

  useEffect(() => {
    let cancelled = false;
    setScenarioCatalogLoaded(false);
    request<ScenariosResponse>("/v1/bench/scenarios")
      .then((raw) => {
        if (!cancelled) setScenarioCatalog(raw.items ?? raw.scenarios ?? []);
      })
      .catch(() => {
        if (!cancelled) setScenarioCatalog([]);
      })
      .finally(() => {
        if (!cancelled) setScenarioCatalogLoaded(true);
      });
    return () => {
      cancelled = true;
    };
  }, [request]);

  useEffect(() => {
    const nextFilters = runsFiltersFromSearchParams(new URLSearchParams(search));
    setScenario(nextFilters.scenario);
    setExam(nextFilters.exam);
    setModel(nextFilters.model);
    setProvider(nextFilters.provider);
    setToolServer(nextFilters.toolServer);
    setToolServerVersion(nextFilters.toolServerVersion);
    setStatus(nextFilters.status);
    setSince(nextFilters.since);
    setAppliedFilters(nextFilters);
    setPage(0);
  }, [search]);

  function handleApply() {
    const nextToolServerVersion = coerceToolServerVersion(catalog, toolServer, toolServerVersion, "All");
    const nextFilters: RunsFilterState = {
      scenario: scenario.trim(),
      exam,
      model,
      provider,
      toolServer,
      toolServerVersion: nextToolServerVersion,
      toolServerUnset: false,
      status,
      since,
    };
    setToolServerVersion(nextToolServerVersion);
    setAppliedFilters(nextFilters);
    setPage(0);
    setSearchParams(runsSearchParamsFromFilters(nextFilters));
  }

  function handleReset() {
    setScenario(DEFAULT_RUNS_FILTERS.scenario);
    setExam(DEFAULT_RUNS_FILTERS.exam);
    setModel(DEFAULT_RUNS_FILTERS.model);
    setProvider(DEFAULT_RUNS_FILTERS.provider);
    setToolServer(DEFAULT_RUNS_FILTERS.toolServer);
    setToolServerVersion(DEFAULT_RUNS_FILTERS.toolServerVersion);
    setStatus(DEFAULT_RUNS_FILTERS.status);
    setSince(DEFAULT_RUNS_FILTERS.since);
    setAppliedFilters(DEFAULT_RUNS_FILTERS);
    setPage(0);
    setSearchParams(new URLSearchParams());
  }

  function handleSort(field: SortField) {
    setSort((prev) =>
      prev.field === field ? { field, dir: prev.dir === "asc" ? "desc" : "asc" } : { field, dir: "asc" },
    );
  }

  // Client-side sort (server could also sort, but we sort the current page)
  const sorted = data?.runs ? [...data.runs] : [];
  sorted.sort((a, b) => {
    const dir = sort.dir === "asc" ? 1 : -1;
    switch (sort.field) {
      case "passed":
        return (Number(a.passed) - Number(b.passed)) * dir;
      case "scenario_id":
        return a.scenario_id.localeCompare(b.scenario_id) * dir;
      case "model":
        return a.model.localeCompare(b.model) * dir;
      case "provider":
        return a.provider.localeCompare(b.provider) * dir;
      case "duration_seconds":
        return (a.duration_seconds - b.duration_seconds) * dir;
      case "turns":
        return (a.turns - b.turns) * dir;
      case "tokens":
        return (a.prompt_tokens + a.completion_tokens - (b.prompt_tokens + b.completion_tokens)) * dir;
      case "estimated_cost_usd":
        return (a.estimated_cost_usd - b.estimated_cost_usd) * dir;
      case "checks":
        return (a.checks_passed - b.checks_passed) * dir;
      case "created_at":
        return a.created_at.localeCompare(b.created_at) * dir;
      default:
        return 0;
    }
  });

  const total = data?.total ?? 0;
  const totalPages = Math.max(1, Math.ceil(total / PAGE_SIZE));
  const rangeStart = total === 0 ? 0 : page * PAGE_SIZE + 1;
  const rangeEnd = Math.min((page + 1) * PAGE_SIZE, total);
  const selectedExamPack = EXAM_PACKS.find((pack) => pack.id === appliedFilters.exam);
  const availableToolServerVersions = toolServerVersionOptions(catalog, toolServer);
  const catalogHasToolServerData =
    catalog.tool_servers.length > 0 || (catalog.tool_server_versions ?? []).length > 0;

  useEffect(() => {
    if (!catalogHasToolServerData) return;

    const nextDraftVersion = coerceToolServerVersion(catalog, toolServer, toolServerVersion, "All");
    if (nextDraftVersion !== toolServerVersion) {
      setToolServerVersion(nextDraftVersion);
    }

    const nextAppliedVersion = coerceToolServerVersion(
      catalog,
      appliedFilters.toolServer,
      appliedFilters.toolServerVersion,
      "All",
    );
    if (nextAppliedVersion !== appliedFilters.toolServerVersion) {
      const nextFilters = { ...appliedFilters, toolServerVersion: nextAppliedVersion };
      setAppliedFilters(nextFilters);
      setSearchParams(runsSearchParamsFromFilters(nextFilters), { replace: true });
    }
  }, [appliedFilters, catalog, catalogHasToolServerData, setSearchParams, toolServer, toolServerVersion]);

  const inputClass =
    "font-sans text-[0.8rem] px-3 py-[0.45rem] border border-border rounded-md bg-bg-elevated text-fg-body focus:outline-none focus:border-accent transition-colors";

  const thClass =
    "text-left text-[0.7rem] font-semibold text-fg-muted uppercase tracking-wide bg-bg-alt/80 px-3 py-2.5 select-none cursor-pointer hover:text-fg transition-colors whitespace-nowrap";

  return (
    <div>
      {/* Header */}
      <div className="mb-5">
        <h1 className="text-[1.35rem] font-bold text-fg tracking-tight">Runs</h1>
        <p className="text-[0.82rem] text-fg-muted mt-0.5">
          Browse and filter benchmark run results
          {selectedExamPack ? ` in ${selectedExamPack.title}` : ""}
        </p>
      </div>

      {/* Filters bar */}
      <div className="flex flex-wrap items-end gap-3 mb-5">
        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Scenario</span>
          <input
            type="text"
            placeholder="e.g. K01"
            value={scenario}
            onChange={(e) => setScenario(e.target.value)}
            className={inputClass + " w-36"}
          />
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Exam Suite</span>
          <select
            value={exam}
            onChange={(e) => setExam(e.target.value as ExamPackFilter)}
            className={inputClass + " w-44"}
          >
            <option value="all">All Suites</option>
            {EXAM_PACKS.map((pack) => (
              <option key={pack.id} value={pack.id}>
                {pack.shortTitle}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Model</span>
          <select value={model} onChange={(e) => setModel(e.target.value)} className={inputClass + " w-32"}>
            {["All", ...catalog.models].map((m) => (
              <option key={m} value={m}>
                {m}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Provider</span>
          <select value={provider} onChange={(e) => setProvider(e.target.value)} className={inputClass + " w-32"}>
            {["All", ...catalog.providers].map((p) => (
              <option key={p} value={p}>
                {p}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Tool Server</span>
          <select
            value={toolServer}
            onChange={(e) => {
              const nextToolServer = e.target.value;
              setToolServer(nextToolServer);
              setToolServerVersion((current) => coerceToolServerVersion(catalog, nextToolServer, current, "All"));
            }}
            className={inputClass + " w-40"}
          >
            {["All", ...catalog.tool_servers].map((tool) => (
              <option key={tool} value={tool}>
                {tool}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Version</span>
          <select
            value={toolServerVersion}
            onChange={(e) => setToolServerVersion(e.target.value)}
            className={inputClass + " w-40"}
            disabled={toolServer !== "All" && availableToolServerVersions.length === 0}
          >
            {["All", ...availableToolServerVersions].map((version) => (
              <option key={version} value={version}>
                {version === "All" ? "All versions" : version}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Status</span>
          <select
            value={status}
            onChange={(e) => setStatus(e.target.value as RunsStatus)}
            className={inputClass + " w-28"}
          >
            {STATUSES.map((s) => (
              <option key={s} value={s}>
                {s}
              </option>
            ))}
          </select>
        </label>

        <label className="flex flex-col gap-1">
          <span className="text-[0.7rem] font-medium text-fg-muted uppercase tracking-wide">Since</span>
          <input
            type="date"
            value={since}
            onChange={(e) => setSince(e.target.value)}
            className={inputClass + " w-36"}
          />
        </label>

        <button
          onClick={handleApply}
          className="bg-accent text-[#064e3b] font-semibold rounded-md text-[0.8rem] px-4 py-[0.45rem] cursor-pointer hover:opacity-90 transition-opacity"
        >
          Apply
        </button>
        <button
          onClick={handleReset}
          className="bg-transparent text-accent border border-accent rounded-md text-[0.8rem] px-4 py-[0.45rem] cursor-pointer hover:bg-accent-tint transition-colors"
        >
          Reset
        </button>
      </div>

      {/* Error */}
      {error && (
        <div className="mb-4 px-4 py-3 rounded-md bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)] text-[0.82rem]">
          {error}
        </div>
      )}

      {/* Table card */}
      <div className="border border-border rounded-lg overflow-hidden bg-bg-elevated">
        {loading ? (
          <div className="flex items-center justify-center py-16 text-fg-muted text-[0.85rem]">
            Loading runs...
          </div>
        ) : sorted.length === 0 ? (
          <div className="flex flex-col items-center justify-center py-16 text-fg-muted text-[0.85rem]">
            <p className="font-medium">No runs found</p>
            <p className="text-[0.78rem] mt-1">Try adjusting your filters or run a benchmark first.</p>
          </div>
        ) : (
          <div className="overflow-x-auto">
            <table className="w-full border-collapse">
              <thead>
                <tr className="border-b border-border">
                  <th className={thClass} onClick={() => handleSort("passed")}>
                    Status <SortArrow field="passed" sort={sort} />
                  </th>
                  <th className={thClass} onClick={() => handleSort("scenario_id")}>
                    Scenario <SortArrow field="scenario_id" sort={sort} />
                  </th>
                  <th className={thClass} onClick={() => handleSort("model")}>
                    Model <SortArrow field="model" sort={sort} />
                  </th>
                  <th className={thClass}>
                    Tool
                  </th>
                  <th className={thClass} onClick={() => handleSort("duration_seconds")}>
                    Duration <SortArrow field="duration_seconds" sort={sort} />
                  </th>
                  <th className={thClass} onClick={() => handleSort("turns")}>
                    Turns <SortArrow field="turns" sort={sort} />
                  </th>
                  <th className={thClass} onClick={() => handleSort("tokens")}>
                    Tokens <SortArrow field="tokens" sort={sort} />
                  </th>
                  <th className={thClass} onClick={() => handleSort("estimated_cost_usd")}>
                    Cost <SortArrow field="estimated_cost_usd" sort={sort} />
                  </th>
                  <th className={thClass} onClick={() => handleSort("checks")}>
                    Checks <SortArrow field="checks" sort={sort} />
                  </th>
                  <th className={thClass} onClick={() => handleSort("created_at")}>
                    Date <SortArrow field="created_at" sort={sort} />
                  </th>
                </tr>
              </thead>
              <tbody>
                {sorted.map((run) => (
                  <tr
                    key={run.id}
                    onClick={() => navigate(benchRunPath(run.id))}
                    className="border-b border-border-subtle cursor-pointer hover:bg-accent-subtle transition-colors"
                  >
                    <td className="px-3 py-2.5">
                      {run.passed ? (
                        <span className="bg-accent-tint text-accent font-mono text-[0.72rem] font-semibold px-2 py-0.5 rounded">
                          PASS
                        </span>
                      ) : (
                        <span className="bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)] font-mono text-[0.72rem] font-semibold px-2 py-0.5 rounded">
                          FAIL
                        </span>
                      )}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-body font-medium">
                      {run.scenario_id}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-body">{run.model}</td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-muted">
                      <span className="block">{run.tool_server || "baseline"}</span>
                      {run.tool_server_version && (
                        <span className="block text-[0.7rem] text-fg-muted/80">{run.tool_server_version}</span>
                      )}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-muted">
                      {formatDuration(run.duration_seconds)}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-muted text-center">
                      {run.turns}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-muted">
                      {formatTokens(run.prompt_tokens + run.completion_tokens)}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-muted">
                      {formatCost(run.estimated_cost_usd)}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-muted">
                      {run.checks_passed}/{run.checks_total}
                    </td>
                    <td className="px-3 py-2.5 font-mono text-[0.78rem] text-fg-muted whitespace-nowrap">
                      {formatDate(run.created_at)}
                    </td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        )}
      </div>

      {/* Pagination */}
      {!loading && total > 0 && (
        <div className="flex items-center justify-between mt-4">
          <span className="text-[0.78rem] text-fg-muted">
            Showing {rangeStart}&ndash;{rangeEnd} of {total} runs
          </span>
          <div className="flex gap-1">
            <button
              disabled={page === 0}
              onClick={() => setPage((p) => Math.max(0, p - 1))}
              className="text-[0.76rem] px-2.5 py-1 border border-border rounded bg-bg-elevated text-fg-muted cursor-pointer disabled:opacity-40 disabled:cursor-default hover:border-accent transition-colors"
            >
              Prev
            </button>
            {Array.from({ length: totalPages }, (_, i) => (
              <button
                key={i}
                onClick={() => setPage(i)}
                className={`text-[0.76rem] px-2.5 py-1 border rounded cursor-pointer transition-colors ${
                  i === page
                    ? "bg-accent-tint text-accent border-accent"
                    : "bg-bg-elevated text-fg-muted border-border hover:border-accent"
                }`}
              >
                {i + 1}
              </button>
            ))}
            <button
              disabled={page >= totalPages - 1}
              onClick={() => setPage((p) => Math.min(totalPages - 1, p + 1))}
              className="text-[0.76rem] px-2.5 py-1 border border-border rounded bg-bg-elevated text-fg-muted cursor-pointer disabled:opacity-40 disabled:cursor-default hover:border-accent transition-colors"
            >
              Next
            </button>
          </div>
        </div>
      )}
    </div>
  );
}
