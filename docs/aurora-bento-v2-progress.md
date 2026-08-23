# Aurora Bento v2 进度与验收总表

> 最后更新：2026-08-23 20:52 +08:00  
> 仓库：`alex-ai-dev-lab/renewapi`  
> 原稿：`renewapi-design-02-aurora-bento-v2`  
> Desktop Light：`9f90bd234d1533dc4f4efb916cc826d5626d4737`  
> Desktop Dark merge：`cf4961baf0a553d86b4c3f0ad90fd031f6c64633`  
> Responsive Phase B merge：`02e467d95a36d0b7d54ea55f3db305205eadc857`  
> Interaction / Accessibility merge：`d7fa60720457f00b4e1766f9443d0648fb6c225b`  
> Settings hardened validated product head：`8b56c883206a8077b3e9bf900ef9564551c93a13`  
> Phase C deep-state branch head：`262c5989731d9dd91f63e4284df0437ce1057db7`  
> Phase C validated PR merge tree：`e6528a8b265162496951604a44c8bf48bd50db0f`

## 1. 当前结论

**Aurora Bento v2 的核心视觉、跨设备响应式基线、核心交互/业务深层状态与 Settings 基础真实后端 mutation 已经完成。**

- Desktop Light：✅ 完成
- Desktop Dark：✅ 完成
- Tablet / Mobile Phase B：✅ 完成当前可验证范围
- Interaction / State Fidelity：✅ 核心全局交互 + Settings foundation mutation/error/restart + 核心业务 deep-state matrix 完成
- Accessibility：🟡 自动化 hardening 与趋势数据辅助技术语义覆盖通过；真实屏幕阅读器、真 200% zoom 与完整对比度仍待做
- Settings 真实后端 mutation：✅ 完成（hardened SQLite + intermediate restart evidence）
- Advanced Settings expanded backend coverage：⏳ 逐项 mutation / destructive state 尚未穷尽
- Secondary Surfaces：⏳ 待做
- Release / 部署 smoke：⏳ 待做

不能把当前状态表述为“全产品所有状态全部完成”；可以表述为：

> **Desktop Light / Dark + 1024 / 768 / 375 核心页面、全局交互、核心业务 loading/empty/error/destructive/bulk/filter/pagination/model-management 深状态与 Settings foundation 真实后端 mutation/validation/SQLite/restart QA 已通过。当前剩余工作集中在辅助技术实测、Advanced Settings 扩展后端状态覆盖、Secondary Surfaces 与部署 smoke。**

## 2. 已完成核心页面

| 页面 | Desktop Light | Desktop Dark | 1024 / 768 / 375 | 备注 |
|---|---:|---:|---:|---|
| Dashboard | ✅ | ✅ | ✅ | Bento、图表、全局 overlay 已复核；请求趋势 24 点辅助技术语义已验收 |
| Channels | ✅ | ✅ | ✅ | Provider overview + 生产表格；菜单语义、deep-state error/destructive/bulk/search 已验收 |
| API Keys | ✅ | ✅ | ✅ | 管理表 + quota；copy / revealed-key 名称、deep-state 与按需明文 key 获取已验收 |
| Usage Logs | ✅ | ✅ | ✅ | KPI + 实时流 + 筛选/表格；loading/empty/error/page-2 已验收 |
| Models | ✅ | ✅ | ✅ | Registry 六卡 + 真实定价；management/edit/destructive/bulk/deep-state 已验收 |
| Users | ✅ | ✅ | ✅ | 用户 / 分组 / 状态统计；loading/empty/error 已验收 |
| System Settings | ✅ | ✅ | ✅ | 6/6 + 12；基础输入语义标签 + hardened real-backend mutation/validation/restart PASS |

## 3. 主要验收证据

