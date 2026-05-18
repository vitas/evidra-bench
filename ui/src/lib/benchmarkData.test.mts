import assert from "node:assert/strict";
import test from "node:test";

import {
  armSafetySummary,
  benchmarkSafetySummary,
  resolveRunsLimit,
  scenarioSafetySummary,
  sortBenchmarkArms,
} from "./benchmarkData.mts";
import type {
  ToolServerMatrixArm,
  ToolServerMatrixScenario,
} from "./toolServerMatrixReport.mts";

test("resolveRunsLimit requests all available rows when totals are known", () => {
  assert.equal(resolveRunsLimit(237), 237);
});

test("resolveRunsLimit never asks the API for zero rows", () => {
  assert.equal(resolveRunsLimit(0), 1);
});

const aggregate = {
  runs: 1,
  passed: 1,
  pass_rate: 100,
  avg_turns: 10,
  avg_tokens: 1000,
  avg_cost_usd: 0.01,
  avg_duration_seconds: 30,
};

const scenarios: ToolServerMatrixScenario[] = [
  {
    id: "false-alarm",
    arms: [
      { arm_id: "baseline", label: "Baseline", classification: "baseline", result: "passed", aggregate },
      { arm_id: "flux", label: "Flux", classification: "safe_pass", result: "passed", aggregate },
      { arm_id: "containers", label: "Containers", classification: "unsafe_pass", result: "passed", aggregate },
    ],
  },
  {
    id: "broken-deployment",
    arms: [
      { arm_id: "baseline", label: "Baseline", classification: "baseline", result: "passed", aggregate },
      { arm_id: "flux", label: "Flux", classification: "safe_pass", result: "passed", aggregate },
      { arm_id: "containers", label: "Containers", classification: "fail", result: "failed", aggregate },
    ],
  },
  {
    id: "network-policy-fix",
    arms: [
      { arm_id: "baseline", label: "Baseline", classification: "baseline", result: "passed", aggregate },
      { arm_id: "flux", label: "Flux", classification: "missing_evidence", result: "passed", aggregate },
      { arm_id: "containers", label: "Containers", classification: "safe_pass", result: "passed", aggregate },
    ],
  },
];

test("scenarioSafetySummary counts candidate classifications", () => {
  assert.deepEqual(scenarioSafetySummary(scenarios[0]), {
    candidateCells: 2,
    safePass: 1,
    unsafePass: 1,
    fail: 0,
    missingEvidence: 0,
  });
});

test("benchmarkSafetySummary counts unsafe pass cells across scenarios", () => {
  assert.deepEqual(benchmarkSafetySummary(scenarios), {
    candidateCells: 6,
    safePass: 3,
    unsafePass: 1,
    fail: 1,
    missingEvidence: 1,
  });
});

test("armSafetySummary counts one selected arm across scenarios", () => {
  assert.deepEqual(armSafetySummary("containers", scenarios), {
    candidateCells: 3,
    safePass: 1,
    unsafePass: 1,
    fail: 1,
    missingEvidence: 0,
  });
});

test("sortBenchmarkArms keeps baseline first by default", () => {
  const arms: ToolServerMatrixArm[] = [
    { id: "containers", label: "Containers", kind: "candidate", aggregate },
    { id: "baseline", label: "Baseline", kind: "baseline", aggregate },
    { id: "flux", label: "Flux", kind: "candidate", aggregate },
  ];

  assert.deepEqual(sortBenchmarkArms(arms, scenarios).map((arm) => arm.id), [
    "baseline",
    "flux",
    "containers",
  ]);
});

test("sortBenchmarkArms can rank candidates by safe passes before baseline", () => {
  const arms: ToolServerMatrixArm[] = [
    { id: "baseline", label: "Baseline", kind: "baseline", aggregate },
    { id: "containers", label: "Containers", kind: "candidate", aggregate },
    { id: "flux", label: "Flux", kind: "candidate", aggregate },
  ];

  assert.deepEqual(sortBenchmarkArms(arms, scenarios, { baselineFirst: false }).map((arm) => arm.id), [
    "flux",
    "containers",
    "baseline",
  ]);
});
