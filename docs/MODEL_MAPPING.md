# 任务菜单与混合模型映射

本次改动在 `D:\Code\renewapi\ui-20261001\renewapi`，继续使用 `feature/obsidian-ui`。未提交、推送或部署；保留之前的 Obsidian 改造及原源码/模板目录。

## 1. 菜单怎么分

总览固定在前，其余按任务分组：

| 分类 | 功能入口 |
|---|---|
| 接入与调试 | 模型与价格、API 密钥、调试台、已有聊天预设 |
| 用量与排障 | 数据分析、请求日志、任务日志 |
| 资金与账户 | 钱包与订阅、个人设置 |
| 模型与渠道 | 上游渠道、模型管理 |
| 用户与运营 | 用户、订阅方案、兑换码 |
| 系统管理 | 平台设置及其上下文导航 |

普通用户只看到允许的使用功能，管理入口按角色过滤，系统设置仍要求 Root。公开模型价格目录不等于后台模型注册表；个人购买订阅与管理员配置订阅方案也分开。

### 配置兼容

- 旧 `SidebarModulesAdmin`、用户 `sidebar_modules` 的分区/模块键继续作为权限与隐藏开关。
- 新展示分类不再承担权限身份。先过滤，再分组，不能通过重分类绕过管理员禁用。
- 新增可选 `SidebarTaskSectionOrder`，合法任务 ID 为 `access,usage,account,models,operations,system`。
- 旧 `SidebarSectionOrder` 保留，经典界面仍使用它。新字段为空时采用新默认或兼容投影旧自定义顺序，组内顺序稳定去重。
- `/system-settings/operations/overview` 现在明确继承旧 `admin.setting`，命令搜索也遵守同样限制。
- 模型价格快捷入口还同时受公开 `HeaderNavModules` 和 `console.pricing` 控制。

## 2. 首次添加映射的 bug

原实现首次添加空行后，把空草稿序列化为 `{}`。父表单回显触发 effect 重新解析，刚加的行被删掉；第二次父值不变才留下。未完成草稿也可能因此丢失。

现在草稿行与持久化值的生命周期分开：首次点击立即出现并聚焦，先填目标或编辑其它行不会删除草稿；外部重置/换记录仍能正确重新加载。未完成行给出结构化错误，不再用故意损坏的 JSON sentinel 阻止保存。

## 3. 新版规则

模型映射仍保存在渠道的 `model_mapping` JSON **字符串**中，数据库仍是原 TEXT，不新增业务表。

编辑器的 JSON 模式使用以下内部内容：

```json
{
  "version": 2,
  "rules": [
    {"id":"r1","from":"model-1","to":"upstream-A","priority":100,"enabled":true},
    {"id":"r2","from":"model-1","to":"upstream-B","priority":80,"enabled":true},
    {"id":"r3","from":"model-2","to":"upstream-A","priority":100,"enabled":true},
    {"id":"r4","from":"model-2","to":"upstream-B","priority":60,"enabled":true}
  ]
}
```

API 请求中，以上对象需再序列化为 `model_mapping` 的字符串值；不要改变原外层字段类型。

### 执行含义

- 来源模型和上游模型均可重复，不要求一对一。
- 优先级数字大者先；相同优先级按行顺序。优先级只比较当前映射输入模型的候选，不给不同 HTTP 请求排队。
- v2 只做一次精确匹配，不递归。因此 `A→B` 与 `B→A` 可以各自直达。
- 完全重复目标只作为同渠道候选尝试一次，不通过重复行增加调用次数。
- 规则 ID 用于稳定编辑及排障，须在本配置内唯一；界面自动生成。这不限制来源/目标模型重名。
- `priority` 缺省为0，`enabled` 缺省为true，显式false被保留。JSON 数值 `1.0` / `1e2` 若表示范围内整数，同样有效；字符串数字无效。
- 规则属于当前渠道。渠道优先级、权重、分组、偏好和严格绑定仍是另一个层级。

### 模型暴露与价格

有效来源会与显式 Models 一起进入路由能力，目标不会仅因为是映射目标就自动对外公开。显式 Models 中已有的模型仍保留；关闭一条映射不等于禁用该公开模型。

映射不会自动为别名创建价格或放宽 Token 白名单。授权和计费继续使用既有客户/路由/计费模型策略，不因为切换上游名称就擅自改收费身份。

## 4. 旧配置与升级

旧格式继续支持：

```json
{"public":["bridge","backup"],"bridge":"upstream-A"}
```

旧格式仍链式展开，所以 `public` 的候选为 `upstream-A, backup`。新规则不会链式展开，不能简单把旧边逐条照抄。

