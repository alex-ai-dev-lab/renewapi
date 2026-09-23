# 2026 年 9 月可靠性完整升级

状态：DONE。用户指定升级范围均已实现或确认 NOOP；22 项核心行为均有验收证据。源码验证完成，生产发布与数据库切换不在本任务范围内。

后续维护说明：工作分支已合入 `main`。本文保留升级期间的历史证据；旧 Actions 已按用户授权清理，当前统一验证及可下载 QA 证据见 [发布维护记录](2026-09-release-pipeline.md) 和 [Releases 分发](../../docs/release-distribution.md)。

## 安全基线

- 审计与验收日期：2026-09-22 至 2026-09-23。
- 基线：`main` / `origin/main` 均为 `4def3a8848e336532c55e9c3070cc78ead67e378`。
- 开发分支：`upgrade/2026-09-reliability`，工作树：`D:\Code\renewapi\upgrade-reliability`。
- 原 `source` 的 `agent/06-billing` 分支和未提交的技能文件删除保持原样。旧 Billing 功能提交已进入主线，独有的阻塞说明不作为源码事实。
- 不部署、不修改生产数据库、不调整生产默认 Billing 模式、不移动已有 tag。

## 源码差距与最终判定

初审基于 `4def3a884`，最终源码为 `a2321c2b1dae57a98c4ed18ec3197f97cd76c62d`。阶段记录保留当时的进度，最终结论以本表及验收结果为准。

| 项目 | 初审 | 最终 | 实际处理 |
| --- | --- | --- | --- |
| 请求级 Failover、最多 6 个不同渠道 | TODO | DONE | `acc460482`：独立状态与跨路由预算；已失败 Channel ID 不重新选择。 |
| Enabled、模型、排除、priority、weight | NOOP | NOOP | 复用 `model/ability.go` 与缓存路径，真实六渠道测试验证排除后先选最高剩余优先级。 |
| 首语义输出 watchdog | PARTIAL | DONE | `99703c6ea`：复用原 15 秒配置，前导/keepalive 不解除期限。 |
| 提交前暂存与提交后协议边界 | PARTIAL | DONE | `99703c6ea`：最终 writer 暂存，复用 StreamStatus 与恢复；提交后不拼流。 |
| 错误分类与健康度 | PARTIAL | DONE | `8b8c9e9a7`：复用 failure scope，逐次失败记录，不因最终成功抹除健康故障。 |
| 公共错误、管理员完整尝试链 | TODO | DONE | `8b8c9e9a7`、`a2321c2b1`：统一 capacity、历史日志脱敏、错误事件扩展字段隔离、真实状态与关联。 |
| Prompt Profile、CRUD、渠道引用 | TODO | DONE | `c36dbfc0c`：独立表、迁移、默认项、引用拒删和兼容回退。 |
| Anti-Poison 保留管理员 Prompt | TODO | DONE | `c36dbfc0c`：追加 nonce，保留原测试语义。 |
| 自动测试仅 AutoDisabled | TODO | DONE | `c36dbfc0c`：默认 false，仅影响自动 scheduler，手动测试继续可用。 |
| default/classic Channel Test UI | TODO | DONE | `3674bfc78`：两套仍在维护，均同步管理页面、渠道选择、参数和开关。 |
| selector 主题与 autofocus | TODO | DONE | `3674bfc78`：最小作用域背景修复、移除强制 dark、避免 dialog 打开即弹出。 |
| shadow 持久化退款 | NOOP | NOOP | 基线 BillingSession 已使用 ledger 事务，保留现有成熟实现。 |
| off 退款、enforce 上线验证 | PARTIAL | DONE | `ea9a4f994`：同步退款、终态锁、故障与幂等验证、独立进程恢复和四数据库 CI；默认不变。 |
| Options 主键及 pricing 回滚 | PARTIAL | DONE / NOOP | `ea9a4f994`：修复旧表缺实际主键；定价更新不重置其他配置已有等价实现，NOOP。 |
| 上游可靠性移植 | PARTIAL | DONE | `ada0a55bb`、`a2321c2b1`：补用量、工具参数与返回模型；限流最终结果、既有 StreamStatus 等价能力保持 NOOP，详见上游账本。 |
| Responses WebSocket | TODO | DONE | `7c2cf87cd`、`a2321c2b1`：独立入口、逐轮执行、原生/HTTP 传输、取消、错误关联、容量与恢复；最后单独实施。 |

## 实施与验证记录

