# Aurora Bento v2 Manual Acceptance Protocol

Status: **MANUAL REQUIRED**

This protocol covers the three Aurora Bento v2 Definition-of-Done items that cannot be represented faithfully by the Linux/headless browser CI. It is an acceptance procedure, not evidence that the checks have already passed.

Run these checks against the exact release-candidate source/image being considered for release. Record the source SHA, image digest, operating system, browser/assistive-technology versions, tester, date, and result with the captured evidence. A failure is actionable only when it is reproducible on the same release-candidate source.

## Evidence record

For each manual session record:

- source commit SHA;
- deployed image reference and immutable digest when applicable;
- OS and version;
- browser and version;
- assistive technology and version, if used;
- theme and viewport/window size;
- PASS/FAIL per case;
- screenshot/video or written reproduction for every FAIL;
- console/network excerpt only when it explains a failure; do not capture secrets or session tokens.

A manual gate is PASS only when all required cases for that gate pass on the same release candidate. Do not substitute deviceScaleFactor, CSS transforms, browser DevTools emulation, DPR proxies, or automated ARIA snapshots for the manual checks below.

## A. Literal browser UI 200% zoom

### Environment

Use current stable Chrome or Chromium on Windows or macOS. Test with a normal desktop browser window; do not use Playwright, DevTools device emulation, page-level CSS zoom, or a transformed viewport.

### Set literal zoom

1. Open the deployed RenewAPI site in Chrome/Chromium.
2. Open the browser menu (`⋮`).
3. In **Zoom**, set the browser to **200%**.
4. Confirm the browser chrome/menu reports `200%`.
5. Keep 200% enabled for the entire matrix. A browser window wide enough for ordinary desktop use is preferred; if a responsive breakpoint is crossed naturally at 200%, that is part of the test rather than a reason to resize back to the old layout.

### Required pages

Run both **Light** and **Dark** themes for:

- Dashboard
- Channels
- API Keys
- Usage Logs
- Models
- Users
- System Settings
- sign-in
- pricing
- rankings
- profile
- wallet
- playground
- 404

For authenticated pages, use a non-production acceptance account with representative data. Do not use credentials that must appear in screenshots.

### Checks on every page

Mark FAIL for any of the following unless the behavior is an intentional component-local scroller such as a wide data table:

- page-level horizontal scrolling caused by layout overflow;
- primary text or controls clipped outside the viewport;
- buttons, menus, tabs, pagination, or destructive actions that cannot be reached or operated;
- dialogs, sheets, popovers, menus, toasts, or confirmation controls rendered partly off-screen with no usable way to reach them;
- fixed/sticky header, sidebar, dock, or navigation covering content or controls;
- focus indicator hidden behind another layer or outside the visible viewport;
- text overlap, unreadable truncation of essential labels, or text drawn over another control;
- table controls becoming unusable rather than remaining reachable through the intended table scroller/responsive representation;
- light/dark theme content becoming indistinguishable because of zoom-triggered layout changes.

Exercise at least one representative interactive state on each applicable page: open a menu/select, focus a form control, and open/close a dialog or sheet. On Channels, API Keys, Models, and System Settings also open a destructive/confirmation path but cancel before any production mutation.

### PASS criteria

PASS requires all required pages in both themes to remain readable and operable at literal browser 200% zoom, with no unintended page-level horizontal overflow, inaccessible action, off-screen modal action, focus obstruction, or essential text overlap. Intentional component-local table scrolling is acceptable when the table and its controls remain keyboard and pointer operable.

## B. Desktop screen-reader smoke

Screen-reader results are manual evidence. Automated axe/ARIA snapshots do not replace these checks.

### B1. NVDA + Chrome or Firefox

Use current stable NVDA with current stable Chrome or Firefox on Windows. Run at least:

- sign-in
- Dashboard
- Channels
- API Keys
- System Settings
- profile
- wallet
- playground

For each page:

