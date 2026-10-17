# Upstream Port Ledger

## 2026-10-15 API 调用与 SSE 专项参考

| GitHub 来源 | 处置 | 本地结果 |
| --- | --- | --- |
| OpenAI Go SDK `036df078239cbc51259a7f4e4015e340d4790fb3`，`packages/ssestream/ssestream.go` | ADAPTED | 参考事件边界合并多行 data 的行为；本地统一至 helper.SSEScanner，保留既有无空行兼容、取消/首语义期限和缓冲上限，不引入 SDK。 |
| OpenAI Python SDK `e71d781ac5e7b3f365ac130bf338baaf13ae720d`，`_streaming.py` 与 `lib/streaming/responses/_responses.py` | ADAPTED | 对照多行解码与完成快照/工具身份处理；本地工具快照共用后缀合并逻辑并拒绝冲突，不移植其对象模型或增加运行时。 |
| New API `718a4e7f9751c5bf1958747cd0c2c895e2b937d4`，`relay/helper/stream_scanner.go` | NOT_APPLICABLE | 仍逐行处理，不能解决本次多行 JSON 复现；保留本地可靠性和生命周期机制。 |

本节仅登记指定文件的行为参考，不推进下方全量审计基线。当前为未提交本地改动；验证见工具流任务。

Audited-Upstream-Ref: 58d4e9bd3bb035df8ea235dd682ccc8a45d0332a

Upstream release at audit time: `v1.0.0-rc.24` plus post-release `main`

Audit date: `2026-08-14`

Fork review base: `origin/main` at `91d636fba978`

The fork and `QuantumNous/new-api` have no common Git ancestor. "Audited" means every upstream commit through the ref above was reviewed; it does not mean every feature was copied. Updates are ported as behavior-focused local commits so fork security, compatibility, and branding remain intact.

Historical entries use the status vocabulary that was recorded at the time
(`ALREADY_COVERED`, `ADAPT`, `DEFER`, and `REJECT`). New entries should use the
normalized dispositions `PORTED`, `ADAPTED`, `ALREADY_PRESENT`,
`NOT_APPLICABLE`, `SKIPPED`, `DEFERRED`, or `REJECTED`.

## Imported In This Audit

| Upstream commit(s) | Local commit | Result |
| --- | --- | --- |
| `70ea899e`, `4e570389` | `87364cb7` | Replaced ineffective GORM v1 locking hints with GORM v2 row locks and CAS redemption settlement. |
| `d0bd8aac`, `c9943d37`, `48b7f491`, `bae799cc`, `043720f9` | `1e47cca8` | Added saturating quota conversions, bounded quantities, finite ratio checks, and unified task settlement. |
| `5fc35e28`, `0d5995eb`, `4a64b870` | `77ce8d49` | Hardened email uniqueness, OAuth/password transitions, and disabled-token behavior. |
| `d2f7f9ee` | `6ba6cfc9` | Added bounded anonymous request bodies, including chunked requests and callback routes. |
| `df087b02` | `f4a848b8` | Added dial-time SSRF validation and DNS-rebinding protection to user-controlled fetches. |
| `153d7f01`, `986d90ae` | `7dc1d3bc` | Joined stream workers, stopped stale writes, and added graceful process shutdown and cache drains. |
| `d2576ddc`, `59a93cf5` | `49690297` | Added OpenAI Images JSON/SSE streaming, multipart replay, error parsing, and usage normalization. |
| `0977965d`, `867d8acf` | `444aba1b` | Added Ollama tool calls and Kimi K2.6 temperature normalization. |
| `90fa6fe6`, `394b023d`, `28e0115a` | `63d0d706` | Fixed wallet quota units, decimal ratio editing, and browser translation mutation of React roots. |
| `230a3592`, `afb470e4` | `be35adb2` | Corrected log ordering and rebuilt the composite index on existing SQLite/MySQL/PostgreSQL databases. |

## 2026-08-14 Review: rc.21 -> rc.24 -> main

Reviewed upstream through QuantumNous/new-api `main` at `58d4e9bd3bb035df8ea235dd682ccc8a45d0332a` (latest commit dated 2026-08-13). The release boundaries were rc.20 `6ce7305c`, rc.21 `bde9b2f4`, rc.22 `bc14c18f`, rc.23 `0ab02020`, and rc.24 `5c3abffe`.

