import { useEffect, useState, type FormEvent } from "react";
import type { BenchRunRecord } from "../../lib/benchTypes.mts";
import type { RunReview } from "../../lib/runReview.mts";
import { verdictLabel } from "../../lib/runReview.mts";
import {
  buildRunReviewPayload,
  createRunReviewDraft,
  reviewLabelKindOptions,
  reviewSeverityOptions,
  reviewVerdictOptions,
  reviewVisibilityOptions,
  type ReviewEditorTimeline,
  type RunReviewDraft,
  updateDraftForEvidenceStep,
} from "../../lib/runReviewEditor.mts";

interface RunReviewEditorProps {
  run: BenchRunRecord;
  review: RunReview | null;
  timeline: ReviewEditorTimeline | null;
  saving: boolean;
  saveError: string | null;
  saved: boolean;
  drafting?: boolean;
  draftError?: string | null;
  draftSeed?: RunReview | null;
  onDraft?: () => Promise<RunReview>;
  onSave: (payload: RunReview) => Promise<void>;
}

const inputClass = "w-full rounded-md border border-border bg-bg-elevated px-3 py-2 text-[0.82rem] text-fg focus:outline-none focus:border-accent";
const labelClass = "block text-[0.68rem] font-semibold uppercase tracking-wide text-fg-muted mb-1.5";