| 阶段 | Run | 结果 |
|---|---|---|
| Desktop Light Design QA | `32614926945` | ✅ 7 页视觉 PASS |
| Desktop Light quality | `32615101043` | ✅ Tests / TypeScript / Build PASS |
| Desktop Dark final | `32618534536` | ✅ 7 页 + 工程门 PASS |
| Responsive Phase B baseline | `32619390956` | ✅ Light/Dark × 1024/768/375；overflow 0 |
| Interaction/A11y final P2-strict | `32624800840` | ✅ `issues=0`, console=0, unhandled=0 |
| Settings real-backend hardened final | `32633291555` | ✅ fresh SQLite + real root auth + UI/API/SQLite mutation + invalid atomicity before overwrite + intermediate/final restart |
| Phase C deep-state P2-strict final | `32640473832` | ✅ loading/empty/error/destructive/bulk/filter/pagination/model-management；P0/P1/P2/P3=0；console/page/unhandled=0 |

Interaction/A11y artifact：`aurora-interaction-a11y-qa` / id `9489423724`。  
Settings hardened artifact：`settings-real-backend-hardened` / id `9491668465` / digest `sha256:a7f925ed004f102a63db904545c96cb3fbf792555f7d2452401b07c783db2839`。  
Phase C deep-state artifact：`aurora-deep-states-qa` / id `9493489716` / digest `sha256:ca1deb4c6b4bd755ae638592d175972dbeafb56e138f7e51bd489cbfe7f4e763`。

旧 Settings run `32628206857` 已被 hardened run 取代为最终证据：旧 run 的产品路径通过，但 invalid-bulk 检查主要读取运行时 OptionMap；新 run 在任何后续成功覆盖写入之前直接读取 SQLite，并执行一次中间进程重启验证，因此证据链更强。

Phase C deep-state 最终证据额外确认：API Keys 仅打开行菜单/删除确认时不会请求 `/api/token/{id}/key`；明文 key 仅在 Copy / Copy Connection / CC Switch / Chat 真正执行时按需获取。

详细报告：

- `design-qa.md`
- `docs/aurora-responsive-phase-b-baseline.md`
- `docs/aurora-interaction-accessibility-qa.md`
- `docs/aurora-settings-real-backend-qa.md`
- 本总表第 6、8、9 节记录 Phase C deep-state 最终验收结论；一次性浏览器 harness 在合并前移除

## 4. Phase A — Dark Mode

**状态：✅ 完成**

- [x] 7 核心页 `1440×1000 / Dark`
- [x] deep navy / blue-cyan Aurora 视觉层
- [x] Dark glass hierarchy / borders / semantic states
- [x] Dock、data panel、Models、Settings 专项复核
- [x] 代表性语义色 WCAG 对比度抽样
- [x] 最终 P0/P1/P2 = 0
- [x] console / fixture API = 0
- [x] tests / typecheck / build 全绿

限制：原始设计包只有 Light desktop board，因此 Dark 是经过浏览器验收的独立 Aurora 派生主题，不伪称不存在的 Dark 原稿像素匹配。

## 5. Phase B — Tablet / Mobile

**状态：✅ 完成当前设计源可验证范围**

- [x] `1024px` Tablet QA
- [x] `768px` Compact Tablet QA
- [x] `375px` Mobile QA
- [x] Desktop Aurora Shell ↔ Mobile Sheet/navigation breakpoint 对齐
- [x] 7 核心页 Light / Dark 响应式复核
- [x] Bento collapse 复核
- [x] 表格 horizontal-scroll / mobile containment 复核
- [x] Sidebar / Command / Notification / Config Drawer viewport 安全性
- [x] en-US 长文案 / 375px Header smoke
- [x] page-level horizontal overflow = 0

关键修复：移动端 Search 在 `<sm` 变为 32px 图标按钮，避免英文 Header 把头像推到 viewport 外。

限制：设计包没有 Tablet/Mobile board，所以不宣称不存在的移动端像素级原稿 parity。

## 6. Phase C — Interaction / State Fidelity

**状态：✅ 核心业务深层状态完成；Advanced Settings 逐项 mutation / destructive 作为 expanded backend coverage 仍待补**

已验证：

