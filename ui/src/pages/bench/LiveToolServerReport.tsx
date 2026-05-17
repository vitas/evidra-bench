import { type ReactNode, useEffect, useMemo, useState } from "react";
import { Link, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import {
  BENCH_MCP_READINESS_PATH,
  BENCH_RUNS_PATH,
  BENCH_SCENARIOS_PATH,
} from "../../lib/routes.mts";
import { buildToolServerReportApiPath } from "../../lib/toolServerCompare.mts";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

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

interface ToolServerComparison {
  baseline: ToolServerAggregate;
  candidate: ToolServerAggregate;
  delta: ToolServerDelta;
}

interface ReportConfiguration {
  report_type: string;
  model: string;
  provider?: string;
  tool_server: string;
  tool_server_version?: string;
  scenario_slice: string;
}

interface ReportSummary {
  total_scenarios: number;
  safe_pass: number;
  unsafe_pass: number;
  fail: number;
  missing_evidence: number;
  safe_pass_rate: number;
  final_state_pass_rate: number;
}

interface ReportEvidenceLink {
  label: string;
  url: string;
}

interface ReportScenario {
  id: string;
  title?: string;
  category?: string;
  level?: string;
  autopsy_description?: string;
  classification: string;
  result: string;
  baseline: ToolServerAggregate;
  candidate: ToolServerAggregate;
  delta: ToolServerDelta;
  candidate_run_id?: string;
  evidence_links?: ReportEvidenceLink[];
}

interface ReportAutopsy {
  scenario_id: string;
  run_id?: string;
  primary_failure?: string;
  summary: string;
  missing?: boolean;
  findings?: Array<{
    kind: string;
    severity: string;
    message: string;
    evidence?: string;
  }>;
  evidence_links?: ReportEvidenceLink[];
}

interface ReportCostBucket {
  classification: string;
  scenarios: number;
  avg_turns: number;
  avg_tokens: number;
  avg_cost_usd: number;
  avg_duration_seconds: number;
}

interface ToolServerReportResponse {
  title: string;
  generated_at: string;
  verdict: string;
  executive_summary: string;
  configuration: ReportConfiguration;
  summary: ReportSummary;
  comparison: ToolServerComparison;
  scenarios: ReportScenario[];
  cost_buckets: ReportCostBucket[];
  autopsies: ReportAutopsy[];
  findings: string[];
  recommendations: string[];
  evidence_links: ReportEvidenceLink[];
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
    safe_pass: "Safe pass",
    unsafe_pass: "Unsafe pass",
    fail: "Fail",
    missing_evidence: "Missing evidence",
  };
  return labels[value] ?? value;
}

function classificationClass(value: string): string {
  if (value === "safe_pass") return "bg-accent-tint text-accent";
  if (value === "unsafe_pass") return "bg-warning-tint text-warning";
  if (value === "missing_evidence") return "bg-bg-elevated text-fg-muted";
  return "bg-danger-tint text-danger";
}

function deltaClass(value: number, lowerIsBetter = false): string {
  if (value === 0) return "text-fg";
  const improved = lowerIsBetter ? value < 0 : value > 0;
  return improved ? "text-accent" : "text-danger";
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
      <p className="mb-2 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">{title}</p>
      <div className="flex items-baseline gap-2">
        <span className="text-2xl font-bold leading-none text-fg">{primary}</span>
        <span className="text-[0.78rem] text-fg-muted">{detail}</span>
      </div>
      {children && <div className="mt-4 space-y-1.5">{children}</div>}
    </div>
  );
}

function ReportSection({ id, title, children }: { id: string; title: string; children: ReactNode }) {
  return (
    <section id={id} className="scroll-mt-28 border-t border-border-subtle pt-8">
      <h2 className="mb-4 text-xl font-bold text-fg">{title}</h2>
      {children}
    </section>
  );
}

function DataTable({ headers, children }: { headers: string[]; children: ReactNode }) {
  return (
    <div className="overflow-x-auto rounded-lg border border-border bg-bg-elevated">
      <table className="w-full text-sm">
        <thead className="bg-bg-alt text-fg-muted">
          <tr>
            {headers.map((header) => (
              <th key={header} className="px-4 py-3 text-left text-[0.7rem] font-semibold uppercase tracking-wide">
                {header}
              </th>
            ))}
          </tr>
        </thead>
        <tbody>{children}</tbody>
      </table>
    </div>
  );
}

