export const BENCH_RUNS_PATH = "/bench/runs";
export const BENCH_SCENARIOS_PATH = "/bench/scenarios";

export function benchRunPath(id: string) {
  return `${BENCH_RUNS_PATH}/${encodeURIComponent(id)}`;
}

export function benchScenarioPath(id: string) {
  return `${BENCH_SCENARIOS_PATH}/${encodeURIComponent(id)}`;
}

export function benchRunsPagePath(params?: Record<string, string | undefined>) {
  const search = new URLSearchParams();
  for (const [key, value] of Object.entries(params ?? {})) {
    if (value) {
      search.set(key, value);
    }
  }
  const query = search.toString();
  return query ? `${BENCH_RUNS_PATH}?${query}` : BENCH_RUNS_PATH;
}
