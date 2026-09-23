# Full UX & Performance Audit Remediation

Status: active

## 2026-09-23 线上复核

- 目标：`https://router.108848.xyz:1443/`；本轮从 `main@b50450a00` 开始，主工作树无预先修改。
- 复核起点的线上镜像为 `7d27402590f5`（2026-08-28），版本字符串同为 `v1.0.0-rc.4`，不能用版本号认定部署修订。
- 已用用户提供的 Clearcote 检查主要后台页面与手机布局，未发现页面脚本崩溃或整体横向溢出。检查会话来自用户授权的服务器认证配置，未修改管理员密码或业务数据。
- 已修复：首页预设/模拟指标、关闭注册后的错误入口、模型/文档链接、中文翻译、代码示例及键盘操作；补齐深色主题与文字对比度、滚动区域焦点问题。
- 浏览器故障注入复现：会话校验 503 强制跳回登录；渠道列表 500 强制跳转全局错误页；初始化检查失败后，刷新仍跳过重新检查。这些是受控复现，不代表服务器当时自然返回这些错误。
- 已修复三个恢复问题及渠道测试空上下文、取消传播、预检误记延迟；异常响应不再输出内部细节或污染已提交流。Controller/Middleware 测试、Go vet/build、前端测试/typecheck/lint/build 通过。完整页面初测仅首页可访问性失败，修正后的首页复测通过；4 个恢复用例通过。
- 发布门禁为 108 个页面场景及 4 个恢复用例。服务端证据目录：`/home/ken/services/new-api/backups/live-audit-20260923-073349/`。隔离迁移及 `quick_check` 通过，用户、令牌、渠道和配置的原列指纹一致；上线后的实际镜像、健康及切换结果记录在同目录，不能由本地测试推断。
- 扩展门禁的首次 hosted 运行 `35837378981` 暴露空数据仪表盘的 `trend=null` 崩溃；已统一统计列表空值并增加前端兼容与数组契约检查。保留失败记录，后续以修复提交的完整 Actions 结果为准。
- 生产继续使用原 `BILLING_LEDGER_MODE=off`，模式切换不属于本轮；历史记录与未完成项不能因此自动标记完成。

Audit source: `FULL_UX_PERFORMANCE_AUDIT.md`

Audit baseline: `4f3eeeb9c22f53d207b1633addc9e5f37ff013bf`

历史审计 HEAD：`e9c2a5a07`；历史产品版本为 `v1.0.0-rc.2`。2026-09-23 当前源码版本已为 rc.4，最新状态见 `docs/PROJECT_STATE.md`。

本轮更新 Billing 与上方线上复核实际覆盖的项目。其余矩阵行保留 2026-08-17 的审计快照，不代表已对 2026-09-23 源码重新确认；不能依据这些旧状态重复修改已修复功能。

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
| ISSUE-003 | P1 | Confirmed | VALIDATED_LOCAL | B | 2026-09-23：单次和自动测试对无上下文预检错误安全返回，保留健康/延迟；两种不支持类型的 HTTP 回归通过。 |
| ISSUE-004 | P1 | Confirmed | ALREADY_FIXED | B | 2026-09-23 源码复核：GetAllChannels 失败已复位运行标记，本轮未重复修改。 |
| ISSUE-005 | P1 | Confirmed | VALIDATED_LOCAL | B | 2026-09-23：初始化只缓存本次页面的成功结果，503 后刷新重新检查；实际浏览器回归通过。 |
| ISSUE-006 | P1 | Confirmed | VALIDATED_LOCAL | C | 2026-09-23：临时故障保留会话，明确 401 才退出；恢复按钮实际浏览器回归通过。 |
| ISSUE-007 | P1 | Confirmed | VALIDATED_LOCAL | C | 2026-09-23：查询重试耗尽后保留当前页面，不再跳转全局 500；实际浏览器回归通过。 |
| ISSUE-008 | P1 | Confirmed | CONFIRMED_CURRENT | D | Rechecked: non-stream channel tests still use unbounded `io.ReadAll` and log the full raw body. |
| ISSUE-009 | P1 | Confirmed | VALIDATED_LOCAL | B | 2026-09-23：两处异常恢复共用脱敏响应，并保留已提交 SSE；Middleware 回归通过。 |
| ISSUE-010 | P1 | Confirmed | VALIDATED_LOCAL | D | 2026-09-23：交互测试传递请求 context，取消终止上游且不覆盖健康/延迟；Controller 回归通过。 |
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

## 历史验收记录（2026-08-17）

- Audit status correction: 9 confirmed current, 13 unverified, 0 already fixed,
  1 partially fixed, and 1 blocked on runtime evidence.
- Release Unit A: local implementation/SQLite gates pass; MySQL, PostgreSQL,
  and process-level restart gates block release readiness.
- Product version: unchanged at `v1.0.0-rc.2` during development.

## Handoff

After each substantial change, update this file and
`tasks/active/full-ux-audit-ru-a-billing.md` with completed tests, blockers,
and the next action. Do not recreate the audit from conversation history.
