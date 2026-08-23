# Aurora Bento v2 — Current Design QA

## Current acceptance state

Aurora Bento v2 has passed browser-rendered Design QA for the seven core authenticated pages across its Desktop Light, Desktop Dark, Tablet and Mobile baseline scopes. The System Settings foundation path has additionally passed real-backend mutation and restart QA against a production RenewAPI binary and isolated SQLite database.

Latest visual/interaction product merge: `d7fa60720457f00b4e1766f9443d0648fb6c225b`  
Latest P2-strict interaction/accessibility run: `32624800840`  
Latest real-backend Settings product head: `d3d30a59a71c587464bfdb7e94a1469efb761dba`  
Latest real-backend Settings run: `32628206857`

Final audited values:

- P0: `0`
- P1: `0`
- P2: `0`
- fixture QA console errors: `0`
- fixture QA unhandled API requests: `0`
- Settings real-backend console errors: `0`
- Settings real-backend page errors: `0`
- Settings invalid bulk atomicity: PASS
- Settings restart persistence: PASS

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
- product merge: `d7fa60720457f00b4e1766f9443d0648fb6c225b`
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

### Settings real-backend mutation / restart

- final run: `32628206857`
- validated product head: `d3d30a59a71c587464bfdb7e94a1469efb761dba`
- artifact: `settings-real-backend-smoke` / id `9490331010`
- artifact digest: `sha256:074ee2048a21fa809cb07b82b7b36ba813461e8e02d96a172c4178da08436ac2`
- production default + classic frontend builds embedded in a real RenewAPI Go binary
- fresh isolated SQLite database with production migrations
- no pre-seeded `theme.frontend` value
- real `/api/setup` root creation and `/api/user/login` session
- real `RootAuth` contract including `New-Api-User`
- real `/system-settings` -> `/api/option/` reads
- real single-option `RetryTimes` mutation `0 -> 2 -> 0`
- real bulk save success
- invalid bulk server-URL validation with no partial persistence and local draft preservation
- clean process stop/restart with saved values still present in API and UI
- browser console errors: `0`
- browser page errors: `0`

The smoke also exposed and fixed a product-default mismatch: the authoritative `ThemeSettings.Frontend` still defaulted to `classic`, which overrode lower-level attempts to make Aurora the default. It now defaults to `default`; a regression test confirms fresh-default synchronization and a second test confirms an explicitly persisted `classic` override remains supported.

## Latest defects found and resolved

1. **375px English header overflow** — Search is now an accessible icon-sized mobile control and expands from `sm` upward.
2. **Channels icon-only menus** — page tools and row-actions triggers expose explicit accessible names.
3. **API Key controls** — copy and revealed read-only key controls expose semantic names.
4. **System Settings foundation inputs** — Gateway URL, System name and New-user quota are programmatically labelled.
5. **Decorative Recharts keyboard stops** — redundant dashboard charts are removed from Tab order/accessibility tree while textual metrics remain available.
6. **Notification glass readability** — the notification surface is more opaque locally without abandoning the Aurora glass language.
7. **Fresh-install frontend mismatch** — authoritative theme configuration now defaults new installations to the redesigned frontend while preserving explicit classic overrides.

## Manual screenshot review

The final responsive screenshots were reviewed after the automated gate:

- 375px en-US Dashboard / Logs / Settings: no header clipping or page overflow
- Light/Dark mobile Sidebar: contained within viewport
- Light/Dark Command dialog: contained and readable
- Light/Dark Notification popover: contained; strengthened foreground/background separation
- Light/Dark desktop Quick Tools and Config Drawer: no clipping, shell collision or dock obstruction

The final real-backend Settings artifact was also reviewed manually:

- validation-error state retains the unsaved local draft and visibly surfaces the backend URL validation error
- successful-save state renders the committed foundation values
- post-restart state cleanly renders `RenewAPI QA Browser`, the persisted gateway URL and quota after a new process and new login session
- backend logs confirm the default `/static/js/...` application, real option API traffic and the restart read path

No new actionable P0/P1/P2 was found in the audited screenshots.

## Accepted limitations / remaining work

- The source package contains Light desktop boards only; no tablet/mobile or Dark board exists, so those scopes do not claim invented pixel parity.
- The automated `720px` check is a CSS-viewport proxy for 200% zoom, not literal browser UI zoom.
- A real assistive-technology / screen-reader session remains pending.
- Exhaustive contrast scanning for every transient state remains pending; Dark semantic foreground sampling already passed representative WCAG contrast checks.
- Loading, empty, destructive, non-Settings bulk-action, filter/pagination and model-management states are not all exhaustively screenshot-audited.
- Settings real-backend QA covers the foundation landing mutation path, validation atomicity and restart persistence; advanced Settings sections are not claimed as exhaustively mutation-tested.
- Secondary Surfaces brand unification and deployment smoke remain separate pending phases.

For the latest detailed boundaries, see:

- `docs/aurora-interaction-accessibility-qa.md`
- `docs/aurora-settings-real-backend-qa.md`
- `docs/aurora-bento-v2-progress.md`

final result: passed for audited scope
