# Aurora Bento v2 进度与验收总表

> 最后更新：2026-08-23 12:45 +08:00  
> 仓库：`alex-ai-dev-lab/renewapi`  
> Desktop Light 产品提交：`9f90bd234d1533dc4f4efb916cc826d5626d4737`  
> Dark Phase A 产品提交：`887cf9c0b4471cf97be7f4f3ad09a46979d27222`  
> 原稿目标：`renewapi-design-02-aurora-bento-v2`  
> 状态口径：**Desktop / Light 原稿匹配与 Desktop / Dark 独立 Aurora 视觉层均已完成 1440×1000 浏览器 Design QA；下一阶段转入 Tablet / Mobile。**

---

## 1. 总体状态

| 维度 | 当前状态 | 结论 |
|---|---|---|
| Desktop Light 原稿视觉匹配 | ✅ 完成 | 浏览器渲染 Design QA PASS，无可执行 P0/P1/P2 |
| Desktop Dark Aurora 验收 | ✅ 完成 | 独立 Dark 视觉层 + 7 页 `1440×1000` QA；无可执行 P0/P1/P2 |
| App Shell / Topbar / Floating Dock | ✅ 完成 | Aurora 顶栏 + 紧凑悬浮 Dock；Light/Dark 均已浏览器验证 |
| Dashboard Bento 几何 | ✅ 完成 | 8/4 + 3/3/3/3 + 12 主构图；Dark 保持相同结构 |
| Channels | ✅ 完成 | Provider Bento + 真实 24H 分布 + 生产管理表 |
| API Keys | ✅ 完成 | 8/4 概览 + 真实令牌消耗分布 + 完整管理表 |
| Usage Logs | ✅ 完成 | 原稿 KPI 节奏 + 实时调用流 + 生产筛选/表格 |
| Models | ✅ 完成 | Model Registry 六卡 + 真实定价；Dark tone family 已复核 |
| Users | ✅ 完成 | 全局用户/分组/停用统计 + 完整用户管理 |
| System Settings | ✅ 完成 | 6/6 + 12 控制台；真实 option-backed switches；Dark active state 已复核 |
| 工程质量门 | ✅ 完成 | Prettier / ESLint / copyright / Tests / TypeScript / Rsbuild 全绿 |
| 浏览器运行时 | ✅ 完成 | Light/Dark 核心 QA 均为 console error = 0、unhandled QA API = 0 |
| Tablet / Mobile 原稿级验收 | ⏳ 待做 | 保留现有移动端 Shell，但尚未做专门视觉匹配 |
| Keyboard / Screen Reader / Reduced Motion | ⏳ 待做 | 尚未进行完整无障碍验收 |
| Settings 真实后端 mutation 端到端验证 | ⏳ 待做 | UI 已绑定真实 API；视觉 QA 使用稳定 fixture，仍需真实后端 smoke |
| Release / 部署后 smoke test | ⏳ 待做 | 需要对实际部署环境做最终巡检 |

### 当前阶段结论

**Aurora Bento v2 的 Desktop Light 与 Desktop Dark 两个主题主目标均已完成。**

Desktop QA 的共同状态：

- viewport：`1440 × 1000`
- deviceScaleFactor：`1`
- locale：`zh-CN`
- timezone：`Asia/Taipei`
- 核心页面：Dashboard / Channels / API Keys / Common Logs / Models / Users / System Settings
- console errors：`0`
- unhandled QA API requests：`0`
- 最终可执行视觉问题：**P0 = 0 / P1 = 0 / P2 = 0**

Dark 的验收边界必须明确：**原始设计包只有 Light board，没有权威 Dark board。** Dark 因此不是伪造“像素级 Dark 原稿匹配”，而是在完全保留已验收结构、密度与信息层级的前提下，建立独立的 deep-navy / blue-cyan Aurora 视觉层，并通过真实浏览器截图、对比度抽样与运行时检查完成验收。

---

## 2. 已完成页面矩阵

| 页面 | 原稿核心结构 | 当前实现 | 真实数据/能力 | Light | Dark |
|---|---|---|---|---|---|
| Dashboard | Hero + 8/4 + 4×3 KPI + 12 列渠道表 | 已重构 | overview stats / channels metadata / cost / success | ✅ | ✅ |
| Channels | Provider 卡片 + 24H 请求分布 | 已重构 | 真实渠道、延迟、成功率、成本、模型数 | ✅ | ✅ |
| API Keys | 8/4 Token 概览 + 清单 | 已重构 | 真实 quota / used quota / token management | ✅ | ✅ |
| Usage Logs | 3×4 KPI + Live Stream | 已重构 | Usage / RPM / TPM + 完整日志筛选/表格 | ✅ | ✅ |
| Models | Registry Header + 6 卡 | 已重构 | 真实模型 / vendor / pricing / deployment entry | ✅ | ✅ |
| Users | 3×4 KPI + 用户与分组 | 已重构 | 全局 user total / groups / disabled count | ✅ | ✅ |
| Settings | 6/6 + 12 控制台 | 已重构 | 真实 `/api/option/` switches + bulk update | ✅ | ✅ |

