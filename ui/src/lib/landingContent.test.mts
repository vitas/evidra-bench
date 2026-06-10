import assert from "node:assert/strict";
import test from "node:test";

import {
  ENTERPRISE_AUDIENCES,
  HERO_CONTENT,
  LANDING_OFFERS,
} from "./landingContent.mts";

test("landing hero leads with private enterprise evaluation", () => {
  assert.match(HERO_CONTENT.title, /enterprise AI agents/i);
  assert.match(HERO_CONTENT.body, /before production/i);
  assert.equal(HERO_CONTENT.ctas[0].label, "Book private evaluation");
  assert.equal(HERO_CONTENT.ctas[0].kind, "primary");
  assert.equal(HERO_CONTENT.ctas[1].label, "View sample report");
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
      "Agent Deployment Readiness Report",
      "Vendor Selection Benchmark",
      "Release Regression Gate",
      "Custom Incident Scenario Pack",
    ],
  );
  for (const offer of LANDING_OFFERS) {
    assert.match(offer.desc, /agent|vendor|release|incident|rollout/i);
  }
});
