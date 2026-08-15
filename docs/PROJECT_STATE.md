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

Release commit:
`f0f8e6ff034906ebb62a4cd752a47f1cca45d55c`
(`release: prepare RenewAPI v1.0.0-rc.2`)

The published annotated tag `renewapi-v1.0.0-rc.2` resolves to this release
commit and must not be moved.

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

## Published prerelease

- Local release preparation validation passed for
  `renewapi-v1.0.0-rc.2`.
- Full Go tests, focused race tests, tracked-package vet/build, both frontend
  production builds, and the SQLite RequestGuard migration check passed.
- `CHANGELOG.md`, the upstream ledger, maintenance records, and version
  identity are synchronized for `v1.0.0-rc.2`.
- GitHub Actions run `31881289216` passed the quality, MySQL/PostgreSQL
  migration, multi-arch image, release-asset, and tag-source verification
  gates.
- GitHub Release `RenewAPI v1.0.0-rc.2` is published as a prerelease targeting
  the release commit.
- GHCR tags `1.0.0-rc.2`, `rc`, and `sha-f0f8e6ff0349` resolve to manifest
  digest `sha256:9238b7cfcf842e754872fe78f605187038245ab2722ad26deb57f38fd57cf5fd`.
- Stable `latest` remains on the older `sha-4a0d431e5d58` image at digest
  `sha256:25796ecdec7a77c501bea1e9a4a6502d3a29d78025c5d975d889b0be9c8ae946`.

## Known high-priority technical debt

- Docker is unavailable in the current environment, so local image runtime was
  not exercised. The hosted multi-arch build and MySQL/PostgreSQL migration
  checks passed during publication.
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
