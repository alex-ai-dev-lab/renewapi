# Aurora Bento v2 Phase C/D QA Plan

This temporary QA branch validates the already merged Aurora desktop/responsive product across interaction and accessibility-sensitive states before any Phase C/D completion claim.

## Browser matrix

- Light + Dark
- 1440x1000 desktop shell interactions
- 1024x1000 tablet
- 768x1000 compact tablet
- 375x812 mobile
- 720x500 responsive proxy for a 1440px viewport at 200% browser zoom
- en-US responsive smoke for longer translated strings
- prefers-reduced-motion: reduce

## Hard checks

- no page-level horizontal overflow
- oversized tables must live inside an x-scroll container
- visible interactive controls have accessible names
- persistent mobile header targets are at least the WCAG 2.2 AA 24px floor
- keyboard Tab order reaches visible named controls
- mobile sidebar and command dialog remain inside the viewport
- desktop configuration drawer remains inside the viewport
- console errors = 0
- unhandled deterministic fixture API requests = 0

## Evidence

The workflow uploads screenshots, `issues.json`, `observations.json`, request/runtime logs and a compact summary. Findings are not marked PASS until the artifact is reviewed. The workflow itself is temporary and will be removed before final merge so `main` keeps only durable product changes and QA documentation.
