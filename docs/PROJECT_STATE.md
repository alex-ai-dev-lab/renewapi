# RenewAPI 当前项目状态

更新日期：2026-10-17。源码状态以当前 Git 提交为准，生产镜像与验收范围以对应部署归档为准。

## v1.5.0 已发布上线

正式 [v1.5.0](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.5.0) 已发布并上线，产品源码 `7cd988595a317c7c79c1c551273a17853f7939d6`。[主分支](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/39020424377)及[正式标签](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/39021051946)完整门禁通过，双架构镜像及 QA 附件已发布。首轮 main 的 Codex 测试替身拦截客户端不匹配，已修正后重新通过门禁，未放宽断言。服务器代理下载、摘要/镜像身份验证、SQLite 加列迁移、独立 Compose 切换及清理完成；候选与公网 14 项真实 API 验收、最长 102.55 秒长流及 495 轮持续监测/19 次切换探测通过。异常、验证边界和清理清单见[发布归档](../tasks/archive/2026-10-deploy-v1.5.0.md)。

## 模块协作复审（随 v1.5.0 已发布）

复审修复 4 项既有运行时缺口：HTTP 客户端重建竞态、长窗口预算提前清理、Redis/内存并发槽释放错配、两分钟长请求并发预算过期；另消除 MiniMax 测试竞态。普通/分片 H2/SSRF 在途响应重建测试、槽位续租/取消/终态释放、全仓 Go 测试/vet/build 及 common/service/middleware/relay/controller race 通过；前端 237 测试、类型、构建和改动文件 lint 通过。全量前端 lint 仍有未修改文件的 31 errors/19 warnings。冷限流 key 当前为 168 B/op，原性能报告中的 152 是上一阶段结果。Redis 新槽位协议升级需排空旧在途请求；发布门禁已补齐四版本外部数据库验证，生产 SQLite 实例已验收，但生产未配置 Redis，本轮未验收真实 Redis 集群。范围与阶段证据见[复审报告](audits/2026-10-integration-review.md)、[任务归档](../tasks/archive/2026-10-integration-review.md)及 ADR-021，发布验收见上方归档。

## 后端性能与调用体验升级（随 v1.5.0 已发布）

已实施 RawMessage 批量独立复制、JSON 无变化快返/局部重建、限流按需存储/LRU 清理、请求内工具索引、独立 HTTP 长流 H2 探活，并新增 HTTP attempt 取消后关闭响应，保留父请求重试及 WS 复用。三轮局部基准：1 MiB 复制 59.686ms→0.1164ms，1,000 轮工具历史转换 6.031s→12.119ms，冷限流 key 当阶段分配 245,785→152 B/op（后续复审为 168）；不是端到端吞吐或生产延迟。全仓 Go 测试、vet、构建、受影响路径 race 和 reasoning 等价 fuzz 通过。实际行为、额外 7 个候选处置与测试边界见[实施报告](audits/2026-10-performance-upgrade.md)、[任务归档](../tasks/archive/2026-10-performance-upgrade.md)，长期约束见 [ADR-021](decisions/021-relay-performance-and-stream-lifecycle.md)。已随 v1.5.0 提交、发布及部署，既有修复全部保留。

## 三个参考源后端功能选型（未实施）

按用户纠正，本轮独立评估可引入的后端新功能，排除前端 UI。保存 24 个功能候选 patch，核对入选生产路径与本地覆盖，提出工具搜索/延迟加载、额度窗口调度、正则条件、Vertex 模型发现、Gemini 显式缓存、聚合站余额、Mantle、搜索引用转换 8 项候选。优先小功能，再实施工具与调度；已有 OAuth、粘连、WS、压缩及 metadata 同步不重复引入。详见[功能评估](audits/2026-10-backend-feature-opportunities.md)和[任务归档](../tasks/archive/2026-10-backend-feature-review.md)。仅更新文档，尚无新功能代码、提交或发布；下方缺陷检查保持为独立材料。

