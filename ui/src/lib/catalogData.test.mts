import assert from "node:assert/strict";
import test from "node:test";

import {
  buildLeaderboardPath,
  buildRunsPath,
  coerceToolServerVersion,
  normalizeCatalog,
  toolServerVersionOptions,
} from "./catalogData.mts";

test("normalizeCatalog deduplicates and sorts models/providers", () => {
  const catalog = normalizeCatalog({
    models: ["sonnet", "haiku", "sonnet", ""],
    providers: ["bifrost", "claude", "bifrost", ""],
    tool_servers: ["kubernetes-mcp", "legacy-mcp", "kubernetes-mcp", ""],
    tool_server_versions: ["1.2.4", "1.2.3", "1.2.3", ""],
    tool_server_versions_by_server: {
      "legacy-mcp": ["0.9.0", "0.9.0", ""],
      "kubernetes-mcp": ["1.2.4", "1.2.3"],
    },
  });

  assert.deepEqual(catalog.models, ["haiku", "sonnet"]);
  assert.deepEqual(catalog.providers, ["bifrost", "claude"]);
  assert.deepEqual(catalog.tool_servers, ["kubernetes-mcp", "legacy-mcp"]);
  assert.deepEqual(catalog.tool_server_versions, ["1.2.3", "1.2.4"]);
  assert.deepEqual(catalog.tool_server_versions_by_server, {
    "kubernetes-mcp": ["1.2.3", "1.2.4"],
    "legacy-mcp": ["0.9.0"],
  });
});

test("toolServerVersionOptions narrows versions to the selected tool server", () => {
  const catalog = normalizeCatalog({
    models: [],
    providers: [],
    tool_servers: ["kubernetes-mcp", "legacy-mcp"],
    tool_server_versions: ["0.9.0", "1.2.3"],
    tool_server_versions_by_server: {
      "kubernetes-mcp": ["1.2.3"],
      "legacy-mcp": ["0.9.0"],
    },
  });

  assert.deepEqual(toolServerVersionOptions(catalog, "kubernetes-mcp"), ["1.2.3"]);
  assert.deepEqual(toolServerVersionOptions(catalog, "legacy-mcp"), ["0.9.0"]);
  assert.deepEqual(toolServerVersionOptions(catalog, "All"), ["0.9.0", "1.2.3"]);
});

test("coerceToolServerVersion resets versions from a different tool server", () => {
  const catalog = normalizeCatalog({
    models: [],
    providers: [],
    tool_servers: ["kubernetes-mcp", "legacy-mcp"],
    tool_server_versions: ["0.9.0", "1.2.3"],
    tool_server_versions_by_server: {
      "kubernetes-mcp": ["1.2.3"],
      "legacy-mcp": ["0.9.0"],
    },
  });

  assert.equal(coerceToolServerVersion(catalog, "kubernetes-mcp", "1.2.3", "All"), "1.2.3");
  assert.equal(coerceToolServerVersion(catalog, "kubernetes-mcp", "0.9.0", "All"), "All");
  assert.equal(coerceToolServerVersion(catalog, "All", "0.9.0", "All"), "0.9.0");
  assert.equal(coerceToolServerVersion(catalog, "legacy-mcp", "1.2.3", ""), "");
});

test("buildRunsPath applies limit and optional since filter", () => {
  assert.equal(buildRunsPath(8), "/v1/bench/runs?limit=8");
  assert.equal(
    buildRunsPath(8, "2026-03-17T10:00:00Z"),
    "/v1/bench/runs?limit=8&since=2026-03-17T10%3A00%3A00Z",
  );
});

test("buildLeaderboardPath applies evidence mode and scenario filters", () => {
  assert.equal(buildLeaderboardPath(3), "/v1/bench/leaderboard?k=3");
  assert.equal(
    buildLeaderboardPath(3, "mcp", ["s1", "s2"]),
    "/v1/bench/leaderboard?k=3&scenarios=s1%2Cs2&evidence_mode=mcp",
  );
});
