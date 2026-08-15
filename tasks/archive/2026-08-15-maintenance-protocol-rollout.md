# RenewAPI Maintenance Protocol Rollout

Status: completed

Created: 2026-08-15
Last updated: 2026-08-15

## Goal

Establish the durable repository memory, independent version policy, upstream
audit records, task handoff structure, and release-channel validation required
by `CODEX_MAINTENANCE_PROTOCOL.md`, adapted to the current RenewAPI checkout.

## Non-goals

- Do not change provider, billing, routing, or security business behavior.
- Do not rewrite historical Git tags or GitHub Releases.
- Do not merge or rebase the unrelated NewAPI history.
- Do not claim a product release until a real tag-matched Actions run succeeds.

## Context

Phase 0 found an existing source-fork architecture, an audited upstream ledger,
a manual source-image workflow, and Docker linker/runtime version injection.
The declared version is `v1.0.0-rc.20-renewapi.1`; later `v1.0.0-rc.21` through
`v1.0.0-rc.24` tags are upstream objects, not RenewAPI product releases.

## Constraints

- compatibility: preserve the Go module path and three supported databases
- architecture: reuse the existing Docker, Go, frontend, and release checks
- database: no schema or migration changes in this rollout
- security: preserve secret handling and immutable source verification
- performance: no runtime hot-path changes
- release: main/edge, SHA, prerelease, stable, and manual source identities
  remain distinct

## Relevant sources

- Code: `Dockerfile`, `common/init.go`, `controller/misc.go`
- Tests: existing Go/frontend/database checks in `build-release.yml`
- ADR: `docs/decisions/001-versioning.md`, `002-upstream-sync-policy.md`,
  `003-build-metadata-release-channels.md`
- Architecture: `docs/ARCHITECTURE.md`
- Upstream: `UPSTREAM.md`, `UPSTREAM_PORTS.md`

## Milestones

### M1 — Repository audit

Status: done

Acceptance criteria:
- [x] actual repository, status, branches, tags, and recent commits inspected
- [x] existing docs, VERSION, upstream ledger, Actions, Docker, and runtime
  version handling inspected

### M2 — Durable memory and policy records

Status: done

Acceptance criteria:
- [x] AGENTS, project state, architecture, maintenance, ADR, task, and
  changelog entry points exist
- [x] current state and upstream baseline are recorded without invented facts

### M3 — Release identity separation

Status: done

Acceptance criteria:
- [x] product version remains separate from build and upstream metadata
- [x] main publishes edge/SHA identities without product Releases
- [x] product tags validate `VERSION == tag`
- [x] prerelease/stable aliases are separated
- [x] manual source packaging remains available

### M4 — Verification and handoff

Status: done

Acceptance criteria:
- [x] scripts pass syntax and targeted checks
- [x] workflow configuration parses successfully
- [x] relevant backend/frontend/build checks pass
- [x] Docker/Compose limitation is documented
- [x] project state is updated and this task is archived

## Current state

Completed:
- Phase 0 audit and version/tag classification.
- Durable repository memory skeleton and ADRs.
- Product release validation scripts.
- Docker build-channel label and local-build product-version defaults.
- Unified workflow event/channel metadata and alias policy.

In progress:
- None.

Remaining:
- A real product-tag Actions run is required before product publishing is
  operationally proven; this is an operational follow-up, not an implementation
  gap in this rollout.

## Decisions made during implementation

- Keep `VERSION` at the declared `v1.0.0-rc.20-renewapi.1` baseline; plan
  `v1.0.0-rc.21` as the next canonical RenewAPI prerelease.
- Treat existing rc.21-rc.24 tags as upstream history, not local releases.
- Reuse the existing quality/build workflow rather than adding a duplicate CI
  test suite.

## Risks / blockers

- GitHub Actions and GHCR behavior cannot be fully proven from this local
  checkout without running the hosted workflow.
- A product tag cannot be created in this maintenance rollout because the
  current VERSION is intentionally unchanged and no release was requested.

## Handoff to next Codex session

Start from:
- `docs/PROJECT_STATE.md`
- this task file
- current `git status` and `.github/workflows/build-release.yml`

Do not redo:
- the Phase 0 tag and upstream classification
- the accepted ADR decisions

Verify first:
- workflow YAML parsing and GitHub expression behavior
- `bash scripts/validate-release.sh` and PowerShell script syntax
- Docker Compose config and existing quality/build checks

## Completion checklist

- [x] implementation complete
- [x] relevant tests pass
- [x] lint/typecheck/build pass
- [x] compatibility reviewed
- [x] ADR updated if necessary
- [x] PROJECT_STATE updated
- [x] CHANGELOG entry point created; no user-visible release entry added
- [x] upstream records preserved and referenced
