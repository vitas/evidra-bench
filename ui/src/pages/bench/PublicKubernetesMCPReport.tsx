import { useEffect, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import { BENCH_MCP_READINESS_PATH, BENCH_RUNS_PATH } from "../../lib/routes.mts";
import {
  buildToolServerMatrixReportApiPath,
  type ToolServerAggregate,
  type ToolServerMatrixReportResponse,
  type ToolServerMatrixScenarioArm,
} from "../../lib/toolServerMatrixReport.mts";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

const DEFAULT_PUBLIC_REPORT_FILTERS = {
  model: "sonnet",
  reportId: "kubernetes-mcp-readiness-2026-05",
  toolServers: ["flux159-mcp-server-kubernetes", "containers-kubernetes-mcp-server"],
};

function parseCSVParam(value: string | null): string[] {
  if (!value) return [];
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

function formatPercent(value: number): string {
  return `${value.toFixed(1)}%`;
}

function formatTokens(value: number): string {
  if (Math.abs(value) >= 1000) return `${(value / 1000).toFixed(1)}k`;
  return value.toFixed(0);
}

function formatCost(value: number): string {
  return Math.abs(value) < 0.01 ? `$${value.toFixed(4)}` : `$${value.toFixed(2)}`;
}

function formatDuration(value: number): string {
  return `${value.toFixed(1)}s`;
}

function classificationLabel(value: string): string {
  const labels: Record<string, string> = {
    baseline: "Baseline",
    safe_pass: "Safe pass",
    unsafe_pass: "Unsafe pass",
    fail: "Fail",
    missing_evidence: "Missing evidence",
  };
  return labels[value] ?? value;
}

function classificationClass(value: string): string {
  if (value === "baseline") return "bg-bg-alt text-fg-muted";
  if (value === "safe_pass") return "bg-accent-tint text-accent";
  if (value === "unsafe_pass") return "bg-warning-tint text-warning";
  if (value === "missing_evidence") return "bg-bg-elevated text-fg-muted";
  return "bg-danger-tint text-danger";
}

function metricLine(label: string, value: string) {
  return (
    <div className="flex items-center justify-between gap-4 text-[0.78rem]">
      <span className="text-fg-muted">{label}</span>
      <span className="font-mono font-semibold text-fg">{value}</span>
    </div>
  );
}

function summaryCard(title: string, value: string, detail: string, tone = "text-fg") {
  return (
    <div className="glass-card p-4">
      <p className="mb-2 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">{title}</p>
      <div className="flex items-baseline gap-2">
        <span className={`text-2xl font-bold leading-none ${tone}`}>{value}</span>
        <span className="text-[0.78rem] text-fg-muted">{detail}</span>
      </div>
    </div>
  );
}

function aggregateDetails(aggregate: ToolServerAggregate) {
  return (
    <div className="mt-3 space-y-1.5">
      {metricLine("Turns", aggregate.avg_turns.toFixed(1))}
      {metricLine("Tokens", formatTokens(aggregate.avg_tokens))}
      {metricLine("Duration", formatDuration(aggregate.avg_duration_seconds))}
      {metricLine("Cost", formatCost(aggregate.avg_cost_usd))}
    </div>
  );
}

function resultCell(arm: ToolServerMatrixScenarioArm) {
  return (
    <div className="min-w-[150px] space-y-2">
      <span className={`inline-flex rounded px-2 py-0.5 text-[0.7rem] font-semibold ${classificationClass(arm.classification)}`}>
        {classificationLabel(arm.classification)}
      </span>
      <div className="space-y-1 text-[0.76rem] text-fg-muted">
        <div>
          <span className="font-mono text-fg">{formatPercent(arm.aggregate.pass_rate)}</span>
          <span> pass rate</span>
        </div>
        <div>
          <span className="font-mono text-fg">{arm.aggregate.runs}</span>
          <span> runs</span>
        </div>
        {arm.run_id && (
          <Link to={`${BENCH_RUNS_PATH}/${encodeURIComponent(arm.run_id)}`} className="font-semibold text-accent hover:underline">
            Evidence
          </Link>
        )}
      </div>
    </div>
  );
}

export function PublicKubernetesMCPReport() {
  usePageTitle("Kubernetes MCP Report");
  const { request } = useApi();
  const { reportId: routeReportId } = useParams();
  const [searchParams] = useSearchParams();
  const [report, setReport] = useState<ToolServerMatrixReportResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const searchKey = searchParams.toString();
  const reportFilters = useMemo(() => {
    const scenarioIds = parseCSVParam(searchParams.get("scenarios") ?? searchParams.get("scenario"));
    const toolServerVersions = parseCSVParam(searchParams.get("tool_server_versions"));
    return {
      model: searchParams.get("model") || DEFAULT_PUBLIC_REPORT_FILTERS.model,
      reportId: searchParams.get("report_id") || routeReportId || DEFAULT_PUBLIC_REPORT_FILTERS.reportId,
      toolServers: parseCSVParam(searchParams.get("tool_servers")).length > 0
        ? parseCSVParam(searchParams.get("tool_servers"))
        : DEFAULT_PUBLIC_REPORT_FILTERS.toolServers,
      toolServerVersions: toolServerVersions.length > 0 ? toolServerVersions : undefined,
      scenarioIds: scenarioIds.length > 0 ? scenarioIds : undefined,
    };
  }, [routeReportId, searchKey, searchParams]);

  const apiPath = useMemo(() => buildToolServerMatrixReportApiPath(reportFilters), [reportFilters]);
  const markdownURL = useMemo(
    () => `${API_BASE}${buildToolServerMatrixReportApiPath({ ...reportFilters, format: "markdown" })}`,
    [reportFilters],
  );
  const rawJSONURL = `${API_BASE}${apiPath}`;

  useEffect(() => {
    setLoading(true);
    setError(null);
    request<ToolServerMatrixReportResponse>(apiPath)
      .then(setReport)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load report"))
      .finally(() => setLoading(false));
  }, [apiPath, request]);

  if (loading && !report) {
    return <p className="py-8 text-center text-sm text-fg-muted">Loading public report...</p>;
  }

  if (error) {
    return (
      <div className="rounded-lg border border-danger/30 bg-danger-tint p-6 text-sm text-danger">
        Failed to load: {error}
      </div>
    );
  }

  if (!report) return null;

  const generatedAt = report.generated_at ? new Date(report.generated_at).toLocaleString() : "";
  const arms = report.arms ?? [];
  const candidateArms = arms.filter((arm) => arm.kind !== "baseline");
  const scenarios = report.scenarios ?? [];
  const methodology = report.methodology ?? [];
  const autopsies = report.autopsies ?? [];
  const findings = report.findings ?? [];
  const recommendations = report.recommendations ?? [];
  const evidenceLinks = report.evidence_links ?? [];

  return (
    <article className="space-y-8">
      <header className="space-y-5">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <div className="mb-3 inline-flex rounded-md border border-border bg-bg-elevated px-2.5 py-1 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">
              Public benchmark report
            </div>
            <h1 className="text-3xl font-extrabold tracking-tight text-fg md:text-4xl">
              {report.title}
            </h1>
            <p className="mt-3 max-w-3xl text-sm leading-relaxed text-fg-muted">
              {report.model} across native tools and {candidateArms.length} Kubernetes MCP server candidates.
            </p>
          </div>
          <div className="flex flex-wrap gap-2">
            <a
              href={markdownURL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex rounded-md bg-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-accent-bright"
            >
              Markdown
            </a>
            <a
              href={rawJSONURL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex rounded-md border border-border px-4 py-2 text-sm font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
            >
              Raw JSON
            </a>
          </div>
        </div>

        <div className="grid gap-3 text-[0.78rem] text-fg-muted sm:grid-cols-3">
          <div className="rounded-md border border-border-subtle bg-bg-elevated px-3 py-2">
            <span className="block text-[0.65rem] font-semibold uppercase tracking-wide">Report ID</span>
            <span className="font-mono text-fg">{report.report_id}</span>
          </div>
          <div className="rounded-md border border-border-subtle bg-bg-elevated px-3 py-2">
            <span className="block text-[0.65rem] font-semibold uppercase tracking-wide">Model</span>
            <span className="font-mono text-fg">{report.model}</span>
          </div>
          <div className="rounded-md border border-border-subtle bg-bg-elevated px-3 py-2">
            <span className="block text-[0.65rem] font-semibold uppercase tracking-wide">Generated</span>
            <span className="text-fg">{generatedAt || "Pending data"}</span>
          </div>
        </div>
      </header>

      <section className="grid gap-3 sm:grid-cols-2 xl:grid-cols-4">
        {summaryCard("Safe pass", String(report.summary.safe_pass), `${report.summary.candidate_cells} candidate cells`, "text-accent")}
        {summaryCard("Unsafe pass", String(report.summary.unsafe_pass), "requires autopsy review", "text-warning")}
        {summaryCard("Fail", String(report.summary.fail), "final state failed", "text-danger")}
        {summaryCard("Missing evidence", String(report.summary.missing_evidence), `${report.summary.total_scenarios} scenarios`, "text-fg-muted")}
      </section>

      <section className="grid gap-4 xl:grid-cols-[1.1fr_0.9fr]">
        <div className="glass-card p-5">
          <h2 className="mb-4 text-lg font-bold text-fg">Tested Configuration</h2>
          <div className="grid gap-3 md:grid-cols-3">
            {arms.map((arm) => (
              <div key={arm.id} className="rounded-md border border-border-subtle bg-bg-elevated p-4">
                <div className="mb-2 flex items-start justify-between gap-3">
                  <div>
                    <p className="text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">{arm.kind}</p>
                    <p className="break-words font-mono text-sm font-semibold text-fg">{arm.label}</p>
                  </div>
                  <span className="rounded bg-bg-alt px-2 py-0.5 text-[0.68rem] font-semibold text-fg-muted">
                    {formatPercent(arm.aggregate.pass_rate)}
                  </span>
                </div>
                {arm.tool_server_version && (
                  <p className="mb-2 break-words font-mono text-[0.72rem] text-fg-muted">{arm.tool_server_version}</p>
                )}
                {aggregateDetails(arm.aggregate)}
              </div>
            ))}
          </div>
        </div>

        <div className="glass-card p-5">
          <h2 className="mb-4 text-lg font-bold text-fg">Methodology</h2>
          <ul className="space-y-3 text-sm leading-relaxed text-fg-muted">
            {methodology.map((item) => (
              <li key={item} className="border-l-2 border-border pl-3">{item}</li>
            ))}
          </ul>
        </div>
      </section>

      <section className="space-y-4">
        <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
          <div>
            <h2 className="text-xl font-bold text-fg">Results Matrix</h2>
            <p className="text-sm text-fg-muted">{scenarios.length} live scenarios across baseline and candidate arms.</p>
          </div>
          <Link to={BENCH_MCP_READINESS_PATH} className="text-sm font-semibold text-accent hover:underline">
            Compare another MCP server
          </Link>
        </div>

        <div className="overflow-x-auto rounded-lg border border-border bg-bg-elevated">
          <table className="w-full min-w-[980px] text-sm">
            <thead className="bg-bg-alt text-fg-muted">
              <tr>
                <th className="w-[240px] px-4 py-3 text-left text-[0.7rem] font-semibold uppercase tracking-wide">
                  Scenario
                </th>
                {arms.map((arm) => (
                  <th key={arm.id} className="px-4 py-3 text-left text-[0.7rem] font-semibold uppercase tracking-wide">
                    {arm.label}
                  </th>
                ))}
              </tr>
            </thead>
            <tbody>
              {scenarios.map((scenario) => (
                <tr key={scenario.id} className="border-t border-border-subtle align-top">
                  <td className="px-4 py-4">
                    <div className="font-mono text-sm font-semibold text-fg">{scenario.id}</div>
                    {scenario.title && <div className="mt-1 text-[0.78rem] text-fg-muted">{scenario.title}</div>}
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {scenario.category && <span className="rounded bg-bg-alt px-2 py-0.5 text-[0.68rem] text-fg-muted">{scenario.category}</span>}
                      {scenario.level && <span className="rounded bg-bg-alt px-2 py-0.5 text-[0.68rem] text-fg-muted">{scenario.level}</span>}
                    </div>
                  </td>
                  {scenario.arms.map((arm) => (
                    <td key={`${scenario.id}-${arm.arm_id}`} className="px-4 py-4">
                      {resultCell(arm)}
                    </td>
                  ))}
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      </section>

      <section className="grid gap-4 xl:grid-cols-2">
        <div className="glass-card p-5">
          <h2 className="mb-4 text-lg font-bold text-fg">Top Findings</h2>
          <ul className="space-y-3 text-sm leading-relaxed text-fg-muted">
            {findings.map((item) => (
              <li key={item} className="border-l-2 border-border pl-3">{item}</li>
            ))}
          </ul>
        </div>

        <div className="glass-card p-5">
          <h2 className="mb-4 text-lg font-bold text-fg">Recommendations</h2>
          <ul className="space-y-3 text-sm leading-relaxed text-fg-muted">
            {recommendations.map((item) => (
              <li key={item} className="border-l-2 border-border pl-3">{item}</li>
            ))}
          </ul>
        </div>
      </section>

      {autopsies.length > 0 && (
        <section className="space-y-4">
          <h2 className="text-xl font-bold text-fg">Failure Autopsy Highlights</h2>
          <div className="grid gap-3 lg:grid-cols-2">
            {autopsies.map((autopsy) => (
              <div key={`${autopsy.tool_server}-${autopsy.scenario_id}-${autopsy.run_id}`} className="rounded-lg border border-border bg-bg-elevated p-4">
                <div className="mb-2 flex flex-wrap items-center gap-2">
                  <span className="font-mono text-sm font-semibold text-fg">{autopsy.scenario_id}</span>
                  <span className="rounded bg-warning-tint px-2 py-0.5 text-[0.68rem] font-semibold text-warning">
                    {autopsy.primary_failure || "autopsy"}
                  </span>
                </div>
                <p className="text-sm leading-relaxed text-fg-muted">{autopsy.summary}</p>
                <p className="mt-3 break-words font-mono text-[0.72rem] text-fg-muted">{autopsy.tool_server}</p>
              </div>
            ))}
          </div>
        </section>
      )}

      {evidenceLinks.length > 0 && (
        <section className="space-y-4">
          <h2 className="text-xl font-bold text-fg">Raw Evidence</h2>
          <div className="grid gap-2 sm:grid-cols-2 lg:grid-cols-3">
            {evidenceLinks.slice(0, 18).map((link, index) => (
              <a
                key={`${link.url}-${index}`}
                href={link.url}
                className="rounded-md border border-border bg-bg-elevated px-3 py-2 text-sm font-semibold text-accent hover:border-accent/50"
              >
                {link.label}
              </a>
            ))}
          </div>
        </section>
      )}
    </article>
  );
}
