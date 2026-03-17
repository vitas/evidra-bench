import assert from "node:assert/strict";
import test from "node:test";

import { resolveRunsLimit } from "./benchmarkData.mts";

test("resolveRunsLimit requests all available rows when totals are known", () => {
  assert.equal(resolveRunsLimit(237), 237);
});

test("resolveRunsLimit never asks the API for zero rows", () => {
  assert.equal(resolveRunsLimit(0), 1);
});
