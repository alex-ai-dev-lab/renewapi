# Full UX & Performance Audit Remediation

Status: active

Audit source: `FULL_UX_PERFORMANCE_AUDIT.md`

Audit baseline: `4f3eeeb9c22f53d207b1633addc9e5f37ff013bf`

历史审计 HEAD：`e9c2a5a07`；历史产品版本为 `v1.0.0-rc.2`。2026-09-23 当前源码版本已为 rc.4，最新状态见 `docs/PROJECT_STATE.md`。

本轮仅更新已有证据发生变化的 Billing 项。其余矩阵行保留 2026-08-17 的审计快照，不代表已对 2026-09-23 源码重新确认；不能依据这些旧状态重复修改已修复功能。

Issue status is based only on material inspection of the current checkout, not
on the absence of a closing commit. Release Unit A rechecked ISSUE-001 and the
P1 code paths (ISSUE-002 through ISSUE-010). P2/P3 issues were deliberately not
expanded during the P0 release unit and remain unverified unless runtime
evidence is explicitly recorded. ISSUE-018 retains the audit's original
`Highly Likely` confidence until generated SQL and three-database EXPLAIN
evidence exist.

## Issue Matrix

| Issue | Severity | Audit confidence | Current status | Release unit | Evidence / notes |
| --- | --- | --- | --- | --- | --- |
| ISSUE-001 | P0 | Confirmed | VALIDATED_CURRENT | A | 2026-09-23：shadow 已有持久化事务；MySQL 5.7/8.4、PostgreSQL 9.6/16 实际 CI 和 SQLite 独立进程崩溃恢复通过。默认仍为 shadow，详情见 RU-A 与本轮可靠性升级任务。 |
| ISSUE-002 | P1 | Confirmed | CONFIRMED_CURRENT | C | Rechecked: both token-key reveal routes still omit `SecureVerificationRequired`. |
| ISSUE-003 | P1 | Confirmed | CONFIRMED_CURRENT | B | Rechecked: unsupported tests still return a nil context which single/batch callers dereference. |
| ISSUE-004 | P1 | Confirmed | CONFIRMED_CURRENT | B | Rechecked: channel-load failure still returns after setting the global running flag and before the goroutine defer. |
| ISSUE-005 | P1 | Confirmed | CONFIRMED_CURRENT | B | Rechecked: failed setup status still becomes `null`, then permanently sets `setup_status_checked`. |
| ISSUE-006 | P1 | Confirmed | CONFIRMED_CURRENT | C | Rechecked: any `getSelf` failure still resets auth and redirects to sign-in. |
| ISSUE-007 | P1 | Confirmed | CONFIRMED_CURRENT | C | Rechecked: global QueryCache handling still navigates every Axios 500 to `/500`. |
| ISSUE-008 | P1 | Confirmed | CONFIRMED_CURRENT | D | Rechecked: non-stream channel tests still use unbounded `io.ReadAll` and log the full raw body. |
| ISSUE-009 | P1 | Confirmed | CONFIRMED_CURRENT | B | Rechecked: custom recovery still embeds the raw panic value in the client response. |
| ISSUE-010 | P1 | Confirmed | CONFIRMED_CURRENT | D | Rechecked: interactive channel-test wrappers still force `context.Background()`. |
| ISSUE-011 | P2 | Confirmed | UNVERIFIED | C | Not materially rechecked during RU-A. Preserve the audit finding without claiming current confirmation. |
| ISSUE-012 | P2 | Confirmed | UNVERIFIED | C | Not materially rechecked during RU-A. |
| ISSUE-013 | P2 | Confirmed | UNVERIFIED | B | Not materially rechecked during RU-A. |
| ISSUE-014 | P2 | Confirmed | UNVERIFIED | C | Not materially rechecked during RU-A. |
| ISSUE-015 | P2 | Confirmed | UNVERIFIED | C | Not materially rechecked during RU-A. |
| ISSUE-016 | P2 | Confirmed | UNVERIFIED | B | Not materially rechecked during RU-A. |
| ISSUE-017 | P2 | Confirmed | UNVERIFIED | D | Not materially rechecked during RU-A. |
| ISSUE-018 | P2 | Highly Likely | BLOCKED_RUNTIME | D | Generated SQL and SQLite/MySQL/PostgreSQL EXPLAIN evidence were not run; do not upgrade this to confirmed. |
| ISSUE-019 | P2 | Confirmed | UNVERIFIED | C | Not materially rechecked during RU-A. |
| ISSUE-020 | P2 | Confirmed | UNVERIFIED | E | Not materially rechecked during RU-A. |
| ISSUE-021 | P2 | Confirmed | UNVERIFIED | E | Not materially rechecked during RU-A. |
| ISSUE-022 | P2 | Confirmed | UNVERIFIED | B | Not materially rechecked during RU-A. |
| ISSUE-023 | P3 | Confirmed | UNVERIFIED | Classic retirement | Not materially rechecked during RU-A. |
| ISSUE-024 | P3 | Confirmed | UNVERIFIED | Classic retirement | Not materially rechecked during RU-A. |

## Current Release Unit

Release Unit A — Critical Billing Correctness (`ISSUE-001`).

Goal: make default shadow billing balance transitions atomic, durable,
idempotent, and recoverable without changing the default mode to `enforce`.

Billing 的源码及开发验证已经补齐，先前缺失的外部数据库与独立进程恢复证据不再作为当前阻塞。最终提交和 CI 链接见 `full-ux-audit-ru-a-billing.md` 与 `../archive/2026-09-reliability-upgrade.md`。生产切换、产品发布及下列其他 release unit 不属于此完成结论。

## Remaining Release Units

- Release Unit B — low-risk reliability quick wins: ISSUE-003, 004, 005, 009,
  013, 016, 022.
- Release Unit C — security/auth/error contract: ISSUE-002, 006, 007, 011,
  012, 014, 015, 019.
- Release Unit D — runtime performance/jobs/logs: ISSUE-008, 010, 017, 018,
  plus relay telemetry and benchmark work.
- Release Unit E — engineering quality infrastructure: ISSUE-020, 021.
- Classic retirement program: ISSUE-023, 024 after the accepted retirement
  exit criteria are met; no automatic removal is part of this task.

## Runtime Verification Backlog

- SQLite: focused failure-injection, duplicate delivery, reconciler replay,
  migration, compatibility, and race tests passed on 2026-08-17.
- MySQL/PostgreSQL：2026-09-23 实际 hosted CI 已通过；不是以配置文件代替执行证据。
- 独立进程崩溃/恢复：2026-09-23 SQLite shadow/enforce 测试通过；未执行生产实例重启。
- Browser/provider/Redis: deferred to the release unit that owns each issue;
  no unavailable runtime is reported as passed.

## Validation Status

- Audit status correction: 9 confirmed current, 13 unverified, 0 already fixed,
  1 partially fixed, and 1 blocked on runtime evidence.
- Release Unit A: local implementation/SQLite gates pass; MySQL, PostgreSQL,
  and process-level restart gates block release readiness.
- Product version: unchanged at `v1.0.0-rc.2` during development.

## Handoff

After each substantial change, update this file and
`tasks/active/full-ux-audit-ru-a-billing.md` with completed tests, blockers,
and the next action. Do not recreate the audit from conversation history.
