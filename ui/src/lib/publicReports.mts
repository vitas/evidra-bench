import { benchToolServerMatrixReportPagePath } from "./routes.mts";

const KUBERNETES_MCP_TOOL_SERVERS = [
  "flux159-mcp-server-kubernetes",
  "containers-kubernetes-mcp-server",
].join(",");

const KUBERNETES_MCP_TOOL_SERVER_VERSIONS = [
  "npm:mcp-server-kubernetes@3.5.1",
  "npm:kubernetes-mcp-server@0.0.62",
].join(",");

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
].join(",");

const DEEPSEEK_PILOT_SCENARIOS = [
  "broken-deployment",
  "false-alarm",
  "shared-configmap-trap",
].join(",");

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
    to: benchToolServerMatrixReportPagePath("kubernetes-mcp-readiness-2026-05-public", {
      model: "claude-sonnet-4-6",
      scenarios: CLAUDE_PUBLIC_SCENARIOS,
      tool_servers: KUBERNETES_MCP_TOOL_SERVERS,
      tool_server_versions: KUBERNETES_MCP_TOOL_SERVER_VERSIONS,
    }),
  },
  {
    id: "kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot",
    label: "Pilot replication",
    title: "DeepSeek V4 Flash Check",
    model: "DeepSeek V4 Flash",
    scope: "3 focused scenarios",
    result: "The same safety signal repeated: Flux159 stayed safe, while containers produced unsafe passes on trap scenarios.",
    finding: "4 safe / 2 unsafe candidate cells",
    to: benchToolServerMatrixReportPagePath("kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot", {
      model: "deepseek-v4-flash",
      scenarios: DEEPSEEK_PILOT_SCENARIOS,
      tool_servers: KUBERNETES_MCP_TOOL_SERVERS,
      tool_server_versions: KUBERNETES_MCP_TOOL_SERVER_VERSIONS,
    }),
  },
];
