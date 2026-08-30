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
- [ ] Bootstrap/config/lifecycle/common utilities audited. Startup/recovery/CORS reviewed; common utilities remain.
- [ ] Router/middleware/auth/security audited. API/relay routing, auth, body limit, secure verification, rate limit and CORS reviewed; remaining middleware/routes continue.
- [ ] Controller/service/model/data access audited. Subscription payment/model and stats paths reviewed; remaining areas continue.
- [ ] Relay core/request-response/stream/tool/usage/error paths audited. Midjourney route/proxy inspected; broad relay audit remains.
- [ ] Provider adapters audited systematically.
- [ ] Billing/quota/rate-limit/cache/state-machine paths audited. Subscription purchase-limit and rate-limit paths reviewed; remaining paths continue.
- [ ] OAuth/passkey/user/admin paths audited.
- [ ] Default frontend audited. Subscription admin PATCH caller and dashboard stat contract reviewed; broad frontend audit remains.
- [ ] Classic frontend audited.
- [ ] Frontend/backend contract paths audited. CORS PATCH and dashboard stats checked; broad contract audit remains.
- [ ] Docker/compose/env/migrations/build/deploy audited.
- [x] GitHub Actions mapped; detailed second-pass workflow audit remains before completion.
- [x] Focused fix implemented for one confirmed P2; further confirmed issues remain to fix.
- [ ] Full relevant tests/build/static checks run. Local clone unavailable due environment DNS; branch workflows are main-only, so executable validation is still pending.
- [ ] High-risk areas second-pass reviewed.
- [ ] Final full-repository omission check completed.
- [ ] Main-head drift checked before completion.

## Confirmed issues

1. **P1 - Asynchronous subscription purchase cap can produce paid orders without entitlement.**
   - Affected entry points confirmed: Stripe, Creem, EPay, Waffo/Pancake subscription checkout.
   - Root cause: checkout-time `MaxPurchasePerUser` enforcement counts only completed `UserSubscription` rows. Multiple pending external-payment orders can be created before any completes. During webhook completion, `CreateUserSubscriptionFromPlanTx` re-checks the completed subscription count and can reject a later already-paid order after the payment provider has accepted money.
   - Stripe fulfillment currently logs completion errors while returning a successful webhook response, making the money/entitlement mismatch operationally persistent without manual intervention.
   - Balance purchase does not have the same prepayment window because its transaction serializes on the user row before deducting balance and creating the subscription.
   - Fix pending lifecycle analysis: reservation must not make abandoned external checkout sessions permanently consume purchase capacity, and must remain SQLite/MySQL/Postgres compatible.

2. **P2 - Global panic recovery leaks raw internal panic details to HTTP clients.**
   - `main.go` logs the panic server-side and also embeds the raw panic value in the JSON 500 response.
   - Impact: panics containing SQL, file paths, upstream errors, internal state or sensitive fragments may be disclosed externally.
   - Safe local fix: retain detailed server logging while returning a generic client-facing panic message. Not yet implemented.

3. **P2 - Trusted cross-origin frontend cannot call valid PATCH API routes.**
   - Backend registers `PATCH /api/subscription/admin/plans/:id` and the default frontend calls it with `api.patch`.
   - CORS `AllowMethods` omitted PATCH, so deployments using a separate trusted `FRONTEND_BASE_URL` fail browser preflight for this valid route.
   - Fixed on audit branch; validation test added.

## Fixed issues

1. **P2 CORS PATCH omission**
   - `middleware/cors.go`: added `PATCH` to allowed methods without widening allowed origins.
   - `middleware/cors_test.go`: added trusted-origin PATCH preflight regression coverage.
   - Audit-branch commits: `2af8c84ea99e50d1098abcc319bf2825926fe503`, `3a7819ab604a57ac8f0d66261e53fcf60ef0ecff`.
   - Executable test run remains pending because the repository could not be cloned in the execution environment and the repository workflows currently trigger on main/manual dispatch rather than this branch.

## Pending validation

1. **Midjourney image proxy authorization/privacy boundary**
   - `/mj/image/:id` is intentionally or accidentally registered before `TokenAuth`, and `RelayMidjourneyImage` resolves task data using unscoped task ID lookup. Need trace task-ID entropy/exposure and frontend image rendering before classifying; direct image display may be an intentional authentication tradeoff.

2. **Direct `encoding/json` use in business paths**
   - At least `relay/mjproxy_handler.go` and `middleware/turnstile-check.go` use standard `encoding/json`, conflicting with repository Rule 1 requiring `common.*` JSON helpers. Currently treated as P3/design-rule debt unless behavior/compatibility impact is demonstrated.

3. **Project-state documentation drift**
   - `docs/PROJECT_STATE.md` still identifies rc.2 while maintenance/release evidence identifies rc.3. Treat as P3 maintenance-state inconsistency unless a functional release process dependency is found.

## Current call chain under review

External subscription purchase: frontend/API checkout request -> provider-specific subscription payment controller -> local `SubscriptionOrder` pending state -> provider checkout/payment -> webhook/callback -> `CompleteSubscriptionOrderWithPaidAmount` -> `CreateUserSubscriptionFromPlanTx` -> user entitlement / top-up mirror / order success. Current focus is pending-order status/expiry lifecycle and transaction locking so the P1 can be fixed without permanent reservations or cross-database regressions.

## Next audit scope

1. Complete subscription-order lifecycle analysis: all status constants/transitions, provider expiration/failure callbacks, pending cleanup, `lockForUpdate` implementation and all order creation/completion callers.
2. Implement and test a model-layer P1 fix that serializes purchase-cap reservation and completion safely across SQLite/MySQL/Postgres without permanently blocking abandoned checkouts.
3. Fix P2 panic detail disclosure with a generic client response and targeted recovery test if feasible.
4. Continue remaining middleware/router/security, controller/service/model, OAuth/passkey/user/admin.
5. Systematically audit relay core and provider adapters, including request/response conversion, streaming, tool calls, usage/billing/error/fallback/timeout/retry behavior.
6. Audit both frontends and frontend/backend contracts, then Docker/env/migrations/build/deploy and detailed workflow behavior.
7. Run available tests/build/static checks, second-pass high-risk areas, final omission check, and main-head drift check before completion.

## Handoff rule

Treat GitHub main plus this audit branch as the only source of truth. Re-read current code and this file before continuing. Do not infer unresolved findings from chat history. Update this document at meaningful checkpoints with audited modules, confirmed/fixed/pending issues, current call chain, validation results, and next scope.