1. Navigate to the page from a fresh route transition and confirm the page/document title is meaningful.
2. Use NVDA landmark and heading navigation to confirm the main landmark, navigation landmarks, and heading order are understandable and not duplicated nonsensically.
3. Tab through interactive controls and confirm the spoken name matches the visible purpose.
4. Check icon-only buttons: each must announce an unambiguous action rather than only `button` or an icon name.
5. Open at least one Select/combobox and verify the control name, expanded/collapsed state, selected option, and option navigation are announced.
6. Check form fields for programmatic labels; placeholder-only naming is a FAIL for required fields.
7. Trigger one safe validation error and confirm the error is discoverable/announced and associated with the relevant field or operation.
8. Open and close a dialog/confirmation. Confirm focus moves into the dialog, the dialog name is announced, background content is not treated as the active interaction surface, and focus returns to a sensible trigger after close/cancel.
9. On Channels/API Keys, inspect table navigation/row actions and pagination where present. Table headers and controls must be understandable without relying on visual position alone.
10. On quota/progress surfaces, confirm the metric has a meaningful accessible name/value rather than only an unlabeled progress role.
11. Trigger a safe toast/live notification and verify it is announced without moving keyboard focus unexpectedly.
12. At a mobile-width window, close the public/mobile navigation and verify its hidden links are not announced or reachable; open it and verify the links become available in a sensible order.

Page-specific minimums:

- **sign-in:** username/password labels, submit button, validation error, forgot-password link.
- **Dashboard:** main heading, KPI names/values, navigation, chart/trend summary or equivalent non-visual data.
- **Channels:** search/filter, table semantics, row action, destructive confirmation cancel, pagination if present.
- **API Keys:** quota/progress name, row actions, sensitive-key reveal/copy action naming, destructive confirmation cancel.
- **System Settings:** section headings, labeled inputs, switches, selects, validation feedback, save action, destructive confirmation where present.
- **profile:** language/select naming and current value.
- **wallet:** read-only/referral field label and relevant balance/recharge controls.
- **playground:** model and group selectors, prompt input, attach/search controls, send/stop action naming.

NVDA PASS requires no unnamed required control, no focus trap/loss in the sampled flows, no hidden mobile navigation announced while closed, and no critical state communicated visually only.

### B2. VoiceOver + Safari

Use current macOS Safari and VoiceOver. Repeat the same required page set and the same semantic checks. At minimum:

1. Use VoiceOver Web Rotor to inspect landmarks, headings, links, and form controls.
2. Traverse controls with VoiceOver navigation and keyboard Tab where applicable.
3. Verify button/icon-only control names, select/combobox state, form labels, validation errors, dialogs, table/row actions, progress/quota values, and live/toast feedback.
4. Verify dialog focus entry and return after close/cancel.
5. Verify closed mobile navigation is absent from VoiceOver navigation and becomes available only when opened.

VoiceOver PASS uses the same functional criteria as NVDA: all critical controls and states must be understandable and operable without visual inference.

### B3. JAWS minimum smoke

If a licensed/current JAWS environment is available, run the minimum smoke on:

- sign-in
- Dashboard
- Channels
- API Keys
- System Settings

Verify page title, landmarks/headings, control names, form labels, select/combobox operation, table/row actions, a validation error, one dialog with focus restoration, and one live/toast message. Record `NOT RUN — environment unavailable` rather than PASS when JAWS is not available.

## C. External deployment and restart smoke

Perform this on staging first. Production may be used only with the normal backup/rollback controls and without weakening rate limits or security settings.

### C1. Pin exact source/image

1. Record the accepted main SHA.
2. Use an immutable image identity, preferably `ghcr.io/alex-ai-dev-lab/renewapi:sha-<12-char-source-sha>` plus its registry digest, or deploy directly by digest.
3. Pull the image before changing the service.
4. Verify the pulled image digest matches the recorded digest.
5. Inspect OCI labels and confirm `org.opencontainers.image.revision` equals the accepted source SHA and `org.opencontainers.image.version` equals the intended `VERSION`.
6. Keep the previous known-good image digest available for rollback.

Do not use a mutable `edge`, `rc`, or `latest` alias as the only deployment identity in the acceptance record.

