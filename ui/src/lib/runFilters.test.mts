import assert from "node:assert/strict";
import test from "node:test";

import {
  buildRunsAPIPath,
  runsFiltersFromSearchParams,
  runsSearchParamsFromFilters,
} from "./runFilters.mts";

test("runs filters are initialized from URL search params", () => {
  const filters = runsFiltersFromSearchParams(
    new URLSearchParams(
      "model=claude-sonnet&scenario=broken-deployment&exam=kubernetes-security&provider=anthropic&tool_server_version=1.2.3&passed=true&since=2026-05-01",
    ),
  );

  assert.deepEqual(filters, {
    scenario: "broken-deployment",
    exam: "kubernetes-security",
    model: "claude-sonnet",
    provider: "anthropic",
    toolServer: "All",
    toolServerVersion: "1.2.3",
    toolServerUnset: false,
    status: "Passed",
    since: "2026-05-01",
  });
});

test("runs filters include tool server from URL search params", () => {
  const filters = runsFiltersFromSearchParams(new URLSearchParams("tool_server=kubernetes-mcp"));

  assert.equal(filters.toolServer, "kubernetes-mcp");
});

test("runs filters include baseline/native tool-server unset from URL search params", () => {
  const filters = runsFiltersFromSearchParams(new URLSearchParams("tool_server_unset=true"));

  assert.equal(filters.toolServerUnset, true);
});

test("runs filters ignore unknown exam ids", () => {
  const filters = runsFiltersFromSearchParams(new URLSearchParams("exam=unknown-suite&passed=false"));

  assert.equal(filters.exam, "all");
  assert.equal(filters.status, "Failed");
});

test("runs filters serialize only active URL filters", () => {
  const params = runsSearchParamsFromFilters({
    scenario: "",
    exam: "kubernetes-security",
    model: "claude-sonnet",
    provider: "All",
    toolServer: "kubernetes-mcp",
    toolServerVersion: "1.2.3",
    toolServerUnset: false,
    status: "All",
    since: "",
  });

  assert.equal(
    params.toString(),
    "exam=kubernetes-security&model=claude-sonnet&tool_server=kubernetes-mcp&tool_server_version=1.2.3",
  );
});

test("runs API path maps selected exam suite to scenario ids", () => {
  const path = buildRunsAPIPath(
    {
      scenario: "",
      exam: "kubernetes-security",
      model: "claude-sonnet",
      provider: "All",
      toolServer: "kubernetes-mcp",
      toolServerVersion: "1.2.3",
      toolServerUnset: false,
      status: "All",
      since: "",
    },
    1,
    ["s1", "s2"],
    25,
  );

  assert.equal(
    path,
    "/v1/bench/runs?scenarios=s1%2Cs2&model=claude-sonnet&tool_server=kubernetes-mcp&tool_server_version=1.2.3&limit=25&offset=25",
  );
});

test("runs API path supports exact baseline/native tool-server unset filter", () => {
  const path = buildRunsAPIPath(
    {
      scenario: "",
      exam: "all",
      model: "claude-sonnet",
      provider: "All",
      toolServer: "All",
      toolServerVersion: "All",
      toolServerUnset: true,
      status: "All",
      since: "",
    },
    0,
    [],
    25,
  );

  assert.equal(
    path,
    "/v1/bench/runs?model=claude-sonnet&tool_server_unset=true&limit=25&offset=0",
  );
});

test("runs API path keeps explicit scenario above exam suite ids", () => {
  const path = buildRunsAPIPath(
    {
      scenario: "specific-scenario",
      exam: "kubernetes-security",
      model: "All",
      provider: "All",
      toolServer: "All",
      toolServerVersion: "All",
      toolServerUnset: false,
      status: "All",
      since: "",
    },
    0,
    ["s1", "s2"],
    25,
  );

  assert.equal(path, "/v1/bench/runs?scenario=specific-scenario&limit=25&offset=0");
});
