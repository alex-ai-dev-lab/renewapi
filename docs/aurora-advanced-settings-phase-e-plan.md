# Aurora Bento v2 — Phase E Advanced Settings real-backend plan

Date: 2026-08-23
Base: `42cb3ebb70932e7a49a373faf6cc44a48ab98378`
Branch: `qa/aurora-advanced-settings-phase-e`

## Goal

Close the remaining **Advanced Settings expanded backend coverage** item with the same evidence standard used by the hardened Settings foundation gate: a real RenewAPI process, fresh isolated SQLite, production migrations, real root setup/login, normal `RootAuth`, browser-driven Settings mutations, authenticated option reads, direct SQLite inspection, and process restart persistence.

This phase does not use mocked `/api/option` responses.

## Wave 1 — Cross-section mutation and validation matrix

Representative high-risk settings across three advanced areas:

### Operations / Worker Proxy

- explicit persisted baseline for Worker URL, access key, and HTTP-image toggle
- invalid Worker URL is rejected by frontend validation before any option mutation
- valid URL/key/toggle mutation is exercised through the real UI
- trailing slash normalization is verified
- API/SQLite synchronization is verified; secret persistence is checked directly in SQLite
- restart UI verifies the normalized URL and toggle state

### Security / Rate Limiting

- explicit persisted baseline
- invalid group-limit JSON/value tuple is rejected locally with no mutation
- enabled state, duration, request limits, and group JSON are saved through the real bulk endpoint
- API/SQLite synchronization and restart UI are verified

### Billing / Check-in Rewards

- explicit persisted baseline
- enabled range with `max < min` is rejected by the cross-field resolver with no mutation
- valid enabled/min/max values are saved through the real bulk endpoint
- API/SQLite synchronization and restart UI are verified

### Security / SSRF Protection

- explicit persisted baseline
- domain/IP filtering, lists, allowed ports, and resolved-domain IP filtering are edited through the real UI
- sequential option mutations are verified through API and direct SQLite reads
- normalized JSON-array storage and restart UI are verified

## Wave 1 acceptance

- frontend tests / typecheck / production build PASS
- real RenewAPI Go binary build PASS
- fresh SQLite `migrate --up` / `migrate --check` PASS
- invalid Worker / Rate Limit / Check-in cases produce no option mutation and preserve SQLite baseline
- all valid mutations synchronize UI/API/SQLite
- one clean process restart preserves all Wave 1 advanced values
- console errors = 0
- page errors = 0
- backend child logs drain cleanly

## Wave 2 — Stateful/destructive advanced flows

After Wave 1 is green, cover separately so failures stay attributable:

- Request Guard CRUD create/update/delete and confirmation behavior
- model/group pricing JSON validation, atomicity, and restart persistence
- log deletion destructive confirmation against seeded records
- selected secret/configured-state behavior where a masked value has special semantics (for example Anti-Poison signed-audit secret)

## Evidence boundary

Wave 1 is deliberately representative rather than a claim that every advanced input has been exhaustively toggled. Phase E is only marked complete after Wave 2 stateful/destructive coverage is also green and the durable tracker is updated.
