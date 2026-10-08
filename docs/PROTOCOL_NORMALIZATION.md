# 模型默认端点与三协议互转

在“系统设置 → 运营设置 → 模型默认端点”启用模型规则和“将文本请求统一转换到模型默认端点”。现有规则无需重新录入；新开关关闭时继续使用旧兼容行为。

| 客户端入口 | 模型默认端点 | 实际行为 |
|---|---|---|
| Messages | Responses | 转换请求并调用 Responses，结果转回 Messages |
| Responses | Messages | 转换请求并调用 Messages，结果转回 Responses |
| Chat Completions | Messages | 转换请求并调用 Messages，结果转回 Chat |
| Messages | Chat Completions | 转换请求并调用 Chat，结果转回 Messages |
| Chat Completions | Responses | 转换请求并调用 Responses，结果转回 Chat |
| Responses | Chat Completions | 转换请求并调用 Chat，结果转回 Responses |

相同协议保持原有路径。本文的 Messages 路径为 `/v1/messages`，Responses 为 `/v1/responses`，Chat 为 `/v1/chat/completions`。本次没有新增 `/v1/message` 单数别名。

## 配置和排障

- 模型配置精确匹配优先，其次取最长前缀。通用聚合渠道保留当前 Base URL，只选择对应的请求协议；原生供应商继续使用自己的适配器和认证。
- 渠道的明确模型端点覆盖优先于全局规则。新整流模式与原来的单渠道协议覆盖开关有不同职责，不能仅从旧开关推断新模式是否生效。
- 编辑渠道的高级设置中可运行“路由预览”，查看保存配置的目标协议和来源；不会调用上游。预览不代表上游服务此刻一定正常，也不代替包含具体请求字段的能力验证。
- 管理员日志详情展示客户端路径 → 实际上游路径及逐次尝试；列表优先显示脱敏真实原因。没有合格渠道时也记录日志，避免只在容器日志中看到错误。
- 余额不足、上游分组权限、网络/TLS、上游自身空流不会由协议转换自动修复。自动禁用状态和恢复时间窗仍然生效。

## 兼容范围

支持三方向组合的文字、普通图片输入、JSON 函数工具、工具选择、并行开关、混合文字与工具历史、两轮结果回传、流式和非流式响应；保留显式零值与用户输出预算。思考配置按目标协议表达，预算到 effort 属于级别映射，不表示两家供应商的计算量相同。

不同协议并非所有专有字段等价。Responses 会话 ID/压缩状态、未知工具类型、外国签名和没有明确表示的扩展会被拒绝；不声称任意 MCP/服务端工具、加密历史和所有媒体任务都能无损互转。图片生成、音频、视频、embeddings、Realtime 和 compact 等专用任务不走普通文本互转。

已有正文透传设置与需要转换的请求冲突时，拒绝该候选，不把未转换的正文送到不同协议接口。普通用户的权限和计费身份保持原有规则；不会因改变端点自动改变模型价格、放宽模型白名单或重复结算。

## 验证入口

```sh
go test ./controller -run 'TestModelDefaultProtocolMatrix|TestNormalizedBridgeFailover' -count=1
go test ./service/openaicompat -run 'TestDirectClaudeResponses' -count=1
```

使用本地 HTTP 上游桩和内存 SQLite，不会调用付费模型。浏览器验证见 `scripts/qa/frontend-repair-check.cjs` 的 protocol normalization 用例。源码行为以 ADR-016 与测试为准。
