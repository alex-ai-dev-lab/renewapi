# 性能升级与既有修复的整体协作复审

2026-10-17，完成本轮源码复审、修复及本地验证。基于 main 及全部既有未提交修复，未提交、推送或部署。

沿入口→请求复制/转换→HTTP/WS/长流→重试/结算→并发与模型限流复核，另检查权限、Stripe、视频代理、日志游标和前端 API 契约。不宣称人工逐行审完仓库全部文件。

确认并修复 4 项既有运行时缺口：默认客户端 Reset 并发竞态及晚发布、共享限流器清理长窗口活预算、Redis 回退请求释放其他请求的并发槽、超过两分钟的长请求丢失并发预算。另修复 MiniMax 并行测试改写 Gin 全局模式的竞态。复用客户端缓存与 TokenRequestLimit，删除旧的无身份释放/汇总标记，实现请求持有的幂等槽位与有界续租；没有增加数据库表或第二套调度。

修复前失败证据、源码备份、全仓 test/vet/build、common/service/middleware/relay/controller race、HTTP 在途重建和并发槽最新定向 race 均保存在 `D:/Code/renewapi/audit/integration-review-20261017/`。前端 237 测试、类型、构建、改动文件 lint 通过。全量前端 lint 的 31 errors/19 warnings 属于相对 HEAD 未修改的 30 个文件，已单独登记，没有改规则掩盖。

冷 key 当前 168 B/op，保留原大幅降内存成果；热 key 和缓存命中零分配。Redis 槽使用 v2 键，发布必须排空旧在途请求；故障回退仅实例内保护。没有重跑真实 Redis 集群、外部三库、浏览器或真实供应商验收。其他待适配上游功能和依赖状态保持原记录。

详细覆盖、量测与限制见[复审报告](../../docs/audits/2026-10-integration-review.md)，长期约束见 [ADR-021](../../docs/decisions/021-relay-performance-and-stream-lifecycle.md)。本轮最终 hash 差异清单用于确认既有修复保留。
