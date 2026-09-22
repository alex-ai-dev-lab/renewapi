# 2026 年 9 月可靠性完整升级

状态：进行中。验收范围以本文件及用户列出的 22 项核心测试为准，不能把部分完成当作完成。

## 安全基线

- 审计日期：2026-09-22。
- 基线：`main` / `origin/main` 均为 `4def3a8848e336532c55e9c3070cc78ead67e378`。
- 开发分支：`upgrade/2026-09-reliability`，工作树：`D:\Code\renewapi\upgrade-reliability`。
- 原 `source` 的 `agent/06-billing` 分支和未提交的技能文件删除保持原样。旧 Billing 功能提交已进入主线，独有的阻塞说明不作为源码事实。
- 不部署、不修改生产数据库、不调整生产默认 Billing 模式、不移动已有 tag。

## 当前源码差距审计

| 项目 | 初审状态 | 当前证据与处理 |
| --- | --- | --- |
| 请求级 Failover 状态、最多 6 个不同渠道 | TODO | `controller/relay.go` 仍用 `RetryTimes`，每个模型路由重置排除集合，同渠道兼容重试可绕过限制。 |
| Enabled、模型、排除、priority、weight | NOOP | `model/ability.go` 与缓存路径已有筛选和排除后最高剩余优先级逻辑；补集成验证。 |
| 首语义输出 watchdog | PARTIAL | 已有默认 15 秒配置，但 `stream_scanner.go` 任意 data 帧即解除。 |
| 提交前暂存与提交后协议边界 | PARTIAL | 已有 StreamStatus、恢复和提交检测，缺少通用语义提交门。保留提交后不拼流的边界。 |
| 错误分类与健康度 | PARTIAL | 已有渠道/模型/会话分类；统一前置失败重试与逐次健康记录。 |
| 公共错误、管理员完整尝试链 | TODO | 有部分脱敏和 admin_info，但错误 Log.Content 仍含真实错误且只记首次。 |
| Prompt Profile、CRUD、渠道引用 | TODO | 仅 `ChannelTestSetting.Prompt`。采用现有 GORM 迁移。 |
| Anti-Poison 保留管理员 Prompt | TODO | nonce 当前直接覆盖 Prompt。 |
| 自动测试仅 AutoDisabled | TODO | scheduler 当前同时测试 Enabled 和 AutoDisabled。 |
| default/classic Channel Test UI | TODO | 两套前端均仍在维护，均需同步。 |
| selector 主题与 autofocus | TODO | ComboboxContent 强制 dark，Chips 为 bg-transparent，按实际组件最小修复。 |
| shadow 持久化退款 | NOOP | ADR-004 和当前 BillingSession 已使用 ledger 事务；不是旧审计中的异步退款。 |
| off 退款、enforce 上线验证 | PARTIAL | off 仍有异步终态风险；现有 enforce 和 reconciler 需故障、幂等、进程重启与数据库验证。 |
| Options 主键及 pricing 回滚 | PARTIAL | Option 已声明 primaryKey，批量写入有回滚；继续验证旧表迁移与实际行为，避免重复实现。 |
| 上游可靠性移植 | PARTIAL | 账本停在 2026-08-14；重新查最新引用，逐项比较当前等价实现。 |
| Responses WebSocket | TODO | 当前仅 Realtime WebSocket 路由；在前述功能稳定后单独实现和测试。 |

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

## 后续执行顺序

1. Failover 状态、语义 staging、统一错误和日志、故障注入。
2. Channel Test 数据模型、CRUD、引用安全、scheduler、两套前端。
3. Billing enforce 验证和必要补修，Options 兼容验证。
4. 最新上游逐项审计与最小移植。
5. Responses WebSocket，逐轮计费、取消、错误关联与容量边界。
6. 完整测试、独立复核、聚焦提交、更新文档及开发分支推送。
