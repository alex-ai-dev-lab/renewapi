# 0815 implementation

Status: complete

Created: 2026-08-15
Last updated: 2026-08-15

## Goal

Implement the complete behavior described by `D:\Code\renewapi\0815.md`
against the current RenewAPI architecture, validate the full change set, and
prepare exactly one next prerelease version when all release gates pass.

## Non-goals

- Do not create or push a Git product tag.
- Do not push commits or create a GitHub Release in this task.
- Do not update the stable Docker `latest` alias.
- Do not restore, discard, or stage unrelated user-owned worktree changes.
- Do not merge or rebase unrelated NewAPI history.

## Constraints

- Preserve SQLite, MySQL, and PostgreSQL compatibility.
- Preserve existing API, configuration, database, provider, and routing
  defaults unless the specification explicitly changes them.
- Keep generic custom OAuth bindings in `user_oauth_bindings`.
- Keep RequestGuard work bounded, fail-closed behavior intact, and disabled
  mode free of scanner, recorder, network, and database work.
- Preserve both default and classic frontend builds.
- Treat all pre-existing dirty worktree entries as user-owned.
- Change `VERSION` only once, after all implementation and validation gates
  pass. Otherwise leave it unchanged.

## Relevant sources

- Specification: `D:\Code\renewapi\0815.md`
- Code: `controller/`, `middleware/`, `model/`, `router/`, `service/requestguard/`
- Frontend: `web/default/`, `web/classic/`
- Architecture: `docs/ARCHITECTURE.md`
- Maintenance: `docs/MAINTENANCE.md`
- ADRs: `docs/decisions/001-versioning.md`,
  `docs/decisions/002-upstream-sync-policy.md`,
  `docs/decisions/003-build-metadata-release-channels.md`
- Upstream ledger: `UPSTREAM_PORTS.md`

## Milestones

### M1 - Worktree and source audit

Status: complete

Acceptance criteria:
- [x] Current branch, HEAD, origin relation, dirty files, and relevant code are
  verified.
- [x] Existing partial implementations are identified before editing.

### M2 - Account binding and critical route limits

Status: complete

Acceptance criteria:
- [x] Built-in OAuth and WeChat binding update only the intended column.
- [x] Binding columns are strictly whitelisted.
- [x] `/api/user/token` and `/api/user/aff_transfer` have isolated per-user
  critical rate limits.
- [x] Focused backend tests pass.

### M3 - Redemption edit precision

Status: complete

Acceptance criteria:
- [x] Default and classic edit flows preserve exact raw quota when unchanged.
- [x] Edited and create flows still recompute quota.
- [x] Focused frontend tests pass.

### M4 - RequestGuard hardening

Status: complete

Acceptance criteria:
- [x] Responses and Claude tool output extraction is bounded and rune-safe.
- [x] Runtime insertion tests prove blocked work stops before routing, billing,
  channel selection, and upstream handling.
- [x] Observe workers start with the process, resize, and stop cleanly.
- [x] RequestGuard tests and race tests pass.

### M5 - Regression coverage

Status: complete

Acceptance criteria:
- [x] HTTP replay concurrency, multipart, and disk-backed cases are covered.
- [x] Protected fetch rejects a public redirect to a private target.
- [x] Billing concurrency and exactly-once invariants are covered.

### M6 - Persistent state and release readiness

Status: complete

Acceptance criteria:
- [x] Upstream dispositions reflect the implemented behavior.
- [x] `CHANGELOG.md` and `docs/PROJECT_STATE.md` reflect current facts.
- [x] All required format, lint, typecheck, test, build, migration, release,
  and diff checks pass or are explicitly recorded as unavailable.
- [x] `VERSION` advances exactly once only if every release gate passes.
- [x] This task is archived only after successful validation.

## Current state

Completed:
- Read the repository maintenance, architecture, versioning, upstream, and
  release-policy documents.
- Confirmed the current branch is
  `fix/routing-rate-limit-channel-edit-20260814` at `4f3eeeb9c` before fetch.
- Confirmed extensive pre-existing dirty maintenance/release changes and
  `.agents/**` deletions that must remain untouched.
- Confirmed the repository-local `tasks/active/0815.md` is an older 0814 task
  record; the external `D:\Code\renewapi\0815.md` is the implementation spec.
- Added a whitelisted single-column user binding update and applied it to
  built-in OAuth and WeChat binding without changing generic OAuth storage.
- Added isolated per-user critical limits to token generation and affiliate
  quota transfer.
- Passed focused M2 Go tests for model, OAuth, middleware, controller, and
  router packages.
- Preserved exact raw redemption quota on untouched edits in both frontends;
  classic edits also distinguish display-amount changes from native-quota
  changes.
- Passed focused redemption tests and the default frontend typecheck.
- Added streaming, bounded RequestGuard extraction for Responses and Claude
  tool outputs while excluding binary media payloads.
- Replaced source-order assertions with runtime relay counters proving guard
  block and fail-closed errors stop route planning, pricing, preconsume,
  channel selection, and upstream dispatch.
