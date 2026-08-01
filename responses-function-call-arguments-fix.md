# Responses `function_call.arguments` 兼容方案（按渠道配置）

## 结论

`function_call.arguments` 不是所有 Responses 上游都要求同一种 JSON 类型：

- Chat Completions 的工具调用参数天然是 JSON string。
- 部分严格 Responses 上游要求 `input[].arguments` 是 object。
- 当前方案不能把所有请求无条件改成 object，否则会破坏仍要求 string 的 OpenAI-compatible 上游。

因此最终采用“渠道能力声明 + 单一规范化函数 + 透传边界明确”的方案：

| 渠道设置 | 实际格式 | 是否改写请求 |
| --- | --- | --- |
| `auto` | OpenAI 默认 string；Codex 默认 object | 只有 Codex/路由强制场景改写 |
| `string` | JSON string | 是，object/其它 JSON 值序列化为 string |
| `object` | JSON object | 是，string 解析为 object；非法值本地 400 |

渠道设置字段为 `responses_function_call_arguments_format`，编辑器位于：

```text
web/default/src/features/channels/lib/channel-form.ts
web/default/src/features/channels/components/drawers/channel-mutate-drawer.tsx
dto/channel_settings.go
```

## 已确认的源码链路

### Chat -> Responses

`service/openaicompat/chat_to_responses.go` 仍把 Chat tool call 的
`Function.Arguments` 作为 string 写入 Responses input。这是协议转换的原始表示，不能在这里硬编码 object。

真正发往上游前，以下两条链路会复用同一个函数：

```text
relay/chat_completions_via_responses.go
  -> adaptor.ConvertOpenAIResponsesRequest
  -> service.NormalizeResponsesFunctionCallArgumentsPayload

relay/responses_handler.go
  -> adaptor.ConvertOpenAIResponsesRequest
  -> service.NormalizeResponsesFunctionCallArgumentsPayload
```

统一实现位于：

```text
service/responses_arguments_format.go
```

### 原生 Responses 和透传

`relay/responses_handler.go` 根据 `EffectiveResponsesFunctionCallArgumentsFormat` 和
`ShouldEnforceResponsesFunctionCallArgumentsFormat` 决定是否执行规范化。启用显式 `string/object`、Codex 默认规则或重试路由强制 object 时，透传 body 不再绕过规范化；未启用时保留原始请求语义。

这保证了两件事：

1. 需要严格 object 的上游不会收到 string。
2. 未声明 object 能力的 OpenAI-compatible 上游不会被平台擅自改成另一种类型。

### 规范化行为

object 模式：

- 缺失、`null` 或空 string → `{}`；
- JSON object string → object；
- 非法 JSON、JSON 数组、数字、布尔或其它非 object → 本地 400，标记 skip-retry；
- 已经是 object → 保持不变。

string 模式：

- 缺失/`null` → `{}`；
- 已经是 string → 保持原值；
- object、数组、数字、布尔 → JSON 序列化为 string。

非法参数不应静默改成 `{}`，因为那会丢失工具调用语义并掩盖客户端问题。

## 与 Responses compaction/model mapping 的边界

这次参数类型修复不能和 compaction 路由混成一个“万能转换器”。当前另有两项独立保护：

- `relay/helper/model_mapped.go` 在 `/v1/responses/compact` 保留原始虚拟模型作为 mapping 起点，避免别名链把 compact 请求错误落到普通模型。
- `relay/responses_handler.go` 在启用 pass-through 且上游模型不同于客户端模型时，仅重写 body 的 `model` 字段；无需改写时继续使用 ReaderOnly，避免大 body 的额外复制。

这两项属于模型映射/请求体生命周期，不应通过修改 `arguments` 规范化函数来解决。

## 回归测试与验收

至少覆盖：

1. `service/responses_arguments_format_test.go`
   - object 模式把 `{"query":"weather"}` 转成 object；
   - 非法 string 被拒绝；
   - string 模式把 object 转回 JSON string；
   - `auto`、Codex 默认和强制 object 的有效格式正确。
2. Chat -> Responses 的三种消息内容形态都能进入统一规范化路径，不能只测试一种 assistant content。
3. `relay/responses_handler.go` 和 `relay/chat_completions_via_responses.go` 在显式 object 模式下不会走原始 pass-through body。
4. 未启用强制格式的 OpenAI 默认渠道保持 string，避免回归。
5. 非法 object 参数返回 400 且不触发渠道重试。
6. `relay/channel/openai/compact_request_test.go`、`relay/helper/model_mapped_test.go` 和 `relay/responses_handler_test.go` 验证 compact mapping、pass-through model rewrite 与正常请求互不干扰。

当前已通过的聚焦命令：

```powershell
go test ./service ./service/openaicompat ./relay ./relay/helper ./relay/channel/openai -count=1
```

## 运维建议

- 对明确返回“expected an object, got a string”的上游，将该渠道设置为 `object`，不要全局改默认值。
- 对仍要求字符串的 OpenAI-compatible 渠道保持 `auto` 或显式 `string`。
- 如果上游能力不确定，先用重复的真实 streaming 请求验证，不以单个 HTTP 200 或静态 schema 推断能力。
- 记录规范化计数和本地 400 计数，但不得记录完整工具参数或敏感 token。
