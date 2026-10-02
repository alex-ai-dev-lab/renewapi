# Obsidian Control 源码改造

> 本页记录首次 UI 改造阶段。后续已经加入任务式菜单和后端混合映射执行，请以 [任务菜单与混合模型映射](MODEL_MAPPING.md) 查看当前功能、兼容性和重试边界。

> v1.0.1 按用户要求改为默认明亮外观和紧凑分组侧栏：中性白灰底色、深色操作、细幅选中标记。初期深色默认值和薄荷操作色不再代表默认外观；用户显式选择的深色模式仍可使用。

## 工作副本

- 目录：`D:\Code\renewapi\ui-20261001\renewapi`
- 当前维护分支：`main`（初期隔离开发使用 `feature/obsidian-ui`，后按用户要求直接发布主分支）
- 基线：`main` / `d35afe9528fb630199b8b40e1a528f33afe3239d`
- 视觉来源：本次 `01-obsidian-control` 模板。不是将静态 HTML 嵌入应用。
- 原 `D:\Code\renewapi\source` 及 20 套模板保持不动。正式发布和验证结果见[发布归档](../tasks/archive/2026-10-release-v1.0.0.md)；发布镜像不等于部署服务器。

## 本轮内容

### 首页与认证

未登录默认首页改为简洁单页：真实品牌和导航、短标题及说明、注册/登录/控制台入口、API 请求路径动效、可复制的接入示例及精简页脚。

- 示例使用配置的 API 地址或当前 origin，明确标记为示例；不自动调用模型。
- 入场/线路动画使用 CSS/SVG，支持系统减少动效偏好。
- 保留 `HomePageContent` 的 Markdown / URL iframe、自定义页脚净化、法律链接、文档链接、注册开关和 SEO。
- 登录、注册、找回、2FA、密码重置及 OAuth 回调仅改展示；密码、Passkey、验证码、OAuth 和认证重定向链路保留。

### 控制台

- 232px 侧栏（折叠为 56px 图标列）+ 56px 顶栏；窄屏使用抽屉，不再使用桌面底部 Dock。
- 桌面、移动端和命令面板复用菜单配置及角色过滤，保留上下文设置导航。
- `Ctrl+K` / `⌘K` 与搜索按钮共用一个命令弹窗。
- 保留第一套的细线、紧凑间距和小圆角，以明亮白灰色作为默认语义 tokens；深色模式保留黑灰和克制的薄荷强调。
- 保留已有的深浅/系统、字体、preset、缩放及站点自定义偏好。无已保存选择时默认明亮，不自动跟随系统进入深色。
- 整理旧视觉 CSS 的生效入口，不继续叠加 Aurora/Interface Zero 的大圆角、渐变和阴影覆盖。

### 真实业务

- Dashboard 保留普通用户/管理员数据、四类分析、时间范围和刷新；改为连续指标条、趋势主区、状态/用量辅助区、明细和快捷操作。不同单位分图；普通用户不显示接口没有提供的成功率、延迟或成本。
- 凭证、日志三分类、钱包、模型价格目录、个人资料、Playground、排行、错误页和首次安装流程完成展示适配。
- 渠道及完整编辑器、用户、模型、订阅、兑换码、测试提示词及七类系统设置完成展示适配。
- 共享表格保留筛选、分页、移动卡片、批量操作、固定列、空/错/加载状态；抽屉和确认框保留原有业务行为。
- 未更改后端接口、数据库结构、支付参数、权限规则或模型转发逻辑，没有添加虚构团队/项目功能。

## 如何运行

两套前端已在本机构建，现代前端产物位于 `web/default/dist`，经典产物位于 `web/classic/dist`。这些是 Git 忽略的构建文件，不会随源码提交。

### 查看完整应用

在 PowerShell 中：

```powershell
cd D:\Code\renewapi\ui-20261001\renewapi
$env:GOTOOLCHAIN = 'go1.25.1'
go run . migrate --up
go run .
```

默认访问 `http://localhost:3000`，全新数据库先进入 `/setup` 创建自己的管理员。没有写入可供生产使用的默认账号密码。

