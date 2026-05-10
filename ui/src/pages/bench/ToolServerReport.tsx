import { useEffect, useMemo, useState } from "react";
import type { ReactNode } from "react";
import { Link, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import { normalizeCatalog, type CatalogResponse } from "../../lib/catalogData.mts";
import {
  categoriesFromScenarios,
  scenarioIdsForCategory,
  type ScenarioCategoryRecord,
} from "../../lib/compareData.mts";
import {
  buildToolServerComparePath,
  toolServerRunsPagePath,
} from "../../lib/toolServerCompare.mts";

interface ToolServerAggregate {
  runs: number;
  passed: number;
  pass_rate: number;
  avg_turns: number;
  avg_tokens: number;
  avg_cost_usd: number;
  avg_duration_seconds: number;
}

interface ToolServerDelta {
  pass_rate_delta: number;
  avg_turns_delta: number;
  avg_tokens_delta: number;
  avg_cost_usd_delta: number;
  avg_duration_seconds_delta: number;
}

interface ToolServerScenarioComparison {
  scenario_id: string;
  baseline: ToolServerAggregate;
  candidate: ToolServerAggregate;
  delta: ToolServerDelta;
}

interface ToolServerComparisonResponse {
  model: string;
  tool_server: string;
  tool_server_version?: string;
  scenario_ids?: string[];
  baseline: ToolServerAggregate;
  candidate: ToolServerAggregate;
  delta: ToolServerDelta;
  scenarios: ToolServerScenarioComparison[];
  improved_scenarios: ToolServerScenarioComparison[];
  regressed_scenarios: ToolServerScenarioComparison[];
}

interface ScenariosResponse {
  scenarios?: ScenarioCategoryRecord[];
  items?: ScenarioCategoryRecord[];
}

function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`;
}

function formatSignedPercent(value: number): string {
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(1)}pp`;
}

function formatSignedNumber(value: number, digits = 1): string {
  const sign = value > 0 ? "+" : "";
  return `${sign}${value.toFixed(digits)}`;
}

function formatTokens(value: number): string {
  if (Math.abs(value) >= 1000) {
    return `${(value / 1000).toFixed(1)}k`;
  }
  return value.toFixed(0);
}

function formatCost(value: number): string {
  return Math.abs(value) < 0.01 ? `$${value.toFixed(4)}` : `$${value.toFixed(2)}`;
}

function formatSignedCost(value: number): string {
  const sign = value > 0 ? "+" : value < 0 ? "-" : "";
  return `${sign}${formatCost(Math.abs(value))}`;
}

function formatDuration(value: number): string {
  return `${value.toFixed(1)}s`;
}

function deltaClass(value: number, lowerIsBetter = false): string {
  if (value === 0) return "text-fg";
  const improved = lowerIsBetter ? value < 0 : value > 0;
  return improved ? "text-accent" : "text-danger";
}

function scenarioStatus(row: ToolServerScenarioComparison): "improved" | "regressed" | "same" | "missing" {
  if (row.baseline.runs === 0 || row.candidate.runs === 0) return "missing";
  if (row.delta.pass_rate_delta > 0) return "improved";
  if (row.delta.pass_rate_delta < 0) return "regressed";
  return "same";
}

function statusBadge(status: ReturnType<typeof scenarioStatus>) {
  const classes = {
    improved: "bg-accent-tint text-accent",
    regressed: "bg-danger-tint text-danger",
    same: "bg-bg-elevated text-fg-muted",
    missing: "bg-warning-tint text-warning",
  };
  return (
    <span className={`inline-flex px-2 py-0.5 rounded text-[0.7rem] font-semibold ${classes[status]}`}>
      {status}
    </span>
  );
}

function metricLine(label: string, value: string, className = "text-fg") {
  return (
    <div className="flex items-center justify-between gap-4 text-[0.8rem]">
      <span className="text-fg-muted">{label}</span>
      <span className={`font-mono font-semibold text-right ${className}`}>{value}</span>
    </div>
  );
}

function summaryCard(title: string, primary: string, detail: string, children?: ReactNode) {
  return (
    <div className="glass-card p-4">
      <p className="text-[0.68rem] uppercase tracking-wide font-semibold text-fg-muted mb-2">{title}</p>
      <div className="flex items-baseline gap-2">
        <span className="text-2xl font-bold text-fg leading-none">{primary}</span>
        <span className="text-[0.78rem] text-fg-muted">{detail}</span>
      </div>
      {children && <div className="mt-4 space-y-1.5">{children}</div>}
    </div>
  );
}

