# RenewAPI full audit 0830

Status: in progress
Task: `renewapi-full-audit-0830`
Audit branch: `audit/renewapi-full-audit-0830`
Main baseline: `7d27402590f5ace20001f3f1e9d0b916ed2d4336`
Started: 2026-08-30

## Goal

Systematically audit the current RenewAPI main baseline across frontend, backend, relay/provider routing, data/state handling, auth/security, configuration, persistence, Docker/build/CI, and cross-module request/response chains. Confirm issues before changing code, fix high-confidence functional defects locally, test and regress, then re-audit high-risk areas and check for omissions.

## Severity

- P0: data loss/corruption, auth bypass, severe outage, dangerous writes, major security flaw.
- P1: core-path functional failure, common-path state corruption, major compatibility problem.
- P2: edge functional bug, clear UX/state/error handling defect.
- P3: low-impact defect or worthwhile technical debt.

## Audit coverage

- [x] Repository rules and current project state read.
- [x] Main baseline pinned.
- [x] Repository structure and architecture mapped at top level; detailed module coverage remains ongoing.
- [x] Existing tests and CI mapped at workflow level; branch does not receive automatic main-only workflow runs.
- [ ] Bootstrap/config/lifecycle/common utilities audited. Startup/setup/recovery/CORS reviewed; common utilities remain.
- [ ] Router/middleware/auth/security audited. API/relay routing, auth, body limit, secure verification, rate limit and CORS reviewed; remaining middleware/routes continue.
- [ ] Controller/service/model/data access audited. Subscription payment/model, Stripe top-up, setup, stats, OAuth and passkey paths reviewed; remaining areas continue.
- [ ] Relay core/request-response/stream/tool/usage/error paths audited. Main Relay retry/preconsume loop, Responses handler, billing session and Midjourney proxy inspected; broad relay audit remains.
- [ ] Provider adapters audited systematically.
- [ ] Billing/quota/rate-limit/cache/state-machine paths audited. Subscription purchase-limit, billing preconsume/refund/settlement and rate-limit paths reviewed; remaining paths continue.
- [x] OAuth/passkey primary auth flows reviewed; no confirmed auth bypass found in first pass.
- [ ] User/admin/security paths audited. Login/setup partially reviewed; remaining endpoints continue.
- [ ] Default frontend audited. Subscription admin PATCH caller and dashboard stat contract reviewed; broad frontend audit remains.
- [ ] Classic frontend audited.
- [ ] Frontend/backend contract paths audited. CORS PATCH and dashboard stats checked; broad contract audit remains.
- [ ] Docker/compose/env/migrations/build/deploy audited.
- [x] GitHub Actions mapped; detailed second-pass workflow audit remains before completion.
- [x] Multiple focused fixes implemented; confirmed subscription-cap and panic issues still remain to fix.
- [ ] Full relevant tests/build/static checks run. Local clone unavailable due environment DNS; branch workflows are main-only. Targeted tests have been added but not executed yet.
- [ ] High-risk areas second-pass reviewed.
- [ ] Final full-repository omission check completed.
- [ ] Main-head drift checked before completion.

## Confirmed issues

1. **P1 - Asynchronous subscription purchase cap can produce paid orders without entitlement.**
   - Affected entry points confirmed: Stripe, Creem, EPay, Waffo/Pancake subscription checkout.
   - Root cause: checkout-time `MaxPurchasePerUser` enforcement counts only completed `UserSubscription` rows. Multiple pending external-payment orders can be created before any completes. During webhook completion, `CreateUserSubscriptionFromPlanTx` re-checks the completed subscription count and can reject a later already-paid order after the payment provider has accepted money.
   - Stripe fulfillment logs the completion error while the webhook endpoint still returns 200, making the mismatch persistent without manual intervention. Creem retries cannot resolve the same cap condition.
   - Balance purchase does not have the same prepayment window because its transaction serializes on the user row before deduction and entitlement creation.
   - Lifecycle review showed that counting all pending orders as permanent reservations is unsafe because providers do not all expose/handle abandonment expiry. Planned minimal semantic fix: keep cap checks on purchase initiation/admin/balance paths, but do not reject an already-confirmed paid `source=order` entitlement a second time.

2. **P1 - Stripe checkout can become externally payable before a matching local order exists.**
   - Confirmed in both subscription Stripe checkout and regular Stripe top-up on baseline.
   - A DB insert failure after Stripe Session creation leaves a valid external payment session whose webhook cannot find/fulfil a local order.
   - Fixed on audit branch for both paths by persisting pending local state first and marking it failed if Stripe session creation fails.

3. **P1 - First-time setup permits unusable root credentials and was non-atomic/racy.**
   - Blank/whitespace username passed setup validation even though password login rejects empty username, allowing an initialized installation without a usable password root.
   - Root creation, option writes and setup marker were independent writes; failures could leave partial setup state.
   - `constant.Setup` was set before the setup row was durably written, causing the current process to refuse retry after a final DB failure.
   - Concurrent first-time requests/instances could both observe no root because `RootUserExists` is a plain SELECT and `Setup` had no singleton constraint, then create two distinct root users.
   - Fixed on audit branch with trimmed/non-empty username validation and one transaction that first reserves fixed `setup.ID=1`, then creates root and setup options atomically. Runtime setup/option state is published only after commit.

4. **P2 - Global panic recovery leaks raw internal panic details to HTTP clients.**
   - `main.go` logs the panic server-side and also embeds the raw panic value in the JSON 500 response.
   - Impact: panics containing SQL, file paths, upstream errors, internal state or sensitive fragments may be disclosed externally.
   - Safe local fix remains pending: retain detailed server logging while returning a generic client-facing panic message.

