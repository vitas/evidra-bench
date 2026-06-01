import assert from "node:assert/strict";
import test from "node:test";

import { BENCH_PRIMARY_NAV, BENCH_SECONDARY_NAV } from "./benchNav.mts";

test("bench primary nav stays focused on newcomer workflows", () => {
  assert.deepEqual(
    BENCH_PRIMARY_NAV.map((item) => item.label),
    ["Leaderboard", "Runs", "Scenarios", "Compare", "Reports"],
  );
});

test("bench secondary nav contains advanced operations", () => {
  assert.deepEqual(
    BENCH_SECONDARY_NAV.map((item) => item.label),
    ["Dashboard", "Skill Impact", "Regressions", "Insights", "Reviews", "Session", "Benchmarks"],
  );
});
