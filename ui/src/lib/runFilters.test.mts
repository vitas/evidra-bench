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
      "model=claude-sonnet&scenario=broken-deployment&exam=kubernetes-security&provider=anthropic&passed=true&since=2026-05-01",
    ),
  );

  assert.deepEqual(filters, {
    scenario: "broken-deployment",
    exam: "kubernetes-security",
    model: "claude-sonnet",
    provider: "anthropic",
    status: "Passed",
    since: "2026-05-01",
  });
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
    status: "All",
    since: "",
  });

  assert.equal(params.toString(), "exam=kubernetes-security&model=claude-sonnet");
});

test("runs API path maps selected exam suite to scenario ids", () => {
  const path = buildRunsAPIPath(
    {
      scenario: "",
      exam: "kubernetes-security",
      model: "claude-sonnet",
      provider: "All",
      status: "All",
      since: "",
    },
    1,
    "mcp",
    ["s1", "s2"],
    25,
  );

  assert.equal(
    path,
    "/v1/bench/runs?scenarios=s1%2Cs2&model=claude-sonnet&limit=25&offset=25&evidence_mode=mcp",
  );
});

test("runs API path keeps explicit scenario above exam suite ids", () => {
  const path = buildRunsAPIPath(
    {
      scenario: "specific-scenario",
      exam: "kubernetes-security",
      model: "All",
      provider: "All",
      status: "All",
      since: "",
    },
    0,
    "all",
    ["s1", "s2"],
    25,
  );

  assert.equal(path, "/v1/bench/runs?scenario=specific-scenario&limit=25&offset=0");
});
