# Classic frontend retirement plan

## Decision

`web/default` is the primary administration frontend. `web/classic` enters a
two-to-three release retirement window and receives only security, build, and
critical compatibility fixes during that window. New channel features and new
administration workflows must target `web/default`.

This is a staged retirement, not an immediate removal. Both frontends remain
embedded in the binary until the exit criteria below are met.

## Why the migration is staged

The classic production build currently emits a main entry bundle of about
7.6 MB (about 2.1 MiB gzip), plus multiple chunks over the 500 KiB budget. Its
React 18, Vite 5, Semi UI, and transitive dependency graph also require major
upgrades to clear the remaining security advisories. Reworking that stack in
the same release as gateway and billing changes would create an unnecessarily
large rollback surface.

## Release policy

| Window | Default frontend | Classic frontend |
| --- | --- | --- |
| Release N | All new work; migration fixes | Security and critical compatibility fixes |
| Release N+1 | Default for all new installs | Available as an explicit rollback option |
| Release N+2 | Verify adoption and parity | Remove only if exit criteria are satisfied |

The window may extend by one release when a documented blocker remains. It
must not be extended silently.

## Migration procedure

1. Back up the `options` table and record the current `theme.frontend` value.
2. Set `theme.frontend` to `default` through the system settings API or admin
   UI. Do not update the database with an unreviewed ad-hoc column name.
3. Verify login, channel create/edit/test, key reveal, billing/top-up,
   subscription purchase, user management, logs, statistics, and system
   settings with the deployment's real authentication and routing setup.
4. Observe browser errors, API 4xx/5xx rates, failed mutations, and support
   reports for at least one normal operating cycle.
5. Keep the same backend build and set `theme.frontend` back to `classic` if a
   blocking regression appears. This switch is the supported rollback; do not
   roll back database migrations solely to change the frontend.

## Exit criteria

Classic assets can be removed only when all of the following are true:

- default has been the production theme for at least two releases;
- no P0/P1 workflow depends on classic-only behavior;
- migration acceptance has been completed on SQLite, MySQL, and PostgreSQL
  deployments where those databases are supported;
- remaining classic dependency advisories are either eliminated by removal or
  documented as non-runtime build-tool exposure;
- the rollback runbook and release notes identify the last image that embeds
  classic;
- CI no longer assumes `web/classic/dist` exists before the Go embed directive
  and Docker build are changed in the same removal commit.

## Ownership rules during the window

- Security fixes that affect shared browser boundaries must be applied to both
  frontends while classic ships.
- Product behavior is implemented in default first. Classic receives no new
  provider forms, channel capabilities, or dashboard features.
- A classic-only production defect may be fixed when migration is not yet a
  viable incident response, but the fix must not introduce a new abstraction
  shared with default.
- Bundle size and dependency audit results are recorded per release so the
  removal decision is based on evidence rather than elapsed time alone.
