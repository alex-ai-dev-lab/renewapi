# Supervisor Handoff — renewapi-full-audit-0830

## Mission
Systematic full-source audit of `alex-ai-dev-lab/renewapi` on `main`. The GitHub repository and current `main` branch are the sole source of truth. Confirm real defects before changing code; fix confirmed P0/P1/P2 issues with focused regression validation and continue until major source areas and critical call chains are covered.

## Baseline
- Initial audit HEAD: `7d27402590f5ace20001f3f1e9d0b916ed2d4336`.
- Current code lineage before this handoff update: `1b8134c8b5f24b723b78f64c6fca61241cfadf13`.
- Actual `VERSION`: `v1.0.0-rc.4`.
- Repo rules read: `AGENTS.md`, `docs/PROJECT_STATE.md`, `docs/ARCHITECTURE.md`, `docs/MAINTENANCE.md`.
- Local `git clone` is unavailable in the current worker environment because `github.com` DNS resolution fails; native GitHub reads/writes remain available and are authoritative.

## Audited Modules
- Repository governance and maintenance documentation: initial pass.
- Process bootstrap, worker lifecycle, graceful HTTP shutdown and resource-close ordering in `main.go` / `lifecycle.go`.
- `/v1` relay route registration and model-list auth coverage in `router/relay-router.go` / `router/relay_router_test.go`.
- Token authentication and channel/model distribution in `middleware/auth.go` and `middleware/distributor.go`: initial end-to-end relay pass.
- Core `controller/relay.go` flow: request validation, token estimation, sensitive-word rejection, preflight/pricing/preconsume, route plan, retry/fallback, fallback repricing/reservation, response-commit guards, session recovery, failure refund/violation handling: initial high-risk pass.
- Billing core: `service/billing.go`, `billing_mode.go`, `billing_session.go`, `billing_reconciler.go`, `funding_source.go`, `model/billing_ledger.go`, `model/billing_outbox.go`, wallet/token quota mutation and subscription preconsume/reserve/refund: initial data-consistency pass.
- Request-scoped subscription fallback reservation path now traced end-to-end from `controller.Relay` repricing -> `BillingSettler.Reserve` -> funding/token reservation -> rollback -> terminal deferred refund.
- `bytedance/gopkg v0.1.3` global gopool lifecycle semantics used by async refund paths.
- Shared relay outbound/stream layer: `relay/channel/adapter.go`, `relay/channel/api_request.go`, `relay/helper/common.go`, OpenAI compatible and Claude handler entry paths. Body replay metadata, request-context binding, header override/passthrough and SSE keepalive/write coordination received an initial high-risk pass.
- Router/stream recovery: `service/stream_recovery.go`, `service/session_route_recovery.go`, `service/router_reliability.go` initial pass. Current-request retries are suppressed after client commitment, and state-bound Responses requests are guarded from cross-channel failover.
- HTTP timeout/config path: `common/init.go`, `service/http_client.go`, `.env.example` initial pass. `RELAY_TIMEOUT` is a per-upstream-attempt `http.Client.Timeout`; no repository contract currently establishes it as a total multi-attempt routing budget. `RELAY_FIRST_BYTE_TIMEOUT` is initialized but currently appears unused and undocumented, so it is tracked as dead-config/design-debt candidate rather than a confirmed functional bug.
- `.github/workflows/build-release.yml`: initial quality-matrix pass. A prior audited lineage passed database-quality on MySQL/PostgreSQL plus both frontend builds and backend quality. Current latest lineage still requires completion verification.

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
- **P1 — SSE ping keepalive could write concurrently with business stream data.** `relay/channel/api_request.go` runs keepalive pings in an independent gopool goroutine while provider response handling writes business SSE on the request goroutine. `helper.PingData`, `StringData`/`ResponseChunkData`, and Claude event helpers all wrote/flushed the same Gin `ResponseWriter` without a shared lock. The pinger-local mutex only serialized pings with other pings, leaving real concurrent ResponseWriter access and possible races/interleaved frames/writer-state corruption.
- **P1 — subscription fallback reserve could lose an extra reservation after a torn token-reserve rollback.** Fallback repricing calls `BillingSettler.Reserve`. The subscription branch previously incremented `UserSubscription.AmountUsed` directly without updating the request-id `SubscriptionPreConsumeRecord`; if token reserve then failed and the immediate subscription rollback also failed, `extraReserved` had not yet been recorded and terminal `RefundSubscriptionPreConsume(requestId)` knew only the initial reservation. The extra subscription charge could therefore survive a failed request.

