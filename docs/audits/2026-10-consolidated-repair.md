# 合并审计修复与二次复审

日期：2026-10-17。基线：`32f0d2bf9a4e41b1564dce167d5a434c9129eaa3`。

修复位于独立工作副本 `D:/Code/renewapi/repair-20261017` 的 `main`，尚未提交、推送或部署。29 条来源观察按 v2 的 14 个修复包实施：28 条已修复并验证，R-007 按用户确认的“图片允许公开分享”契约保留现有行为；没有部分解决或待产品决定项。在已运行的检查范围内，未发现仍未处理的新增确认回归；这不代表全仓全部源码已人工逐行审完，也不代表生产渠道验收完成。

权威输入为审计目录的 `repair-design-v2.json`、`consolidated-findings.json`、`CONSOLIDATED_ISSUES_AND_FIX_PLAN.zh-CN.md` 和 `PLAN_REVIEW.zh-CN.md`。原证据保持不动。机器可读结果见 [repair-results.json](../../../audit/source-review-20261017/repair-results.json)，长期设计见 [ADR-020](../decisions/020-audit-repair-boundaries.md)。

## 实现与复审原则

授权只增加一次轻量快照查询；支付沿用 TopUp/SubscriptionOrder，嵌入不可变报价和履约状态；名额与订阅共用用户锁顺序。Webhook、管理员核实和补单复用校验与事务，移除废弃的实付反推额度算法，没有新增事件、报价或名额账本。

前端复用 Query、URL 和表单现有职责；共享筛选输入处理防抖、历史导航与 IME；集中处理错误、超时与重试。内存修复保持规范化、身份注入、BodyStorage 的既有顺序，继续支持磁盘与 GetBody。未引入第二套状态库、通用 HTTP 包装层或无量测依据的缓存。

用户确认公开分享后，继续收尾 F14/R-009：日志两种分页共用筛选和渠道补充查询，UI 使用双向游标；off 新会话复用既有账本和 reconciler，删除上一阶段新增且不再使用的 legacy reservation 实现，没有新增恢复表或自动切换生产模式。

## 原问题逐项结果

“已修复”指原触发路径已修改，并有下表所指的自动化或浏览器证据；不扩大为所有网络故障、真实支付及生产容量均已覆盖。

