# Responses WebSocket 使用与配置

连接 `ws(s)://网关/v1/responses`，使用与 HTTP Responses 相同的 `Authorization: Bearer ...`。这不是 `/v1/realtime` 音频协议。

```json
{"type":"response.create","stream_id":"main","event_id":"turn_1","model":"你的模型","store":false,"input":"hi"}
```

网关返回普通 Responses 事件，例如 `response.output_text.delta`、`response.completed`。指定 `stream_id` 时每个事件回传相同 ID；未指定时使用默认 lane，事件不添加该字段。ID 长度为 1–256，仅支持 ASCII 字母、数字、下划线、点和连字符。

完成后可以提交增量输入：

```json
{"type":"response.create","stream_id":"main","event_id":"turn_2","model":"你的模型","store":false,"previous_response_id":"resp_上一轮ID","input":"继续"}
```

同 lane 顺序执行，不同 lane 可以并行。`response.cancel` 取消对应 lane 当前轮次，可附带 `response_id` 防止误取消。`generate:false` 只进行本地输入预热并返回可续写的 response ID；不请求上游，不计费，也不预热上游模型。下一生成轮次仍需提供 model、tools、instructions 等请求级设置。`background:true` 和 mid-turn steering 不受支持。

默认通过现有 HTTP/SSE 上游转发。若渠道支持原生 Responses WebSocket，可在两套前端的渠道高级设置中开启“原生 Responses WebSocket”，对应 `setting.responses_websocket=true`。此开关只影响 WebSocket 客户端，并继续复用模型映射、参数覆盖、上游请求头、代理和 TLS 配置。

| 配置 | 默认 | 含义 |
| --- | --- | --- |
| `RESPONSES_WS_MAX_CONNECTIONS` | `256` | 每进程客户端连接上限 |
| `RESPONSES_WS_MAX_CONNECTIONS_PER_TOKEN` | `4` | 每进程、每 Token 连接上限 |
| `RELAY_FIRST_BYTE_TIMEOUT` | `15` | 兼容旧配置键，实际为首语义输出期限，单位秒 |

连接最多保留 60 分钟；每连接最多 32 个 lane、32 个排队或执行中轮次。请求体和恢复后的输入受既有 `MAX_REQUEST_BODY_MB` 及 16 MiB 连接上限约束。历史最多保留 64 份、合计 16 MiB。容量限制会返回带关联 ID 的错误，不会在后台静默继续生成。

首语义输出前的 keepalive、`response.created` 和失败内容不会提交给客户端。最多尝试六个不同渠道；全部失败返回公共 `model_capacity` 错误。提交后断流正确结束当前流，不拼接另一渠道输出。每轮只有一个 BillingSession，terminal 在结算及限流释放之后发送。

`store=false` 的恢复快照仅属于当前连接，断线、淘汰或无法形成完整输出快照后，客户端应去掉 `previous_response_id` 并发送完整 input。`store=true` 的跨连接恢复取决于上游持久化能力。原生上游重新连接时，只有持有完整上下文才安全回放。

管理员可在 Usage Logs 的 `Other.admin_info` 中检查尝试链、真实错误、用量、返回模型、WebSocket `stream_id` 和 `billing_error`。普通用户只能看到公共错误。账本补偿及上线条件见 [Billing enforce 说明](billing-ledger-enforce.md)。
