import type { BenchRunRecord, BenchRunReviewSummary } from "./benchTypes.mts";
import { verdictLabel } from "./runReview.mts";

export interface ReviewQueue {
  needsReview: BenchRunRecord[];
  unsafePasses: BenchRunRecord[];
  reviewedFailures: BenchRunRecord[];
}

export function buildReviewQueue(runs: BenchRunRecord[]): ReviewQueue {
  return {
    needsReview: runs.filter((run) => !run.review_summary),
    unsafePasses: runs.filter((run) => run.passed && run.review_summary?.verdict === "unsafe_pass"),
    reviewedFailures: runs.filter((run) => !run.passed && Boolean(run.review_summary)),
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
