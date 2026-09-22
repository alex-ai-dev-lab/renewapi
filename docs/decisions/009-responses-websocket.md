# ADR-009：Responses WebSocket 与逐轮隔离

状态：接受。日期：2026-09-23。

## 决策

- 新增 `GET /v1/responses`，与 `/v1/realtime` 分开。每个 `response.create` 建立新的请求 context，重新执行鉴权、Token/模型限流、路由、请求校验、Anti-Poison、Failover 和 BillingSession；握手只占用连接容量。
- 支持默认 lane 和符合官方格式的 `stream_id`，每连接最多 32 个 lane。同 lane 按 FIFO 串行，不同 lane 并行。错误关联客户端 `event_id`、`stream_id` 及已提交的 `response_id`；取消只影响目标轮次。
- 默认将现有 HTTP/SSE Responses 上游转成 WebSocket 消息。渠道 `setting.responses_websocket=true` 时，仅替换当前 Responses 请求的传输层为原生上游 WebSocket。URL、模型映射、参数/头部覆盖、代理和 TLS 规则仍由原路径产生。
- 每个客户端连接的每个 lane 拥有独立上游连接，不跨用户、Token 或连接共享。渠道、认证、URL、请求头、代理或 TLS 设置变化时重新握手。常驻 reader 负责闲置期间的 ping/close，异常和取消使连接失效。
- 复用 ADR-006 的首语义 watchdog、提交前暂存和六渠道预算。已提交当前轮次内容后绝不拼接另一渠道输出。迟到 terminal、response ID 冲突和无法解析的事件不能作为下一轮成功。
- 每轮创建唯一上游 `event_id`，避免客户端复用标识造成旧错误误关联。上游明确属于其他 lane 或 event 的错误不能终结当前轮次。
- 客户端 terminal 延后到 Relay、账本及限流中间件退出之后。结算失败只返回公共错误；持久化 `reconcile_required` 保留结算意图，管理员日志保留 `billing_error`，补偿器继续恢复。
- 连接内保存有限的完整 input/output 快照。已知 `previous_response_id` 先展开完整输入供鉴权、校验和预扣估算；原生连接仅在前次实际请求与输出前缀完全一致时发送增量。换渠道、换密钥、表示改变时发送完整上下文，不重放当前轮次已提交内容或工具调用。
- `store=false` 且快照不可用时返回明确恢复错误，要求客户端发送完整 input。默认/显式 `store=true` 的未知 ID 可交给上游持久化状态处理，但不臆造本地历史。
- `generate:false` 在本地完成校验和输入预热，不调用上游、不计费；不声称预热上游模型。工具、instructions 等请求级参数仍由客户端在生成轮次提供。
- 连接寿命 60 分钟。排队与执行中的输入共同受 32 轮/16 MiB 上限约束，历史最多 64 份/16 MiB，恢复后仍检查请求体与连接容量。连接数默认每进程 256、每 Token 4，可通过环境变量调整。
- 进程关闭时显式取消并等待 hijacked 连接的 worker，再关闭日志、账本和数据库。不能仅依赖 `http.Server.Shutdown`。

## 验证与边界

真实本地 WebSocket/HTTP 服务覆盖连接复用、续写、密钥轮换、跨 lane 并行、同 lane FIFO、逐轮 Token 模型/RPM/concurrency 校验、容量释放、输入上限、warmup、取消、进程关闭、六渠道切换、统一错误、迟到事件和结算补偿；专项 race 通过。完整验收证据见当前升级任务。

本轮不实现官方后来扩展的 mid-turn steering，也不建立跨连接共享池。客户端断线后需用完整 input 恢复 `store=false` 会话。流已提交后的恢复仅限既有安全边界，不自动重新执行已交付工具调用。
