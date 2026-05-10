import { benchRunsPagePath } from "./routes.mts";

export interface ToolServerCompareFilters {
  model: string;
  toolServer: string;
  toolServerVersion?: string;
  scenarioIds?: string[];
}

export function buildToolServerComparePath(filters: ToolServerCompareFilters): string {
  const params = new URLSearchParams();
  params.set("model", filters.model);
  params.set("tool_server", filters.toolServer);
  if (filters.toolServerVersion) {
    params.set("tool_server_version", filters.toolServerVersion);
  }
  if (filters.scenarioIds && filters.scenarioIds.length > 0) {
    params.set("scenarios", filters.scenarioIds.join(","));
  }
  return `/v1/bench/compare/tool-server?${params.toString()}`;
}

export interface ToolServerRunsLinkFilters {
  side: "baseline" | "candidate";
  model: string;
  scenarioId: string;
  toolServer: string;
  toolServerVersion?: string;
}

export function toolServerRunsPagePath(filters: ToolServerRunsLinkFilters): string {
  if (filters.side === "baseline") {
    return benchRunsPagePath({
      scenario: filters.scenarioId,
      model: filters.model,
      evidence_mode: "none",
    });
  }
  return benchRunsPagePath({
    scenario: filters.scenarioId,
    model: filters.model,
    evidence_mode: "mcp",
    tool_server: filters.toolServer,
    tool_server_version: filters.toolServerVersion,
  });
}
