import assert from "node:assert/strict";
import test from "node:test";

import {
  DEFAULT_RUN_SELECTION,
  SCENARIO_CATEGORIES,
  getModelsForProvider,
  normalizeRunSelection,
} from "./runOptions.mts";

test("normalizeRunSelection keeps valid provider/model pairs", () => {
  assert.deepEqual(normalizeRunSelection("claude", "opus"), {
    provider: "claude",
    model: "opus",
  });
  assert.deepEqual(normalizeRunSelection("bifrost", "gpt-4o"), {
    provider: "bifrost",
    model: "gpt-4o",
  });
});

test("normalizeRunSelection coerces invalid pairings to provider defaults", () => {
  assert.deepEqual(normalizeRunSelection("claude", "gpt-4o"), DEFAULT_RUN_SELECTION);
  assert.deepEqual(normalizeRunSelection("anthropic", "sonnet"), {
    provider: "anthropic",
    model: "claude-sonnet-4-20250514",
  });
});

test("getModelsForProvider only exposes supported models", () => {
  assert.deepEqual(getModelsForProvider("anthropic"), [
    "claude-haiku-4-20250514",
    "claude-sonnet-4-20250514",
    "claude-opus-4-20250514",
  ]);
});

test("scenario categories match backend values", () => {
  assert.equal(SCENARIO_CATEGORIES.includes("kubernetes"), true);
  assert.equal(SCENARIO_CATEGORIES.includes("aws"), true);
  assert.equal(SCENARIO_CATEGORIES.includes("kubectl"), false);
});
