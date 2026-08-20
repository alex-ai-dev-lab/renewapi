# Responses Compaction 503 – gpt-5.6-sol

Status: BLOCKED_RUNTIME

Created: 2026-08-20
Last updated: 2026-08-20

## Goal

Eliminate the strict-routing 503 for Codex remote compaction by making
capability evidence route-current, promptly probeable after route changes, and
diagnosable per rejected channel/model candidate without weakening strict
enforcement.

## Non-goals

- Do not permanently change strict enforcement to observe.
- Do not treat unknown capability as supported in strict mode.
- Do not enable global cross-family fallback or remove the capability gate.
- Do not infer compaction support from an ordinary Responses success.
- Do not rewrite published migrations. Product release publication is handled
  separately through the repository's `renewapi-v<version>` GitHub Actions
  workflow after the source commits and release identity are validated.

## Baseline

- base commit: `bd3d4c20aff85e89db07f5f0e109a20654d717a2`
- branch: `main` (`origin/HEAD` is `origin/main`)
- worktree initial state: dirty; no staged changes
- pre-existing files:
  - deleted: `.agents/skills/classic-to-default-sync/SKILL.md`, `.agents/skills/i18n-translate/SKILL.md`, `.agents/skills/shadcn-ui/**`, `.agents/skills/vercel-react-best-practices/AGENTS.md`
  - modified: `.github/workflows/build-release.yml`, `CHANGELOG.md`, `docs/PROJECT_STATE.md`, `model/billing_ledger.go`, `model/billing_ledger_test.go`, `model/billing_outbox.go`, `model/main.go`, `service/billing.go`, `service/billing_ledger_test.go`, `service/billing_session.go`
  - untracked: `docs/decisions/004-shadow-billing-reconciliation.md`, `model/billing_ledger_migration_test.go`, `tasks/active/full-ux-audit-ru-a-billing.md`, `tasks/active/full-ux-performance-audit.md`
- PREEXISTING policy: preserve all listed changes; do not stage them with this task.

## Issues

| ID | Status | Description | Evidence | Fix | Validation |
|---|---|---|---|---|---|
| ISSUE-001 | FIXED | Strict enforcement still rejects unknown; the fix establishes route-current evidence instead of weakening the gate. | `responsesRequirementDecision` preserves strict unknown rejection and routes only concrete evidence. | No observe bypass or unknown-as-supported behavior added. | Service/distributor tests PASS. |
| ISSUE-002 | PARTIAL | Strict/probe-disabled remains the safe compose default, so production must bootstrap evidence. | Compose and env example defaults inspected; root Force Probe endpoint exists. | Added explicit bootstrap/final operational record and verifier. | Code/config PASS; live bootstrap NOT RUN. |
| ISSUE-003 | FIXED | Stale fingerprint plus future backoff now re-probes immediately. | Scheduler checks current `ResponsesObservedRouteFingerprint` before honoring `NextProbeAt`. | Added stale/empty/matching/expired regression coverage. | Controller tests and race PASS. |
| ISSUE-004 | FIXED | Distributor 503 now carries stable error code and reason counts. | `ResponsesRoutePlanError`, reason enum, structured candidate counts, redacted log fields. | Added unknown/native-stream diagnostics regression. | Middleware/service tests PASS. |
| ISSUE-005 | FIXED | Manual Channel Test remains single-test; existing root-auth Force Probe is confirmed as the full matrix entry point. | `controller/channel_capability.go` calls the four-facet probe implementation; route is root-authenticated. | No misleading UI claim added. | Source inspection and probe code tests PASS. |
| ISSUE-006 | BLOCKED_RUNTIME | Native, native stream, and continuation are represented and probed independently, but real gpt-5.6-sol evidence is unavailable locally. | Separate persisted facet fields, probe calls, and route decisions. | Runtime remains conditional on credentials/environment. | Automated tests PASS; runtime BLOCKED_RUNTIME. |
| ISSUE-007 | FIXED | Ordinary Responses remains independent from compaction capability, including third-party channels. | `RequiresResponsesCompactionCapability` and normal distributor regression. | No global cross-family fallback added. | Service/middleware/full suite PASS. |
| ISSUE-008 | FIXED | DB-first upsert/cache refresh and striped observation locks are retained; route-current regression is covered. | Capability cache updates only after DB success; fingerprint invalidates stale rows. | No migration/schema change. | Model/service race tests PASS. |
| ISSUE-009 | BLOCKED_RUNTIME | Strict Codex compact + continuation + repeated compact requires a live deployment, Test group, channel credentials, and upstream. | No runtime credentials or database endpoint established; no local service listener. | Run only in an authorized environment. | BLOCKED_RUNTIME. |
| ISSUE-010 | FIXED | Continuation evidence is independent, and legacy compact output can seed continuation probing. | Native status is no longer promoted by continuation; scheduler prefers legacy output when available. | Full facet unit coverage added. | Focused unit tests PASS. |
| ISSUE-011 | FIXED | Probe observer now classifies non-2xx upstream results and preserves actual HTTP status. | `localErr` + `newAPIError` no longer bypasses observation; generic invalid 400/422 does not write evidence. | Added explicit unsupported/status and invalid-probe tests. | Focused tests PASS. |
| ISSUE-012 | FIXED | Route-plan-disabled distributor selection previously collapsed all compaction capability rejections into `model_not_found`. | Shared selector now records the same `ResponsesRequirementDecision` reason counts and returns `responses_compaction_no_eligible_channel` when actual candidates are rejected. | Route-plan-disabled unknown/native-stream/continuation and successful-candidate regressions; ordinary and missing-model semantics preserved. | Middleware/service tests PASS. |
| ISSUE-013 | FIXED | `PROBE_ENABLED=true` does not discover every ordinary advertised model by design. | Candidate selection remains explicit: configured capability/evidence, compact suffix, TestModel with default capability, persisted capability rows, or matching `RESPONSES_COMPACTION_MODEL`; scheduler also requires auto-test prerequisites. | Force Probe is documented as first bootstrap; scheduler refresh and candidate-scope tests retained and extended. | Controller tests and documentation PASS. |
| ISSUE-014 | FIXED | Native SSE verifier previously treated any `data:` frame as a compaction PASS. | Verifier now requires `text/event-stream`, parseable SSE JSON, and a real compaction item with non-empty `encrypted_content`; malformed/ordinary/missing-content fixtures are rejected. | Offline fixture harness added alongside the shared parser. | Bash syntax and fixture runtime PASS using a temporary jq binary. |

