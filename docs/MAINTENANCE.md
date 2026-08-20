# RenewAPI Maintenance Workflow

This document is the operational entry point for future Codex sessions. The
repository is the durable memory; prior conversation is not authoritative.

## Start a substantial task

1. Run `git status`, inspect the branch and recent relevant history, and keep
   unknown user changes intact.
2. Read `AGENTS.md` and `docs/PROJECT_STATE.md`.
3. Open the matching file under `tasks/active/` when one exists.
4. Read only the relevant architecture, ADR, upstream, and development docs.
5. Inspect implementation and tests before editing.

Use a task document for work spanning modules, sessions, database migrations,
provider/routing/security changes, large upstream audits, or releases. Promote
long-lived architecture, compatibility, security, routing, upstream, and
version decisions to `docs/decisions/`.

## Upstream review

`UPSTREAM.md` records the audited NewAPI baseline. `UPSTREAM_PORTS.md` records
the disposition of each reviewed behavior. The fork has no common Git
ancestor with NewAPI, so use `scripts/check-upstream.ps1` or
`scripts/check-upstream.sh`; never merge or rebase the unrelated history.

For every candidate change: identify intent, inspect the RenewAPI equivalent,
classify it, adapt only the required behavior, test it, and record the local
reference and reason.

## Version and release policy

- `VERSION` contains only the pure RenewAPI product version, such as
  `v1.0.0-rc.1`.
- RenewAPI product Git tags use `renewapi-v<version>`, such as
  `renewapi-v1.0.0-rc.1`. Raw upstream `v*` tags never trigger product
  releases.
- `main` builds publish `edge` and immutable `sha-<short-sha>` images without
  creating a product Release.
- `renewapi-vX.Y.Z-rc.N` tags create prereleases and must not update stable
  `latest`.
- `renewapi-vX.Y.Z` tags create stable Releases and may update `latest`,
  `major`, and `minor` aliases after validation.
- Historical tags and Releases are never rewritten.
- Product-tag validation requires
  `stripPrefix(gitTag, "renewapi-") == VERSION`, source checkout integrity,
  relevant tests, build checks, and migration checks. See
  `scripts/validate-release.*` and `.github/workflows/build-release.yml`.

Before creating a product tag, validate the prepared identity with
`scripts/validate-release.ps1 -Prepare -Tag renewapi-vX.Y.Z` or
`scripts/validate-release.sh --prepare renewapi-vX.Y.Z`. The default validator
mode remains the post-tag gate and additionally requires the tag to resolve to
the checked-out clean HEAD.

The current canonical RenewAPI prerelease is `v1.0.0-rc.3`, using the exact
Git tag `renewapi-v1.0.0-rc.3`. It advances the independent RenewAPI rc
sequence established at `rc.1` while leaving upstream raw `v*` tags untouched.

## Finish and hand off

Run relevant format, lint, typecheck, tests, build, migration, Docker, and
workflow-static checks. Update the task, `PROJECT_STATE.md`, ADRs, upstream
records, and `CHANGELOG.md` only when their facts changed. Archive a completed
task only after validation passes; leave a concise handoff for blocked work.