## 三个参考源更新检查（待适配）

已固定 NewAPI / Sub2API / CPA 的 10-17 参考 HEAD，列举近期 291 条非合并提交、保存 49 个候选 patch，形成 13 组待适配行为和一组依赖安全升级建议。已复现 Playground 跨账号本地数据、缓存用量、分组限流、旧成功清冷却、混合流正文及图片/工具转换缺口；Go1.25.1、Linux 构建配置下 govulncheck 有 43 个去重符号可达告警。优先统一升级 Go/相关依赖，再按现有组件最小修复，详见[更新检查报告](audits/2026-10-upstream-update-review.md)与 [处置表](../UPSTREAM_PORTS.md)。本轮只检查并记录候选，没有导入、提交或发布；原全量上游审计基线和上一轮修复验收记录保持不变。

## 合并审计修复（随 v1.5.0 已发布）

在独立工作副本 `repair-20261017` 完成合并修复及尾项复审：28 条已修复验证，MJ 图片按用户确认的公开分享契约保留，没有部分解决或待产品决定项。日志 UI 使用双向游标，旧 API 限制深 OFFSET；off 新请求复用既有持久账本并修复任务重复扣退，默认仍为 shadow。Go 全仓、前端 237 测试、真实三库矩阵/既有完整迁移、24 组独立进程崩溃重启、40 页面/视口及最终 7 项游标浏览器回归通过。逐项结果与量测边界见[复审报告](audits/2026-10-consolidated-repair.md)，设计见 [ADR-020](decisions/020-audit-repair-boundaries.md)，交接见 [修复归档](../tasks/archive/2026-10-consolidated-audit-repair.md)。已随 v1.5.0 提交、发布及部署；旧 off 无账本差额不猜测回填。未执行真实 Stripe 扣款或退款，真实商户验收仍为独立验证边界。

## 长流约 60 秒断开修复

用户提供的请求实际已有大量有效输出，59.902 秒下游取消；日志公共 model_capacity 文案掩盖了原因。容器直连 78.24 秒完成，Caddy 本机和公网入口均约 60 秒截断。已按实际 Caddy 2.11.6 源码核实请求体读取空闲期限在 EOF 后残留的问题，并仅对 :1443 设置 read_body_idle -1s，热加载后公网 Responses 81.54 秒、Claude 74.05 秒均收到协议终态并成功计费；临时配置备份已删除。应用镜像无需变更；配置与长流验收规则见 [ADR-019](decisions/019-caddy-long-stream-deadline.md)、[反代说明](reverse-proxy.md) 和[排障任务](../tasks/archive/2026-10-long-stream-60s.md)。上游真实并发限制、余额不足和首输出超时仍需分别判断。

## v1.4.2 工具流与 API 转换修复

已发布并上线 [v1.4.2](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.4.2)，产品源码 `687aa8e2acee4ded9b39bc55237a7f3292e4097f`。修复工具语义空输出误判、混合文本/工具丢失、工具聚合身份与终态参数、SSE 多行解析、命名空间转换和跨渠道尝试污染；缓存参数保留，无法等价转换的参数明确拒绝。GitHub 固定 SHA 参考与边界见 ADR-018、UPSTREAM_PORTS.md 和[修复归档](../tasks/archive/2026-10-custom-tool-stream.md)。

本地全仓测试/构建、前端 231 测试及正式标签完整 Actions 门禁通过。三端点真实鉴权矩阵 42 组/84 次成功请求；候选与公网入口 14 项真实 API 验收通过，原故障自定义工具流正常。服务器代理下载、摘要验证、独立 Compose 蓝绿切换完成，89 次准备/切换探测未见失败；healthy、重启 0、数据库及渠道/端点配置校验通过。本次备份、下载包和退役容器已清理；旧进程在连接归零后停止超时，被 Docker 终止，详见[发布归档](../tasks/archive/2026-10-deploy-v1.4.2.md)。

