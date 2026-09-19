# Aurora Bento v2 进度与验收总表

> 最后更新：2026-08-24  
> 仓库：`alex-ai-dev-lab/renewapi`  
> 当前开发/事实来源：`main`  
> Secondary Surfaces 最终验证产品 HEAD：`b2c73729c170d619d95170b4234804b813866710`  
> Secondary Surfaces 最终验证 run：`32703725766`  
> 最终 evidence artifact：`9511597674` / `sha256:79c4f2ab67ed267a096895e76a6ea12aee5f845d345ebbd05275bfacadd9f12e`

## 1. 当前结论

Aurora Bento v2 当前已完成核心视觉、Light/Dark、响应式、核心业务 deep states、Settings/Advanced Settings 真实后端覆盖，以及 Secondary Surfaces 的真实浏览器矩阵与 WCAG A/AA 自动化收尾。

- Desktop Light：✅
- Desktop Dark：✅
- Tablet / Mobile：✅ 当前设计源可验证范围
- Core interaction / deep states：✅
- Settings foundation / Advanced Settings real backend：✅
- Secondary Surfaces：✅ 72/72 browser audits
- Secondary axe actionable violations：✅ 0
- Secondary console errors：✅ 0
- Secondary page errors：✅ 0
- Secondary horizontal overflow：✅ 0
- Secondary unexpected HTTP 4xx/5xx：✅ 0（最终 backend log 635 个请求均为 HTTP 200）
- Secondary 429 / ChunkLoadError：✅ 0 / 0
- Real desktop screen reader smoke：⏳ environment blocked / manual verification required
- Literal browser UI 200% zoom：⏳ environment blocked；现有 DPR/CSS viewport 只能作为 proxy
- External production deployment smoke：⏳ 当前 GitHub CI 无正式部署环境

因此当前状态可以表述为：

> **Aurora Bento v2 的代码与自动化 browser/a11y DoD 已封版；仍未完成的是必须依赖真实桌面辅助技术、浏览器 UI literal zoom 或正式部署环境的人工/环境项。**

## 2. Secondary Surfaces 最终矩阵

覆盖 13 个 Public surfaces：sign-in、sign-up、forgot-password、about、pricing、privacy-policy、terms-of-service、rankings、401、403、404、500、503。

覆盖 5 个 Authenticated surfaces：profile、wallet、subscriptions、playground、redemption-codes。

矩阵为 Light / Dark × Desktop / Mobile，共 `(13 + 5) × 2 × 2 = 72` audits。

最终 run `32703725766`：

```text
passed=true
publicCases=13
authenticatedCases=5
themes=2
viewports=2
totalAudits=72
failures=0
consoleErrors=0
pageErrors=0
```

Artifact 内同时确认：`429=0`、`ChunkLoadError=0`、`audit-exception=0`、`horizontal-overflow=0`、`failures.json=[]`；backend HTTP status 为 `200 × 635`，没有 4xx/5xx。

## 3. Secondary 429 根因与最终修复

旧 run `32700055663` 的大量 UI failure 不是几十个独立 UI bug，而是基础设施污染链：

```text
GlobalWebRateLimit
→ static JS async chunk 计入同一 IP web budget
→ /static/js/async/*.js 返回 429
→ ChunkLoadError
→ React 无法挂载
→ route/body timeout + audit-exception
```

生产修复：

- `router/web-router.go` 对 GET/HEAD 静态资源限定豁免 web document limiter：`/static/*`、`/assets/*`、root-level public files。
- HTML/application routes 继续受生产 web limiter 保护。
- `router/web_router_test.go` 锁定 static-vs-document scope。
- Secondary workflow 在 browser gate 前运行 `go test ./router` 与真实 Go build。

随后 run `32702043194` 证明静态资源 429 已消失，但 72-case synthetic loopback matrix 又耗尽默认 Global API budget，最终 root login 收到 429。该问题只属于 QA traffic shape，因此最终做法不是关闭生产限流，而是仅为隔离的 CI QA process 提高 synthetic burst budget：

```text
GLOBAL_WEB_RATE_LIMIT=600
GLOBAL_API_RATE_LIMIT=2000
```

生产 limiter 仍然启用；最终 run `32703725766` 中 429 完全归零。

## 4. Secondary UI / Accessibility 收尾

已完成：

