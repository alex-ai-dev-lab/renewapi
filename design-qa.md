# Aurora Bento v2 — Current Design QA

## Current acceptance state

Aurora Bento v2 has passed browser-rendered Design QA for the seven core authenticated pages across its Desktop Light, Desktop Dark, Tablet and Mobile baseline scopes.

Latest audited head: `38169318c1d8e8815ddb1084baf8fd12b9291da1`  
Latest P2-strict interaction/accessibility run: `32624800840`  
Latest artifact: `aurora-interaction-a11y-qa` / id `9489423724`

Final latest-run values:

- P0: `0`
- P1: `0`
- P2: `0`
- console errors: `0`
- unhandled QA API requests: `0`

## Evidence chain

### Desktop Light

- final visual run: `32614926945`
- final quality run: `32615101043`
- product commit: `9f90bd234d1533dc4f4efb916cc826d5626d4737`
- seven pages at `1440×1000`
- same-theme supplied Light reference comparison
- actionable visual findings after final pass: `0`

### Desktop Dark

- final Dark run: `32618534536`
- product merge: `cf4961baf0a553d86b4c3f0ad90fd031f6c64633`
- seven pages at `1440×1000`
- Dark is an independently art-directed Aurora derivation because the supplied package contains no authoritative Dark board
- actionable visual findings after final pass: `0`

### Responsive Phase B baseline

- baseline run: `32619390956`
- responsive product merge: `02e467d95a36d0b7d54ea55f3db305205eadc857`
- widths: `1024×1000`, `768×1000`, `375×812`
- themes: Light + Dark
- seven core pages
- page-level horizontal overflow: `0`

### Responsive / interaction / accessibility hardening

- final run: `32624800840`
- widths: `1024`, `768`, `375`, plus `720px` responsive proxy for 200% zoom
- themes: Light + Dark
- zh-CN plus en-US long-string smoke
- mobile Sidebar / Command / Notifications open states
- desktop Quick Tools / Config Drawer open states
- sampled keyboard Tab order and focus-visible indicators
- visible-control accessible-name checks
- WCAG 2.2 AA 24px persistent mobile-header target floor
- reduced-motion emulation
- final severity gate: any P0/P1/P2 fails the workflow

Final artifact result: `issues=[]`.

## Latest defects found and resolved

1. **375px English header overflow** — Search is now an accessible icon-sized mobile control and expands from `sm` upward.
2. **Channels icon-only menus** — page tools and row-actions triggers expose explicit accessible names.
3. **API Key controls** — copy and revealed read-only key controls expose semantic names.
4. **System Settings foundation inputs** — Gateway URL, System name and New-user quota are programmatically labelled.
5. **Decorative Recharts keyboard stops** — redundant dashboard charts are removed from Tab order/accessibility tree while textual metrics remain available.
6. **Notification glass readability** — the notification surface is more opaque locally without abandoning the Aurora glass language.

## Manual screenshot review

The final mobile screenshots were reviewed after the automated gate:

- 375px en-US Dashboard / Logs / Settings: no header clipping or page overflow
- Light/Dark mobile Sidebar: contained within viewport
- Light/Dark Command dialog: contained and readable
- Light/Dark Notification popover: contained; strengthened foreground/background separation
- Light/Dark desktop Quick Tools and Config Drawer: no clipping, shell collision or dock obstruction

No new actionable P0/P1/P2 was found in manual review.

## Accepted limitations / remaining work

- The source package contains Light desktop boards only; no tablet/mobile or Dark board exists, so those scopes do not claim invented pixel parity.
- The automated `720px` check is a CSS-viewport proxy for 200% zoom, not literal browser UI zoom.
- A real assistive-technology / screen-reader session remains pending.
- Exhaustive contrast scanning for every transient state remains pending; Dark semantic foreground sampling already passed representative WCAG contrast checks.
- Loading, empty, error, destructive, bulk-action, filter/pagination and model-management states are not all exhaustively screenshot-audited.
- Settings real-backend mutation success/error/rollback and deployment smoke remain separate pending phases.

For the full latest evidence and boundary, see `docs/aurora-interaction-accessibility-qa.md`.

final result: passed for audited scope