## Invariants

| ID | Status | Evidence |
|---|---|---|
| INV-001 ordinary Responses does not depend on compaction capability | PASS | Central `RequiresResponsesCompactionCapability` excludes `ResponsesNormal`; existing unit coverage. |
| INV-002 strict unknown is not supported | PASS | Existing evaluator and tests reject unknown in strict. |
| INV-003 native non-stream support does not imply native stream support | PASS | Separate `NativeStatus` and `NativeStreamStatus`; strict stream decision checks the latter. |
| INV-004 compact success does not imply continuation support | PASS | Separate `ContinuationStatus`; continuation routing checks it; continuation no longer promotes native status. |
| INV-005 stale fingerprint evidence is not current evidence | PASS | Effective capability requires equality with `ResponsesObservedRouteFingerprint`. |
| INV-006 manual concrete override is not overwritten by automatic probe | PASS | Probe model selection excludes untimestamped concrete manual declarations; existing focused test PASS. |
| INV-007 transient upstream failure is not permanently unsupported | PASS | 429/5xx/timeout paths do not set facet unsupported; focused transient probe test PASS. |
| INV-008 ordinary-only third-party channel is excluded from strict compaction | PASS | Protocol capability gate plus third-party/non-Responses adaptor coverage; full suite PASS. |
| INV-009 probe does not send real user history | PASS | Probe request uses fixed synthetic messages and `store=false`. |
| INV-010 logs do not leak API key / Authorization / Cookie | PASS | Diagnostics contain request id/group/model/kind/stream and stable reason counts only; focused 503 test PASS; runtime log inspection not run. |
| INV-011 route config change permits timely relearning | PASS | Scheduler skips only when future `NextProbeAt` and current `ResponsesObservedRouteFingerprint` match; stale/empty/expired evidence re-probes. |
| INV-012 migrations, if any, are backward compatible | NOT_APPLICABLE | No schema or migration change was made. |
| INV-013 route-plan=false compaction diagnostics remain actionable | PASS | Shared selector emits the stable error code and evaluator reason counts; middleware regression covers unknown, native stream, continuation, ordinary, missing-model, and successful cases. |
| INV-014 native SSE PASS requires a valid streamed compaction item | PASS | Shared verifier parser requires SSE content type, valid JSON frames, recognized compaction item type, and non-empty `encrypted_content`; fixture cases cover false positives and malformed data. |

