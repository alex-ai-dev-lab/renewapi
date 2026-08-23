# Aurora Bento v2 — Phase A Dark Mode Notes

This branch introduces the first independently art-directed Dark Aurora layer and a deterministic browser capture harness.

## Scope

- Dark-only Aurora color/surface tokens.
- Deep navy canvas with restrained blue/cyan/mint ambient light.
- Dark glass cards, tables, popovers, dialogs, and sheets.
- Dark primary gradient and semantic pulse treatment.
- Seven-page `1440 × 1000` Playwright capture using the same authenticated fixture as the accepted Light QA.

## Acceptance boundary

The supplied Aurora reference package contains Light source boards only. Dark validation therefore uses:

1. Light source boards for structure, geometry, density, typography hierarchy, and component placement.
2. The documented Aurora design language for Dark color/surface derivation.
3. Browser-rendered evidence for all seven core pages.
4. Runtime cleanliness: zero console errors and zero unhandled QA API requests.
5. No remaining actionable P0/P1/P2 findings before Phase A is marked complete.

The branch must not claim exact same-theme pixel parity with a nonexistent Dark source board.
