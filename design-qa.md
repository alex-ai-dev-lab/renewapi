# Aurora Bento v2 — Dark Mode Design QA

## Source truth and capture state

- Source visual truth: supplied Aurora Bento v2 reference restored from immutable commit `3d91c3fd22fbfae284d48563d5a09ac046a81cd2`.
- Important limitation: the supplied design package contains Light boards only. Dark is therefore an independently art-directed Aurora derivation; this report does **not** claim nonexistent same-theme Dark pixel parity.
- Final implementation evidence: GitHub Actions `Aurora dark visual QA` run `32618534536` (run #8), artifact `aurora-dark-visual-qa`, artifact id `9487693376`.
- Core routes: Dashboard / Channels / API Keys / Common Logs / Models / Users / System Settings.
- CSS viewport: `1440 × 1000`.
- Source screenshot pixels: `1440 × 1000`.
- Implementation screenshot pixels: `1440 × 1000`.
- deviceScaleFactor: `1`.
- locale: `zh-CN`.
- timezone: `Asia/Taipei`.
- state: deterministic authenticated Super Admin fixture; implementation theme `dark`; source theme `light` for structural reference only.
- final runtime: console errors `0`; unhandled QA API requests `0`.

## Full-view comparison evidence

Run #8 captured source and implementation pairs for all seven core pages. Each pair was reviewed in the same-coordinate side-by-side composite with source on the left and Dark implementation on the right.

| Page | Source artifact path | Implementation artifact path | Final result |
|---|---|---|---|
| Dashboard | `source/dashboard.png` | `implementation/dashboard.png` | PASS |
| Channels | `source/channels.png` | `implementation/channels.png` | PASS |
| API Keys | `source/keys.png` | `implementation/keys.png` | PASS |
| Common Logs | `source/logs.png` | `implementation/logs.png` | PASS |
| Models | `source/models.png` | `implementation/models.png` | PASS |
| Users | `source/users.png` | `implementation/users.png` | PASS |
| System Settings | `source/settings.png` | `implementation/settings.png` | PASS |

The Dark implementation preserves the accepted Light geometry: Aurora topbar, shrink-to-content floating dock, hero rhythm, 12-column Bento relationships, compact model registry, settings 6/6 + 12 layout, and unified production data panels. Dark changes are limited to intentional theme treatment: deep navy canvas, restrained cyan/blue/mint ambient light, dark glass depth, active-state gradients, and dark semantic surfaces.

## Focused region comparison evidence

### Models registry

The final capture preserves the accepted six-card density and restores intentional per-card tone separation. Blue, warm, and green/cyan families remain subtle on the dark glass instead of collapsing into six identical navy cards. Registry height, card padding, metadata hierarchy, and floating dock position remain aligned with the structural source.

### System Settings

Checked switches use a visible blue→cyan active track with a high-contrast light thumb. The 6/6 strategy panels and 12-column foundation panel keep the accepted density, card boundaries, input rhythm, and bottom-dock geometry. Disabled/off state remains visually distinct without relying on color alone.

### API Keys / production data panels

Filters, actions, and the real production table remain functional, but the chrome reads as one glass data panel rather than a stacked legacy admin toolbar plus table. Header density, sticky behavior, row spacing, and subdued separators are preserved in Dark.

### App shell

The Sidebar provider/inset no longer paints over the ambient canvas. The topbar and dock sit over the deep navy Aurora background; the dock remains content-width rather than stretching toward full width.

## Findings

### Final actionable findings

- P0: `0`
- P1: `0`
- P2: `0`

### Accepted P3 / documented limitations

- [P3] No Dark source board exists in the supplied reference package.
  - Impact: Dark color/surface fidelity is evaluated against the established Aurora design language, structural Light boards, accessibility contrast, and browser-rendered consistency—not against an invented Dark source image.
  - Disposition: accepted and explicitly documented. If an authoritative Dark board is supplied later, run a same-theme pixel audit.
- [P3] Production data, filters, actions, model names, counts, pricing, and option-backed settings intentionally differ from static source placeholders.
  - Impact: content-level differences do not change the accepted geometry or visual hierarchy.
  - Disposition: retained to avoid fabricating product data or deleting production capability for screenshot similarity.

## Accessibility color sampling

Representative semantic colors were sampled against the defined dark-glass base (approximately `#0e192c`):

| Foreground | Approx. contrast ratio |
|---|---:|
| primary text `#f8fbff` | `16.95:1` |
| Aurora blue `#78a7ff` | `7.34:1` |
| Aurora cyan `#55d9ff` | `10.68:1` |
| success `#5ee0ae` | `10.68:1` |
| warning `#ffc56a` | `11.26:1` |
| danger `#ff8c9a` | `7.93:1` |

These samples exceed the `4.5:1` normal-text threshold for the explicit semantic foregrounds. Full keyboard/screen-reader/zoom coverage remains a later accessibility phase.

## Comparison history

### Iteration 0 — legacy Dark baseline

- Finding: Dark relied on the generic legacy theme, with no independent Aurora art direction or dedicated browser QA.
- Fix: introduced `aurora-dark-reference.css`, wired it into the authenticated shell, and restored the deterministic seven-page capture harness.

### Iteration 1 — run `32617884749`

- Findings: dock stretched too wide; Sidebar surface hid the ambient canvas; Settings checked switches lost Aurora active color; Keys/Logs/Users fell back to two-layer legacy table chrome; Models was too tall.
- Fixes: exposed the Dark canvas, restored compact dock geometry, added Dark switch states, mirrored the accepted unified data-panel rules, and restored Model Registry density.
- Runtime: clean.

### Iteration 2 — run `32618050936`

- Result: the major shell/table/settings/geometry issues were fixed.
- Remaining P2: an `!important` Dark glass rule flattened the Models six-card tone families into one uniform navy surface.
- Fix: made `.glass-tile` overridable by intentional per-card inline tone gradients.

### Iteration 3 — run `32618241427`

- Result: all seven full views visually passed; Models tone families restored; no remaining actionable P0/P1/P2.
- Follow-up: strengthened the engineering gate before marking Phase A complete.

### Engineering gate hardening

- Prettier canonical formatting, repository ESLint, and repository copyright enforcement were added to the Dark QA workflow.
- Intermediate runs intentionally exposed and fixed canonical import ordering, CSS formatting, and the exact repository license-header boundary.

### Final — run `32618534536`

- Prettier / git-diff cleanliness: PASS.
- ESLint: PASS.
- repository copyright check: PASS.
- `bun test`: PASS.
- `bun run typecheck`: PASS.
- `bun run build`: PASS.
- seven-page Dark Playwright recapture: PASS.
- console errors: `0`.
- unhandled QA API requests: `0`.
- final visual spot-check (Dashboard / Models / Settings plus full contact sheet): no regressions.

## Implementation Checklist

- [x] Add independent Dark Aurora tokens and ambient canvas.
- [x] Wire Dark reference layer into the authenticated product shell.
- [x] Preserve accepted Light structural geometry in Dark.
- [x] Restore the proven deterministic fixture and switch implementation to Dark.
- [x] Capture all seven core pages at `1440 × 1000` / dSF `1`.
- [x] Compare full views source+implementation together.
- [x] Compare focused shell, data-table, Models, and Settings regions.
- [x] Resolve all actionable P0/P1/P2 Dark findings.
- [x] Verify representative semantic contrast.
- [x] Verify console errors = `0` and unhandled QA API requests = `0`.
- [x] Pass Prettier / ESLint / copyright / tests / typecheck / build.
- [x] Update the Aurora progress tracker after evidence is complete.

## Follow-up Polish

- Perform exact same-theme Dark pixel comparison if an authoritative Dark source board is supplied.
- Continue with Phase B: `1024px`, `768px`, and `375px` responsive QA.
- Continue with the dedicated interaction/accessibility phases for keyboard, screen reader, 200% zoom, reduced motion, and non-default states.

final result: passed
