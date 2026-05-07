import assert from "node:assert/strict";
import test from "node:test";

import {
  buildBenchCommand,
  buildRunCommand,
  EVIDENCE_MODES,
} from "./commandBuilder.mts";

test("baseline bench command does not force proxy or smart-prescribe flags", () => {
  const command = buildBenchCommand({
    scenarios: ["kubernetes/broken-deployment"],
    model: "gpt-4o",
    evidenceMode: "baseline",
  });

  assert.match(command, /^bench-cli bench/m);
  assert.equal(command.includes("--proxy-mode"), false);
  assert.equal(command.includes("--smart-prescribe"), false);
  assert.equal(command.includes("--mcp-server"), false);
  assert.equal(command.includes("--evidra-url"), false);
  assert.equal(command.includes("--bench-url"), true);
});

test("evidra-mcp bench command uses the MCP server path", () => {
  const command = buildBenchCommand({
    scenarios: ["kubernetes/broken-deployment"],
    model: "gpt-4o",
    evidenceMode: "evidra-mcp",
  });

  assert.match(command, /--mcp-server "evidra-mcp --signing-mode optional"/);
  assert.equal(command.includes("--smart-prescribe"), false);
});

test("designer run command uses the selected evidence mode semantics", () => {
  const baseline = buildRunCommand({
    scenario: "./my-puzzle",
    model: "gpt-4o",
    evidenceMode: "baseline",
  });
  const viaEvidra = buildRunCommand({
    scenario: "./my-puzzle",
    model: "gpt-4o",
    evidenceMode: "evidra-mcp",
  });

  assert.equal(baseline.includes("--proxy-mode"), false);
  assert.equal(viaEvidra.includes("--mcp-server"), true);
});

test("evidence modes expose the UI labels for the supported command paths", () => {
  assert.deepEqual(EVIDENCE_MODES.map((mode) => mode.id), ["baseline", "evidra-mcp"]);
});
