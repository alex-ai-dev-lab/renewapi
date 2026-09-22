# 2026 年 9 月可靠性完整升级

状态：进行中。验收范围以本文件及用户列出的 22 项核心测试为准，不能把部分完成当作完成。

## 安全基线

- 审计日期：2026-09-22。
- 基线：`main` / `origin/main` 均为 `4def3a8848e336532c55e9c3070cc78ead67e378`。
- 开发分支：`upgrade/2026-09-reliability`，工作树：`D:\Code\renewapi\upgrade-reliability`。
- 原 `source` 的 `agent/06-billing` 分支和未提交的技能文件删除保持原样。旧 Billing 功能提交已进入主线，独有的阻塞说明不作为源码事实。
- 不部署、不修改生产数据库、不调整生产默认 Billing 模式、不移动已有 tag。

## 当前源码差距审计

| 项目 | 初审状态 | 当前证据与处理 |
| --- | --- | --- |
| 请求级 Failover 状态、最多 6 个不同渠道 | TODO | `controller/relay.go` 仍用 `RetryTimes`，每个模型路由重置排除集合，同渠道兼容重试可绕过限制。 |
| Enabled、模型、排除、priority、weight | NOOP | `model/ability.go` 与缓存路径已有筛选和排除后最高剩余优先级逻辑；补集成验证。 |
| 首语义输出 watchdog | PARTIAL | 已有默认 15 秒配置，但 `stream_scanner.go` 任意 data 帧即解除。 |
| 提交前暂存与提交后协议边界 | PARTIAL | 已有 StreamStatus、恢复和提交检测，缺少通用语义提交门。保留提交后不拼流的边界。 |
| 错误分类与健康度 | PARTIAL | 已有渠道/模型/会话分类；统一前置失败重试与逐次健康记录。 |
| 公共错误、管理员完整尝试链 | TODO | 有部分脱敏和 admin_info，但错误 Log.Content 仍含真实错误且只记首次。 |
| Prompt Profile、CRUD、渠道引用 | TODO | 仅 `ChannelTestSetting.Prompt`。采用现有 GORM 迁移。 |
| Anti-Poison 保留管理员 Prompt | TODO | nonce 当前直接覆盖 Prompt。 |
| 自动测试仅 AutoDisabled | TODO | scheduler 当前同时测试 Enabled 和 AutoDisabled。 |
| default/classic Channel Test UI | TODO | 两套前端均仍在维护，均需同步。 |
| selector 主题与 autofocus | TODO | ComboboxContent 强制 dark，Chips 为 bg-transparent，按实际组件最小修复。 |
| shadow 持久化退款 | NOOP | ADR-004 和当前 BillingSession 已使用 ledger 事务；不是旧审计中的异步退款。 |
| off 退款、enforce 上线验证 | PARTIAL | off 仍有异步终态风险；现有 enforce 和 reconciler 需故障、幂等、进程重启与数据库验证。 |
| Options 主键及 pricing 回滚 | PARTIAL | Option 已声明 primaryKey，批量写入有回滚；继续验证旧表迁移与实际行为，避免重复实现。 |
| 上游可靠性移植 | PARTIAL | 账本停在 2026-08-14；重新查最新引用，逐项比较当前等价实现。 |
| Responses WebSocket | TODO | 当前仅 Realtime WebSocket 路由；在前述功能稳定后单独实现和测试。 |

## 实施与验证记录

- 已读取根目录 AGENTS、两套前端维护约定、PROJECT_STATE、ARCHITECTURE、维护文档、相关 ADR、active 任务和 UPSTREAM_PORTS。
- Go 1.25.1、Bun 1.3.14 可用。未发现 Docker、MySQL/PostgreSQL 命令；数据库运行证据待实际建立，不能把 CI 配置当作通过。
- 基线 `go test ./controller ./relay/helper ./relay/common ./model ./service` 通过。
- 第一组：独立 `ChannelFailoverState`，预算跨路由共享；同渠道不再执行已失败的兼容重试；取消停止选择；旧 `RetryTimes=0` 不压缩新预算。真实 HTTP 集成覆盖六次失败、前五次失败第六次成功、两个 priority 100 先于 90、禁用渠道排除、只结算成功请求。`go test ./controller ./service ./relay/common` 通过。

## 后续执行顺序

1. Failover 状态、语义 staging、统一错误和日志、故障注入。
2. Channel Test 数据模型、CRUD、引用安全、scheduler、两套前端。
3. Billing enforce 验证和必要补修，Options 兼容验证。
4. 最新上游逐项审计与最小移植。
5. Responses WebSocket，逐轮计费、取消、错误关联与容量边界。
6. 完整测试、独立复核、聚焦提交、更新文档及开发分支推送。
