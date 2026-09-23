# Changelog

## Unreleased

### 新增

- 请求级渠道 Failover：初始渠道加最多五次备用切换，同一 Channel ID 不重复尝试；继续遵守模型、Enabled、priority 和 weight 规则。
- Channel Test Prompt Profile 管理、每渠道提示词、默认回退和引用拒删；增加默认关闭的“自动测试仅 AutoDisabled”模式。两套前端均可配置。
- 独立 Responses WebSocket Relay，支持逐轮鉴权、限流、计费、lane 隔离、取消、有限历史恢复和连接容量限制。默认 HTTP/SSE 桥接，渠道可启用原生 WS 上游。

### 修复

- 修复没有调用记录时仪表盘遍历空趋势数据崩溃的问题；统计接口统一返回空数组，前端兼容旧接口的 null 集合。
- 首页不再把预设或模拟指标显示为实时数据；展示实际服务状态、版本和启动时间，接口故障时明确提示不可用。
- 首页注册、模型广场、文档和政策入口跟随实际配置；补齐中文文案、示例模型占位符、代码标签页键盘操作和复制失败提示。
- 修复首页深色主题下导航与代码块文字不可读、部分文字对比度不足、重复跳转主内容入口及移动端图示无法键盘滚动。
- 会话校验遇到临时故障保留登录状态并允许重试；列表查询失败保留当前页面；初始化检查失败后不再永久跳过重查。
- 不支持的渠道测试返回正常错误，取消测试会终止上游请求；预检失败不覆盖上游健康和延迟。异常恢复不向客户端泄露内部信息，也不向已提交流追加 JSON。
- 原首字节超时升级为默认 15 秒首语义输出期限；keepalive、response.created 和协议前导不解除超时。提交前暂存失败可安全切换，提交后不拼接其他渠道的输出。
- Relay 客户端和用户错误日志统一 capacity 文案；管理员保留真实错误及完整尝试链，前序失败独立参与健康度判断。清理错误事件额外 debug/metadata，修正 WS 事件关联和真实错误状态记录。
- Anti-Poison 在管理员测试提示词后追加 nonce，保留原测试语义；修复自动测试读取失败后的运行锁与错误恢复边界。
- 修复渠道模型选择器 light/dark 背景和 dialog 打开时 dropdown 自动展开。
- 保留 shadow 已有持久化余额事务；off 退款改为同步原子资金/Token 事务，失败可重试。补齐计费终态幂等、失败补偿、进程恢复和 Failover 成功渠道用量归属。
- Responses 缺失或 null 的 terminal usage 使用内容估算，完整终态输出不会与 delta 重复相加；保留原生显式零及缓存、推理明细。补齐跨协议 usage 请求、工具参数 done 去重与管理员返回模型诊断。

### 配置与兼容

- 新增 `channel-test-prompts:v1` 和 `options-primary-key:v1` 迁移；旧提示词继续回退，旧 Options 配置保留，已有主键无需重建。
- `RELAY_FIRST_BYTE_TIMEOUT` 沿用原配置键；`BILLING_LEDGER_MODE` 默认继续为 shadow，enforce 切换条件和真实数据库验证见 `docs/billing-ledger-enforce.md`。
- 新增 WS 连接容量配置及渠道 `setting.responses_websocket` 开关。`generate:false` 仅预热本地输入；不支持 mid-turn steering，`store=false` 断线后需要完整 input 恢复。详见 `docs/responses-websocket.md`。

## v1.0.0-rc.2 - 2026-08-15

### Changed

- RequestGuard observe workers now start and stop with the server process,
  resize with configuration, and drain through a bounded shutdown path.

### Fixed

- Built-in OAuth and WeChat account binding no longer risks overwriting
  concurrent quota, status, group, or profile updates.
- Editing an existing redemption code in either frontend preserves its exact
  stored quota unless the amount or native quota is intentionally changed.
- Request replay now has regression coverage for concurrent readers,
  multipart bodies, and large disk-backed payloads.
- Async task refunds, concurrent recharge callbacks, and fallback repricing
  are covered by exactly-once billing invariants.

### Security

- Token regeneration and affiliate quota transfer now have isolated per-user
  critical rate limits.
- RequestGuard inspects bounded Responses and Claude tool-result text while
  excluding binary media, and blocked or fail-closed requests stop before
  routing, pricing, billing, channel selection, or upstream dispatch.
- Protected fetches reject redirects from a public URL to a private network
  target before issuing the redirected request.

## v1.0.0-rc.1 - 2026-08-15

### Changed

- Product versions became pure RenewAPI SemVer values; product Git tags use
  the `renewapi-` namespace while upstream raw `v*` tags remain untouched.
- The independent RenewAPI prerelease sequence was established at
  `v1.0.0-rc.1` under the product tag `renewapi-v1.0.0-rc.1`.

Release notes are maintained at the product-version level. Do not copy every
commit into this file; use Git history for implementation detail and record
only user-visible behavior, configuration, compatibility, security, migration,
and breaking changes here.
