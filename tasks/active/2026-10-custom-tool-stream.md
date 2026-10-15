# 2026-10-15 自定义工具流被误判为空

状态：已定位并修复本地源码，尚未发布或部署。

生产：new-api-v1-4-1 / renewapi:1.4.1，发布源码 08b2b35。

## 复现证据

对渠道 267 team、模型 gpt-6.1-sol，以相同请求分别直接访问其上游和通过生产 renewapi：

- 普通文本：两者均正常返回 OK；renewapi 成功日志 179218。
- 自定义工具 emit（强制 tool_choice custom）：直连完整返回 custom_tool_call、两条 response.custom_tool_call_input.delta（hello 和换行）、response.completed。
- 同一自定义工具请求通过 renewapi：约 3259ms 返回容量错误；管理员日志 179219 的真实原因是 upstream stream ended without semantic output，HTTP 502，已观察到 354 input / 12 output tokens。

原始脱敏证据：D:/Code/renewapi/audit/20261015-stream-diagnosis/{probe.json,probe_direct.json,probe_logs.json,inspect.json}。
初次 curl 使用 socks5 本地 DNS 时 TLS 失败；改为 socks5h 后直连成功。该初次探测不作为服务缺陷证据。

## 根因与修复

relay/common/stream_semantics.go 的 ClassifyStreamPayload 缺少 response.custom_tool_call_input.delta，streamOutputItem 缺少 custom_tool_call。
首语义缓存因此始终不提交这些有效工具输出；终态 output 内的自定义工具同样识别失败。relay/helper/stream_staging.go 的 validate 在响应结束时生成空流 502，并触发重试，导致换渠道后可能重复失败。

本地补齐以上两项识别，保留原始 SSE 转发。增加起始工具项、输入 delta、仅终态携带工具项三条缓存回归。
验证：go test ./relay/common ./relay/helper ./relay/channel/openai 全部通过。

近期历史日志可见相同模型在 267、264、141 的多渠道空流失败，与此缺陷相符；未捕获历史请求完整响应，不能断言每条历史错误均为此原因。219 另有实际 429 并发限制，141 另有 first_semantic_timeout，应分开判断。

边界：本次修复为原生 Responses SSE 自定义工具识别，不代表 Chat/Claude 协议转换或 Anti-Poison 对自定义工具语义已完整支持。没有更改线上配置、渠道状态或部署。

## 后续代码审计（同日）

新增确认并修复：

1. Responses → Chat 的流式和非流式转换把文本与工具调用互斥处理；流式 sendToolCallDelta 在已有文本时直接跳过，非流式仅在无文本时读取工具。现保留两者，正确输出 tool_calls 终止原因。聚合回退也取消有文本时提前返回。
2. 聚合 output_item 按 call_id 入表，参数 delta 却按 item_id 入表；例如 fc_1 / call_1 被拆成两个记录，真实工具缺参数。现建立 item_id → call_id 关联，支持 arguments.done 补齐。
3. ResponsesOutput 未定义 custom input；正常终态有工具输入，聚合转 JSON 却丢失。增加指针字段保留合法空字符串，并补齐 custom delta/done 与终态 output 缺省时的工具回退。
4. 原生 SSE 的 computer_call/local_shell_call/shell_call/apply_patch_call 即使携带可执行动作，也不被首语义识别。现按明确动作字段识别；text/refusal/arguments/custom input 的有内容 done 事件同样纳入。心跳、进度、空消息、空 output + usage 仍不算内容。

验证过程先添加失败回归，证实混合输出工具数为 0、自定义 input 消失，以及 ID 错配；修复后通过。

- `go test ./relay/... ./service/... ./dto`：全部通过，包含渠道适配器、协议桥、Anti-Poison 和请求保护。
- `go test ./relay/channel/openai -run 'TestAggregateResponses|TestOaiResponsesToChatStream' -count=1`：新增聚合/完成事件回退测试通过。
- `go build ./...`：通过。
- `git diff --check`：通过。

审计范围：共享流语义、首输出缓存、原生 Responses SSE 与 SSE 聚合、Responses → Chat 转换、文本桥能力校验。新增动作类型为协议样本回归验证，没有逐一向生产供应商发送真实电脑/Shell/补丁调用。没有声称所有第三方扩展或所有跨协议工具均已支持。长期规则见 ADR-018。原有认证页面和 Changelog 修改保留。

## 端点转换与渠道切换复查

