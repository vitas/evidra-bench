import assert from "node:assert/strict";
import test from "node:test";

import {
  scenarioPatchValidationApiPath,
  scenarioPatchValidationProgress,
  scenarioPatchValidationRunIDs,
  scenarioPatchValidationStatus,
  scenarioPatchPreviewDiffFilename,
  scenarioPatchPreviewDownloadHref,
  scenarioPatchPreviewDownloadContent,
  scenarioPatchPreviewStatus,
  type ScenarioPatchValidation,
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

test("scenarioPatchPreviewDownloadHref uses the backend diff artifact URL", () => {
  const preview: ScenarioPatchPreview = {
    run_id: "r1",
    scenario_id: "shared-configmap-trap",
    changed: true,
    diff: "--- scenario.yaml\n+++ scenario.yaml (review preview)\n",
    diff_url: "/v1/bench/runs/r1/scenario-patch.diff",
  };

  assert.equal(scenarioPatchPreviewDownloadHref(preview), "/v1/bench/runs/r1/scenario-patch.diff");
  assert.equal(
    scenarioPatchPreviewDownloadHref(preview, "https://api.example.test/"),
    "https://api.example.test/v1/bench/runs/r1/scenario-patch.diff",
  );
  assert.equal(scenarioPatchPreviewDownloadHref({ ...preview, changed: false }), null);
  assert.equal(scenarioPatchPreviewDownloadHref({ ...preview, diff_url: "" }), null);
});

test("scenarioPatchValidation helpers target the backend validation endpoint", () => {
  const validation: ScenarioPatchValidation = {
    version: "scenario_patch_validation.v1",
    source_run_id: "run-123",
    source_run_url: "/v1/bench/runs/run-123",
    scenario_id: "shared-configmap-trap",
    model: "sonnet",
    provider: "anthropic",
    trigger_id: "job-1",
    trigger_url: "/v1/bench/trigger/job-1",
    validation_url: "/v1/bench/runs/run-123/scenario-patch-validation",
    status: "pending",
    total: 1,
    completed: 0,
    passed: 0,
    failed: 0,
    validation_run_ids: ["validation-run", "validation-run", " "],
    patch_preview_url: "/v1/bench/runs/run-123/scenario-patch-preview",
    patch_diff_url: "/v1/bench/runs/run-123/scenario-patch.diff",
  };

  assert.equal(
    scenarioPatchValidationApiPath("run/123"),
    "/v1/bench/runs/run%2F123/scenario-patch-validation",
  );
  assert.equal(scenarioPatchValidationStatus(validation), "Validation rerun queued for shared-configmap-trap");
  assert.equal(scenarioPatchValidationProgress(validation), "0/1 complete; 0 passed, 0 failed");
  assert.deepEqual(scenarioPatchValidationRunIDs(validation), ["validation-run"]);
});
