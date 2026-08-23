# Aurora Settings real-backend QA

Date: 2026-08-23

## Result

**PASS** for the audited System Settings mutation path against a real RenewAPI process and an isolated SQLite database.

Final validated product head before removal of the one-time QA harness: `d3d30a59a71c587464bfdb7e94a1469efb761dba`.

Final workflow evidence:

- workflow: `Settings real backend mutation smoke`
- run: `32628206857` (run #14)
- result: success
- artifact: `settings-real-backend-smoke`
- artifact id: `9490331010`
- artifact digest: `sha256:074ee2048a21fa809cb07b82b7b36ba813461e8e02d96a172c4178da08436ac2`

The temporary workflow and Playwright harness used to produce this evidence were removed before merge.

## Environment

The final run used:

- production RenewAPI Go binary built from the PR head
- production default and classic frontend builds embedded into that binary
- a brand-new isolated SQLite database
- production schema migrations via `migrate --up` followed by `migrate --check`
- no pre-seeded `theme.frontend` database value
- a real root account created through `/api/setup`
- a real login session created through `/api/user/login`
- the normal `New-Api-User` header required by `RootAuth`
- the production `/system-settings` page served by the embedded default frontend

Backend access logs confirm the redesigned frontend was served through `/static/js/...` assets and that `/system-settings` issued real `/api/option/` requests.

## Assertions exercised

### Fresh-install frontend default

The authoritative `ThemeSettings.Frontend` default is now `default`.

The final run starts from a fresh database without writing `theme.frontend` before boot. The real binary naturally serves the redesigned Aurora frontend. A focused Go regression test also verifies the default/synchronization path, while a second test verifies that an explicitly persisted `classic` value remains supported as an override.

### Real Settings reads and authentication

The browser loads `/system-settings` with a real root session and the same user-id header contract used by the application. `GET /api/option/` succeeds through `RootAuth`; there are no mocked option responses.

### Single-option mutation

The `Automatic retries` switch performs the production single-setting mutation:

- baseline `RetryTimes = 0`
- UI enable -> `RetryTimes = 2`
- UI disable -> `RetryTimes = 0`

Both writes are observed as real `PUT /api/option/` requests and verified by subsequent authenticated reads.

### Invalid bulk mutation / rollback behavior

The browser drafts:

- `SystemName = SHOULD_NOT_PERSIST`
- `ServerAddress = not-a-valid-url`
- `QuotaForNewUser = 999999`

`Save all` sends the real `PUT /api/option/bulk` request. Backend validation rejects the invalid server URL with the expected business error.

Verified behavior:

- no partial persistence of the other drafted values
- baseline database values remain unchanged
- the browser keeps the local draft so the operator can correct it
- the backend validation message is visible in the Aurora UI

### Valid bulk mutation

The browser then saves:

- `SystemName = RenewAPI QA Browser`
- `ServerAddress = http://127.0.0.1:4173/qa`
- `QuotaForNewUser = 424242`
- `RetryTimes = 0`

The real bulk mutation succeeds and subsequent authenticated reads return the same values.

### Process restart persistence

The RenewAPI process is terminated cleanly and restarted against the same SQLite file. A new root login session is established.

After restart:

- authenticated `/api/option/` reads return the saved values
- `/system-settings` renders the same values
- the shell brand reflects `RenewAPI QA Browser`

This verifies persistence beyond in-memory option state.

## Browser evidence

The artifact contains:

- `settings-validation-error.png`
- `settings-success.png`
- `settings-after-restart.png`
- initial and restart backend logs
- `summary.json`
- `summary.txt`

Manual review found no actionable layout or interaction defect in the audited states. The invalid-save screenshot intentionally retains the local draft and shows the backend validation toast; the restart screenshot is clean and shows the persisted values.

Final diagnostics:

- console errors: `0`
- page errors: `0`
- restart persistence: PASS
- invalid bulk atomicity: PASS
- real option API path: PASS

## Product fix discovered by the smoke

The first real-backend runs exposed that fresh installations still served the classic frontend even though Aurora had become the accepted primary UI. The root cause was a conflicting authoritative default:

- `common` held one runtime theme value
- `setting/system_setting/theme.go` registered `ThemeSettings` with `Frontend: "classic"`
- package initialization synchronized that registered value back into `common`, overriding attempts to change only the low-level fallback

The final fix changes the authoritative `ThemeSettings.Frontend` default to `default`. The unrelated low-level `common/constants.go` change was reverted so there is a single source of product-default intent. Existing databases that explicitly store `classic` continue to override the default.

## Scope boundary

This gate proves the Settings landing-page foundation mutation path, backend validation/atomicity behavior, authentication contract, fresh-install Aurora selection and restart persistence. It does not claim exhaustive coverage of every advanced Settings section or every destructive/confirmation state; those remain part of the broader Phase C deep-state backlog.
