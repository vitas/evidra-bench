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

test("mcp bench command uses the generic MCP server placeholder", () => {
  const command = buildBenchCommand({
    scenarios: ["kubernetes/broken-deployment"],
    model: "gpt-4o",
    evidenceMode: "mcp",
  });

  assert.match(command, /--mcp-server "\$MCP_SERVER"/);
  assert.equal(command.includes("--smart-prescribe"), false);
});

test("designer run command uses the selected evidence mode semantics", () => {
  const baseline = buildRunCommand({
    scenario: "./my-puzzle",
    model: "gpt-4o",
    evidenceMode: "baseline",
  });
  const viaMCP = buildRunCommand({
    scenario: "./my-puzzle",
    model: "gpt-4o",
    evidenceMode: "mcp",
  });

  assert.equal(baseline.includes("--proxy-mode"), false);
  assert.equal(viaMCP.includes("--mcp-server"), true);
});

test("evidence modes expose the UI labels for the supported command paths", () => {
  assert.deepEqual(EVIDENCE_MODES.map((mode) => mode.id), ["baseline", "mcp"]);
});