| Upstream ref | Classification | Local result |
| --- | --- | --- |
| `58d4e9b`, `ccd535e`, `50e5377`, `df43f80`, `cfaba1d` | `ALREADY_COVERED` | Existing BillingSession, transport, thinking-budget, and compatibility behavior already cover these invariants; no duplicate port was added. |
| `e926e5c` | `ADAPTED` | Preserved the exact stored redemption quota when an edit leaves the amount unchanged in both default and classic frontends, while retaining existing create and explicit-edit conversion behavior. |
| `d799267` | `ADAPTED` | Built-in OAuth and WeChat binding now update only a whitelisted binding column, preserving concurrent quota, status, group, and profile changes; custom OAuth bindings remain in `user_oauth_bindings`. |
| `1da23d6` | `ADAPTED` | Added isolated per-user critical throttles for token regeneration and affiliate quota transfer using RenewAPI's existing limiter abstraction. |
| `d6b5ce9`, `253a74d`, `2399de9`, `3d5dc36`, `d49160f`, `bd585d7` | `ADAPT` | Ported the relevant HTTP replay, Responses penalty, backend length validation, Gemini model listing, Ali `top_p`, cancellation, and bounded cooldown behavior in renewapi-native code and tests. |
| CPA `Retry-After` edge cases | `ADAPT` | Retained the existing router reliability boundary and added only a bounded cooldown hint. |
| DeepSeek Responses, broad UI/refactor/relaykit changes, and unrelated dependency churn | `DEFER` / `REJECT` | Not required by this audit or incompatible with the fork's current scope. |

The reviewed range contains 133 commits from the previous audit ref `4e570389dd433a717373ce9c9b822b59f5ed3d5d` to the current upstream main. Because the fork and upstream have unrelated histories, this ledger records behavior-focused ports rather than a merge or cherry-pick.

## Preserved Fork Behavior

- Public `PriceData.OtherRatios` remains available; upstream pricing refactors that remove or reshape it were not copied wholesale.
- Compatibility bridges, anti-poison profiles, channel TLS controls, and custom Interface Zero metadata remain fork-owned.
- Stream changes use the fork's buffer pool, first-byte timeout, write deadline, and `StreamStatus` lifecycle.
- Account updates retain explicit-column writes so concurrent quota changes are not overwritten.

## Deferred Or Rejected

| Area | Decision |
| --- | --- |
| Resizable admin tables and broad default-frontend redesign | Deferred as product/UI work; not required for protocol, security, or correctness compatibility. |
| Subscription quota reset and stale-instance administration | Deferred feature additions; require a separate permission and operations review. |
| GPT-5.6 pricing/model catalog updates | Deferred to the fork's price-sync path instead of copying static catalog churn. |
| Dependency-only and Electron updates | Deferred until the normal dependency audit; unrelated to the server update batch. |
| Upstream dead-file cleanup and launch-history repairs | Rejected as lineage-specific and not applicable to this repository. |

## 2026-09-23 可靠性专项复审

本轮按当前实现逐项核对可靠性和 Responses 协议，不声称已分类两个上游的所有无关功能提交。因此顶部完整历史审计基线保持原值；专项引用如下。

