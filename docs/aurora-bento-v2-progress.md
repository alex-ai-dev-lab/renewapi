# Aurora Bento v2 进度与验收总表

> 最后更新：2026-08-23 12:04 +08:00  
> 仓库：`alex-ai-dev-lab/renewapi`  
> 当前产品提交：`9f90bd234d1533dc4f4efb916cc826d5626d4737`  
> 原稿目标：`renewapi-design-02-aurora-bento-v2`  
> 状态口径：**Desktop / Light / 1440×1000 原稿匹配已完成；跨主题、跨设备与更深交互验收进入下一阶段。**

---

## 1. 总体状态

| 维度 | 当前状态 | 结论 |
|---|---|---|
| Desktop Light 原稿视觉匹配 | ✅ 完成 | 浏览器渲染 Design QA 已 PASS，无可执行 P0/P1/P2 |
| App Shell / Topbar / Floating Dock | ✅ 完成 | 已从传统永久 Sidebar 框架切换为 Aurora 顶栏 + 紧凑悬浮 Dock |
| Dashboard Bento 几何 | ✅ 完成 | 已恢复 8/4 + 3/3/3/3 + 12 主构图 |
| Channels | ✅ 完成 | Provider Bento + 真实 24H 分布 + 生产管理表 |
| API Keys | ✅ 完成 | 8/4 概览 + 真实令牌消耗分布 + 完整管理表 |
| Usage Logs | ✅ 完成 | 原稿 KPI 节奏 + 实时调用流 + 生产筛选/表格 |
| Models | ✅ 完成 | Model Registry 六卡 + 真实定价；完整管理默认收起 |
| Users | ✅ 完成 | 全局用户/分组/停用统计 + 完整用户管理 |
| System Settings | ✅ 完成 | 6/6 + 12 控制台；真实 option-backed switches + foundation fields |
| 工程质量门 | ✅ 完成 | Prettier / ESLint / TypeScript / Tests / Rsbuild / diff check 全绿 |
| 浏览器运行时 | ✅ 完成 | 7 个核心页面 console error = 0，unhandled QA API = 0 |
| Dark Mode 原稿级验收 | ⏳ 待做 | 目前仅保证主题兼容，未按原稿标准独立 QA |
| Tablet / Mobile 原稿级验收 | ⏳ 待做 | 保留现有移动端 Shell，但尚未做专门视觉匹配 |
| Keyboard / Screen Reader / Reduced Motion | ⏳ 待做 | 尚未进行完整无障碍验收 |
| Settings 真实后端 mutation 端到端验证 | ⏳ 待做 | UI 已绑定真实 API；QA 截图使用稳定 fixture，尚未在真实生产后端做 mutation smoke test |
| Release / 部署后 smoke test | ⏳ 待做 | 代码已合入 main，下一阶段需对实际部署环境做最终巡检 |

### 当前阶段结论

**Aurora Bento v2 的 Desktop / Light 主目标已经完成。**

最终浏览器 Design QA 使用与原稿一致的桌面状态：

- viewport：`1440 × 1000`
- deviceScaleFactor：`1`
- locale：`zh-CN`
- timezone：`Asia/Taipei`
- theme：`light`
- 核心页面：Dashboard / Channels / API Keys / Common Logs / Models / Users / System Settings
- console errors：`0`
- unhandled QA API requests：`0`
- 最终可执行视觉问题：**P0 = 0 / P1 = 0 / P2 = 0**

当前没有把“100%”解释成像素逐点相等；验收口径是：**不再存在会明显改变原稿构图、层级、密度、色彩语言和首屏产品身份的 P0/P1/P2 偏差。**

---

## 2. 已完成页面矩阵

| 页面 | 原稿核心结构 | 当前实现 | 真实数据/能力 | 视觉状态 |
|---|---|---|---|---|
| Dashboard | Hero + 8/4 + 4×3 KPI + 12 列渠道表 | 已重构 | overview stats / channels metadata / cost / success | ✅ PASS |
| Channels | Provider 卡片 + 24H 请求分布 | 已重构 | 真实渠道、延迟、成功率、成本、模型数 | ✅ PASS |
| API Keys | 8/4 Token 概览 + 清单 | 已重构 | 真实 quota / used quota / token management | ✅ PASS |
| Usage Logs | 3×4 KPI + Live Stream | 已重构 | Usage / RPM / TPM + 完整日志筛选/表格 | ✅ PASS |
| Models | Registry Header + 6 卡 | 已重构 | 真实模型 / vendor / pricing / deployment entry | ✅ PASS |
| Users | 3×4 KPI + 用户与分组 | 已重构 | 全局 user total / groups / disabled count | ✅ PASS |
| Settings | 6/6 + 12 控制台 | 已重构 | 真实 `/api/option/` switches + bulk update | ✅ PASS |

---

## 3. App Shell / 全局设计语言

