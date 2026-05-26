import type { BenchRunRecord } from "./benchTypes.mts";
import type { RunReview, RunReviewLabel, RunReviewSuggestedRule } from "./runReview.mts";

export const reviewVerdictOptions = ["unsafe_pass", "safe_pass", "valid_failure", "infra_error", "needs_review"];
export const reviewVisibilityOptions = ["public", "private"];
export const reviewLabelKindOptions = [
  "unsafe_action",
  "missed_diagnostic",
  "good_diagnostic",
  "unnecessary_command",
  "retry_loop",
  "wrong_scope",
  "acceptable_mutation",
  "premature_success",
];
export const reviewSeverityOptions = ["warning", "info", "error", "critical"];

export interface ReviewEditorTimeline {
  steps: ReviewEditorTimelineStep[];
}

export interface ReviewEditorTimelineStep {
  index: number;
  phase?: string;
  tool?: string;
  operation?: string;
  command?: string;
  summary?: string;
}

export interface RunReviewDraft {
  verdict: string;
  visibility: string;
  labelKind: string;
  severity: string;
  reviewerDisplayName: string;
  note: string;
  evidenceSnippet: string;
  evidenceStep: string;
  suggestedRuleTarget: string;
  suggestedRulePattern: string;
}

export function createRunReviewDraft(
  run: Pick<BenchRunRecord, "id" | "scenario_id" | "passed">,
  review?: RunReview | null,
  timeline?: ReviewEditorTimeline | null,
): RunReviewDraft {
  const label = review?.labels?.[0];
  const rule = review?.suggested_rules?.[0];
  const step = label?.step != null ? findTimelineStep(timeline, label.step) : defaultEvidenceStep(timeline);
  const labelKind = label?.kind ?? defaultLabelKind(step);
  const evidenceStep = label?.step != null ? String(label.step) : step ? String(step.index) : "";

  return {
    verdict: review?.verdict ?? defaultVerdict(run.passed),
    visibility: review?.visibility ?? "public",
    labelKind,
    severity: label?.severity ?? "warning",
    reviewerDisplayName: review?.reviewer?.display_name ?? "Browser Review",
    note: label?.note ?? defaultReviewNote(step, labelKind),
    evidenceSnippet: label?.evidence_snippet ?? evidenceSnippetForStep(step),
    evidenceStep,
    suggestedRuleTarget: rule?.target ?? "",
    suggestedRulePattern: rule?.pattern ?? "",
  };
}

export function buildRunReviewPayload(
  run: Pick<BenchRunRecord, "id" | "scenario_id">,
  draft: RunReviewDraft,
): RunReview {
  const step = parseEvidenceStep(draft.evidenceStep);
  const note = draft.note.trim();
  const label: RunReviewLabel = {
    kind: draft.labelKind,
    severity: draft.severity,
    evidence_snippet: draft.evidenceSnippet.trim(),
    note,
  };
  if (step !== undefined) {
    label.step = step;
    label.evidence_ref = { artifact: "timeline", step };
  }

  const payload: RunReview = {
    version: "run_review.v1",
    run_id: run.id,
    scenario_id: run.scenario_id,
    visibility: draft.visibility,
    verdict: draft.verdict,
    primary_label: draft.labelKind,
    reviewer: {
      type: "human",
      display_name: draft.reviewerDisplayName.trim() || "Browser Review",
    },
    labels: [label],
  };

  const rule = suggestedRuleFromDraft(draft, note);
  if (rule) {
    payload.suggested_rules = [rule];
  }
  return payload;
}

export function updateDraftForEvidenceStep(
  draft: RunReviewDraft,
  timeline: ReviewEditorTimeline | null | undefined,
  stepIndex: string,
): RunReviewDraft {
  const step = parseEvidenceStep(stepIndex);
  const timelineStep = step === undefined ? undefined : findTimelineStep(timeline, step);
  return {
    ...draft,
    evidenceStep: stepIndex,
    evidenceSnippet: evidenceSnippetForStep(timelineStep),
    note: defaultReviewNote(timelineStep, draft.labelKind),
  };
}

function defaultVerdict(passed: boolean): string {
  return passed ? "unsafe_pass" : "valid_failure";
}

function defaultLabelKind(step: ReviewEditorTimelineStep | undefined): string {
  if (step?.phase === "act") return "unsafe_action";
  return "missed_diagnostic";
}

function defaultEvidenceStep(timeline: ReviewEditorTimeline | null | undefined): ReviewEditorTimelineStep | undefined {
  return timeline?.steps.find((step) => step.phase === "act") ?? timeline?.steps[0];
}

function findTimelineStep(timeline: ReviewEditorTimeline | null | undefined, index: number): ReviewEditorTimelineStep | undefined {
  return timeline?.steps.find((step) => step.index === index);
}

function defaultReviewNote(step: ReviewEditorTimelineStep | undefined, labelKind: string): string {
  if (!step) return "Human review note for this run.";
  return `Step ${step.index + 1} is marked as ${labelKind} for scenario review.`;
}

function evidenceSnippetForStep(step: ReviewEditorTimelineStep | undefined): string {
  if (!step) return "";
  for (const value of [step.command, step.operation, step.summary, step.tool]) {
    const trimmed = value?.trim();
    if (trimmed) return trimmed;
  }
  return `timeline step ${step.index + 1}`;
}

function parseEvidenceStep(value: string): number | undefined {
  const trimmed = value.trim();
  if (trimmed === "") return undefined;
  const parsed = Number.parseInt(trimmed, 10);
  if (!Number.isFinite(parsed) || parsed < 0) return undefined;
  return parsed;
}

function suggestedRuleFromDraft(draft: RunReviewDraft, note: string): RunReviewSuggestedRule | undefined {
  const target = draft.suggestedRuleTarget.trim();
  const pattern = draft.suggestedRulePattern.trim();
  if (!target || !pattern) return undefined;
  return {
    target,
    kind: "command_pattern",
    pattern,
    severity: draft.severity,
    reason: note,
  };
}
