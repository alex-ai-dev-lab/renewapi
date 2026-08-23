# Aurora Bento v2 — Responsive Phase B Baseline

> Recorded: 2026-08-23 +08:00  
> Repository: `alex-ai-dev-lab/renewapi`  
> Product merge before this note: `02e467d95a36d0b7d54ea55f3db305205eadc857`

## Scope

Responsive Phase B aligns the application mobile/desktop shell boundary with the Aurora desktop shell and establishes a deterministic browser QA baseline across tablet and mobile widths.

The product change moves `useIsMobile()` from a `768px` boundary to the Aurora `lg / 1024px` boundary and uses `useSyncExternalStore` with `matchMedia` so the mobile Sheet/navigation shell and the desktop Aurora Topbar/Floating Dock switch at the same breakpoint.

## Browser QA evidence

Final baseline workflow run: `32619390956`

Artifact: `aurora-responsive-visual-qa` / id `9487941102`

Validated matrix:

- themes: Light + Dark
- viewports: `1024×1000`, `768×1000`, `375×812`
- pages: Dashboard, Channels, API Keys, Usage Logs, Models, Users, System Settings
- deviceScaleFactor: `1`
- locale: `zh-CN`
- timezone: `Asia/Taipei`

Quality/runtime results:

- changed-file Prettier: PASS
- changed-file ESLint: PASS
- copyright check: PASS
- `bun test`: PASS
- `bun run typecheck`: PASS
- `bun run build`: PASS
- browser capture matrix: PASS
- console errors: `0`
- unhandled QA API requests: `0`
- horizontal overflow: `0`

## Acceptance boundary

This run proves the responsive shell breakpoint alignment and establishes a clean no-overflow baseline for the seven core pages in both themes. The supplied Aurora source package does not contain dedicated tablet/mobile boards, so this evidence does not claim invented pixel-level tablet/mobile parity.

Remaining Phase B work belongs to deeper responsive visual and interaction review: mobile navigation behavior, Bento collapse quality, dense-table strategy, Drawer/Dialog viewport safety, and long-i18n wrapping.

## Repository hygiene

The temporary responsive QA workflow used to generate the evidence was removed before the product PR was merged into `main`; only the validated product change was retained.
