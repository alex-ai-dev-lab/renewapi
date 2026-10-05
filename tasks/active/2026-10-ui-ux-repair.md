# 2026-10 功能与体验修复

状态：本地实现与回归完成，v1.1.2 发布进行中。日期：2026-10-05。

## 起点与边界

2026-10-05 用户追加授权：提交修复，经 GitHub Actions 构建镜像到 Releases，再由服务器代理下载并热切换；验收通过后清理本次部署中间文件和临时备份。生产切换结果另行记录，既有历史 Release/标签及此前备份不属于本次清理范围。

基于 `main@4e3325b` 的真实站点评估，修复 SnowAPI 界面与 RenewAPI 后端的合同/权限差异，保留业务、支付、渠道映射/CAS、URL 与开源署名。没有修改生产设置或生产账号数据；测试账号、密码、认证器秘密不进入仓库。

## 实施结果

- 定价响应新增 `current_group`；模型可用性优先依据可用分组，缺失信息显示未知；供应商名称不再从图标字符串生成。
- 本人资料新增安全的 `has_password` 布尔值；缺失充值/限流数据明确显示无数据。恢复密码、Passkey 和 2FA 管理。
- 修复本人资料的部分更新：只更新显式请求的 username/display_name/password，忽略角色和额度等字段；防止空结构体覆盖角色、状态、分组、绑定等属性。
- 套餐管理使用现有 topup info 合规摘要，不放宽全量 options 权限；区分读取失败、加载中、未确认、已确认。模型 options 查询仅超级管理员可触发。
- 侧栏与命令面板统一过滤与分类，保留旧模块配置/深链；不同日志标题、资金记录及套餐名称明确。
- 新账号减少空白统计面板，提供受权限控制的接入步骤和文档入口。
- 初始 HTML、路由 pending、懒加载弹窗都有反馈与恢复；公开页面修复头部重叠、手机页脚、按钮对比度、Logo 回退、站点名称与中文文案。
- 追加交互修复：取消密码弹窗清空敏感草稿；密码长度与后端约束对齐；2FA 状态失败不冒充关闭状态并支持重试；代码块支持键盘滚动；看板辅助文字对比度改善。

## 验证

- 初轮 `cd web/default && bun test`：213 项通过；审查后补充回归：218 项通过。
- `bun run typecheck`、生产构建、改动前端 ESLint、Prettier 与版权检查通过；默认被规则排除的 Markdown 渲染器额外执行 `eslint --no-ignore` 检查。
- `go test ./controller ./model -count=1` 与 `go vet ./controller ./model` 通过。
- `scripts/qa/frontend-repair-check.cjs` 使用本地 Go 二进制、一次性 SQLite、随机凭据，不调用付费模型、支付网关或外部邮件。覆盖普通用户/管理员/超级管理员准备流程、真实密码修改、2FA 启停、虚拟认证器 Passkey 注册、手机/明暗页面、中文文档、权限与加载恢复。
- 初轮浏览器 21 项检查通过；审查发现部分 `newPage()` 未收集运行时错误，不能据初轮 `pageErrors=[]` 推断全部页面均无异常。现统一在 context 注册 pageerror 监听。
- 浏览器明细与截图位于忽略目录 `.cache/frontend-repair-qa/`；最终数量与失败列表以该目录 `report.json` 为准。该目录不应提交。

本地复现命令：

```sh
cd web/default
bun run build
cd ../..
go build -o .cache/renewapi-ui-repair-qa.exe .
node scripts/qa/frontend-repair-check.cjs
```

该浏览器入口默认使用 Windows Chrome 路径和 localhost:39011。Passkey 用显式匹配的 localhost RP ID / Origin；IP 地址不符合浏览器的 RP 域名要求，不能据此判断生产 Passkey 失败。物理认证器、第三方 OAuth、真实支付与生产发布不属于本轮验证。

## 审查后补充修复（2026-10-05）

- 已读取原 Claude 会话并核对授权边界。本轮保留现有修复，以本地随机账号验证，没有使用生产测试账户做设置或业务写入。
- 一次性后端环境只继承必要系统变量，显式指定本地 SQLite，清除 SQL/日志库/Redis 外部连接和外部应用秘密；启动前检测固定端口占用。`node --test scripts/qa/disposable-environment.test.cjs` 通过，验证父进程连接配置不会进入测试后端。
- 套餐分块加载和错误态复用共享 Dialog，支持 Escape/关闭按钮与焦点管理；关闭时移除遮罩，迟到分块不重新打开弹窗，网络恢复后可重试。
- 模型目录复用共享菜单定义及既有 `console.pricing` 模块键，与分类开关、侧栏、命令面板和新手引导一致；保留模型目录 URL 和公开定价入口，概览仍使用 `console.detail`。
- 模型权限区分缺字段、明确空集合和可用集合；定价 hook 与目录合并过程保留未知状态，避免中途把缺失字段归一化为无权限。
- 手机导航和侧栏 Logo 统一使用 BrandImage，保留原图颜色、配置站点名称与本地/文字回退。
- 新增浏览器断言覆盖实际模型目录字段故障、禁用目录的侧栏/命令面板/引导一致性、超级管理员命令、套餐分块加载取消和失败恢复，以及手机导航 Logo 与键盘关闭。
- 最新构建的最终浏览器回归 27 项通过，issues=[]、pageErrors=[]；前端 218 项单测、类型检查、生产构建、增量 ESLint、Prettier 和版权检查通过。既有后端改动已独立重跑 `go test ./controller ./model -count=1` 通过，本次追加修复未修改 Go 业务代码。这些检查不等同于所有页面、角色、主题与功能的笛卡尔积穷举。
