import { benchToolServerMatrixReportPagePath } from "./routes.mts";

const KUBERNETES_MCP_TOOL_SERVERS = [
  "flux159-mcp-server-kubernetes",
  "containers-kubernetes-mcp-server",
];

const KUBERNETES_MCP_TOOL_SERVER_VERSIONS = [
  "npm:mcp-server-kubernetes@3.5.1",
  "npm:kubernetes-mcp-server@0.0.62",
];

const CLAUDE_PUBLIC_SCENARIOS = [
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
];

const DEEPSEEK_PILOT_SCENARIOS = [
  "broken-deployment",
  "false-alarm",
  "shared-configmap-trap",
];

export type PublicReportFilters = {
  model: string;
  reportId: string;
  toolServers: string[];
  toolServerVersions: string[];
  scenarioIds: string[];
};

export const PUBLIC_REPORT_DEFAULTS: Record<string, PublicReportFilters> = {
  "kubernetes-mcp-readiness-2026-05-public": {
    model: "claude-sonnet-4-6",
    reportId: "kubernetes-mcp-readiness-2026-05-public",
    toolServers: KUBERNETES_MCP_TOOL_SERVERS,
    toolServerVersions: KUBERNETES_MCP_TOOL_SERVER_VERSIONS,
    scenarioIds: CLAUDE_PUBLIC_SCENARIOS,
  },
  "kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot": {
    model: "deepseek-v4-flash",
    reportId: "kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot",
    toolServers: KUBERNETES_MCP_TOOL_SERVERS,
    toolServerVersions: KUBERNETES_MCP_TOOL_SERVER_VERSIONS,
    scenarioIds: DEEPSEEK_PILOT_SCENARIOS,
  },
};

export function publicReportDefaults(reportId: string | undefined): PublicReportFilters {
  if (reportId && PUBLIC_REPORT_DEFAULTS[reportId]) {
    return PUBLIC_REPORT_DEFAULTS[reportId];
  }
  return PUBLIC_REPORT_DEFAULTS["kubernetes-mcp-readiness-2026-05-public"];
}

function publicReportPath(reportId: string): string {
  const defaults = publicReportDefaults(reportId);
  return benchToolServerMatrixReportPagePath(reportId, {
    model: defaults.model,
    scenarios: defaults.scenarioIds.join(","),
    tool_servers: defaults.toolServers.join(","),
    tool_server_versions: defaults.toolServerVersions.join(","),
  });
}

export type LandingPublicReport = {
  id: string;
  label: string;
  title: string;
  model: string;
  scope: string;
  result: string;
  finding: string;
  to: string;
};

export const LANDING_PUBLIC_REPORTS: LandingPublicReport[] = [
  {
    id: "kubernetes-mcp-readiness-2026-05-public",
    label: "Primary public report",
    title: "Kubernetes MCP Readiness",
    model: "Claude Sonnet 4.6",
    scope: "10 live scenarios",
    result: "Flux159 produced 10 safe passes. containers reached final-state pass but triggered 4 unsafe-pass autopsies.",
    finding: "16 safe / 4 unsafe candidate cells",
    to: publicReportPath("kubernetes-mcp-readiness-2026-05-public"),
  },
  {
    id: "kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot",
    label: "Pilot replication",
    title: "DeepSeek V4 Flash Check",
    model: "DeepSeek V4 Flash",
    scope: "3 focused scenarios",
    result: "The same safety signal repeated: Flux159 stayed safe, while containers produced unsafe passes on trap scenarios.",
    finding: "4 safe / 2 unsafe candidate cells",
    to: publicReportPath("kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot"),
  },
];
