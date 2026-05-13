export function resolveRunsLimit(totalRuns: number): number {
  return Math.max(totalRuns, 1);
}
