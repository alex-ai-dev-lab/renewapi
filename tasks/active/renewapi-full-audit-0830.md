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
- [ ] Repository structure and architecture mapped.
- [ ] Existing tests and CI mapped.
- [ ] Bootstrap/config/lifecycle/common utilities audited.
- [ ] Router/middleware/auth/security audited.
- [ ] Controller/service/model/data access audited.
- [ ] Relay core/request-response/stream/tool/usage/error paths audited.
- [ ] Provider adapters audited systematically.
- [ ] Billing/quota/rate-limit/cache/state-machine paths audited.
- [ ] OAuth/passkey/user/admin paths audited.
- [ ] Default frontend audited.
- [ ] Classic frontend audited.
- [ ] Frontend/backend contract paths audited.
- [ ] Docker/compose/env/migrations/build/deploy audited.
- [ ] GitHub Actions audited.
- [ ] Focused fixes implemented and validated.
- [ ] Full relevant tests/build/static checks run.
- [ ] High-risk areas second-pass reviewed.
- [ ] Final full-repository omission check completed.
- [ ] Main-head drift checked before completion.

## Confirmed issues

None yet.

## Fixed issues

None yet.

## Pending validation

None yet.

## Current call chain under review

Repository bootstrap and architecture discovery: entrypoint -> router/middleware -> controller/service/model; relay routing -> provider adapters -> response/stream settlement; frontend API client -> page/component state -> backend contracts.

## Next audit scope

1. Enumerate the full source/test/workflow tree and identify nested repository rules.
2. Read architecture/maintenance docs and current active task interactions.
3. Establish baseline test/build/CI status.
4. Audit backend request-routing and relay high-risk paths first, then frontend and infrastructure, while recording every confirmed issue and fix here.

## Handoff rule

Treat GitHub main plus this audit branch as the only source of truth. Re-read current code and this file before continuing. Do not infer unresolved findings from chat history. Update this document at meaningful checkpoints with audited modules, confirmed/fixed/pending issues, current call chain, validation results, and next scope.
