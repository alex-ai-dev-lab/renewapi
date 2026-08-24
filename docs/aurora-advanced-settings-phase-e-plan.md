# Aurora Bento v2 — Phase E Advanced Settings real-backend plan

Date: 2026-08-24
Wave 1 base: `42cb3ebb70932e7a49a373faf6cc44a48ab98378`
Current execution branch: `main`
Current Wave 2 code head at documentation time: `04868f9c243bfb4bfd8c76e9eb8d0ee8bc87759b`

## Goal

Close the remaining **Advanced Settings expanded backend coverage** item with the same evidence standard used by the hardened Settings foundation gate: a real RenewAPI process, fresh isolated SQLite, production migrations, real root setup/login, normal `RootAuth`, browser-driven Settings mutations, authenticated option reads, direct SQLite inspection, and process restart persistence.

This phase does not use mocked `/api/option` responses for the final real-backend proof.

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

**Status: 🚧 IN PROGRESS — product hardening and durable regression coverage landed; final real-process browser/API/SQLite/restart proof still pending**

Wave 2 is being executed directly on `main`; no separate Wave 2 branch is used.

### Request Guard

- [x] destructive endpoint removal now requires confirmation in the UI (`b3ddb6cd`)
- [x] Cancel leaves the local draft unchanged
- [x] Confirm removes only from the unsaved draft and explicitly tells the operator that Save is required to persist
- [x] endpoint removal control has an endpoint-specific accessible name
- [x] backend replacement validates the complete next configuration before persistence
- [x] deleted endpoint secret cleanup is included in the same `UpdateOptionsBulk` call as `request_guard_setting`
- [x] controller regression verifies removed endpoint secret is cleared while other endpoint secrets remain (`4e301890`, quality-gate cleanup `04868f9c`)
- [x] controller regression verifies invalid configuration returns 400 without mutating SQLite, OptionMap, secret state, or runtime Request Guard state
- [ ] real browser create/edit/delete flow against a fresh RenewAPI process
- [ ] restart persistence/absence proof after create, edit, and delete

### Model / group pricing

- [x] JSON is validated before save
- [x] Model Pricing changed fields now persist in one bulk mutation instead of sequential option writes (`91664d9d`)
- [x] Group Pricing changed fields now persist in one bulk mutation instead of sequential option writes (`91664d9d`)
- [x] special API-key mappings remain intact
- [x] backend `UpdateOptionsBulk` uses a DB transaction and restores DB/OptionMap if runtime configuration application fails
- [x] existing `model/option_rollback_test.go` locks the bulk rollback invariant
- [ ] real browser invalid-JSON no-mutation proof
- [ ] real browser valid multi-field API/SQLite synchronization and restart proof

### Log deletion

- [x] UI already requires a destructive confirmation before issuing the delete request
- [x] Cancel path issues no delete mutation
- [x] backend deletes strictly `created_at < targetTimestamp`
- [x] durable regression verifies batched deletion removes old rows while preserving the exact cutoff row and newer rows (`43a805ef`)
- [ ] seeded real-process Cancel/Confirm browser proof with direct SQLite row-count verification
- [ ] restart proof after deletion

### Anti-Poison signed-audit secret

- [x] `/api/option/` does not expose the real secret
- [x] backend emits only `anti_poison_setting.signed_header_audit_secret_configured=true/false`
- [x] frontend carries configured state separately from the blank secret input
- [x] dirty-field submission means saving unrelated Anti-Poison fields does not overwrite an already configured secret with an empty value
- [x] durable regression verifies secret masking plus configured-state reporting (`adfb8d12`, JSON-boundary cleanup `e09a0054`)
- [ ] real-process set/change/unrelated-save persistence proof with a dummy secret and direct SQLite verification
- [ ] restart configured-state proof without exposing the secret in logs/artifacts

## Wave 2 engineering evidence already established

- `model.UpdateOptionsBulk` serializes option updates and persists changed options inside `DB.Transaction`.
- If runtime application fails after DB persistence, rollback restores the database snapshot and OptionMap; existing rollback regression coverage remains in place.
- Request Guard full-config replacement and removed-secret cleanup share that same bulk persistence unit.
- The repository quality workflow rejects newly introduced direct `encoding/json` usage outside the shared JSON boundary; Wave 2 tests were aligned to `common.Marshal/common.Unmarshal` before final QA.

## Evidence boundary

Wave 1 is complete. Wave 2 product fixes and durable regression coverage are substantially in place, but **Phase E is not yet marked complete** because code inspection/unit-regression evidence is not a substitute for the required real RenewAPI browser/API/SQLite/restart proof.

Do not describe Wave 2 or Advanced Settings expanded backend coverage as PASS until that final matrix is green and the durable tracker is updated.
