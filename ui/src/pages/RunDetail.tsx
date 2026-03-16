import { useEffect, useState } from "react";
import { useParams, Link } from "react-router";
import { useApi } from "../hooks/useApi";

interface RunRecord {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  passed: boolean;
  duration_seconds: number;
  turns: number;
  prompt_tokens: number;
  completion_tokens: number;
  estimated_cost_usd: number;
  exit_code: number;
  checks_passed: number;
  checks_total: number;
  checks_json: string;
  created_at: string;
}

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
  args: string;
  result: string;
}

interface Scorecard {
  score: number;
  band: string;
  signals: Record<string, number>;
  [key: string]: unknown;
}

type Tab = "summary" | "transcript" | "tool-calls" | "scorecard";

const TABS: { key: Tab; label: string }[] = [
  { key: "summary", label: "Summary" },
  { key: "transcript", label: "Transcript" },
  { key: "tool-calls", label: "Tool Calls" },
  { key: "scorecard", label: "Scorecard" },
];

function formatDuration(seconds: number): string {
  if (seconds < 60) return `${seconds.toFixed(1)}s`;
  const m = Math.floor(seconds / 60);
  const s = Math.round(seconds % 60);
  return `${m}m ${s}s`;
}

function formatTokens(n: number): string {
  if (n >= 1_000_000) return `${(n / 1_000_000).toFixed(1)}M`;
  if (n >= 1_000) return `${(n / 1_000).toFixed(1)}k`;
  return String(n);
}

function truncate(s: string, max: number): string {
  if (s.length <= max) return s;
  return s.slice(0, max) + "\u2026";
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
  const { request } = useApi();

  const [run, setRun] = useState<RunRecord | null>(null);
  const [loading, setLoading] = useState(true);
  const [error, setError] = useState<string | null>(null);
  const [activeTab, setActiveTab] = useState<Tab>("summary");

  const [transcript, setTranscript] = useState<string | null>(null);
  const [transcriptLoading, setTranscriptLoading] = useState(false);
  const [transcriptError, setTranscriptError] = useState<string | null>(null);

  const [toolCalls, setToolCalls] = useState<ToolCall[] | null>(null);
  const [toolCallsLoading, setToolCallsLoading] = useState(false);
  const [toolCallsError, setToolCallsError] = useState<string | null>(null);

  const [scorecard, setScorecard] = useState<Scorecard | null>(null);
  const [scorecardLoading, setScorecardLoading] = useState(false);
  const [scorecardError, setScorecardError] = useState<string | null>(null);

  // Fetch run record
  useEffect(() => {
    if (!id) return;
    setLoading(true);
    request<RunRecord>(`/v1/bench/runs/${id}`)
      .then(setRun)
      .catch((err) => setError(err.message))
      .finally(() => setLoading(false));
  }, [id, request]);

  // Fetch transcript on tab switch
  useEffect(() => {
    if (activeTab !== "transcript" || transcript !== null || transcriptLoading || !id) return;
    setTranscriptLoading(true);
    fetch(`/v1/bench/runs/${id}/transcript`)
      .then((res) => {
        if (!res.ok) throw new Error(res.statusText);
        return res.text();
      })
      .then(setTranscript)
      .catch((err) => setTranscriptError(err.message))
      .finally(() => setTranscriptLoading(false));
  }, [activeTab, transcript, transcriptLoading, id]);

  // Fetch tool calls on tab switch
  useEffect(() => {
    if (activeTab !== "tool-calls" || toolCalls !== null || toolCallsLoading || !id) return;
    setToolCallsLoading(true);
    fetch(`/v1/bench/runs/${id}/tool-calls`)
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
  }, [activeTab, toolCalls, toolCallsLoading, id]);

  // Fetch scorecard on tab switch
  useEffect(() => {
    if (activeTab !== "scorecard" || scorecard !== null || scorecardLoading || !id) return;
    setScorecardLoading(true);
    request<Scorecard>(`/v1/bench/runs/${id}/scorecard`)
      .then(setScorecard)
      .catch((err) => {
        if (err.message.includes("Not Found") || err.message.includes("404")) {
          setScorecard(null);
          setScorecardError("not-found");
        } else {
          setScorecardError(err.message);
        }
      })
      .finally(() => setScorecardLoading(false));
  }, [activeTab, scorecard, scorecardLoading, id, request]);

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
        <Link to="/runs" className="text-accent text-[0.82rem] hover:underline">
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
          to="/runs"
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
              ? "bg-success/15 text-success"
              : "bg-danger/15 text-danger"
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
          value={`${formatTokens(run.prompt_tokens)} / ${formatTokens(run.completion_tokens)}`}
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
      <div className="border-b border-border-subtle flex gap-0">
        {TABS.map(({ key, label }) => (
          <button
            key={key}
            onClick={() => setActiveTab(key)}
            className={`text-[0.82rem] font-medium px-4 py-2 border-b-2 transition-colors cursor-pointer ${
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
    <div className="bg-bg-alt rounded-lg px-3.5 py-2.5">
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
        {checks && checks.checks.length > 0 ? (
          <div className="space-y-1.5">
            {checks.checks.map((c, i) => (
              <div
                key={i}
                className="flex items-center gap-3 px-3 py-2 bg-bg-alt rounded-md text-[0.8rem]"
              >
                <span
                  className={`inline-block w-2 h-2 rounded-full flex-shrink-0 ${
                    c.verdict === "pass" ? "bg-success" : "bg-danger"
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
                className="flex items-center gap-3 px-3 py-2 bg-bg-alt rounded-md text-[0.8rem]"
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

function TranscriptTab({
  transcript,
  loading,
  error,
}: {
  transcript: string | null;
  loading: boolean;
  error: string | null;
}) {
  if (loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">Loading transcript...</p>;
  }
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load transcript: {error}</p>;
  }
  if (!transcript) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No transcript available.</p>;
  }

  return (
    <pre className="bg-code-bg border border-border-subtle rounded-lg p-4 font-mono text-[0.78rem] leading-relaxed max-h-[500px] overflow-y-auto whitespace-pre-wrap break-words">
      {highlightTranscript(transcript)}
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
  if (loading) {
    return <p className="text-fg-muted text-[0.82rem] py-6">Loading tool calls...</p>;
  }
  if (error) {
    return <p className="text-danger text-[0.82rem] py-6">Failed to load tool calls: {error}</p>;
  }
  if (!toolCalls || toolCalls.length === 0) {
    return <p className="text-fg-muted text-[0.82rem] py-6">No tool calls recorded.</p>;
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
                          : "bg-success/15 text-success"
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
