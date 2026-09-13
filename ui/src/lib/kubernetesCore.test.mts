import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import { readFileSync } from "node:fs";
import test from "node:test";

import {
  ADMISSION_CONTRACT,
  CONTROL_MATRIX,
  CORE_CASES,
  CORE_PACK_FACTS,
  CORE_PACK_HERO,
  CORE_PACK_LIMIT,
  DROPPED_CASES,
  WHY_FEWER,
} from "./kubernetesCore.mts";

const pagePath = new URL("../pages/KubernetesCore.tsx", import.meta.url);

test("the core pack admits twelve distinct cases", () => {
  assert.equal(CORE_CASES.length, 12);

  const ids = CORE_CASES.map((item) => item.id);
  assert.equal(new Set(ids).size, ids.length, "case ids must be unique");

  for (const item of CORE_CASES) {
    assert.match(item.level, /^L[1-4]$/);
    assert.ok(item.catches.length > 30, `${item.id} needs a concrete failure it catches`);
  }
});

test("the pack states its own numbers", () => {
  assert.deepEqual(
    CORE_PACK_FACTS.map((fact) => fact.value),
    ["12", "3", "2", "3"],
  );
  assert.match(CORE_PACK_FACTS[0].label, /67 triaged/);
  assert.match(CORE_PACK_HERO.title, /Twelve cases/);
  assert.match(CORE_PACK_LIMIT, /not a production certification/);
});

test("the control matrix rejects an unsafe repair that restores the final state", () => {
  assert.equal(CONTROL_MATRIX.rows.length, 3);

  const [safe, noop, unsafe] = CONTROL_MATRIX.rows;
  assert.deepEqual([safe.repair, safe.falseAlarm], ["PASS", "PASS"]);
  assert.deepEqual([noop.repair, noop.falseAlarm], ["FAIL", "PASS"]);
  assert.deepEqual([unsafe.repair, unsafe.falseAlarm], ["UNSAFE", "UNSAFE"]);
  assert.match(unsafe.control, /restores the final state/);
});

test("the admission contract covers authority, verification, and incompleteness", () => {
  assert.ok(ADMISSION_CONTRACT.length >= 6);
  assert.ok(
    ADMISSION_CONTRACT.every((item) => item.length < 140),
    "contract lines stay short enough to scan",
  );

  const contract = ADMISSION_CONTRACT.join("\n");
  assert.match(contract, /on_denied: unsafe/);
  assert.match(contract, /command-succeeds checks are rejected/);
  assert.match(contract, /three deterministic runs on kind and k3d/);
  assert.match(contract, /no pod exec/);
  assert.match(contract, /INCOMPLETE/);
});

test("the three principles stay short enough to read", () => {
  assert.equal(WHY_FEWER.length, 3);
  for (const item of WHY_FEWER) {
    assert.ok(item.body.length < 260, `${item.title} is too long for a landing section`);
  }
});

test("dropped cases name the verifier failure that removed them", () => {
  assert.ok(DROPPED_CASES.length >= 3);

  const ids = DROPPED_CASES.map((item) => item.id);
  assert.equal(new Set(ids).size, ids.length, "dropped case ids must be unique");

  for (const item of DROPPED_CASES) {
    assert.ok(item.reason.length > 40, `${item.id} needs a substantive reason`);
    assert.ok(item.reason.length < 160, `${item.id} reason stays one line`);
  }

  const reasons = DROPPED_CASES.map((item) => item.reason).join("\n");
  assert.match(reasons, /fabricating credential material/);
  assert.match(reasons, /failurePolicy: Ignore/);
});

test("the pack page keeps the rigor argument and stays lean", async () => {
  const source = await readFile(pagePath, "utf8");

  assert.match(source, /aria-label="Pack facts"/);
  assert.match(source, /Why fewer is harder/);
  assert.match(source, /What a case must prove before it counts/);
  assert.match(source, /What we dropped/);
  assert.match(source, /Twelve cases, twelve ways to be wrong/);
  assert.match(source, /kubernetes-core@1/);
  assert.match(source, /CORE_PACK_LIMIT/);

  assert.doesNotMatch(source, /Book evaluation|private-evaluation/);
  assert.doesNotMatch(source, /VERDICT_CLASSES|EVIDENCE_QUALIFICATION/, "verdict essays stay off the landing page");
});

test("the shared public chrome owns navigation and the footer", () => {
  const chrome = readFileSync(new URL("../components/PublicChrome.tsx", import.meta.url), "utf8");

  assert.match(chrome, /aria-label="Primary navigation"/);
  assert.match(chrome, /aria-label="Footer navigation"/);
  assert.match(chrome, /Open public Bench/);
});

test("the public report page renders run evidence, not marketing copy", async () => {
  const source = await readFile(new URL("../pages/KubernetesCoreReport.tsx", import.meta.url), "utf8");

  assert.match(source, /CORE_REPORT/);
  assert.match(source, /How to read this report/);
  assert.match(source, /Audit events/);
  assert.match(source, /artifactDir/);
  assert.match(source, /not a production certification/);
  assert.doesNotMatch(source, /Book evaluation|private-evaluation/);
});
