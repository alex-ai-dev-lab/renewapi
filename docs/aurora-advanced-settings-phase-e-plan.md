# Aurora Bento v2 — Phase E Advanced Settings real-backend plan

Date: 2026-08-24
Wave 1 base: `42cb3ebb70932e7a49a373faf6cc44a48ab98378`
Current execution branch: `main`
Final Wave 2 validated product head: `f5fa30573b11096aca69007e30bb2787fa2d4449`

## Goal

Close the remaining **Advanced Settings expanded backend coverage** item with the same evidence standard used by the hardened Settings foundation gate: a real RenewAPI process, fresh isolated SQLite, production migrations, real root setup/login, normal `RootAuth`, browser-driven Settings mutations, authenticated option reads, direct SQLite inspection, and process restart persistence.

The final Wave 2 proof does not use mocked `/api/option` responses.

## Wave 1 — Cross-section mutation and validation matrix

**Status: ✅ PASS**

Final proof:

- GitHub Actions run: `32649601628`
- job: `real-backend-wave1`
- artifact: `settings-advanced-phase-e-wave1` / id `9495852590`
- artifact digest: `sha256:c9442910699ae2115fbf2e31578b4c0a2fbf6b8ee64e8c8a65734148751a3d69`
- frontend tests: `53 pass / 0 fail`
- TypeScript / default production build / classic production build: PASS
- real RenewAPI Go binary + fresh SQLite + production migrations: PASS
- console errors: `0`
- page errors: `0`

### Operations / Worker Proxy

- [x] explicit persisted baseline for Worker URL, access key, and HTTP-image toggle
- [x] invalid Worker URL is rejected by frontend validation before any option mutation
- [x] valid URL/key/toggle mutation is exercised through the real UI
- [x] trailing slash normalization is verified
- [x] API/SQLite synchronization is verified; secret persistence is checked directly in SQLite
- [x] restart UI verifies the normalized URL and toggle state

### Security / Rate Limiting

- [x] explicit persisted baseline
- [x] invalid group-limit JSON/value tuple is rejected locally with no mutation
- [x] enabled state, duration, request limits, and group JSON are saved through the real bulk endpoint
- [x] API/SQLite synchronization and restart UI are verified
- [x] numeric controls have stable accessible names for browser and assistive-tech automation

### Billing / Check-in Rewards

- [x] explicit persisted baseline
- [x] enabled range with `max < min` is rejected by the cross-field resolver with no mutation
- [x] valid enabled/min/max values are saved through the real bulk endpoint
- [x] API/SQLite synchronization and restart UI are verified

### Security / SSRF Protection

- [x] explicit persisted baseline
- [x] domain/IP filtering, lists, allowed ports, and resolved-domain IP filtering are edited through the real UI
- [x] normalized JSON-array storage and restart UI are verified
- [x] `fetch_setting.allowed_ports` is aligned with the backend string-array contract
- [x] one Save uses a single `/api/option/bulk` mutation so multi-field SSRF persistence is atomic

Wave 1 found an actionable product defect rather than only producing QA evidence: SSRF previously persisted changed fields sequentially and typed `allowed_ports` as `number[]`. The product fix now batches all changed SSRF options in one bulk mutation and preserves the backend `string[]` representation.

## Wave 2 — Stateful/destructive advanced flows

**Status: ✅ PASS**

Final proof:

- validated product head: `f5fa30573b11096aca69007e30bb2787fa2d4449`
- GitHub Actions run: `32681901244`
- job: `real-backend-wave2` / id `97299914277`
- commit status context: `settings-advanced-phase-e-wave2 = success`
- artifact: `settings-advanced-phase-e-wave2` / id `9504447296`
- artifact digest: `sha256:2c9c91046c309d849a2c3b44fb0ea13ca6aa726e2154879c25b9f5aafa021588`
- targeted Go regressions: PASS
- default frontend: `53 pass / 0 fail`, TypeScript PASS, production build PASS
- classic production build: PASS
- real RenewAPI Go binary + fresh SQLite + production migrations: PASS
- final real-backend harness summary: PASS
- restart persistence: PASS
- evidence secret-redaction gate: PASS
- console errors: `0`
- page errors: `0`

### Request Guard