- [x] Mobile Sidebar open state
- [x] Mobile Command/Search dialog open state
- [x] Mobile Notification popover open state
- [x] Desktop Quick Tools open state
- [x] Desktop Config Drawer open state
- [x] sampled focus / active navigation behavior
- [x] global overlay viewport containment
- [x] Settings real `/api/option/` read / single update
- [x] Settings `RetryTimes` UI ↔ API ↔ SQLite 同步 `0 -> 2 -> 0`
- [x] Settings real `/api/option/bulk` valid update
- [x] Settings invalid bulk validation / no partial SQLite persistence / local 3-field draft preservation
- [x] Settings invalid bulk 后的中间 process restart 仍保持 baseline
- [x] Settings valid bulk SQLite persistence + final process restart
- [x] Settings validation error 不再与旧 success toast 同时显示
- [x] fresh install naturally serves redesigned default frontend
- [x] persisted `classic` frontend remains a supported override
- [x] Channels / API Keys / Usage Logs / Models / Users loading / skeleton 截图验收
- [x] Channels / API Keys / Usage Logs / Models / Users empty state 截图验收
- [x] Channels / API Keys / Usage Logs / Models / Users forced business-error state 截图验收
- [x] destructive / confirmation states（Channels / Keys / Models，含 zh-CN 抽样）
- [x] bulk actions（Channels / Keys / Models，含 zh-CN accessible-name/live-region 抽样）
- [x] Channels 非默认 search/filter path
- [x] Usage Logs page-2 pagination path
- [x] Models management expand / edit / destructive states
- [x] HTTP 200 `{ success: false }` 不再伪装成正常 empty result，而是进入持久 `role=alert` error surface
- [x] deterministic business error 不重试；transport/network error 保留最多两次有限重试
- [x] API Keys 行菜单打开不再预取明文 key；敏感值改为动作触发时按需获取
- [x] 最终 deep-state P2-strict gate：P0/P1/P2/P3 = 0；console/page/unhandled = 0

扩展覆盖仍待：

- [ ] Advanced Settings sections 的逐项 mutation / destructive state 穷尽覆盖

## 7. Phase D — Accessibility

**状态：🟡 自动化 hardening PASS，辅助技术实测待补**

已验证：

- [x] audited visible controls accessible-name 检查
- [x] sampled keyboard Tab order
- [x] sampled visible focus indicator
- [x] icon-only Channels / API Key controls语义修复
- [x] Settings foundation inputs 程序化标签
- [x] 冗余 Recharts 图形退出 Tab 顺序 / accessibility tree
- [x] Dashboard request trend 提供完整 24 点 sr-only 有序数据；装饰性 Recharts 图层退出辅助技术树
- [x] WCAG 2.2 AA `24px` persistent mobile-header target floor
- [x] `prefers-reduced-motion: reduce` browser emulation
- [x] `720px` CSS viewport 作为 1440px / 200% zoom 响应式代理检查
- [x] final P2-strict gate：任何 P0/P1/P2 都失败

Interaction/A11y final run `32624800840`：`issues=[]`。  
Phase C deep-state final run `32640473832` 再次验证 Dashboard 24 点趋势语义与 API Key localization regression：`issues=[]`。

仍待：

- [ ] 真实 screen reader smoke（VoiceOver / NVDA 等）
- [ ] 浏览器 UI 真 200% zoom 实测
- [ ] 全页面/全 transient state 对比度扫描

## 8. 本轮发现并修复的问题