5. **P2 - Trusted cross-origin frontend cannot call valid PATCH API routes.**
   - Backend registers `PATCH /api/subscription/admin/plans/:id` and the default frontend calls it with `api.patch`.
   - CORS `AllowMethods` omitted PATCH, so deployments using a separate trusted `FRONTEND_BASE_URL` fail browser preflight for this valid route.
   - Fixed on audit branch; validation test added.

6. **P2 - Setup endpoint exposed raw internal errors before authentication.**
   - Baseline returned password-hash/DB/option/setup errors directly in setup responses.
   - Fixed together with setup transaction changes: detailed errors are logged server-side while clients receive stable generic setup errors.

7. **P2 - Regular Stripe top-up ignored user lookup errors and dereferenced the result.**
   - `model.GetUserById` error was discarded and the returned pointer dereferenced. A DB/read failure could panic the request before checkout.
   - Fixed while repairing Stripe order ordering.

## Fixed issues

1. **P2 CORS PATCH omission**
   - `middleware/cors.go`: added `PATCH` to allowed methods without widening allowed origins.
   - `middleware/cors_test.go`: added trusted-origin PATCH preflight regression coverage.
   - Commits: `2af8c84ea99e50d1098abcc319bf2825926fe503`, `3a7819ab604a57ac8f0d66261e53fcf60ef0ecff`.

2. **P1 Stripe subscription orphan-payable session**
   - `controller/subscription_payment_stripe.go`: local pending `SubscriptionOrder` now exists before Stripe Session creation; Stripe link failure marks order failed.
   - Commit: `f2ffdcb3e965c037edfbc3a604d98014c8357099`.

3. **P1/P2 setup validation, atomicity, concurrency and error disclosure**
   - `controller/setup.go`: reject blank root username; setup row/root/options committed in one transaction; fixed singleton `setup.ID=1` reserves initialization before root creation; runtime state only published after durable commit; internal setup errors no longer returned verbatim.
   - Commits: `7204adddec07c19b8878cc714728562fe702fd05`, `e815dc6f554c915e33ff57e6d8a4be54f6a34f2`.
   - `controller/setup_test.go`: SQLite regression coverage for blank username and singleton durable setup/options.
   - Test commit: `0aa57cad5154c12a708c4a81cf1a494c32a40286`.

4. **P1/P2 regular Stripe top-up ordering and user lookup**
   - `controller/topup_stripe.go`: validates user lookup; persists pending TopUp before creating Stripe Session; marks TopUp failed when Stripe link creation fails.
   - Commit: `af7671b0ca6fa9f269c578ed8ffb0f3cd84e6a6d`.

Executable test runs for these changes remain pending because the repository clone was unavailable in the current execution environment and branch workflows do not automatically run.

## Pending validation

1. **Midjourney image proxy authorization/privacy boundary**
   - `/mj/image/:id` is registered before `TokenAuth`, and `RelayMidjourneyImage` resolves task data using unscoped task ID lookup. Need finish tracing task-ID entropy/exposure and frontend image rendering before classifying; direct image display may be an intentional authentication tradeoff.

2. **Direct `encoding/json` use in business paths**
   - Confirmed examples include `relay/mjproxy_handler.go`, `middleware/turnstile-check.go`, and `controller/user.go`, conflicting with repository Rule 1 requiring `common.*` JSON helpers. Currently P3/design-rule debt unless concrete behavior impact is demonstrated.

3. **Project-state documentation drift**
   - `docs/PROJECT_STATE.md` still identifies rc.2 while maintenance/release evidence identifies rc.3. P3 maintenance-state inconsistency unless a functional release dependency is found.

4. **Billing settlement secondary token adjustment errors**
   - BillingSession intentionally prevents refund after funding source settlement and returns token quota adjustment errors. Need follow all `Post*ConsumeQuota` callers and ledger/reconciler modes before deciding whether token/accounting drift can persist outside reconcile-enabled modes.

## Current call chains under review

1. External subscription purchase: API checkout -> provider controller -> local `SubscriptionOrder` -> provider payment -> webhook -> `CompleteSubscriptionOrderWithPaidAmount` -> `CreateUserSubscriptionFromPlanTx` -> entitlement/top-up mirror/order success. Current task: change paid-order cap semantics without weakening initiation/admin/balance checks.
2. Relay request: TokenAuth/Distribute -> request decode/validation -> token estimate -> preflight/route plan -> price/preconsume -> channel/retry -> adaptor request/response -> stream commit boundary -> usage settlement/refund. Current task: trace settlement and provider conversion paths for duplicate/missing charge and error mapping.

## Next audit scope

1. Implement model-layer paid-order purchase-cap fix and add focused regression coverage for max=1 with two already-paid pending orders.
2. Fix P2 panic detail disclosure and add recovery regression coverage if feasible.
3. Continue user/admin/security and remaining middleware/router/controller/model paths.
4. Systematically audit relay core and provider adapters, including request/response conversion, streaming, tool calls, usage/billing/error/fallback/timeout/retry behavior.
5. Audit both frontends and frontend/backend contracts, then Docker/env/migrations/build/deploy and detailed workflow behavior.
6. Run available tests/build/static checks, second-pass high-risk areas, final omission check, and main-head drift check before completion.

## Handoff rule

Treat GitHub main plus this audit branch as the only source of truth. Re-read current code and this file before continuing. Do not infer unresolved findings from chat history. Update this document at meaningful checkpoints with audited modules, confirmed/fixed/pending issues, current call chain, validation results, and next scope.
