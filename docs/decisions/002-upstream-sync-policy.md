# ADR-002: Selective Upstream Synchronization

Status: Accepted

Date: 2026-08-15

## Context

RenewAPI and `QuantumNous/new-api` have unrelated Git histories. The fork also
owns compatibility, security, billing, routing, and deployment behavior that
cannot be assumed to match upstream changes.

## Decision

NewAPI is the principal upstream/reference source. RenewAPI does not merge or
rebase the unrelated history. Each candidate change is reviewed by intent,
compared with the local implementation, selectively ported or adapted, tested,
and recorded in `UPSTREAM_PORTS.md`.

Sub2API, CPA, and other projects are reference sources only. They require the
same behavior-first adaptation and must not be copied wholesale.

## Rationale

Behavior-focused ports preserve local security and compatibility boundaries and
leave a reviewable audit trail.

## Compatibility constraints

- Keep the audited baseline and review date in `UPSTREAM.md`.
- Preserve fork-owned behavior listed in `UPSTREAM_PORTS.md`.
- Advance the audited ref only after the complete candidate range is
  classified.

## Alternatives considered

### Merge or rebase NewAPI

Rejected because the histories are unrelated and the conflict surface is
large.

### Copy external modules wholesale

Rejected because provider, billing, routing, persistence, and security
contracts differ.

## Consequences

Positive:
- Local behavior remains explicit and reviewable.
- Upstream audits can be resumed from a durable baseline.

Negative:
- Selective porting requires more review and maintenance work.

## Validation

Run `scripts/check-upstream.ps1` or `scripts/check-upstream.sh`, focused tests,
and record every disposition in `UPSTREAM_PORTS.md`.

## Supersedes / Superseded by

- None