- Registered a startup-owned observe worker manager with dynamic resize and
  bounded shutdown drain.
- Passed focused RequestGuard, controller insertion, main lifecycle, and
  RequestGuard race tests.
- Added concurrent replay readers, parsed multipart replay, and large
  disk-backed file-reader coverage without changing replay production code.
- Added a real `http.Client.Do` redirect test proving a public URL cannot
  redirect protected fetches to a private target or trigger a second
  transport call.
- Added concurrent billing regressions for account updates, task refund token
  usage, exactly-once recharge credit, and fallback repricing with the final
  route group.
- Updated the upstream ledger, changelog, project state, maintenance guidance,
  and README release identity.
- Advanced `VERSION` exactly once from `v1.0.0-rc.1` to
  `v1.0.0-rc.2` after the implementation and validation gates passed.
- Validated the prepared tag identity as `renewapi-v1.0.0-rc.2` without
  creating the tag.

## Decisions made during implementation

- The specification's commit, push, GitHub Release, and GHCR verification
  phase is superseded by the current user instruction to stop at a locally
  release-ready prerelease state.
- Use a separate task file so the older 0814 task record remains intact.
- No new ADR was required because the implementation follows the accepted
  architecture, upstream-sync, and release-channel decisions.

## Remaining risks

- Docker is unavailable locally, so image build/runtime validation was not run.
- MySQL and PostgreSQL container migration checks were not run locally; the
  SQLite migration check passed and this iteration adds no new migration.
- Hosted GitHub Actions, GHCR aliases, and the GitHub prerelease remain
  unverified until a later explicitly authorized publish.
- `go mod tidy -diff` reports only whole-file CRLF/LF differences on Windows;
  `go mod verify` passes and `go.mod`/`go.sum` have no content diff.
- Bash is unavailable because WSL has no installed distribution, so the shell
  validator was not parsed locally; the PowerShell validator and YAML parse
  passed.

## Handoff

- Release identity: `v1.0.0-rc.2` / `renewapi-v1.0.0-rc.2`.
- Expected Docker tags: `1.0.0-rc.2`, `rc`, and `sha-*`; do not update
  `latest`.
- Do not create or push the product tag, publish images, or create a GitHub
  Release without explicit authorization.
- Preserve the unrelated `.agents/**` deletions and all other user-owned dirty
  worktree changes.

## Validation

- `go test ./model ./oauth ./middleware ./controller ./router`: pass.
- `bun test src/features/redemption-codes/lib/redemption-form.test.ts`: pass.
- `bun run typecheck` in `web/default`: pass.
- `bun test src/components/table/redemptions/redemption-form.test.js` in
  `web/classic`: pass.
- `go test ./service/requestguard`: pass.
- `go test ./controller -run 'TestRequestGuard' -count=1`: pass.
- `go test .`: pass.
- `go test -race -count=1 ./service/requestguard`: pass.
- `go test ./common ./relay/common ./relay/channel ./service ./model ./controller`: pass.
- `go test -race -count=1 ./common ./relay/common`: pass.
- Focused `go test -race` for protected-fetch/task-refund service invariants:
  pass.
- Focused `go test -race` for concurrent account and recharge model
  invariants: pass.
- `go test -count=1 ./...`: pass.
- CI-focused service/controller and relay-channel race suites: pass.
- Changed-file `gofmt`, tracked-package `go vet`, and tracked-package
  `go build`: pass.
- `go mod verify`: pass.
- `go mod tidy -diff`: Windows-only CRLF/LF diff; no module content change.
- `go test -count=1 -run TestMigrateRequestGuardEventsSQLite ./model`: pass.
- Default frontend full `bun test`: 52 passed; typecheck, changed-file ESLint,
  copyright, Prettier, and production build: pass.
- Classic frontend focused tests: 4 passed; changed-file ESLint, Prettier, and
  production build: pass.
- Workflow and Compose YAML parse: pass.
- PowerShell release-validator AST parse: pass.
- `powershell -File scripts/validate-release.ps1 -Prepare -Tag renewapi-v1.0.0-rc.2`:
  pass.
- `git diff --check`: pass.
- Docker runtime, MySQL/PostgreSQL container checks, shell validator, Hosted
  Actions, GHCR, and GitHub Release: unavailable or intentionally not run as
  recorded above.

## Completion checklist

- [x] implementation complete
- [x] relevant tests pass
- [x] lint/typecheck/build pass
- [x] compatibility reviewed
- [x] ADR updated if necessary
- [x] PROJECT_STATE updated
- [x] CHANGELOG updated
- [x] upstream records updated
- [x] release validator passes
- [x] version decision recorded

## Version decision

Previous version: `v1.0.0-rc.1`

New version: `v1.0.0-rc.2`

Future Git tag: `renewapi-v1.0.0-rc.2`

The version advanced because all implementation milestones and locally
available release gates passed. Stable `latest` remains unchanged.
