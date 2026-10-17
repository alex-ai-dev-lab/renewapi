# 后端性能与调用体验升级

2026-10-17，完成，尚未发布。用户确认采用上一轮四项性能优化及 H2 长流探活，并要求继续查三仓可适配的后端体验/性能优化；前端 UI 仍排除。

在 main 当前未提交修复之上最小适配，保留全部既有工作；不提交、推送或部署。已确认范围：P1 RawMessage 批量深拷贝、P2 JSON 无变化返回/局部重建、P3 限流按需分配与空闲清理（保留权重/预留取消）、P4 请求级工具查找索引、P5 H2 长流连接健康探测。额外候选先读固定提交、核对本地路径，已有能力不重复引入。

已实现 P1–P5，并适配 Sub2API `3fdd54ca` 的 HTTP attempt 取消后关闭响应，避免阻塞读取；WS 合成响应保留原关闭/复用顺序。额外 7 个候选逐项比对，已有终态即时结束、Gemini failover、并发释放不重复实现，不引入不对应的账号池/监控子系统。

最终验证通过：`go test ./... -count=1`、`go vet ./...`、`go build`；受影响 common/service/openaicompat/relay channel/helper 完整 race；最终新增 UTF-8/代理与 controller WS 复用/续接回归另跑 race。真实 TLS/H2 无 ACK/正常静默、HTTP1、分片、HTTP CONNECT 代理、缓存失效与禁用、关闭后父请求可重试均通过。reasoning 对照原解码器 fuzz 20s，执行 162,428 次，无不一致或崩溃。未重跑外部三库、前端或线上供应商验收，本轮无相应改动。

三轮实际源码 before overlay 对照：1 MiB Responses 复制 59.686ms→0.1164ms、1,000 轮 Claude 工具转换 6.031s→12.119ms、冷限流 key 分配 245,785→152 B/op；reasoning/桥接另有基准。冷 key 增加两个小分配，饱和热 key 15.63ns→16.63ns，不能声称所有场景 CPU 都下降。非端到端吞吐、RSS 或生产 P95 结论。

长期边界记 [ADR-021](../../docs/decisions/021-relay-performance-and-stream-lifecycle.md)，完整行为、基准、额外来源与验收见[实施报告](../../docs/audits/2026-10-performance-upgrade.md)。已更新 PROJECT_STATE、UPSTREAM_PORTS 与 CHANGELOG，不提升全量上游审计基线。

证据：`D:/Code/renewapi/audit/performance-upgrade-20261017/`。本轮开始前文件清单/哈希及关键文件副本在该目录；历史测量见上一轮 performance-review-20261017，不能作为本轮正式实现验收。
