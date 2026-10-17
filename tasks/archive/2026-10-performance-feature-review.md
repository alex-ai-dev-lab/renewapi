# 三个参考源的后端性能候选评估

2026-10-17，评估完成，尚未实施。用户将功能选型进一步聚焦到性能，排除 UI。

沿用既有固定三仓 HEAD，保存 15 个性能候选 patch，检查其关键路径与当前工作副本。确定优先候选：RawMessage 批量复制、无修改不重建 JSON、限流按需分配/空闲清理、请求级工具索引；H2 PING 为条件候选，现有统计聚合/连接池/WS 正文共享不重复引入。

使用外部 Go overlay 比较实际 Responses DTO 的当前 DeepCopy 与批量 RawMessage converter，三轮微基准及独立性检查通过。约 1 MiB 合成 Input 复制阶段中位数 59.374ms→0.116ms，分配 1,048,634→29；当前每轮样本仅 2 次，非端到端性能承诺。无业务源码改动，不提交或部署。

报告：[性能选型](../../docs/audits/2026-10-upstream-performance-opportunities.md)。证据、完整日志和校验：`D:/Code/renewapi/audit/performance-review-20261017/`。来源、文档和原修复文件哈希核对结果见 `verification.json`。