- 已读取根目录 AGENTS、两套前端维护约定、PROJECT_STATE、ARCHITECTURE、维护文档、相关 ADR、active 任务和 UPSTREAM_PORTS。
- Go 1.25.1、Bun 1.3.14 可用。未发现 Docker、MySQL/PostgreSQL 命令；数据库运行证据待实际建立，不能把 CI 配置当作通过。
- 基线 `go test ./controller ./relay/helper ./relay/common ./model ./service` 通过。
- 第一组：独立 `ChannelFailoverState`，预算跨路由共享；同渠道不再执行已失败的兼容重试；取消停止选择；旧 `RetryTimes=0` 不压缩新预算。真实 HTTP 集成覆盖六次失败、前五次失败第六次成功、两个 priority 100 先于 90、禁用渠道排除、只结算成功请求。`go test ./controller ./service ./relay/common` 通过。
- 第二组：跨协议语义分类、响应头/SSE 暂存、15 秒整体首语义期限、结算前有效流校验和提交后不拼流。真实故障注入及 `go test ./controller ./relay/... ./service` 通过；原“仅 response.created 即提交”的测试已改为实际内容后断流。
- 第二组并发检查：`go test -race ./controller ./relay/helper -run 'TestChannelFailoverUsesSix|TestSemantic|TestStreamStaging|TestStringDataPartial' -count=1` 通过。
- 第三组：统一客户端 capacity 错误；保留本地用户拒绝；失败日志与历史 self/token 日志脱敏；管理员保存凭证清理后的真实错误和完整尝试链；成功消费日志也保存前序失败。所有可切换失败写入渠道健康记录，禁用资格继续复用现有规则。相关九个 Go package 测试通过。
- 第三组定向 race 测试通过。
- Channel Test 后端：独立 Prompt 表与增量迁移、管理员 CRUD、引用计数、并发安全拒删、默认唯一性、渠道配置与审计接入、完整回退链、追加 nonce、自动测试仅 AutoDisabled 开关；修复本地错误误恢复和测试全部读取失败后运行锁未释放。SQLite 与实际 HTTP 行为验证通过；两套 UI 及外部数据库运行验证继续推进。
- 两套前端：在现有运营设置中增加 Prompt 管理、引用提示、自动恢复筛选及所有测试参数，渠道编辑器保存稳定 Prompt ID，管理员用量详情展示完整错误链。default 仅渠道模型选择器增加不透明背景，移除 Combobox 强制 dark，修复聚焦时自动展开。classic 修复 Prompt 弹窗重复字段 ID，并在选中时同步保存字段，避免关闭动画前提交旧值。
- 前端验证：default `bun test` 为 68 通过、0 失败，typecheck、改动文件 eslint/Prettier、copyright 通过；两套生产 build 通过。隔离 SQLite 服务和临时浏览器实际验证两套 Prompt CRUD、引用拒删、开关保存、渠道 Prompt ID 0/1 来回保存，以及 8 组 light/dark、1440/390 宽度、新建/编辑模型选择器（已有/自定义模型、hover、selected、dropdown、无自动展开）。
- 前端全量存量检查：default lint 基线 121 errors / 34 warnings，当前 119 / 34；format 基线 94 个文件，当前 91；classic Prettier 基线 56 个文件，当前 55。未扩大修改无关存量问题。构建产物排除在 classic 格式检查之外。测试日志和截图仅位于忽略目录 `.test/`，没有修改生产数据或浏览器用户配置。
- Billing：off 退款改为同步资金/Token 事务，失败保留重试状态；补齐退款/结算终态锁、终态后预扣拒绝及补充预扣失败不提前结算。修复 enforce 在 Failover 后将用量记入最初渠道的问题，补偿也保留成功渠道。shadow 的持久化余额路径保持 NOOP，默认模式不变。
- Billing 验证：钱包/订阅 × off/shadow/enforce 的故障回滚、重复终态与并发验证通过；独立进程强制退出和重启覆盖待结算、待退款、孤儿预扣及未提交事务回滚。相关 model/service/controller/channelconfig 完整 package 测试及定向 race 通过；新增成功渠道补偿、六渠道真实 Failover 的账本归属回归及 race 通过。零额度 reservation 失败后也立即完成退款终态。
- Options：已复现旧唯一索引表无法自动补主键，新增 `options-primary-key:v1`；SQLite 保留外键引用、索引、触发器，歧义数据拒绝无损迁移。配置 key 条件交给 GORM 引用，定价单项写入及失败回滚通过。新增开发分支专用 MySQL 5.7/8.4、PostgreSQL 9.6/16 验证工作流，等待实际运行，不将配置文件记作 PASS。
- 数据库实际证据：`ea9a4f994` 对应 Actions [35764799379](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35764799379) 的 MySQL 5.7/8.4、PostgreSQL 9.6/16 全部成功，已检查每个 job 的结果。
- 上游专项复审：已获取 New API `996adffe`、Sub2API `20a94fbb`，补 Responses 截断用量、Gemini/Responses 跨协议 usage 请求、管理员返回模型记录和 function arguments done 去重。限流最终结果、已有 StreamStatus 与语义输出行为标记 NOOP；不采用上游“仅前导事件即计费”的行为。相关 Go package 测试、default 类型检查、改动文件 lint 与 68 项前端测试通过。WebSocket 仍待独立阶段。
- WebSocket 阶段：新增独立 Responses WS 入口、逐轮鉴权/限流/账本、32 lane FIFO/并行、错误关联、取消、容量边界与有限历史恢复。默认 HTTP/SSE 桥接，渠道可开启原生 WS；连接按客户端/lane 隔离，带常驻 reader、执行范围校验及安全增量续写。两套前端开关在实际浏览器中均完成 true/false 保存验证。
- WebSocket 故障验证：真实 WS 与 HTTP 服务覆盖六渠道切换、原生握手失败、response.failed、错误 lane/event、迟到 terminal、提交后 EOF、不拼流、密钥轮换、idle ping、warmup、跨连接历史隔离、Token 模型/RPM/concurrency 重新检查、队列/输入/连接容量释放、关闭进程时退款，以及结算失败后隐藏 terminal 并补偿。专项 race 通过。
- WebSocket 复核补修：客户端在握手阶段取消时不记渠道故障；Anti-Poison 内部探测仍走原有 HTTP，不借用生成会话；正文/历史有上限；终态及限流释放先于客户端 terminal。决策及配置见 ADR-009 和 `docs/responses-websocket.md`。
- 完整检查第一轮：`go test ./... -count=1 -timeout 5m`、`go vet ./...`、Go 可执行文件构建、双前端生产构建通过；default 类型检查、68 项测试、改动文件 lint 和 copyright 通过。全量存量 lint/format 仍为 default 119 errors / 34 warnings、91 个格式文件，classic 55 个格式文件。独立 diff 复核、最终文档及最终分支数据库 CI 仍在收尾。
- 独立复核补修：复现并修复 terminal 补齐部分 delta 时低估用量、null 用量误当显式零、失败事件额外 debug/metadata 泄露和无 response 对象时缺公共文案。原生 WS 不再把服务端顶层 event_id 当作请求 ID；明确旧请求错误仍隔离。统一解析 HTTP/WS error 事件，管理员尝试链保留真实 429 等状态。回归先复现失败后修正，`types/dto/service/relay/.../controller` 完整 package 测试及六个 package 的相关定向 race 通过；最终全仓检查继续执行。

