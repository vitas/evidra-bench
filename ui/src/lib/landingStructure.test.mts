import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const landingPath = new URL("../pages/Landing.tsx", import.meta.url);

test("landing renders the path-aware public Bench flow", async () => {
  const source = await readFile(landingPath, "utf8");

  assert.match(source, /aria-label="Primary navigation"/);
  assert.match(source, /aria-label="Safe and unsafe pass comparison"/);
  assert.match(source, /What ordinary benchmarks miss/);
  assert.match(source, /From candidate to inspectable verdict/);
  assert.match(source, /See what a pass is hiding/);
  assert.doesNotMatch(source, /Book evaluation|private-evaluation/);
  assert.doesNotMatch(source, /text-\[0\.(?:6|7|8)/);
});