- 打开旧记录或修改其它字段不自动升级。
- 使用“预览升级为直达规则”，确认展开后的最终候选及顺序，再保存。
- 旧循环或超限配置不能被自动猜测修复。
- v2 禁止被旧客户端静默降级为单值字典或空串；请用新版界面管理。
- 单渠道明确清空使用 `{"version":2,"rules":[]}`。
- 标签批量编辑中空字段仍是“不更新”。添加草稿又删掉最后一行不会清空全批配置，只有明确“清空映射”才会提交空规则。
- 上线前先升级所有读取配置的后端实例，再写入 v2；不要混合旧解析器节点。本轮未进行任何部署。

## 5. 候选切换的安全边界

同渠道 A→B 与跨渠道共用原请求总尝试预算（默认最多6次，且还受既有较小重试预算约束），不无限重试。候选耗尽后才按原策略选下一渠道。

允许推进的范围为明确模型级拒绝（400/404/422）或精确模型容量错误码的429/503。**不是所有429、5xx或超时都会尝试下一模型**。认证、泛限流等交回原渠道错误/重试策略。

以下情况不新增同渠道候选尝试：
- 客户端取消、预算耗尽、严格绑定或已有 SkipRetry；
- 已写响应/流式内容、观察到 usage、账务已结算或已退款；
- 异步任务已受理、执行状态不明、未知账务实现。

active 零预扣不等于已结算，使用只读账务状态判定。一个逻辑请求保留同一预扣/结算生命周期；这不保证上游在未知失败前没有收费。

### 参数覆盖的优先级

高级 `param_override` 可能在映射之后改变实际 model。为避免配置 A/B 实际却重复发送 X：
- 记录独立的最终 wire model，不改变已有 adapter URL 或计费模型语义。
- 固定/改变 model 的覆盖会关闭同渠道候选推进，即使固定值恰好等于首候选。
- 普通 legacy 非 model 覆盖（例如 `{"temperature":0.2}`）仍可正常切换候选。
- **当前保守限制：`param_override` 的 `operations` 格式会关闭同渠道候选推进，包括非 model 操作。** 这是另一个高级覆盖 DSL，不是模型映射 v2 的 `rules` 数组；两者不要混淆。

管理员日志现在显示实际目标与映射规则 ID，并将“渠道优先级”明确标注，避免与规则优先级混淆。

## 6. 输入上限

- 配置原文：1 MiB；最多1024条规则或旧来源。
- 模型名：255个 Unicode 字符；规则 ID：128字符。
- 优先级：有符号32位整数。
- 旧图总边数4096，单来源最多128个最终候选，链深度32。
- 展开工作量：单次8192步、完整写入验证65536步。

“模型名称不限制唯一”不代表没有资源和重试上限。重复 JSON 对象字段会被严格写入验证拒绝；需要一对多请使用规则数组，而非在同一对象中写两个同名键。

## 7. 验证与排障

最终结果：前端 **146 tests / 0 fail**，类型检查和生产构建通过；相关八个Go包及Go构建通过；真实Go+SQLite+Chrome **453项检查通过**，无脚本、控制台或网络错误。147个前端改动文件未新增ESLint诊断，已有无关全仓lint/格式基线仍保留。

测试没有连接生产数据库、真实付费模型或支付服务，也未单独验证真实MySQL/PostgreSQL实例和Firefox/Safari；本地上游桩用于验证实际HTTP请求、重试顺序与账务边界，不代表第三方上游绝不会收费。

本次补充了：
- 前后端共用20组契约夹具：`common/testdata/model-mapping-contract.json`。
- 编辑器受控父组件回归、草稿/重置/批量语义、菜单权限与顺序测试。
- 真正 Distribute→Relay→adapter→HTTP 上游桩→SQLite 账务的候选链路测试。
- 旧 PUT 无 If-Match 的受控并发回归，防止旧快照覆盖较新 v2 映射而造成 abilities 不一致。
- 非 passthrough 参数覆盖固定 model 的实际请求回归，防止重复 X。

主要命令：

```text
bun test --cwd web/default
bun run --cwd web/default typecheck
bun run --cwd web/default build:check
GOTOOLCHAIN=go1.25.1 go test ./common ./model ./service/channelconfig ./relay/helper ./relay/common ./relay ./controller ./service
```

浏览器专项复用真实临时后端：

```text
QA_MAPPING_FLOWS=1 QA_OUT=qa-artifacts/model-mapping node scripts/obsidian-browser-check.cjs
```

Windows PowerShell 请用 `$env:QA_MAPPING_FLOWS='1'` 等设置本终端环境变量。`QA_BINARY` 指向包含最新前端产物的本地构建程序。测试全部使用随机测试密码、临时 SQLite，不访问生产库、不执行真实付费模型或支付。

原始报告在被 Git 忽略的 `qa-artifacts/model-mapping/`。初期验收见[映射任务归档](../tasks/archive/2026-10-ui/menu-model-mapping.md)，正式发布验证见[发布归档](../tasks/archive/2026-10-release-v1.0.0.md)；不截图。架构理由见 [ADR-012](decisions/012-model-mapping-rules.md)。