| 原编号 | 状态 | 修复与验证 |
| --- | --- | --- |
| F01 | 已修复 | 表格按业务 ID 选择；分页、筛选、排序、账号变化清选，刷新剔除消失记录；浏览器跨页无误选。 |
| F02 | 已修复 | keys/users 每次打开按 ID 重建编辑会话；详情未就绪禁用字段和保存，失败可重试，初始化后后台刷新不覆盖 dirty；延迟、重开、错误恢复回归通过。 |
| F03 | 已修复 | 先按时间及 ID 取有界主键，再短事务删除，单批最多 500；支持取消和批间让步，三库验证。 |
| F04 | 已修复 | 浮条排除页面全宽布局规则；实际 1000/600 高度均不遮分页，鼠标翻页通过。 |
| F05 | 已修复 | status[] 进入服务端 list/search/count 的共享条件，QueryKey 与筛选一致；跨页状态筛选通过。 |
| F06 | 已修复 | URL 为唯一已提交筛选，导航清理旧草稿和 debounce；浏览器后退及 3 个共享 hook 回归通过。 |
| F07 | 已修复 | 业务失败抛出统一错误，列表呈现错误和 Retry；真实业务失败注入不会伪装为空列表。 |
| F08 | 已修复 | 普通 4xx、取消、超时及业务失败不重试；429 尊重有限 Retry-After，网络/5xx 有限重试；浏览器 400 仅 1 次请求。 |
| F09 | 已修复 | Error 去重、message 优先、组件走集中处理；状态操作失败仅 1 条具体通知。 |
| F10 | 已修复 | 批量创建区分成功、业务失败、传输结果未知及未开始；停止后刷新成功项并显示统计，不自动重投未知项；数量须为 1..100 整数。 |
| F11 | 已修复 | 公共分页严格正整数、size 1..100、乘法溢出保护，兼容 page_size/ps/size；后端及非法 URL 回归通过。 |
| F12 | 已修复 | 入口仅同步英文 fallback，其他资源动态导入；当前可选语言仍为简中/英文/日文，历史 locale 映射保留；首屏资源、加载失败恢复和快速切换通过。 |
| F13 | 已修复 | 额度 Progress 增加本地化可访问名称、剩余及总额；类型、构建与浏览器检查通过。 |
| F14 | 已修复 | UI 改用加密双向游标，无 COUNT/OFFSET；按原排序、查看者/筛选/页大小和初始最大 ID 约束翻页。旧 API 保留 offset≤10000，计数以 10001 哨兵返回 total_capped，超限 400。三库及实际浏览器的前后翻页、历史/刷新、筛选、插入/删除和手机交互通过。 |
| F15 | 已修复 | 出站 JSON 去除重复 Reader/String/ReadAll 往返，保留重放及磁盘存储；1/10/50 MiB、身份/磁盘开关 12 组合通过，累计分配明显降低。 |
| F16 | 已修复 | OpenAI 非流式读取采用 N+1 字节限额，默认 64 MiB；渠道测试在 Writer 捕获时限制 256 KiB，超限判失败，debug 仅记长度。 |
| F17 | 已修复 | 全时段先读 MIN/MAX，再自适应最多 500 桶；SQLite/MySQL 5.7/PostgreSQL 9.6 的 600 天历史及首尾覆盖通过。 |
| F18 | 已修复 | profile 复用单 Query 缓存、signal 和 staleTime，失败显示 Retry；页内恢复通过。 |
| F19 | 已修复 | 默认交互请求 30 秒，后台/导出/上传分别 60/120/300 秒；显式 timeout（含 0）优先；32 秒故障场景仅 1 请求并出现 Retry。 |
| F20 | 已修复 | 总览同一渠道只聚合一次，内存派生榜单、稳定排序，名称只查一次；SQL 次数断言与本地量测通过。 |
| R-001 | 已修复 | Session 仅提供身份，最新 DB 快照决定角色/状态/分组；降权、封禁、删除及 DB 错误 fail closed，单 SQL 验证；TokenAuth 热路径不加查询。 |
| R-002 | 已修复 | 履约存储失败返回 Webhook 503，成功/重复/已持久 paid_review 才确认；签名事件故障后重投成功，不同 event 不重复入账。 |
| R-003 | 已修复 | 访问日志只记 EscapedPath，不改真实 query；SafeLogURL 与公共错误脱敏覆盖嵌套 URL、已知凭据及编码变体，协议兼容回归通过。 |
| R-004 | 已修复 | 按订单固化权益入账，校验 Session、币种、subtotal/discount/tax/total；允许优惠准确履约，非法回执持久记录 paid_review；三库事务/幂等验证通过。 |
| R-005 | 已修复 | pending/checkout_unknown 占名额，统一先锁用户再检查；终态释放，晚到付款重新检查，冲突进入 paid_review；8 并发最多 1 单、余额/管理员旁路拦截、三库回归通过。 |
| R-006 | 已修复 | 前端携带报价指纹，配置变化 409 重新报价；订单保存最小货币单位和权益，Checkout 复用同一总报价及幂等键；本地 Stripe SDK backend 核对参数和 Session 绑定。 |
| R-007 | 按产品契约保留 | 用户明确允许图片公开分享；保留匿名图片路由并在路由处说明，提交/任务 API 仍鉴权。匿名请求实际到达图片 handler；本轮未调用真实上游读取图片，不将公开分享定性为已确认越权。 |
| R-008 | 已修复 | 视频代理 URL、错误和出站调试日志统一净化；本地上游 503 场景确认渠道实际收到 key/签名，日志和返回错误不含凭据。 |
| R-009 | 已修复 | off 新请求的首次/追加预扣、结算、退款及任务终态复用现有持久账本，余额两腿原子提交，恢复已持久化意图和超时孤儿预扣。三模式×钱包/订阅×四场景，共 24 组独立进程崩溃/重启通过，重复恢复幂等；三库余额/任务回归通过。旧无账本历史差额不自动推算，默认 shadow 不变。 |

原 S01“时间桶除法/DECIMAL 扫描”是待验证观察，未计入 29 条。现在已在 MySQL 5.7.44、PostgreSQL 9.6.24 与 SQLite 实际执行趋势和总览，未复现该特定怀疑，无需以此为理由继续扩展架构。

## 二次审计发现及处理

复审覆盖本次差异及授权、支付履约/人工补偿/订阅名额、off 预扣、列表/编辑/错误恢复、统计 SQL、请求存储、响应捕获和语言资源等关联调用路径。

