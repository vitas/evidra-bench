import type { BenchRunRecord, BenchRunReviewSummary } from "./benchTypes.mts";
import { verdictLabel } from "./runReview.mts";

export interface ReviewQueue {
  needsReview: BenchRunRecord[];
  unsafePasses: BenchRunRecord[];
  reviewedFailures: BenchRunRecord[];
  scenarioImprovements: BenchRunRecord[];
}

export type ReviewQueueKey = keyof ReviewQueue;

export function reviewQueueApiPath(queue: ReviewQueueKey, limit: number): string {
  const params = new URLSearchParams();
  params.set("limit", String(limit));
  switch (queue) {
    case "needsReview":
      params.set("review", "unreviewed");
      break;
    case "unsafePasses":
      params.set("passed", "true");
      params.set("review_verdict", "unsafe_pass");
      break;
    case "reviewedFailures":
      params.set("passed", "false");
      params.set("review", "reviewed");
      break;
    case "scenarioImprovements":
      params.set("review", "reviewed");
      params.set("has_suggested_rules", "true");
      break;
  }
  return `/v1/bench/runs?${params.toString()}`;
}

export function buildReviewQueue(runs: BenchRunRecord[]): ReviewQueue {
  return {
    needsReview: runs.filter((run) => !run.review_summary),
    unsafePasses: runs.filter((run) => run.passed && run.review_summary?.verdict === "unsafe_pass"),
    reviewedFailures: runs.filter((run) => !run.passed && Boolean(run.review_summary)),
    scenarioImprovements: runs.filter((run) => (run.review_summary?.suggested_rule_count ?? 0) > 0),
  };
}

export function reviewSummaryText(summary: BenchRunReviewSummary | undefined): string {
  if (!summary) return "No review";
  const verdict = verdictLabel(summary.verdict);
  const label = compactLabel(summary.primary_label ?? "");
  return label ? `${verdict} / ${label}` : verdict;
}

export function reviewSeverityTone(summary: BenchRunReviewSummary | undefined): "none" | "info" | "warning" | "error" | "critical" {
  if (!summary) return "none";
  switch (summary.max_severity) {
    case "critical":
      return "critical";
    case "error":
      return "error";
    case "warning":
      return "warning";
    default:
      return "info";
  }
}

function compactLabel(value: string): string {
  return value.split("_").filter(Boolean).join(" ");
}