## Runtime Validation

- ordinary Responses: NOT RUN
- legacy compact: NOT RUN
- native compact: NOT RUN
- native SSE: NOT RUN
- continuation: NOT RUN
- compact again and continue: NOT RUN
- stale fingerprint re-probe: PASS in controller regression; live runtime BLOCKED_RUNTIME
- route-plan=false diagnostics: PASS in middleware regression
- native SSE verifier fixtures: PASS with a temporary jq binary; parser syntax PASS

## Bootstrap / Final Configuration

Bootstrap, only while evidence is being established:

```env
RESPONSES_COMPACTION_ENFORCEMENT=observe
RESPONSES_COMPACTION_PROBE_ENABLED=true
RESPONSES_COMPACTION_PROBE_MAX_MODELS=4
RESPONSES_COMPACTION_ROUTE_PLAN_ENABLED=false
RESPONSES_COMPACTION_CROSS_FAMILY_FALLBACK=false
REQUEST_MODELS_FALLBACK_ENABLED=false
```

Final production configuration after capability evidence is established:

```env
RESPONSES_COMPACTION_ENFORCEMENT=strict
RESPONSES_COMPACTION_PROBE_ENABLED=true
RESPONSES_COMPACTION_PROBE_MAX_MODELS=4
RESPONSES_COMPACTION_ROUTE_PLAN_ENABLED=false
RESPONSES_COMPACTION_MAX_ROUTE_CANDIDATES=12
RESPONSES_COMPACTION_CROSS_FAMILY_FALLBACK=false
REQUEST_MODELS_FALLBACK_ENABLED=false
```

The repository default remains strict with the scheduler disabled. No
production deployment configuration was changed in this repository, so
applying and verifying the final scheduler-enabled configuration is an
operational `BLOCKED_RUNTIME` item. Route planning remains disabled for this
rollout.

First bootstrap runbook for a new channel/model:

1. Configure the channel and verify its model mapping.
2. Call the root-authenticated Force Probe endpoint with
   `model=gpt-5.6-sol`.
3. Inspect legacy, native, native-stream, and continuation facet results plus
   the current route fingerprint.
4. Keep `RESPONSES_COMPACTION_ENFORCEMENT=strict` after evidence is valid.
5. Leave the scheduler enabled for later refresh; it still requires the
   ordinary channel auto-test, `AllowAutoTestAndRecover`, due interval, valid
   time window, and master-node execution.

## Verification Record

- `go test ./service ./controller ./middleware ./model`: PASS
- `go test -race ./service ./controller ./middleware ./model`: PASS
- `go test ./...`: PASS
- `go vet ./...`: PASS
- `go build ./...`: PASS
- `git diff --check`: PASS
- runtime verifier: NOT RUN; no authorized live deployment or Test-group credentials
- real Test-group channel inventory/capability evidence: BLOCKED_RUNTIME
- native SSE fixture verifier: PASS using a temporary jq binary; temporary tool removed
- hosted GitHub Actions release run `32357902879`: PASS (quality,
  database-quality, multi-arch build, release assets, and tag-source check)
- published prerelease: `renewapi-v1.0.0-rc.3`, source commit
  `a3cfa2bd35998a665d912d87ffd19b09600c3620`, image digest
  `sha256:fdc1c240644ae4d87acd917e7d1e4d7201f93753f84a9e1ae8fe8343625316dd`

## Current state

Completed:
- Git baseline and PREEXISTING boundary captured.
- Applicable repository, maintenance, security, version, task, and migration rules read.
- Real request classification, distributor, route plan, capability evaluator, observed evidence, scheduler probe, persistence/cache, and execution paths located.

Completed:
- Route-current probe scheduling, independent facet observation, rejection
  diagnostics, focused tests, repository-wide tests, ADR, task ledger, and
  runtime verification workflow.
- Route-plan-disabled diagnostics, explicit probe bootstrap documentation, and
  strict native SSE item validation with offline fixtures.
- Source pushed to `main`; GitHub Actions built and published the rc.3
  prerelease. No server build, deployment, or restart was performed.

Remaining external work:
- Apply the bootstrap configuration in an authorized deployment, establish
  per-channel evidence, restore/confirm the final strict configuration, and
  execute the live Test-group and Codex end-to-end matrix.