- PostgreSQL 统计正则中的 `?` 被 GORM Raw 当作 SQL 参数替换，导致总览失败。改用等价 `{0,1}`，补 600 天趋势中的 `frt=123.5` 均值断言，三库通过。
- 浏览器返回时筛选草稿可能被旧 debounce 再提交。抽成共用 hook，补历史恢复、导航取消与 IME 测试。
- 语言加载失败后 i18next 可能缓存失败状态，快速选择也可能被迟到资源覆盖。显式重试导入并用选择序号约束提交，最终浏览器验证通过。
- 核对旧 Stripe 管理员补单、通用订阅完成和 Session 绑定旁路：旧无快照订单禁止猜权益，新快照按固化权益补单，通用完成不能绕过 Stripe 回执校验，绑定采用 CAS。
- 基准补上本地 RoundTripper 调用、磁盘模式和 SHA256 重放断言，排除“未实际走转发”的无效性能样本。最终只引用 `*-local.log`。
- 本轮延伸审计发现异步任务已有持久账本时，shadow 仍可能落到兼容余额调整路径，off 接入后也会受影响。现在有账本即只由账本调整余额，统计归属保持原模式；钱包/订阅×三模式×成功/失败及 MJ 三模式退款测试验证没有重复扣退。
- 深游标量测发现 SQLite 对冗余 ID 上界和参数化 OR 的执行计划仍会扫描较多记录。self 去除被锚点包含的 snapshot 上界，admin 在 SQLite/PostgreSQL 用复合行比较，MySQL 5.7 用展开范围，沿用现有索引。修后真实三库的双向深位置量测与跨时间组排序回归通过，未以“没有 OFFSET”代替性能证据。

## 本轮收尾验证

本轮未新增表或迁移；此前完整迁移证据继续有效。以下 `repair-followup-*` 记录针对收尾后的实现，旧日志仅代表上一阶段结果。

| 验证 | 结果与证据文件 |
| --- | --- |
| 后端 | 最新 `go test ./... -count=1`、`go vet ./...`、Go 构建通过：`repair-followup-go-final.log`、`repair-followup-vet-final.log`、`repair-followup-go-build.log`。 |
| 崩溃恢复 | 24 组独立进程强退/重启，重复恢复幂等：`repair-followup-restart.log`；全仓最新运行也包含此测试。 |
| 三库 | MySQL 5.7.44、PostgreSQL 9.6.24 的 off/日志/任务模型及 service 回归通过：`repair-followup-{mysql57,postgres96}-{model,service}.log`；最终游标条件调整、跨时间组前后翻页与量测见 `repair-followup-{mysql57,postgres96}-cursor-final.log`。SQLite 同类回归包含在最新全仓测试。 |
| 前端 | 237 pass/0 fail、47 文件：`repair-followup-frontend-tests.log`；最新 TypeScript、构建、改动文件 ESLint/Prettier 通过：`repair-followup-typecheck-final.log`、`repair-followup-build-final.log`、`repair-followup-eslint-final.log`、`repair-followup-eslint-last.log`、`repair-followup-format-last.log`。 |
| 共享分页回归 | 20 路由×两宽度共 40 页面/视口，pageerror/溢出 0、桌面 axe 0：`repair-followup-pages/report.json`；原 9 项重点交互通过：`repair-followup-extra/report.json`。 |
| 最终浏览器 | 嵌入最终前端的 Go 构建，7 项全部通过：三页无重复、双向/历史/刷新、插入/删除锚点、筛选/页大小重置、self 隔离、非法游标恢复、旧深链接及手机布局、匿名图片路由；无页面异常及列表重定向：`repair-followup-cursor-browser/report.json`、`repair-followup-browser.log`。 |

## 上一阶段验证证据

以下文件均位于 [审计证据目录](../../../audit/source-review-20261017/)。故障注入日志中的预期数据库报错不代表测试失败，以测试最终结果为准。

| 验证 | 结果与证据文件 |
| --- | --- |
| Go 全仓 | `go test ./... -count=1` 通过：`repair-go-final.log`；最后新增初次订阅回滚测试后，七个相关核心包通过：`repair-final-core.log`。 |
| Go 静态检查 | `go vet ./...` 通过：`repair-vet-final.log`。 |
| 前端 | 237 pass / 0 fail，47 文件、489 断言：`repair-frontend-tests-final.log`。TypeScript、生产构建、改动文件 Prettier/ESLint 均通过：`repair-typecheck-final.log`、`repair-frontend-build-final.log`、`repair-format-final.log`、`repair-eslint-final.log`。 |
| 数据库 | SQLite 核心回归；真实 MySQL 5.7.44 与 PostgreSQL 9.6.24 的支付/名额/迁移/日志/统计矩阵通过：`repair-mysql57-tests-final.log`、`repair-postgres96-tests-final.log`；最新初次预扣回滚见 `repair-mysql57-initial-reservation.log`、`repair-postgres96-initial-reservation.log`。 |
| 完整迁移 | 三库各执行首次与重复 `migrate --up`，共 6 次 exit 0：`repair-migration-report.json`；旧列迁移不丢数据与 version=0 另有模型测试。 |
| 浏览器页面 | 20 路由 × 1440/390 两种宽度，共 40 页面/视口，无横向溢出及 pageerror；20 个桌面页面 axe 0 违规：`repair-browser/report.json`。 |
| 浏览器交互 | 9 项含浮条、详情延迟/重开/错误恢复、批量未知结果、整数校验、单通知、profile、30 秒超时：`repair-browser-extra/report.json`。 |
| 语言 | 7 个历史 locale 输入按现有三语言契约映射，资源失败可恢复、快速切换最后选择生效：`repair-browser-languages/report.json`。 |

