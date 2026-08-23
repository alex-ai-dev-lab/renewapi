# Aurora Bento v2 — Phase D Accessibility Audit Plan

> Scope branch: `qa/aurora-accessibility-phase-d`  
> Base product: Phase C merge `57c4145fdb66c4cb912c5dcd9fb587713282e59c`

## Goal

Close the remaining automation-friendly accessibility gaps without overstating what a Linux headless browser can prove.

## Automated evidence in this pass

- WCAG A/AA axe scan on every captured Phase C deep-state surface.
- Light/Dark baseline scan for all seven core pages.
- High-DPI 200% layout equivalence: `720×500` CSS viewport at device scale factor `2`, representing a 1440-device-pixel-wide browser surface at 200% CSS scaling.
- Horizontal-overflow and overlay containment checks on the high-DPI matrix.
- Playwright ARIA snapshots for each core baseline and high-DPI page.
- Deterministic console/page-error/unhandled-fixture gates.
- Existing frontend tests, typecheck and production build.

Any axe WCAG violation is actionable and fails the P2-strict gate. Axe `color-contrast` nodes that cannot be resolved automatically (for example because of layered/gradient backgrounds) are retained as P3 manual-review evidence rather than silently discarded.

## Explicit non-claim

This pass does **not** call headless Chromium an actual VoiceOver/NVDA session, and it does **not** call viewport/DPR emulation literal browser-chrome zoom. Real desktop screen-reader smoke and literal browser UI zoom remain separate environment-dependent acceptance items unless a suitable runner becomes available.
