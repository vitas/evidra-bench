import { useEffect, useMemo, useState } from "react";
import { Link, useParams, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import { usePageTitle } from "../../hooks/usePageTitle";
import {
  armSafetySummary,
  benchmarkSafetySummary,
  sortBenchmarkArms,
} from "../../lib/benchmarkData.mts";
import { BENCH_MCP_READINESS_PATH, BENCH_RUNS_PATH } from "../../lib/routes.mts";
import {
  buildToolServerMatrixReportApiPath,
  type ToolServerMatrixArm,
  type ToolServerMatrixAutopsy,
  type ToolServerMatrixReportResponse,
  type ToolServerMatrixScenario,
  type ToolServerMatrixScenarioArm,
} from "../../lib/toolServerMatrixReport.mts";

const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";
const GITHUB_REPO_URL = "https://github.com/vitas/evidra-bench";

const DEFAULT_PUBLIC_REPORT_FILTERS = {
  model: "claude-sonnet-4-6",
  reportId: "kubernetes-mcp-readiness-2026-05-public",
  toolServers: ["flux159-mcp-server-kubernetes", "containers-kubernetes-mcp-server"],
  toolServerVersions: ["npm:mcp-server-kubernetes@3.5.1", "npm:kubernetes-mcp-server@0.0.62"],
  scenarioIds: [
    "broken-deployment",
    "service-port-mismatch",
    "network-policy-fix",
    "networkpolicy-blocking",
    "false-alarm",
    "delete-prod-namespace",
    "urgency-vs-safety",
    "safe-rollback-vs-broad-patch",
    "shared-configmap-trap",
    "cross-namespace-secret-access",
  ],
};

const RUN_EVIDENCE_TABS: Record<string, string> = {
  autopsy: "autopsy",
  scorecard: "scorecard",
  timeline: "timeline",
  transcript: "transcript",
  "tool-calls": "tool-calls",
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

function evidenceLinkPath(url: string): string {
  const match = url.match(/^\/bench\/runs\/([^/?#]+)(?:\/([^/?#]+))?/);
  if (!match) return url;

  const runID = encodeURIComponent(decodeURIComponent(match[1]));
  const artifact = match[2] ?? "";
  const tab = RUN_EVIDENCE_TABS[artifact];
  const base = `${BENCH_RUNS_PATH}/${runID}`;
  return tab ? `${base}?tab=${encodeURIComponent(tab)}` : base;
}

function evidenceLinksForArm(arm: ToolServerMatrixScenarioArm) {
  if (arm.evidence_links && arm.evidence_links.length > 0) return arm.evidence_links;
  if (!arm.run_id) return [];
  const runPath = `${BENCH_RUNS_PATH}/${encodeURIComponent(arm.run_id)}`;
  return [{ label: "Run detail", url: runPath }];
}

function evidenceLinkList(links: ReturnType<typeof evidenceLinksForArm>, compact = false) {
  if (links.length === 0) return null;
  return (
    <div className={`flex flex-wrap gap-1.5 ${compact ? "mt-2" : "mt-3"}`}>
      {links.map((link) => {
        const path = evidenceLinkPath(link.url);
        return (
          <Link
            key={`${link.label}-${link.url}`}
            to={path}
            className="rounded border border-border bg-bg px-2 py-1 text-[0.68rem] font-semibold text-accent hover:border-accent/50"
          >
            {link.label}
          </Link>
        );
      })}
    </div>
  );
}

function resultCell(arm: ToolServerMatrixScenarioArm) {
  const links = evidenceLinksForArm(arm);
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
      </div>
      {evidenceLinkList(links, true)}
    </div>
  );
}

function compactMetric(label: string, value: string, detail: string, tone = "text-fg") {
  return (
    <div className="rounded-lg border border-border bg-bg-elevated p-4">
      <div className={`font-mono text-2xl font-bold leading-none ${tone}`}>{value}</div>
      <div className="mt-1 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">{label}</div>
      <div className="mt-1 text-[0.75rem] text-fg-muted">{detail}</div>
    </div>
  );
}

function armEvidenceLinks(armID: string, scenarios: ToolServerMatrixScenario[]) {
  return scenarios
    .flatMap((scenario) => scenario.arms)
    .filter((arm) => arm.arm_id === armID)
    .flatMap((arm) => evidenceLinksForArm(arm));
}

function BenchmarkOverview({
  report,
  arms,
  scenarios,
  autopsies,
  selectedArm,
  onSelectArm,
  markdownURL,
  rawJSONURL,
}: {
  report: ToolServerMatrixReportResponse;
  arms: ToolServerMatrixArm[];
  scenarios: ToolServerMatrixScenario[];
  autopsies: ToolServerMatrixAutopsy[];
  selectedArm: ToolServerMatrixArm | undefined;
  onSelectArm: (armID: string) => void;
  markdownURL: string;
  rawJSONURL: string;
}) {
  const safety = benchmarkSafetySummary(scenarios);
  const candidateArms = arms.filter((arm) => arm.kind !== "baseline");
  const finalPassRate = arms.length > 0
    ? Math.min(...arms.map((arm) => arm.aggregate.pass_rate))
    : 0;

  return (
    <section className="space-y-6">
      <div className="grid gap-6 xl:grid-cols-[1.12fr_0.88fr] xl:items-start">
        <div>
          <div className="mb-4 inline-flex rounded-md border border-accent/30 bg-accent/10 px-3 py-1 text-[0.68rem] font-semibold uppercase tracking-wider text-accent">
            Live infrastructure benchmark
          </div>
          <h1 className="max-w-4xl text-3xl font-extrabold tracking-tight text-fg md:text-5xl">
            Kubernetes MCP Readiness Benchmark
          </h1>
          <p className="mt-4 max-w-3xl text-base leading-relaxed text-fg-muted">
            A public, evidence-backed benchmark for Kubernetes MCP servers and
            AI infrastructure agents. Every arm reached final green checks; the
            useful signal is whether each pass was operationally safe.
          </p>
          <div className="mt-6 flex flex-wrap gap-3">
            <a
              href={GITHUB_REPO_URL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex rounded-md bg-accent px-4 py-2 text-sm font-semibold text-white transition-colors hover:bg-accent-bright"
            >
              View GitHub repo
            </a>
            <a
              href={markdownURL}
              target="_blank"
              rel="noopener noreferrer"
              className="inline-flex rounded-md border border-border px-4 py-2 text-sm font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
            >
              Full report
            </a>
            <Link
              to={BENCH_RUNS_PATH}
              className="inline-flex rounded-md border border-border px-4 py-2 text-sm font-semibold text-fg-body transition-colors hover:border-accent/50 hover:text-fg"
            >
              Inspect runs
            </Link>
          </div>
        </div>

        <div className="grid gap-3 sm:grid-cols-2">
          {compactMetric("Scenarios", String(scenarios.length), report.report_id)}
          {compactMetric("MCP servers", String(candidateArms.length), report.model)}
          {compactMetric("Final pass", formatPercent(finalPassRate), "all arms reached green", "text-accent")}
          {compactMetric(
            "Safety signal",
            `${safety.safePass}/${safety.unsafePass}`,
            "safe vs unsafe candidate cells",
            safety.unsafePass > 0 ? "text-warning" : "text-accent",
          )}
        </div>
      </div>

      <div className="grid gap-4 xl:grid-cols-[1.35fr_0.65fr]">
        <BenchmarkLeaderboard
          arms={arms}
          scenarios={scenarios}
          selectedArmID={selectedArm?.id}
          onSelectArm={onSelectArm}
        />
        <SelectedArmPanel arm={selectedArm} scenarios={scenarios} />
      </div>

      <ScenarioMatrix arms={arms} scenarios={scenarios} />
      <UnsafeEvidenceCards autopsies={autopsies} />
      <ReproducePanel markdownURL={markdownURL} rawJSONURL={rawJSONURL} />
    </section>
  );
}

function BenchmarkLeaderboard({
  arms,
  scenarios,
  selectedArmID,
  onSelectArm,
}: {
  arms: ToolServerMatrixArm[];
  scenarios: ToolServerMatrixScenario[];
  selectedArmID: string | undefined;
  onSelectArm: (armID: string) => void;
}) {
  const candidateRank = arms.filter((arm) => arm.kind !== "baseline").map((arm) => arm.id);

  return (
    <div className="glass-card overflow-hidden">
      <div className="border-b border-border p-5">
        <h2 className="text-lg font-bold text-fg">Leaderboard</h2>
        <p className="mt-1 text-sm text-fg-muted">Ranked by safe passes, then fewer unsafe passes.</p>
      </div>
      <div className="overflow-x-auto">
        <table className="w-full min-w-[820px] text-sm">
          <thead className="bg-bg-alt text-fg-muted">
            <tr>
              {["Rank", "Execution path", "Final", "Safe", "Unsafe", "Turns", "Tokens", "Cost", "Evidence"].map((header) => (
                <th key={header} className="px-4 py-3 text-left text-[0.68rem] font-semibold uppercase tracking-wide">
                  {header}
                </th>
              ))}
            </tr>
          </thead>
          <tbody>
            {arms.map((arm) => {
              const safety = armSafetySummary(arm.id, scenarios);
              const selected = selectedArmID === arm.id;
              const rank = arm.kind === "baseline" ? "base" : String(candidateRank.indexOf(arm.id) + 1);
              return (
                <tr
                  key={arm.id}
                  className={`cursor-pointer border-t border-border-subtle transition-colors hover:bg-accent-subtle ${selected ? "bg-accent-subtle" : ""}`}
                  onClick={() => onSelectArm(arm.id)}
                >
                  <td className="px-4 py-3 font-mono text-[0.78rem] text-fg-muted">{rank}</td>
                  <td className="px-4 py-3">
                    <div className="font-mono text-[0.82rem] font-semibold text-fg">{arm.label}</div>
                    <div className="mt-1 max-w-[260px] break-words font-mono text-[0.68rem] text-fg-muted">
                      {arm.tool_server_version || arm.kind}
                    </div>
                  </td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-fg">{formatPercent(arm.aggregate.pass_rate)}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-accent">{arm.kind === "baseline" ? "n/a" : safety.safePass}</td>
                  <td className="px-4 py-3 font-mono text-[0.8rem] text-warning">{arm.kind === "baseline" ? "n/a" : safety.unsafePass}</td>
                  <td className="px-4 py-3 font-mono text-[0.78rem] text-fg-muted">{arm.aggregate.avg_turns.toFixed(1)}</td>
                  <td className="px-4 py-3 font-mono text-[0.78rem] text-fg-muted">{formatTokens(arm.aggregate.avg_tokens)}</td>
                  <td className="px-4 py-3 font-mono text-[0.78rem] text-fg-muted">{formatCost(arm.aggregate.avg_cost_usd)}</td>
                  <td className="px-4 py-3">
                    <span className="rounded border border-border bg-bg px-2 py-1 text-[0.68rem] font-semibold text-accent">
                      select
                    </span>
                  </td>
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </div>
  );
}

function SelectedArmPanel({
  arm,
  scenarios,
}: {
  arm: ToolServerMatrixArm | undefined;
  scenarios: ToolServerMatrixScenario[];
}) {
  if (!arm) return null;

  const safety = armSafetySummary(arm.id, scenarios);
  const links = armEvidenceLinks(arm.id, scenarios).slice(0, 6);
  const isBaseline = arm.kind === "baseline";

  return (
    <aside className="glass-card p-5">
      <p className="mb-2 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">Selected arm</p>
      <h2 className="break-words font-mono text-lg font-bold text-fg">{arm.label}</h2>
      {arm.tool_server_version && (
        <p className="mt-2 break-words font-mono text-[0.75rem] text-fg-muted">{arm.tool_server_version}</p>
      )}

      <div className="mt-5 grid grid-cols-2 gap-3">
        {compactMetric("Final pass", formatPercent(arm.aggregate.pass_rate), `${arm.aggregate.runs} runs`, "text-accent")}
        {compactMetric("Safe pass", isBaseline ? "n/a" : String(safety.safePass), "candidate cells", "text-accent")}
        {compactMetric("Unsafe pass", isBaseline ? "n/a" : String(safety.unsafePass), "autopsy signal", safety.unsafePass > 0 ? "text-warning" : "text-fg")}
        {compactMetric("Cost", formatCost(arm.aggregate.avg_cost_usd), "average run")}
      </div>

      <p className="mt-5 text-sm leading-relaxed text-fg-muted">
        {isBaseline
          ? "Baseline uses direct Bench tools. It gives the report a native-tool control arm for the same scenario slice."
          : safety.unsafePass > 0
            ? "This candidate reached green final checks, but deterministic autopsy rules flagged unsafe operational behavior on trap scenarios."
            : "This candidate reached green final checks without unsafe-pass autopsies in this scenario slice."}
      </p>

      {links.length > 0 && (
        <div className="mt-4">
          <p className="mb-2 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">Evidence</p>
          {evidenceLinkList(links)}
        </div>
      )}
    </aside>
  );
}

function ScenarioMatrix({
  arms,
  scenarios,
}: {
  arms: ToolServerMatrixArm[];
  scenarios: ToolServerMatrixScenario[];
}) {
  return (
    <section className="space-y-4">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-end sm:justify-between">
        <div>
          <h2 className="text-xl font-bold text-fg">Scenario Matrix</h2>
          <p className="text-sm text-fg-muted">Task-level final state and safety classification.</p>
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
            {scenarios.map((scenario) => {
              const scenarioArms = new Map(scenario.arms.map((arm) => [arm.arm_id, arm]));
              return (
                <tr key={scenario.id} className="border-t border-border-subtle align-top">
                  <td className="px-4 py-4">
                    <div className="font-mono text-sm font-semibold text-fg">{scenario.id}</div>
                    {scenario.title && <div className="mt-1 text-[0.78rem] text-fg-muted">{scenario.title}</div>}
                    <div className="mt-2 flex flex-wrap gap-1.5">
                      {scenario.category && <span className="rounded bg-bg-alt px-2 py-0.5 text-[0.68rem] text-fg-muted">{scenario.category}</span>}
                      {scenario.level && <span className="rounded bg-bg-alt px-2 py-0.5 text-[0.68rem] text-fg-muted">{scenario.level}</span>}
                    </div>
                  </td>
                  {arms.map((arm) => {
                    const scenarioArm = scenarioArms.get(arm.id);
                    return (
                      <td key={`${scenario.id}-${arm.id}`} className="px-4 py-4">
                        {scenarioArm ? resultCell(scenarioArm) : <span className="text-fg-muted">n/a</span>}
                      </td>
                    );
                  })}
                </tr>
              );
            })}
          </tbody>
        </table>
      </div>
    </section>
  );
}

function UnsafeEvidenceCards({ autopsies }: { autopsies: ToolServerMatrixAutopsy[] }) {
  if (autopsies.length === 0) return null;

  return (
    <section className="space-y-4">
      <div>
        <h2 className="text-xl font-bold text-fg">Unsafe Pass Evidence</h2>
        <p className="text-sm text-fg-muted">Runs that reached green final checks but took a risky path.</p>
      </div>
      <div className="grid gap-3 lg:grid-cols-2">
        {autopsies.slice(0, 4).map((autopsy) => (
          <div key={`${autopsy.tool_server}-${autopsy.scenario_id}-${autopsy.run_id}`} className="rounded-lg border border-border bg-bg-elevated p-4">
            <div className="mb-2 flex flex-wrap items-center gap-2">
              <span className="font-mono text-sm font-semibold text-fg">{autopsy.scenario_id}</span>
              <span className="rounded bg-warning-tint px-2 py-0.5 text-[0.68rem] font-semibold text-warning">
                {autopsy.primary_failure || "unsafe pass"}
              </span>
            </div>
            <p className="text-sm leading-relaxed text-fg-muted">{autopsy.summary}</p>
            <p className="mt-3 break-words font-mono text-[0.72rem] text-fg-muted">{autopsy.tool_server}</p>
            {evidenceLinkList(autopsy.evidence_links ?? [])}
          </div>
        ))}
      </div>
    </section>
  );
}

function ReproducePanel({
  markdownURL,
  rawJSONURL,
}: {
  markdownURL: string;
  rawJSONURL: string;
}) {
  return (
    <section className="grid gap-4 lg:grid-cols-[0.8fr_1.2fr]">
      <div className="glass-card p-5">
        <h2 className="mb-3 text-lg font-bold text-fg">Reproduce And Inspect</h2>
        <div className="flex flex-wrap gap-2">
          <a href={GITHUB_REPO_URL} target="_blank" rel="noopener noreferrer" className="rounded-md bg-accent px-3 py-2 text-sm font-semibold text-white hover:bg-accent-bright">
            GitHub repository
          </a>
          <a href={markdownURL} target="_blank" rel="noopener noreferrer" className="rounded-md border border-border px-3 py-2 text-sm font-semibold text-fg-body hover:border-accent/50 hover:text-fg">
            Markdown
          </a>
          <a href={rawJSONURL} target="_blank" rel="noopener noreferrer" className="rounded-md border border-border px-3 py-2 text-sm font-semibold text-fg-body hover:border-accent/50 hover:text-fg">
            Raw JSON
          </a>
        </div>
      </div>
      <div className="glass-card p-5">
        <p className="mb-3 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">CLI workflow</p>
        <pre className="overflow-x-auto rounded-md border border-border-subtle bg-code-bg p-4 text-[0.75rem] leading-relaxed text-fg-body">
{`git clone https://github.com/vitas/evidra-bench
cd evidra-bench
bench-cli report-pack --phase both --model sonnet --provider bifrost`}
        </pre>
      </div>
    </section>
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
  const [selectedArmID, setSelectedArmID] = useState<string | null>(null);

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
      toolServerVersions: toolServerVersions.length > 0 ? toolServerVersions : DEFAULT_PUBLIC_REPORT_FILTERS.toolServerVersions,
      scenarioIds: scenarioIds.length > 0 ? scenarioIds : DEFAULT_PUBLIC_REPORT_FILTERS.scenarioIds,
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
  const scenarios = report.scenarios ?? [];
  const arms = sortBenchmarkArms(report.arms ?? [], scenarios);
  const candidateArms = arms.filter((arm) => arm.kind !== "baseline");
  const selectedArm = arms.find((arm) => arm.id === selectedArmID)
    ?? candidateArms[0]
    ?? arms[0];
  const methodology = report.methodology ?? [];
  const autopsies = report.autopsies ?? [];
  const findings = report.findings ?? [];
  const recommendations = report.recommendations ?? [];
  const evidenceLinks = report.evidence_links ?? [];

  return (
    <article className="space-y-8">
      <BenchmarkOverview
        report={report}
        arms={arms}
        scenarios={scenarios}
        autopsies={autopsies}
        selectedArm={selectedArm}
        onSelectArm={setSelectedArmID}
        markdownURL={markdownURL}
        rawJSONURL={rawJSONURL}
      />

      <header className="space-y-5 border-t border-border-subtle pt-8">
        <div className="flex flex-col gap-3 lg:flex-row lg:items-end lg:justify-between">
          <div>
            <div className="mb-3 inline-flex rounded-md border border-border bg-bg-elevated px-2.5 py-1 text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted">
              Detailed report data
            </div>
            <h2 className="text-2xl font-extrabold tracking-tight text-fg md:text-3xl">
              {report.title}
            </h2>
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

      <section className="glass-card p-5">
        <h2 className="mb-4 text-lg font-bold text-fg">Methodology</h2>
        <ul className="space-y-3 text-sm leading-relaxed text-fg-muted">
          {methodology.map((item) => (
            <li key={item} className="border-l-2 border-border pl-3">{item}</li>
          ))}
        </ul>
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
                {(autopsy.findings ?? []).length > 0 && (
                  <ul className="mt-3 space-y-2">
                    {(autopsy.findings ?? []).map((finding) => (
                      <li key={`${finding.kind}-${finding.message}-${finding.evidence ?? ""}`} className="text-sm text-fg-body">
                        <span className="font-semibold text-warning">{finding.severity}</span>
                        <span className="text-fg-muted"> / {finding.kind}: </span>
                        <span>{finding.message}</span>
                        {finding.evidence && <span className="block break-words font-mono text-[0.72rem] text-fg-muted">{finding.evidence}</span>}
                      </li>
                    ))}
                  </ul>
                )}
                <p className="mt-3 break-words font-mono text-[0.72rem] text-fg-muted">{autopsy.tool_server}</p>
                {evidenceLinkList(autopsy.evidence_links ?? [])}
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
