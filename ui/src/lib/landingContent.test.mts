import assert from "node:assert/strict";
import test from "node:test";

import {
  BENCHMARK_DIFFERENCES,
  HERO_CONTENT,
  HERO_PROOF_POINTS,
  PATH_COMPARISON,
  WORKFLOW_STEPS,
} from "./landingContent.mts";
import { BENCH_ONLINE_PATH } from "./routes.mts";

test("landing hero leads with path-aware infrastructure evaluation", () => {
  assert.equal(HERO_CONTENT.eyebrow, "Path-aware infrastructure benchmark");
  assert.equal(HERO_CONTENT.title, "A green check doesn't mean the agent was safe.");
  assert.match(HERO_CONTENT.body, /how the agent got there/i);
  assert.equal(HERO_CONTENT.ctas[0].label, "Open public Bench");
  assert.equal(HERO_CONTENT.ctas[0].href, BENCH_ONLINE_PATH);
  assert.equal(HERO_CONTENT.ctas[0].kind, "primary");
  assert.equal(HERO_CONTENT.ctas[1].label, "See a safe vs unsafe pass");
});

test("landing proves the difference between final state and action path", () => {
  assert.deepEqual(HERO_PROOF_POINTS, [
    "Final state + action path",
    "Safe pass vs unsafe pass",
    "Inspectable artifacts for every verdict",
  ]);

  assert.deepEqual(PATH_COMPARISON.rows, [
    { label: "Infrastructure restored", safe: "Pass", unsafe: "Pass" },
    { label: "Action path", safe: "Safe", unsafe: "Unsafe" },
    { label: "Wrong-scope changes", safe: "None", unsafe: "Production modified" },
    { label: "Verification", safe: "Complete", unsafe: "Skipped" },
  ]);
});

test("landing explains what ordinary benchmarks miss", () => {
  assert.deepEqual(
    BENCHMARK_DIFFERENCES.map((item) => item.label),
    ["Outcome", "Verdict", "Evidence", "Comparison target"],
  );
  for (const item of BENCHMARK_DIFFERENCES) {
    assert.ok(item.typical.length > 0);
    assert.ok(item.evidra.length > 0);
  }
});

test("landing explains the public Bench in three steps", () => {
  assert.equal(WORKFLOW_STEPS.length, 3);
  assert.deepEqual(
    WORKFLOW_STEPS.map((step) => step.title),
    ["Choose the stack", "Run a live incident", "Inspect the evidence"],
  );
});
