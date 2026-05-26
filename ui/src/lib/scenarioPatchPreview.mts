export interface ScenarioPatchPreview {
  version?: string;
  run_id?: string;
  scenario_id?: string;
  scenario_path?: string;
  changed?: boolean;
  diff?: string;
  added_rules?: ScenarioPatchRuleChange[];
  skipped_rules?: ScenarioPatchRuleSkip[];
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
