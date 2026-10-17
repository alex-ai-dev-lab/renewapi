# NewAPI、Sub2API、CPA 更新检查（2026-10-17）

结论：有值得适配的更新。优先处理依赖安全升级、Playground 账号隔离、Claude 缓存用量、分组限流、冷却竞态和混合流正文丢失。兼容修复应落在已有转换器、限流器和状态模型中，避免复制上游整套框架。

本轮交付是候选审查和修复方案，**尚未导入这些更新**。保留当前 main 工作副本的上轮未提交修复，没有提交、推送或部署。以下发现独立于上一轮 29 条问题的验收记录；已有回归通过不能证明其他输入和路径没有缺口。

## 来源与范围

本地 HEAD：`32f0d2bf9a4e41b1564dce167d5a434c9129eaa3`，工作副本 `D:/Code/renewapi/repair-20261017`，包含上轮未提交修复。

| 来源 | 本轮固定 HEAD | GitHub `/releases/latest` 当日指向 |
| --- | --- | --- |
| [NewAPI](https://github.com/QuantumNous/new-api) | `3e2ce2c8856e32ca7054d673f8cd059ae466e80c` | [v1.0.0-rc.44](https://github.com/QuantumNous/new-api/releases/tag/v1.0.0-rc.44) |
| [Sub2API](https://github.com/Wei-Shaw/sub2api) | `b0a02a3304a0a78cdd003ddda1dd23ba02c03084` | [v0.2.17](https://github.com/Wei-Shaw/sub2api/releases/tag/v0.2.17) |
| [CLIProxyAPI（CPA）](https://github.com/router-for-me/CLIProxyAPI) | `911f49cbd49f4fe2516979754a7488a3fd14fe64` | [v8.0.32](https://github.com/router-for-me/CLIProxyAPI/releases/tag/v8.0.32) |

列举 2026-10-08T00:00:00Z 之后非合并提交：85 / 90 / 116，共 291 条；保存其中 49 个候选完整 patch，针对下表涉及的生产路径定向核对和复现。**不表示 291 条全部逐行审计，也不表示 49 个 patch 全部具备导入条件。** Release API 受 403 限流，版本由 GitHub latest 网页重定向补核；HEAD 与 release 标签不是同一概念。

原全量上游审计基线保持 `58d4e9bd3bb035df8ea235dd682ccc8a45d0332a`，不因本次专项检查推进。NewAPI 为 AGPL-3.0，Sub2API 根许可证为 LGPL-3.0，CPA 为 MIT；实际移植时保留适用文件的来源及许可说明。

## 优先适配

P1 表示应优先安排修复，不等同于已证明可在生产利用。表中“复现”使用本地合成输入和隔离存储/数据库，不访问真实账号或上游模型。

| ID / 优先级 | 来源与本地证据 | 最小修复与验收 |
| --- | --- | --- |
| U01 / P1：Playground 跨账号本地数据 | NewAPI [`06dd76a`](https://github.com/QuantumNous/new-api/commit/06dd76a9d38eb0eaf8e900b1264b500250156c4d)。本地 `web/default/src/features/playground/constants.ts:69` 使用全局键；`lib/storage.ts` 无账号作用域。复现 A 写入、退出、B 登录后仍加载 A 消息；query cache 已清理 3 次，不能解决 localStorage 问题。 | 消息、配置、参数状态统一按账号 ID 命名空间；退出不读写共享数据；无归属旧数据不能默认认领给下个登录账号。复用现有 sessionVersion，在账号变化时重置挂载状态、终止旧流和写入；验证 A→B→A、跨标签、匿名、旧键、存储不可用、旧异步写入。无需新增第二套认证 store。 |
| U02 / P1：Claude 纯缓存用量与零值 | NewAPI [`1b63c28`](https://github.com/QuantumNous/new-api/commit/1b63c28dcc999c4445c70f21ec90103ab49e8be5)。`relay/channel/claude/relay-claude.go:943` 将普通 input=0 当作缺失。复现缓存读 50000、估算输入 1200 时，完整零输出及断流均错误补入普通输入 1200。`service/text_quota.go` 还以 TotalTokens=0 清零 quota。 | 缺失判断包含缓存读写及 5m/1h；有输出且缺用量时再估算，保留真正零输出。缓存自身可计费，统一一个 billable-usage 判定，计算、结算、统计和日志共用。保留现有缓存口径及账本提交门；验证缓存-only、拒绝、工具-only、断流、正常用量，及 off/shadow/enforce 与三库结算。当前复现证明用量污染，不把它冒充真实支付扣费复现。 |
| U03 / P1：独立分组限流串桶 | NewAPI [`45fbb14`](https://github.com/QuantumNous/new-api/commit/45fbb149f0c89ff48d805335f22419be392d0350)。`middleware/model-rate-limit.go:31,126` 的 key 只有 user。内存和 miniredis 均复现：A/B 各限 1 次，结果 204/429；未独立配置的两组共享限制 204/429 符合原契约。 | 仅配置独立限制的分组使用 user+group key，其余继续共享 user key；总请求与成功请求作用域一致。保留现有 reservation、语义成功、失败归还和 Redis fail-closed。验证两后端并发、无独立配置、服务端失败归还及配置切换的旧窗口边界。 |
| U04 / P1：迟到成功清除新冷却 | CPA [`35af74f`](https://github.com/router-for-me/CLIProxyAPI/commit/35af74f92721b67c2eb704d0809f26b034946f56)。`model/channel_model_status.go:455` 未校验尝试版本。隔离 SQLite 复现失败建立 600 秒冷却后，任意成功更新把 DisabledUntil 清零、状态恢复。 | 派发时捕获状态代次，成功恢复用原子版本/CAS 判断；失败推进版本。版本需要适用于多实例，不能照搬进程内序号。保留合法新探活恢复、手动禁用和过期重试；三库验证失败/旧成功乱序、新探活、重复结果及跨实例。无需导入 CPA auth manager。 |
| U05 / P1：reasoning 与正文同块时丢正文 | NewAPI [`5ac7562`](https://github.com/QuantumNous/new-api/commit/5ac75628409de7e3adde7d36771700714af50853)。`service/convert.go:456,628` 仍有互斥分支。外部调用 `StreamResponseOpenAI2Claude` 复现首块和后续块都只输出 thinking，丢 `ANSWER_REVIEW`。 | 同一块顺序输出 thinking、text、tool，用现有 stop/start/index 生命周期管理，避免首块和后续块各维护不同逻辑。验证三类内容组合、首块终态、并行工具、空流和内容块类型，不与已修复的文本+工具聚合混为一项。 |

## 后续兼容、性能与交互适配

| ID | 来源与缺口 | 最小适配 / 验证 |
| --- | --- | --- |
| U06 / P2：图片双向转换 | NewAPI [`ff36846`](https://github.com/QuantumNous/new-api/commit/ff3684658fc1ab5747bda9a142910a617df8eb10)、[`13c8e90`](https://github.com/QuantumNous/new-api/commit/13c8e90b8f4b929aa88c2e9616efc462e0068d72)。复现 Chat 的 low/high/auto/original 全丢，Claude 图未给默认 detail；反向 Responses→Chat 仍输出字符串 image_url（`responses_to_chat.go:219`）。 | 两个方向集中处理图片形状与 detail；Chat→Responses 保留值、缺省 auto；反向必须包装 `{url:...}`。对新版 detail 的兼容按目标能力明确处理，不全局静默删除；验证普通图、工具结果图、URL/data/file_id 和严格校验。继续保留用户确认的 MJ 图片公开分享契约。 |
| U07 / P2：工具失败语义 | CPA [`59930ba`](https://github.com/router-for-me/CLIProxyAPI/commit/59930ba36a8228d199f7b1baa0ae61ef44435ff2)。`responses_claude_request.go:198` 忽略 function_call_output 的可选 status；复现 incomplete 与 completed 输出相同且没有 is_error。 | 兼容客户端明确提供 incomplete 时映射 is_error；不根据错误文本猜测失败。复用已有反向工具错误包装，验证双向及多模态工具结果。该 status 处理是兼容约定，不把可选扩展宣称成所有 Responses 客户端必填字段。 |
| U08 / P2：多行 anthropic-beta | NewAPI [`767a82b`](https://github.com/QuantumNous/new-api/commit/767a82b8764778b847462ebecdc231eacd1e282c)。`claude/adaptor.go:75` Header.Get 只取第一行；复现 beta-a/b 只转发 beta-a。 | 使用 Header.Values 合并、trim、去空值，接现有渠道覆盖规则；验证 Claude、Bedrock 转 body、Vertex 的实际适用路径。thinking.block_binding 和未知字段的原生透传已具备，不增加重复 DTO，也不采用 CPA 自动注入 binding/beta。 |
| U09 / P2：容器过载误判 | NewAPI [`09109ec`](https://github.com/QuantumNous/new-api/commit/09109ec315a5086366f94bcce00f177ec831dbe1)。静态确认 `common/system_monitor.go:62` 仍用宿主机 cpu.Percent/mem.VirtualMemory，`middleware/performance.go:37` 单样本直接拒绝且无新鲜度。仅启用性能哨兵时触发；本轮未用容器压力实验确认生产发生频率。 | 在现有采样器内支持 cgroup v1/v2 CPU 配额、cpuset 和内存限制；累计连续样本并记录时间，过期样本不持续熔断。保留非容器回退、无配额情况及错误码；用真实限额容器、抖动、采样停滞验证。不要在每个请求读文件或增加另一套轮询。 |
| U10 / P2：Responses reasoning 完整性 | NewAPI [`ca2cae6`](https://github.com/QuantumNous/new-api/commit/ca2cae676db87333a583b6e104d0f1f7366f3258)。本地缺口更大：`dto/openai_response.go:350` 没 summary；复现 typed roundtrip 丢 summary，`ChatResponseToResponses` 和 `ResponsesStreamEmitter` 丢 reasoning。Sub2API [`bdfe215c`](https://github.com/Wei-Shaw/sub2api/commit/bdfe215cbb06f5089112873b14c34c538fc37118) 提醒 created_at/sequence_number 一致性：本地 SSE 已有两者，非流式上游缺时间时合成结果为 0。 | 一个 reasoning 输出模型覆盖非流式和流式；首次 item 必须 summary:[]，后续 summary 与终态一致，稳定 ID/index 和顺序。合成时间仅缺失时补且整轮一致，保留上游有效时间；created_at=0 是值质量问题，不能描述为当前所有路径缺字段。原生透传不为此全部反序列化重写，typed 路径单独验证。 |
| U11 / P2：Vertex API key / 多区域 | NewAPI [`a0bf6a8`](https://github.com/QuantumNous/new-api/commit/a0bf6a8a412764bc296a8e27cf8c4aaa3f3f0ed9)、[`718a4e7`](https://github.com/QuantumNous/new-api/commit/718a4e7f9751c5bf1958747cd0c2c895e2b937d4)。ChannelOtherSettings 无 project_id，API key URL 固定传空 project；复现 us 生成 us-aiplatform.googleapis.com。 | 增加可选项目 ID，API key 有项目走 projects/locations，无项目保留 Express；复用 URL builder 修正 us/eu host。表单与后端联动，验证 JSON 凭据、Express/global/单区/us/eu/自定义 base。CPA 完整 publisher 前缀属于可选兼容，先确认 base URL 约定，不能重复拼路径。 |
| U12 / P2：Gemini schema 约束 | NewAPI [`c10780e`](https://github.com/QuantumNous/new-api/commit/c10780e054e6e518c9d0d8a903526663c9b57d75)，替代早期只删非字符串 enum 的方案。复现 `CovertOpenAI2Gemini` 丢 const/oneOf；本地已有 responseJsonSchema DTO，但工具仍走有损 cleaner。 | 按 API 版本/渠道能力选择 parametersJsonSchema / responseJsonSchema；旧接口仅做明确降级并反馈约束损失。保留深度/大小限制，验证引用、联合、enum、结构化输出及 v1/v1beta/Vertex；不全局删除合法 schema 或直接复制 CPA 的其他供应商过滤器。 |
| U13 / P2：分块加载失败恢复 | NewAPI [`b892a6b`](https://github.com/QuantumNous/new-api/commit/b892a6b2e44df572698f6cf731ff381f1977a1f2)。静态确认 GeneralError 缺 chunk 专门恢复，且 `general-error.tsx:71` 对无 HTTP 状态显示 500。本地 Retry/Reload 按钮已经具备。 | 在当前路由错误页识别 JS/CSS chunk 错误，仅展示错误页时按构建/失败资源在 sessionStorage 标记后最多刷新一次；预加载不刷新，无法写标记不自动刷新，无真实 HTTP 状态不显示 500。验证旧部署 chunk404、持续失败、网络断开、预加载和 429；不用新增第二个全局异常层。 |

主题首屏和 Dark Reader、WebSocket 服务端终止的 close frame、模型卡状态时段属于可继续验证的项目：尚未完成对应浏览器/真实 WS 复现，不计入上表已复现结论。NewAPI 前端目录、状态数据和本 fork 不同，不能按提交名直接认定本地同样存在缺陷。

## 依赖升级：应与行为修复分开验收

NewAPI [`3fe394c`](https://github.com/QuantumNous/new-api/commit/3fe394cdba280c00bc566de493b5fc8399b0a7d1) 的 websocket 升级、Sub2API `c61f6eba` 的 Go/x/net 安全升级均适用。本地不是单纯版本落后：已运行 govulncheck v1.9.0，按 Go1.25.1、linux/amd64、CGO_ENABLED=0 完成源码与符号可达扫描。共返回 86 个去重告警，其中 43 个存在符号可达路径；同一告警可能同时涉及 stdlib 与 x/net，不能把模块条目重复相加。静态可达不等于可利用，漏洞数据库版本时间为 2026-10-08，需在实施和发布门禁再扫。

| 组件 | 当前 | 修复候选 / 依赖关系 |
| --- | --- | --- |
| Go 工具链、标准库 | go.mod / Docker / CI 均固定 1.25.1 | 官方下载列表已核实支持版 1.27.2 和 1.26.9。建议统一升至 1.27.2；若需较小迁移，以 1.26.9 为候选验证。旧 1.25 分支不能只补到早期 patch 就覆盖当前安全库。 |
| gorilla/websocket | 1.5.0 | 至少 1.5.3；GO-2026-6278 弱掩码随机数扫描可达。验证 HTTP/WS 接口、控制帧、取消、错误关联及逐轮结算。 |
| x/net | 0.47.0 | 至少 0.60.0；HTTP/2 资源耗尽、流控/HPACK 等可达告警。该版要求 Go≥1.26，并要求 x/crypto 0.57.0、x/sys 0.48.0、x/term 0.46.0、x/text 0.42.0，必须作为一致依赖集合处理。 |
| x/text | 0.40.0 | 单项修复至少 0.41.0（GO-2026-6629）；与上述 x/net 同升时采用至少 0.42.0，避免两次升级。 |
| x/image | 0.44.0 | 至少 0.45.0（GO-2026-6222，VP8L 内存耗尽可达）；验证图片解码限制及转发路径。 |

最小版本来自当前告警，不代表已经验证的最终锁定版本。升级时同时修改 go.mod/go.sum、Docker 构建参数和 CI 固定版本，选择满足要求的受支持版本后执行全仓测试/vet/build、前端构建、真实三库迁移和关键 HTTP/SSE/WS/图片回归。当前旧版本检查没有替代升级后的验收。

## 去重与不导入项

| 行为 | 判定及理由 |
| --- | --- |
| thinking 扩展字段 | `ALREADY_PRESENT`：dto.Thinking 嵌入 RequestExtensions，原生序列化复现保留 block_binding 和显式 false 未知字段。仅 U08 多行头待补。 |
| 文本与工具共存 | `ALREADY_PRESENT`：Claude→Chat 请求 `service/convert.go:239` 已无 len(toolCalls)==0 限制；已有混合响应/Responses 工具聚合回归。不能由此推断 U05 reasoning+text 已修复。 |
| SSE 多行帧 | `ALREADY_PRESENT`：现有 helper.SSEScanner 及帧总大小限制；不复制 CPA `7795de1` 再造 scanner。 |
| TLS 握手限时 | `ALREADY_PRESENT`：service/http_client.go 已配置 bounded TLSHandshakeTimeout，相关回归通过；不引入 CPA 整套重试。 |
| 搜索菜单权限、图标按需加载 | `ALREADY_PRESENT`：command-menu 已复用权限过滤导航；lobe-icon-loaders 按组件动态导入。保留现有实现，不照搬上游图标包或另一套权限过滤。 |
| Sub2API composite OAuth 歧义路由补丁 | `NOT_APPLICABLE`：补丁针对其 composite 平台账号池和 OAuth 凭据投递。RenewAPI 按 channel 类型与 adaptor 选择上游；不引入该账号池。继续保留本项目路由/凭据边界。 |
| 上游前端重构、插件、账号池、自动 thinking/beta 注入、静态价格目录 | `SKIPPED`：本轮不导入整套架构或默认策略；价格沿既有官方同步路径，不擅改预算、身份签名或 MJ 公开分享。 |
| Sub2API 整套支付/退款流程 | `NOT_APPLICABLE` 于直接复制；本地支付快照/幂等已有独立修复与验收，未做其退款产品功能的移植审计。 |

## 实施分组与验证记录

建议依次实施四组，每组只保留一个共用实现：①安全依赖集合；②账号隔离、计量/限流/冷却；③协议转换（图片、混合流、工具、reasoning、beta）；④按渠道启用的 Vertex/Gemini 及哨兵/错误页体验。分组是后续验收边界，不新增并行框架。

本轮完成外部 Go/Bun 的 23 条行为观测、15 条观测断言及只读 overlay 模型探针；结果是**缺口复现**，不是修复后测试通过。现有 `service/openaicompat`、`relay/responsebridge`、`relay/channel/claude`、`middleware`、`relay/helper` 五包回归及 service 的 HTTP 握手/混合响应定向回归通过。git diff --check 通过，17 个报告提交链接已对照参考 Git 对象核实，49 个 patch 已保存。上轮 110 个文件快照中 109 个完全一致，另一个为本轮明确更新的 PROJECT_STATE 文档，原修复证据未重写。没有业务源码改动，因此未重复上一轮全前端/三库/全浏览器验收。

外部证据目录：`D:/Code/renewapi/audit/upstream-review-20261017/`。关键文件：`sources.json`、`release-license-check.json`、`*-commits.json`、`patches/`、`reproduce.go` / `reproduce.log`、`frontend-probe.ts` / `.log`、`cooldown_probe_test.go` / `cooldown-probe.log`、`probe-summary.json`、`govulncheck-go1.25.1-linux.json`、`vulnerability-summary.json`、`focused-regression.log`、`covered-regression.log`。探针只使用合成账号/消息/模型与内存 SQLite/Redis。

执行扫描器初次自动切至 Go1.26.9，不作为当前 Go1.25.1 标准库结论；随后显式指定实际 Go1.25.1，并按 Docker 的 linux/amd64、CGO_ENABLED=0 配置复核。具体扫描统计与校验清单见外部 summary 和 manifest；源码可达性检查不等于真实生产漏洞利用验证。
