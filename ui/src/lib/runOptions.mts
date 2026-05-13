export const SCENARIO_CATEGORIES = ["All", "kubernetes", "helm", "argocd", "terraform", "aws"] as const;

export const RUN_MODELS_BY_PROVIDER = {
  claude: ["sonnet", "haiku", "opus"],
  bifrost: ["gpt-4o", "gpt-4.1", "gpt-5.2", "gemini-2.5-flash", "gemini-2.5-pro", "qwen-plus"],
  anthropic: ["claude-haiku-4-20250514", "claude-sonnet-4-20250514", "claude-opus-4-20250514"],
} as const;

export type RunProvider = keyof typeof RUN_MODELS_BY_PROVIDER;

export const RUN_PROVIDERS = Object.keys(RUN_MODELS_BY_PROVIDER) as RunProvider[];

const DEFAULT_MODEL_BY_PROVIDER: Record<RunProvider, string> = {
  claude: "sonnet",
  bifrost: "gpt-4o",
  anthropic: "claude-sonnet-4-20250514",
};

export const DEFAULT_RUN_SELECTION = {
  provider: "claude",
  model: "sonnet",
} as const;

export function isRunProvider(value: string): value is RunProvider {
  return value in RUN_MODELS_BY_PROVIDER;
}

export function getModelsForProvider(provider: string): string[] {
  if (!isRunProvider(provider)) {
    return [...RUN_MODELS_BY_PROVIDER[DEFAULT_RUN_SELECTION.provider]];
  }
  return [...RUN_MODELS_BY_PROVIDER[provider]];
}

export function normalizeRunSelection(provider: string, model: string): { provider: RunProvider; model: string } {
  const normalizedProvider = isRunProvider(provider) ? provider : DEFAULT_RUN_SELECTION.provider;
  const models = getModelsForProvider(normalizedProvider);
  const normalizedModel = models.includes(model) ? model : DEFAULT_MODEL_BY_PROVIDER[normalizedProvider];
  return {
    provider: normalizedProvider,
    model: normalizedModel,
  };
}
