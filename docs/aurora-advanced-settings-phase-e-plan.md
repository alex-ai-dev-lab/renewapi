# Aurora Bento v2 — Phase E Advanced Settings real-backend plan

Date: 2026-08-23
Base: `42cb3ebb70932e7a49a373faf6cc44a48ab98378`
Branch: `qa/aurora-advanced-settings-phase-e`

## Goal

Close the remaining **Advanced Settings expanded backend coverage** item with the same evidence standard used by the hardened Settings foundation gate: a real RenewAPI process, fresh isolated SQLite, production migrations, real root setup/login, normal `RootAuth`, browser-driven Settings mutations, authenticated option reads, direct SQLite inspection, and process restart persistence.

This phase does not use mocked `/api/option` responses.

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

**Status: ⏳ next**

Keep these flows separate so stateful/destructive failures remain attributable:

- Request Guard CRUD create/update/delete and confirmation behavior
- model/group pricing JSON validation, atomicity, and restart persistence
- log deletion destructive confirmation against seeded records
- selected secret/configured-state behavior where a masked value has special semantics, for example Anti-Poison signed-audit secret

## Evidence boundary

Wave 1 is representative rather than a claim that every advanced input has been exhaustively toggled. Phase E is only marked complete after Wave 2 stateful/destructive coverage is also green and the durable tracker is updated.

The Wave 1 workflow and patch harness are one-time QA machinery and are removed before merge; durable product fixes and QA documentation remain.
