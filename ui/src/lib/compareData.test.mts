import assert from "node:assert/strict";
import test from "node:test";

import {
  categoriesFromScenarios,
  scenarioIdsForCategory,
} from "./compareData.mts";

const scenarios = [
  { id: "broken-deployment", category: "kubernetes" },
  { id: "wrong-probes", category: "kubernetes" },
  { id: "failed-upgrade", category: "helm" },
  { id: "out-of-sync", category: "argocd" },
];

test("categoriesFromScenarios uses real scenario metadata", () => {
  assert.deepEqual(categoriesFromScenarios(scenarios), ["argocd", "helm", "kubernetes"]);
});

test("scenarioIdsForCategory expands category to concrete scenario IDs", () => {
  assert.deepEqual(scenarioIdsForCategory(scenarios, "kubernetes"), [
    "broken-deployment",
    "wrong-probes",
  ]);
  assert.deepEqual(scenarioIdsForCategory(scenarios, ""), []);
});
