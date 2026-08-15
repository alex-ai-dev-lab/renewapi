# ADR-003: Build Metadata and Release Channels

Status: Accepted

Date: 2026-08-15

## Context

The existing Docker build already injects a runtime version and labels images
with commit, build time, and upstream metadata, but the manual workflow also
used source-SHA tags as release identities. The maintenance policy requires
edge, immutable SHA, prerelease, and stable product identities to stay distinct.

## Decision

Use `VERSION` for the pure RenewAPI product version. Product Git tags use
`renewapi-v<version>`; raw upstream `v*` tags are never product triggers. Keep
Git commit, build time, build channel, and audited upstream ref as independent
build metadata. The workflow publishes `edge` and `sha-*` for `main`,
product-tagged images for prereleases/stables, and source-image packaging only
for explicit manual runs.

Only stable `renewapi-vX.Y.Z` product tags may update `latest` aliases.
Prerelease and SHA builds must not update stable aliases. Product Docker tags
strip the leading `v`, while GitHub Release names retain it.

## Rationale

This preserves reproducible deployment identities without changing runtime
business behavior or deleting historical source-image releases.

## Compatibility constraints

- Existing `VERSION` environment overrides remain supported.
- Existing manual source-package inputs remain supported.
- Existing Docker labels and artifact verification remain in the workflow.

## Validation

Use workflow YAML parsing, release-script tests, Docker Compose config checks,
and the existing Go/frontend/database quality jobs.

## Supersedes / Superseded by

- None
