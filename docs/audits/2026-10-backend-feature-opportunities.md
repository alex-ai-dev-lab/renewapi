# NewAPI / Sub2API / CPA 后端功能引入评估

日期：2026-10-17。评估对象为 RenewAPI `main@32f0d2bf9a4e41b1564dce167d5a434c9129eaa3` 加当前未提交修复。

本轮回答“其他项目有哪些好用的后端功能可引入”，排除前端 UI。上一份[更新检查](2026-10-upstream-update-review.md)主要讨论缺陷与依赖，保留为独立材料，不代替功能选型。本轮没有导入业务代码、提交、推送或部署。

## 推荐结论

以下成本是相对实施量，包含适配与回归，不是工期承诺；收益尚未在 RenewAPI 生产流量上量测。

| 编号 | 功能与参考源 | 直接用途 | RenewAPI 的实际增量 | 成本 / 建议 |
| --- | --- | --- | --- | --- |
| F01 | 工具搜索与延迟加载，CPA | Codex / Responses 客户端通过 Claude、Gemini 使用较大的工具集合，按需发现工具 | 目前桥接支持 function / namespace，尚无 tool_search 生命周期与 deferred 引用转换 | 中高；首批核心能力，先 Claude 再 Gemini |
| F02 | 上游额度窗口感知调度，Sub2API | 在 5 小时、周额度耗尽时避开对应凭据，窗口恢复后重新纳入 | 已有 Codex 用量查询、渠道/模型冷却与会话粘连；缺用量快照驱动的主动路由 | 中高；首批核心能力，先复用 Codex 数据源 |
| F03 | 参数覆盖的正则条件，NewAPI | 按模型名或字段模式应用一组参数规则，减少重复规则 | 已有 regex_replace 操作，但条件比较没有 regex 模式 | 低；可先独立落地 |
| F04 | Vertex Gemini 专用模型发现，Sub2API | 获取区域内的 Google 发布方 Gemini 模型，减少手填列表 | 已有 Gemini 模型获取和公共 metadata 同步；缺 Vertex service account 专用分页获取 | 中；使用 Vertex 时优先 |
| F05 | Gemini 显式上下文缓存管理，Sub2API | 反复问同一文档/长上下文时，创建缓存并在后续生成中复用 | 已有 cachedContent 字段及命中 token 用量，缺资源创建、TTL、归属、上游绑定和存储计费 | 高；Gemini 重度使用时进入第二批 |
| F06 | NewAPI / Sub2API 上游余额查询，NewAPI | 聚合其他中转站时自动获取实际可用额度，用于运维与后续路由 | 现有余额更新器缺两种查询协议；无需新增两套推理渠道 | 低中；有对应上游时先落地 |
| F07 | AWS Mantle Messages 接入，NewAPI | 通过 Mantle 调用 Claude 原生 Messages / SSE | 已有 AWS 适配器，缺 Mantle 模式及其鉴权/请求格式 | 中；有 Mantle 渠道再做 |
| F08 | Gemini 搜索到 Responses 的事件与引用转换，CPA | Responses 客户端使用 Gemini 原生联网搜索，返回搜索过程、来源与引用 | 已有原生搜索参数，跨协议桥接缺内置搜索工具与 grounding/citation 语义 | 中高；有联网搜索需求再做 |

建议先完成 F03，以及实际在用的 F04/F06 小功能；随后以 F01、F02 为主要能力升级。F05 涉及独立资源生命周期和账务，应单独实施。F07/F08 按渠道和调用需求选择，避免为了凑齐功能增加维护成本。

## 逐项适配方案

### F01：工具搜索与延迟加载