| 项目 | 状态 | 当前实现 |
|---|---|---|
| 1240px 主画布 | ✅ | 桌面端统一 Aurora Canvas |
| Aurora Topbar | ✅ | 品牌 / System Health / Profile 为主视觉，辅助工具收进轻量 pocket |
| Floating Dock | ✅ | shrink-to-content，按真实权限显示入口 |
| 传统永久 Sidebar | ✅ 已退出桌面主视觉 | 移动端与复杂上下文仍保留兼容能力 |
| Hero | ✅ | 38px 级显示标题、渐变强调、原稿节奏 |
| Glass Surface | ✅ | 22px radius / 18px blur / 低对比 border / ambient glow |
| Bento Geometry | ✅ | 页面级 12-column 组合，不再只是统一 Card reskin |
| Light palette | ✅ | 蓝 / 紫 / 粉 / 绿环境光与原稿 surface tint 已落实 |
| Production tables | ✅ | 保留完整操作能力，同时降低传统 admin-console chrome 存在感 |
| i18n | ✅ | Aurora 可见文案走 `t('aurora.*')` / 双语 fallback |
| 权限边界 | ✅ | Channels/Models/Users = Admin，Settings = Super Admin |

---

## 4. 关键工程与数据约束处理

| 约束 | 处理结果 |
|---|---|
| 不伪造预算 / 昨日环比 | ✅ 未伪造 |
| 不伪造 429 / 5xx 全局统计 | ✅ Logs 使用真实 Usage / RPM / TPM 替代 |
| 不伪造兑换码“待处理”总数 | ✅ Users 第三 KPI 使用真实停用账户统计 |
| Model pricing | ✅ 复用现有 `/api/pricing` 与 Pricing 页格式化逻辑 |
| Settings 原稿开关 | ✅ 只映射存在真实 option 的策略；无真实键的原稿字段不伪造 |
| Settings live mutation | ✅ 使用现有 `useUpdateOption()` / `useUpdateOptionsBulk()` |
| 渠道总数/启用数分页误差 | ✅ 使用真实 total 或明确当前视图口径 |
| Models 当前页 vs 全局总数 | ✅ 分开显示，避免把当前页数量误称全局 |
| Keys 聚合只加载部分数据 | ✅ 文案明确“已载入”口径，不伪装全局精确统计 |
| Dashboard 管理员 Provider 类型 | ✅ 仅管理员读取对应元数据；普通用户不越权 |

---

## 5. 工程质量证据

最终产品树已通过以下质量门：

| 检查 | 状态 |
|---|---|
| `bun install --frozen-lockfile` | ✅ PASS |
| changed-file repository Prettier | ✅ PASS |
| changed-file ESLint + copyright | ✅ PASS |
| `bun run typecheck` | ✅ PASS |
| `bun test` | ✅ PASS（52 tests / 0 failures） |
| `bun run build` / Rsbuild | ✅ PASS |
| product diff / whitespace check | ✅ PASS |

最终质量证据 run：`32615101043`

最终浏览器 Design QA 证据 run：`32614926945`

最终合入提交：`9f90bd234d1533dc4f4efb916cc826d5626d4737`

PR：`#89 feat(web): match Aurora Bento reference experience`

---

## 6. Visual QA 历史与修复记录

| 阶段 | 发现 | 修复 |
|---|---|---|
| Round 1 | Dock 被拉成近整宽；页面仍有 admin-console 感 | Dock shrink-to-content；DataTable chrome 降级 |
| Round 2 | Keys 上半区过高；表格 toolbar 太重 | Keys 8/4 高度收紧；toolbar 降级为轻量控制行 |
| Round 3 | tint 被全局 card background 覆盖 | 提高 Aurora reference surface 优先级，真实显示蓝/粉/绿 tint |
| Round 4 | Settings 仍是目录页；且一次 QA 出现运行时 500 | Settings 改为真实 option-backed 控制台；修复 QA workflow/response shape |
| Round 5 | Settings 色彩/高度偏重；Models 首屏密度偏高 | 进一步收紧 surface、卡高与 Registry 密度 |
| Final | 对 7 页同 viewport/source 进行最终比对 | 无剩余可执行 P0/P1/P2，Design QA PASS |

---

## 7. 已接受的 P3 偏差

这些差异是**有意保留**，不算当前缺陷：

1. 使用真实 RenewAPI logo / avatar / counts / model names / pricing / statuses，而不是原稿里的示意占位内容。
2. Settings 使用真实 option-backed 字段；原稿中没有安全产品映射的控件不会伪造。
3. 生产筛选器和操作栏仍保留，所以比静态原稿的简化清单略密。
4. Channels 的详细管理区放在原稿 overview fold 下方，而不是为了截图删掉生产能力。
5. Models 的完整管理能力保留在“管理模型”入口之后，默认首屏优先原稿 Registry。

---

## 8. 下一阶段路线图

### Phase A — Dark Mode 原稿化

**优先级：P1 / 下一阶段首要任务**

目标：不是简单把 Light 反色，而是建立独立的 Dark Aurora Bento 视觉层。

待做：

