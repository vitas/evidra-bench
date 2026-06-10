import assert from "node:assert/strict";
import { readFile } from "node:fs/promises";
import test from "node:test";

const indexHtmlPath = new URL("../../index.html", import.meta.url);

async function readIndexHtml() {
  return readFile(indexHtmlPath, "utf8");
}

test("landing metadata targets enterprise AI agent evaluation", async () => {
  const html = await readIndexHtml();

  assert.match(html, /<title>Enterprise AI Agent Evaluation - Evidra Bench<\/title>/);
  assert.match(html, /AI agent deployment readiness/i);
  assert.match(html, /private evaluation reports/i);
});

test("landing structured data exposes the private evaluation service", async () => {
  const html = await readIndexHtml();
  const jsonLdMatch = html.match(/<script type="application\/ld\+json">\s*([\s\S]*?)\s*<\/script>/);
  assert.ok(jsonLdMatch, "missing JSON-LD script");

  const structuredData = JSON.parse(jsonLdMatch[1]);
  const graph = structuredData["@graph"];
  assert.ok(Array.isArray(graph), "JSON-LD should use @graph");

  const service = graph.find((entry: { "@type"?: string; name?: string }) => entry["@type"] === "Service");
  assert.ok(service, "missing Service entry");
  assert.match(service.name, /Private AI Agent Evaluation/);
  assert.match(service.description, /deployment readiness/i);
});