- New API：`QuantumNous/new-api main@996adffe5165bd5e311e33a03a86b8aede1fe376`。
- Sub2API：`Wei-Shaw/sub2api main@20a94fbb567b62208751292ed7786b24a7e7c0fe`。
- 官方协议：2026-09-23 读取 [Responses WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode) 和 [WebSocket events](https://developers.openai.com/api/reference/resources/responses/websocket-events)。

| 来源 / 对应提交 | 判定 | 原因与 RenewAPI 实现 |
| --- | --- | --- |
| New API `55a6cd2`，terminal usage 缺失 | IMPLEMENT | `ada0a55bb`、`a2321c2b1`：补齐文本、工具参数、推理、拒绝、仅 terminal 输出和 null 用量估算；terminal 与 delta 分别估算取较大值，保留原生缓存/推理用量和显式零。失败尝试仅记录诊断，收费由提交门与账本决定。 |
| New API `55a6cd2`，任意前导事件即计输入 | REJECT | `response.created` 不能证明用户收到有效内容；违反本轮提交前失败不收费的要求。 |
| New API `610334d`，跨协议 stream usage | IMPLEMENT | `ada0a55bb`：Claude 已覆盖；补 Gemini → Chat，以及 Responses → Chat 在关闭全局强制 usage 时的请求。仅对已有支持 stream options 的渠道请求 usage。 |
| New API `6237d9d`，限流槽位最终结果 | NOOP | `ModelRouteOutcome`、`RelaySemanticSuccess` 和内存/Redis reservation 已按最终语义结果提交或归还。已有集成测试继续通过。 |
| New API `8b2c710`，StreamStatus 协议分类 | NOOP | RenewAPI 已有传输/语义结果分离、取消、空流、畸形、terminal、提交状态与重试边界；本轮 `99703c6ea` 补齐语义提交门。保留有效 max_output_tokens incomplete 的原有兼容行为。 |
| New API 当前错误输出与内部标识 | IMPLEMENT | `8b8c9e9a7`、`a2321c2b1` 提供单一公共 capacity 错误及管理员链；普通日志、流 terminal 和错误扩展字段均脱敏，HTTP/WS 错误保留管理员所需真实状态，不复制上游插件标识。 |
| New API 当前 Combobox / autofocus | IMPLEMENT | `3674bfc78` 已在当前双主题前端最小修复，无需复制已重构的上游前端目录。 |
| New API `ed7c4e3`、`972aed1`，返回模型记录 | IMPLEMENT | `ada0a55bb`：`relay/common/response_model.go` 在转换前收集模型声明，管理员日志展示请求/映射/返回名称；不保存容易过期的 mismatch 标志，不用返回值覆盖路由或计费模型。 |
| Options 主键与 pricing 写入 | IMPLEMENT / NOOP | `ea9a4f994` 复现旧表缺少主键并补迁移；定价更新不重置其他配置已验证，NOOP。四个外部数据库版本实际通过。 |
| Sub2API `51f73840`，工具参数 done | IMPLEMENT | `ada0a55bb`：补 `response.function_call_arguments.done` 的缺失参数及前缀去重；不把完整参数重复追加到已发送 delta。 |
| Sub2API `13be6ca2`、`e26abaef`，keepalive 与 terminal EOF | NOOP | `99703c6ea` 已将 keepalive / response.created 排除在语义之外，真实 15 秒 stall、terminal 缺失及提交后不拼流回归通过。 |
| New API `75f3d24`、`ae249f4`；Sub2API 当前 WS reader / execution scope / pool | IMPLEMENT | `7c2cf87cd`、`a2321c2b1`：新增独立 Responses WS Relay、逐轮执行与计费、lane FIFO/并行、错误关联、容量限制、常驻 reader、有限历史和安全增量续写。复用现有 HTTP/SSE Relay，原生上游按渠道开启，不机械引入全局跨用户连接池或整套 relaykit。 |
| 官方 WS error envelope、New API 当前错误关联 | IMPLEMENT | `a2321c2b1`：官方错误按 stream 关联，不要求根 event_id 等于请求 ID。明确 error.event_id、response_id 和已知旧请求用于隔离，服务端自建事件 ID 不再导致错误等待到 timeout。真实 WS 保持连接打开的故障测试及旧错误隔离测试通过。 |

本轮代码与行为验收见 `tasks/archive/2026-09-reliability-upgrade.md`。旧的完整历史审计范围不因本专项复审自动扩大。

## 后续完整审计

### 2026-10 三端点整流专项

| 来源 | 判定 | 实际适配与边界 |
|---|---|---|
| New API `6370b29424168039e94d40d610191e7d2e65dbf4` 的 `relaykit/relayconvert/internal/claude_messages/to_oai_responses_req.go` 与 `oai_responses/to_claude_messages_req.go` | ADAPT | 直接请求转换适配至 `service/openaicompat`，保留内容顺序、工具 ID/结果、显式零值、图片、并行和思考意图；继续使用本项目 DTO、common JSON 边界和账务/保护层。AGPL-3.0 来源头保留，不引入整个 relaykit。 |
| Sub2API `5fc0e486c3f6a8a191b8bd140f39b60457f611cf` | REFERENCE | 对照工具历史、system 和媒体处理；不采用其模型重映射、默认预算抬高或订阅分组规则。 |
| CLIProxyAPI `0f96f568e4dbf6f84ad7399a74b78344c5eac7e6` | ADAPT / REFERENCE | 参考并行工具与事件生命周期测试，修复首块多工具、禁止并行丢失及跨尝试输出状态；预算到 effort 使用本项目明确的级别映射，不复制身份伪装或跨供应商签名规则。 |
| Bifrost `dfa57061c46793d1229885dcf7c54ef1e74b754b` | REFERENCE | 对照 Anthropic Responses 转换和供应商能力例外；保留本项目适配器与能力筛选，不引入其 SDK。 |
| LiteLLM `d8c0e2c7153d82234ec46f0231bf9b9714e7d52a` | REFERENCE | 对照 Messages/Responses 参数和 reasoning 映射；不增加 Python 运行时或复制自动处理外部签名的默认策略。 |

本轮仅是指定协议范围的源码专项审阅，没有更新顶部全量上游审计基线。实施决策见 ADR-016，验收和发布记录见三端点整流任务。

### 2026-10-17 NewAPI / Sub2API / CPA 更新检查（候选，未导入）

固定参考：NewAPI `3e2ce2c8856e32ca7054d673f8cd059ae466e80c`；Sub2API `b0a02a3304a0a78cdd003ddda1dd23ba02c03084`；CPA `911f49cbd49f4fe2516979754a7488a3fd14fe64`。本轮列举近期 291 条非合并提交、保存 49 个候选 patch，定向核对下列行为，没有扩大顶部完整历史审计基线。详细定位、复现、优先级和适配验收见 [更新检查报告](docs/audits/2026-10-upstream-update-review.md)。本表的 DEFERRED 表示已经识别但尚未实现；不是修复完成或已发布。

| 来源 / 行为 | 判定 | 本地处置 |
| --- | --- | --- |
| NewAPI `06dd76a`，Playground 按账号隔离 | DEFERRED | U01；合成 A→退出→B 复现仍读到 A 消息；需统一存储命名空间及账号变更后的流/状态清理。 |
| NewAPI `1b63c28`，Claude 零值和缓存-only | DEFERRED | U02；复现普通 input 被补入 1200；需适配现有缓存语义和持久账本，不能整段覆盖计费。 |
| NewAPI `45fbb14`，分组独立限流 | DEFERRED | U03；内存及 miniredis 均串桶；仅修 key 作用域，保留 reservation/语义归还。 |
| CPA `35af74f`，旧成功清新冷却 | DEFERRED | U04；模型层复现冷却被清零；用跨实例可用的状态版本/CAS，保留合法探活。 |
| NewAPI `5ac7562`，混合 reasoning+正文 | DEFERRED | U05；首块和后续块都复现正文丢失，接现有 Claude 内容块生命周期。 |
| NewAPI `ff36846`、`13c8e90`，图片双向转换 | DEFERRED | U06；detail 丢失和反向 image_url 字符串均复现；能力化处理新版 detail，不改 MJ 公开分享契约。 |
| CPA `59930ba`，工具 incomplete→is_error | DEFERRED | U07；可选状态被忽略；仅传播显式失败信号，复用反向包装。 |
| NewAPI `767a82b`，多行 beta | DEFERRED | U08；Header.Get 只转发第一行；适配 Header.Values 及已有覆盖规则。 |
| NewAPI `767a82b` 的原生 thinking 扩展 | ALREADY_PRESENT | RequestExtensions 已保留 block_binding/未知字段；原生序列化复现通过，不增重复 DTO。 |
| NewAPI `09109ec`，容器资源哨兵 | DEFERRED | U09；静态确认宿主机口径、无连续样本/新鲜度；需真实容器验收，不在请求中增加采样。 |
| NewAPI `ca2cae6`、Sub2API `bdfe215c`，Responses 完整性 | DEFERRED | U10；本地 reasoning/summary 缺口更大；SSE 已有时间及序号，仅补实际不足，保留透传边界。 |
| NewAPI `a0bf6a8`、`718a4e7`，Vertex 项目及多区域 | DEFERRED | U11；可选项目配置缺失、us host 错误；保留 Express/JSON 凭据模式。 |
| NewAPI `c10780e` / `4b13d93`，Gemini schema | DEFERRED | U12；优先后续能力化无损方案，不单独移植早期有损删除；const/oneOf 丢失已复现。 |
| NewAPI `b892a6b`，chunk 加载恢复 | DEFERRED | U13；适配当前错误页的一次恢复和真实 HTTP 状态，不新增第二层异常处理。 |
| NewAPI `3fe394c`、Sub2API `c61f6eba`，依赖安全升级 | DEFERRED | 本地 Go1.25.1/Linux 符号扫描有 43 个去重可达告警；Go、websocket、x/net 及配套 x/* 按一致版本集合升级并重新验收。 |
| NewAPI `ba0e478`、CPA `7795de1`、TLS 握手限时 | ALREADY_PRESENT | 本地已保留 Claude→Chat 请求文本/工具，复用 SSEScanner 和有界 TLSHandshakeTimeout；相关既有回归通过，不能据此认为 U05 已覆盖。 |
| Sub2API `d19265dd` composite OAuth 凭据错投 | NOT_APPLICABLE | 补丁依赖其平台账号池/composite 入口；本地按 channel/adaptor 选上游，不引入该架构。 |
| 整套前端/插件/账号池、自动 binding 注入、静态价格替换 | SKIPPED | 本轮不导入其产品架构与默认策略，保留本地安全、路由、价格同步和品牌边界。 |

### 2026-10-17 后端功能选型（未实施，排除 UI）

沿用上节固定三仓 HEAD，额外保存 24 个功能 patch，定向核对入选路径；不扩大完整历史审计基线。此处 DEFERRED 仅表示已提出适配候选，未导入、验收或发布。完整价值、现有覆盖、最小实施与验证要求见[功能评估](docs/audits/2026-10-backend-feature-opportunities.md)，与上节缺陷更新检查分开维护。

| 来源 / 功能 | 判定 | RenewAPI 最小增量 |
| --- | --- | --- |
| CPA `58c0f6a` / `aeec5e9` / `03a3cfe` / `77fd91e`，客户端工具搜索 | DEFERRED | F01；接现有协议桥接与能力筛选，Claude 原生 deferred/reference 与 Gemini 函数桥接分别声明；不引入工具执行服务。 |
| Sub2API `6624947b` / `ae2735cd` / `901a0439` / `5a25507d`，额度采集与窗口调度 | DEFERRED | F02；从已有 Codex 查询扩展为凭据级快照和候选过滤，采集去重；不重复建设冷却和粘连系统。 |
| NewAPI `04c597b`，regex 条件 | DEFERRED | F03；现有 regex_replace 不等于条件匹配，仅扩现有条件比较。 |
| Sub2API `6f9b02f1`，Vertex 发布方模型发现 | DEFERRED | F04；复用 service account、模型获取和更新任务；列表不等于推理授权。 |
| Sub2API `d5a802f8`，Gemini cachedContents | DEFERRED | F05；新增资源归属/凭据绑定/TTL/存储费生命周期，不重复现有缓存引用 DTO。 |
| NewAPI `63cecab` / `7de10f2`，聚合上游余额 | DEFERRED | F06；余额查询策略支持 Sub2API/NewAPI，有限/无限/未知分开；owner quota 另行逐 key 授权，不自动扩权。 |
| NewAPI `d032204`，AWS Mantle | DEFERRED | F07；AWS adaptor 加显式 Messages 模式，复用 Claude 响应与现有 HTTP 客户端。 |
| CPA `7fcbdf8` / `16d9888`，搜索事件与引用 | DEFERRED | F08；优先 Gemini→Responses，复用流/工具用量通路，补迟到来源及字符索引。 |
| 本地 Codex OAuth、会话粘连、Responses WS/compaction、模型 metadata | ALREADY_PRESENT | 按实际增量实施上述候选，不把已有能力重新列为升级。 |
| CPA `2bc236d`，Claude 自有压缩 capsule | DEFERRED | 不采用默认丢弃外部 capsule；自有摘要格式、密钥、多实例及计费契约需独立设计，首批不做。 |
| CPA `2d5351b`，Vertex Interactions | DEFERRED | 条件候选，已核对 native executor 入口/流/鉴权；资源归属和状态续接另做专项设计，不视为已移植。 |

### 2026-10-17 后端性能适配（未发布）

沿用固定三仓 HEAD，前轮保存 15 个性能候选 patch，本轮适配以下五项并检查 7 个额外候选，新增 HTTP 响应生命周期优化。详见[实施与验证报告](docs/audits/2026-10-performance-upgrade.md)、[ADR-021](docs/decisions/021-relay-performance-and-stream-lifecycle.md)；原[性能评估](docs/audits/2026-10-upstream-performance-opportunities.md)作为选型时证据保留。不推进完整历史审计基线。

| 来源 / 性能方向 | 判定 | 本地结论 |
| --- | --- | --- |
| NewAPI `2cf177a`，RawMessage 批量深拷贝 | ADAPT | P1；`common/copy.go` 独立字节复制；嵌套/指针/零值/隔离回归通过，正式三轮 1 MiB 复制 59.686ms→0.1164ms。 |
| Sub2API `7e627318` / `61e1afdd` / `379db191`，大 JSON 按改动重建 | ADAPT | P2；reasoning 局部拼接及桥接无操作快返，复杂/异常输入保留原转换；等价 fuzz 162,428 次通过。 |
| NewAPI `d3874db`，限流按需分配与 LRU 清理 | ADAPT | P3；桶内数组按需增长/复用，桶级 LRU；保留 Reserve/Cancel/RequestN/RetryAfter，边界/并发/race 与冷/热 key 基准通过。 |
| NewAPI `946c730` / CPA `8a6a396`，工具索引 | ADAPT | P4；Claude→Chat 请求内首次命中索引，显式名称/空 ID/乱序回归；1,000 轮局部转换 6.031s→12.119ms。 |
| Sub2API `81fd8530`，H2 PING | ADAPT | P5；仅 HTTP 长流 profile，每 shard 独立配置，30s/15s 可关闭；真实 H2 静默/无 ACK/代理/HTTP1/缓存回归通过。 |
| Sub2API `3fdd54ca`，取消后关闭响应 | ADAPT | P6；仅 HTTP attempt，先取消再安全关闭；保留父请求及 WS turn 的既有关闭/复用顺序，并发/race/真实 HTTP/WS 回归通过。 |
| Sub2API `277aa141` / `d0ed0eac`，终态即时结束/Gemini 传输失败切换 | ALREADY_PRESENT | 本地 `sr.Done/Stop` 与统一 DoRequestFailed/failover 已覆盖对应行为；不重复增加终态/重试机制。 |
| Sub2API `9be9c0b6` / `e652f6e2`、CPA `1dffddf` | NOT_APPLICABLE / REFERENCE | 本地无相同账号 session 槽/额度监控器；身份注入已用解码 map；具体差异与已有释放逻辑见实施报告。 |
| NewAPI `219c9e0`，公共内容 ETag/匿名 session | DEFERRED | 包含面板客户端配套，本轮排除 UI，不作为推理性能适配。 |
| Sub2API `a9514a68` / `18d483c2` / `ca9b4d73`，统计/历史共享 | ALREADY_PRESENT / REFERENCE | 本地已具备合并概览聚合与 WS 正文共享，非相同架构的完整等价实现；剩余序列化开销归 P2，不套用上游 O(T²) 结论。 |

### 2026-10-17 性能适配后的模块协作复审

本地复审补修客户端重建竞态、长窗口预算清理，以及 Redis/内存并发槽释放错配和长请求租约过期，另消除 MiniMax 测试竞态。这些为 RenewAPI 既有调用链缺口，没有新增上游来源或推进完整审计基线；其余 DEFERRED 功能/依赖项仍未导入。源码、测试、性能成本及发布前排空旧并发槽协议的要求见[复审报告](docs/audits/2026-10-integration-review.md)、ADR-021。

Run `scripts/check-upstream.ps1` or `scripts/check-upstream.sh`. The scripts read `Audited-Upstream-Ref`, list only later upstream commits, and refuse merge/rebase while histories remain unrelated. After review, add each imported or rejected item here and advance the audited ref only when the complete range has been classified.
