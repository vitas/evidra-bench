import assert from "node:assert/strict";
import test from "node:test";

import { selectAvailableModels } from "./modelData.mts";

test("selectAvailableModels falls back to bundled models when API returns none", () => {
  const models = selectAvailableModels([]);

  assert.equal(models.length > 0, true);
  assert.equal(models[0]?.id, "gemini-2.5-flash");
  assert.equal(models[0]?.display_name, "Gemini 2.5 Flash");
});

test("selectAvailableModels prefers API-provided models when available", () => {
  const models = selectAvailableModels([
    {
      id: "private-model",
      display_name: "Private Model",
      provider: "custom",
      available: true,
      input_cost_per_mtok: 1,
      output_cost_per_mtok: 2,
    },
  ]);

  assert.deepEqual(models, [
    {
      id: "private-model",
      display_name: "Private Model",
      provider: "custom",
      available: true,
      input_cost_per_mtok: 1,
      output_cost_per_mtok: 2,
    },
  ]);
});
