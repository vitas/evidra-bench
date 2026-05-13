import assert from "node:assert/strict";
import test from "node:test";

import {
  buildToolServerComparePath,
  buildToolServerReportApiPath,
  toolServerRunsPagePath,
} from "./toolServerCompare.mts";

test("buildToolServerComparePath encodes required and optional filters", () => {
  assert.equal(
    buildToolServerComparePath({
      model: "qwen plus",
      toolServer: "kubernetes-mcp",
      toolServerVersion: "1.2.3+build 4",
      scenarioIds: ["broken-deployment", "stuck rollout"],
    }),
    "/v1/bench/compare/tool-server?model=qwen+plus&tool_server=kubernetes-mcp&tool_server_version=1.2.3%2Bbuild+4&scenarios=broken-deployment%2Cstuck+rollout",
  );
});

test("buildToolServerComparePath omits empty optional filters", () => {
  assert.equal(
    buildToolServerComparePath({
      model: "sonnet",
      toolServer: "kubernetes-mcp",
      toolServerVersion: "",
      scenarioIds: [],
    }),
    "/v1/bench/compare/tool-server?model=sonnet&tool_server=kubernetes-mcp",
  );
});

test("buildToolServerReportApiPath builds JSON and markdown report endpoints", () => {
  assert.equal(
    buildToolServerReportApiPath({
      model: "qwen plus",
      toolServer: "kubernetes-mcp",
      toolServerVersion: "1.2.3+build 4",
      scenarioIds: ["broken-deployment", "stuck rollout"],
    }),
    "/v1/bench/reports/tool-server?model=qwen+plus&tool_server=kubernetes-mcp&tool_server_version=1.2.3%2Bbuild+4&scenarios=broken-deployment%2Cstuck+rollout",
  );

  assert.equal(
    buildToolServerReportApiPath({
      model: "qwen-plus",
      toolServer: "kubernetes-mcp",
      format: "markdown",
    }),
    "/v1/bench/reports/tool-server?model=qwen-plus&tool_server=kubernetes-mcp&format=markdown",
  );
});

test("toolServerRunsPagePath links baseline and candidate run slices", () => {
  assert.equal(
    toolServerRunsPagePath({
      side: "baseline",
      model: "qwen-plus",
      scenarioId: "resource pressure",
      toolServer: "kubernetes-mcp",
      toolServerVersion: "1.2.3",
    }),
    "/bench/runs?scenario=resource+pressure&model=qwen-plus&tool_server_unset=true",
  );

  assert.equal(
    toolServerRunsPagePath({
      side: "candidate",
      model: "qwen-plus",
      scenarioId: "resource pressure",
      toolServer: "kubernetes-mcp",
      toolServerVersion: "1.2.3",
    }),
    "/bench/runs?scenario=resource+pressure&model=qwen-plus&tool_server=kubernetes-mcp&tool_server_version=1.2.3",
  );
});
