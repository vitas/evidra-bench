# Landing Bench CTA Design

**Date:** 2026-03-17

**Goal:** Add a `Bench` call-to-action button to the landing-page hero row so benchmark access is visible alongside the existing onboarding and docs actions.

## Context

The landing hero currently exposes three actions:
- `Quick Start`
- `Get API Key`
- `API Docs`

The benchmark already exists elsewhere in the UI (`Bench` nav/footer links), but not in the hero CTA cluster.

## Design

- Add a fourth hero CTA labeled `Bench`.
- Link it to `https://bench.evidra.cc`.
- Style it like the existing secondary outline CTAs so `Quick Start` remains the primary action.

## Testing

- Add a landing-page test that asserts a `Bench` link is rendered.
- Verify the link points to `https://bench.evidra.cc`.