- [x] destructive endpoint removal requires confirmation in the UI (`b3ddb6cd`)
- [x] Cancel leaves the local draft unchanged
- [x] Confirm removes only from the unsaved draft and explicitly tells the operator that Save is required to persist
- [x] endpoint removal control has an endpoint-specific accessible name
- [x] backend validates the complete replacement before persistence
- [x] deleted endpoint secret cleanup is included in the same `UpdateOptionsBulk` call as `request_guard_setting`
- [x] controller regression verifies removed endpoint secret is cleared while other endpoint secrets remain (`4e301890`, quality-gate cleanup `04868f9c`)
- [x] controller regression verifies invalid configuration returns 400 without mutating SQLite, OptionMap, secret state, or runtime Request Guard state
- [x] real browser create/edit/delete flow runs against a fresh RenewAPI process
- [x] real browser verifies destructive confirmation Cancel/Confirm semantics
- [x] direct SQLite and authenticated API checks verify persisted configuration and removed-secret cleanup
- [x] restart verifies persisted presence/absence after the CRUD sequence

### Model / group pricing

- [x] JSON is validated before save
- [x] Model Pricing changed fields persist in one bulk mutation instead of sequential option writes (`91664d9d`)
- [x] Group Pricing changed fields persist in one bulk mutation instead of sequential option writes (`91664d9d`)
- [x] special API-key mappings remain intact
- [x] backend `UpdateOptionsBulk` uses a DB transaction and restores DB/OptionMap if runtime configuration application fails
- [x] existing `model/option_rollback_test.go` locks the bulk rollback invariant
- [x] real browser invalid-JSON path proves no backend mutation
- [x] valid multi-field changes are observed as one bulk persistence unit
- [x] authenticated API + direct SQLite synchronization is verified
- [x] restart persistence is verified

### Log deletion

- [x] UI requires a destructive confirmation before issuing the delete request
- [x] Cancel path issues no delete mutation
- [x] backend deletes strictly `created_at < targetTimestamp`
- [x] durable regression verifies batched deletion removes old rows while preserving the exact cutoff row and newer rows (`43a805ef`)
- [x] fresh SQLite is seeded with controlled log rows for the real-process browser proof
- [x] Cancel preserves seeded rows
- [x] Confirm deletes only rows older than the selected cutoff
- [x] direct SQLite row verification confirms cutoff/newer rows remain
- [x] restart preserves the deletion result

### Anti-Poison signed-audit secret

- [x] `/api/option/` does not expose the real secret
- [x] backend emits only `anti_poison_setting.signed_header_audit_secret_configured=true/false`
- [x] frontend carries configured state separately from the blank secret input
- [x] dirty-field submission means saving unrelated Anti-Poison fields does not overwrite an already configured secret with an empty value
- [x] durable regression verifies secret masking plus configured-state reporting (`adfb8d12`, JSON-boundary cleanup `e09a0054`)
- [x] real-process dummy secret is persisted and verified directly in SQLite without entering evidence output
- [x] an unrelated Anti-Poison save preserves the existing secret
- [x] explicit secret rotation persists the replacement value
- [x] restart reports configured state while still not exposing the real secret
- [x] artifact redaction gate scans for all QA secret literals before upload

## Wave 2 engineering evidence

- `model.UpdateOptionsBulk` serializes option updates and persists changed options inside `DB.Transaction`.
- If runtime application fails after DB persistence, rollback restores the database snapshot and OptionMap; existing rollback regression coverage remains in place.
- Request Guard full-config replacement and removed-secret cleanup share that same bulk persistence unit.
- The repository quality workflow rejects newly introduced direct `encoding/json` usage outside the shared JSON boundary; Wave 2 tests use `common.Marshal/common.Unmarshal`.
- `.github/settings-advanced-phase-e-wave2.mjs` and `.github/workflows/settings-advanced-phase-e-wave2.yml` are retained as durable regression infrastructure rather than one-time QA patches.

## Final evidence boundary

**Phase E Advanced Settings expanded backend coverage is complete.** Wave 1 and Wave 2 both have real-backend evidence covering browser interaction, authenticated API state, direct SQLite state, validation/no-mutation behavior, destructive confirmation, secret semantics, and restart persistence.

This closes the Advanced Settings item only. It does **not** close the remaining Aurora Bento v2 product DoD items: Secondary Surfaces, real desktop screen-reader / literal browser UI 200% zoom checks, and release/deployment smoke.