- Pricing icon/theme/quota controls 补程序化 accessible name。
- Profile language SelectTrigger 补 accessible name。
- Wallet readonly referral link input 补 programmatic label。
- Playground shared model/group selector triggers 补 accessible name。
- Public mobile navigation 在关闭状态同时退出可见布局/keyboard accessibility，消除 `aria-hidden` 内 focusable descendant。
- Profile / Wallet / Rankings 弱化小字 contrast 调整到完整 semantic muted token。
- Rankings Pulse section 的 12px `/80` muted text 改为完整 semantic muted foreground。
- InputGroup 不再因为任意 disabled descendant 把整个 composite control 降到 50% opacity；避免一个 disabled Send/selector 污染 textarea placeholder、Attach/Search 等仍启用控件的 contrast。
- 最终 axe actionable violations = 0。

## 5. Aurora production visual language

正式基础链仍为：

```text
main.tsx
→ styles/index.css
→ styles/aurora-bento.css
```

Authenticated layout 另外明确加载三个 active production supplement：`aurora-reference.css`、`aurora-dark-reference.css`、`aurora-accessibility.css`。这些文件不是 abandoned reference；CI 构建证明它们当前处于 production import chain，因此保留。

生产主品牌渐变已统一回 Aurora Bento v2 的 blue → cyan 轴：active accessibility action/text gradients、active light fidelity layer、dark fidelity layer 均为 blue → cyan；不再以 purple/pink 作为 production primary gradient。

## 6. Core / Settings regression baseline

此前已验证并保持的核心范围：Dashboard、Channels、API Keys、Usage Logs、Models、Users、System Settings，以及 Worker Proxy、Rate Limiting、Check-in Rewards、SSRF、Request Guard、Pricing、Logs、Anti-Poison、secret masking、atomic persistence、destructive confirmation、restart persistence。

Secondary 最终 run 还再次通过：

- default frontend tests
- TypeScript
- default production build
- classic production build
- router regression tests
- Go production build
- fresh SQLite migration up/check
- real RenewAPI binary + real browser QA

## 7. 旧分支清理

本轮前的 Aurora 开发分支 `__DO_NOT_CREATE__`、`fix/aurora-request-guard-confirmation`、`design/aurora-phase-c-deep-states` 均已清理。`design/aurora-phase-c-deep-states` 没有整体 merge；仅把仍成立且 main 尚缺失的 Deployments load-failure handling 按当前架构重新 port 到 main。

旧 Phase C mock harness / workflow 未重新引入。PR #95 已关闭未合并。`main` 是唯一开发分支与最终事实来源。

## 8. Remaining environment-dependent DoD

### Literal browser UI 200% zoom

- [x] CSS viewport / high-DPI proxy 已有通过证据
- [ ] 浏览器 UI literal 200% zoom：当前 headless GitHub runner 无法等价执行

必须记录为 `environment blocked / manual verification required`，不能把 DPR/CSS proxy 冒充 literal browser zoom。

### Screen reader

- [ ] VoiceOver smoke
- [ ] NVDA smoke
- [ ] JAWS smoke

当前 Linux/headless GitHub runner 不具备这些真实桌面 AT 环境，记录为 `environment blocked / manual verification required`。

### Deployment smoke

CI 已验证真实 production binary build/start、fresh SQLite migrations、real auth/API/static/browser flows；但当前没有正式 external production deployment target，因此 external production container/binary deployment smoke 与 external restart smoke 记录为 `environment blocked`，不伪造 PASS。

## 9. Aurora Bento v2 DoD

- [x] Desktop Light
- [x] Desktop Dark
- [x] 1024 / 768 / 375 responsive baseline
- [x] Core deep states
- [x] Settings real-backend mutation/destructive/persistence
- [x] Advanced Settings real-backend regression
- [x] Secondary Surfaces 72-case browser matrix
- [x] Secondary route correctness
- [x] Secondary screenshots / ARIA snapshots
- [x] axe actionable violations = 0
- [x] console errors = 0
- [x] page errors = 0
- [x] unexpected 4xx/5xx = 0
- [x] ChunkLoadError = 0
- [x] horizontal overflow = 0
- [x] static assets no longer consume web document rate-limit budget
- [x] production primary gradient remains blue → cyan
- [x] old Aurora development branches cleaned
- [x] stale Phase C PR closed
- [ ] literal browser UI 200% zoom — environment blocked
- [ ] real desktop screen reader smoke — environment blocked
- [ ] external production deployment smoke — environment blocked
