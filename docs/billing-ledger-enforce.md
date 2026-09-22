# Billing Ledger enforce 验证与切换

默认仍为 `BILLING_LEDGER_MODE=shadow`，本轮没有切换生产模式。

## 模式语义

| 模式 | reserve / settle / refund | 用量统计 | 进程重启后补偿 |
| --- | --- | --- | --- |
| shadow | 持久化账本及资金、Token 原子事务 | 保留兼容统计路径 | 支持 |
| enforce | 同上 | 账本拥有余额及统计写入，避免重复计数 | 支持 |
| off | 兼容预扣；结算和退款为同步原子事务 | 保留兼容路径 | 不提供持久化账本保证 |

`off` 退款只有事务成功后才标记完成。失败可在同一会话重试；需要自动补偿或跨进程幂等时使用 shadow/enforce。不要把 off 当作账本故障的自动回退方式。

同一会话的 reserve、settle、refund 共用终态锁。补充预扣失败会回滚，保留原 reservation；不把仍在执行的请求交给 reconciler 提前结算。Failover 首次成功结算及其补偿意图保存最终成功渠道，失败渠道的用量不会被误记为成功消费。

## 开发验证

- `go test ./model ./service ./controller`：钱包和订阅、预扣与补充预扣、终态幂等、故障回滚、成功渠道归属。
- `go test -race ./service ./model -run 'TestBillingSession|TestBillingReconciler|TestBillingLedger|TestShadowLedger|TestShadowSubscription' -count=1`。
- `TestBillingReconcilerSurvivesProcessRestart`：独立进程强制退出后，用同一 SQLite 文件启动新进程，验证待结算、待退款、孤儿请求及未提交事务回滚；重复补偿不改变余额。
- `.github/workflows/reliability-validation.yml`：仅验证开发分支，运行 MySQL 5.7/8.4、PostgreSQL 9.6/16 的迁移和账本行为，不构建发布镜像或部署。
- 2026-09-23 已检查 [Actions 35764799379](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35764799379)，提交 `ea9a4f994` 的上述四个数据库 job 全部成功；SQLite 独立进程崩溃/重启测试在本地通过。生产切换仍不属于此次源码任务。
- 本地外部数据库测试使用 `BILLING_TEST_DRIVER`、`BILLING_TEST_DSN`；迁移使用既有 `REQUEST_GUARD_TEST_DRIVER`、`REQUEST_GUARD_TEST_DSN`。这些用例会创建、清理测试表，必须指向隔离测试库。

## 上线准备

1. 先在隔离副本运行 `migrate --up`，再运行 `migrate --check`；验证应用提交和测试证据一致。
2. 检查 `billing_ledgers` 的 `reconcile_required`、`last_error`、组件状态及 `billing_outboxes`，处理现有差额。shadow 已经使用持久化余额事务，无需重新扣除历史 reservation。
3. 通过配置将目标实例设为 `BILLING_LEDGER_MODE=enforce`。已有账本继续按其记录的 Mode 恢复，不重写历史模式。
4. 对照钱包余额、订阅额度、Token remain/used quota、用户及渠道统计和账本 applied/counted quota；验证取消、失败与重复回调。
5. 保留 reconciler。`BILLING_RECONCILE_INTERVAL_SECONDS` 默认 15 秒，单批默认 100；孤儿任务默认 900 秒，普通请求默认 21600 秒。普通请求窗口必须大于允许的最长请求时间。
6. 如需回到 shadow，只修改新请求配置；先让已存在的 enforce 账本完成对账，不能删除账本、补偿记录或清零余额。

生产切换仍由运维在确认对应提交的真实数据库验证结果后执行。
