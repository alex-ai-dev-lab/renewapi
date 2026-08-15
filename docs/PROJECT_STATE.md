# RenewAPI Project State

Last updated: 2026-08-15

## Current product version

Declared product version in `VERSION`:
`v1.0.0-rc.2`

The exact product tag for this version is `renewapi-v1.0.0-rc.2`. Raw
`v1.0.0-rc.21` through `v1.0.0-rc.24` tags present here are imported NewAPI
upstream tag objects and remain untouched.

Development branch:
`fix/routing-rate-limit-channel-edit-20260814`

Release preparation base commit:
`4f3eeeb9c` (`fix: isolate compaction routing from normal responses`)

The release commit containing this state is the only valid target for
`renewapi-v1.0.0-rc.2`; the tag must not be moved after publication.

## Upstream baseline

Principal upstream: `QuantumNous/new-api`

Audited through:
- release boundary: `v1.0.0-rc.24` plus post-release `main`
- commit: `58d4e9bd3bb035df8ea235dd682ccc8a45d0332a`
- audited at: `2026-08-14`

See `UPSTREAM.md` and `UPSTREAM_PORTS.md`.

## Completed initiative

- The complete 0815 correctness and RequestGuard hardening implementation is
  recorded in `tasks/archive/0815-implementation.md`.

## Release readiness

- Local release preparation validation passed for
  `renewapi-v1.0.0-rc.2`.
- Full Go tests, focused race tests, tracked-package vet/build, both frontend
  production builds, and the SQLite RequestGuard migration check passed.
- `CHANGELOG.md`, the upstream ledger, maintenance records, and version
  identity are synchronized for `v1.0.0-rc.2`.

## Known high-priority technical debt

- The existing GitHub workflow historically packaged source-SHA images and
  releases; the updated workflow must be exercised by a real Actions run before
  treating product-tag publishing as operationally proven.
- Docker is unavailable in the current environment, so local image/runtime and
  MySQL/PostgreSQL container migration checks remain publication-time checks.
- Runtime endpoints expose the product version. Git commit, build time, build
  channel, and audited upstream identity remain build/image metadata rather
  than application response fields.

## Compatibility invariants

- Keep the Go module path `github.com/QuantumNous/new-api` for upstream
  compatibility.
- Support SQLite, MySQL >= 5.7.8, and PostgreSQL >= 9.6 through GORM-safe
  migrations and queries.
- Preserve both `web/default` and `web/classic` builds until the documented
  classic-frontend retirement decision changes.
- Keep provider/routing behavior and existing API/data-model compatibility
  unchanged unless a separate feature or bug-fix task explicitly changes it.
- Preserve fork-owned compatibility, anti-poison, security, billing, and
  deployment controls when adapting upstream behavior.

## Remaining publish verification

- No known local implementation blocker remains for the prerelease.
- After explicit authorization, the final committed source must receive the
  exact tag `renewapi-v1.0.0-rc.2`; Hosted Actions must then verify the GHCR
  `1.0.0-rc.2`, `rc`, and `sha-*` tags without moving `latest`.
