import { usePageTitle } from "../../hooks/usePageTitle";
import { useCallback, useEffect, useState } from "react";
import { useParams, Link, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import type { AutopsyReport } from "../../lib/autopsyView.mts";
import { formatCompactTokens, formatDuration } from "../../lib/benchFormatters.mts";
import type { BenchRunRecord } from "../../lib/benchTypes.mts";
import type { RunReview } from "../../lib/runReview.mts";
import { reviewDraftAvailable } from "../../lib/runReviewEditor.mts";
import type { ScenarioPatchPreview, ScenarioPatchValidation } from "../../lib/scenarioPatchPreview.mts";
import { scenarioPatchValidationApiPath } from "../../lib/scenarioPatchPreview.mts";
import {
  AutopsyTab,
  MetaItem,
  ReviewTab,
  ScorecardTab,
  SummaryTab,
  TimelineTab,
  ToolCallsTab,
  TranscriptTab,
  parseChecks,
} from "./runDetail/tabs";
import { useRunArtifacts } from "./runDetail/useRunArtifacts";
import { useRunReview } from "./runDetail/useRunReview";
import { useScenarioPatchWorkflow } from "./runDetail/useScenarioPatchWorkflow";
import type { Scorecard, TimelineData, ToolCall } from "./runDetail/types";

type Tab = "summary" | "review" | "autopsy" | "timeline" | "transcript" | "tool-calls" | "scorecard";

const TABS: { key: Tab; label: string }[] = [
  { key: "summary", label: "Summary" },
  { key: "review", label: "Review" },
  { key: "autopsy", label: "Autopsy" },
  { key: "timeline", label: "Timeline" },
  { key: "transcript", label: "Transcript" },
  { key: "tool-calls", label: "Tool Calls" },
  { key: "scorecard", label: "Scorecard" },
];

function parseTab(value: string | null): Tab {
  if (TABS.some((tab) => tab.key === value)) return value as Tab;
  return "summary";
}

async function responseMessage(res: Response, fallback: string): Promise<string> {
  try {
    const body = await res.clone().json();
    if (body && typeof body === "object") {
      const errorBody = body as { error?: unknown; message?: unknown };
      const message = errorBody.error ?? errorBody.message;
      if (typeof message === "string" && message.trim() !== "") return message;
    }
  } catch {}
  return res.statusText || fallback;
}

export function RunDetail() {
  const { id } = useParams<{ id: string }>();
  const [searchParams, setSearchParams] = useSearchParams();
  const { request, fetchResponse } = useApi();
  usePageTitle(id ? `Run ${id}` : "Run Detail");

  const [run, setRun] = useState<BenchRunRecord | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>(() => parseTab(searchParams.get("tab")));

  const {
    transcript,
    setTranscript,
    transcriptLoading,
    setTranscriptLoading,
    transcriptError,
    setTranscriptError,
    toolCalls,
    setToolCalls,
    toolCallsLoading,
    setToolCallsLoading,
    toolCallsError,
    setToolCallsError,
    scorecard,
    setScorecard,
    scorecardLoading,
    setScorecardLoading,
    scorecardError,
    setScorecardError,
    timeline,
    setTimeline,
    timelineLoading,
    setTimelineLoading,
    timelineError,
    setTimelineError,
    autopsy,
    setAutopsy,
    autopsyLoading,
    setAutopsyLoading,
    autopsyError,
    setAutopsyError,
  } = useRunArtifacts();
  const {
    review,
    setReview,
    reviewLoading,
    setReviewLoading,
    reviewError,
    setReviewError,
    reviewSaving,
    setReviewSaving,
    reviewSaveError,
    setReviewSaveError,
    reviewSaved,
    setReviewSaved,
    reviewDrafting,
    setReviewDrafting,
    reviewDraftError,
    setReviewDraftError,
    reviewDraftSeed,
    setReviewDraftSeed,
    reviewAutoDraftedRunID,
    setReviewAutoDraftedRunID,
  } = useRunReview();
  const {
    scenarioPatchPreview,
    setScenarioPatchPreview,
    scenarioPatchPreviewLoading,
    setScenarioPatchPreviewLoading,
    scenarioPatchPreviewError,
    setScenarioPatchPreviewError,
    scenarioPatchValidation,
    setScenarioPatchValidation,
    scenarioPatchValidationLoading,
    setScenarioPatchValidationLoading,
    scenarioPatchValidationError,
    setScenarioPatchValidationError,
    scenarioPatchValidationLoaded,
    setScenarioPatchValidationLoaded,
  } = useScenarioPatchWorkflow();

  useEffect(() => {
    setActiveTab(parseTab(searchParams.get("tab")));
  }, [searchParams]);

  useEffect(() => {
    setScenarioPatchPreview(null);
    setScenarioPatchPreviewError(null);
    setScenarioPatchValidation(null);
    setScenarioPatchValidationLoading(false);
    setScenarioPatchValidationError(null);
    setScenarioPatchValidationLoaded(false);
    setReviewDraftSeed(null);
    setReviewAutoDraftedRunID(null);
  }, [id]);

  function selectTab(tab: Tab) {
    setActiveTab(tab);
    const next = new URLSearchParams(searchParams);
    if (tab === "summary") {
      next.delete("tab");
    } else {
      next.set("tab", tab);
    }
    setSearchParams(next, { replace: true });
  }

  async function saveReview(payload: RunReview) {
    if (!id) return;
    setReviewSaving(true);
    setReviewSaveError(null);
    setReviewDraftError(null);
    setScenarioPatchPreview(null);
    setScenarioPatchPreviewError(null);
    setScenarioPatchValidation(null);
    setScenarioPatchValidationLoading(false);
    setScenarioPatchValidationError(null);
    setScenarioPatchValidationLoaded(false);
    setReviewSaved(false);
    try {
      const saved = await request<RunReview>(`/v1/bench/runs/${id}/review`, {
        method: "PUT",
        body: JSON.stringify(payload),
      });
      setReview(saved);
      setReviewError(null);
      setReviewSaved(true);
      try {
        const refreshedRun = await request<BenchRunRecord>(`/v1/bench/runs/${id}`);
        setRun(refreshedRun);
      } catch (refreshErr) {
        setReviewSaveError(refreshErr instanceof Error ? `Saved review, but failed to refresh run summary: ${refreshErr.message}` : "Saved review, but failed to refresh run summary");
      }
    } catch (err) {
      setReviewSaveError(err instanceof Error ? err.message : "Failed to save human review");
      throw err;
    } finally {
      setReviewSaving(false);
    }
  }

  async function draftReview(): Promise<RunReview> {
    if (!id) throw new Error("Run id is required");
    setReviewDrafting(true);
    setReviewDraftError(null);
    setReviewSaveError(null);
    setScenarioPatchPreview(null);
    setScenarioPatchPreviewError(null);
    setScenarioPatchValidation(null);
    setScenarioPatchValidationLoading(false);
    setScenarioPatchValidationError(null);
    setScenarioPatchValidationLoaded(false);
    setReviewSaved(false);
    try {
      const draft = await request<RunReview>(`/v1/bench/review-candidates/${id}/draft`, {
        method: "POST",
      });
      setReviewDraftSeed(draft);
      return draft;
    } catch (err) {
      setReviewDraftError(err instanceof Error ? err.message : "Failed to draft review");
      throw err;
    } finally {
      setReviewDrafting(false);
    }
  }

  async function previewScenarioPatch() {
    if (!id) return;
    setScenarioPatchPreviewLoading(true);
    setScenarioPatchPreviewError(null);
    setScenarioPatchValidation(null);
    setScenarioPatchValidationLoading(false);
    setScenarioPatchValidationError(null);
    setScenarioPatchValidationLoaded(false);
    try {
      const preview = await request<ScenarioPatchPreview>(`/v1/bench/runs/${id}/scenario-patch-preview`, {
        method: "POST",
      });
      setScenarioPatchPreview(preview);
    } catch (err) {
      setScenarioPatchPreviewError(err instanceof Error ? err.message : "Failed to preview scenario patch");
    } finally {
      setScenarioPatchPreviewLoading(false);
    }
  }

  const loadScenarioPatchValidation = useCallback(async (options: { silentNotFound?: boolean } = {}) => {
    if (!id) return;
    setScenarioPatchValidationLoading(true);
    setScenarioPatchValidationError(null);
    try {
      const res = await fetchResponse(scenarioPatchValidationApiPath(id));
      if (res.status === 404 && options.silentNotFound) {
        setScenarioPatchValidation(null);
        setScenarioPatchValidationLoaded(true);
        return;
      }
      if (!res.ok) {
        throw new Error(await responseMessage(res, "Failed to load validation status"));
      }
      const validation = (await res.json()) as ScenarioPatchValidation;
      setScenarioPatchValidation(validation);
      setScenarioPatchValidationLoaded(true);
    } catch (err) {
      setScenarioPatchValidationError(err instanceof Error ? err.message : "Failed to load validation status");
      setScenarioPatchValidationLoaded(true);
    } finally {
      setScenarioPatchValidationLoading(false);
    }
  }, [fetchResponse, id]);

  async function validateScenarioPatch() {
    if (!id) return;
    setScenarioPatchValidationLoading(true);
    setScenarioPatchValidationError(null);
    try {
      const validation = await request<ScenarioPatchValidation>(scenarioPatchValidationApiPath(id), {
        method: "POST",
      });
      setScenarioPatchValidation(validation);
      setScenarioPatchValidationLoaded(true);
    } catch (err) {
      setScenarioPatchValidationError(err instanceof Error ? err.message : "Failed to queue validation rerun");
    } finally {
      setScenarioPatchValidationLoading(false);
    }
  }

  useEffect(() => {
    if (activeTab !== "review" || !id || scenarioPatchValidationLoaded || scenarioPatchValidationLoading) return;
    void loadScenarioPatchValidation({ silentNotFound: true });
  }, [activeTab, id, loadScenarioPatchValidation, scenarioPatchValidationLoaded, scenarioPatchValidationLoading]);

  // Fetch run record
  useEffect(() => {
    if (!id) return;
    setLoading(true);
    request<BenchRunRecord>(`/v1/bench/runs/${id}`)
      .then(setRun)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id, request]);

  // Fetch transcript on tab switch
  useEffect(() => {
    if (activeTab !== "transcript" || transcript !== null || transcriptLoading || !id) return;
    setTranscriptLoading(true);
    fetchResponse(`/v1/bench/runs/${id}/transcript`)
      .then((res) => {
        if (!res.ok) throw new Error(res.statusText);
        return res.text();
      })
      .then(setTranscript)
      .catch((err) => setTranscriptError(err.message))
      .finally(() => setTranscriptLoading(false));
  }, [activeTab, transcript, transcriptLoading, id, fetchResponse]);

  // Fetch tool calls on tab switch
  useEffect(() => {
    if (activeTab !== "tool-calls" || toolCalls !== null || toolCallsLoading || !id) return;
    setToolCallsLoading(true);
    fetchResponse(`/v1/bench/runs/${id}/tool-calls`)
      .then((res) => {
        if (!res.ok) {
          if (res.status === 404) return null;
          throw new Error(res.statusText);
        }
        return res.text();
      })
      .then((text) => {
        if (text === null) {
          setToolCalls([]);
          return;
        }
        try {
          setToolCalls(JSON.parse(text) as ToolCall[]);
        } catch {
          setToolCalls([]);
          setToolCallsError("Failed to parse tool calls");
        }
      })
      .catch((err) => setToolCallsError(err.message))
      .finally(() => setToolCallsLoading(false));
  }, [activeTab, toolCalls, toolCallsLoading, id, fetchResponse]);

  // Fetch scorecard on tab switch
  useEffect(() => {
    if (activeTab !== "scorecard" || scorecard !== null || scorecardError !== null || scorecardLoading || !id) return;
    setScorecardLoading(true);
    fetchResponse(`/v1/bench/runs/${id}/scorecard`)
      .then((res) => {
        if (res.status === 404) {
          setScorecardError("not-found");
          return;
        }
        if (!res.ok) throw new Error(res.statusText);
        return res.json();
      })
      .then((data) => {
        if (data) setScorecard(data as Scorecard);
      })
      .catch((err) => setScorecardError(err.message))
      .finally(() => setScorecardLoading(false));
  }, [activeTab, scorecard, scorecardError, scorecardLoading, id, fetchResponse]);

  // Fetch timeline on tab switch. The Review editor also uses timeline steps
  // for evidence prefill.
  useEffect(() => {
    if ((activeTab !== "timeline" && activeTab !== "review") || timeline !== null || timelineError !== null || timelineLoading || !id) return;
    setTimelineLoading(true);
    fetchResponse(`/v1/bench/runs/${id}/timeline`)
      .then((res) => {
        if (res.status === 404) {
          setTimelineError("not-found");
          return;
        }
        if (!res.ok) throw new Error(res.statusText);
        return res.json();
      })
      .then((data) => {
        if (data) setTimeline(data as TimelineData);
      })
      .catch((err) => setTimelineError(err.message))
      .finally(() => setTimelineLoading(false));
  }, [activeTab, timeline, timelineError, timelineLoading, id, fetchResponse]);

  // Fetch failure autopsy on tab switch
  useEffect(() => {
    if (activeTab !== "autopsy" || autopsy !== null || autopsyError !== null || autopsyLoading || !id) return;
    setAutopsyLoading(true);
    fetchResponse(`/v1/bench/runs/${id}/autopsy`)
      .then((res) => {
        if (res.status === 404) {
          setAutopsyError("not-found");
          return;
        }
        if (!res.ok) throw new Error(res.statusText);
        return res.json();
      })
      .then((data) => {
        if (data) setAutopsy(data as AutopsyReport);
      })
      .catch((err) => setAutopsyError(err.message))
      .finally(() => setAutopsyLoading(false));
  }, [activeTab, autopsy, autopsyError, autopsyLoading, id, fetchResponse]);

  // Fetch human review on tab switch
  useEffect(() => {
    if (activeTab !== "review" || review !== null || reviewError !== null || reviewLoading || !id) return;
    setReviewLoading(true);
    fetchResponse(`/v1/bench/runs/${id}/review`)
      .then((res) => {
        if (res.status === 404) {
          setReviewError("not-found");
          return;
        }
        if (!res.ok) throw new Error(res.statusText);
        return res.json();
      })
      .then((data) => {
        if (data) setReview(data as RunReview);
      })
      .catch((err) => setReviewError(err.message))
      .finally(() => setReviewLoading(false));
  }, [activeTab, review, reviewError, reviewLoading, id, fetchResponse]);

  useEffect(() => {
    if (activeTab !== "review" || searchParams.get("draft") !== "1" || !id) return;
    if (!run || !reviewDraftAvailable(run)) return;
    if (review || reviewLoading || reviewDrafting || reviewDraftSeed || reviewAutoDraftedRunID === id) return;
    if (reviewError !== "not-found") return;
    setReviewAutoDraftedRunID(id);
    void draftReview();
  }, [activeTab, searchParams, id, run, review, reviewLoading, reviewDrafting, reviewDraftSeed, reviewAutoDraftedRunID, reviewError]);

  if (loading) {
    return (
      <div className="flex items-center justify-center py-20 text-fg-muted text-[0.85rem]">
        Loading run...
      </div>
    );
  }

  if (error || !run) {
    return (
      <div className="py-12 text-center">
        <p className="text-danger text-[0.9rem] mb-4">{error || "Run not found"}</p>
        <Link to="/bench/runs" className="text-accent text-[0.82rem] hover:underline">
          &larr; Back to Runs
        </Link>
      </div>
    );
  }

  const checks = parseChecks(run.checks_json);

  return (
    <div className="space-y-6">
      {/* Breadcrumb header */}
      <div className="flex items-center gap-3 flex-wrap">
        <Link
          to="/bench/runs"
          className="text-accent text-[0.82rem] font-medium hover:underline"
        >
          &larr; Runs
        </Link>
        <span className="text-fg-muted text-[0.75rem]">/</span>
        <span className="text-fg font-semibold text-[0.95rem]">
          {run.scenario_id}
        </span>
        <span
          className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide ${
            run.passed
              ? "bg-accent-tint text-accent"
              : "bg-[var(--color-danger-badge-bg)] text-[var(--color-danger-badge-fg)]"
          }`}
        >
          {run.passed ? "Pass" : "Fail"}
        </span>
        <span className="font-mono text-fg-muted text-[0.75rem] ml-auto">
          {run.id}
        </span>
      </div>

      {/* Meta grid */}
      <div className="grid gap-2.5" style={{ gridTemplateColumns: "repeat(auto-fill, minmax(140px, 1fr))" }}>
        <MetaItem label="Model" value={run.model} />
        <MetaItem label="Provider" value={run.provider} />
        <MetaItem label="Duration" value={formatDuration(run.duration_seconds)} />
        <MetaItem label="Turns" value={String(run.turns)} />
        <MetaItem
          label="Tokens"
          value={`${formatCompactTokens(run.prompt_tokens)} / ${formatCompactTokens(run.completion_tokens)}`}
        />
        <MetaItem
          label="Cost"
          value={`$${run.estimated_cost_usd.toFixed(4)}`}
        />
        <MetaItem label="Exit Code" value={String(run.exit_code)} />
        <MetaItem
          label="Checks"
          value={`${run.checks_passed}/${run.checks_total}`}
        />
      </div>

      {/* Tabs */}
      <div className="border-b border-border-subtle flex gap-0 overflow-x-auto">
        {TABS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => selectTab(key)}
            className={`text-[0.82rem] font-medium px-4 py-2 border-b-2 transition-colors cursor-pointer whitespace-nowrap flex-shrink-0 ${
              activeTab === key
                ? "text-accent border-accent font-semibold"
                : "text-fg-muted border-transparent hover:text-fg"
            }`}
          >
            {label}
          </button>
        ))}
      </div>

      {/* Tab content */}
      {activeTab === "summary" && <SummaryTab checks={checks} scorecard={scorecard} />}
      {activeTab === "review" && (
        <ReviewTab
          run={run}
          review={review}
          timeline={timeline}
          loading={reviewLoading}
          error={reviewError}
          saving={reviewSaving}
          saveError={reviewSaveError}
          saved={reviewSaved}
          drafting={reviewDrafting}
          draftError={reviewDraftError}
          draftSeed={reviewDraftSeed}
          scenarioPatchPreview={scenarioPatchPreview}
          scenarioPatchPreviewLoading={scenarioPatchPreviewLoading}
          scenarioPatchPreviewError={scenarioPatchPreviewError}
          scenarioPatchValidation={scenarioPatchValidation}
          scenarioPatchValidationLoading={scenarioPatchValidationLoading}
          scenarioPatchValidationError={scenarioPatchValidationError}
          onDraft={draftReview}
          onPreviewScenarioPatch={previewScenarioPatch}
          onValidateScenarioPatch={validateScenarioPatch}
          onRefreshScenarioPatchValidation={loadScenarioPatchValidation}
          onSave={saveReview}
        />
      )}
      {activeTab === "autopsy" && (
        <AutopsyTab
          autopsy={autopsy}
          loading={autopsyLoading}
          error={autopsyError}
        />
      )}
      {activeTab === "timeline" && (
        <TimelineTab
          timeline={timeline}
          loading={timelineLoading}
          error={timelineError}
        />
      )}
      {activeTab === "transcript" && (
        <TranscriptTab
          transcript={transcript}
          loading={transcriptLoading}
          error={transcriptError}
        />
      )}
      {activeTab === "tool-calls" && (
        <ToolCallsTab
          toolCalls={toolCalls}
          loading={toolCallsLoading}
          error={toolCallsError}
        />
      )}
      {activeTab === "scorecard" && (
        <ScorecardTab
          scorecard={scorecard}
          loading={scorecardLoading}
          error={scorecardError}
        />
      )}
    </div>
  );
}
