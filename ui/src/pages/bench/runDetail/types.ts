export interface ToolCall {
  tool: string;
  args: Record<string, unknown>;
  result: string;
}

export interface Scorecard {
  score: number;
  band: string;
  signals: Record<string, number>;
  [key: string]: unknown;
}

export interface TimelineStep {
  index: number;
  phase: string;
  tool: string;
  operation: string;
  command: string;
  summary: string;
  exit_code: number;
}

export interface TimelineData {
  steps: TimelineStep[];
  phase_count: Record<string, number>;
  mutation_count: number;
  total_steps: number;
  diagnosis_depth: number;
}