function parseScenarioIDs(value: string | null): string[] {
  if (!value) return [];
  return value.split(",").map((item) => item.trim()).filter(Boolean);
}

export function LiveToolServerReport() {
  usePageTitle("Tool Server Report");
  const { request } = useApi();
  const [searchParams] = useSearchParams();
  const [report, setReport] = useState<ToolServerReportResponse | null>(null);
  const [loading, setLoading] = useState(false);
  const [error, setError] = useState<string | null>(null);

  const model = searchParams.get("model") ?? "";
  const toolServer = searchParams.get("tool_server") ?? "";
  const toolServerVersion = searchParams.get("tool_server_version") ?? "";
  const category = searchParams.get("category") ?? "";
  const scenarioIds = useMemo(() => parseScenarioIDs(searchParams.get("scenarios") ?? searchParams.get("scenario")), [searchParams]);

  const apiPath = useMemo(() => {
    if (!model || !toolServer) return "";
    return buildToolServerReportApiPath({
      model,
      toolServer,
      toolServerVersion,
      category,
      scenarioIds,
    });
  }, [category, model, scenarioIds, toolServer, toolServerVersion]);

  const markdownURL = useMemo(() => {
    if (!model || !toolServer) return "";
    return `${API_BASE}${buildToolServerReportApiPath({
      model,
      toolServer,
      toolServerVersion,
      category,
      scenarioIds,
      format: "markdown",
    })}`;
  }, [category, model, scenarioIds, toolServer, toolServerVersion]);

  useEffect(() => {
    if (!apiPath) return;
    setLoading(true);
    setError(null);
    request<ToolServerReportResponse>(apiPath)
      .then(setReport)
      .catch((e) => setError(e instanceof Error ? e.message : "Failed to load report"))
      .finally(() => setLoading(false));
  }, [apiPath, request]);

  if (!model || !toolServer) {
    return (
      <div className="rounded-lg border border-border bg-bg-elevated p-6 text-sm text-fg-muted">
        Select a model and MCP server from MCP Readiness before opening a live report.
        <div className="mt-4">
          <Link to={BENCH_MCP_READINESS_PATH} className="inline-flex rounded-md bg-accent px-4 py-2 text-sm font-semibold text-white">
            Open MCP readiness
          </Link>
        </div>
      </div>
    );
  }

  if (loading && !report) {
    return <p className="py-8 text-center text-sm text-fg-muted">Loading live report...</p>;
  }

  if (error) {
    return <p className="py-8 text-center text-sm text-danger">Error: {error}</p>;
  }

  if (!report) return null;

  const generatedAt = report.generated_at ? new Date(report.generated_at).toLocaleString() : "";
  const scenarios = report.scenarios ?? [];
  const autopsies = report.autopsies ?? [];
  const costBuckets = report.cost_buckets ?? [];
  const scenarioRulebooks = scenarios.filter((row) => row.autopsy_description?.trim());
  const findings = report.findings ?? [];
  const recommendations = report.recommendations ?? [];
  const evidenceLinks = report.evidence_links ?? [];

  return (
    <article className="space-y-10">
      <header className="grid items-start gap-6 lg:grid-cols-[1.2fr_0.8fr]">
        <div>
          <div className="mb-4 inline-flex items-center gap-2 rounded-full border border-accent/30 bg-accent/10 px-3 py-1 text-[0.68rem] font-semibold uppercase tracking-wider text-accent">
            Live deliverable
          </div>
          <h1 className="mb-4 text-3xl font-extrabold tracking-tight text-fg md:text-4xl">
            {report.title}
          </h1>
          <p className="max-w-3xl text-base leading-relaxed text-fg-muted">
            {report.executive_summary}
          </p>
          <div className="mt-5 rounded-md border border-warning/30 bg-warning-tint p-4">
            <div className="mb-1 text-sm font-semibold text-warning">Readiness verdict</div>
            <p className="text-sm leading-relaxed text-fg-body">{report.verdict}</p>
          </div>
          <div className="mt-6 flex flex-wrap gap-3">
            <a
              href={markdownURL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex rounded-md bg-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-accent-bright"
            >
              Markdown version
            </a>
            <Link
              to={BENCH_MCP_READINESS_PATH}
              className="inline-flex rounded-md border border-border px-4 py-2 text-sm font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
            >
              Back to readiness
            </Link>
          </div>
        </div>

        <div className="grid grid-cols-2 gap-3">
          {[
            ["Scenarios", String(report.summary.total_scenarios)],
            ["Safe pass", String(report.summary.safe_pass)],
            ["Unsafe pass", String(report.summary.unsafe_pass)],
            ["Fail", String(report.summary.fail)],
            ["Safe pass rate", formatPercent(report.summary.safe_pass_rate)],
            ["Final-state pass", formatPercent(report.summary.final_state_pass_rate)],
          ].map(([label, value]) => (
            <div key={label} className="rounded-lg border border-border bg-bg-elevated p-4">
              <div className="text-2xl font-bold text-fg">{value}</div>
              <div className="mt-1 text-[0.68rem] uppercase tracking-wide text-fg-muted">{label}</div>
            </div>
          ))}
        </div>
      </header>

      <div className="grid items-start gap-8 lg:grid-cols-[220px_1fr]">
        <aside className="sticky top-28 hidden rounded-lg border border-border bg-bg-elevated p-4 lg:block">
          <div className="mb-3 text-[0.7rem] font-semibold uppercase tracking-wide text-fg-muted">
            Report sections
          </div>
          <nav className="space-y-2 text-sm">
            {[
              ["summary", "Executive summary"],
              ["configuration", "Tested configuration"],
              ["suite", "Scenario suite"],
              ["results", "Results table"],
              ["classification", "Safety classification"],
              ["autopsy", "Failure autopsy"],
              ["cost", "Cost, tokens, turns"],
              ["findings", "Top findings"],
              ["recommendations", "Recommendations"],
              ["evidence", "Raw evidence"],
            ].map(([id, label]) => (
              <a key={id} href={`#${id}`} className="block text-fg-muted transition-colors hover:text-accent">
                {label}
              </a>
            ))}
          </nav>
        </aside>

        <div className="space-y-10">
          <ReportSection id="summary" title="1. Executive Summary">
            <div className="grid grid-cols-1 gap-4 md:grid-cols-2 xl:grid-cols-4">
              {summaryCard(
                "Baseline",
                formatPercent(report.comparison?.baseline?.pass_rate ?? 0),
                `${report.comparison?.baseline?.passed ?? 0}/${report.comparison?.baseline?.runs ?? 0} passed`,
                <>
                  {metricLine("Turns", (report.comparison?.baseline?.avg_turns ?? 0).toFixed(1))}
                  {metricLine("Tokens", formatTokens(report.comparison?.baseline?.avg_tokens ?? 0))}
                  {metricLine("Cost", formatCost(report.comparison?.baseline?.avg_cost_usd ?? 0))}
                </>,
              )}
              {summaryCard(
                "Candidate",
                formatPercent(report.comparison?.candidate?.pass_rate ?? 0),
                `${report.comparison?.candidate?.passed ?? 0}/${report.comparison?.candidate?.runs ?? 0} passed`,
                <>
                  {metricLine("Turns", (report.comparison?.candidate?.avg_turns ?? 0).toFixed(1))}
                  {metricLine("Tokens", formatTokens(report.comparison?.candidate?.avg_tokens ?? 0))}
                  {metricLine("Cost", formatCost(report.comparison?.candidate?.avg_cost_usd ?? 0))}
                </>,
              )}
              {summaryCard(
                "Safe pass rate",
                formatPercent(report.summary.safe_pass_rate),
                "strict safety metric",
              )}
              {summaryCard(
                "Evidence gaps",
                String(report.summary.missing_evidence),
                "missing sides",
              )}
            </div>
          </ReportSection>

          <ReportSection id="configuration" title="2. Tested Configuration">
            <DataTable headers={["Field", "Value"]}>
              {[
                ["Report type", report.configuration.report_type],
                ["Model", report.configuration.model],
                ["Provider", report.configuration.provider || "-"],
                ["Tool server", report.configuration.tool_server],
                ["Tool server version", report.configuration.tool_server_version || "-"],
                ["Scenario slice", report.configuration.scenario_slice],
                ["Generated at", generatedAt],
              ].map(([label, value]) => (
                <tr key={label} className="border-t border-border-subtle">
                  <td className="px-4 py-3 text-fg-muted">{label}</td>
                  <td className="px-4 py-3 font-mono text-[0.82rem] text-fg">{value}</td>
                </tr>
              ))}
            </DataTable>
          </ReportSection>

          <ReportSection id="suite" title="3. Scenario Suite">
            <DataTable headers={["Scenario", "Category", "Level", "Title"]}>
              {scenarios.map((row) => (
                <tr key={row.id} className="border-t border-border-subtle">
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-[0.8rem] text-fg">{row.id}</td>
                  <td className="px-4 py-3 text-fg-body">{row.category || "-"}</td>
                  <td className="px-4 py-3 text-fg-body">{row.level || "-"}</td>
                  <td className="px-4 py-3 text-fg-muted">{row.title || "-"}</td>
                </tr>
              ))}
            </DataTable>
            {scenarioRulebooks.length > 0 && (
              <div className="mt-5 space-y-3">
                <h3 className="text-sm font-semibold text-fg">Autopsy Rulebook</h3>
                {scenarioRulebooks.map((row) => (
                  <div key={`${row.id}-rulebook`} className="rounded-lg border border-border bg-bg-elevated p-4">
                    <div className="mb-2 font-mono text-[0.78rem] font-semibold text-fg">{row.id}</div>
                    <p className="whitespace-pre-line text-sm leading-relaxed text-fg-muted">
                      {row.autopsy_description?.trim()}
                    </p>
                  </div>
                ))}
              </div>
            )}
          </ReportSection>

          <ReportSection id="results" title="4. Results Table">
            <DataTable headers={["Scenario", "Result", "Classification", "Turns", "Tokens", "Duration", "Cost", "Delta"]}>
              {scenarios.map((row) => (
                <tr key={row.id} className="border-t border-border-subtle">
                  <td className="whitespace-nowrap px-4 py-3 font-mono text-[0.8rem] text-fg">{row.id}</td>
                  <td className="px-4 py-3 text-fg-body">{row.result}</td>
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded px-2 py-1 text-[0.68rem] font-semibold ${classificationClass(row.classification)}`}>
                      {classificationLabel(row.classification)}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{row.candidate.avg_turns.toFixed(1)}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{formatTokens(row.candidate.avg_tokens)}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{formatDuration(row.candidate.avg_duration_seconds)}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{formatCost(row.candidate.avg_cost_usd)}</td>
                  <td className={`px-4 py-3 font-mono text-[0.8rem] font-semibold ${deltaClass(row.delta.pass_rate_delta)}`}>
                    {row.delta.pass_rate_delta > 0 ? "+" : ""}{row.delta.pass_rate_delta.toFixed(1)}pp
                  </td>
                </tr>
              ))}
            </DataTable>
          </ReportSection>

          <ReportSection id="classification" title="5. Safe Pass / Unsafe Pass / Fail">
            <div className="grid gap-4 md:grid-cols-4">
              {[
                ["safe_pass", "Final state passed and no deterministic safety findings were present."],
                ["unsafe_pass", "Final state passed, but deterministic evidence flagged unsafe behavior."],
                ["fail", "Candidate evidence did not pass the selected scenario."],
                ["missing_evidence", "Baseline or candidate evidence is missing for the scenario."],
              ].map(([key, body]) => (
                <div key={key} className="rounded-lg border border-border bg-bg-elevated p-4">
                  <div className={`mb-3 inline-flex rounded px-2 py-1 text-[0.68rem] font-semibold ${classificationClass(key)}`}>
                    {classificationLabel(key)}
                  </div>
                  <p className="text-sm leading-relaxed text-fg-body">{body}</p>
                </div>
              ))}
            </div>
          </ReportSection>

          <ReportSection id="autopsy" title="6. Failure Autopsy">
            {autopsies.length === 0 ? (
              <div className="rounded-lg border border-border bg-bg-elevated p-5 text-sm text-fg-muted">
                No failure autopsy rows for this report slice.
              </div>
            ) : (
              <div className="space-y-4">
                {autopsies.map((item) => (
                  <div key={`${item.scenario_id}-${item.run_id || "missing"}`} className="rounded-lg border border-border bg-bg-elevated p-5">
                    <div className="mb-2 flex flex-wrap items-center gap-2">
                      <h3 className="font-mono text-sm font-semibold text-fg">{item.scenario_id}</h3>
                      {item.primary_failure && (
                        <span className="rounded bg-danger-tint px-2 py-0.5 text-[0.68rem] font-semibold text-danger">
                          {item.primary_failure}
                        </span>
                      )}
                    </div>
                    <p className="mb-4 text-sm leading-relaxed text-fg-muted">{item.summary}</p>
                    {(item.findings ?? []).length > 0 && (
                      <ul className="space-y-2">
                        {(item.findings ?? []).map((finding) => (
                          <li key={`${finding.kind}-${finding.message}`} className="flex gap-3 text-sm text-fg-body">
                            <span className="mt-2 h-1.5 w-1.5 shrink-0 rounded-full bg-danger" />
                            <span>{finding.kind}: {finding.message}</span>
                          </li>
                        ))}
                      </ul>
                    )}
                  </div>
                ))}
              </div>
            )}
          </ReportSection>

          <ReportSection id="cost" title="7. Cost / Tokens / Turns">
            <DataTable headers={["Bucket", "Scenarios", "Avg turns", "Avg tokens", "Avg duration", "Avg cost"]}>
              {costBuckets.map((row) => (
                <tr key={row.classification} className="border-t border-border-subtle">
                  <td className="px-4 py-3">
                    <span className={`inline-flex rounded px-2 py-1 text-[0.68rem] font-semibold ${classificationClass(row.classification)}`}>
                      {classificationLabel(row.classification)}
                    </span>
                  </td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{row.scenarios}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{row.avg_turns.toFixed(1)}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{formatTokens(row.avg_tokens)}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{formatDuration(row.avg_duration_seconds)}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-body">{formatCost(row.avg_cost_usd)}</td>
                </tr>
              ))}
            </DataTable>
          </ReportSection>

          <ReportSection id="findings" title="8. Top Findings">
            <ol className="space-y-3">
              {findings.map((finding, index) => (
                <li key={finding} className="flex gap-3 text-sm text-fg-body">
                  <span className="font-mono text-accent">{String(index + 1).padStart(2, "0")}</span>
                  <span>{finding}</span>
                </li>
              ))}
            </ol>
          </ReportSection>

          <ReportSection id="recommendations" title="9. Recommendations">
            <ol className="space-y-3">
              {recommendations.map((recommendation, index) => (
                <li key={recommendation} className="flex gap-3 text-sm text-fg-body">
                  <span className="font-mono text-accent">{String(index + 1).padStart(2, "0")}</span>
                  <span>{recommendation}</span>
                </li>
              ))}
            </ol>
          </ReportSection>

          <ReportSection id="evidence" title="10. Raw Evidence Links / Artifacts">
            {evidenceLinks.length === 0 ? (
              <p className="text-sm text-fg-muted">No candidate evidence links available.</p>
            ) : (
              <DataTable headers={["Artifact", "Link"]}>
                {evidenceLinks.map((link, index) => (
                  <tr key={`${link.url}-${index}`} className="border-t border-border-subtle">
                    <td className="px-4 py-3 text-fg-body">{link.label}</td>
                    <td className="px-4 py-3 font-mono text-[0.8rem] text-fg-muted">{link.url}</td>
                  </tr>
                ))}
              </DataTable>
            )}
            <div className="mt-5 flex flex-wrap gap-3">
              <Link to={BENCH_RUNS_PATH} className="inline-flex rounded-md bg-accent-tint px-4 py-2 text-sm font-semibold text-accent">
                Browse live runs
              </Link>
              <Link to={BENCH_SCENARIOS_PATH} className="inline-flex rounded-md border border-border px-4 py-2 text-sm font-semibold text-fg-body">
                Browse scenarios
              </Link>
            </div>
          </ReportSection>
        </div>
      </div>
    </article>
  );
}
