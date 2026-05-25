export interface RunReview {
  version?: string;
  run_id?: string;
  scenario_id?: string;
  visibility?: string;
  verdict?: string;
  primary_label?: string;
  reviewer?: RunReviewReviewer;
  labels?: RunReviewLabel[];
  suggested_rules?: RunReviewSuggestedRule[];
}

export interface RunReviewReviewer {
  type?: string;
  display_name?: string;
}

export interface RunReviewLabel {
  kind?: string;
  severity?: string;
  step?: number;
  step_end?: number;
  evidence_snippet?: string;
  evidence_ref?: RunReviewEvidenceRef;
  note?: string;
}

export interface RunReviewEvidenceRef {
  artifact?: string;
  step?: number;
}

export interface RunReviewSuggestedRule {
  target?: string;
  kind?: string;
  pattern?: string;
  severity?: string;
  reason?: string;
}

export interface RunReviewView {
  verdictLabel: string;
  visibilityLabel: string;
  reviewerLabel: string;
  labels: RunReviewLabelView[];
  suggestedRules: string[];
}

export interface RunReviewLabelView {
  kindLabel: string;
  severityLabel: string;
  stepLabel: string;
  evidenceRefLabel: string;
  evidenceSnippet: string;
  note: string;
}

export function normalizeRunReviewView(review: RunReview): RunReviewView {
  return {
    verdictLabel: verdictLabel(review.verdict ?? "needs_review"),
    visibilityLabel: titleLabel(review.visibility ?? "private"),
    reviewerLabel: reviewerLabel(review.reviewer),
    labels: (review.labels ?? []).map((label) => ({
      kindLabel: titleLabel(label.kind ?? "label"),
      severityLabel: titleLabel(label.severity ?? "info"),
      stepLabel: stepRangeText(label.step, label.step_end),
      evidenceRefLabel: evidenceRefText(label.evidence_ref),
      evidenceSnippet: label.evidence_snippet ?? "",
      note: label.note ?? "",
    })),
    suggestedRules: (review.suggested_rules ?? []).map(ruleSummary),
  };
}

export function verdictLabel(value: string): string {
  return titleLabel(value);
}

export function evidenceRefText(ref: RunReviewEvidenceRef | undefined): string {
  if (!ref || !ref.artifact) return "";
  if (typeof ref.step === "number") {
    return `${ref.artifact} step ${ref.step + 1}`;
  }
  return ref.artifact;
}

export function ruleSummary(rule: RunReviewSuggestedRule): string {
  const target = rule.target ?? "rule";
  const kind = rule.kind ?? "pattern";
  const pattern = rule.pattern ?? "";
  const severity = rule.severity ? ` (${rule.severity})` : "";
  const reason = rule.reason ? ` - ${rule.reason}` : "";
  return `${target}: ${kind}${pattern ? ` ${pattern}` : ""}${severity}${reason}`;
}

function reviewerLabel(reviewer: RunReviewReviewer | undefined): string {
  if (!reviewer) return "Human review";
  return reviewer.display_name || reviewer.type || "Human review";
}

function stepRangeText(step: number | undefined, stepEnd: number | undefined): string {
  if (typeof step !== "number") return "";
  if (typeof stepEnd === "number" && stepEnd !== step) {
    return `steps ${step + 1}-${stepEnd + 1}`;
  }
  return `step ${step + 1}`;
}

function titleLabel(value: string): string {
  const normalized = value.replace(/[_-]+/g, " ").trim();
  if (normalized === "") return "";
  return normalized.charAt(0).toUpperCase() + normalized.slice(1);
}