- Responses → Messages：原先能力校验支持 namespace，但实际转换没有展平，导致工具列表为空。现共用 PrepareResponsesRequestForTextBridge，保存名称逆向映射；input 内 additional_tools 也先提升再验证。四组控制器回归覆盖普通/提升声明 × 流式/非流式，检查上游展平声明及客户端原名称恢复。
- 跨渠道切换：原先只重置部分统计，失败尝试的防投毒 nonce、工具限制、配置缓存、检测结果和 Responses 工具映射可能留给下一渠道。现每次尝试前清理，并在选渠道前恢复模式、路径、最终格式与转换链基线。双渠道控制循环回归先制造失败和污染，再检查下一渠道状态并实际完成 Responses 请求。
- 保留原有首语义提交、客户端取消、固定渠道、已尝试渠道排除、重试预算及计费边界；相关控制器矩阵一并验证。此次没有改变生产渠道配置或部署。

最终验证：`go test ./controller ./relay/... ./service/... ./dto` 全部通过；
`go build ./...` 与 `git diff --check` 通过。测试使用本地模拟上游，生产真实端点未部署本次修复。

## 全流程 API 与 GitHub 方案复查

新增复现和修复：

1. Responses → Chat 流忽略仅终态携带的工具调用/参数后缀。已按 call_id 共用 item、arguments.done 和终态快照合并，避免重复发送，并拒绝冲突参数。
2. Responses → Chat 丢失 prompt_cache_options，现按扩展字段保留，包含 false/0 并支持往返。Messages 没有等价项的缓存/惩罚参数，以及尚未完整转换输出的跨协议 logprobs，提前明确拒绝；原生协议仍可转发。
3. 合法 SSE 多行 JSON 被逐行拆坏。流式转发、普通 Responses 聚合和 compact 聚合共用 helper.SSEScanner，支持 event 类型回退、CRLF、注释和 EOF 回退。保留单行无空行上游的即时处理，整帧累计大小受现有缓冲上限约束。
4. Chat → Messages 非流式仍以 finish_reason 决定仅保留工具或文本，导致混合内容丢失，length 时工具也消失。现按实际内容分别保留，终止原因独立映射；已先复现两个失败回归再修复。

既有三端点矩阵直接升级为 TokenAuth、TokenRequestLimit、ModelRequestRateLimit、解压/BodyStorageCleanup、Distribute 与真实 Relay 链；使用本地 HTTP 模拟上游，覆盖 42 组配置、84 次成功双轮调用、42 次无 Token 拒绝及 42 次 Gzip 后续调用（属于前述 84 次）。保留用户请求计数断言；Codex 矩阵首轮只发创建/终态，覆盖三种客户端的终态工具恢复。真实鉴权 fixture 增加订阅表；Token 查询改用 GORM 条件映射，自动转义 key 列并避免依赖数据库初始化的引号全局变量。

GitHub 实现已实际读取并固定到 SHA，链接：

- [OpenAI Go SSE](https://github.com/openai/openai-go/blob/036df078239cbc51259a7f4e4015e340d4790fb3/packages/ssestream/ssestream.go)
- [OpenAI Python SSE](https://github.com/openai/openai-python/blob/e71d781ac5e7b3f365ac130bf338baaf13ae720d/src/openai/_streaming.py)
- [OpenAI Python Responses](https://github.com/openai/openai-python/blob/e71d781ac5e7b3f365ac130bf338baaf13ae720d/src/openai/lib/streaming/responses/_responses.py)
- [New API scanner](https://github.com/QuantumNous/new-api/blob/718a4e7f9751c5bf1958747cd0c2c895e2b937d4/relay/helper/stream_scanner.go)

取舍见 UPSTREAM_PORTS.md，本地只适配行为，不引入新 SDK 或整套框架。参考副本和测试日志保存于 D:/Code/renewapi/audit/20261015-api-flow-references。GitHub API 限流后使用 git ls-remote + 固定 SHA raw 文件读取；Sub2API 猜测路径返回 404，没有把它当作已审阅方案。

验证范围不等同于全部 40+ 厂商真实上游调用，也不代表所有第三方工具、扩展字段和 logprobs 已实现跨协议转换。前端用户改动保持不变；未提交、未发布或部署。

本轮最终验证：`go test ./...` 全仓通过；`go build ./...`、`git diff --check` 通过。
先前一次全仓运行与编辑重叠造成编译读取不一致，已在源码冻结后完整重跑通过；最终日志为 audit/20261015-api-flow-references/go-test-all.log。