- [ ] 7 个核心页面 `1440×1000 / Dark` 浏览器截图
- [ ] Deep navy / charcoal canvas 与低亮 ambient glow
- [ ] Dark glass 层级与边界对比
- [ ] 状态色在暗背景的 WCAG 对比复核
- [ ] gradient / chart / table header 在 Dark 下重新采样
- [ ] Dark 模式最终 P0/P1/P2 Design QA

### Phase B — Tablet / Mobile

**优先级：P1**

待做：

- [ ] `1024px` Tablet QA
- [ ] `768px` Compact Tablet QA
- [ ] `375px` Mobile QA
- [ ] Dock → mobile navigation 映射检查
- [ ] 12-column Bento → 8 / 4 / 1 column 自适应
- [ ] 表格 → mobile card / horizontal scroll 策略复核
- [ ] Drawer / Dialog / Inspector 在小屏的 viewport 安全性
- [ ] Hero 文字换行与超长 i18n 文案测试

### Phase C — Interaction / State Fidelity

**优先级：P1-P2**

待做：

- [ ] hover / focus / active / selected
- [ ] loading / skeleton
- [ ] empty state
- [ ] error state
- [ ] disabled state
- [ ] Drawer / Dialog open states
- [ ] bulk actions
- [ ] filter/pagination states
- [ ] Settings switch mutation success/error rollback
- [ ] Model management expand/collapse
- [ ] Command palette / notification / quick-tools pocket

### Phase D — Accessibility

**优先级：P1-P2**

待做：

- [ ] Keyboard-only navigation
- [ ] visible focus ring
- [ ] semantic labels / aria-expanded / aria-current
- [ ] screen reader smoke test
- [ ] 200% zoom layout
- [ ] prefers-reduced-motion
- [ ] color contrast sampling
- [ ] mobile tap target ≥ practical threshold

### Phase E — Real Backend / Deployment Smoke

**优先级：P1**

待做：

- [ ] 在真实后端登录 Super Admin
- [ ] Settings 单个 switch mutation 实测
- [ ] Settings bulk save / error handling / refetch 实测
- [ ] Channels 真实大量数据分页/搜索
- [ ] Models 大量模型与 pricing missing 场景
- [ ] Logs 大数据量性能与 sticky header
- [ ] Keys 大量 token / expiry / restriction 场景
- [ ] Production build 部署后的跨浏览器 smoke

### Phase F — Secondary Surfaces

**优先级：P2**

核心原稿未覆盖或覆盖较弱，后续再统一：

- [ ] Billing / Subscriptions
- [ ] Redemption
- [ ] Profile
- [ ] Chat / Chat2Link
- [ ] Auth
- [ ] Public pages
- [ ] Deployments 详情页
- [ ] Drawing / Task Logs 子页

这些页面不要求机械套 Bento，而是使用 Aurora Shell / typography / glass / spacing language 保持品牌一致。

---

## 9. 建议的下一步执行顺序

1. **Dark 7 页截图 QA**
2. **375px Mobile 7 页截图 QA**
3. **1024px Tablet QA**
4. **Settings 真实后端 mutation smoke**
5. **交互状态与可访问性**
6. **Secondary Surfaces 品牌统一**
7. **正式 Release / 部署后 smoke**

原则：每个阶段都沿用同一套流程：

`源码改动 → changed-file formatter → TypeScript / ESLint / tests / build → Playwright screenshot → source/implementation composite → P0/P1/P2 修复 → 再截图`。

---

## 10. 完成定义（Definition of Done）

### Desktop Light — 已达到

- [x] 7 个核心页面浏览器真实渲染
- [x] 同 viewport / density / locale / timezone / theme
- [x] 原稿与实现组合图比较
- [x] 无可执行 P0 / P1 / P2
- [x] console error = 0
- [x] unhandled QA API = 0
- [x] 工程质量门全绿
- [x] 合入 `main`

### 全产品 Aurora Bento v2 — 尚未达到

还需要完成：

- [ ] Dark Mode
- [ ] Tablet / Mobile
- [ ] Interaction states
- [ ] Accessibility
- [ ] Real backend mutation smoke
- [ ] Secondary surfaces
- [ ] Deployment smoke

因此当前应表述为：

> **Aurora Bento v2 Desktop Light reference implementation：完成并已合入。**  
> **Aurora Bento v2 全产品跨主题/跨设备完成度：进入下一阶段，不宣称全部完成。**

---

## 11. 维护规则

后续每完成一轮工作，必须同步更新本文件：

1. 更新顶部“最后更新”与当前提交。
2. 更新“总体状态”矩阵。
3. 在“Visual QA 历史”追加证据 run 与修复结论。
4. 将完成的 roadmap checkbox 改为 `[x]`。
5. 不允许把未运行的检查写成 PASS。
6. Design QA 必须有真实浏览器截图证据；仅代码审查不能标记视觉 PASS。
7. 不以“共享 CSS 覆盖”作为 Feature-level 完成依据。
8. 不为贴近原稿而伪造后端不存在的数据或配置能力。