## 最终验收

| 编号 | 用户要求 | 结果与证据 |
| --- | --- | --- |
| 1 | priority 100 剩余候选先于 90 | PASS，`TestChannelFailoverUsesSixDistinctChannelsAndRemainingPriority`。 |
| 2 | 同一 Channel ID 每请求最多一次 | PASS，同上，验证实际上游调用集合。 |
| 3 | 初始渠道加 5 次切换后停止 | PASS，同上，保留第七个候选仍最多调用六次。 |
| 4 | 六渠道全失败返回 capacity | PASS，真实 HTTP 与 `TestResponsesWSFailoverAndCapacityCorrelation`。 |
| 5 | 前五失败、第六成功只见成功 | PASS，真实 HTTP/WS 验证响应没有前序 hostname、ID 或错误。 |
| 6 | 15 秒无语义输出触发切换 | PASS，`TestSemanticFailoverFaultInjection/keepalive_stall` 实际等待 15 秒。 |
| 7 | SSE keepalive 不算语义 | PASS，同上及 `TestStreamStagingCommitsOnlySemanticOutput`。 |
| 8 | response.created 不算语义 | PASS，前导后 EOF/stall 故障注入与 staging 行为测试。 |
| 9 | 真正 content delta 解除 watchdog | PASS，`TestSemanticDeltaDisarmsWatchdog`。 |
| 10 | 客户端取消停止 Failover | PASS，HTTP client_cancel、WS 单 lane 及整连接取消。 |
| 11 | 用户日志没有真实上游错误 | PASS，`TestUserLogsRedactHistoricalUpstreamDetails`、真实 Failover 日志及错误事件回归。 |
| 12 | 管理员完整 attempt chain | PASS，六渠道真实日志断言，以及原生 WS 429 状态/真实错误断言。 |
| 13 | AutoDisabled 被 scheduler 测试并恢复 | PASS，`TestAutomaticChannelTestOnlyAutoDisabledAndPerChannelPrompt`。 |
| 14 | only_auto_disabled 时 Enabled 调用为 0 | PASS，同上检查实际 HTTP 调用次数。 |
| 15 | 每渠道独立 Prompt | PASS，同上检查各渠道真实请求正文；双前端保存验证通过。 |
| 16 | 删除引用 Prompt 无悬空引用 | PASS，CRUD、`TestChannelTestPromptLifecycle`、并发引用/删除及四数据库验证。 |
| 17 | Anti-Poison 不覆盖管理员 Prompt | PASS，`TestChannelTestAntiPoisonKeepsAdministratorPrompt`。 |
| 18 | light/dark selector 背景正常 | PASS，临时浏览器覆盖 1440/390、新建/编辑、已有/自定义、hover/selected/dropdown 共 8 组；无自动展开。 |
| 19 | truncated Responses usage 正确 | PASS，`TestResponsesTruncationRetainsUsageWithoutSettling`、terminal 部分 delta 补齐、null/显式零、工具参数去重。 |
| 20 | Failover 仅有效成功请求结算 | PASS，六渠道真实账本归属、原生/HTTP WS 逐轮账本；结算故障保留 reconcile_required。 |
| 21 | Billing settle/refund 幂等 | PASS，钱包/订阅 × off/shadow/enforce，故障回滚和并发终态；四数据库 CI。 |
| 22 | enforce reconcile 可恢复 | PASS，故障注入、成功渠道恢复、重复补偿、SQLite 独立进程崩溃与重启。 |

