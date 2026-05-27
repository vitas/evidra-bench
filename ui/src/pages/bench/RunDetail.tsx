import { usePageTitle } from "../../hooks/usePageTitle";
import { useCallback, useEffect, useState } from "react";
import { useParams, Link, useSearchParams } from "react-router";
import { useBenchApi as useApi } from "../../hooks/useBenchApi";
import type { AutopsyReport } from "../../lib/autopsyView.mts";
import { normalizeAutopsyReport } from "../../lib/autopsyView.mts";
import { formatCompactTokens, formatDuration } from "../../lib/benchFormatters.mts";
import type { BenchRunRecord } from "../../lib/benchTypes.mts";
import type { RunReview } from "../../lib/runReview.mts";
import { normalizeRunReviewView } from "../../lib/runReview.mts";
import { reviewDraftAvailable } from "../../lib/runReviewEditor.mts";
import type { ScenarioPatchPreview, ScenarioPatchValidation } from "../../lib/scenarioPatchPreview.mts";
import {
  scenarioPatchValidationApiPath,
  scenarioPatchValidationProgress,
  scenarioPatchValidationRunIDs,
  scenarioPatchValidationStatus,
  scenarioPatchPreviewDiffFilename,
  scenarioPatchPreviewDownloadHref,
  scenarioPatchPreviewStatus,
} from "../../lib/scenarioPatchPreview.mts";
import { benchRunPath } from "../../lib/routes.mts";
import { RunReviewEditor } from "./RunReviewEditor";

interface Check {
  name: string;
  type: string;
  verdict: string;
  message: string;
}

interface ChecksPayload {
  passed: boolean;
  checks: Check[];
}

interface ToolCall {
  tool: string;
  args: Record<string, unknown>;
  result: string;
}

interface Scorecard {
  score: number;
  band: string;
  signals: Record<string, number>;
  [key: string]: unknown;
}

interface TimelineStep {
  index: number;
  phase: string;
  tool: string;
  operation: string;
  command: string;
  summary: string;
  exit_code: number;
}

interface TimelineData {
  steps: TimelineStep[];
  phase_count: Record<string, number>;
  mutation_count: number;
  total_steps: number;
  diagnosis_depth: number;
}

const PHASE_STYLES: Record<string, string> = {
  discover: "text-blue-400 bg-blue-400/10",
  diagnose: "text-purple-400 bg-purple-400/10",
  decide: "text-amber-400 bg-amber-400/10",
  act: "text-green-400 bg-green-400/10",
  verify: "text-teal-400 bg-teal-400/10",
  explain: "text-gray-400 bg-gray-400/10",
};

const PHASE_ORDER = ["discover", "diagnose", "decide", "act", "verify", "explain"];
const API_BASE = import.meta.env.VITE_BENCH_API_URL || "";

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

function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  return s.slice(0, max) + "\u2026";
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

