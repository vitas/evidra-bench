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

export function buildLeaderboardPath(k: number, scenarios?: string[]): string {
  const params = new URLSearchParams();
  params.set("k", String(k));
  if (scenarios && scenarios.length > 0) {
    params.set("scenarios", scenarios.join(","));
  }
  return `/v1/bench/leaderboard?${params.toString()}`;
}

export function buildRunsPath(limit: number, since?: string): string {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  if (since) {
    params.set("since", since);
  }
  return `/v1/bench/runs?${params.toString()}`;
}
