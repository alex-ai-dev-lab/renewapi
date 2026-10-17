# 性能升级与既有修复的模块协作复审

2026-10-17；基于 `main@32f0d2bf9a4e41b1564dce167d5a434c9129eaa3` 和本轮开始时全部未提交修复。按实际调用链审查并修复确认问题，未提交、推送、发布或部署。

## 结果

本轮确认并修复 4 项运行时缺口，另修复 1 项测试竞态。这些触发条件在此前实现中已存在，不能归因为上一轮性能适配新引入。所覆盖路径未发现其他已确认的功能回归；这不等于人工逐行审完全部仓库或保证不存在缺陷。

| 问题 | 触发与影响 | 修复及复现证据 |
| --- | --- | --- |
| 客户端重建与读取竞争 | 凭据刷新等操作 Reset 时，无锁读取默认客户端会竞争；锁外创建的缓存 miss 可能在重建后发布旧客户端 | 普通和 SSRF 缓存共用一个内部入口；创建、发布、重建同锁，重建不暴露 nil。`client-race-before.log` 有真实 DATA RACE，`client-race-after.log` 连续三轮通过 |
| 空闲清理提前释放长窗口预算 | 共享限流器先按较短周期初始化，另一路使用更长窗口；清理可能删除仍有效的预算 | 桶保存实际请求窗口；清理同时检查活动时间和最后一条记录的到期时间，跳过长窗口桶继续清理后面的短窗口。`limiter-before.log` 失败复现，修复后核心及 race 测试通过 |
| Redis 回退时释放错配 | 同 token 同时存在 Redis 和内存回退请求，完成顺序不同；按 token 汇总的标记不能确定请求实际持有哪个槽位 | 改为请求持有槽位及幂等 Release，捕获实际 Redis 客户端/唯一成员；移除旧汇总标记和无身份的释放接口。`concurrency-before.log` 复现释放了另一个请求的 Redis 预算 |
| 长请求超过两分钟失去并发保护 | 原 Redis 计数键固定 TTL 到期，即使请求仍活跃也允许新请求；旧请求之后释放还可能减掉新计数 | 同一限流模块使用请求级 ZSET 租约，Redis 时间计时、30 秒续租、两分钟租期；只移除本成员，不复活已丢失成员。续租丢失或持续失败时取消请求；停止 worker 后再释放。原过期放行见 `concurrency-before.log` |
| MiniMax 并行测试修改 Gin 全局模式 | 一个并行测试 SetMode，另一个 gin.New 读取模式，导致扩大 race 检查失败 | 删除无需改写全局状态的一行，保留并行测试及断言。`race-all-affected.log` 保存原始失败，`minimax-race-after.log` 连续十轮通过 |

HTTP 缓存构造不进行网络 IO，放入同一临界区避免晚发布和重复建池，没有另加缓存或配置代数。Reset 只关闭空闲连接：真实 HTTP/1、分片 H2 长流、SSRF H2 响应均测试了读取途中 Reset，原响应完成且后续 lookup 获得新客户端。

并发槽继续由现有 `TokenRequestLimit` 中间件获取和 defer 释放，槽位上下文传递给下游请求。只有 Redis 模式且配置了并发上限的在途请求有续租 worker，每 30 秒增加一次 Redis 脚本调用；内存模式和未配置上限的请求没有该 worker。单次续租受 Redis 操作期限及 15 秒上界约束；连续失败约 90 秒取消请求，保留距离租期到期的余量。没有引入第二套调度器、数据库表或计费实现。

## 调用链覆盖