function aggregateDetail(aggregate: ToolServerAggregate) {
  return (
    <>
      {metricLine("Turns", aggregate.avg_turns.toFixed(1))}
      {metricLine("Tokens", formatTokens(aggregate.avg_tokens))}
      {metricLine("Duration", formatDuration(aggregate.avg_duration_seconds))}
      {metricLine("Cost", formatCost(aggregate.avg_cost_usd))}
    </>
  );
}

export function ToolServerReport() {
  usePageTitle("MCP Readiness");
  const { request } = useApi();
  const [searchParams, setSearchParams] = useSearchParams();
  const [catalog, setCatalog] = useState<CatalogResponse>({
    models: [],
    providers: [],
    tool_servers: [],
    tool_server_versions: [],
  });
  const [scenarios, setScenarios] = useState<ScenarioCategoryRecord[]>([]);
  const [model, setModel] = useState(searchParams.get("model") ?? "");
  const [toolServer, setToolServer] = useState(searchParams.get("tool_server") ?? "");
  const [toolServerVersion, setToolServerVersion] = useState(searchParams.get("tool_server_version") ?? "");
  const [category, setCategory] = useState(searchParams.get("category") ?? "");
  const [scenario, setScenario] = useState(searchParams.get("scenario") ?? "");
  const [data, setData] = useState<ToolServerComparisonResponse | null>(null);
  const [loadingCatalog, setLoadingCatalog] = useState(true);
  const [loadingReport, setLoadingReport] = useState(false);
  const [error, setError] = useState<string | null>(null);

  useEffect(() => {
    setLoadingCatalog(true);
    Promise.all([
      request<CatalogResponse>("/v1/bench/catalog"),
      request<ScenariosResponse>("/v1/bench/scenarios"),
    ])
      .then(([catalogRes, scenariosRes]) => {
        const normalized = normalizeCatalog(catalogRes);
        setCatalog(normalized);
        setScenarios(scenariosRes.scenarios ?? scenariosRes.items ?? []);
        setModel((current) => current || normalized.models[0] || "");
        setToolServer((current) => current || normalized.tool_servers[0] || "");
      })
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load report filters"))
      .finally(() => setLoadingCatalog(false));
  }, [request]);

  const categories = useMemo(() => categoriesFromScenarios(scenarios), [scenarios]);
  const selectedScenarioIDs = useMemo(() => {
    if (scenario) return [scenario];
    return scenarioIdsForCategory(scenarios, category);
  }, [category, scenario, scenarios]);

  useEffect(() => {
    if (!model || !toolServer) return;
    setLoadingReport(true);
    setError(null);
    request<ToolServerComparisonResponse>(
      buildToolServerComparePath({
        model,
        toolServer,
        toolServerVersion,
        scenarioIds: selectedScenarioIDs,
      }),
    )
      .then(setData)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load comparison"))
      .finally(() => setLoadingReport(false));
  }, [model, request, selectedScenarioIDs, toolServer, toolServerVersion]);

  function updateFilters(next: Partial<{
    model: string;
    toolServer: string;
    toolServerVersion: string;
    category: string;
    scenario: string;
  }>) {
    const nextModel = next.model ?? model;
    const nextToolServer = next.toolServer ?? toolServer;
    const nextVersion = next.toolServerVersion ?? toolServerVersion;
    const nextCategory = next.category ?? category;
    const nextScenario = next.scenario ?? scenario;

    setModel(nextModel);
    setToolServer(nextToolServer);
    setToolServerVersion(nextVersion);
    setCategory(nextCategory);
    setScenario(nextScenario);

    const params = new URLSearchParams();
    if (nextModel) params.set("model", nextModel);
    if (nextToolServer) params.set("tool_server", nextToolServer);
    if (nextVersion) params.set("tool_server_version", nextVersion);
    if (nextCategory) params.set("category", nextCategory);
    if (nextScenario) params.set("scenario", nextScenario);
    setSearchParams(params, { replace: true });
  }

  const rows = useMemo(() => {
    return [...(data?.scenarios ?? [])].sort((a, b) => {
      const statusA = scenarioStatus(a);
      const statusB = scenarioStatus(b);
      const weight = { regressed: 0, improved: 1, same: 2, missing: 3 };
      if (weight[statusA] !== weight[statusB]) return weight[statusA] - weight[statusB];
      if (a.delta.pass_rate_delta !== b.delta.pass_rate_delta) {
        return a.delta.pass_rate_delta - b.delta.pass_rate_delta;
      }
      return a.scenario_id.localeCompare(b.scenario_id);
    });
  }, [data]);

  const canLoadReport = model && toolServer;
  const emptyReport = data && data.baseline.runs === 0 && data.candidate.runs === 0;

  return (
    <div className="space-y-5">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h1 className="text-xl font-bold text-fg">MCP Readiness</h1>
          <p className="text-[0.84rem] text-fg-muted">
            {model && toolServer ? (
              <>
                <span className="font-mono text-fg">{model}</span>
                <span> through </span>
                <span className="font-mono text-fg">{toolServer}</span>
                {toolServerVersion && <span className="font-mono"> / {toolServerVersion}</span>}
              </>
            ) : (
              "Select a model and MCP server"
            )}
          </p>
        </div>
        {data && (
          <div className="flex gap-2 text-[0.76rem] text-fg-muted">
            <span>{data.scenarios.length} scenarios</span>
            <span>&middot;</span>
            <span>{data.improved_scenarios.length} improved</span>
            <span>&middot;</span>
            <span>{data.regressed_scenarios.length} regressed</span>
          </div>
        )}
      </div>

      <div className="glass-card p-4">
        <div className="grid grid-cols-1 sm:grid-cols-2 xl:grid-cols-5 gap-3">
          <label className="flex flex-col gap-1">
            <span className="text-[0.68rem] uppercase tracking-wide font-semibold text-fg-muted">Model</span>
            <select
              value={model}
              onChange={(e) => updateFilters({ model: e.target.value })}
              className="text-[0.82rem] px-2 py-1.5 rounded-md border border-border bg-bg-elevated text-fg"
              disabled={loadingCatalog}
            >
              {catalog.models.map((item) => (
                <option key={item} value={item}>{item}</option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-[0.68rem] uppercase tracking-wide font-semibold text-fg-muted">Tool Server</span>
            <select
              value={toolServer}
              onChange={(e) => updateFilters({ toolServer: e.target.value })}
              className="text-[0.82rem] px-2 py-1.5 rounded-md border border-border bg-bg-elevated text-fg"
              disabled={loadingCatalog}
            >
              {catalog.tool_servers.map((item) => (
                <option key={item} value={item}>{item}</option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-[0.68rem] uppercase tracking-wide font-semibold text-fg-muted">Version</span>
            <select
              value={toolServerVersion}
              onChange={(e) => updateFilters({ toolServerVersion: e.target.value })}
              className="text-[0.82rem] px-2 py-1.5 rounded-md border border-border bg-bg-elevated text-fg"
              disabled={loadingCatalog}
            >
              <option value="">All versions</option>
              {(catalog.tool_server_versions ?? []).map((item) => (
                <option key={item} value={item}>{item}</option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-[0.68rem] uppercase tracking-wide font-semibold text-fg-muted">Category</span>
            <select
              value={category}
              onChange={(e) => updateFilters({ category: e.target.value, scenario: "" })}
              className="text-[0.82rem] px-2 py-1.5 rounded-md border border-border bg-bg-elevated text-fg"
              disabled={loadingCatalog}
            >
              <option value="">All categories</option>
              {categories.map((item) => (
                <option key={item} value={item}>{item}</option>
              ))}
            </select>
          </label>

          <label className="flex flex-col gap-1">
            <span className="text-[0.68rem] uppercase tracking-wide font-semibold text-fg-muted">Scenario</span>
            <select
              value={scenario}
              onChange={(e) => updateFilters({ scenario: e.target.value })}
              className="text-[0.82rem] px-2 py-1.5 rounded-md border border-border bg-bg-elevated text-fg"
              disabled={loadingCatalog}
            >
              <option value="">Scenario slice</option>
              {scenarios.map((item) => (
                <option key={item.id} value={item.id}>{item.id}</option>
              ))}
            </select>
          </label>
        </div>
      </div>

      {error && <p className="text-danger text-sm">Error: {error}</p>}

      {loadingCatalog || loadingReport ? (
        <p className="text-fg-muted text-sm py-8 text-center">Loading MCP readiness report...</p>
      ) : !canLoadReport ? (
        <p className="text-fg-muted text-sm py-8 text-center">No model or MCP server catalog data yet.</p>
      ) : emptyReport ? (
        <p className="text-fg-muted text-sm py-8 text-center">No matching baseline or MCP runs found.</p>
      ) : data ? (
        <>
          <div className="grid grid-cols-1 md:grid-cols-2 xl:grid-cols-4 gap-4">
            {summaryCard(
              "Baseline",
              formatPercent(data.baseline.pass_rate),
              `${data.baseline.passed}/${data.baseline.runs} passed`,
              aggregateDetail(data.baseline),
            )}
            {summaryCard(
              "Candidate",
              formatPercent(data.candidate.pass_rate),
              `${data.candidate.passed}/${data.candidate.runs} passed`,
              aggregateDetail(data.candidate),
            )}
            {summaryCard(
              "Pass Delta",
              formatSignedPercent(data.delta.pass_rate_delta),
              `${data.improved_scenarios.length} up / ${data.regressed_scenarios.length} down`,
              <>
                {metricLine("Turns", formatSignedNumber(data.delta.avg_turns_delta), deltaClass(data.delta.avg_turns_delta, true))}
                {metricLine("Tokens", formatSignedNumber(data.delta.avg_tokens_delta, 0), deltaClass(data.delta.avg_tokens_delta, true))}
                {metricLine("Duration", `${formatSignedNumber(data.delta.avg_duration_seconds_delta)}s`, deltaClass(data.delta.avg_duration_seconds_delta, true))}
                {metricLine("Cost", formatSignedCost(data.delta.avg_cost_usd_delta), deltaClass(data.delta.avg_cost_usd_delta, true))}
              </>,
            )}
            {summaryCard(
              "Coverage",
              String(data.scenarios.length),
              selectedScenarioIDs.length > 0 ? "selected scenarios" : "observed scenarios",
              <>
                {metricLine("Baseline runs", String(data.baseline.runs))}
                {metricLine("Candidate runs", String(data.candidate.runs))}
                {metricLine("Missing sides", String(data.scenarios.filter((row) => scenarioStatus(row) === "missing").length))}
              </>,
            )}
          </div>

          <div className="overflow-x-auto rounded-[10px] border border-border-subtle">
            <table className="w-full border-collapse text-[0.82rem]">
              <thead>
                <tr className="border-b border-border-subtle bg-bg-elevated">
                  <th className="text-left px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">Scenario</th>
                  <th className="text-center px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">Status</th>
                  <th className="text-right px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">Baseline</th>
                  <th className="text-right px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">Candidate</th>
                  <th className="text-right px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">Delta</th>
                  <th className="text-right px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">Effort Delta</th>
                  <th className="text-right px-3 py-2.5 text-[0.7rem] uppercase tracking-wide text-fg-muted font-semibold">Runs</th>
                </tr>
              </thead>
              <tbody>
                {rows.map((row) => {
                  const status = scenarioStatus(row);
                  return (
                    <tr key={row.scenario_id} className="border-b border-border-subtle last:border-b-0">
                      <td className="px-3 py-2 font-mono text-fg whitespace-nowrap">{row.scenario_id}</td>
                      <td className="px-3 py-2 text-center">{statusBadge(status)}</td>
                      <td className="px-3 py-2 text-right font-mono text-fg">
                        {formatPercent(row.baseline.pass_rate)}
                        <span className="block text-[0.68rem] text-fg-muted">{row.baseline.passed}/{row.baseline.runs}</span>
                      </td>
                      <td className="px-3 py-2 text-right font-mono text-fg">
                        {formatPercent(row.candidate.pass_rate)}
                        <span className="block text-[0.68rem] text-fg-muted">{row.candidate.passed}/{row.candidate.runs}</span>
                      </td>
                      <td className={`px-3 py-2 text-right font-mono font-semibold ${deltaClass(row.delta.pass_rate_delta)}`}>
                        {formatSignedPercent(row.delta.pass_rate_delta)}
                      </td>
                      <td className="px-3 py-2 text-right font-mono text-[0.74rem]">
                        <span className={deltaClass(row.delta.avg_turns_delta, true)}>
                          {formatSignedNumber(row.delta.avg_turns_delta)} turns
                        </span>
                        <span className={`block ${deltaClass(row.delta.avg_tokens_delta, true)}`}>
                          {formatSignedNumber(row.delta.avg_tokens_delta, 0)} tokens
                        </span>
                      </td>
                      <td className="px-3 py-2 text-right whitespace-nowrap">
                        <Link
                          to={toolServerRunsPagePath({
                            side: "baseline",
                            model: data.model,
                            scenarioId: row.scenario_id,
                            toolServer: data.tool_server,
                            toolServerVersion: data.tool_server_version,
                          })}
                          className="text-[0.74rem] hover:underline"
                        >
                          baseline
                        </Link>
                        <span className="text-fg-muted px-1">/</span>
                        <Link
                          to={toolServerRunsPagePath({
                            side: "candidate",
                            model: data.model,
                            scenarioId: row.scenario_id,
                            toolServer: data.tool_server,
                            toolServerVersion: data.tool_server_version,
                          })}
                          className="text-[0.74rem] hover:underline"
                        >
                          candidate
                        </Link>
                      </td>
                    </tr>
                  );
                })}
              </tbody>
            </table>
          </div>
        </>
      ) : null}
    </div>
  );
}
