# Billing Ledger enforce 验证与切换

默认仍为 `BILLING_LEDGER_MODE=shadow`，本轮没有切换生产模式。

## 模式语义

| 模式 | reserve / settle / refund | 用量统计 | 进程重启后补偿 |
| --- | --- | --- | --- |
| shadow | 持久化账本及资金、Token 原子事务 | 保留兼容统计路径 | 支持 |
| enforce | 同上 | 账本拥有余额及统计写入，避免重复计数 | 支持 |
| off | 新请求复用同一持久化账本及资金、Token 原子事务 | 保留兼容路径 | 支持新请求的持久化意图与孤儿预扣恢复 |

2026-10-17 起，`off` 保留模式名和兼容统计归属，但新请求也写入既有 `BillingLedger`；首次/追加预扣、结算、退款及任务终态共用持久事务和幂等恢复，不新增第二套账本。账本故障会使请求失败，不回退到无持久保护的扣款。

此保证覆盖已落盘意图和超时孤儿预扣，不推算进程退出前尚未持久化的真实上游用量；off/shadow 的兼容统计也不承诺崩溃后与余额同时恢复。升级旧 off 实例须先排空旧在途请求；旧版本未写账本的历史差额只能根据历史凭据人工核实，不自动回填或重新预扣。

同一会话的 reserve、settle、refund 共用终态锁。补充预扣失败会回滚，保留原 reservation；不把仍在执行的请求交给 reconciler 提前结算。Failover 首次成功结算及其补偿意图保存最终成功渠道，失败渠道的用量不会被误记为成功消费。

## 开发验证

- `go test ./model ./service ./controller`：钱包和订阅、预扣与补充预扣、终态幂等、故障回滚、成功渠道归属。
- `go test -race ./service ./model -run 'TestBillingSession|TestBillingReconciler|TestBillingLedger|TestShadowLedger|TestShadowSubscription' -count=1`。
- `TestBillingReconcilerSurvivesProcessRestart`：off/shadow/enforce × 钱包/订阅 × 待结算/待退款/孤儿请求/未提交事务，共 24 组独立进程强制退出与重启，只复用原 SQLite 文件；重复补偿不改变余额。异步任务终态另覆盖三模式，避免账本与兼容路径重复扣退。
- `.github/workflows/reliability-validation.yml`：由统一发布流程调用，运行 MySQL 5.7/8.4、PostgreSQL 9.6/16 的迁移和账本行为。四版本验证全部成功后才允许构建镜像；不会部署生产。
- 升级阶段的 `ea9a4f994` 和 `a2321c2b1` 均已通过四数据库验证，SQLite 独立进程崩溃/重启及相关本地 race 也已通过。历史运行的清理和证据保存位置见发布维护任务。
- 发布维护后的 [Actions 35806368451](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35806368451) 在 `8d8bef09a9b7980c0e490a049621583bc1842724` 上再次通过上述四版本数据库、全仓 Go test/vet/build 及发布流程的定向 race。对应 [Release](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-build-8d8bef09a9b7-35806368451) 的 `qa-databases.tar.gz` 保存各数据库日志，生产模式仍未切换。
- 本地外部数据库测试使用 `BILLING_TEST_DRIVER`、`BILLING_TEST_DSN`；迁移使用既有 `REQUEST_GUARD_TEST_DRIVER`、`REQUEST_GUARD_TEST_DSN`。这些用例会创建、清理测试表，必须指向隔离测试库。

## 上线准备

1. 先在隔离副本运行 `migrate --up`，再运行 `migrate --check`；验证应用提交和测试证据一致。
2. 检查 `billing_ledgers` 的 `reconcile_required`、`last_error`、组件状态及 `billing_outboxes`，处理现有差额。shadow 已经使用持久化余额事务，无需重新扣除历史 reservation。
3. 通过配置将目标实例设为 `BILLING_LEDGER_MODE=enforce`。已有账本继续按其记录的 Mode 恢复，不重写历史模式。
4. 对照钱包余额、订阅额度、Token remain/used quota、用户及渠道统计和账本 applied/counted quota；验证取消、失败与重复回调。
5. 保留 reconciler。`BILLING_RECONCILE_INTERVAL_SECONDS` 默认 15 秒，单批默认 100；孤儿任务默认 900 秒，普通请求默认 21600 秒。普通请求窗口必须大于允许的最长请求时间。
6. 如需回到 shadow，只修改新请求配置；先让已存在的 enforce 账本完成对账，不能删除账本、补偿记录或清零余额。

生产切换仍由运维在确认对应提交的真实数据库验证结果后执行。
