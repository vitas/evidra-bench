export interface ModelInfo {
  id: string;
  label: string;
  cost: string;
  costPerRun: number;
}

export const MODELS: ModelInfo[] = [
  { id: "gemini-2.5-flash", label: "Gemini 2.5 Flash", cost: "$0.001/run", costPerRun: 0.001 },
  { id: "gpt-4.1", label: "GPT-4.1", cost: "$0.08/run", costPerRun: 0.08 },
  { id: "gpt-4o", label: "GPT-4o", cost: "$0.03/run", costPerRun: 0.03 },
  { id: "claude-sonnet-4-20250514", label: "Claude Sonnet 4", cost: "$0.24/run", costPerRun: 0.24 },
  { id: "gpt-5.2", label: "GPT-5.2", cost: "$0.10/run", costPerRun: 0.10 },
  { id: "qwen-plus", label: "Qwen Plus", cost: "$0.02/run", costPerRun: 0.02 },
];