### C2. Pre-deploy safety

- back up the database and persistent data according to the deployment's normal procedure;
- record current image digest and configuration;
- verify `SESSION_SECRET` remains a strong deployment secret and is not replaced by CI/test values;
- do not disable `GLOBAL_WEB_RATE_LIMIT`, `GLOBAL_API_RATE_LIMIT`, or other production safety controls for the smoke test;
- validate the Compose configuration with the real deployment environment before replacement.

### C3. Startup, migration, and health

After deploying the exact image:

1. Confirm the container/process reaches healthy/running state without crash-looping.
2. Review startup logs for migration failures, panic/fatal messages, repeated authentication/config errors, and static bundle errors.
3. Confirm production migrations complete normally; where operationally available, run the image/binary migration check used by the release pipeline before serving traffic.
4. Verify `GET /api/status` returns success through the same network path used by the deployment health check.
5. Confirm the service remains healthy for at least one normal health-check interval after startup.

### C4. Static assets and web rate limiting

This check is mandatory because Aurora RC includes the static-asset limiter fix.

1. Load sign-in and at least one authenticated page with browser DevTools Network recording enabled.
2. Confirm HTML/application navigation and `/static/*` or `/assets/*` JavaScript/CSS/font/image requests load successfully.
3. Reload/navigate repeatedly within a bounded smoke window and confirm static JS/CSS/image requests do **not** return 429 and no `ChunkLoadError` occurs.
4. Confirm application/document routes are still routed through the configured web limiter. On staging, a controlled bounded test may exceed the configured document-route budget and should observe the expected limiter response on an application/document route while direct static assets continue to load. Do not generate an abusive burst against production merely to prove this point.
5. Confirm API rate limiting remains enabled and behaves according to production policy; do not copy the elevated synthetic CI budgets into the deployment.

FAIL if static bundles are rate-limited as documents, if `ChunkLoadError` appears, or if the production limiter has been disabled/bypassed globally.

### C5. Functional route smoke

Using a real browser and the deployed endpoint, verify:

- sign-in and authenticated session creation;
- session remains valid across normal navigation;
- Dashboard loads real data;
- Channels list and a safe read/detail action;
- API Keys list and a safe read action without exposing a secret unintentionally;
- Models list;
- System Settings read path and a non-destructive safe setting read/validation path;
- public sign-in, pricing, rankings, and 404 routes;
- representative secondary authenticated route: profile, wallet, or playground;
- one representative authenticated API request returns the expected status/body;
- browser console has no release-blocking runtime error.

Do not perform destructive changes on production solely for smoke coverage.

### C6. Restart and persistence

1. Record a pre-restart value/state that should persist, using a safe acceptance/staging setting or seeded acceptance data.
2. Restart the deployed container/process through the normal orchestration path.
3. Confirm health returns without manual database repair.
4. Sign in again and confirm the persisted state remains correct.
5. Recheck Dashboard, one authenticated list page, `/api/status`, and one static JS asset.
6. Confirm there is no post-restart migration loop, session/config corruption, 429 static-asset regression, or startup panic.

### C7. Rollback criteria

Rollback to the previously recorded immutable image digest if any P0/P1 release blocker appears, including startup/migration failure, auth failure, data corruption/persistence failure, critical route outage, systematic static-asset 429/ChunkLoadError, or a security-control regression. Preserve logs and the failed image/source identities before rollback.

## Manual acceptance result

Record the final state explicitly:

- Literal browser UI 200% zoom: `PASS` / `FAIL` / `PENDING MANUAL`
- NVDA: `PASS` / `FAIL` / `PENDING MANUAL`
- VoiceOver: `PASS` / `FAIL` / `PENDING MANUAL`
- JAWS: `PASS` / `FAIL` / `NOT RUN — environment unavailable`
- External deployment/restart: `PASS` / `FAIL` / `PENDING ENVIRONMENT`

Do not mark Aurora Bento v2 `FULL RELEASE PASS` until every release-required manual/environment gate has an explicit accepted result.