---

## 3. App Shell / 全局设计语言

| 项目 | 状态 | 当前实现 |
|---|---|---|
| 1240px 主画布 | ✅ | 桌面端统一 Aurora Canvas |
| Aurora Topbar | ✅ | 品牌 / System Health / Profile 为主视觉，辅助工具收进轻量 pocket |
| Floating Dock | ✅ | shrink-to-content，按真实权限显示入口；Dark 未回归全宽 |
| 传统永久 Sidebar | ✅ 已退出桌面主视觉 | 移动端与复杂上下文仍保留兼容能力 |
| Hero | ✅ | 38px 级显示标题、渐变强调、原稿节奏 |
| Glass Surface / Light | ✅ | 22px radius / 18px blur / 低对比 border / pastel ambient glow |
| Glass Surface / Dark | ✅ | deep navy glass / blue-cyan edge / 20–24px blur / restrained ambient glow |
| Bento Geometry | ✅ | 页面级 12-column 组合，不再只是统一 Card reskin |
| Light palette | ✅ | 蓝 / 紫 / 粉 / 绿环境光与原稿 surface tint 已落实 |
| Dark palette | ✅ | navy canvas + blue / cyan / mint；semantic warning/danger 独立调校 |
| Production tables | ✅ | 保留完整操作能力，同时在两个主题下统一为轻量 glass data panel |
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
| Dark 没有原稿 | ✅ 明确标记为 derived theme，不虚构同主题 reference |

---

## 5. 工程质量与浏览器证据

### Desktop Light

| 检查 | 状态 |
|---|---|
| `bun install --frozen-lockfile` | ✅ PASS |
| changed-file Prettier / ESLint / copyright | ✅ PASS |
| `bun run typecheck` | ✅ PASS |
| `bun test` | ✅ PASS |
| `bun run build` / Rsbuild | ✅ PASS |
| browser Design QA | ✅ PASS |

Light 最终质量证据 run：`32615101043`  
Light 最终浏览器 Design QA run：`32614926945`  
Light 合入提交：`9f90bd234d1533dc4f4efb916cc826d5626d4737`  
PR：`#89 feat(web): match Aurora Bento reference experience`

### Desktop Dark / Phase A

最终 run `32618534536` 在**同一次工作流**里通过：

| 检查 | 状态 |
|---|---|
| immutable Aurora fixture SHA 校验 | ✅ PASS |
| `bun install --frozen-lockfile` | ✅ PASS |
| repository Prettier + clean diff | ✅ PASS |
| ESLint | ✅ PASS |
| repository copyright check | ✅ PASS |
| `bun test` | ✅ PASS |
| `bun run typecheck` | ✅ PASS |
| `bun run build` | ✅ PASS |
| 7 页 Dark Playwright capture | ✅ PASS |
| console errors | ✅ `0` |
| unhandled QA API requests | ✅ `0` |

Dark 最终浏览器/工程证据 run：`32618534536`  
Dark QA artifact：`aurora-dark-visual-qa` / id `9487693376`  
Dark 产品提交：`887cf9c0b4471cf97be7f4f3ad09a46979d27222`  
PR：`#90 feat(web): add Aurora Bento dark mode Phase A`

代表性 Dark 语义色在定义 glass base 附近的抽样对比度：primary text `16.95:1`、Aurora blue `7.34:1`、cyan `10.68:1`、success `10.68:1`、warning `11.26:1`、danger `7.93:1`。

---

## 6. Visual QA 历史与修复记录

### Desktop Light

| 阶段 | 发现 | 修复 |
|---|---|---|
| Round 1 | Dock 被拉成近整宽；页面仍有 admin-console 感 | Dock shrink-to-content；DataTable chrome 降级 |
| Round 2 | Keys 上半区过高；表格 toolbar 太重 | Keys 8/4 高度收紧；toolbar 降级为轻量控制行 |
| Round 3 | tint 被全局 card background 覆盖 | 提高 Aurora reference surface 优先级，显示蓝/粉/绿 tint |
| Round 4 | Settings 仍是目录页；一次 QA 出现运行时 500 | Settings 改为真实 option-backed 控制台；修复 QA workflow/response shape |
| Round 5 | Settings 色彩/高度偏重；Models 首屏密度偏高 | 收紧 surface、卡高与 Registry 密度 |
| Final | 7 页同 viewport/source 最终比对 | 无剩余可执行 P0/P1/P2 |

### Desktop Dark / Phase A

