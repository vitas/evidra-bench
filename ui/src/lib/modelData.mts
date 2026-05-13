import { MODELS } from "../data/models.ts";
import type { EnabledModel } from "../types/models";

export function fallbackModels(): EnabledModel[] {
  return MODELS.map((model) => ({
    id: model.id,
    display_name: model.label,
    provider: "",
    available: true,
    input_cost_per_mtok: 0,
    output_cost_per_mtok: 0,
  }));
}

export function selectAvailableModels(models: EnabledModel[] | null | undefined): EnabledModel[] {
  if (!Array.isArray(models) || models.length === 0) {
    return fallbackModels();
  }
  // Show only models with configured API keys.
  const available = models.filter((m) => m.available);
  return available.length > 0 ? available : fallbackModels();
}
