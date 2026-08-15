# ADR-001: Independent RenewAPI Versioning

Status: Accepted

Date: 2026-08-15

## Context

The repository contains imported NewAPI release tags alongside RenewAPI fork
commits. Raw upstream `v1.0.0-rc.21` through `v1.0.0-rc.24` already occupy the
shared Git tag namespace, and the previous declared `VERSION`
`v1.0.0-rc.20-renewapi.1` was not a pure product version.

## Decision

RenewAPI uses an independent SemVer product version in `VERSION`. Product
versions do not contain upstream versions, Git SHAs, build IDs, or external
project suffixes. The first independent RenewAPI product version is
`v1.0.0-rc.1`.

RenewAPI product Git tags use the namespace prefix `renewapi-`; the exact next
tag is `renewapi-v1.0.0-rc.1`. Validation is:

```text
stripPrefix(gitTag, "renewapi-") == VERSION
```

Raw upstream `v*` tags never trigger RenewAPI product releases. Historical
tags, releases, and upstream refs remain untouched; no migration rewrites
history or silently announces a stable release.

## Rationale

Independent numbering communicates RenewAPI compatibility and product change,
while the prefix avoids collisions with imported upstream tags. Upstream audits
and immutable builds remain separately traceable.

## Compatibility constraints

- Existing images and runtime consumers may continue to use the current
  `VERSION` environment override.
- Existing upstream `v1.0.0-rc.*` tags remain reference history only.
- Product Docker tags strip the leading `v`; GitHub Release names retain it as
  `RenewAPI vX.Y.Z`.
- A stable `v1.0.0` requires a separate release decision and compatibility
  review.

## Alternatives considered

### Continue using raw RenewAPI `v*` tags

Rejected because raw `v*` tags collide with imported upstream release tags.

### Continue the upstream rc sequence

Rejected because the first product tag has not been published, so there is no
compatibility cost to starting an independent RenewAPI rc sequence at `rc.1`.

## Consequences

Positive:
- Product compatibility and upstream audit state are unambiguous.
- Release automation can validate the prefixed tag/version relationship
  without touching upstream tags.

Negative:
- Git tags and product versions have distinct visible forms, so release
  tooling must display both correctly.

## Validation

Validate `stripPrefix(gitTag, "renewapi-") == VERSION` with
`scripts/validate-release.*` and the product-tag path in
`.github/workflows/build-release.yml`.

## Supersedes / Superseded by

- None
