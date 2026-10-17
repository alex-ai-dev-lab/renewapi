# 上游后端性能优化选型

日期：2026-10-17。用户将重点进一步收敛到性能；不含前端 UI，不引入新产品功能。本轮仅评估与隔离微基准，未修改业务代码。

沿用前两轮固定 HEAD：NewAPI `3e2ce2c8856e32ca7054d673f8cd059ae466e80c`，Sub2API `b0a02a3304a0a78cdd003ddda1dd23ba02c03084`，CPA `911f49cbd49f4fe2516979754a7488a3fd14fe64`。从三仓性能相关提交中保存 15 个候选 patch，定向核对下列生产路径；收集不代表逐行审完所有 patch。RenewAPI 以 `main@32f0d2bf9a4e41b1564dce167d5a434c9129eaa3` 加当前未提交修复为准。

## 最值得适配的四项

| 优先级 | 改动 | 本地源码证据 | 预期作用与边界 |
| --- | --- | --- | --- |
| P1 | RawMessage 批量深拷贝 | `common/copy.go` 使用反射 copier，尚无 RawMessage converter；Responses/Chat/Claude 等 Relay 调用 DeepCopy | 已有本地微基准，明显减少大输入复制 CPU 和分配；首选，改动集中 |
| P2 | JSON 未变更时直接返回，仅重建被修改片段 | `service/responses_reasoning_compat.go` 在无须修改时仍先解码整个 input，并递归构建 map/slice；工具桥接也解码并重写 input | 减少长历史、图片/base64、大工具结果的临时对象及 GC；需按各路径语义适配，收益尚未测 |
| P3 | 限流按实际使用分配，按空闲顺序清理 | `common/rate-limit.go` 新 key 按 maxRequestNum 分配完整容量，清理持全局锁扫描全部 key | 高限额、多 key 时降低内存占用和清理锁等待；未启用相应本地限流路径时收益有限 |
| P4 | 工具名/调用身份每请求只建一次索引 | `service/convert.go` 每个无 name 的 tool_result 调用 `dto/claude.go` 的 SearchToolNameByToolCallId，再扫描历史并 ParseContent | 长工具会话可减少重复解析，避免工具结果数与历史长度相乘；短普通对话收益小 |

### P1：已测的直接移植候选

