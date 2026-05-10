import assert from "node:assert/strict";
import test from "node:test";

import {
  BENCH_LEADERBOARD_PATH,
  BENCH_MCP_READINESS_PATH,
  BENCH_RUNS_PATH,
  BENCH_SAMPLE_REPORT_PATH,
  BENCH_SCENARIOS_PATH,
  BENCH_TOOL_SERVER_REPORT_PATH,
  benchMCPReadinessPagePath,
  benchLeaderboardPagePath,
  benchRunPath,
  benchRunsPagePath,
  benchSampleReportPagePath,
  benchScenariosPagePath,
  benchScenarioPath,
  benchToolServerReportPagePath,
} from "./routes.mts";

test("bench route constants use canonical bench paths", () => {
  assert.equal(BENCH_LEADERBOARD_PATH, "/bench/leaderboard");
  assert.equal(BENCH_MCP_READINESS_PATH, "/bench/mcp-readiness");
  assert.equal(BENCH_RUNS_PATH, "/bench/runs");
  assert.equal(BENCH_SAMPLE_REPORT_PATH, "/bench/sample-report");
  assert.equal(BENCH_SCENARIOS_PATH, "/bench/scenarios");
  assert.equal(BENCH_TOOL_SERVER_REPORT_PATH, "/bench/reports/tool-server");
});

test("bench detail helpers encode path ids", () => {
  assert.equal(benchRunPath("run 1"), "/bench/runs/run%201");
  assert.equal(benchScenarioPath("kubernetes/foo"), "/bench/scenarios/kubernetes%2Ffoo");
});

test("bench runs page helper appends encoded query parameters", () => {
  assert.equal(benchRunsPagePath(), "/bench/runs");
  assert.equal(
    benchRunsPagePath({ scenario: "missing secret", model: "claude/sonnet" }),
    "/bench/runs?scenario=missing+secret&model=claude%2Fsonnet",
  );
});

test("bench scenarios page helper appends encoded query parameters", () => {
  assert.equal(benchScenariosPagePath(), "/bench/scenarios");
  assert.equal(
    benchScenariosPagePath({ exam: "kubernetes-admin", q: "bad image" }),
    "/bench/scenarios?exam=kubernetes-admin&q=bad+image",
  );
});

test("bench leaderboard page helper appends encoded query parameters", () => {
  assert.equal(benchLeaderboardPagePath(), "/bench/leaderboard");
  assert.equal(
    benchLeaderboardPagePath({ exam: "kubernetes-security" }),
    "/bench/leaderboard?exam=kubernetes-security",
  );
});

test("bench MCP readiness page helper appends encoded query parameters", () => {
  assert.equal(benchMCPReadinessPagePath(), "/bench/mcp-readiness");
  assert.equal(
    benchMCPReadinessPagePath({ model: "qwen plus", tool_server: "kubernetes-mcp" }),
    "/bench/mcp-readiness?model=qwen+plus&tool_server=kubernetes-mcp",
  );
});

test("bench sample report page helper appends encoded query parameters", () => {
  assert.equal(benchSampleReportPagePath(), "/bench/sample-report");
  assert.equal(
    benchSampleReportPagePath({ source: "landing cta" }),
    "/bench/sample-report?source=landing+cta",
  );
});

test("bench tool server report page helper preserves report filters", () => {
  assert.equal(
    benchToolServerReportPagePath({
      model: "qwen plus",
      tool_server: "kubernetes-mcp",
      tool_server_version: "1.2.3+build 4",
      category: "kubernetes",
      scenarios: "broken-deployment,stuck rollout",
    }),
    "/bench/reports/tool-server?model=qwen+plus&tool_server=kubernetes-mcp&tool_server_version=1.2.3%2Bbuild+4&category=kubernetes&scenarios=broken-deployment%2Cstuck+rollout",
  );
});
