export interface CatalogResponse {
  models: string[];
  providers: string[];
}

function normalizeList(values: string[]): string[] {
  return [...new Set(values.map((value) => value.trim()).filter(Boolean))].sort();
}

export function normalizeCatalog(catalog: CatalogResponse): CatalogResponse {
  return {
    models: normalizeList(catalog.models),
    providers: normalizeList(catalog.providers),
  };
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

export function buildRunsPath(limit: number, since?: string, mode?: string): string {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  if (since) {
    params.set("since", since);
  }
  applyEvidenceMode(params, mode);
  return `/v1/bench/runs?${params.toString()}`;
}
