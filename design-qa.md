# Aurora Bento v2 — Current Design QA

## Current acceptance state

Aurora Bento v2 has passed browser-rendered Design QA for the seven core authenticated pages across its Desktop Light, Desktop Dark, Tablet and Mobile baseline scopes. The System Settings foundation path has additionally passed hardened real-backend mutation, validation and restart QA against a production RenewAPI binary and isolated SQLite database.

Latest visual/interaction product merge: `d7fa60720457f00b4e1766f9443d0648fb6c225b`  
Latest P2-strict interaction/accessibility run: `32624800840`  
Latest hardened real-backend Settings product head: `8b56c883206a8077b3e9bf900ef9564551c93a13`  
Latest hardened real-backend Settings run: `32633291555`

Final audited values:

- P0: `0`
- P1: `0`
- P2: `0`
- fixture QA console errors: `0`
- fixture QA unhandled API requests: `0`
- Settings real-backend console errors: `0`
- Settings real-backend page errors: `0`
- Settings invalid bulk direct-SQLite atomicity: PASS
- Settings invalid bulk intermediate restart: PASS
- Settings final restart persistence: PASS
- Settings backend log drain: PASS

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

### Settings real-backend hardened mutation / validation / restart

- final run: `32633291555` (run #2)
- validated product head: `8b56c883206a8077b3e9bf900ef9564551c93a13`
- artifact: `settings-real-backend-hardened` / id `9491668465`
- artifact digest: `sha256:a7f925ed004f102a63db904545c96cb3fbf792555f7d2452401b07c783db2839`
- production default frontend tests/typecheck/build plus classic production build
- real RenewAPI Go binary and production migrations on a fresh isolated SQLite database
- no pre-seeded `theme.frontend` value
- real `/api/setup` root creation and `/api/user/login` session
- real `RootAuth` contract including `New-Api-User`
- real `/system-settings` -> `/api/option/` reads
- single-option `RetryTimes` mutation `0 -> 2 -> 0` synchronized across controlled UI state, authenticated API and direct SQLite reads
- invalid bulk server-URL validation with all three local draft fields preserved
- invalid bulk baseline verified in API and direct SQLite before any later successful overwrite
- intermediate clean process restart after the rejected bulk, with the complete baseline still present in API and UI
- valid bulk save verified through API and direct SQLite
- final clean process restart with saved values still present in API, SQLite and UI
- three backend logs drained through child close and ending in `server exited`
- browser console errors: `0`
- browser page errors: `0`

The earlier Settings run `32628206857` is retained as historical evidence but is superseded by the hardened run because the newer gate proves failed-bulk database atomicity before a later valid save can mask partial persistence.

The real-backend sequence also exposed and fixed two product issues: the authoritative `ThemeSettings.Frontend` still defaulted to `classic`, and a stale Settings success toast could coexist with a later validation error. Fresh installs now naturally select the redesigned frontend while explicit classic overrides remain supported; Settings mutations share a stable toast id so the latest result replaces the prior Settings result.

## Latest defects found and resolved

1. **375px English header overflow** — Search is now an accessible icon-sized mobile control and expands from `sm` upward.
2. **Channels icon-only menus** — page tools and row-actions triggers expose explicit accessible names.
3. **API Key controls** — copy and revealed read-only key controls expose semantic names.
4. **System Settings foundation inputs** — Gateway URL, System name and New-user quota are programmatically labelled.
5. **Decorative Recharts keyboard stops** — redundant dashboard charts are removed from Tab order/accessibility tree while textual metrics remain available.
6. **Notification glass readability** — the notification surface is more opaque locally without abandoning the Aurora glass language.
7. **Fresh-install frontend mismatch** — authoritative theme configuration defaults new installations to the redesigned frontend while preserving explicit classic overrides.
8. **Contradictory Settings mutation feedback** — the previous success toast no longer remains beside a later validation error; single/bulk Settings mutations use one stable toast id.

## Manual screenshot review

The final responsive screenshots were reviewed after the automated gate:

- 375px en-US Dashboard / Logs / Settings: no header clipping or page overflow
- Light/Dark mobile Sidebar: contained within viewport
- Light/Dark Command dialog: contained and readable
- Light/Dark Notification popover: contained; strengthened foreground/background separation
- Light/Dark desktop Quick Tools and Config Drawer: no clipping, shell collision or dock obstruction

The final hardened real-backend Settings artifact was also reviewed manually:

- validation-error state retains all three unsaved local draft fields and shows only the red backend URL validation error, with no stale success toast
- after-invalid-restart state restores the full baseline from persistent storage
- successful-save state renders the committed foundation values and success feedback
- after-success-restart state cleanly renders `RenewAPI QA Browser`, the persisted gateway URL and quota after a new process and new login session
- backend logs are fully drained and each final shutdown ends in `server exited`

No remaining actionable P0/P1/P2 was found in the audited screenshots.

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
