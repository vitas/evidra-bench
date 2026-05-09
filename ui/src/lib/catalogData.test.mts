import assert from "node:assert/strict";
import test from "node:test";

import { buildLeaderboardPath, buildRunsPath, normalizeCatalog } from "./catalogData.mts";

test("normalizeCatalog deduplicates and sorts models/providers", () => {
  const catalog = normalizeCatalog({
    models: ["sonnet", "haiku", "sonnet", ""],
    providers: ["bifrost", "claude", "bifrost", ""],
  });

  assert.deepEqual(catalog.models, ["haiku", "sonnet"]);
  assert.deepEqual(catalog.providers, ["bifrost", "claude"]);
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
