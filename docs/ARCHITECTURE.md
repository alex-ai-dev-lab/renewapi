# RenewAPI Architecture

## Product boundary

RenewAPI is a downstream source fork of NewAPI. The Go module path and core
API/data model remain upstream-compatible, while compatibility bridges, routing
hardening, billing safeguards, anti-poison controls, and deployment tooling
are fork-owned. NewAPI, Sub2API, and CPA changes are reviewed as references;
they are not merge sources.

## Runtime layers

```text
HTTP router
  -> controller
  -> service
  -> model/GORM
  -> SQLite, MySQL, or PostgreSQL
```

Relay traffic follows the same route/controller boundary, then enters the
provider adapter pipeline under `relay/`. Shared concerns live in
`middleware/`, `common/`, `setting/`, `dto/`, `types/`, and `pkg/`.

## Fork-owned surfaces

- `pkg/compat/`: compatibility hooks, error normalization, schedulers, and
  price synchronization.
- `relay/antipoison/`: channel risk profiles, probes, envelope checks, opaque
  payload scanning, and tool-call guards.
- `service/requestguard/`: request admission and audit persistence.
- `scripts/`: upstream audit, build, release, deploy, rollback, and secret
  loading helpers.
- `web/default/` and `web/classic/`: the primary and compatibility frontend
  bundles embedded by the Docker build.

## Version and build metadata

`VERSION` is the RenewAPI product version and is injected into the frontend
build and `common.Version` through the existing Docker linker path. The
runtime accepts a `VERSION` environment override for compatibility. `/api/status`
and `/healthz` expose this product version.

RenewAPI product Git tags use `renewapi-v<version>`; raw upstream `v*` tags are
not product releases. Docker product tags strip the leading `v`.

Git commit, build time, build channel, and audited upstream reference are
separate Docker image labels and release-note metadata. They must not be
encoded into the product version.

## Data and compatibility boundaries

- All three supported databases are first-class compatibility targets.
- Provider adapters should use shared relay/request conversion and error
  normalization paths rather than duplicating routing logic.
- Database migrations and concurrency-sensitive updates require focused tests
  on SQLite plus the relevant external database checks.
- Feature flags and existing defaults remain stable unless an explicit
  compatibility decision is recorded in an ADR.

## 请求级可靠性边界

`RelayInfo.Failover` 在整个请求内保存渠道预算和 attempts，模型、分组和恢复路由共享最多六个不同 Channel ID。路由继续复用现有 Enabled、模型支持、排除、priority 和 weight 选择；手动指定渠道及既有能力约束保持有效。

`relay/helper/stream_staging.go` 位于最终协议写入层，协议转换和 Anti-Poison 校验后的真实内容才触发提交。`relay/common/stream_semantics.go` 共用跨协议语义分类；`RELAY_FIRST_BYTE_TIMEOUT` 沿用旧配置键，含义变为默认 15 秒首语义期限。暂存失败不污染客户端；提交后错误只结束当前流，不从另一个上游重新拼流。结算前同时检查暂存门和协议终态。

公共错误由 `types.PublicRelayError` 和协议错误事件入口统一处理。真实错误、尝试链、timeout stage、上游 request ID、返回模型和观察到的用量保存在既有日志 `Other.admin_info`；用户日志读取还会清理历史数据。每次失败独立更新健康度，最终成功不抹除前序故障。见 [ADR-006](decisions/006-channel-failover-and-semantic-commit.md)。

## 持久化扩展

- `channel_test_prompts` 独立保存 Prompt Profile，渠道用 `channel_test_prompt_id` 引用。删除和引用写入共用事务锁，禁止悬空引用；可空唯一槽位保证默认项唯一。迁移为 `channel-test-prompts:v1`，见 [ADR-007](decisions/007-channel-test-prompt-profiles.md)。
- `options-primary-key:v1` 只修复缺少实际主键的旧 Options 表；SQLite 重建时保留原值、索引、触发器及外键引用，存在歧义数据就停止迁移。定价写入继续使用原有事务和运行时回滚，见 [ADR-008](decisions/008-options-primary-key-migration.md)。
- Billing 的 reserve/settle/refund 保持既有账本与 outbox 结构。shadow/enforce 使用持久化资金与 Token 事务，enforce 同时拥有统计写入；失败终态由 reconciler 恢复。off 的同步原子退款只保证当前会话重试，不提供跨进程账本恢复。默认仍为 shadow，见 [enforce 验证说明](billing-ledger-enforce.md)。

## Responses WebSocket

`GET /v1/responses` 是独立入口，与 Realtime 分开。握手执行鉴权和连接容量检查；每个 `response.create` 经独立 Gin context 再次执行 TokenAuth、Token/模型限流、Distribute 和 Relay。每轮拥有自己的 Failover 与 BillingSession，terminal 延迟到账本和限流中间件退出后发送。

默认桥接已有 HTTP/SSE 上游；渠道开启 `setting.responses_websocket` 时使用原生 WebSocket transport。每个客户端连接的每个 lane 独占上游连接；URL、渠道、认证、请求头、代理或 TLS 设置变化时重新握手。常驻 reader 处理 idle ping/close，并校验 lane、response 和明确请求关联，避免迟到事件结束后续轮次。

历史快照、排队请求、输入体积、lane 数和连接数均有上限。同 lane FIFO，不同 lane 可并行；取消只影响目标轮次，断开取消全部。恢复先展开完整上下文校验和预扣，只有原生前缀一致时才发送增量；流提交后不跨渠道续接。进程退出显式等待 hijacked WS worker 后再关闭数据库。配置与已知边界见 [WS 说明](responses-websocket.md) 和 [ADR-009](decisions/009-responses-websocket.md)。
