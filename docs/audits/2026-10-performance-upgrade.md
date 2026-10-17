# 后端性能与调用体验升级

2026-10-17；在 `main@32f0d2bf9a4e41b1564dce167d5a434c9129eaa3` 加既有未提交修复之上适配。保留之前工作，不包含新的前端 UI 改动；未提交、推送、发布或部署。

沿用前轮固定三仓 HEAD：NewAPI `3e2ce2c8856e32ca7054d673f8cd059ae466e80c`、Sub2API `b0a02a3304a0a78cdd003ddda1dd23ba02c03084`、CPA `911f49cbd49f4fe2516979754a7488a3fd14fe64`。本次实施前轮五项，并增加关闭上游响应的生命周期优化；没有把整个候选清单视为全部可移植。

## 实际实现

| 项目 | 实现与边界 | 参考 |
| --- | --- | --- |
| P1 批量深拷贝 | `common/copy.go` 为 RawMessage 使用 bytes.Clone；保留嵌套、显式零和双向修改隔离 | NewAPI [2cf177a](https://github.com/QuantumNous/new-api/commit/2cf177ac487e62c627c7d423b65735ba2481ef4f) |
| P2 JSON 无操作快返/局部重建 | reasoning 清理只拼接删除范围；桥接无额外工具/命名空间修改时保留已复制输入；异常/复杂输入回退原转换 | Sub2API [7e627318](https://github.com/Wei-Shaw/sub2api/commit/7e62731863c8f21ca32292e3923860e7f45221e9)、[61e1afdd](https://github.com/Wei-Shaw/sub2api/commit/61e1afdd499b37668fdcda374ea51aac7ec9ee3f)、[379db191](https://github.com/Wei-Shaw/sub2api/commit/379db1914158ec4329b2755c32c7c250998025b7) |
| P3 限流按需存储/空闲索引 | 按实际量追加，复用过期空间；清理只遍历空闲桶；保留权重/取消/等待语义，不引入每条请求的链表节点 | NewAPI [d3874db](https://github.com/QuantumNous/new-api/commit/d3874db61fce61b6c0f5c37523b21912a3f2658d) |
| P4 工具名索引 | Claude→Chat 已解析消息时建立请求内索引，完成遍历后回填结果；消除每条工具结果重扫历史 | NewAPI [946c730](https://github.com/QuantumNous/new-api/commit/946c730ed00806650fef509c74a868dda2847f7c)、CPA `8a6a396` |
| P5 H2 长流探活 | 每分片独立配置；正常静默生成不超时，失联 PING 无 ACK 才关闭；普通客户端与 HTTP/1 不开启 | Sub2API [81fd8530](https://github.com/Wei-Shaw/sub2api/commit/81fd8530056c6480df8e69770a85d8f411340290) |
| P6 取消后关闭响应 | 仅普通 HTTP attempt，先 cancel 打断 Read，随后安全关闭 body；幂等关闭，保留父请求/重试及 WS 复用顺序 | Sub2API [3fdd54ca](https://github.com/Wei-Shaw/sub2api/commit/3fdd54ca12433a53666f1a58de1ae6bcaa886b0f) |

未修改数据库、计费规则或首语义提交策略。探活默认 30s/15s，可通过 `.env.example` 中两个环境变量配置，read-idle=0 禁用。设计与关闭顺序见 [ADR-021](../decisions/021-relay-performance-and-stream-lifecycle.md)。

## 实测

以下是性能适配完成时的阶段结果。后续[模块协作复审](2026-10-integration-review.md)补充实际窗口元数据后，冷限流 key 当前为 168 B/op，不能继续把 152 作为最终当前值；其他阶段结果未在复审中重新测量。

Windows/amd64、AMD Ryzen 7 7840HS、Go1.25.1，三轮 `-benchtime=200ms` 中位数。before 来自本轮开始时保存的实际源码，以 Go overlay 替换对照；没有 reset 当前修复。合成请求，不包含真实上游推理或网络延迟。

| 阶段/负载 | 改前→改后 | 每次分配字节，改前→改后 |
| --- | --- | --- |
| 实际 Responses DTO，约 64 KiB Input 深拷贝 | 3.794ms → 0.0229ms | 141,842 → 76,880 |
| 实际 Responses DTO，约 1 MiB Input 深拷贝 | 59.686ms → 0.1164ms | 2,107,940 → 1,059,924 |
| Claude→Chat，10 轮工具交互 | 0.572ms → 0.122ms | 271,145 → 67,157 |
| Claude→Chat，100 轮工具交互 | 44.828ms → 1.169ms | 20,998,555 → 646,814 |
| Claude→Chat，1,000 轮工具交互 | 6.031s → 12.119ms | 2,044,599,272 → 6,499,264 |
| 上限 10,000 的冷 key，预留后取消 | 19.516µs → 0.155µs | 245,785 → 152 |
| reasoning 清理，1 MiB 无修改输入 | 3.744ms → 0.0213ms | 1,049,650 → 0 |
| reasoning 清理，1 MiB 输入删除一个成员 | 4.493ms → 3.566ms | 2,382,274 → 2,114,360 |
| 桥接准备，1 MiB 无修改输入，P1+P2 合计 | 61.535ms → 3.252ms | 5,803,674 → 1,060,539 |

隔离 P1 后，仅将桥接逻辑从原转换改为快返：4.740ms → 3.252ms、3,413,081 → 1,060,539 B/op。该独立对照与合计收益不能相加。

1 MiB 复制分配次数 1,048,634 → 30，1,000 轮工具转换 37,089,091 → 100,050。冷 key 的分配次数从 2 增至 4（增加桶和 LRU 节点），但字节大幅下降。已饱和单热 key 拒绝路径仍零分配，15.63ns → 16.63ns；没有宣称所有负载的 CPU 都下降。旧版 1,000 轮基准每轮仅一次，旧版大 Input 复制每轮样本也少，不能据此承诺线上倍数。

没有测量真实生产端到端吞吐、RSS、GC 或 P95/P99，没有把上述局部结果换算成整站提升；高负载实例的真实收益仍取决于请求规模、协议转换比例和本地/Redis 限流模式。

## 额外候选核对

除前轮 15 个性能 patch，本轮保存并检查了 7 个额外候选的相关生产片段。这里只对具体行为判定，不宣称逐行审完三仓全部源码。

| 来源 | 判定 | 本地证据/原因 |
| --- | --- | --- |
| Sub2API `3fdd54ca`，cancel before close | ADAPT | P6；`relay/channel/api_request.go` 原来没有 attempt 级 cancel-on-close；scanner 的内部取消不等于取消上游读取 |
| Sub2API `277aa141`，terminal 不等 EOF | ALREADY_PRESENT | `relay/channel/openai/relay_responses.go` 对 completed/incomplete 调 sr.Done，错误调 sr.Stop；不再增加第二套终态判断 |
| Sub2API `9be9c0b6`，失败释放账号会话槽 | NOT_APPLICABLE / ALREADY_PRESENT | 本地没有该 OAuth 账号 max_sessions 槽模型；令牌并发由 `middleware/token_limit.go` defer 释放，成功限流预留已有 Cancel 接口 |
| Sub2API `d0ed0eac`，Gemini 传输错误切换 | ALREADY_PRESENT | `relay/gemini_handler.go` 已返回统一 DoRequestFailed；`service/relay_failover.go` 允许切换并排除取消，本地另有冷却策略，不照搬账号临时摘除实现 |
| Sub2API `e652f6e2`，singleflight 内重查额度缓存 | NOT_APPLICABLE | 本地没有对应 ChannelMonitorQuotaFetcher；不为复用一个缓存修复引入新的额度监控模块 |
| CPA `1dffddf`，session 字段一次索引 | REFERENCE | 本地身份注入使用已解码 map，而非该多路径 GJSON 查找器；不引入第二个正文索引。工具查找的真实重复扫描已由 P4 处理 |
| NewAPI `219c9e0`，公共内容 ETag/匿名 session | DEFERRED | 上游补丁包含面板 API 客户端和会话提示配套；本轮排除前端，未把它作为推理热路径的确定收益项 |

## 验证记录

实现过程中定向包回归通过；复制含嵌套 map/slice、RawMessage 指针、nil/空值、显式零和双向隔离。reasoning 测试对照原解码器，覆盖重复键、转义键、嵌套删除、异常 UTF-8、超大数字、无效 JSON，且原始未修改数字精度保留。

- Go 全仓测试 `go test ./... -count=1`、`go vet ./...` 和 Windows 原生构建通过。
- `-race ./common ./service/openaicompat ./service ./relay/channel ./relay/helper` 通过；最终新增代理/UTF-8 用例及 controller WS 连续两轮/会话复用用例另跑 race，通过。
- reasoning 等价模糊测试 20s，执行 162,428 次，无不一致或崩溃。
- 真实 TLS/H2 测试验证健康静默生成、无 ACK 断连、两个 shard 的独立连接、HTTP/1、HTTP CONNECT 代理、缓存隔离/失效、禁用配置；HTTP 请求关闭验证上游取消且父请求可继续。

证据：`D:/Code/renewapi/audit/performance-upgrade-20261017/`，含 before 文件/hash、patch/manifest、overlay/复现脚本、三轮基准、fuzz/race/全仓测试/vet/build 日志。此轮无数据库改动，未重跑外部三库矩阵、前端或线上真实供应商验收，不复用旧结果冒充本轮验证。