`go.mod` 要求 Go 1.25.1；本次已通过 Go 工具链机制获取并成功构建。请不要将 QA 临时账号/数据库用于实际部署。

### 前端开发

先启动上面的本地后端，再开一个终端：

```powershell
cd D:\Code\renewapi\ui-20261001\renewapi\web\default
bun install --frozen-lockfile
bun run dev
```

访问终端输出的 Rsbuild 地址。默认代理到 `http://localhost:3000`；需要其它本地后端时通过 `VITE_REACT_APP_SERVER_URL` 指定。

### 入口与已有配置

- 站点级 `theme.frontend=default` 表示现代前端，本轮在此包内实现 Obsidian；未添加新的服务端主题枚举。
- `classic` 仍可显式选择，且保持原样。`default/classic` 与浏览器的 `dark/light/system` 是两层不同设置。
- 已有站点的 `HomePageContent` 若非空，会按原规则覆盖内置新首页。这是兼容行为，不是新首页失效。
- 非 master 部署若配置 `FRONTEND_BASE_URL`，页面可能按原后端规则跳转到外部前端。

## 验证方式

在 `web/default`：

```text
bun test
bun run test:security
bun run test:request-errors
bun run typecheck
bun run build:check
bun run lint
bun run format:check
```

实际执行记录：

- 修改前：冻结依赖安装、单测、typecheck 和生产构建通过。
- 本轮单测：96 通过、0 失败，253 个断言（23 个文件）。
- 新版前端生产构建及包含两套前端的 Go 程序构建通过。
- 真实 Go + 临时 SQLite 的初始化、登录、普通用户创建、凭证创建/列出/删除、自定义 Markdown 首页验证通过。
- 浏览器脚本：`scripts/obsidian-browser-check.cjs`。采用本机 Chrome，覆盖 1440 / 1280 / 1024 / 768 / 390px、深浅色、访客/Root管理员/普通用户，并检查命令面板、默认深色、系统配色响应及权限路由。最终 **447 项检查通过，0 个问题、0 个脚本错误、0 个控制台错误、0 个网络失败**。普通管理员角色边界另由菜单单测覆盖。
- 原始浏览器结果和 ESLint 基线对比位于 `qa-artifacts/obsidian/`；最终结果以该目录的 `browser-report.json` 为准。
- **未生成截图，也未进行截图像素回归或人工截图审美签核。**

浏览器测试依赖通过前端目录解析 `playwright` 和 `@axe-core/playwright`；本工作目录可解析到同级模板工程此前安装的测试依赖。若单独复制此仓库，需先在 `web/default` 中安装临时 QA 依赖（例如 `bun add --no-save playwright @axe-core/playwright`），或在独立测试环境提供它们。`QA_BINARY` 指向已构建的后端程序，`CHROME_PATH` 可指定浏览器；脚本没有修改正式依赖锁文件。

```powershell
$env:QA_BINARY = 'C:\path\to\renewapi-qa.exe'
node scripts/obsidian-browser-check.cjs
```

脚本始终使用新建临时 SQLite、随机测试密码和本地请求；不访问正式数据库。测试阶段仍启用限流，但只在该测试子进程提高 panel/search 等预算，防止批量页面遍历被正常限流中断；没有修改产品默认限制。

## 明确限制

1. 全仓 ESLint 未清零：完整检查记录为118个 errors / 34个 warnings；已将本次改动与 HEAD 逐文件比较，没有新增同规则/同严重性诊断。未借 UI 改造重写历史支付、流式和 effect 逻辑。
2. 全仓 `format:check` 对67个未修改的基线文件给出提示。本次修改文件已单独格式化，没有批量格式化无关源码。
3. 浏览器验收使用本地真实后端及临时测试数据，不等价于验证第三方支付、邮件、真实 OAuth、真实模型服务和生产数据规模。未发起真实扣款或收费模型请求。
4. 只运行 Chrome；没有单独执行 Firefox、Safari、真实屏幕阅读器或移动设备测试。自动 axe 检查不等于完整 WCAG 合规认证。
5. 旧视觉实现文件没有盲目删除；它们已从本轮新界面的有效入口中退出或不再引用。后续可在独立清理任务中确认无引用后删除。
