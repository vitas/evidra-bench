import assert from "node:assert/strict";
import test from "node:test";

import { BENCH_FEATURE_NAV, BENCH_PRIMARY_NAV } from "./benchNav.mts";

test("bench primary nav stays focused on newcomer workflows", () => {
  assert.deepEqual(
    BENCH_PRIMARY_NAV.map((item) => item.label),
    ["Leaderboard", "Runs", "Scenarios", "Reports", "Benchmarks", "Compare"],
  );
});

test("bench feature nav exposes online bench operations", () => {
  assert.deepEqual(
    BENCH_FEATURE_NAV.map((item) => item.label),
    ["Dashboard", "Skill Impact", "Regressions", "Insights", "Reviews", "Session"],
  );
});