待诊断：同提交主分支 CI 的 keepalive_stall 故障注入两次提前遇到连接关闭，备用渠道恢复成功但等待时长断言失败；正式标签和本地连续三轮通过。未放宽断言，失败证据已保留，不能声称全部 CI 运行成功。

## v1.4.1 Codex 请求规范与可解释错误

已发布并上线 [v1.4.1](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.4.1)，产品源码 `08b2b35f12b586872f150e11547f3dd0ccd3d0b4`。新增独立渠道 Codex 规范与最终参数覆盖，保留 Claude 扩展/工具失败语义，模型和本地额度等错误保留明确分类；恢复厂商模型图标与 OpenAI/Claude 优先顺序，包含上一轮 UI 可访问性修复。
自动缓存键修正为符合上游限制的 64 字符完整哈希。主分支及正式版完整 Actions 门禁、实际 Chat/Claude 请求、明确模型错误和页面检查通过。
准备阶段部署脚本误复用 Compose project，造成约 3 分半中断，已恢复并修正；最终切换阶段 68 次探测无失败。服务器备份和主要中间文件已清理，最后约 46 KB 本地缓存/调试文件删除被自动审批拦截。规则见 [ADR-017](decisions/017-codex-request-profile-and-public-errors.md)，实现见[功能归档](../tasks/archive/2026-10-codex-normalization-and-channel-icons.md)，部署、异常及清理详见[发布归档](../tasks/archive/2026-10-deploy-v1.4.1.md)。

UI 第二轮复查修复已随本版部署，覆盖成功率对比度、复制按钮、分组设置可访问名称和兑换码文案，见[复查归档](../tasks/archive/2026-10-ui-review-round2.md)。

## v1.3.0 三端点整流

已发布并上线 [v1.3.0](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.3.0)，产品源码 `ee77092d8d66a92168fe9c81d5de80d06ba2bac4`。三种文本协议按模型默认端点互转，工具、图片/思考、跨尝试状态及路由预览完成验证；生产已启用原 113 条规则的整流模式。完整 Actions 门禁与真实长工具请求通过，部署记录见 [发布归档](../tasks/archive/2026-10-deploy-v1.3.0.md)，长期边界见 [ADR-016](decisions/016-model-default-protocol-normalization.md)。

## v1.2.0 UI 审计修复与 classic 退役

已修复语言标记、预设与语义色、默认密度、输入边界、强制颜色焦点、通知主题和图表非颜色区分；按用户授权移除 classic 源码/依赖/构建分支及确认无引用的依赖，旧配置和常用地址保持兼容。
正式 [v1.2.0](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.2.0) 已发布并部署，源码 `453ed0b501a562a1c46985aae584a88d0c816cc7`。完整 Actions 门禁通过；实现及验收见 [审计归档](../tasks/archive/2026-10-ui-accessibility-audit.md)、[部署记录](../tasks/archive/2026-10-deploy-v1.2.0.md)，长期约束见 [ADR-015](decisions/015-default-frontend-and-semantic-colors.md)。

## v1.1.3 状态文字对比度

状态和分类标签使用独立明暗文字色，图表配色不再直接作为标签文字；主内容区与 Portal 规则一致。浅色图表 3/4/5 和深色控制台灰阶提高可见性。
正式 [v1.1.3](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.1.3) 已发布并部署，源码 `3cc7d0c233f95d7fb2e32ec3bb28ec3caf45b725`。完整对比度矩阵最低文字 4.77:1、图表 3.19:1，218 单测和 27 项隔离浏览器回归通过，发布门禁新增颜色检查。详见 [修复记录](../tasks/archive/2026-10-status-text-contrast.md) 和 [部署记录](../tasks/archive/2026-10-deploy-v1.1.3.md)。

## v1.1.2 功能与体验修复

