# ADR-004: Durable Shadow Billing Balance Reconciliation

Status: Accepted

Date: 2026-08-17

## Context

RenewAPI keeps `BILLING_LEDGER_MODE=shadow` as the default for compatibility
and observation. The previous shadow path applied wallet, subscription, and
token changes through separate legacy calls, then marked the in-memory
`BillingSession` terminal. A failure between those calls could leave a user
balance and the ledger permanently inconsistent, while a process restart lost
the retry context.

## Decision

Shadow ledger reservations and all later balance transitions use the existing
`BillingLedger` transaction and outbox path. The ledger applies wallet,
subscription, and token balance legs atomically for `off`, `shadow`, and
`enforce` modes (extended to new off requests on 2026-10-17). `enforce` remains
the owner of usage/request counters; all new sessions own async task terminal
insertion through the ledger, while off/shadow retain their existing
compatibility counter paths.

Each ledger records funding, token, subscription, and statistics component
states. A transition is terminal only after all applicable balance legs commit.
The existing `request_id` uniqueness and ledger row lock provide idempotency;
reconciliation replays the signed delta from `AppliedQuota` to `DesiredQuota`.

The default mode remains `shadow`. Explicit `off` preserves its mode name and
statistics ownership, but new requests now share durable balance recovery.
Pre-upgrade off requests without a ledger cannot be reconstructed automatically.

## Compatibility and migration

- The change is additive: component-state columns are added by
  `billing-ledger:v2` and have defaults for existing rows.
- Existing shadow ledgers already store the legacy pre-consumed amount in
  `AppliedQuota`; a later transition adjusts only the delta from that value,
  preventing replay of the initial reservation.
- SQLite, MySQL, and PostgreSQL use GORM `AutoMigrate`; no database-specific
  SQL or lock hint is introduced.
- Existing API responses and product version remain unchanged.

## Validation

Focused SQLite tests cover atomic rollback when the second balance leg fails,
retry after restoring the failed component, duplicate settle/refund, repeated
reconciler delivery, concurrent terminal transitions, subscription extra
reserve/refund, missing-row `RowsAffected`, legacy-ledger compatibility, and
the default shadow `BillingSession` and async pending-task paths. External
MySQL/PostgreSQL execution and application-process crash/restart checks remain
release gates; adding CI steps without an inspected run is not pass evidence.

## 2026-09-23 补充

默认继续为 shadow。off 的退款改为同步资金/Token 原子事务，成功后才设置会话终态；off 仍不提供跨进程恢复保证。所有模式的退款与结算共用会话锁。补充预扣失败不再提前安排结算；首次结算及补偿意图记录最终成功渠道，保证 Failover 用量归属正确。

新增独立进程崩溃/重启验证，以及开发分支专用 MySQL 5.7/8.4、PostgreSQL 9.6/16 验证工作流。最终源码 `a2321c2b1` 的 [Actions 35773097071](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35773097071) 四个数据库 job 均成功；SQLite 独立进程验证通过，上文原日期的缺失执行证据已补齐。执行配置和切换条件见 [enforce 验证说明](../billing-ledger-enforce.md)。

## 2026-10-17 补充

本轮将 off 新会话统一接入既有 BillingLedger/outbox，首次及追加预扣、结算和退款两腿在同一事务提交，跨进程继续恢复持久化意图和超时孤儿预扣；不新建账本、不更改默认 shadow 或统计归属。异步任务只由持久账本调整余额，避免 shadow/off 在兼容路径重复扣退。

此补充替代上述 9 月记录中 off 无持久恢复的现行限制；9 月文字仅保留历史语境。旧 off 无账本差额不猜测回填，升级先排空旧在途请求。未持久化的真实上游用量仍不可恢复，off/shadow 兼容统计保留原边界。24 组独立进程验证覆盖三模式、钱包/订阅与四种崩溃状态；真实 MySQL 5.7/PostgreSQL 9.6 余额及任务回归通过。支付快照及限购边界另见 [ADR-020](020-audit-repair-boundaries.md)。
