# Landing Bench CTA Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Add a `Bench` hero CTA to the landing page that links to the public benchmark site.

**Architecture:** Keep the existing hero CTA structure intact and append one more secondary outline link. Cover the behavior with a UI test that verifies the CTA label and destination.

**Tech Stack:** React, React Router, Testing Library, Vitest

---

### Task 1: Add coverage for the hero Bench CTA

**Files:**
- Modify: `ui/test/components/App.test.tsx`
- Modify: `ui/src/pages/Landing.tsx`

**Step 1: Write the failing test**

Add a test that renders `<App />`, finds the `Bench` link, and expects its `href` to be `https://bench.evidra.cc`.

**Step 2: Run test to verify it fails**

Run: `cd ui && npm test -- App.test.tsx`

Expected: FAIL because the hero row does not yet include a `Bench` CTA.

**Step 3: Write minimal implementation**

Append a `Bench` anchor to the hero CTA row in `ui/src/pages/Landing.tsx` and reuse the existing secondary CTA classes.

**Step 4: Run test to verify it passes**

Run: `cd ui && npm test -- App.test.tsx`

Expected: PASS.

**Step 5: Verify broader impact**

Run:
- `cd ui && npm test`
- `cd ui && npm run build`
- `make lint`

Expected: all pass.
