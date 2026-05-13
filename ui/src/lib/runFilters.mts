import { resolveExamPackFilter, type ExamPackFilter } from "./examPacks.mts";

export type RunsStatus = "All" | "Passed" | "Failed";

export interface RunsFilterState {
  scenario: string;
  exam: ExamPackFilter;
  model: string;
  provider: string;
  toolServer: string;
  toolServerVersion: string;
  toolServerUnset: boolean;
  skillID: string;
  skillVersion: string;
  skillUnset: boolean;
  status: RunsStatus;
  since: string;
}

export const DEFAULT_RUNS_FILTERS: RunsFilterState = {
  scenario: "",
  exam: "all",
  model: "All",
  provider: "All",
  toolServer: "All",
  toolServerVersion: "All",
  toolServerUnset: false,
  skillID: "All",
  skillVersion: "All",
  skillUnset: false,
  status: "All",
  since: "",
};

function statusFromPassedParam(value: string | null): RunsStatus {
  if (value === "true") return "Passed";
  if (value === "false") return "Failed";
  return "All";
}

export function runsFiltersFromSearchParams(params: URLSearchParams): RunsFilterState {
  return {
    scenario: params.get("scenario") ?? "",
    exam: resolveExamPackFilter(params.get("exam")),
    model: params.get("model") || "All",
    provider: params.get("provider") || "All",
    toolServer: params.get("tool_server") || "All",
    toolServerVersion: params.get("tool_server_version") || "All",
    toolServerUnset: params.get("tool_server_unset") === "true",
    skillID: params.get("skill_id") || "All",
    skillVersion: params.get("skill_version") || "All",
    skillUnset: params.get("skill_unset") === "true",
    status: statusFromPassedParam(params.get("passed")),
    since: params.get("since") ?? "",
  };
}

export function runsSearchParamsFromFilters(filters: RunsFilterState): URLSearchParams {
  const params = new URLSearchParams();
  const scenario = filters.scenario.trim();

  if (filters.exam !== "all") params.set("exam", filters.exam);
  if (scenario) params.set("scenario", scenario);
  if (filters.model !== "All") params.set("model", filters.model);
  if (filters.provider !== "All") params.set("provider", filters.provider);
  if (filters.toolServer !== "All") params.set("tool_server", filters.toolServer);
  if (filters.toolServerVersion !== "All") params.set("tool_server_version", filters.toolServerVersion);
  if (filters.toolServerUnset) params.set("tool_server_unset", "true");
  if (filters.skillID !== "All") params.set("skill_id", filters.skillID);
  if (filters.skillVersion !== "All") params.set("skill_version", filters.skillVersion);
  if (filters.skillUnset) params.set("skill_unset", "true");
  if (filters.status === "Passed") params.set("passed", "true");
  if (filters.status === "Failed") params.set("passed", "false");
  if (filters.since) params.set("since", filters.since);

  return params;
}

export function buildRunsAPIPath(
  filters: RunsFilterState,
  page: number,
  suiteScenarioIDs: string[],
  pageSize: number,
): string {
  const params = new URLSearchParams();
  const scenario = filters.scenario.trim();

  if (scenario) {
    params.set("scenario", scenario);
  } else if (filters.exam !== "all" && suiteScenarioIDs.length > 0) {
    params.set("scenarios", suiteScenarioIDs.join(","));
  }

  if (filters.model !== "All") params.set("model", filters.model);
  if (filters.provider !== "All") params.set("provider", filters.provider);
  if (filters.toolServerUnset) {
    params.set("tool_server_unset", "true");
  } else if (filters.toolServer !== "All") {
    params.set("tool_server", filters.toolServer);
  }
  if (filters.toolServerVersion !== "All") params.set("tool_server_version", filters.toolServerVersion);
  if (filters.skillUnset) {
    params.set("skill_unset", "true");
  } else if (filters.skillID !== "All") {
    params.set("skill_id", filters.skillID);
  }
  if (filters.skillVersion !== "All") params.set("skill_version", filters.skillVersion);
  if (filters.status === "Passed") params.set("passed", "true");
  if (filters.status === "Failed") params.set("passed", "false");
  if (filters.since) params.set("since", filters.since);
  params.set("limit", String(pageSize));
  params.set("offset", String(page * pageSize));

  return `/v1/bench/runs?${params.toString()}`;
}