1. 375px en-US Header 横向溢出。
2. Channels 页面工具菜单缺可访问名称。
3. Channels row-actions 图标菜单缺明确名称。
4. API Key copy / revealed-key input 语义名称不足。
5. Settings 三个 foundation inputs 仅有视觉标签、缺程序化名称。
6. Recharts 3 默认 accessibility layer 产生无名 SVG Tab stop。
7. Notification glass 在 overlay 状态下底层内容竞争过强，局部提高背景不透明度。
8. fresh install 的权威 `ThemeSettings.Frontend` 仍为 `classic`，导致真实后端默认服务旧前端；现改为 `default`，并保留数据库显式 `classic` 覆盖兼容性。
9. Settings validation error 会与前一笔 single mutation 的绿色 success toast 同时残留，造成成功/失败状态冲突；现 single/bulk mutation 共用稳定 toast id，让最新结果替换旧 Settings 提示而不全局清空其他通知。
10. 核心列表 API 的 HTTP 200 `{ success: false }` 会被 React Query 当作成功，从而把业务失败渲染成“0 条数据”；现统一通过 `ApiBusinessError` / `unwrapApiResponse` 转成持久 error surface。
11. 明确的 business error 仍沿用默认 query retry，造成错误态延迟；现统一 `shouldRetryQuery`：业务失败立即终止，网络/transport 异常保留有限重试，并有单测覆盖。
12. API Keys 行菜单此前在仅打开菜单时就预取完整明文 key，既扩大敏感值暴露面，也会触发 Provider-wide state 更新；现改为 Copy / Connection / CC Switch / Chat 动作执行时按需获取，删除场景 request log 已验证不触发 `/api/token/{id}/key`。
13. Deep-state QA 还修复并验证了 destructive/bulk 的 zh-CN 可访问文案、Models management 状态、Usage Logs page-2 路径，以及 Dashboard 请求趋势 24 点辅助技术语义。

前 7 项已进入 Interaction/A11y final P2-strict run 并通过；第 8、9 项以及更强的 SQLite/重启证据已进入 Settings hardened final run `32633291555` 并通过；第 10–13 项已进入 Phase C deep-state final run `32640473832` 并通过。

## 9. 工程质量门

核心 Aurora 浏览器审计已通过：

- [x] `bun install --frozen-lockfile`
- [x] `bun test`
- [x] `bun run typecheck`
- [x] `bun run build`
- [x] deterministic browser fixture
- [x] console errors = `0`
- [x] unhandled QA API requests = `0`

Settings hardened final run 额外通过：

- [x] default frontend `bun test` / typecheck / production build
- [x] classic frontend production build
- [x] authoritative theme Go regression tests
- [x] real RenewAPI Go binary build
- [x] production SQLite migrations `--up` / `--check`
- [x] fresh database + real setup/login/RootAuth
- [x] real single mutation UI/API/SQLite synchronization
- [x] invalid bulk API + direct SQLite atomicity before overwrite
- [x] invalid bulk intermediate process restart baseline persistence
- [x] valid bulk API + SQLite persistence
- [x] final process restart persistence
- [x] drained backend logs end in `server exited`
- [x] browser console errors = `0`
- [x] browser page errors = `0`

Phase C deep-state final run `32640473832` 额外通过：

- [x] `53 pass / 0 fail / 104 expect()`
- [x] TypeScript typecheck / production build
- [x] changed Aurora files Prettier 全部 `unchanged`，`git diff` 为空
- [x] loading / empty / error / destructive / bulk / filter / pagination / model-management matrix
- [x] P0 / P1 / P2 / P3 = `0`
- [x] browser console errors = `0`
- [x] browser page errors = `0`
- [x] unhandled fixture API = `0`
- [x] API Key delete-menu 场景无 plaintext-key endpoint request

一次性 hardened / deep-state QA workflow 与 harness 在最终合并前删除，不污染长期仓库维护面；artifact 与 run id 保留为可追溯证据。

## 10. 下一步执行顺序

1. **Accessibility 实测补口**：screen reader、真 200% zoom、完整 contrast audit。
2. **Advanced Settings expanded backend coverage**：逐项 mutation / destructive state 覆盖。
3. **Secondary Surfaces 品牌统一**：登录、错误页、个人设置、非核心管理面。
4. **正式 release / deployment smoke**：实际部署环境巡检。

## 11. 完成定义

当前已满足“核心 Aurora UI + 跨设备基础 + 核心全局交互 + 核心业务 deep states + Settings foundation 真实后端 mutation”完成定义；以下全部完成后才能宣称 Aurora Bento v2 全产品 DoD：

- [x] Settings 真实后端 mutation smoke
- [x] Phase C 核心业务深层状态清零
- [ ] Advanced Settings expanded mutation / destructive coverage
- [ ] screen reader / true 200% zoom / exhaustive contrast
- [ ] Secondary Surfaces brand unification
- [ ] deployment smoke
