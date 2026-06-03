import { useState } from "react";
import type { AutopsyReport } from "../../../lib/autopsyView.mts";
import type { Scorecard, TimelineData, ToolCall } from "./types";

export function useRunArtifacts() {
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

  return {
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
  };
}
