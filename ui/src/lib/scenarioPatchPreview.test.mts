import assert from "node:assert/strict";
import test from "node:test";

import {
  scenarioPatchPreviewDiffFilename,
  scenarioPatchPreviewDownloadContent,
  scenarioPatchPreviewStatus,
  type ScenarioPatchPreview,
} from "./scenarioPatchPreview.mts";

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

test("scenarioPatchPreviewDiffFilename uses scenario and run identifiers", () => {
  const preview: ScenarioPatchPreview = {
    run_id: "run/123",
    scenario_id: "kubernetes/shared-configmap-trap",
    changed: true,
    diff: "--- scenario.yaml\n+++ scenario.yaml (review preview)\n",
  };

  assert.equal(
    scenarioPatchPreviewDiffFilename(preview),
    "evidra-scenario-patch-kubernetes-shared-configmap-trap-run-123.diff",
  );
});

test("scenarioPatchPreviewDownloadContent returns only the unified diff", () => {
  const diff = "--- scenario.yaml\n+++ scenario.yaml (review preview)\n@@ -1 +1,3 @@\n autopsy:\n+  expected_diagnostics:\n+    - kind: command_pattern\n";
  const preview: ScenarioPatchPreview = {
    run_id: "r1",
    scenario_id: "shared-configmap-trap",
    changed: true,
    diff,
  };

  assert.equal(scenarioPatchPreviewDownloadContent(preview), diff);
  assert.equal(scenarioPatchPreviewDownloadContent({ ...preview, changed: false }), null);
  assert.equal(scenarioPatchPreviewDownloadContent({ ...preview, diff: "" }), null);
});
