# RenewAPI Architecture

## Product boundary

RenewAPI is a downstream source fork of NewAPI. The Go module path and core
API/data model remain upstream-compatible, while compatibility bridges, routing
hardening, billing safeguards, anti-poison controls, and deployment tooling
are fork-owned. NewAPI, Sub2API, and CPA changes are reviewed as references;
they are not merge sources.

## Runtime layers

```text
HTTP router
  -> controller
  -> service
  -> model/GORM
  -> SQLite, MySQL, or PostgreSQL
```

Relay traffic follows the same route/controller boundary, then enters the
provider adapter pipeline under `relay/`. Shared concerns live in
`middleware/`, `common/`, `setting/`, `dto/`, `types/`, and `pkg/`.

## Fork-owned surfaces

- `pkg/compat/`: compatibility hooks, error normalization, schedulers, and
  price synchronization.
- `relay/antipoison/`: channel risk profiles, probes, envelope checks, opaque
  payload scanning, and tool-call guards.
- `service/requestguard/`: request admission and audit persistence.
- `scripts/`: upstream audit, build, release, deploy, rollback, and secret
  loading helpers.
- `web/default/` and `web/classic/`: the primary and compatibility frontend
  bundles embedded by the Docker build.

## Version and build metadata

`VERSION` is the RenewAPI product version and is injected into the frontend
build and `common.Version` through the existing Docker linker path. The
runtime accepts a `VERSION` environment override for compatibility. `/api/status`
and `/healthz` expose this product version.

RenewAPI product Git tags use `renewapi-v<version>`; raw upstream `v*` tags are
not product releases. Docker product tags strip the leading `v`.

Git commit, build time, build channel, and audited upstream reference are
separate Docker image labels and release-note metadata. They must not be
encoded into the product version.

## Data and compatibility boundaries

- All three supported databases are first-class compatibility targets.
- Provider adapters should use shared relay/request conversion and error
  normalization paths rather than duplicating routing logic.
- Database migrations and concurrency-sensitive updates require focused tests
  on SQLite plus the relevant external database checks.
- Feature flags and existing defaults remain stable unless an explicit
  compatibility decision is recorded in an ADR.