参考 NewAPI [2cf177ac](https://github.com/QuantumNous/new-api/commit/2cf177ac487e62c627c7d423b65735ba2481ef4f) 的 `common/copy.go`。在原 copier 中为 json.RawMessage 配置 bytes.Clone 类型转换，不再逐字节反射，仍保留独立存储以隔离重试/参数修改。

本地测试通过 Go overlay 注入外部测试文件，不改变 `common/copy.go`；比较当前 `common.DeepCopy` 与具有该 converter 的候选函数，输入使用实际 `dto.OpenAIResponsesRequest` 类型和合成 ASCII 长文本 Input。两条路径使用同一份源请求。三轮、每项 `-benchtime=100ms`，下表为中位数：

| Input 规模 | 当前耗时 | 候选耗时 | 当前分配次数 | 候选分配次数 | 每次分配字节，当前→候选 |
| --- | --- | --- | --- | --- | --- |
| 约 64 KiB | 3.849 ms | 0.0240 ms | 65,594 | 29 | 141,843 → 76,864 |
| 约 1 MiB | 59.374 ms | 0.116 ms | 1,048,634 | 29 | 2,107,944 → 1,059,906 |

环境：Windows/amd64，AMD Ryzen 7 7840HS，本地 Go1.25.1。约 1 MiB 的当前路径每轮只运行 2 次，样本量较小；这些是复制阶段的微基准，不是实际请求端到端延迟，也不能将约 500 倍的阶段差异宣传为网关吞吐提升。三轮日志和测试源均保留；小型独立性检查通过。

实际调用点包括 `relay/responses_handler.go`、`relay/compatible_handler.go`、`relay/claude_handler.go` 和 `service/openaicompat/responses_bridge_tools.go`；同一请求若多处复制，优化可能多处受益。不能简单改成共享可变字节。

实施验收：嵌套 RawMessage、nil/空值、显式零和指针语义、map/slice 及源/副本双向修改隔离；针对原生 Responses、跨协议和失败重试分别量测分配、CPU、GC 及 P95。此次小型检查不能替代这些回归。

### P2：大 JSON 少解析、少重写

参考 Sub2API [大 Responses 输入重写](https://github.com/Wei-Shaw/sub2api/commit/7e62731863c8f21ca32292e3923860e7f45221e9)、[延迟重建 namespace input](https://github.com/Wei-Shaw/sub2api/commit/61e1afdd499b37668fdcda374ea51aac7ec9ee3f)、[schema 多处修改一次拼接](https://github.com/Wei-Shaw/sub2api/commit/379db1914158ec4329b2755c32c7c250998025b7)。已核对其原始片段保留、首个改动才分配和修改偏移集中拼接的实现。

最小适配从 `sanitizeResponsesReasoningContentJSON` 开始：结构化检查是否真有待清理的 reasoning 内容，无改动返回原字节；有改动只处理相关项目。保留本地当前清理范围，不引入上游额外删除规则。后续再优化工具桥接中不受命名空间改写影响的图片、消息和工具输出。原生无需桥接的流量不应为这些功能承担全量转换。

字段模式、重复 JSON key、无效 UTF-8、转义、数字精度和索引有效性决定快路径是否安全；不能只用字符串包含判断作最终语义。上游部分只读视图涉及 unsafe，本地应先采用可证明所有权的安全路径，不为追求零拷贝直接移入 unsafe。需要 buffer 跨请求/跨 goroutine 保留时仍须独立所有权。

本地参数覆盖已使用 byte-based gjson/sjson，不能将“参数覆盖改为 bytes”再列为新优化。这里的增量是无操作不重建、多个独立改动避免反复复制同一大文档；不能任意合并有顺序依赖的覆盖规则。

验收：无改动、单个/多个改动、长图片和工具结果、所有上述边界、重试隔离；对 1/10/50 MiB 合成请求分别比较 B/op、allocs/op、耗时和峰值存活内存。目前仅静态确认适配价值，没有量化收益。

### P3：限流器分配和清理

参考 NewAPI [d3874db6](https://github.com/QuantumNous/new-api/commit/d3874db61fce61b6c0f5c37523b21912a3f2658d)，已读按实际接受请求追加、LRU 空闲清理路径。

本地 `rateLimitEntry` 在 amd64 上由 int64 与 string header 组成，单项约 24 字节；配置上限 10,000 时，冷 key 即使只来一个请求也申请约 240,000 字节元素容量，未计分配器取整和 map 开销。这是数据布局推算，不是生产内存量测。当前清理还在同一把锁下扫描所有 key。

实施时保留 RenewAPI 的 Reserve/Cancel、RequestN 权重和 RetryAfter；上游无相同契约，不能整文件替换。优先选择按实际流量增长的有界存储与空闲索引，再以有/无取消的负载比较环形数组和链式结构。上游链表每条请求也有分配，不能未经压测就认定所有 QPS 场景都会更快；若未来分片也需单独量测锁竞争。

验收包括窗口边界、取消与重试、权重原子性，以及高限额低活动、高活动热 key、大量过期 key 的内存/锁等待。Redis 模式与本地限流具体调用点须分别确认覆盖，不能声称所有线上限流都因此提速。

### P4：工具查找索引

参考 NewAPI [946c730e](https://github.com/QuantumNous/new-api/commit/946c730ed00806650fef509c74a868dda2847f7c)，以及 CPA [8a6a396：请求级工具索引及验证缓存](https://github.com/router-for-me/CLIProxyAPI/commit/8a6a396)。已读前者用一次遍历收集 id→name 再回填结果，后者用请求级 map 处理身份查询，避免逐事件重扫工具定义。

本地可以直接在现有 Claude→Chat 转换中建立索引，保留首次命中、显式 name、异常/缺失 ID 和引用顺序的现有行为。不建全局工具名缓存，避免跨请求污染；其他协议路径已经有 NamespaceTools map，不为其再加重复层。

验收：大量历史/工具结果、重复/未知 ID、乱序历史、显式 name 和命名空间冲突；比较 10/100/1,000 次工具交互的时间曲线。当前只确认了重复扫描路径，没有宣称达到某个固定提速比例。

## 条件候选及排除项

- **HTTP/2 长流 PING 探活**：Sub2API [81fd8530](https://github.com/Wei-Shaw/sub2api/commit/81fd8530056c6480df8e69770a85d8f411340290) 在传输层设置 ReadIdleTimeout/PingTimeout。RenewAPI 已有连接缓存、HTTP/2 和分片 Transport，但未看到相应探活配置。它可能缩短代理/NAT 静默断连接造成的长尾等待，属于尾延迟与可靠性优化；正常网络下不保证提高吞吐。只在长流 profile 配置并做静默丢包、代理兼容及合法静默生成测试，不直接全局照抄 10s/5s。
- **统计聚合**：Sub2API 单次统计扫描和汇总表可以参考，但 RenewAPI 本轮修复已合并概览统计、复用渠道聚合结果、限制日志深分页。不能将这些再次列为尚未实施的性能更新。后续汇总表是否值得引入要先看真实 SQL 执行计划及数据量。
- **WebSocket 历史共享**：Sub2API `ca9b4d73` 解决其重复正文复制。RenewAPI 的 history.remember 已复制 slice 头并共享 RawMessage 正文；并非相同的全部正文重复复制问题。prepare 中仍有解析/序列化成本，可归入 P2，不能直接承诺移植就消除本地 O(T²)。
- **CPA 减少 SSE 临时拷贝、重复请求转换**：保存了 `a26cf2a`、`639b7f1` 等候选。RenewAPI 转换结构不同，而且流缓冲的跨 goroutine 所有权要求不同，尚无可直接对应的双重转换路径证据，不作为首批确定收益项。异步交付仍需保留必要拷贝。
- **插件运行时与 Devin 专用 Transport 优化**：对应子系统不同，现有 HTTP 客户端已经缓存；不为获得优化而引入另一套运行时。

建议实施顺序：P1 → P4 → P2；P3 依据本地限流使用情况推进。仅当存在静默断连长尾时再引入 H2 探活。实施后以端到端吞吐、CPU、RSS、GC 和 P95/P99 判断是否达标，不累加微基准倍数推算整站收益。

证据位于 `D:/Code/renewapi/audit/performance-review-20261017/`：候选 patch、`candidates.json`、overlay 测试与 `copy-bench.log`。本轮只跑该微基准及独立性检查；无业务改动，不重跑全仓测试。来源与文档校验见同目录 `verification.json`，不推进完整上游历史审计基线。