故障注入包含 401/403/429/500/502/503、connection refused、TLS、真实 15 秒 stall、empty/malformed SSE、response.failed、中途 EOF、客户端取消；原生 WS 另覆盖握手、错误 lane/event、迟到 terminal、密钥轮换、idle ping 和结算失败。

## 最终检查与提交证据

- 最新源码 `a2321c2b1`：`go test ./... -count=1 -timeout 5m`、`go vet ./...`、`go build -o .test/renewapi-upgrade-final.exe .` 全部通过；本轮所有 Go 改动文件 `gofmt -l` 无输出。
- 最新定向 race：`go test -race ./controller ./relay/channel ./relay/channel/openai ./relay/helper ./service ./types -run 'TestResponsesWS|TestResponsesErrorEvents|TestResponsesUsage|TestPublicRelay|TestSemantic|TestStreamStaging' -count=1 -timeout 180s` 通过。Billing/model 的专项 race 已在所属提交阶段通过。
- default：`bun run typecheck`、`bun test`（68 pass / 0 fail）、改动文件 eslint/Prettier、`bun run copyright:check`、`bun run build` 通过；classic `bun run build` 通过。classic 没有独立 test script，使用真实浏览器验证相关流程。
- 全量检查已执行但仍有存量问题：default lint 从基线 121 errors / 34 warnings 降为 119 / 34，format 从 94 个文件降为 91；classic lint 从 56 个格式文件降为 55。未将局部通过写成全量通过，也未扩大无关重构。
- 实际数据库 CI：[35773097071](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35773097071) 对应 `a2321c2b1dae57a98c4ed18ec3197f97cd76c62d`，MySQL 5.7/8.4、PostgreSQL 9.6/16 全部 success，已逐 job 核对。更早 Billing 提交 `ea9a4f994` 的 [35764799379](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/35764799379) 也全部成功。
- 独立最终 review 重新检查基线至当前提交的 routing、错误/日志、staging、Prompt 引用、Billing/Options、双前端和 WS。发现的用量、错误扩展字段、event_id 关联及真实状态问题已复现、修复并重测；没有新增依赖、无关重构、生产开关强制启用或生产数据变更。
- 本地完整日志与浏览器产物保存在忽略目录 `.test/`，最新日志为 `final-source-go-test.log`、`final-source-go-vet.log`、`final-source-go-build.log`、`final-review-race.log`；CI 是可独立查看的远端数据库证据。
- 聚焦提交：`917bc5c8b` 审计、`acc460482` Failover、`99703c6ea` staging、`8b8c9e9a7` 公共错误与链路、`c36dbfc0c` Prompt/scheduler、`3674bfc78` 双前端、`ea9a4f994` Billing/Options、`ada0a55bb` 上游可靠性、`7c2cf87cd` WS、`a2321c2b1` 最终复核补修。文档收尾独立提交，功能源码保持不变。

## 边界与未完成项

本轮升级范围没有 TODO/PARTIAL/BLOCKED 项。没有生产部署、生产数据库迁移、生产 enforce 切换或产品发布；这些不在本轮授权范围内。默认 Billing 仍为 shadow，off 没有跨进程恢复保证。

WS 不支持 mid-turn steering；generate:false 仅预热本地输入；不跨客户端共享带状态的上游连接。store=false 在断线/快照淘汰后必须完整 input 恢复，已提交工具调用不自动重放。两套前端存量 lint/format 问题仍属于原有技术债。

最新上游仅完成指定可靠性专项复审，完整历史审计基线仍为 2026-08-14。旧 UX/性能任务的其他条目未自动归入此次完成状态。

本轮升级范围已全部完成。
