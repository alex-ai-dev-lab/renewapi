# ADR-006：请求级 Failover 与首语义提交

状态：接受。日期：2026-09-22。

## 决策

- 一个请求最多尝试六个不同 Channel ID（初始渠道加五次切换）。预算跨模型、分组、route plan 和恢复逻辑共享，通用 `RetryTimes` 不再代表此预算。
- 已尝试渠道不重新选择，包括同渠道的 key、请求格式和映射模型兼容重试。新的请求仍可使用现有映射和会话路由能力。
- 复用现有 Enabled、模型支持、排除集合、priority 与 weight 筛选，保持既有能力与会话约束。
- 继续使用 `RELAY_FIRST_BYTE_TIMEOUT` 配置键，默认 15 秒，语义升级为从渠道调用开始到首个可交付语义输出的期限；HTTP headers、keepalive、空行、response.created 和 usage 不解除期限。
- 在最终客户端协议写入层暂存 SSE 和响应头。适配器转换及 Anti-Poison 校验通过后的内容、推理、拒绝、工具或受支持的多模态语义输出触发提交。
- 前导暂存上限为 1 MiB，单帧继续使用现有 scanner 上限；超限、格式错误、空流、超时与提交前失败丢弃暂存并切换。
- 已有内容提交后不得在同一流拼接另一渠道。既有会话恢复可修复下一请求的路由；当前流失败结束并记录真实原因。
- 结算前验证 staging 与协议 terminal，失败尝试不得提前结算。同一请求保留既有 BillingSession 及持久化账本幂等键。

## 验证

真实 HTTP 故障注入覆盖 401、403、429、500、502、503、连接拒绝、TLS 错误、empty SSE、前导后 EOF、malformed SSE、response.failed、15 秒前导与 keepalive stall、取消和提交后断流。跨协议 writer 回归覆盖 Responses、OpenAI、Claude、Gemini、工具调用以及响应头隔离。
