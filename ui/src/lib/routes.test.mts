import assert from "node:assert/strict";
import test from "node:test";

import {
  BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
  BENCH_ARTICLE_PASS_FAIL_PATH,
  BENCH_LEADERBOARD_PATH,
  BENCH_MCP_READINESS_PATH,
  BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH,
  BENCH_REVIEWS_PATH,
  BENCH_RUNS_PATH,
  BENCH_SAMPLE_REPORT_PATH,
  BENCH_SCENARIOS_PATH,
  BENCH_SESSION_PATH,
  BENCH_TOOL_SERVER_REPORT_PATH,
  benchAiSreBenchmarkArticlePagePath,
  benchPassFailArticlePagePath,
  benchPublicKubernetesMCPReportPagePath,
  benchMCPReadinessPagePath,
  benchLeaderboardPagePath,
  benchRunPath,
  benchRunsPagePath,
  benchSampleReportPagePath,
  benchScenariosPagePath,
  benchScenarioPath,
  benchToolServerMatrixReportPagePath,
  benchToolServerReportPagePath,
} from "./routes.mts";

test("bench route constants use canonical bench paths", () => {
  assert.equal(
    BENCH_ARTICLE_AI_SRE_BENCHMARK_PATH,
    "/bench/articles/what-ai-sre-benchmarks-should-catch-before-production",
  );
  assert.equal(BENCH_ARTICLE_PASS_FAIL_PATH, "/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough");
  assert.equal(BENCH_LEADERBOARD_PATH, "/bench/leaderboard");
  assert.equal(BENCH_MCP_READINESS_PATH, "/bench/mcp-readiness");
  assert.equal(BENCH_PUBLIC_KUBERNETES_MCP_REPORT_PATH, "/bench/reports/kubernetes-mcp-readiness-2026-05");
  assert.equal(BENCH_REVIEWS_PATH, "/bench/reviews");
  assert.equal(BENCH_RUNS_PATH, "/bench/runs");
  assert.equal(BENCH_SAMPLE_REPORT_PATH, "/bench/sample-report");
  assert.equal(BENCH_SCENARIOS_PATH, "/bench/scenarios");
  assert.equal(BENCH_SESSION_PATH, "/bench/session");
  assert.equal(BENCH_TOOL_SERVER_REPORT_PATH, "/bench/reports/tool-server");
});

test("bench AI SRE benchmark article helper appends encoded query parameters", () => {
  assert.equal(
    benchAiSreBenchmarkArticlePagePath({ source: "buyers guide" }),
    "/bench/articles/what-ai-sre-benchmarks-should-catch-before-production?source=buyers+guide",
  );
});

test("bench pass/fail article helper appends encoded query parameters", () => {
  assert.equal(
    benchPassFailArticlePagePath({ source: "landing article" }),
    "/bench/articles/kubernetes-mcp-servers-passed-that-was-not-enough?source=landing+article",
  );
});

test("bench public Kubernetes MCP report helper appends encoded query parameters", () => {
  assert.equal(benchPublicKubernetesMCPReportPagePath(), "/bench/reports/kubernetes-mcp-readiness-2026-05");
  assert.equal(
    benchPublicKubernetesMCPReportPagePath({ variant: "pilot run" }),
    "/bench/reports/kubernetes-mcp-readiness-2026-05?variant=pilot+run",
  );
});

test("bench tool server matrix report helper encodes report id and filters", () => {
  assert.equal(
    benchToolServerMatrixReportPagePath("kubernetes-mcp-readiness-2026-05-pilot", {
      model: "qwen plus",
      tool_servers: "flux159-mcp-server-kubernetes,containers-kubernetes-mcp-server",
    }),
    "/bench/reports/kubernetes-mcp-readiness-2026-05-pilot?model=qwen+plus&tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server",
  );
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
