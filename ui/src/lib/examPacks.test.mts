import assert from "node:assert/strict";
import test from "node:test";

import {
  EXAM_PACKS,
  countExamPackMatches,
  resolveExamPackFilter,
  scenarioIDsForExamPack,
  scenarioMatchesExamPack,
  type ExamPackScenario,
} from "./examPacks.mts";

const scenarios: ExamPackScenario[] = [
  { category: "kubernetes", track: "workloads", level: "L1" },
  { category: "kubernetes", track: "troubleshooting", level: "L2" },
  { category: "kubernetes", track: "networking", level: "L3" },
  { category: "kubernetes", track: "pod-security", level: "L2" },
  { category: "kubernetes", track: "runtime-security", level: "L2" },
  { category: "argocd", track: "release-ops", level: "L2" },
  { category: "helm", track: "release-ops", level: "L3" },
  { category: "terraform", track: "platform-eng", level: "L3" },
  { category: "aws", track: "pod-security", level: "L2" },
  { category: "kubernetes", track: "workloads", level: "L1", chaos: true },
];

const identifiedScenarios = scenarios.map((scenario, index) => ({
  ...scenario,
  id: `s${index + 1}`,
}));

test("exam packs expose the public marketing suites in display order", () => {
  assert.deepEqual(
    EXAM_PACKS.map((pack) => pack.id),
    [
      "kubernetes-admin",
      "kubernetes-security",
      "gitops-release",
      "terraform-cloud",
      "mcp-readiness",
    ],
  );
});

test("scenarioMatchesExamPack maps scenarios into public exam suites", () => {
  assert.equal(scenarioMatchesExamPack(scenarios[0], "kubernetes-admin"), true);
  assert.equal(scenarioMatchesExamPack(scenarios[2], "kubernetes-admin"), true);
  assert.equal(scenarioMatchesExamPack(scenarios[3], "kubernetes-admin"), false);
  assert.equal(scenarioMatchesExamPack(scenarios[3], "kubernetes-security"), true);
  assert.equal(scenarioMatchesExamPack(scenarios[5], "gitops-release"), true);
  assert.equal(scenarioMatchesExamPack(scenarios[7], "terraform-cloud"), true);
  assert.equal(scenarioMatchesExamPack(scenarios[8], "terraform-cloud"), true);
});

test("mcp-readiness is a cross-suite execution pack for non-trivial scenarios", () => {
  assert.equal(scenarioMatchesExamPack(scenarios[0], "mcp-readiness"), false);
  assert.equal(scenarioMatchesExamPack(scenarios[1], "mcp-readiness"), true);
  assert.equal(scenarioMatchesExamPack(scenarios[9], "mcp-readiness"), true);
});

test("countExamPackMatches returns stable counts per pack", () => {
  assert.deepEqual(countExamPackMatches(scenarios), {
    "kubernetes-admin": 4,
    "kubernetes-security": 2,
    "gitops-release": 2,
    "terraform-cloud": 2,
    "mcp-readiness": 9,
  });
});

test("resolveExamPackFilter accepts only known URL values", () => {
  assert.equal(resolveExamPackFilter("kubernetes-admin"), "kubernetes-admin");
  assert.equal(resolveExamPackFilter("mcp-readiness"), "mcp-readiness");
  assert.equal(resolveExamPackFilter("unknown"), "all");
  assert.equal(resolveExamPackFilter(""), "all");
  assert.equal(resolveExamPackFilter(null), "all");
});

test("scenarioIDsForExamPack returns concrete scenario IDs for a suite", () => {
  assert.deepEqual(
    scenarioIDsForExamPack(identifiedScenarios, "kubernetes-security"),
    ["s4", "s5"],
  );
  assert.deepEqual(
    scenarioIDsForExamPack(identifiedScenarios, "all"),
    [],
  );
});