function highlightTranscript(text: string): (React.ReactElement | string)[] {
  const parts = text.split(/(\[(?:user|assistant|tool)\])/g);
  return parts.map((part, i) => {
    if (part === "[user]") {
      return (
        <span key={i} className="text-info font-semibold">
          {part}
        </span>
      );
    }
    if (part === "[assistant]") {
      return (
        <span key={i} className="text-accent font-semibold">
          {part}
        </span>
      );
    }
    if (part === "[tool]") {
      return (
        <span key={i} className="text-warning font-semibold">
          {part}
        </span>
      );
    }
    return part;
  });
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

  const [transcript, setTranscript] = useState<string | null>(null);
  const [transcriptLoading, setTranscriptLoading] = useState(false);
  const [transcriptError, setTranscriptError] = useState<string | null>(null);

  const [toolCalls, setToolCalls] = useState<ToolCall[] | null>(null);
  const [toolCallsLoading, setToolCallsLoading] = useState(false);
  const [toolCallsError, setToolCallsError] = useState<string | null>(null);

  const [scorecard, setScorecard] = useState<Scorecard | null>(null);
  const [scorecardLoading, setScorecardLoading] = useState(false);
  const [scorecardError, setScorecardError] = useState<string | null>(null);

  const [timeline, setTimeline] = useState<TimelineData | null>(null);
  const [timelineLoading, setTimelineLoading] = useState(false);
  const [timelineError, setTimelineError] = useState<string | null>(null);

  const [autopsy, setAutopsy] = useState<AutopsyReport | null>(null);
  const [autopsyLoading, setAutopsyLoading] = useState(false);
  const [autopsyError, setAutopsyError] = useState<string | null>(null);

  const [review, setReview] = useState<RunReview | null>(null);
  const [reviewLoading, setReviewLoading] = useState(false);
  const [reviewError, setReviewError] = useState<string | null>(null);
  const [reviewSaving, setReviewSaving] = useState(false);
  const [reviewSaveError, setReviewSaveError] = useState<string | null>(null);
  const [reviewSaved, setReviewSaved] = useState(false);
  const [reviewDrafting, setReviewDrafting] = useState(false);
  const [reviewDraftError, setReviewDraftError] = useState<string | null>(null);
  const [reviewDraftSeed, setReviewDraftSeed] = useState<RunReview | null>(null);
  const [reviewAutoDraftedRunID, setReviewAutoDraftedRunID] = useState<string | null>(null);
  const [scenarioPatchPreview, setScenarioPatchPreview] = useState<ScenarioPatchPreview | null>(null);
  const [scenarioPatchPreviewLoading, setScenarioPatchPreviewLoading] = useState(false);
  const [scenarioPatchPreviewError, setScenarioPatchPreviewError] = useState<string | null>(null);
  const [scenarioPatchValidation, setScenarioPatchValidation] = useState<ScenarioPatchValidation | null>(null);
  const [scenarioPatchValidationLoading, setScenarioPatchValidationLoading] = useState(false);
  const [scenarioPatchValidationError, setScenarioPatchValidationError] = useState<string | null>(null);
  const [scenarioPatchValidationLoaded, setScenarioPatchValidationLoaded] = useState(false);

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

function MetaItem({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-bg-alt/80 rounded-lg px-3.5 py-2.5">
      <div className="text-[0.68rem] font-semibold text-fg-muted uppercase tracking-wide">
        {label}
      </div>
      <div className="font-mono text-[0.85rem] font-semibold text-fg mt-0.5">
        {value}
      </div>
    </div>
  );
}

function parseChecks(checksJson: string): ChecksPayload | null {
  if (!checksJson) return null;
  try {
    return JSON.parse(checksJson) as ChecksPayload;
  } catch {
    return null;
  }
}

function SummaryTab({
  checks,
  scorecard,
}: {
  checks: ChecksPayload | null;
  scorecard: Scorecard | null;
}) {
  return (
    <div className="space-y-6">
      {/* Verification checks */}
      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">
          Verification Checks
        </h3>
        {checks && checks.checks && checks.checks.length > 0 ? (
          <div className="space-y-1.5">
            {checks.checks.map((c, i) => (
              <div
                key={i}
                className="flex items-center gap-3 px-3 py-2 bg-bg-alt/80 rounded-md text-[0.8rem]"
              >
                <span
                  className={`inline-block w-2 h-2 rounded-full flex-shrink-0 ${
                    c.verdict === "pass" ? "bg-accent" : "bg-danger"
                  }`}
                />
                <span className="font-mono text-fg-muted text-[0.75rem] min-w-[140px]">
                  {c.type}
                </span>
                <span className="text-fg">{c.name}</span>
                {c.message && (
                  <span className="text-fg-muted ml-auto text-[0.75rem] truncate max-w-[300px]">
                    {c.message}
                  </span>
                )}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">No checks recorded.</p>
        )}
      </div>

      {/* Signals */}
      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">
          Signals Detected
        </h3>
        {scorecard && scorecard.signals && Object.keys(scorecard.signals).length > 0 ? (
          <div className="space-y-1.5">
            {Object.entries(scorecard.signals).map(([name, count]) => (
              <div
                key={name}
                className="flex items-center gap-3 px-3 py-2 bg-bg-alt/80 rounded-md text-[0.8rem]"
              >
                <span className="font-mono text-fg">{name}</span>
                <span className="text-fg-muted ml-auto">&times;{count}</span>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">
            No scorecard available. Run with scoring enabled to see signal data.
          </p>
        )}
      </div>
    </div>
  );
}

function ReviewTab({
  run,
  review,
  timeline,
  loading,
  error,
  saving,
  saveError,
  saved,
  drafting,
  draftError,
  draftSeed,
  scenarioPatchPreview,
  scenarioPatchPreviewLoading,
  scenarioPatchPreviewError,
  scenarioPatchValidation,
  scenarioPatchValidationLoading,
  scenarioPatchValidationError,
  onDraft,
  onPreviewScenarioPatch,
  onValidateScenarioPatch,
  onRefreshScenarioPatchValidation,
  onSave,
}: {
  run: BenchRunRecord;
  review: RunReview | null;
  timeline: TimelineData | null;
  loading: boolean;
  error: string | null;
  saving: boolean;
  saveError: string | null;
  saved: boolean;
  drafting: boolean;
  draftError: string | null;
  draftSeed: RunReview | null;
  scenarioPatchPreview: ScenarioPatchPreview | null;
  scenarioPatchPreviewLoading: boolean;
  scenarioPatchPreviewError: string | null;
  scenarioPatchValidation: ScenarioPatchValidation | null;
  scenarioPatchValidationLoading: boolean;
  scenarioPatchValidationError: string | null;
  onDraft: () => Promise<RunReview>;
  onPreviewScenarioPatch: () => Promise<void>;
  onValidateScenarioPatch: () => Promise<void>;
  onRefreshScenarioPatchValidation: () => Promise<void>;
  onSave: (payload: RunReview) => Promise<void>;
}) {
  const canDraftReview = reviewDraftAvailable(run);

  if (loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">Loading human review...</p>;
  }
  if (error === "not-found" || (!review && !loading && !error)) {
    return (
      <div className="space-y-5">
        <div className="rounded-lg border border-border-subtle bg-bg-alt/60 p-4">
          <h3 className="text-[0.9rem] font-semibold text-fg mb-1">No human review yet</h3>
          <p className="text-fg-muted text-[0.82rem]">
            This run has no saved review artifact.
          </p>
        </div>
        <RunReviewEditor
          run={run}
          review={review}
          timeline={timeline}
          saving={saving}
          saveError={saveError}
          saved={saved}
          drafting={drafting}
          draftError={draftError}
          draftSeed={draftSeed}
          humanOnly={!canDraftReview}
          onDraft={canDraftReview ? onDraft : undefined}
          onSave={onSave}
        />
      </div>
    );
  }
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load human review: {error}</p>;
  }
  if (!review) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No human review yet.</p>;
  }

  const view = normalizeRunReviewView(review);
  const canPreviewScenarioPatch = (review.suggested_rules?.length ?? 0) > 0;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div>
          <div className="flex flex-wrap items-center gap-2">
            <span className="inline-block px-2 py-0.5 rounded bg-accent-tint text-accent text-[0.7rem] font-semibold uppercase tracking-wide">
              {view.verdictLabel}
            </span>
            <span className="inline-block px-2 py-0.5 rounded bg-bg-alt/80 text-fg-muted text-[0.7rem] font-semibold uppercase tracking-wide">
              {view.visibilityLabel}
            </span>
            {review.primary_label && (
              <span className="inline-block px-2 py-0.5 rounded bg-warning/15 text-warning text-[0.7rem] font-mono">
                {review.primary_label}
              </span>
            )}
          </div>
          <p className="text-fg-muted text-[0.82rem] mt-2">
            Reviewed by {view.reviewerLabel}
          </p>
        </div>
        <span className="font-mono text-fg-muted text-[0.72rem]">
          {review.version || "run_review.v1"}
        </span>
      </div>

      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">Labels</h3>
        {view.labels.length > 0 ? (
          <div className="space-y-2">
            {view.labels.map((label, i) => (
              <div key={i} className="rounded-md bg-bg-alt/80 px-3 py-2.5 text-[0.8rem]">
                <div className="flex flex-wrap items-center gap-2">
                  <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide ${severityStyle(label.severityLabel.toLowerCase())}`}>
                    {label.severityLabel}
                  </span>
                  <span className="font-mono text-fg">{label.kindLabel}</span>
                  {label.stepLabel && (
                    <span className="font-mono text-fg-muted text-[0.72rem]">
                      {label.stepLabel}
                    </span>
                  )}
                  {label.evidenceRefLabel && (
                    <span className="font-mono text-fg-muted text-[0.72rem]">
                      {label.evidenceRefLabel}
                    </span>
                  )}
                </div>
                {label.note && (
                  <p className="text-fg-muted mt-2 leading-relaxed">{label.note}</p>
                )}
                {label.evidenceSnippet && (
                  <pre className="mt-2 rounded bg-code-bg border border-border-subtle px-3 py-2 font-mono text-[0.72rem] text-fg-muted whitespace-pre-wrap break-words">
                    {label.evidenceSnippet}
                  </pre>
                )}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">No labels saved.</p>
        )}
      </div>

      <div>
        <div className="mb-3 flex flex-col gap-2 sm:flex-row sm:items-center sm:justify-between">
          <h3 className="text-[0.9rem] font-semibold text-fg">Suggested Rules</h3>
          {canPreviewScenarioPatch && (
            <button
              type="button"
              onClick={onPreviewScenarioPatch}
              disabled={scenarioPatchPreviewLoading}
              className="rounded-md border border-border bg-bg-alt px-3 py-1.5 text-[0.78rem] font-semibold text-fg transition-colors hover:border-accent disabled:cursor-default disabled:opacity-50"
            >
              {scenarioPatchPreviewLoading ? "Previewing..." : "Preview scenario patch"}
            </button>
          )}
        </div>
        {view.suggestedRules.length > 0 ? (
          <div className="space-y-1.5">
            {view.suggestedRules.map((rule, i) => (
              <div key={i} className="rounded-md bg-bg-alt/80 px-3 py-2 font-mono text-[0.76rem] text-fg-muted break-words">
                {rule}
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">
            No scenario-rule suggestions saved with this review.
          </p>
        )}
        <ScenarioPatchPreviewPanel
          preview={scenarioPatchPreview}
          loading={scenarioPatchPreviewLoading}
          error={scenarioPatchPreviewError}
          validation={scenarioPatchValidation}
          validationLoading={scenarioPatchValidationLoading}
          validationError={scenarioPatchValidationError}
          onValidate={onValidateScenarioPatch}
          onRefreshValidation={onRefreshScenarioPatchValidation}
        />
      </div>

      <RunReviewEditor
        run={run}
        review={review}
        timeline={timeline}
        saving={saving}
        saveError={saveError}
        saved={saved}
        drafting={drafting}
        draftError={draftError}
        draftSeed={draftSeed}
        humanOnly={!canDraftReview}
        onDraft={canDraftReview ? onDraft : undefined}
        onSave={onSave}
      />
    </div>
  );
}

function ScenarioPatchPreviewPanel({
  preview,
  loading,
  error,
  validation,
  validationLoading,
  validationError,
  onValidate,
  onRefreshValidation,
}: {
  preview: ScenarioPatchPreview | null;
  loading: boolean;
  error: string | null;
  validation: ScenarioPatchValidation | null;
  validationLoading: boolean;
  validationError: string | null;
  onValidate: () => Promise<void>;
  onRefreshValidation: () => Promise<void>;
}) {
  if (loading) {
    return (
      <div className="mt-3 rounded-md border border-border-subtle bg-bg-alt/50 px-3 py-2 text-[0.8rem] text-fg-muted">
        Building scenario patch preview...
      </div>
    );
  }
  if (error) {
    return (
      <div className="mt-3 rounded-md bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.8rem] text-[var(--color-danger-badge-fg)]">
        {error}
      </div>
    );
  }

  const validationProgress = validation ? scenarioPatchValidationProgress(validation) : null;
  const validationRunIDs = validation ? scenarioPatchValidationRunIDs(validation) : [];
  const validationPanel = validation ? (
    <div className="mb-2 rounded border border-border-subtle bg-bg-elevated px-3 py-2 text-[0.76rem] text-fg-muted">
      <div className="flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <span className="block text-fg-body">{scenarioPatchValidationStatus(validation)}</span>
          {validationProgress && <span className="block">{validationProgress}</span>}
          <span className="block font-mono break-all">{validation.trigger_url}</span>
        </div>
        <button
          type="button"
          onClick={onRefreshValidation}
          disabled={validationLoading}
          className="w-fit rounded-md border border-border bg-bg-alt px-2.5 py-1 text-[0.72rem] font-semibold text-fg transition-colors hover:border-accent disabled:cursor-default disabled:opacity-50"
        >
          {validationLoading ? "Refreshing..." : "Refresh"}
        </button>
      </div>
      {validationRunIDs.length > 0 && (
        <div className="mt-2 flex flex-wrap gap-1.5">
          {validationRunIDs.map((runID) => (
            <Link
              key={runID}
              to={benchRunPath(runID)}
              className="rounded border border-border-subtle bg-bg-alt px-2 py-0.5 font-mono text-[0.7rem] text-fg-muted hover:text-accent"
            >
              {runID}
            </Link>
          ))}
        </div>
      )}
    </div>
  ) : null;

  if (!preview) {
    if (!validationPanel && !validationError) return null;
    return (
      <div className="mt-3 rounded-md border border-border-subtle bg-bg-alt/50 p-3">
        {validationPanel}
        {validationError && (
          <div className="rounded bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.76rem] text-[var(--color-danger-badge-fg)]">
            {validationError}
          </div>
        )}
      </div>
    );
  }

  const diffHref = scenarioPatchPreviewDownloadHref(preview, API_BASE);

  return (
    <div className="mt-3 rounded-md border border-border-subtle bg-bg-alt/50 p-3">
      <div className="mb-2 flex flex-col gap-2 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <span className="block text-[0.8rem] font-semibold text-fg">{scenarioPatchPreviewStatus(preview)}</span>
          {preview.scenario_path && (
            <span className="block font-mono text-[0.72rem] text-fg-muted break-all">{preview.scenario_path}</span>
          )}
        </div>
        {diffHref && (
          <div className="flex flex-wrap gap-2">
            <a
              href={diffHref}
              download={scenarioPatchPreviewDiffFilename(preview)}
              className="w-fit rounded-md border border-border bg-bg-alt px-3 py-1.5 text-[0.78rem] font-semibold text-fg transition-colors hover:border-accent"
            >
              Download diff
            </a>
            <button
              type="button"
              onClick={onValidate}
              disabled={validationLoading}
              className="w-fit rounded-md border border-border bg-bg-alt px-3 py-1.5 text-[0.78rem] font-semibold text-fg transition-colors hover:border-accent disabled:cursor-default disabled:opacity-50"
            >
              {validationLoading ? "Queuing..." : "Validate rerun"}
            </button>
          </div>
        )}
      </div>
      {validationPanel}
      {validationError && (
        <div className="mb-2 rounded bg-[var(--color-danger-badge-bg)] px-3 py-2 text-[0.76rem] text-[var(--color-danger-badge-fg)]">
          {validationError}
        </div>
      )}
      {preview.diff ? (
        <pre className="max-h-[360px] overflow-auto rounded border border-border-subtle bg-code-bg px-3 py-2 font-mono text-[0.72rem] leading-relaxed text-fg-muted whitespace-pre">
          {preview.diff}
        </pre>
      ) : (
        <p className="text-[0.8rem] text-fg-muted">No scenario YAML changes were produced.</p>
      )}
      {(preview.skipped_rules?.length ?? 0) > 0 && (
        <div className="mt-2 space-y-1">
          {preview.skipped_rules?.map((rule, index) => (
            <div key={index} className="font-mono text-[0.72rem] text-fg-muted break-words">
              skipped {rule.target || "rule"} {rule.pattern || ""}: {rule.reason || "not applied"}
            </div>
          ))}
        </div>
      )}
    </div>
  );
}

function TranscriptTab({
  transcript,
  loading,
  error,
}: {
  transcript: string | null;
  loading: boolean;
  error: string | null;
}) {
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load transcript: {error}</p>;
  }
  if (!transcript && !loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No transcript available.</p>;
  }

  return (
    <pre
      className={`bg-code-bg border border-border-subtle rounded-lg p-4 font-mono text-[0.78rem] leading-relaxed max-h-[500px] overflow-y-auto whitespace-pre-wrap break-words min-h-[100px] transition-opacity duration-200 ${
        loading ? "opacity-40 animate-pulse" : "opacity-100"
      }`}
    >
      {transcript ? highlightTranscript(transcript) : "\u00A0"}
    </pre>
  );
}

function ToolCallsTab({
  toolCalls,
  loading,
  error,
}: {
  toolCalls: ToolCall[] | null;
  loading: boolean;
  error: string | null;
}) {
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load tool calls: {error}</p>;
  }
  if (!toolCalls && !loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No tool calls recorded.</p>;
  }
  if (loading || !toolCalls || toolCalls.length === 0) {
    return (
      <div className={`min-h-[100px] rounded-lg bg-bg-alt/80 transition-opacity duration-200 ${loading ? "opacity-40 animate-pulse" : "opacity-100"}`}>
        {!loading && <p className="text-fg-muted text-[0.82rem] py-6 text-center">No tool calls recorded.</p>}
      </div>
    );
  }

  return (
    <div className="overflow-x-auto">
      <table className="w-full text-[0.8rem]">
        <thead>
          <tr className="border-b border-border-subtle text-fg-muted text-left">
            <th className="py-2 pr-3 font-semibold w-10">#</th>
            <th className="py-2 pr-3 font-semibold">Tool</th>
            <th className="py-2 pr-3 font-semibold">Arguments</th>
            <th className="py-2 font-semibold">Result</th>
          </tr>
        </thead>
        <tbody>
          {toolCalls.map((tc, i) => {
            const toolColor =
              tc.tool === "prescribe"
                ? "text-accent"
                : tc.tool === "report"
                  ? "text-info"
                  : "text-fg";
            return (
              <tr key={i} className="border-b border-border-subtle/50">
                <td className="py-2 pr-3 font-mono text-fg-muted">{i + 1}</td>
                <td className={`py-2 pr-3 font-mono font-semibold ${toolColor}`}>
                  {tc.tool}
                </td>
                <td className="py-2 pr-3 font-mono text-fg-muted text-[0.75rem]">
                  {truncate(typeof tc.args === "string" ? tc.args : JSON.stringify(tc.args), 80)}
                </td>
                <td className="py-2 font-mono text-fg-muted text-[0.75rem]">
                  {truncate(typeof tc.result === "string" ? tc.result : JSON.stringify(tc.result), 80)}
                </td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function AutopsyTab({
  autopsy,
  loading,
  error,
}: {
  autopsy: AutopsyReport | null;
  loading: boolean;
  error: string | null;
}) {
  if (loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">Loading failure autopsy...</p>;
  }
  if (error === "not-found" || (!autopsy && !loading && !error)) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No failure autopsy available.</p>;
  }
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load failure autopsy: {error}</p>;
  }
  if (!autopsy) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No failure autopsy available.</p>;
  }

  const view = normalizeAutopsyReport(autopsy);
  const findings = view.findings;
  const metrics = view.metrics;

  return (
    <div className="space-y-6">
      <div className="flex flex-col gap-3 sm:flex-row sm:items-start sm:justify-between">
        <div className="min-w-0">
          <div className="flex flex-wrap items-center gap-2">
            <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide ${
              view.outcome === "pass" ? "bg-accent-tint text-accent" : "bg-danger/15 text-danger"
            }`}>
              {view.outcome}
            </span>
            <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide ${confidenceStyle(view.confidence)}`}>
              {view.confidence}
            </span>
            <span className="inline-block px-2 py-0.5 rounded bg-bg-alt/80 text-fg-muted text-[0.7rem] font-mono">
              {view.version}
            </span>
            <span className="text-fg font-semibold text-[1rem] break-words">
              {view.primaryLabel}
            </span>
          </div>
          <p className="text-fg-muted text-[0.82rem] mt-2 max-w-3xl break-words">
            {view.summary}
          </p>
        </div>
        <div className="flex flex-wrap gap-2 text-[0.78rem] flex-shrink-0 sm:justify-end">
          <span className="font-mono text-fg-muted bg-bg-alt/80 rounded-md px-2.5 py-1">
            wasted turns {view.waste.turns}
          </span>
          <span className="font-mono text-fg-muted bg-bg-alt/80 rounded-md px-2.5 py-1">
            wasted tokens {formatCompactTokens(view.waste.tokens)}
          </span>
          {view.waste.basis && (
            <span className="font-mono text-fg-muted bg-bg-alt/80 rounded-md px-2.5 py-1">
              basis {view.waste.basis}
            </span>
          )}
        </div>
      </div>

      <div className="grid grid-cols-2 md:grid-cols-4 gap-2">
        <AutopsyMetric label="Turns" value={String(metrics.turns)} />
        <AutopsyMetric label="Tokens" value={formatCompactTokens(metrics.total_tokens)} />
        <AutopsyMetric label="Checks" value={`${metrics.checks_passed}/${metrics.checks_total}`} />
        <AutopsyMetric label="Cost" value={`$${metrics.estimated_cost_usd.toFixed(4)}`} />
        <AutopsyMetric label="Steps" value={String(metrics.total_steps)} />
        <AutopsyMetric label="Mutations" value={String(metrics.mutation_count)} />
        <AutopsyMetric label="Diagnosis" value={String(metrics.diagnosis_depth)} />
        <AutopsyMetric label="Prompt / Completion" value={`${formatCompactTokens(metrics.prompt_tokens)} / ${formatCompactTokens(metrics.completion_tokens)}`} />
      </div>

      <div>
        <h3 className="text-[0.9rem] font-semibold text-fg mb-3">
          Findings
        </h3>
        {findings.length > 0 ? (
          <div className="space-y-1.5">
            {findings.map((finding, i) => (
              <div
                key={`${finding.kind}-${i}`}
                className="flex flex-col gap-2 px-3 py-2.5 bg-bg-alt/80 rounded-md text-[0.8rem] sm:flex-row sm:items-start"
              >
                <span className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide flex-shrink-0 ${severityStyle(finding.severity)}`}>
                  {finding.severity}
                </span>
                <div className="min-w-0 flex-1">
                  <div className="font-mono text-fg text-[0.78rem] break-words">
                    {finding.kindLabel}
                  </div>
                  <div className="text-fg-muted mt-0.5 break-words">
                    {finding.message}
                  </div>
                  {finding.evidenceText && (
                    <div className="font-mono text-fg-muted text-[0.72rem] mt-1 break-all">
                      {finding.evidenceText}
                    </div>
                  )}
                </div>
              </div>
            ))}
          </div>
        ) : (
          <p className="text-fg-muted text-[0.82rem]">
            No deterministic failure pattern matched.
          </p>
        )}
      </div>
    </div>
  );
}

function AutopsyMetric({ label, value }: { label: string; value: string }) {
  return (
    <div className="bg-bg-alt/80 rounded-md px-3 py-2 min-w-0">
      <div className="text-[0.65rem] font-semibold text-fg-muted uppercase tracking-wide">
        {label}
      </div>
      <div className="font-mono text-[0.78rem] font-semibold text-fg mt-0.5 break-words">
        {value}
      </div>
    </div>
  );
}

function severityStyle(severity: string): string {
  if (severity === "critical") return "bg-danger/15 text-danger";
  if (severity === "warning") return "bg-warning/15 text-warning";
  return "bg-accent-tint text-accent";
}

function confidenceStyle(confidence: string): string {
  if (confidence === "high") return "bg-accent-tint text-accent";
  if (confidence === "medium") return "bg-warning/15 text-warning";
  if (confidence === "low") return "bg-danger/15 text-danger";
  return "bg-bg-alt/80 text-fg-muted";
}

function TimelineTab({
  timeline,
  loading,
  error,
}: {
  timeline: TimelineData | null;
  loading: boolean;
  error: string | null;
}) {
  if (loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">Loading timeline...</p>;
  }
  if (error === "not-found" || (!timeline && !loading && !error)) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No timeline available.</p>;
  }
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load timeline: {error}</p>;
  }
  if (!timeline || timeline.steps.length === 0) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No timeline available.</p>;
  }

  const summaryParts = PHASE_ORDER
    .filter((p) => timeline.phase_count[p] && timeline.phase_count[p] > 0)
    .map((p) => `${timeline.phase_count[p]} ${p}`);

  return (
    <div className="space-y-4">
      {/* Summary line */}
      <p className="text-fg-muted text-[0.82rem]">
        {timeline.total_steps} step{timeline.total_steps !== 1 ? "s" : ""}
        {summaryParts.length > 0 && ": "}
        {summaryParts.join(" \u2192 ")}
      </p>

      {/* Step list */}
      <div className="space-y-1.5">
        {timeline.steps.map((step) => {
          const phaseStyle = PHASE_STYLES[step.phase] || PHASE_STYLES.explain;
          return (
            <div
              key={step.index}
              className="flex items-start gap-3 px-3 py-2 bg-bg-alt/80 rounded-md text-[0.8rem]"
            >
              <span className="font-mono text-fg-muted text-[0.75rem] min-w-[1.5rem] text-right flex-shrink-0 pt-0.5">
                {step.index + 1}
              </span>
              <span
                className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold uppercase tracking-wide flex-shrink-0 ${phaseStyle}`}
              >
                {step.phase}
              </span>
              <div className="min-w-0 flex-1">
                <span className="text-fg">{step.summary}</span>
                {step.command && (
                  <div className="font-mono text-fg-muted text-[0.72rem] mt-0.5 truncate">
                    {step.command}
                  </div>
                )}
              </div>
              {step.exit_code !== 0 && (
                <span className="text-[0.7rem] font-mono text-danger flex-shrink-0 pt-0.5">
                  exit {step.exit_code}
                </span>
              )}
            </div>
          );
        })}
      </div>
    </div>
  );
}

function ScorecardTab({
  scorecard,
  loading,
  error,
}: {
  scorecard: Scorecard | null;
  loading: boolean;
  error: string | null;
}) {
  if (loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">Loading scorecard...</p>;
  }
  if (error === "not-found" || (!scorecard && !loading && !error)) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No scorecard available.</p>;
  }
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load scorecard: {error}</p>;
  }
  if (!scorecard) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No scorecard available.</p>;
  }

  const signals = scorecard.signals || {};

  return (
    <div className="space-y-6">
      {/* Hero score */}
      <div className="flex items-baseline gap-4">
        <span className="font-mono text-accent font-bold" style={{ fontSize: "3rem", lineHeight: 1 }}>
          {scorecard.score}
        </span>
        <div>
          <span className="text-fg font-semibold text-[1rem]">{scorecard.band}</span>
          <p className="text-fg-muted text-[0.78rem] mt-0.5">
            {Object.keys(signals).length} signal{Object.keys(signals).length !== 1 ? "s" : ""} evaluated
          </p>
        </div>
      </div>

      {/* Signal breakdown */}
      {Object.keys(signals).length > 0 && (
        <div>
          <h3 className="text-[0.9rem] font-semibold text-fg mb-3">
            Signal Breakdown
          </h3>
          <table className="w-full text-[0.8rem]">
            <thead>
              <tr className="border-b border-border-subtle text-fg-muted text-left">
                <th className="py-2 pr-3 font-semibold">Signal</th>
                <th className="py-2 pr-3 font-semibold w-20">Count</th>
                <th className="py-2 font-semibold w-28">Status</th>
              </tr>
            </thead>
            <tbody>
              {Object.entries(signals).map(([name, count]) => (
                <tr key={name} className="border-b border-border-subtle/50">
                  <td className="py-2 pr-3 font-mono text-fg">{name}</td>
                  <td className="py-2 pr-3 font-mono text-fg-muted">{count}</td>
                  <td className="py-2">
                    <span
                      className={`inline-block px-2 py-0.5 rounded text-[0.7rem] font-semibold ${
                        count > 0
                          ? "bg-warning/15 text-warning"
                          : "bg-accent-tint text-accent"
                      }`}
                    >
                      {count > 0 ? "detected" : "clear"}
                    </span>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        </div>
      )}
    </div>
  );
}
