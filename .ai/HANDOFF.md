# Supervisor Handoff — renewapi-full-audit-0830

## Mission
Systematic full-source audit of `alex-ai-dev-lab/renewapi` on `main`. The GitHub repository and current `main` branch are the sole source of truth. Confirm real defects before changing code; fix confirmed P0/P1/P2 issues with focused regression validation and continue until major source areas and critical call chains are covered.

## Baseline
- Initial audit HEAD: `7d27402590f5ace20001f3f1e9d0b916ed2d4336`.
- Current audited lineage before this handoff update: `7698d8a13cfd883ee1ebb19231452c982a157f89`.
- Actual `VERSION`: `v1.0.0-rc.4`.
- Repo rules read: `AGENTS.md`, `docs/PROJECT_STATE.md`, `docs/ARCHITECTURE.md`, `docs/MAINTENANCE.md`.
- Local `git clone` is unavailable in the current worker environment because `github.com` DNS resolution fails; native GitHub reads/writes remain available and are authoritative.

## Audited Modules
- Repository governance and maintenance documentation: initial pass.
- Process bootstrap, worker lifecycle, graceful HTTP shutdown and resource-close ordering in `main.go` / `lifecycle.go`.
- `/v1` relay route registration and model-list auth coverage in `router/relay-router.go` / `router/relay_router_test.go`.
- Token authentication and channel/model distribution in `middleware/auth.go` and `middleware/distributor.go`: initial end-to-end relay pass.
- Core `controller/relay.go` flow: request validation, token estimation, sensitive-word rejection, preflight/pricing/preconsume, route plan, retry/fallback, response-commit guards, failure refund/violation handling: initial high-risk pass.
- Billing core: `service/billing.go`, `billing_mode.go`, `billing_session.go`, `billing_reconciler.go`, `funding_source.go`, and `model/billing_ledger.go`: initial data-consistency pass.
- `bytedance/gopkg v0.1.3` global gopool lifecycle semantics used by async refund paths.
- `.github/workflows/build-release.yml`: initial quality-matrix pass; remaining workflow/release sections still need full review.

## Unaudited Modules
- Remaining relay/provider adapters and complete protocol-conversion matrix, including all streaming/tool-call/retry/timeout/fallback implementations.
- Remaining backend controllers, middleware, services, models, migrations, cache and transaction paths across SQLite/MySQL/PostgreSQL.
- Full permissions/security surface, OAuth/passkey/webhook/file/download/requestguard paths.
- Production and classic frontend source, state management, API integration, responsive/loading/empty/error/duplicate-submit UX.
- Configuration/environment variable matrix and startup edge cases.
- Docker/compose/deployment assets.
- Remaining GitHub Actions workflows and full release/build sections.
- Second-pass high-risk review and final repository omission scan.

## Confirmed Issues
- **P2 — sensitive-word rejection misclassified as server failure.** When request-sensitive-word checking matched user input, `controller/relay.go` called `types.NewError(nil, ErrorCodeSensitiveWordsDetected)`. The generic constructor defaulted to HTTP 500 and retryable semantics even though the route outcome was explicitly `ClientRejected`.
- **P1 — failed-request refund could be lost during graceful shutdown in shadow/off billing.** In non-enforced billing, actual wallet/subscription/token refunds run in the global gopool. Prior shutdown waited HTTP handlers and explicit background workers but not that pool, then closed DB resources. A failed precharged request followed by process shutdown could therefore leave the real balance deducted even though the shadow ledger had already been marked refunded.

## Fixed Issues
- **P2 fixed:** commit `8f210fd8c7fcd627a32657848a932829b0bbbb69` makes `ErrorCodeSensitiveWordsDetected` default to HTTP 400 and skip retry; commit `9d3755b0198195f67679fb5a0393e561a5089143` adds regression coverage for status/retry/code/message semantics.
- **P1 fixed:** commit `207c20a47529a7da57c3b0abcee8d0098eebfe1f` drains the global gopool inside `backgroundWorkers.Stop` before DB shutdown, bounded by the existing shutdown context; commit `7698d8a13cfd883ee1ebb19231452c982a157f89` adds a regression test proving Stop waits for a real blocked gopool task and still honors deadlines.

## Pending Verification
- Non-enforced `BillingSession.Refund()` sets `refunded=true` before the actual wallet/token compensation runs. Wallet refund is explicitly non-idempotent (`quota += N`) and therefore cannot be naively retried; token semantics also need review. Determine whether existing request-keyed transaction/outbox primitives can provide durable exactly-once compensation before classifying/fixing this separate failure mode.
- `BillingSession.Settle()` can finish funding adjustment and then fail token quota adjustment; verify whether current accounting/reconciliation deliberately tolerates that split or can leave unrecoverable inconsistent balances.
- Shared shutdown timeout is used for HTTP drain, worker/gopool drain and later flushes; verify whether exhausted budget can skip durability work beyond intended forced-shutdown semantics.
- `docs/PROJECT_STATE.md` and `docs/MAINTENANCE.md` version claims differ; `VERSION` is authoritative at `v1.0.0-rc.4`. Classify only after checking intended doc contract.
- Analytics HTML interpolation in `main.go` uses deployment environment values; verify trust boundary before classifying.
- Latest quality workflow for the code lineage was still pending at last check; re-check after this handoff-only commit and inspect failing jobs if any.

## Current Call Chain
`/v1` relay route -> TokenAuth/rate limits -> Distribute/model+channel plan -> `controller.Relay` -> request validation/compat/preflight/pricing -> `PreConsumeBilling` -> provider/relay retry/stream response -> settle on success or refund on terminal failure -> async non-enforced funding/token compensation -> shutdown lifecycle/gopool drain -> DB/resource close.

Current focus is the exactly-once/durable compensation boundary for wallet/token refunds, then provider/streaming/tool-call protocol conversion and retry/timeout/fallback paths.

## Validation
- Every code write so far was preceded by a current-HEAD check and followed by commit-diff inspection; no unintended whole-file replacement changes were observed.
- Added focused unit regressions for sensitive-word error semantics and gopool shutdown draining.
- Full GitHub Actions validation is not yet claimed; current/latest runs must be inspected to completion and failures repaired.

## Next Audit Scope
1. Trace `IncreaseUserQuota` / token quota mutation and existing outbox/idempotency primitives to determine whether refund failures can be recovered safely.
2. Inspect latest GitHub Actions jobs/results for the current `main` lineage and repair any regressions.
3. Continue `relay` + provider adapters + stream/tool/fallback/recovery services end-to-end.
4. Audit model/database/migrations/cache/concurrency across all supported databases.
5. Audit remaining auth/security/controllers/services, then both frontends, config/deployment/Docker and all workflows.
6. Re-run a high-risk second pass and full repository omission scan before declaring mission complete.

## Notes for Supervisor
`.ai/STATE.json` is not worker-authored authority and is intentionally not used as audit state. Durable decisions, if any, belong in `.ai/DECISIONS.md`; transient findings and coverage state remain here.