export function RunReviewEditor({
  run,
  review,
  timeline,
  saving,
  saveError,
  saved,
  drafting = false,
  draftError = null,
  draftSeed = null,
  onDraft,
  onSave,
}: RunReviewEditorProps) {
  const [draft, setDraft] = useState<RunReviewDraft>(() => createRunReviewDraft(run, review, timeline));
  const [dirty, setDirty] = useState(false);
  const timelineSteps = timeline?.steps ?? [];
  const hasSelectedTimelineStep = draft.evidenceStep === "" || timelineSteps.some((step) => String(step.index) === draft.evidenceStep);

  useEffect(() => {
    if (!dirty) {
      setDraft(createRunReviewDraft(run, review, timeline));
    }
  }, [dirty, run, review, timeline]);

  useEffect(() => {
    if (draftSeed && !dirty) {
      setDraft(createRunReviewDraft(run, draftSeed, timeline));
      setDirty(true);
    }
  }, [draftSeed, dirty, run, timeline]);

  function patchDraft(patch: Partial<RunReviewDraft>) {
    setDirty(true);
    setDraft((current) => ({ ...current, ...patch }));
  }

  function selectEvidenceStep(value: string) {
    setDirty(true);
    setDraft((current) => updateDraftForEvidenceStep(current, timeline, value));
  }

  async function submit(e: FormEvent) {
    e.preventDefault();
    try {
      await onSave(buildRunReviewPayload(run, draft));
      setDirty(false);
    } catch {
      // The parent surfaces the backend error in saveError.
    }
  }

  async function applyGeneratedDraft() {
    if (!onDraft) return;
    try {
      const generated = await onDraft();
      setDraft(createRunReviewDraft(run, generated, timeline));
      setDirty(true);
    } catch {
      // The parent surfaces the backend error in draftError.
    }
  }

  return (
    <form onSubmit={submit} className="rounded-lg border border-border bg-bg-elevated p-4">
      <div className="mb-4 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
        <h3 className="text-[0.95rem] font-semibold text-fg">Review Editor</h3>
        <div className="flex items-center gap-2">
          {saved && !dirty && <span className="text-[0.76rem] text-accent">Saved</span>}
          {onDraft && (
            <button
              type="button"
              disabled={drafting || saving}
              onClick={applyGeneratedDraft}
              className="rounded-md border border-border bg-bg-alt px-3 py-1.5 text-[0.78rem] font-semibold text-fg transition-colors hover:border-accent disabled:cursor-default disabled:opacity-50"
            >
              {drafting ? "Drafting..." : "Draft with AI"}
            </button>
          )}
          <button
            type="submit"
            disabled={saving}
            className="rounded-md border border-accent bg-accent px-3 py-1.5 text-[0.78rem] font-semibold text-white transition-opacity disabled:cursor-default disabled:opacity-50"
          >
            {saving ? "Saving..." : "Save Review"}
          </button>
        </div>
      </div>

      {saveError && (
        <div className="mb-4 rounded-md bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.8rem] text-[var(--color-danger-badge-fg)]">
          {saveError}
        </div>
      )}
      {draftError && (
        <div className="mb-4 rounded-md bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.8rem] text-[var(--color-danger-badge-fg)]">
          {draftError}
        </div>
      )}

      <div className="grid gap-3 md:grid-cols-4">
        <SelectField label="Verdict" value={draft.verdict} values={reviewVerdictOptions} onChange={(value) => patchDraft({ verdict: value })} />
        <SelectField label="Visibility" value={draft.visibility} values={reviewVisibilityOptions} onChange={(value) => patchDraft({ visibility: value })} />
        <SelectField label="Label" value={draft.labelKind} values={reviewLabelKindOptions} onChange={(value) => patchDraft({ labelKind: value })} />
        <SelectField label="Severity" value={draft.severity} values={reviewSeverityOptions} onChange={(value) => patchDraft({ severity: value })} />
      </div>

      <div className="mt-3 grid gap-3 md:grid-cols-[minmax(0,1fr)_minmax(0,2fr)]">
        <label>
          <span className={labelClass}>Reviewer</span>
          <input
            value={draft.reviewerDisplayName}
            onChange={(e) => patchDraft({ reviewerDisplayName: e.target.value })}
            required
            className={inputClass}
          />
        </label>
        <label>
          <span className={labelClass}>Evidence Step</span>
          <select
            value={draft.evidenceStep}
            onChange={(e) => selectEvidenceStep(e.target.value)}
            className={inputClass}
          >
            <option value="">Manual evidence</option>
            {!hasSelectedTimelineStep && (
              <option value={draft.evidenceStep}>
                Step {Number.parseInt(draft.evidenceStep, 10) + 1}
              </option>
            )}
            {timelineSteps.map((step) => (
              <option key={step.index} value={String(step.index)}>
                {step.index + 1}. {step.phase || "step"} {step.command || step.operation || step.summary || step.tool || ""}
              </option>
            ))}
          </select>
        </label>
      </div>

      <div className="mt-3 grid gap-3 md:grid-cols-2">
        <label>
          <span className={labelClass}>Reviewer Note</span>
          <textarea
            value={draft.note}
            onChange={(e) => patchDraft({ note: e.target.value })}
            rows={4}
            required
            className={inputClass + " resize-y"}
          />
        </label>
        <label>
          <span className={labelClass}>Evidence Snippet</span>
          <textarea
            value={draft.evidenceSnippet}
            onChange={(e) => patchDraft({ evidenceSnippet: e.target.value })}
            rows={4}
            required
            className={inputClass + " resize-y font-mono"}
          />
        </label>
      </div>

      <div className="mt-3 grid gap-3 md:grid-cols-2">
        <label>
          <span className={labelClass}>Suggested Rule Target</span>
          <input
            value={draft.suggestedRuleTarget}
            onChange={(e) => patchDraft({ suggestedRuleTarget: e.target.value })}
            placeholder="autopsy.forbidden_actions"
            className={inputClass}
          />
        </label>
        <label>
          <span className={labelClass}>Suggested Rule Pattern</span>
          <input
            value={draft.suggestedRulePattern}
            onChange={(e) => patchDraft({ suggestedRulePattern: e.target.value })}
            placeholder="Pod/*"
            className={inputClass}
          />
        </label>
      </div>
    </form>
  );
}

function SelectField({
  label,
  value,
  values,
  onChange,
}: {
  label: string;
  value: string;
  values: string[];
  onChange: (value: string) => void;
}) {
  return (
    <label>
      <span className={labelClass}>{label}</span>
      <select value={value} onChange={(e) => onChange(e.target.value)} className={inputClass}>
        {values.map((item) => (
          <option key={item} value={item}>
            {verdictLabel(item)}
          </option>
        ))}
      </select>
    </label>
  );
}
