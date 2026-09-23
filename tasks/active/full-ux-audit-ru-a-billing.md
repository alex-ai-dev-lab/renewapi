# Release Unit A — Billing Correctness

状态：源码与开发验证已完成；历史发布计划由当前产品状态记录接管。

Created: 2026-08-17

Audit issue: `ISSUE-001`

## Goal

Repair default `BILLING_LEDGER_MODE=shadow` so wallet, token, and subscription
balance transitions are performed through the existing `BillingLedger` in one
database transaction, with durable component state and reconciler replay. Keep
the default mode as shadow and preserve SQLite/MySQL/PostgreSQL compatibility.

## Non-goals

- Do not switch the default mode to `enforce`.
- Do not remove the legacy billing path or redesign the relay architecture.
- Do not mix frontend, channel-test, CI, or Classic retirement work into this
  release unit.

## Invariants

For a ledger reservation with target quota `Q`:

- Wallet balance: `wallet_after = wallet_before - applied_funding_delta`.
- Token quota: `token_after = token_before - applied_token_delta`, except
  unlimited/playground tokens where the token leg is not applicable.
- Subscription quota: `amount_used` changes by the same signed delta as the
  funding leg and never leaves `[0, amount_total]`.
- Ledger `AppliedQuota` is the durable amount already applied to balance legs;
  `DesiredQuota` is the target requested by settle/refund.
- A settle/refund transition is terminal only after all applicable balance
  components are applied in the same transaction.
- Repeating `request_id + phase` or replaying the reconciler cannot double
  debit, credit, quota consume, or refund.
- A process crash before commit leaves the old ledger state and balance values;
  a retry then applies the transition once.
- Statistics/counters remain owned by the existing enforce path; shadow keeps
  its compatibility counter path and never double-counts it.

## Current state

Completed:

- Corrected the master audit status policy: only materially rechecked issues
  are confirmed current; deferred P2/P3 findings remain unverified.
- Confirmed the original ISSUE-001 path, then routed shadow reservation,
  settle, refund, and extra reserve balance writes through the ledger-owned
  transaction.
- Added durable component state through additive migration
  `billing-ledger:v2`.
- Added SQLite tests for settle/refund partial failure rollback and recovery,
  duplicate transitions, repeated reconciler delivery, concurrent settle,
  concurrent refund, settle-vs-refund, subscription extra reserve/refund,
  missing-row `RowsAffected`, legacy-ledger migration compatibility, and
  shadow async-task preparation/acknowledgement through the durable ledger.
- 修复 stale reservation 回收：在行锁内使用 `updated_at` 和 ledger `version`
  做 fence，避免旧快照退款已完成或已刷新额度的 reservation；写入新的
  acknowledgement 对象时保留 prepared async task 的计费上下文。
- 补齐 fresh acknowledgement 的 prepared Task merge：保留持久化身份、路由、
  模型、私有 key 和计费快照，只合并上游运行态字段；reconcile replay 和
  retry marker 使用同一组 `state + desired + version` fence。
- 新增 activity refresh、completed settlement race、未变化 stale refund、重复
  stale delivery 和 fresh async-task acknowledgement 对象的回归测试。
- Focused model/service tests, focused race tests, all Go tests, `go vet`, and
  `go build` pass locally.
- Created the master task and this release-unit task.

2026-09-23 验证补充：

- 先前 MySQL/PostgreSQL 的 `BLOCKED_RUNTIME` 已由真实 [Actions 35764799379](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35764799379) 补齐；`ea9a4f994` 的 MySQL 5.7/8.4、PostgreSQL 9.6/16 迁移、账本事务、回滚、幂等、并发和渠道配置 job 全部成功。
- 新增 `TestBillingReconcilerSurvivesProcessRestart`，独立子进程强制退出后在同一 SQLite 文件中验证 settle/refund/orphan/uncommitted 恢复；shadow/enforce 均通过。该测试证明持久化账本恢复，不代表生产重启或部署验收。
- 本轮继续补齐 off 原子退款、终态状态锁、补充预扣失败边界及最终成功渠道归属；shadow 持久化余额事务本身保持 NOOP，默认仍为 shadow。
- 最终源码与后续 CI 证据由 [2026 年 9 月升级任务](../archive/2026-09-reliability-upgrade.md) 和 [Billing enforce 说明](../../docs/billing-ledger-enforce.md) 记录。
- 原 rc.2 → rc.3 发布计划已过时：当前 `VERSION` 为 rc.4，本轮不修改版本、创建 tag、发布或部署。其他 UX/性能任务未自动纳入 Billing 完成状态。

## Validation plan

```text
gofmt -w model/billing_ledger.go model/main.go service/billing_session.go
go test -count=1 ./model ./service
go test -race -count=1 ./model ./service
go test -count=1 ./...
go vet ./...
go build ./...
```

External MySQL/PostgreSQL migration checks must use the repository's CI
services when local services are unavailable.

## Validation result — 2026-08-17

