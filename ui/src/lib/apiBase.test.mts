import assert from "node:assert/strict";
import test from "node:test";

import { buildBenchApiURL } from "./apiBase.mts";

test("buildBenchApiURL prefixes API base for absolute API paths", () => {
  assert.equal(
    buildBenchApiURL("https://api.evidra.cc", "/v1/bench/runs/run-1/transcript"),
    "https://api.evidra.cc/v1/bench/runs/run-1/transcript",
  );
});

test("buildBenchApiURL keeps relative paths relative when API base is empty", () => {
  assert.equal(buildBenchApiURL("", "/v1/bench/info"), "/v1/bench/info");
});