| 路径 | 核对重点 |
| --- | --- |
| 请求复制、Responses reasoning、Responses 桥接、Claude 工具转换 | 深拷贝隔离、无修改快返、异常输入回退、显式零、首次匹配/前向工具引用；沿用既有兼容转换，不扩大原生透传 |
| 渠道请求、HTTP 客户端、protected fetch、分片 Transport | profile/代理/TLS/SSRF 缓存隔离、渠道失效、重建、连接复用；构造函数和 CloseIdleConnections 没有反向获取客户端锁 |
| StreamScanner、Claude pause-turn、Responses WS | attempt cancel 与父请求分离；阻塞读取退出、关闭和 worker 等待顺序；WS 合成响应继续使用专用复用顺序 |
| Controller relay、BillingSession、任务结算与并发/模型限流 | 重试不污染下一尝试，提交后不拼接另一上游；结算/退款幂等；失败预留取消；中间件正常、取消、panic 均释放并发槽 |
| Session 授权、Stripe 快照/核实、视频代理、日志查询/游标 | 权限白名单、支付状态与快照匹配、错误脱敏、分页边界、游标 viewer/filter 绑定及 self scope |
| 前端 API、错误处理、日志游标和 URL 状态 | 账号作用域、请求/响应契约、筛选及分页重置；本轮无前端源码改动 |

## 验证

证据目录：`D:/Code/renewapi/audit/integration-review-20261017/`。

- `go test ./... -count=1`：`go-test-all-final.log`，通过。最后增加的取消释放及终态状态断言另由最新定向 race 覆盖。
- `go test -race ./common ./service/... ./middleware ./relay/... ./controller -count=1`：`race-final.log`，通过，覆盖 WS 续接/复用、计费和渠道/流处理现有用例。
- 最新并发槽及中间件生命周期定向 race 连续三轮：`concurrency-lifecycle-final.log`，通过；包括 Redis/内存混用、重复释放、原客户端释放、跨两分钟续租、旧租约替换、真实续租 worker 丢失取消、父请求取消清理，以及 handler 的正常/取消/panic 退出。
- `go vet ./...`、原生 Go 构建：`go-vet-final.log`、`go-build-final.log`，通过。
- 前端：`frontend-tests.log` 为 237 pass / 0 fail；`frontend-typecheck.log`、`frontend-build.log`、`frontend-changed-lint.log` 均通过。
- 扩展执行的全量前端 lint **未通过**：31 errors、19 warnings，涉及 30 个文件；这些文件、ESLint 配置和依赖清单相对 HEAD 未修改，见 `frontend-lint.log`、`frontend-lint-file-state.json`。这是既有静态检查债务，不能把“改动文件 lint 通过”写成全量通过。

## 性能与发布边界

本轮保存源码 overlay 比较，没有 reset 或覆盖既有工作。长窗口元数据使冷 key 预留/取消从上一轮 152 增至 **168 B/op**，仍为 4 次分配；相对最初 245,785 B/op 的按上限预分配，节省仍保留。饱和热 key 和 HTTP 客户端缓存命中继续零分配。默认客户端读取增加同步是修复竞态的必要开销，不能把未同步读取的并行速度作为正确性等价基准。具体样本见 `benchmark-*.log`；不是整站吞吐或生产 P95。

其余检查完成后，单独顺序运行 before/after、CPU=1、三次 300ms 基准，中位数：冷 key 200.5→199.7 ns/op，饱和热 key 16.47→16.63 ns/op，默认客户端 lookup 23.72→26.44 ns/op，长流客户端 lookup 52.57→56.53 ns/op。后两项同步开销约 3–4ns；样本只用于检查局部开销，没有统计显著性或端到端提升结论。

Redis 新槽位使用 `token_limit:concurrency:v2:`，避免与旧字符串计数键类型冲突。发布时排空旧进程的在途请求再切换，不能在混用两种槽位协议的滚动窗口内声称严格共享并发上限。Redis 故障时仍沿用原有内存回退策略，仅提供实例内保护；网络分区期间不承诺精确的跨实例全局上限。续租失败后取消长请求是本次明确的保护行为。

本轮未修改数据库、支付契约或前端界面，未重跑外部三库矩阵、浏览器全页面或真实供应商验收；Redis 租约测试使用 miniredis，没有冒充真实 Redis 集群/故障转移验收。上一轮模糊测试和性能结果保留在原报告，不作为本轮重跑结果。已登记的其他上游功能及依赖升级仍以 UPSTREAM_PORTS.md 的待适配状态为准。

设计约束见 [ADR-021](../decisions/021-relay-performance-and-stream-lifecycle.md)。本轮 hash 差异另存于证据目录，确认原修复保留且只有预期增量。