## Fixed Issues
- **P2 fixed:** commit `8f210fd8c7fcd627a32657848a932829b0bbbb69` makes `ErrorCodeSensitiveWordsDetected` default to HTTP 400 and skip retry; commit `9d3755b0198195f67679fb5a0393e561a5089143` adds regression coverage for status/retry/code/message semantics.
- **P1 fixed:** commit `207c20a47529a7da57c3b0abcee8d0098eebfe1f` drains the global gopool inside `backgroundWorkers.Stop` before DB shutdown, bounded by the existing shutdown context; commit `7698d8a13cfd883ee1ebb19231452c982a157f89` adds a regression test proving Stop waits for a real blocked gopool task and still honors deadlines.
- **P1 fixed:** commits `0508843f6ee4009e67ac0152507a86fba766d11a` and `9038c197acc9af05e5d062abeb40d351b8f3af61` add a per-request shared stream-write mutex around ping, OpenAI/Responses business SSE and Claude event write+flush sequences while preserving prior Claude marshal-error flush behavior. Commit `8accf470b3ef5aa93416416488d2c72e5c93e871` adds a concurrent ResponseWriter regression test that detects overlapping Write/Flush operations.
- **P1 fixed:** commit `68e1dba3baa875b21df919ed4de6272bc9f117ba` adds transactional `model.AdjustSubscriptionPreConsume(requestId, subscriptionId, delta)` so fallback reservation deltas atomically update both the request record and subscription balance. Commit `1b8134c8b5f24b723b78f64c6fca61241cfadf13` switches subscription reserve/rollback to that primitive and makes terminal refund rely on the idempotent request-id refund for the full reservation rather than a separate in-memory `extraReserved` compensation. Commit `3e0082d21ac1360bd98313f6bbca07b48a3c37ce` covers successful extra reserve + full refund, rollback restoration, idempotent second refund and atomic capacity failure.

## Pending Verification
- Latest build-release workflow for `1b8134c8b5f24b723b78f64c6fca61241cfadf13` is still pending; verify quality/database/release jobs to completion and repair any regression. Superseded workflow cancellations from intermediate commits are not product failures.
- Non-enforced `BillingSession.Refund()` sets `refunded=true` before the actual wallet/token compensation runs. Wallet and token refund mutations are non-idempotent SQL increments/decrements without request-id keys, while billing outbox only mirrors ledger events and does not perform legacy compensation. A direct compensation DB failure is therefore not durably retried; design a safe exactly-once/idempotent repair rather than naively retrying ambiguous non-idempotent updates.
- `BillingSession.Settle()` can finish funding adjustment and then fail token quota adjustment; verify the full success/response-commit caller chain and whether the split can leave unrecoverable balance/accounting inconsistency or cause a misleading request failure.
- Shared shutdown timeout is used for HTTP drain, worker/gopool drain and later flushes; verify whether exhausted budget can skip durability work beyond intended forced-shutdown semantics.
- `docs/PROJECT_STATE.md` and `docs/MAINTENANCE.md` version claims differ; `VERSION` is authoritative at `v1.0.0-rc.4`. Classify only after checking intended doc contract.
- Analytics HTML interpolation in `main.go` uses deployment environment values; verify trust boundary before classifying.
- `RELAY_FIRST_BYTE_TIMEOUT` is currently an unused/unpublished configuration value; keep as design debt unless a supported contract or real caller expectation proves a functional bug.

## Current Call Chain
`/v1` relay route -> TokenAuth/rate limits -> Distribute/model+channel plan -> `controller.Relay` -> request validation/compat/preflight/pricing -> `PreConsumeBilling` -> fallback repricing / request-scoped `Reserve` -> provider adaptor/outbound request -> streaming/non-stream `DoResponse` -> settle on success or refund on terminal failure -> async non-enforced funding/token compensation -> shutdown lifecycle/gopool drain -> DB/resource close.

Current focus is the non-enforced `BillingSession.Settle` funding/token split and provider response/commit semantics, then remaining provider/tool/timeout/fallback coverage and the durable exactly-once refund compensation boundary.

## Validation
- Every code write so far was preceded by a current-HEAD check and followed by commit-diff inspection; whole-file writes were checked for unintended changes. A small unrelated Claude flush behavior change introduced during the SSE fix was detected by self-review and immediately restored in `9038c197...`.
- Added focused unit regressions for sensitive-word error semantics, gopool shutdown draining, concurrent ping/business SSE writes, and request-id subscription fallback reservation/refund behavior.
- The three subscription fallback-reservation commits were individually diff-inspected and contained only intended model/test/service changes.
- A prior lifecycle/billing/streaming lineage passed MySQL/PostgreSQL database-quality, default/classic frontend builds and backend quality. Full validation of current `1b8134c...` is not yet claimed because its workflow remains pending.

## Next Audit Scope
1. Trace every `SettleBilling` / `BillingSession.Settle` caller through successful provider responses and client-commit state; verify funding/token split-failure semantics and repair only if a safe consistency invariant is established.
2. Inspect current-head GitHub Actions jobs/results and repair regressions.
3. Continue relay/provider adapter matrix, especially stream framing, tool calls, response conversion, response-body lifetime, timeouts, fallback/retry and committed-response behavior.
4. Audit model/database/migrations/cache/concurrency across all supported databases.
5. Audit remaining auth/security/controllers/services, then both frontends, config/deployment/Docker and all workflows.
6. Re-run a high-risk second pass and full repository omission scan before declaring mission complete.

## Notes for Supervisor
`.ai/STATE.json` is not worker-authored authority and is intentionally not used as audit state. Durable decisions, if any, belong in `.ai/DECISIONS.md`; transient findings and coverage state remain here.
