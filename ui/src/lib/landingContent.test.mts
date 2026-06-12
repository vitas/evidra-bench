import assert from "node:assert/strict";
import test from "node:test";

import {
  ENTERPRISE_AUDIENCES,
  FEATURED_REPORT_SPOTLIGHT,
  HERO_CONTENT,
  LANDING_OFFERS,
} from "./landingContent.mts";

test("landing hero leads with private enterprise evaluation", () => {
  assert.match(HERO_CONTENT.title, /enterprise AI agents/i);
  assert.match(HERO_CONTENT.body, /before production/i);
  assert.equal(HERO_CONTENT.ctas[0].label, "Book private evaluation");
  assert.equal(HERO_CONTENT.ctas[0].kind, "primary");
  assert.ok(HERO_CONTENT.proofChips.length <= 3);
  assert.equal(HERO_CONTENT.ctas[1].label, "View real report");
  assert.doesNotMatch(
    HERO_CONTENT.ctas.map((cta) => `${cta.label} ${cta.href}`).join(" "),
    /sample/i,
  );
});

test("landing spotlights benchmark scale and the largest inspectable public report", () => {
  assert.equal(
    FEATURED_REPORT_SPOTLIGHT.reportId,
    "kubernetes-mcp-readiness-2026-05-public",
  );
  assert.match(FEATURED_REPORT_SPOTLIGHT.title, /Enterprise-scale scenario coverage/i);
  assert.match(FEATURED_REPORT_SPOTLIGHT.summary, /83 scenarios/i);
  assert.match(FEATURED_REPORT_SPOTLIGHT.summary, /40\+ scenarios/i);
  assert.deepEqual(FEATURED_REPORT_SPOTLIGHT.metrics, [
    ["Scenario catalog", "83"],
    ["Broadest leaderboard slice", "62 scenarios"],
    ["Largest public report", "34 runs / 10 scenarios"],
  ]);
  assert.doesNotMatch(FEATURED_REPORT_SPOTLIGHT.to, /deepseek-v4-flash-pilot/);
  assert.ok(FEATURED_REPORT_SPOTLIGHT.strengths.length <= 3);
});

test("landing audience targets enterprise buyers and implementers", () => {
  assert.deepEqual(
    ENTERPRISE_AUDIENCES.map((audience) => audience.title),
    [
      "Enterprise AI implementation teams",
      "Systems integrators and consultancies",
      "Platform, SRE, and risk leaders",
    ],
  );
});

test("landing offers package the benchmark as purchasable evaluation products", () => {
  assert.deepEqual(
    LANDING_OFFERS.map((offer) => offer.title),
    [
      "Private rollout-risk report",
      "Vendor shortlist benchmark",
      "Release regression check",
    ],
  );
  for (const offer of LANDING_OFFERS) {
    assert.match(offer.desc, /agent|vendor|release|incident|rollout|model|MCP/i);
  }
});