| 阶段 | 发现 | 修复 |
|---|---|---|
| Dark Round 1 — `32617884749` | Dock 拉宽、ambient canvas 被 Sidebar 覆盖、Settings active state 变中性、data table 回到旧两层 chrome、Models 过高 | 透明化桌面 sidebar shell；同步结构规则；恢复 Dark active/tables/model density |
| Dark Round 2 — `32618050936` | 主要问题已消失；Models 六卡被 `!important` 压成统一 navy | 允许 intentional per-card tone 覆盖 `.glass-tile` |
| Dark Round 3 — `32618241427` | 7 页视觉终检通过 | 无可执行 P0/P1/P2；进入工程门加固 |
| Engineering hardening | canonical Prettier/import order 与 license boundary 被更严格 gate 捕获 | 按仓库实际 Prettier + copyright checker 规则规范化 |
| Dark Final — `32618534536` | 重新执行完整工程门 + 7 页截图 | 全绿；最终无视觉回归 |

---

## 7. 已接受的 P3 偏差 / 限制

1. 使用真实 RenewAPI logo / avatar / counts / model names / pricing / statuses，而不是原稿里的示意占位内容。
2. Settings 使用真实 option-backed 字段；原稿中没有安全产品映射的控件不会伪造。
3. 生产筛选器和操作栏仍保留，所以比静态原稿的简化清单略密。
4. Channels 的详细管理区放在原稿 overview fold 下方，而不是为了截图删掉生产能力。
5. Models 的完整管理能力保留在“管理模型”入口之后，默认首屏优先原稿 Registry。
6. 原设计包没有 Dark board，因此 Dark 不宣称同主题像素级 reference parity；若未来提供权威 Dark board，需要再做一次 exact same-theme audit。

---

## 8. 下一阶段路线图

### Phase A — Dark Mode 原稿化

**状态：✅ 完成**

目标：不是简单把 Light 反色，而是建立独立的 Dark Aurora Bento 视觉层。

- [x] 7 个核心页面 `1440×1000 / Dark` 浏览器截图
- [x] Deep navy / charcoal canvas 与低亮 ambient glow
- [x] Dark glass 层级与边界对比
- [x] 状态色在暗背景的代表性 WCAG 对比抽样
- [x] gradient / primary / table header / semantic surfaces 在 Dark 下重新采样
- [x] Models card tone family 与 Settings active state 专项复核
- [x] Dark 模式最终 P0/P1/P2 Design QA
- [x] console error = 0 / unhandled QA API = 0
- [x] Prettier / ESLint / copyright / tests / typecheck / build 全绿

### Phase B — Tablet / Mobile

**优先级：P1 / 当前下一阶段**

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

- [ ] Keyboard-only navigation
- [ ] visible focus ring
- [ ] semantic labels / aria-expanded / aria-current
- [ ] screen reader smoke test
- [ ] 200% zoom layout
- [ ] prefers-reduced-motion
- [ ] full-page color contrast sampling
- [ ] mobile tap target ≥ practical threshold

### Phase E — Real Backend / Deployment Smoke

**优先级：P1**

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

1. **375px Mobile 7 页截图 QA**
2. **1024px Tablet QA**
3. **768px Compact Tablet QA**
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

### Desktop Dark / Phase A — 已达到

- [x] 独立 Dark Aurora 视觉层，而非简单反色
- [x] 7 个核心页面浏览器真实渲染
- [x] 与 Light structural source 同 viewport / density / locale / timezone 对照
- [x] full-view + focused region 组合比较
- [x] 无可执行 P0 / P1 / P2
- [x] console error = 0
- [x] unhandled QA API = 0
- [x] 代表性 semantic contrast 抽样
- [x] 工程质量门全绿
- [x] `design-qa.md` 最终结果 PASS

### 全产品 Aurora Bento v2 — 尚未达到

还需要完成：

- [ ] Tablet / Mobile
- [ ] Interaction states
- [ ] Accessibility 全量验收
- [ ] Real backend mutation smoke
- [ ] Secondary surfaces
- [ ] Deployment smoke

因此当前应表述为：

> **Aurora Bento v2 Desktop Light + Desktop Dark：完成浏览器 Design QA。**  
> **Aurora Bento v2 全产品跨设备/全状态完成度：进入 Phase B，不宣称全部完成。**

---

## 11. 维护规则

后续每完成一轮工作，必须同步更新本文件：

1. 更新顶部“最后更新”与对应产品提交。
2. 更新“总体状态”矩阵。
3. 在“Visual QA 历史”追加证据 run 与修复结论。
4. 将完成的 roadmap checkbox 改为 `[x]`。
5. 不允许把未运行的检查写成 PASS。
6. Design QA 必须有真实浏览器截图证据；仅代码审查不能标记视觉 PASS。
7. 不以“共享 CSS 覆盖”作为 Feature-level 完成依据。
8. 不为贴近原稿而伪造后端不存在的数据或配置能力。
9. 缺失 Dark source board 时必须明确 derived-theme 边界，不伪造 reference truth。