已针对生产实测修复字段适配、密码与安全设置、管理员合规读取、导航分类、首屏/异步加载与公开页面样式。
额外真实后端回归发现本人资料更新可能覆盖账户无关字段，现改为白名单部分更新，并验证密码改变后身份、角色、分组和余额不变。
审查后补齐模型缺字段/空分组状态、模型目录与分类设置一致性、套餐分块失败关闭与恢复、导航 Logo 回退，并收紧浏览器测试环境隔离及全页面错误收集。
正式 [v1.1.2](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.1.2) 已发布并部署，源码 `12b129886c9bcfb3578881c8d424096db6a9d19b`。正式 Actions 门禁通过，线上 25 项浏览器检查及旧会话延续验证通过；本次部署下载包、临时副本及备份已按授权清理。
范围、验证命令及边界见 [修复记录](../tasks/archive/2026-10-ui-ux-repair.md) 和 [部署记录](../tasks/archive/2026-10-deploy-v1.1.2.md)。

## v1.1.1 Logo 原色修复

已修复 SnowAPI 的单色滤镜把配置的 JPG Logo 压黑/压白及反色的问题，按原图显示。
正式版 [v1.1.1](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.1.1)，源码 `db8a6949d0111785344bd3311eea61fa5932ecdd`；
完整 Actions 检查、本地 16 项 Logo 回归和线上 12 项 Logo 检查通过。
细节见 [Logo 修复部署归档](../tasks/archive/2026-10-logo-color-v1.1.1.md)。

## SnowAPI 界面替换与 v1.1.0

已按用户要求用 SnowAPI 的 Snowflake 首页、Astryx 导航及模块界面替换旧 UI，
默认明亮，认证、模型映射 v2、CAS、支付和订阅沿用 RenewAPI 业务规则。
正式发布：[v1.1.0](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.1.0)；源码 `0d83a1f8fbf607eb00f7a3270fc325003d1e88f7`。
完整前后端、四数据库、108 页面、8 项权限、4 项恢复和高级设置检查全部通过。
来源和边界见 ADR-014，[迁移归档](../tasks/archive/2026-10-snowapi-ui.md)。

## v1.0.0 发布历史

默认前端已改造为 Obsidian Control，包含单页首页、认证、控制台与管理页面。
菜单采用六类任务分组，权限和旧开关兼容；新版模型映射支持有优先级的多对多
直达规则及真实转发候选切换，旧链式配置不自动迁移。行为和限制见
[模型映射说明](MODEL_MAPPING.md)、ADR-012。

