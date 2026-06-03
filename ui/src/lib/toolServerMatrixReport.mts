export interface ToolServerAggregate {
  runs: number;
  passed: number;
  pass_rate: number;
  avg_turns: number;
  avg_tokens: number;
  avg_cost_usd: number;
  avg_duration_seconds: number;
}

export interface ToolServerMatrixReportFilters {
  model: string;
  reportId: string;
  toolServers: string[];
  toolServerVersions?: string[];
  scenarioIds?: string[];
  format?: "json" | "markdown";
}

export interface ToolServerMatrixArm {
  id: string;
  label: string;
  kind: "baseline" | "candidate" | string;
  tool_server?: string;
  tool_server_version?: string;
  aggregate: ToolServerAggregate;
}

export interface ToolServerMatrixScenarioArm {
  arm_id: string;
  label: string;
  classification: string;
  result: string;
  run_id?: string;
  aggregate: ToolServerAggregate;
  evidence_links?: ToolServerReportEvidenceLink[];
}

export interface ToolServerMatrixScenario {
  id: string;
  title?: string;
  category?: string;
  level?: string;
  arms: ToolServerMatrixScenarioArm[];
}

export interface ToolServerMatrixSummary {
  total_scenarios: number;
  candidate_cells: number;
  safe_pass: number;
  unsafe_pass: number;
  fail: number;
  missing_evidence: number;
}

export interface ToolServerMatrixFailureModeBreakdownRow {
  arm_id: string;
  tool_server: string;
  failure_mode: string;
  failure_mode_label: string;
  unsafe_pass: number;
  fail: number;
  missing_evidence: number;
  scenario_ids?: string[];
}

export interface ToolServerReportEvidenceLink {
  label: string;
  url: string;
}

export interface ToolServerMatrixAutopsy {
  arm_id: string;
  tool_server: string;
  tool_server_version?: string;
  scenario_id: string;
  run_id?: string;
  primary_failure?: string;
  failure_mode?: string;
  failure_mode_label?: string;
  summary: string;
  missing?: boolean;
  findings?: Array<{
    kind: string;
    severity: string;
    message: string;
    evidence?: string;
  }>;
  evidence_links?: ToolServerReportEvidenceLink[];
}

export interface ToolServerMatrixReportResponse {
  title: string;
  generated_at: string;
  model: string;
  report_id: string;
  scenario_ids?: string[];
  arms: ToolServerMatrixArm[];
  summary: ToolServerMatrixSummary;
  methodology: string[];
  scenarios: ToolServerMatrixScenario[];
  autopsies: ToolServerMatrixAutopsy[];
  failure_mode_breakdown?: ToolServerMatrixFailureModeBreakdownRow[];
  findings: string[];
  recommendations: string[];
  evidence_links: ToolServerReportEvidenceLink[];
}

export function buildToolServerMatrixReportApiPath(filters: ToolServerMatrixReportFilters): string {
  const params = new URLSearchParams();
  params.set("model", filters.model);
  params.set("report_id", filters.reportId);
  params.set("tool_servers", filters.toolServers.join(","));
  if (filters.toolServerVersions && filters.toolServerVersions.length > 0) {
    params.set("tool_server_versions", filters.toolServerVersions.join(","));
  }
  if (filters.scenarioIds && filters.scenarioIds.length > 0) {
    params.set("scenarios", filters.scenarioIds.join(","));
  }
  if (filters.format && filters.format !== "json") {
    params.set("format", filters.format);
  }
  return `/v1/bench/reports/tool-server-matrix?${params.toString()}`;
}
