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

export function buildRunsPath(limit: number, since?: string): string {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  if (since) {
    params.set("since", since);
  }
  return `/v1/bench/runs?${params.toString()}`;
}
