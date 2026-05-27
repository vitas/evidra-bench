import { buildBenchApiURL } from "./apiBase.mts";

export interface ScenarioPatchPreview {
  version?: string;
  run_id?: string;
  scenario_id?: string;
  scenario_path?: string;
  changed?: boolean;
  diff?: string;
  artifact_url?: string;
  diff_url?: string;
  added_rules?: ScenarioPatchRuleChange[];
  skipped_rules?: ScenarioPatchRuleSkip[];
}

export interface ScenarioPatchValidation {
  version?: string;
  source_run_id: string;
  scenario_id: string;
  model: string;
  provider?: string;
  trigger_id: string;
  trigger_url: string;
  status: string;
  mode?: string;
  patch_preview_url: string;
  patch_diff_url: string;
}

export interface ScenarioPatchRuleChange {
  target?: string;
  section?: string;
  kind?: string;
  pattern?: string;
}

export interface ScenarioPatchRuleSkip {
  target?: string;
  kind?: string;
  pattern?: string;
  reason?: string;
}

export function scenarioPatchPreviewStatus(preview: ScenarioPatchPreview): string {
  const added = preview.added_rules?.length ?? 0;
  const skipped = preview.skipped_rules?.length ?? 0;
  if (preview.changed && added > 0) {
    return `${added} scenario ${added === 1 ? "rule" : "rules"} previewed`;
  }
  if (skipped > 0) {
    return `No scenario changes; ${skipped} ${skipped === 1 ? "suggestion" : "suggestions"} skipped`;
  }
  return "No scenario changes";
}

export function scenarioPatchPreviewDownloadContent(preview: ScenarioPatchPreview): string | null {
  if (!preview.changed || !preview.diff) return null;
  return preview.diff;
}

export function scenarioPatchPreviewDownloadHref(
  preview: ScenarioPatchPreview,
  apiBase: string | undefined = "",
): string | null {
  if (!preview.changed || !preview.diff_url?.trim()) return null;
  return buildBenchApiURL(apiBase, preview.diff_url);
}

export function scenarioPatchPreviewDiffFilename(preview: ScenarioPatchPreview): string {
  const scenario = slugPart(preview.scenario_id || preview.scenario_path || "scenario");
  const run = slugPart(preview.run_id || "run");
  return `evidra-scenario-patch-${scenario}-${run}.diff`;
}

export function scenarioPatchValidationApiPath(runID: string): string {
  return `/v1/bench/runs/${encodeURIComponent(runID)}/scenario-patch-validation`;
}

export function scenarioPatchValidationStatus(validation: ScenarioPatchValidation): string {
  const scenario = validation.scenario_id || "scenario";
  if (validation.status === "completed") {
    return `Validation rerun completed for ${scenario}`;
  }
  if (validation.status === "failed" || validation.status === "error") {
    return `Validation rerun failed for ${scenario}`;
  }
  return `Validation rerun queued for ${scenario}`;
}

function slugPart(value: string): string {
  const slug = value.trim().toLowerCase().replace(/[^a-z0-9]+/g, "-").replace(/^-+|-+$/g, "");
  return slug || "unknown";
}
