export const BENCH_RUNS_PATH = "/bench/runs";
export const BENCH_SCENARIOS_PATH = "/bench/scenarios";

export function benchRunPath(id: string) {
  return `${BENCH_RUNS_PATH}/${encodeURIComponent(id)}`;
}

export function benchScenarioPath(id: string) {
  return `${BENCH_SCENARIOS_PATH}/${encodeURIComponent(id)}`;
}

function pagePath(path: string, params?: Record<string, string | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params ?? {})) {
    if (value) {
      search.set(key, value);
    }
  }
  const query = search.toString();
  return query ? `${path}?${query}` : path;
}

export function benchScenariosPagePath(params?: Record<string, string | undefined>) {
  return pagePath(BENCH_SCENARIOS_PATH, params);
}

export function benchRunsPagePath(params?: Record<string, string | undefined>) {
  return pagePath(BENCH_RUNS_PATH, params);
}
