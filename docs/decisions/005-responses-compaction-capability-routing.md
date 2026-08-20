# ADR-005: Route-Scoped Responses Compaction Capability

Status: accepted

Date: 2026-08-20

## Decision

Responses compaction capability is evaluated per `(channel, model, route
fingerprint)` and per protocol facet. Ordinary Responses requests remain
outside the compaction gate. Strict mode treats `unknown` as unverified and
does not route it as supported.

Observed evidence is valid only when its `ResponsesObservedRouteFingerprint`
matches the current channel route, mapping, and configuration version. A
future `NextProbeAt` is therefore a backoff for that exact route, not a global
lock. Route changes immediately permit a new probe.

The probe matrix records legacy compact, native compact, native SSE, and
continuation independently. Continuation success cannot promote native
compaction support. Transient and auth/config failures remain non-support
evidence, while generic invalid synthetic probe requests do not create
capability evidence.

Distributor route-plan failures use a stable
`responses_compaction_no_eligible_channel` error code and redacted reason
counts. The existing root-authenticated capability probe endpoint is the
operational Force Probe entry point; the ordinary Channel Test remains a
single endpoint/stream test and is not described as a full compaction probe.

## Consequences

- Production bootstrap must establish evidence with the scheduler or Force
  Probe before strict compaction routing can succeed.
- Third-party ordinary `/responses` success does not imply compaction support.
- No schema or migration change is required; existing capability rows and
  fields are reused.
- Real upstream validation remains an environment-dependent operational step.
