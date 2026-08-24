import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const globalStylesPath = new URL("../styles/global.css", import.meta.url);

test("landing theme exposes semantic safe-state colors in both themes", async () => {
  const styles = await readFile(globalStylesPath, "utf8");

  assert.match(styles, /--color-success: #15803d;/);
  assert.match(styles, /--color-success-tint: #f0fdf4;/);
  assert.match(styles, /--color-success: #4ade80;/);
  assert.match(styles, /--color-success-tint: rgba\(74,222,128,0\.10\);/);
});
