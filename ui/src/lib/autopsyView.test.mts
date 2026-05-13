import assert from "node:assert/strict";
import test from "node:test";

import { formatFailureKind, normalizeAutopsyReport } from "./autopsyView.mts";

test("formatFailureKind renders autopsy identifiers for display", () => {
  assert.equal(formatFailureKind("unsafe_action"), "Unsafe Action");
  assert.equal(formatFailureKind(undefined), "none");
});

test("normalizeAutopsyReport supports autopsy v1 waste and structured evidence", () => {
  const view = normalizeAutopsyReport({
    version: "autopsy.v1",
    outcome: "fail",
    primary_failure: "unsafe_action",
    summary: "Run failed with primary failure unsafe_action.",
    confidence: "medium",
    waste: {
      turns: 4,
      tokens: 1234,
      basis: "retry_loop",
    },
    metrics: {
      turns: 9,
      prompt_tokens: 1000,
      completion_tokens: 234,
      total_tokens: 1234,
      estimated_cost_usd: 0.042,
      checks_passed: 1,
      checks_total: 3,
      mutation_count: 2,
      diagnosis_depth: 1,
      total_steps: 8,
    },
    findings: [
      {
        kind: "unsafe_action",
        severity: "critical",
        message: "Agent performed forbidden action.",
        evidence: {
          artifact: "tool-calls",
          selector: "0",
          command: "kubectl delete namespace bench",
        },
      },
    ],
  });

  assert.equal(view.version, "autopsy.v1");
  assert.equal(view.confidence, "medium");
  assert.equal(view.primaryLabel, "Unsafe Action");
  assert.equal(view.waste.turns, 4);
  assert.equal(view.waste.tokens, 1234);
  assert.equal(view.waste.basis, "retry_loop");
  assert.equal(
    view.findings[0].evidenceText,
    "tool-calls 0 kubectl delete namespace bench",
  );
});

test("normalizeAutopsyReport preserves legacy wasted fields", () => {
  const view = normalizeAutopsyReport({
    outcome: "fail",
    primary_failure: "retry_loop",
    summary: "Run failed with primary failure retry_loop.",
    metrics: {
      turns: 6,
      prompt_tokens: 100,
      completion_tokens: 50,
      total_tokens: 150,
      estimated_cost_usd: 0.01,
      checks_passed: 0,
      checks_total: 1,
      mutation_count: 0,
      diagnosis_depth: 0,
      total_steps: 3,
    },
    wasted_turns: 2,
    wasted_tokens: 150,
    findings: [
      {
        kind: "retry_loop",
        severity: "warning",
        message: "Repeated the same command 3 times.",
        evidence: "kubectl get pods -n bench",
      },
    ],
  });

  assert.equal(view.version, "legacy");
  assert.equal(view.confidence, "unknown");
  assert.equal(view.waste.turns, 2);
  assert.equal(view.waste.tokens, 150);
  assert.equal(view.findings[0].evidenceText, "kubectl get pods -n bench");
});