| Invariant | Result | Evidence / limitation |
| --- | --- | --- |
| Shadow settle partial-failure recovery | PASS | SQLite transaction rollback plus reconciler retry. |
| Shadow refund partial-failure recovery | PASS | SQLite wallet credit rolls back when token refund fails; reconciler retry succeeds. |
| Duplicate Settle | PASS | Model and session replay leave balances unchanged. |
| Duplicate Refund | PASS | Model and session replay leave balances unchanged. |
| Repeated reconciler delivery | PASS | Second delivery resolves zero work and does not change balances. |
| Second balance component failure rollback | PASS | SQLite funding write rolls back when token leg fails. |
| Reconciler retry semantics | PASS | Durable `reconcile_required` rows settle/refund once after repair. |
| Application process restart | NOT RUN | No process-level restart environment was available. |
| Shadow async task durable prepare/acknowledge | PASS | Shadow now uses the persistent pending-task path; acknowledgement and settlement preserve task/ledger state. |
| Concurrent Settle | PASS | Focused Go race test on SQLite; external locking behavior is still blocked. |
| Concurrent Refund | PASS | Focused Go race test on SQLite; external locking behavior is still blocked. |
| Concurrent Settle vs Refund | PASS | SQLite ends in one valid terminal state with consistent balances; external locking behavior is still blocked. |
| Wallet/token invariants | PASS | SQLite reserve/settle/refund and failure tests. |
| Subscription extra invariants | PASS | SQLite reserve-more/settle/refund test covers subscription, token, and pre-consume record. |
| Existing ledger compatibility | PASS | SQLite pre-v2 shadow row preserves `AppliedQuota` and applies only the later delta. |
| Additive migration compatibility | PASS | SQLite v1 fixture gains only v2 component-state columns. |
| MySQL | NOT RUN | `BLOCKED_RUNTIME`; workflow coverage is unexecuted. |
| PostgreSQL | NOT RUN | `BLOCKED_RUNTIME`; workflow coverage is unexecuted. |

Commands passed locally:

```text
go test -count=1 ./model ./service
go test -race -count=1 ./model ./service
go test -count=1 ./...
go vet ./...
go build ./...
```

最新实现检查点 — 2026-09-19：

- Branch：`main`
- 当前源码 commit：`6e9dd4d2533e55558dda2436d9effc5349de6176`
- GitHub commit/raw 可读取；`origin/main` 仍为 `2e41e9bf63495b73e2e9e8b4346e45cf5f930811`
- 候选 `agent/06-billing-main-sync@5b4b0ba628ea4fbf57cdd63a9772fd1b6a17c783` 已完成合并前审计：四个候选 commit 均不合并，最小合并集合为 `∅`
- `go test -count=1 ./model ./service`：PASS
- `go test -race -count=1 ./model ./service`：PASS
- `go test -count=1 ./...`：PASS
- `go vet ./...`：PASS
- `go build ./...`：PASS
- `git diff --check`：PASS
- MySQL/PostgreSQL 和进程级 crash/restart：`BLOCKED_RUNTIME` / `NOT RUN`

历史集成状态 — 2026-09-19（已由下述 2026-09-23 结果替代）：

- `origin/main` 已前进到 `2e41e9bf6`，并删除/回退了 RU-A Billing v2 的多项
  实现；不能把 `agent/06-billing` 直接 fast-forward 到该远端状态。
- 从旧 `main` 到 `6e9dd4d25` 的本地 fast-forward 已在隔离 worktree 完成，
  但 `git push origin main` 因远端前进返回 `fetch first`；未执行 force push。
- Billing 适配最新 `origin/main` 需要重新审计和移植，当前为
  `WAITING_INTEGRATION`；MySQL、PostgreSQL、进程级 crash/restart 仍为
  `BLOCKED_RUNTIME` / `NOT RUN`。

集成结果 — 2026-09-23：

- Billing 实现已在本轮可靠性升级中适配并合入 `main`；四版本外部数据库和
  SQLite 进程重启验证见本文前述补充，旧阻塞状态不再代表当前源码。
- `agent/06-billing` 最后独有提交只包含上述历史记录，现已合并其提交历史并
  明确标注时效；没有回退当前实现，原工作树的用户未提交修改保持不变。

## Risks / blockers

- Existing pre-release shadow ledgers were created by the legacy path. New code
  must treat their `AppliedQuota` as already applied and must not replay the
  initial reservation.
- A database outage can prevent both the transition and its reconcile marker;
  this remains a runtime/operations blocker rather than a reason to claim a
  successful recovery test.
- SQLite serializes the focused test workload and does not prove MySQL InnoDB
  or PostgreSQL row-lock/isolation behavior. `SELECT ... FOR UPDATE`, unique
  request IDs, and `RowsAffected` checks were reviewed statically but require
  real external execution.
- Product tag creation and publication are explicitly out of scope.

## Handoff

Do not change `VERSION`. Start by running the external database tests already
wired into `.github/workflows/build-release.yml` on the exact RU-A source, then
add/execute a process-level crash/restart recovery check. Inspect actual logs;
the presence of CI YAML is not release evidence.
