import assert from "node:assert/strict";
import test from "node:test";

import {
  BENCH_LEADERBOARD_PATH,
  BENCH_RUNS_PATH,
  BENCH_SCENARIOS_PATH,
  benchLeaderboardPagePath,
  benchRunPath,
  benchRunsPagePath,
  benchScenariosPagePath,
  benchScenarioPath,
} from "./routes.mts";

test("bench route constants use canonical bench paths", () => {
  assert.equal(BENCH_LEADERBOARD_PATH, "/bench/leaderboard");
  assert.equal(BENCH_RUNS_PATH, "/bench/runs");
  assert.equal(BENCH_SCENARIOS_PATH, "/bench/scenarios");
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
