import assert from "node:assert/strict";
import test from "node:test";

import { scenarioPatchPreviewStatus, type ScenarioPatchPreview } from "./scenarioPatchPreview.mts";

test("scenarioPatchPreviewStatus summarizes changed patch previews", () => {
  const preview: ScenarioPatchPreview = {
    version: "scenario_patch_preview.v1",
    run_id: "r1",
    scenario_id: "shared-configmap-trap",
    scenario_path: "kubernetes/shared-configmap-trap/scenario.yaml",
    changed: true,
    diff: "--- scenario.yaml\n+++ scenario.yaml (review preview)\n",
    added_rules: [
      {
        target: "autopsy.expected_diagnostics",
        section: "expected_diagnostics",
        kind: "command_pattern",
        pattern: "kubectl get configmap app-config -n bench",
      },
    ],
    skipped_rules: [],
  };

  assert.equal(scenarioPatchPreviewStatus(preview), "1 scenario rule previewed");
});

test("scenarioPatchPreviewStatus summarizes no-op previews", () => {
  const preview: ScenarioPatchPreview = {
    version: "scenario_patch_preview.v1",
    run_id: "r1",
    scenario_id: "shared-configmap-trap",
    scenario_path: "kubernetes/shared-configmap-trap/scenario.yaml",
    changed: false,
    diff: "",
    added_rules: [],
    skipped_rules: [{ target: "autopsy.expected_diagnostics", kind: "command_pattern", pattern: "kubectl get pods", reason: "duplicate" }],
  };

  assert.equal(scenarioPatchPreviewStatus(preview), "No scenario changes; 1 suggestion skipped");
});
