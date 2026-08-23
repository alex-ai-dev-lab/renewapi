# Aurora Bento v2 — Phase A Dark Mode Notes

Phase A introduces an independently art-directed Dark Aurora layer plus a deterministic seven-page browser QA gate.

## Scope completed

- Dark-only Aurora color/surface tokens.
- Deep navy canvas with restrained blue/cyan/mint ambient light.
- Transparent desktop shell surfaces so the ambient canvas remains visible.
- Dark glass cards, data panels, popovers, dialogs, and sheets.
- Blue→cyan primary treatment and semantic success/warning/danger colors.
- Dark-specific Settings checked states and compact floating Dock.
- Accepted Light structural geometry mirrored into Dark for data-table density and Model Registry layout.
- Intentional Models card tone families preserved in Dark.
- Seven-page `1440 × 1000` Playwright capture using the same authenticated fixture as accepted Light QA.

## Acceptance boundary

The supplied Aurora reference package contains Light source boards only. Dark validation therefore uses:

1. Light source boards for structure, geometry, density, typography hierarchy, and component placement.
2. The established Aurora design language for Dark color/surface derivation.
3. Browser-rendered evidence for Dashboard, Channels, API Keys, Common Logs, Models, Users, and System Settings.
4. Representative semantic contrast sampling against the Dark glass base.
5. Runtime cleanliness: zero console errors and zero unhandled QA API requests.
6. No remaining actionable P0/P1/P2 findings.

This Phase does **not** claim exact same-theme pixel parity with a nonexistent Dark source board.

## Final evidence

Final GitHub Actions run: `32618534536` (`Aurora dark visual QA`, run #8).

Final artifact: `aurora-dark-visual-qa`, artifact id `9487693376`.

The final run passed:

- repository Prettier + clean diff
- ESLint
- repository copyright check
- `bun test`
- `bun run typecheck`
- `bun run build`
- seven-page Dark Playwright capture
- browser console errors = `0`
- unhandled QA API requests = `0`

Visual iteration history and focused comparison details are recorded in the repository-root `design-qa.md`.

## Result

Phase A Dark Mode browser Design QA: **PASS**.

Next: Phase B Tablet / Mobile (`1024px`, `768px`, `375px`).
