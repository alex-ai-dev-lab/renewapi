# RenewAPI Product Tag Namespace Correction

Status: completed

Created: 2026-08-15
Last updated: 2026-08-15

## Goal

Prevent raw upstream `v*` tags from colliding with RenewAPI product releases
by separating pure `VERSION` values from `renewapi-`-prefixed product Git tags.

## Non-goals

- Do not rewrite or delete historical upstream tags or releases.
- Do not change provider, routing, billing, security, or persistence behavior.
- Do not create or push a product tag in this maintenance task.

## Context

The repository contains upstream tags `v1.0.0-rc.21` through `v1.0.0-rc.24`.
The previous declared value `v1.0.0-rc.20-renewapi.1` was a hybrid version and
not a pure product version. The next monotonic product version is normalized to
`v1.0.0-rc.21`, with exact Git tag `renewapi-v1.0.0-rc.21`.

## Constraints

- compatibility: preserve runtime `VERSION` handling and Docker metadata
- release: raw `v*` tags must not trigger this workflow
- Docker: prerelease exact tag strips leading `v`, plus `rc` and SHA only
- history: all existing tags remain untouched

## Milestones

### M1 — Namespace audit

Status: done

- [x] Confirm upstream raw tags occupy the shared namespace.
- [x] Confirm no current RenewAPI product tag requires migration.

### M2 — Mechanism correction

Status: done

- [x] Normalize `VERSION` to pure `v1.0.0-rc.21`.
- [x] Require `renewapi-v*` in workflow triggers and validators.
- [x] Keep Docker tags version-only without the leading `v`.

### M3 — Verification and handoff

Status: done

- [x] Validate YAML, scripts, tag/version parsing, tests, and builds.
- [x] Archive this task and update current project state.

## Current state

Completed:
- Version/tag namespace implementation and documentation updates.

Remaining:
- Local Docker/Compose execution and hosted Actions behavior require an
  environment with Docker and GitHub Actions.

Validation:
- `go test ./...`
- `bun run typecheck` in `web/default`
- `bun run build` in `web/default` and `web/classic`
- workflow YAML parsing
- PowerShell and Git Bash syntax checks
- raw `v1.0.0-rc.21` rejected; `renewapi-v1.0.0-rc.21` parsed and stopped at
  missing tag as expected

## Handoff

Next canonical product version: `v1.0.0-rc.21`
Exact product Git tag: `renewapi-v1.0.0-rc.21`
