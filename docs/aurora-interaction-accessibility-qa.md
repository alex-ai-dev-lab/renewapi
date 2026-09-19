# Aurora Bento v2 — Responsive / Interaction / Accessibility QA

> Recorded: 2026-08-23 +08:00  
> Repository: `alex-ai-dev-lab/renewapi`  
> Final audited product head: `38169318c1d8e8815ddb1084baf8fd12b9291da1`  
> Final P2-strict workflow: `32624800840` (run #18)  
> Artifact: `aurora-interaction-a11y-qa` / id `9489423724`

## Result

**PASS for the audited responsive, overlay, keyboard-sampling and semantic-control scope.**

Final artifact values:

- issues: `0`
- console errors: `0`
- unhandled deterministic QA API requests: `0`
- final severity gate: any P0, P1 **or P2** fails the workflow

The temporary QA workflow and harness were removed before merge. Only durable product fixes and this evidence remain.

## Audited matrix

- themes: Light + Dark
- core pages: Dashboard, Channels, API Keys, Usage Logs, Models, Users, System Settings
- responsive widths: `1024×1000`, `768×1000`, `375×812`
- responsive proxy for 200% zoom: `720×500`
- locale smoke: `zh-CN`; additional long-string responsive smoke in `en-US`
- deviceScaleFactor: `1`
- timezone: `Asia/Taipei`
- reduced-motion emulation: `prefers-reduced-motion: reduce`

## Interaction states audited

- mobile Sidebar open state
- mobile Command/Search dialog open state
- mobile Notification popover open state
- desktop Quick Tools popover open state
- desktop Config Drawer open state
- sampled keyboard Tab order and focus indicators
- persistent mobile-header target sizing
- table horizontal-scroll containment
- page-level horizontal overflow
- visible-control accessible names

All audited overlays remained inside the viewport in both Light and Dark.

## Product defects found and fixed

### 1. 375px English header overflow

The full textual Search control consumed too much horizontal space in the mobile header and pushed the profile avatar outside the viewport.

Fix: the Search control now becomes an icon-sized accessible button below `sm`, while keeping its full text form from `sm` upward.

### 2. Channels menu accessible names

Both the row-actions menu trigger and the Channels page tools menu had icon-only states that could be exposed without a reliable accessible name.

Fix: explicit translated `aria-label` values were added to the actual trigger buttons.

### 3. API Key copy and revealed-key controls

The per-row copy button depended on tooltip text, and the revealed read-only key input lacked an explicit accessible name.

Fix: both controls now expose semantic `aria-label` values.

### 4. System Settings foundation inputs

Gateway base URL, System name and New-user quota were visually labelled by adjacent copy but were not programmatically named.

Fix: each input now receives the same translated label via `aria-label`.

### 5. Decorative Recharts keyboard stops

Recharts 3 enables its accessibility layer by default. The dashboard decorative charts therefore introduced unnamed SVG keyboard stops even though the same metrics already exist as nearby text.

Fix: redundant chart graphics are hidden from the accessibility tree; the chart accessibility layer is disabled for those decorative instances, and the Pie root is explicitly removed from Tab order. Empty-state text remains accessible when no trend data is present.

### 6. Notification glass readability

The notification popover used a glass surface that allowed too much underlying dashboard detail to compete with its content.

Fix: the notification surface locally strengthens the `--aurora-glass-strong` background while preserving the Aurora glass treatment.

## Responsive acceptance

The earlier Phase B baseline run `32619390956` established the clean 1024 / 768 / 375 seven-page matrix. This follow-up closes the deeper responsive checks that baseline intentionally left open:

- mobile navigation behavior: PASS
- Bento collapse / core-page layout at 1024, 768 and 375: PASS
- dense-table containment strategy: PASS
- Drawer / Dialog / Popover viewport safety: PASS for audited global overlays
- long English copy / header wrapping smoke: PASS
- page-level horizontal overflow: `0`

No dedicated tablet/mobile source boards exist in the supplied Aurora package, so this does not claim invented pixel-level parity on those widths.

## Accessibility acceptance boundary

Validated in this pass:

- visible controls have accessible names in the audited matrix
- sampled keyboard focus reaches visible named controls
- sampled focus-visible styling is present
- persistent mobile-header targets meet the WCAG 2.2 AA `24px` minimum floor
- `prefers-reduced-motion: reduce` is detected and the tested layouts remain stable
- `720px` responsive proxy for a `1440px` viewport at 200% zoom has no page-level overflow

Still outside this automated evidence and therefore **not claimed complete**:

- real assistive-technology / screen-reader session
- literal browser UI zoom at 200% (the automated run uses an equivalent CSS-viewport proxy)
- exhaustive full-page contrast scanner across every transient state
- every loading / empty / error / destructive / bulk-action state
- real-backend Settings mutation success/error/rollback smoke

## Engineering gate

Final run #18 passed before artifact capture:

- `bun install --frozen-lockfile`
- `bun test`
- `bun run typecheck`
- `bun run build`
- deterministic Playwright interaction/accessibility harness

The product changes are limited to responsive/a11y hardening and notification-surface readability; no backend API contract or business-data behavior was changed.

final result: passed for audited scope
