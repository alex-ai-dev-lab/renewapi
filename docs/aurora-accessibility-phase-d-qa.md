# Aurora Bento v2 — Phase D Accessibility QA

> Final validated product head: `054378370c727b36e613c8c54cd7c85b4f9955b9`  
> Validated PR merge tree: `f68be6c909dfe38ff0d15e46f6d811f86ceed996`  
> GitHub Actions final proof run: `32645510077`  
> Artifact: `aurora-accessibility-phase-d-final-proof` / id `9494793377`  
> Artifact digest: `sha256:c29272a0b22e45a6452934cfe95c21dfd8531173fb3bc3a6334c6d0e84d19be0`

## 1. Conclusion

The automation-friendly Aurora Phase D accessibility gate is complete for the validated core/deep-state matrix.

- P0: `0`
- P1: `0`
- P2: `0`
- axe WCAG A/AA violations: `0`
- console errors: `0`
- page errors: `0`
- unhandled deterministic fixture requests: `0`
- `720×500` CSS viewport at DPR `2`: `14/14` Light/Dark core-page cases with no page-level horizontal overflow
- ARIA snapshots: `28`
- frontend tests/typecheck/production build: PASS

The final report still contains `62` P3 `color-contrast` incomplete records. These are not axe violations. They are nodes for which axe cannot determine the final background because of gradients, compositing/overlap, or extremely short text. They were retained deliberately and manually reviewed instead of being hidden or excluded from the gate.

## 2. Automated Matrix

The audit reuses the already validated deterministic Phase C fixture and adds accessibility scanning to real rendered product states.

Covered states include:

- Channels / API Keys / Usage Logs / Models / Users loading, empty and forced business-error states.
- Destructive confirmations for Channels / Keys / Models.
- Channels / Keys / Models bulk-selection toolbars and confirmations.
- Channels search, Usage Logs page-2 pagination and Models management/edit.
- Dashboard request-trend accessibility regression and API-key localization regression.
- Seven core pages in Light and Dark baseline states.
- Seven core pages in Light and Dark at `720×500` CSS px, DPR `2`.
- Playwright ARIA snapshots for each Light/Dark baseline and high-DPI page.
- WCAG `2A`, `2AA`, `2.1 AA` and `2.2 AA` axe rule sets.

The high-DPI matrix is intentionally described as a layout/high-DPI 200% equivalence check. It is not presented as literal browser-chrome zoom.

## 3. Findings Closed

### 3.1 Accessible names

Phase D found controls whose visible value was not an adequate programmatic field name. The following were fixed with localized, field-specific names:

- table page-size selector
- Dashboard refresh-interval selector
- Usage Logs log-type selector
- Models vendor selector
- Models endpoint-template selector
- Models row actions and management more-actions menu triggers
- API Key and User remaining-quota progress bars

### 3.2 ARIA semantics

The Models loading registry used `aria-label` on a generic `div`, which axe flagged as a prohibited ARIA attribute in that context. The label was removed and the loading headline is now exposed as screen-reader-only text while preserving `aria-busy`.

### 3.3 Light semantic status contrast

The Light semantic tokens were strengthened while leaving Dark tokens unchanged:

- destructive: `oklch(0.48 0.245 27.325)`
- success: `oklch(0.46 0.148 165.5)`
- warning: `oklch(0.48 0.162 75.834)`
- info: `oklch(0.48 0.21 254)`

This closes repeated small-text failures in status chips, alerts and selected-row states while preserving the existing semantic hues.

### 3.4 Dashboard small-text accents

Dashboard request-rate/KPI subtitle colors were moved away from low-contrast fixed RGB accents and routed through the theme-aware semantic system so Light and Dark each use a suitable contrast value.

### 3.5 Avatar fallback contrast

Axe could not classify one-character avatar fallbacks reliably and reported them as `color-contrast` incomplete. Manual calculation found a real hole in the old translucent avatar palette: a white initial could fall to roughly `3.43:1` in Light.

The avatar generator now keeps the deterministic hashed hue and 54–61% saturation but constrains lightness to 26–28% and removes transparency. Exhaustive calculation across all `360` hues and the full generated saturation/lightness range gives a worst-case white-text contrast of approximately `5.00:1`, above the `4.5:1` normal-text requirement.

### 3.6 Text-carrying Aurora gradients

The supplied Aurora reference gradient was retained for decorative surfaces, but several text-carrying uses were too bright for small white text and could not be evaluated automatically by axe because the background was a gradient.

A separate `aurora-accessibility.css` layer now provides darker same-hue stops for text-carrying Light gradients:

- action gradient: `#3f66d8 → #7046d8`
- hover action gradient: `#365abf → #6339c5`
- heading text gradient: `#3f66d8 → #8a4ddb → #c1457f`

Every action-gradient endpoint clears `4.5:1` against white. The captured `text-aurora` uses are 34–38px headings; their stops clear the WCAG large-text threshold over the supplied Light ambient pastel surfaces.

The active Light desktop Dock item is explicitly mapped to the accessible action gradient as well.

### 3.7 Legacy fixed accent colors

Three legacy fixed RGB utility colors surfaced during manual review of axe incomplete nodes. They are now mapped to theme-aware tokens:

- `text-[#B4655F]` → `var(--muted-foreground)`
- `text-[#7C5CBF]` → `var(--info)`
- `text-[#2F7748]` → `var(--success)`

This removes the cross-theme failure mode where a color that was safe in Light became insufficient on Dark glass.

## 4. Manual Review of Axe Incomplete Nodes

The final artifact retained `62` P3 `color-contrast` incomplete scenario records. Their underlying nodes fall into three categories.

### Gradient background could not be determined

Most nodes are ordinary foreground/muted text or large Aurora headings over layered glass/ambient gradients. The regular theme foreground and muted-foreground tokens have strong margin over the darkest sampled Light ambient surface; the large `text-aurora` headings are covered by the accessible gradient constraints above. Small white text on Light action gradients is covered by endpoint-level `>4.5:1` constraints.

### Content too short to determine

Remaining short-text cases are single digits or compact tokens such as pagination values and selected-count badges. Their foreground/background combinations are now high-confidence theme colors or the accessible action gradient. The one short-text class that actually had a contrast defect—the avatar fallback—was identified manually and fixed with a mathematically bounded palette.

### Background partially overlaps another element

Two retained cases are API Key destructive-confirmation descriptions. They render as muted dark text on the light, raised dialog surface; the Light muted-foreground token has comfortably more than `4.5:1` against that near-white surface. The final screenshots were sampled to confirm the expected surface and text color are actually rendered.

## 5. Evidence Boundary

This QA intentionally does **not** claim either of the following:

1. A real VoiceOver/NVDA/JAWS session. Headless Chromium and ARIA snapshots are useful automation evidence but are not a substitute for a desktop assistive-technology smoke test.
2. Literal browser UI zoom at 200%. The `720 CSS px @ DPR2` matrix is a high-DPI/layout equivalence test, not browser-chrome zoom controls.

Those environment-dependent checks remain separate acceptance items. The result that is closed here is: automated WCAG/ARIA/core deep-state hardening plus manual resolution of the contrast classes that axe could not determine automatically.
