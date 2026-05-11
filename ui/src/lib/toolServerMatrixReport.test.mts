import assert from "node:assert/strict";
import test from "node:test";

import { buildToolServerMatrixReportApiPath } from "./toolServerMatrixReport.mts";

test("buildToolServerMatrixReportApiPath encodes public report filters", () => {
  assert.equal(
    buildToolServerMatrixReportApiPath({
      model: "qwen plus",
      reportId: "kubernetes-mcp-readiness-2026-05",
      toolServers: ["flux159-mcp-server-kubernetes", "containers-kubernetes-mcp-server"],
      toolServerVersions: ["1.0.0", "2.0.0+build 4"],
      scenarioIds: ["broken-deployment", "stuck rollout"],
    }),
    "/v1/bench/reports/tool-server-matrix?model=qwen+plus&report_id=kubernetes-mcp-readiness-2026-05&tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server&tool_server_versions=1.0.0%2C2.0.0%2Bbuild+4&scenarios=broken-deployment%2Cstuck+rollout",
  );
});

test("buildToolServerMatrixReportApiPath builds markdown endpoint", () => {
  assert.equal(
    buildToolServerMatrixReportApiPath({
      model: "sonnet",
      reportId: "kubernetes-mcp-readiness-2026-05",
      toolServers: ["flux159-mcp-server-kubernetes", "containers-kubernetes-mcp-server"],
      format: "markdown",
    }),
    "/v1/bench/reports/tool-server-matrix?model=sonnet&report_id=kubernetes-mcp-readiness-2026-05&tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server&format=markdown",
  );
});