源码参考：CPA [首次桥接](https://github.com/router-for-me/CLIProxyAPI/commit/58c0f6a2b2471fecce9f7b7782060f16803e3698)、[原生 deferred / tool_reference](https://github.com/router-for-me/CLIProxyAPI/commit/aeec5e9761a0e4bf224b3c15b4a88b8296c2793a)、[执行器和模型目录能力一致性](https://github.com/router-for-me/CLIProxyAPI/commit/03a3cfecb540f4d4a195f9c3574dd14cb6cc1ff0)、[Gemini 客户端工具搜索](https://github.com/router-for-me/CLIProxyAPI/commit/77fd91e8f463fe6f61fa37b3450aa9e314a2bee1)。核心为 `internal/translator/{claude,gemini}/openai/responses/*tool_search.go`、Claude request/response 转换及 executor 的 `SupportsNativeToolSearch`。

已读实现把客户端执行的 `tool_search_call/output` 映射到供应商的工具调用/结果，后续发现的工具继续可用。Claude 后续实现使用 `defer_loading` 与 `tool_reference`，比仅把搜索结果改成说明文本更适合保持原生工具语义及缓存前缀。Gemini 实现是客户端搜索桥接，不能称为 Gemini 原生 deferred 协议。

本地定位：[工具桥接](../../service/openaicompat/responses_bridge_tools.go)、[扩展参数回归](../../service/openaicompat/request_extensions_test.go)、`relay/responsebridge/`。生产代码未发现 tool_search / tool_reference / defer_loading 的桥接实现；原生透传不等于跨协议支持。

最小实施：在现有工具转换中加入客户端搜索调用、结果和已发现定义的统一表示；先打通 Responses→Claude→Responses，再接 Gemini。路由按上游模型、端点和显式支持配置筛选，模型目录对客户端声明的能力必须与实际执行路径一致。不另建工具执行服务器，不默认支持 hosted/server 搜索；不支持时沿用明确拒绝，不能静默删工具。

验收：多轮发现、命名空间重名、重复工具覆盖、搜索失败/不完整、流与非流、取消后终态、未知工具拒绝、保护层及计费仍生效。用相同工具集比较输入 token、缓存命中和首输出延迟，不能先承诺固定降幅。

### F02：额度窗口驱动调度

源码参考：Sub2API [OpenCode Go 用量窗口与自动刷新](https://github.com/Wei-Shaw/sub2api/commit/6624947b627b6f625e0e8d068cdf655125d135f0)、[同凭据共享与活动防抖](https://github.com/Wei-Shaw/sub2api/commit/ae2735cdb6861ef372b43e2ddb4aab5989b18fd9)、[模型/账号隔离范围](https://github.com/Wei-Shaw/sub2api/commit/5a25507dcbb33fc40e3ef0659af210f206229a31)、[Kimi/智谱额度快照和调度阈值](https://github.com/Wei-Shaw/sub2api/commit/901a0439f1575a45c29150e04d2ccc3ed87f4948)。

已核对 `opencode_go_usage.go` 的去重刷新、防抖与 max-wait；`cn_provider_quota_service.go` 的查询及快照持久化；`account_scheduling_threshold_eval.go` 在 used_percent 达阈值且 reset_at 尚未到达时停调。OpenCode 的采集/刷新与 Kimi/智谱的阈值调度是不同实现路径，不能把所有采集器都描述成已自动接入调度。

本地定位：[Codex 采集](../../service/codex_wham_usage.go)、[查询控制器](../../controller/codex_usage.go)、[模型状态](../../model/channel_model_status.go)、[会话粘连](../../service/channel_affinity.go)。`FetchCodexWhamUsage` 的业务调用在查询控制器和凭据导入预检，尚无将其结果接到调度选择的消费链。

最小实施：建立一份以凭据身份、供应商、必要时模型为作用域的额度快照，记录使用比例、窗口到期、采集时间和状态；在既有渠道/key 候选过滤中读取快照。先接已有 Codex 采集器，再按实际渠道增加 Kimi、智谱、OpenCode。相同凭据共享快照；后台有界刷新、活动防抖与请求合并，转发热路径不等待额度查询。

额度停调与错误冷却保留各自原因，在同一候选判定处组合；会话粘连不能强行选回已耗尽凭据。unknown、过期快照和查询失败不应写成余额 0；窗口到期解除该窗口约束，并安排受控刷新，不等同于宣称额度一定恢复。复用现有状态版本约束，防止旧查询覆盖新凭据状态。

验收：多窗口取约束交集、模型与凭据作用域、重置边界、刷新失败、凭据更换、多实例去重、全部耗尽时的明确响应；验证采集量不会随转发 QPS 线性增长。

### F03：正则条件覆盖

源码参考：NewAPI [条件 regex](https://github.com/QuantumNous/new-api/commit/04c597bf6cce1eea76b9e67a4936e15fb54a5612)，`relay/common/override.go`。

本地同名文件的 `compareGjsonValues` 只有 full/prefix/suffix/contains 和数值比较；已有 regex_replace 解决的是修改内容，不是判断是否执行规则。

最小实施：在现有 ConditionOperation 中增加 regex 条件，沿用 invert、缺字段及 AND/OR 语义；使用 Go regexp，对重复匹配复用已编译表达式。若配置对象跨请求共享，不能照搬上游请求内可变缓存而引入竞态，应采用不可变预编译配置或有界缓存。不新增规则引擎或前端编辑器。

验收：有效/无效表达式、非字符串 pattern、缺字段、反选和组合规则，已有覆盖行为不变；配置复用场景检查并发安全。

### F04：Vertex 模型发现

源码参考：Sub2API [Vertex 列表同步](https://github.com/Wei-Shaw/sub2api/commit/6f9b02f12c3d414ed324460a0e82d7793a471f72)，`upstream_models.go` 的 `fetchVertexGeminiUpstreamModels`、`buildVertexPublisherModelsURL`。

上游使用 service account access token，访问区域/global 的 `v1beta1/publishers/google/models`，分页、限页、去重并筛选 Gemini。这个列表是发布方目录，不保证项目拥有每个模型的推理权限；它也不是完整的 context/token/endpoint 能力目录。该实现不支持 Vertex API key 列表获取。

本地定位：[渠道模型获取](../../controller/channel.go)、[公共 metadata 同步](../../controller/model_sync.go)。已有 Gemini 特判与定时更新，通用获取路径不能替代 Vertex 专用请求。

最小实施：在现有模型获取服务增加 Vertex 分支，复用凭据刷新、代理及模型更新任务；发现结果先作为候选，保持管理员确认/现有启用规则，必要时另做轻量可用性探测。不新建第二套模型目录或直接覆盖定价。

验收：global/区域、分页截断、部分页失败、403、空结果、service account 刷新、API key 明确不支持；列表失败不能把原可用模型批量清空。

### F05：Gemini 显式缓存

源码参考：Sub2API [cachedContents 完整后端链路](https://github.com/Wei-Shaw/sub2api/commit/d5a802f84a6ae3405bee2a6e0160675785abf237)，`gemini_cached_contents_handler.go`、`gemini_cached_content.go`、repository 与存储费处理。

已核对创建入口、TTL 校验、按 API key + group 解析本地公共 ID、绑定持有上游账号、token-hour 计量和创建后计费失败的资源清理。它是供应商显式上下文缓存，不是网关复用完整回答，也不能替代现有 prompt cache 计费。

本地定位：[Gemini DTO](../../dto/gemini.go)、[Relay 路由](../../router/relay-router.go)。已有缓存引用和 cachedContentTokenCount，但没有 cachedContents 的 CRUD 路由。

最小实施：增加缓存资源表及服务，记录公共 ID、owner、渠道/凭据身份、上游资源名、模型、区域和到期时间。提供创建/查询/列表/延长/删除；生成请求必须解析并绑定原凭据，不能故障转移到拿不到缓存的账号。沿用现有鉴权、模型权限和账本，增加存储费计量及幂等操作标识，支持三库迁移。

创建、续期、提前删除和失败补偿的收费契约需在实施设计中明确；不能直接照搬上游一次性收费规则。只保存必要资源元数据，避免再次持久化整份用户文档。先支持明确可用的 Gemini/Vertex 凭据类型，其他上游显式启用。

验收：跨账号/跨 key/跨组访问、到期、凭据轮换、续期并发、上游成功而本地失败、清理重试、重复提交不重复扣费、进程重启与三库迁移。它需要完整验收后上线，不适合当成几个透传接口拼入。

### F06：聚合站余额协议

源码参考：NewAPI [Sub2API `/v1/usage`](https://github.com/QuantumNous/new-api/commit/63cecab824bad1a421d19950004c309bb2054c09)、[NewAPI `/api/usage/token/` 与 owner quota](https://github.com/QuantumNous/new-api/commit/7de10f2c602b50122017556e8a09cb7c488c273c)，`controller/channel-billing.go`。

Sub2API 优先 remaining、回退 balance，并区分无效 key、无限订阅与合法负余额。NewAPI 从令牌余额和可选账号余额中取实际限制，按上游 `/api/status` 的 quota_per_unit 换算，不硬编码本地换算比例。owner quota 是否返回取决于上游逐 key 授权。

本地定位：[现有余额更新器](../../controller/channel-billing.go)。未实现上述两个协议，当前批量任务跳过 multi-key，且成功余额不大于 0 会触发禁用。

最小实施：给既有渠道余额获取增加明确的查询协议配置/策略，复用后台更新任务，不为余额另加两套 Relay。返回结构区分有限金额、无限、未知、过期和错误；只有确认的有限金额才能进入既有余额不足判断。NewAPI quota_per_unit 可缓存，但要保留新鲜度。涉及跨币种时沿用本地显式换算规则。

如需要 RenewAPI 也向下游提供 owner quota，作为独立逐 key 授权字段，旧 key 默认不扩权；不为了读取别人的余额而自动公开本站账号总余额。

验收：有限/无限 token、缺 owner 权限、零余额、负钱包、无限订阅、响应缺字段、单位变化、key IP 限制、查询失败不误禁用。multi-key 的余额展示/调度留到 F02 的凭据级快照统一接入，避免两套数据互相覆盖。

### F07：Mantle

源码参考：NewAPI [AWS Mantle Messages](https://github.com/QuantumNous/new-api/commit/d032204733db31208f10bac118b6e683aab3bf29)，`relay/channel/aws/mantle.go` 与 adaptor 分支。

实现使用 `/anthropic/v1/messages`，支持 API key 或 AccessKey/Secret/Region 的 SigV4；签最终请求体和最终请求头，响应复用 Claude JSON/SSE 处理。它与现有 InvokeModel envelope 不同；该路径要求 Claude，Nova 继续走原路径。

本地 AWS 适配器没有 Mantle。最小实施为现有 AWS 渠道增加显式 API 模式，复用 Claude 响应、错误、用量和已有 HTTP 客户端；默认配置仍沿用既有模式。不照搬上游业务代码的 encoding/json 调用，使用 common JSON 封装。

验收：两种鉴权、区域与模型映射、最终覆盖后签名、流/非流、错误及缓存用量；确认实际账户/区域可用后接入。没有对应供应资源时收益有限。

### F08：搜索事件及引用跨协议转换

源码参考：CPA [Gemini 搜索流与 grounding 映射](https://github.com/router-for-me/CLIProxyAPI/commit/7fcbdf8896753d7af60f75c6c20d27bf0473f195)、[Codex→Chat URL 引用](https://github.com/router-for-me/CLIProxyAPI/commit/16d98881d4bb37adaa827599e4be8f5154e81646)。核心为 `gemini_openai-responses_web_search.go` 和 response 状态机。

已核对搜索能力判断、流状态、迟到 grounding 的合并与索引重映射、按 URL/字符区间去重、UTF-8 字节到字符位置转换。其价值是让搜索工具执行过程和引用跟随答案一起返回，不只是给文本后面加链接。

本地已有 Gemini googleSearch 字段与原生搜索请求；`service/openaicompat` / `relay/responsebridge` 没有对应 grounding→Responses 语义转换。最小实施先做 Gemini→Responses 的明确模型组合，补齐搜索开始/完成、sources 和 citations，再考虑其他协议对；能力声明与工具收费同时覆盖。复用现有流状态机，禁止将整个普通回答无条件缓存到结束才输出；需等待 grounding 的部分设置缓冲上限。

验收：中文和 emoji 引用位置、多文本块、多条消息、重复/迟到来源、搜索失败、无搜索结果、流与非流一致、取消及工具用量。官方搜索计价口径需实施时重新核对，不复制参考仓库静态价格作为长期事实。

## 已有能力与暂缓项

| 项目 | 结论 |
| --- | --- |
| Codex OAuth、导入、预检、刷新、usage 查询 | 本地已有；F02 只补查询结果到调度的连接。 |
| 会话/渠道/key 粘连、失败后会话避 key | 本地已有；不搬 CPA 整套账号池或新建粘连层。 |
| Responses WebSocket 逐轮执行与计费 | 已有；不作为新功能重复引入。 |
| 原生/legacy Responses compaction、能力探测和续写绑定 | 已有；不笼统推荐“新增上下文压缩”。 |
| 公共模型 metadata 同步、ETag、渠道模型后台更新 | 已有；F04 只补 Vertex 发现路径，CPA 模型目录可作为参考，不再另建同步系统。 |
| CPA Claude 自有 compaction capsule | 暂缓。已读 [摘要/封装实现](https://github.com/router-for-me/CLIProxyAPI/commit/2bc236d2fcc1b43d4405b85cd8155b4b8561e149)：额外调用模型生成摘要再 seal，自有格式并非 OpenAI 原生 encrypted_content。该提交还会丢弃不认识的外部 compaction 项，本地应保留明确拒绝，不能搬此默认行为。若未来实现，必须独立定义可识别格式、租户绑定、持久密钥、多实例兼容和额外计费。 |
| Gemini / Vertex Interactions 原生接口 | 后续需求候选。已读 [Vertex executor 与协议处理](https://github.com/router-for-me/CLIProxyAPI/commit/2d5351b3e5822ad60c34410c5e62a88897f00b38)：仅配置启用且请求为 Interactions 时进入对应路径，包含流/非流、鉴权与 revision。RenewAPI 目前无该入口；若要做需另审资源归属、状态续接、取消和账务，不视为已经完成跨协议能力。 |
| 整套插件运行时、自动身份/签名伪装、其他项目账号管理框架 | 不纳入本轮引入方案。按现有 adaptor / service 架构适配具体能力。 |

## 实施时避免重复的约束

1. 工具转换与能力声明统一放在现有协议桥接及路由能力体系；F01/F08 共用工具生命周期、流状态及用量通路，不各建一套 Relay。
2. F02 的窗口额度与 F06 的货币余额共享采集调度、凭据身份、新鲜度和状态类型；保持两种业务量的语义，不能把窗口百分比换成钱包余额。
3. F04/F07 使用既有供应商适配器与凭据刷新，不新增通用网关框架。所有后台查询使用现有代理/超时配置、有界并发和请求合并。
4. 先增加明确能力，再放开对应请求；不靠删字段让不支持请求看起来成功。新增持久化走 GORM 三库兼容，新增账务走现有持久账本和幂等结算。
5. 每项实施只运行受影响模块的有意义验证；涉及协议、路由、账务或数据库时再完成对应集成矩阵和项目门禁。单独验收、小步引入，不直接合并三家历史。

## 来源与验证边界

固定参考 HEAD：

- NewAPI `QuantumNous/new-api@3e2ce2c8856e32ca7054d673f8cd059ae466e80c`。
- Sub2API `Wei-Shaw/sub2api@b0a02a3304a0a78cdd003ddda1dd23ba02c03084`。
- CPA `router-for-me/CLIProxyAPI@911f49cbd49f4fe2516979754a7488a3fd14fe64`。

在上一轮提交目录基础上额外保存 24 个功能候选 patch，并对上述入选功能的生产函数、后续关键修订及本地对应实现做定向阅读。引用是固定提交链接，不以发布说明或提交标题代替代码证据；不是三仓库所有文件逐行审完的声明，也不是上游所有功能的穷举。未采用的 patch 仅收集，不代表全部审阅。

证据目录：`D:/Code/renewapi/audit/backend-features-20261017/`，包含 `candidates.json`、patch 和校验记录。仓库间不运行外部脚本，不执行真实供应商请求。本轮仅更新评估文档与处置记录，验证来源 SHA/祖先关系、文档引用与差异；没有新功能实现或运行时验收，不重跑全仓测试，不推进原完整上游审计基线。