检查使用隔离数据及本地模拟上游；没有调用生产数据库、真实 Stripe 账单/退款或付费上游。临时应用与数据库实例均已停止。上一阶段页面/语言/迁移证据对应当时构建；本轮重新运行共享分页的 40 页面及 9 项交互，最后的 7 项游标回归使用最终 Go/前端构建。

## 性能结果及解释边界

本机 1/10/50 MiB × 身份开关 × 磁盘开关的 12 组请求基准中，累计 B/op 减少约 54%–69%。50 MiB、无身份注入约 996.4 MB → 314.7 MB；有身份注入约 1206.1 MB → 524.4 MB。有效证据为 `repair-perf-before-local.log`、`repair-perf-after-local.log`。

JSON 编码的 sync.Pool 会暂留缓冲：50 MiB 身份场景单次 GC 后 heap 新约 152.5 MiB、旧约 112.7 MiB；连续两次 GC 后新约 102.5 MiB、旧约 112.7 MiB。因此不声称所有 heap 指标都下降。基准含 GC 和重放哈希，只是本机单次样本，不是生产 SLA。早期 `repair-perf-before.log/after.log` 缺少实际转发断言，已废弃，不用于结论。

上一阶段 SQLite 合成 10 万条日志：首页 p95 5.902 → 1.004 ms，offset=50000 p95 9.564 → 6.001 ms（各 21 样本）；总览 p95 558.670 → 433.982 ms（7 样本）。见 `repair-query-perf-before.log/after.log`。这是有界计数后的历史量测；当前旧 API 已拒绝该深 OFFSET。

本轮游标性能证据：10 万条日志、页大小 20、21 组每组 10 次读取，按每次均值计算分位数，避免 Windows 亚毫秒计时量化。下表是本机批次均值的 p95（ms），不是单次请求 p95 或生产 SLA。

| 数据库 | self 首页 / 第 5 万条附近 / 向前返回 | admin 首页 / 第 5 万条附近 / 向前返回 |
| --- | --- | --- |
| SQLite | 0.300 / 0.300 / 0.264 | 0.300 / 0.353 / 0.351 |
| MySQL 5.7.44 | 0.806 / 0.962 / 1.035 | 0.719 / 1.357 / 1.468 |
| PostgreSQL 9.6.24 | 0.589 / 0.551 / 0.552 | 0.498 / 0.717 / 0.707 |

证据：`repair-followup-query-perf-final.log`、`repair-followup-mysql57-cursor-final.log`、`repair-followup-postgres96-cursor-final.log`。self/admin 均保留原排序，不读取精确总数。未固定筛选的选择性、并发或缓存状态，不能据此计算跨阶段严格加速比或承诺所有筛选都恒定耗时。同期 SQLite 总览 p95 为 558.867 ms（7 样本），不宣称其比上一阶段继续提升；本轮未修改总览算法。

同一冷缓存场景的入口 index JS：解码 4,571,576 → 1,517,770 字节，编码传输 1,329,806 → 356,949 字节，约下降 67%/73%。这是入口文件大小，不能当作整个页面资源总量或 LCP/INP 改善值。当前只量测资源与导航时长，v2 原列的 LCP/INP 尚未量测。当前语言首屏只有 fallback 加所选语言资源，其他异步业务包仍可能较大。

## 保留事项与发布边界

1. MJ 图片允许公开分享，保留匿名访问；本次没有新增私有、到期或可撤销分享功能。
2. 日志旧 API 的 offset 上限为 10000；需要继续浏览的外部调用方应使用 `pagination=cursor` 和返回游标。UI 无任意跳页及精确总数；轮换密钥后旧链接需返回最新日志。ID 水位排除后续插入，不是跨请求的数据库事务快照。
3. off 新请求可恢复持久化意图和超时孤儿预扣；旧 off 无账本历史差额及未落盘的真实上游用量不能自动恢复。升级须排空旧 off 在途请求，off/shadow 的兼容统计仍不保证随余额一起崩溃恢复；生产模式未切换。
4. 旧 Stripe 无快照订单、paid_review、checkout_unknown 需要管理员核对渠道与历史权益。核实接口只读取真实 Session，退款和人工权益补偿仍走经核实的既有管理流程；本次不自动退款或猜算旧权益。
5. 真实 Stripe 测试账户的付款、优惠、税费及退款验收尚未执行；没有生产并发容量及 LCP/INP 数据。部署前应完成渠道验收并按现有发布流程处理迁移及配置边界。

本次实现与可执行回归已完成，29 条来源观察均有明确处理结果；发布验收边界如上，见 [修复任务](../../tasks/archive/2026-10-consolidated-audit-repair.md)。
