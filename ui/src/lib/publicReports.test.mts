import assert from "node:assert/strict";
import test from "node:test";

import {
  LANDING_PUBLIC_REPORTS,
  PUBLIC_REPORT_DEFAULTS,
  publicReportDefaults,
} from "./publicReports.mts";

test("landing public reports include primary and pilot reports", () => {
  assert.deepEqual(
    LANDING_PUBLIC_REPORTS.map((report) => report.id),
    [
      "kubernetes-mcp-readiness-2026-05-public",
      "kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot",
    ],
  );
});

test("landing public report links preserve report filters", () => {
  const [primary, pilot] = LANDING_PUBLIC_REPORTS;

  assert.equal(
    primary.to,
    "/bench/reports/kubernetes-mcp-readiness-2026-05-public?model=claude-sonnet-4-6&scenarios=broken-deployment%2Cservice-port-mismatch%2Cnetwork-policy-fix%2Cnetworkpolicy-blocking%2Cfalse-alarm%2Cdelete-prod-namespace%2Curgency-vs-safety%2Csafe-rollback-vs-broad-patch%2Cshared-configmap-trap%2Ccross-namespace-secret-access&tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server&tool_server_versions=npm%3Amcp-server-kubernetes%403.5.1%2Cnpm%3Akubernetes-mcp-server%400.0.62",
  );
  assert.equal(
    pilot.to,
    "/bench/reports/kubernetes-mcp-readiness-2026-05-deepseek-v4-flash-pilot?model=deepseek-v4-flash&scenarios=broken-deployment%2Cfalse-alarm%2Cshared-configmap-trap&tool_servers=flux159-mcp-server-kubernetes%2Ccontainers-kubernetes-mcp-server&tool_server_versions=npm%3Amcp-server-kubernetes%403.5.1%2Cnpm%3Akubernetes-mcp-server%400.0.62",
  );
});

test("landing public reports have unique ids and matching defaults", () => {
  const ids = LANDING_PUBLIC_REPORTS.map((report) => report.id);
  assert.equal(new Set(ids).size, ids.length);
  for (const id of ids) {
    assert.ok(PUBLIC_REPORT_DEFAULTS[id], `missing defaults for ${id}`);
  }
});

test("unknown public report falls back to the primary report defaults", () => {
  assert.equal(publicReportDefaults("missing-report").reportId, "kubernetes-mcp-readiness-2026-05-public");
});
