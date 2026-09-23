# RenewAPI 当前项目状态

更新日期：2026-09-23。源码和验证状态以当前 Git 提交为准，本文不代表生产部署状态。

## 当前发布维护

可靠性升级已合并至主分支。随后实际浏览器 CI 暴露渠道集合路径缺少显式无尾斜杠
注册的问题；`347398928` 已修复，本地和 GitHub runner 的完整 88 项浏览器复测均通过，控制台错误为 0。

统一发布已经启用：成功构建只向 GitHub Releases 发布经过校验的 amd64、arm64 离线镜像、配置和 QA 证据，不再发布 GHCR。
已清理 74 个旧 Releases、366 个附件、702 条旧 Actions 运行和 GHCR 的 685 个历史版本，重新查询确认无旧记录遗漏。
已有工作分支已合并到 `main`，后续直接在主分支维护。验证与清理证据见
[发布维护归档](../tasks/archive/2026-09-release-pipeline.md)、[分发说明](release-distribution.md)
和 ADR-010。历史 Git tags、用户原有修改及生产数据保持不变。

## 产品与开发基线

- `VERSION` 当前为 `v1.0.0-rc.4`；发布维护已创建独立源码构建标签和 Releases，未修改产品版本、移动历史 tag 或部署生产。
- 可靠性升级原开发分支：`upgrade/2026-09-reliability`，已合入 `main`；后续直接在主分支维护。
- 分支基线：`4def3a8848e336532c55e9c3070cc78ead67e378`。
- 可靠性升级功能及复核修复提交截至 `a2321c2b1dae57a98c4ed18ec3197f97cd76c62d`；本次发布维护的路由修复和构建验证另见上方归档。
- 原 `agent/06-billing` 工作树及其用户未提交修改保持原样。
- 产品 tag 使用 `renewapi-` 前缀；原上游 `v*` tag 保持不变，历史 Releases、Actions 与 GHCR 已按本次明确授权清理。

## 本轮升级

- 独立请求级 Failover 状态，最多六个不同 Channel ID；复用已有 priority、weight、模型支持、Enabled 筛选和 route plan。
- 默认 15 秒首语义输出期限；提交前隔离 SSE 与响应头，空流、错误、超时或畸形响应可切换。内容提交后不拼接另一个渠道的流。
- 客户端与用户错误日志统一为公共 capacity 文案；管理员在 `Other.admin_info` 查看真实错误、逐渠道尝试链、用量和返回模型。前序失败独立进入渠道健康记录。
- 独立 Prompt Profile 表、管理员 CRUD、稳定渠道引用、默认回退、引用拒删、Anti-Poison 追加 nonce，以及默认关闭的“自动测试仅 AutoDisabled”模式。
- default/classic 两套前端同步 Prompt 管理、渠道设置、管理员错误链及原生 WS 开关；修复渠道模型选择器 light/dark 背景与自动展开。
- 补齐 off 原子退款、终态幂等、enforce 故障恢复和成功渠道用量归属；旧 Options 表增加实际主键迁移，保留定价选项。
- Responses WebSocket 独立于 Realtime，每轮重做鉴权、限流、路由和计费。支持 HTTP/SSE 桥接、可选原生上游、lane 隔离、取消、有限历史恢复、连接容量和关闭等待。
- 复核补齐终态用量估算、null 用量回退、HTTP/WS error 解析、错误扩展字段隔离及上游事件关联。

验收状态、22 项核心行为映射、命令和 CI 证据记录在 [本轮升级任务](../tasks/archive/2026-09-reliability-upgrade.md)。架构边界见 [ARCHITECTURE](ARCHITECTURE.md)、ADR-006 至 ADR-009。

## Billing 与历史任务

默认继续为 `BILLING_LEDGER_MODE=shadow`。旧审计描述的 shadow 异步退款问题在本轮基线已有持久化事务实现，因此未重复实现。off 仍不提供跨进程补偿保证；shadow/enforce 的切换条件见 [Billing enforce 说明](billing-ledger-enforce.md)。本轮不执行生产模式切换或生产数据库迁移。

历史 RU-A 缺少外部数据库与进程重启证据的状态已由本轮开发验证补齐：实际 MySQL 5.7/8.4、PostgreSQL 9.6/16 Actions，以及 SQLite 独立进程崩溃/恢复。证据以任务及 Billing 文档记录的提交为准。其余旧 UX/性能审计结论未因此自动变成已完成，仍按对应任务的历史证据边界处理。

## 上游审计边界

完整历史审计基线仍为 New API `58d4e9bd3bb035df8ea235dd682ccc8a45d0332a`（2026-08-14）。本轮是针对用户指定可靠性项目的专项复审，引用 New API `996adffe5165bd5e311e33a03a86b8aede1fe376`、Sub2API `20a94fbb567b62208751292ed7786b24a7e7c0fe`；不声称已分类两个上游的所有后续提交。判定与本地实现见 [UPSTREAM_PORTS](../UPSTREAM_PORTS.md)。

## 已知边界

- 两套前端继续维护。全量 lint/format 存量问题在任务中单独记录，不将改动文件检查通过写成全量通过。
- WS 的 `generate:false` 只预热本地输入，不预热上游模型；不支持 mid-turn steering，不跨客户端共享带会话状态的上游连接。
- `store=false` 的历史仅在当前连接中有限保存；断线或历史不可用时需要完整 input。已提交工具调用不会自动重放。
- 当前未运行生产凭证、生产数据库或生产部署验收；本地故障注入和 hosted 数据库验证不代表生产已切换。
- Go module 路径保持 `github.com/QuantumNous/new-api`，支持 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6，保留既有兼容、安全和 Anti-Poison 边界。
