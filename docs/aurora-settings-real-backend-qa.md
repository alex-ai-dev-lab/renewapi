# Aurora Settings real-backend QA

Date: 2026-08-23

## Result

**PASS** for the audited System Settings mutation path against a real RenewAPI process and an isolated SQLite database.

Final validated product head before removal of the one-time hardened QA harness: `8b56c883206a8077b3e9bf900ef9564551c93a13`.

Final hardened workflow evidence:

- workflow: `Settings real backend hardened smoke`
- run: `32633291555` (run #2)
- result: success
- artifact: `settings-real-backend-hardened`
- artifact id: `9491668465`
- artifact digest: `sha256:a7f925ed004f102a63db904545c96cb3fbf792555f7d2452401b07c783db2839`

The earlier run `32628206857` proved the product path but was superseded as final evidence because its invalid-bulk atomicity check read the in-memory option map before a later valid save could overwrite the same database keys. The hardened run closes that evidence gap with direct SQLite checks and an intermediate process restart before any successful overwrite.

The temporary hardened workflow and Playwright harness used to produce this evidence are removed before merge.

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
- direct SQLite inspection for the audited option keys

## Assertions exercised

### Fresh-install frontend default

The authoritative `ThemeSettings.Frontend` default is `default`.

The final run starts from a fresh database without writing `theme.frontend` before boot. The real binary naturally serves the redesigned Aurora frontend. A focused Go regression test verifies the default/synchronization path, while a second test verifies that an explicitly persisted `classic` value remains supported as an override.

### Real Settings reads and authentication

The browser loads `/system-settings` with a real root session and the same user-id header contract used by the application. `GET /api/option/` succeeds through `RootAuth`; there are no mocked option responses.

### Single-option mutation

The `Automatic retries` switch performs the production single-setting mutation:

- baseline `RetryTimes = 0`
- UI enable -> `RetryTimes = 2`
- UI disable -> `RetryTimes = 0`

For each transition the hardened run waits for the controlled switch state to reflect the mutation, verifies the authenticated API value, and verifies the SQLite row. This removes the race that could otherwise let a second click repeat the first value.

### Invalid bulk mutation / atomicity

The browser drafts:

- `SystemName = SHOULD_NOT_PERSIST`
- `ServerAddress = not-a-valid-url`
- `QuotaForNewUser = 999999`

`Save all` sends the real `PUT /api/option/bulk` request. Backend validation rejects the invalid server URL with the expected business error.

Before any subsequent successful write, the hardened run verifies:

- the authenticated option API still returns the complete baseline
- direct SQLite reads still contain the complete baseline
- all three local draft fields remain in the browser
- the backend validation message is visible in the Aurora UI

The process is then stopped cleanly and restarted against the same SQLite file. After a new root login, both API and browser UI still render the complete baseline. This proves the rejected bulk mutation did not partially persist rows and avoids a later successful save masking corruption.

### Error-state feedback fidelity

Manual review of the first hardened artifact exposed one additional P2 UI issue: after the preceding single-option success, the old green success toast could coexist with the red bulk-validation error toast, creating contradictory feedback.

The Settings mutation hooks now use one stable Sonner toast id for single and bulk mutations. A later success or error replaces the previous Settings mutation toast instead of stacking contradictory statuses. The final validation-error screenshot contains only the red validation error and retains all three unsaved draft values.

### Valid bulk mutation

The browser then saves:

- `SystemName = RenewAPI QA Browser`
- `ServerAddress = http://127.0.0.1:4173/qa`
- `QuotaForNewUser = 424242`
- `RetryTimes = 0`

The real bulk mutation succeeds. The hardened run verifies the saved values through authenticated API reads and direct SQLite reads.

### Process restart persistence

The RenewAPI process is terminated cleanly again and restarted against the same SQLite file. A new root login session is established.

After the success restart:

- authenticated `/api/option/` reads return the saved values
- direct SQLite reads return the saved values
- `/system-settings` renders the same values
- the shell brand reflects `RenewAPI QA Browser`

This verifies persistence beyond in-memory option state.

### Backend log integrity

The harness waits for the child process `close` event and drains both stdout/stderr streams before ending the shared log writer. All three final backend logs end with `server exited`, including the intermediate invalid-bulk restart and final successful restart.

## Browser evidence

The final artifact contains:

- `settings-validation-error.png`
- `settings-after-invalid-restart.png`
- `settings-success.png`
- `settings-after-success-restart.png`
- three drained backend logs
- `summary.json`
- `summary.txt`

Manual review found no remaining actionable P0/P1/P2 in the audited states:

- validation error: only the red error toast is present; all three local draft values remain visible
- intermediate restart: the rejected draft is gone and the full baseline is restored from persistent storage
- successful save: the committed values and success feedback are visible
- final restart: the saved values and updated system brand persist after a new process and login

Final diagnostics:

- console errors: `0`
- page errors: `0`
- RetryTimes UI/API/SQLite synchronization: PASS
- invalid bulk API atomicity: PASS
- invalid bulk SQLite atomicity before overwrite: PASS
- invalid bulk intermediate restart persistence: PASS
- all three local draft fields preserved after rejection: PASS
- valid bulk SQLite persistence: PASS
- final restart persistence: PASS
- backend log drain: PASS

## Product fixes discovered by the real-backend QA

### Fresh-install frontend mismatch

The first real-backend runs exposed that fresh installations still served the classic frontend even though Aurora had become the accepted primary UI. The root cause was a conflicting authoritative default:

- `common` held one runtime theme value
- `setting/system_setting/theme.go` registered `ThemeSettings` with `Frontend: "classic"`
- package initialization synchronized that registered value back into `common`

The fix changes the authoritative `ThemeSettings.Frontend` default to `default`. Existing databases that explicitly store `classic` continue to override the default.

### Contradictory Settings mutation toasts

The hardened screenshot review exposed stale success feedback coexisting with a later validation failure. Settings single/bulk mutations now share one stable toast id so the latest mutation result replaces the previous Settings result without globally dismissing unrelated notifications.

## Scope boundary

This gate proves the Settings landing-page foundation mutation path, backend validation/atomicity behavior, authentication contract, fresh-install Aurora selection, error-feedback fidelity and restart persistence. It does not claim exhaustive coverage of every advanced Settings section or every destructive/confirmation state; those remain part of the broader Phase C deep-state backlog.