已直接在 `main` 提交，并由 [Actions](https://github.com/alex-ai-dev-lab/renewapi/actions/runs/36982359440)
成功发布 [RenewAPI v1.0.0](https://github.com/alex-ai-dev-lab/renewapi/releases/tag/renewapi-v1.0.0)，
源码为 `20cdd2d0c6dd83c5b2e58b852f11684223475d47`。amd64/arm64 离线镜像均通过迁移和下载校验，
正式版已设为 latest。前端 150 测试、Go 完整检查、四版本数据库、108 页面场景、
4 个故障恢复及高级设置流程通过；额外本地浏览器回归 453 项通过。

当时按已确认范围清理 6 个旧/临时 Release、78 个附件和 80 个旧标签，清理后仅剩
`renewapi-v1.0.0` 一个正式发布与标签。清单和证据见[发布归档](../tasks/archive/2026-10-release-v1.0.0.md)
及 ADR-013。后续服务器部署已完成，见下节；文档归档提交不改变已发布产品源码。

## 当前线上部署

2026-10-17 已上线 v1.5.0，站点 `router.108848.xyz:1443`，容器 `new-api-v1-5-0`，Caddy 上游 `127.0.0.1:3025`。源码 `7cd988595a317c7c79c1c551273a17853f7939d6`，正式标签完整门禁通过；环境、会话密钥、挂载及 off 计费模式保留，数据库、105 个渠道、86 项配置及 121 条端点规则校验通过，healthy、重启 0。保留 ADR-019 的长流配置，公网 Claude 100.77 秒、Responses 102.55 秒均正常终态并成功计费。本次下载包、临时文件、备份及退役容器已清理；旧 v1.4.2 连接排空后停止超时，被 Docker 终止（137，非 OOM），新版本隔离停止验证 exit 0。详见[部署归档](../tasks/archive/2026-10-deploy-v1.5.0.md)。

## 线上产品复核

以下任务保留早期复核过程；已完成的对应源码修复包含于当前 v1.1.0，历史“未部署”描述以最新部署记录为准。

本地新增首语义输出超时后台配置、管理员列表真实错误展示，以及渠道连续保存版本同步修复。
验证和未部署边界见 [管理员修复任务](../tasks/active/2026-09-admin-relay-channel-fixes.md)；
超时阶段和优先级见 ADR-011。旧 source 工作树的早期补丁不作为本次发布输入。

本轮修复首页模拟指标、关闭注册后的错误入口、中文与深色主题可读性、临时故障
导致退出登录/离开当前页面、初始化失败缓存，以及渠道测试预检与取消处理。
浏览器门禁扩展为 108 个页面场景和 4 个恢复用例；本地复现、回归和服务端隔离
迁移记录见 [UX 复核任务](../tasks/active/full-ux-performance-audit.md)。
线上镜像修订必须读取运行容器标签，不能仅凭同为 `rc.4` 的版本字符串判断。

## 文档与维护入口

首页已简化为中文介绍、五项主要能力和 Releases 首次安装命令，明确下载校验、
本地镜像导入、数据库迁移与启动顺序。发布规则统一见 [分发说明](release-distribution.md)，
安全政策以根目录 [SECURITY.md](../SECURITY.md) 为准，上游审计入口见 [维护流程](MAINTENANCE.md)。
旧 GHCR 入口、上游镜像安装教程、重复说明和空赞助模板已移除；移动端 Hook
只保留实际解析到的 `.ts` 实现。默认前端及其公共资源、两份发布 Compose、开发
配置、历史 ADR/迁移/验收证据和未完审计继续保留，不能仅凭重名或年代删除。

## 当前发布维护

可靠性升级已合并至主分支。随后实际浏览器 CI 暴露渠道集合路径缺少显式无尾斜杠
注册的问题；`347398928` 已修复，本地和 GitHub runner 的完整 88 项浏览器复测均通过，控制台错误为 0。

统一发布已经启用：成功构建只向 GitHub Releases 发布经过校验的 amd64、arm64 离线镜像、配置和 QA 证据，不再发布 GHCR。
已清理 74 个旧 Releases、366 个附件、702 条旧 Actions 运行和 GHCR 的 685 个历史版本，重新查询确认无旧记录遗漏。
已有工作分支已合并到 `main`，后续直接在主分支维护。验证与清理证据见
[发布维护归档](../tasks/archive/2026-09-release-pipeline.md)、[分发说明](release-distribution.md)
和 ADR-010。历史 Git tags、用户原有修改及生产数据保持不变。

## 产品与开发基线

- `VERSION` 及线上正式版本为 `v1.4.1` / `renewapi-v1.4.1`，源码构建继续使用独立预发行身份。生产部署结果按运行镜像修订和服务端验收记录核实。
- 可靠性升级原开发分支：`upgrade/2026-09-reliability`，已合入 `main`；后续直接在主分支维护。
- 分支基线：`4def3a8848e336532c55e9c3070cc78ead67e378`。
- 可靠性升级功能及复核修复提交截至 `a2321c2b1dae57a98c4ed18ec3197f97cd76c62d`；本次发布维护的路由修复和构建验证另见上方归档。
- 原 `agent/06-billing` 工作树及其用户未提交修改保持原样。
- 产品 tag 使用 `renewapi-` 前缀；九月清理未删除 Git tags，十月首次正式发布的全部旧标签清理另有明确授权，见 ADR-013。

## 本轮升级

- 独立请求级 Failover 状态，最多六个不同 Channel ID；复用已有 priority、weight、模型支持、Enabled 筛选和 route plan。
- 默认 15 秒首语义输出期限；提交前隔离 SSE 与响应头，空流、错误、超时或畸形响应可切换。内容提交后不拼接另一个渠道的流。
- 本地模型/权限/额度等可解释错误保留明确分类，未知上游故障使用公共 capacity 文案；管理员在 `Other.admin_info` 查看真实错误、逐渠道尝试链、用量和返回模型。前序失败独立进入渠道健康记录。
- 独立 Prompt Profile 表、管理员 CRUD、稳定渠道引用、默认回退、引用拒删、Anti-Poison 追加 nonce，以及默认关闭的“自动测试仅 AutoDisabled”模式。
- default/classic 两套前端同步 Prompt 管理、渠道设置、管理员错误链及原生 WS 开关；修复渠道模型选择器 light/dark 背景与自动展开。
- 补齐 off 原子退款、终态幂等、enforce 故障恢复和成功渠道用量归属；旧 Options 表增加实际主键迁移，保留定价选项。
- Responses WebSocket 独立于 Realtime，每轮重做鉴权、限流、路由和计费。支持 HTTP/SSE 桥接、可选原生上游、lane 隔离、取消、有限历史恢复、连接容量和关闭等待。
- 复核补齐终态用量估算、null 用量回退、HTTP/WS error 解析、错误扩展字段隔离及上游事件关联。

验收状态、22 项核心行为映射、命令和 CI 证据记录在 [本轮升级任务](../tasks/archive/2026-09-reliability-upgrade.md)。架构边界见 [ARCHITECTURE](ARCHITECTURE.md)、ADR-006 至 ADR-009。

## Billing 与历史任务

代码默认继续为 `BILLING_LEDGER_MODE=shadow`。2026-10-17 审计收尾将 off 新请求接入既有持久账本，补修 shadow/off 异步任务重复扣退路径；此前“off 无跨进程保证”的记录仅描述旧版本。新恢复边界与模式切换条件见 [Billing enforce 说明](billing-ledger-enforce.md)。线上已升级 v1.5.0 并保留现有 off 配置；隔离副本数据核对、加列迁移与旧实例连接排空已完成，旧无账本差额不自动推算。

历史 RU-A 缺少外部数据库与进程重启证据的状态已由本轮开发验证补齐：实际 MySQL 5.7/8.4、PostgreSQL 9.6/16 Actions，以及 SQLite 独立进程崩溃/恢复。证据以任务及 Billing 文档记录的提交为准。其余旧 UX/性能审计结论未因此自动变成已完成，仍按对应任务的历史证据边界处理。

## 上游审计边界

完整历史审计基线仍为 New API `58d4e9bd3bb035df8ea235dd682ccc8a45d0332a`（2026-08-14）。本轮是针对用户指定可靠性项目的专项复审，引用 New API `996adffe5165bd5e311e33a03a86b8aede1fe376`、Sub2API `20a94fbb567b62208751292ed7786b24a7e7c0fe`；不声称已分类两个上游的所有后续提交。判定与本地实现见 [UPSTREAM_PORTS](../UPSTREAM_PORTS.md)。

## 已知边界

- v1.2.0 起仅维护默认前端；v1.1.3 及更早版本的双前端记录保留为历史。全量 lint/format 存量问题在任务中单独记录，不将改动文件检查通过写成全量通过。
- WS 的 `generate:false` 只预热本地输入，不预热上游模型；不支持 mid-turn steering，不跨客户端共享带会话状态的上游连接。
- `store=false` 的历史仅在当前连接中有限保存；断线或历史不可用时需要完整 input。已提交工具调用不会自动重放。
- 本地故障注入和 hosted 数据库验证不代表生产已切换；线上验证范围、镜像修订及数据备份需结合 UX 复核任务指向的服务端记录。
- Go module 路径保持 `github.com/QuantumNous/new-api`，支持 SQLite、MySQL >= 5.7.8、PostgreSQL >= 9.6，保留既有兼容、安全和 Anti-Poison 边界。
