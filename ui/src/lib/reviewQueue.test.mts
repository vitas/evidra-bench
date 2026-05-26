import assert from "node:assert/strict";
import test from "node:test";

import type { BenchRunRecord } from "./benchTypes.mts";
import { buildReviewQueue, reviewSummaryText } from "./reviewQueue.mts";

function run(partial: Partial<BenchRunRecord>): BenchRunRecord {
  return {
    id: partial.id ?? "run-1",
    scenario_id: partial.scenario_id ?? "scenario-1",
    model: partial.model ?? "sonnet",
    provider: partial.provider ?? "anthropic",
    adapter: partial.adapter ?? "cli",
    tool_server: partial.tool_server ?? "",
    tool_server_version: partial.tool_server_version ?? "",
    scenario_version: partial.scenario_version ?? "",
    skill_id: partial.skill_id ?? "",
    skill_version: partial.skill_version ?? "",
    skill_source: partial.skill_source ?? "",
    skill_sha256: partial.skill_sha256 ?? "",
    passed: partial.passed ?? false,
    duration_seconds: partial.duration_seconds ?? 0,
    exit_code: partial.exit_code ?? 0,
    turns: partial.turns ?? 0,
    memory_window: partial.memory_window ?? 0,
    prompt_tokens: partial.prompt_tokens ?? 0,
    completion_tokens: partial.completion_tokens ?? 0,
    estimated_cost_usd: partial.estimated_cost_usd ?? 0,
    checks_passed: partial.checks_passed ?? 0,
    checks_total: partial.checks_total ?? 0,
    checks_json: partial.checks_json ?? "",
    metadata_json: partial.metadata_json ?? "",
    artifact_dir: partial.artifact_dir ?? "",
    created_at: partial.created_at ?? "2026-05-26T00:00:00Z",
    review_summary: partial.review_summary,
  };
}

test("buildReviewQueue classifies unreviewed, unsafe pass, and reviewed failure runs", () => {
  const queue = buildReviewQueue([
    run({ id: "needs-review", passed: true }),
    run({
      id: "unsafe-pass",
      passed: true,
      review_summary: {
        verdict: "unsafe_pass",
        primary_label: "unsafe_action",
        visibility: "public",
        label_count: 1,
        max_severity: "warning",
      },
    }),
    run({
      id: "reviewed-failure",
      passed: false,
      review_summary: {
        verdict: "valid_failure",
        visibility: "private",
        label_count: 2,
        max_severity: "critical",
      },
    }),
  ]);

  assert.deepEqual(queue.needsReview.map((item) => item.id), ["needs-review"]);
  assert.deepEqual(queue.unsafePasses.map((item) => item.id), ["unsafe-pass"]);
  assert.deepEqual(queue.reviewedFailures.map((item) => item.id), ["reviewed-failure"]);
});

test("reviewSummaryText formats compact review verdicts", () => {
  assert.equal(reviewSummaryText(undefined), "No review");
  assert.equal(
    reviewSummaryText({
      verdict: "unsafe_pass",
      primary_label: "unsafe_action",
      visibility: "public",
      label_count: 1,
      max_severity: "warning",
    }),
    "Unsafe pass / unsafe action",
  );
});
