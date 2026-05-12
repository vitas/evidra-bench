export interface CatalogResponse {
  models: string[];
  providers: string[];
  tool_servers: string[];
  tool_server_versions?: string[];
  tool_server_versions_by_server?: Record<string, string[]>;
}

function normalizeList(values: string[]): string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))].sort();
}

function normalizeVersionMap(values?: Record<string, string[]>): Record<string, string[]> {
  const normalized: Record<string, string[]> = {};
  for (const [toolServer, versions] of Object.entries(values ?? {})) {
    const server = toolServer.trim();
    if (!server) continue;
    normalized[server] = normalizeList(versions ?? []);
  }
  return normalized;
}

export function normalizeCatalog(catalog: CatalogResponse): CatalogResponse {
  return {
    models: normalizeList(catalog.models),
    providers: normalizeList(catalog.providers),
    tool_servers: normalizeList(catalog.tool_servers ?? []),
    tool_server_versions: normalizeList(catalog.tool_server_versions ?? []),
    tool_server_versions_by_server: normalizeVersionMap(catalog.tool_server_versions_by_server),
  };
}

export function toolServerVersionOptions(catalog: CatalogResponse, toolServer: string): string[] {
  if (!toolServer || toolServer === "All") {
    return catalog.tool_server_versions ?? [];
  }
  const versionsByServer = catalog.tool_server_versions_by_server ?? {};
  if (Object.keys(versionsByServer).length === 0) {
    return catalog.tool_server_versions ?? [];
  }
  const versions = versionsByServer[toolServer];
  return versions && versions.length > 0 ? versions : [];
}

export function coerceToolServerVersion(
  catalog: CatalogResponse,
  toolServer: string,
  version: string,
  emptyValue: string,
): string {
  if (!version || version === emptyValue) return emptyValue;
  if (!toolServer || toolServer === "All") return version;
  return toolServerVersionOptions(catalog, toolServer).includes(version) ? version : emptyValue;
}

/** Default evidence mode for all API queries. "all" = no filter. */
export const DEFAULT_EVIDENCE_MODE = "all";

/** Append evidence_mode param to a URLSearchParams object. */
export function applyEvidenceMode(params: URLSearchParams, mode?: string): void {
  const m = mode ?? DEFAULT_EVIDENCE_MODE;
  if (m && m !== "all") params.set("evidence_mode", m);
}

/** Build query string fragment: ?evidence_mode=X or &evidence_mode=X. Empty for "all". */
export function evidenceModeParam(prefix: "?" | "&", mode?: string): string {
  const m = mode ?? DEFAULT_EVIDENCE_MODE;
  return m && m !== "all" ? `${prefix}evidence_mode=${encodeURIComponent(m)}` : "";
}

export function buildLeaderboardPath(k: number, mode?: string, scenarios?: string[]): string {
  const params = new URLSearchParams();
  params.set("k", String(k));
  if (scenarios && scenarios.length > 0) {
    params.set("scenarios", scenarios.join(","));
  }
  applyEvidenceMode(params, mode);
  return `/v1/bench/leaderboard?${params.toString()}`;
}

export function buildRunsPath(limit: number, since?: string, mode?: string): string {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  if (since) {
    params.set("since", since);
  }
  applyEvidenceMode(params, mode);
  return `/v1/bench/runs?${params.toString()}`;
}
