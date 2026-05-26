export interface BenchRunRecord {
  id: string;
  scenario_id: string;
  model: string;
  provider: string;
  adapter: string;
  tool_server: string;
  tool_server_version: string;
  scenario_version: string;
  skill_id: string;
  skill_version: string;
  skill_source: string;
  skill_sha256: string;
  passed: boolean;
  duration_seconds: number;
  exit_code: number;
  turns: number;
  memory_window: number;
  prompt_tokens: number;
  completion_tokens: number;
  estimated_cost_usd: number;
  checks_passed: number;
  checks_total: number;
  checks_json: string;
  metadata_json: string;
  artifact_dir: string;
  created_at: string;
  review_summary?: BenchRunReviewSummary;
}

export interface BenchRunReviewSummary {
  verdict: string;
  primary_label?: string;
  visibility: string;
  label_count: number;
  max_severity?: string;
}

export interface BenchRunsResponse {
  runs: BenchRunRecord[];
  total: number;
  limit?: number;
  offset?: number;
}

export interface BenchScenarioSummary {
  id: string;
  title: string;
  description?: string;
  autopsy_description?: string;
  category: string;
  track?: string;
  level?: string;
  tags: string[];
  chaos: boolean;
}

export interface BenchScenariosResponse {
  scenarios?: BenchScenarioSummary[];
  items?: BenchScenarioSummary[];
}
