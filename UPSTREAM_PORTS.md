# Upstream Port Ledger

Audited-Upstream-Ref: 58d4e9bd3bb035df8ea235dd682ccc8a45d0332a

Upstream release at audit time: `v1.0.0-rc.24` plus post-release `main`

Audit date: `2026-08-14`

Fork review base: `origin/main` at `91d636fba978`

The fork and `QuantumNous/new-api` have no common Git ancestor. "Audited" means every upstream commit through the ref above was reviewed; it does not mean every feature was copied. Updates are ported as behavior-focused local commits so fork security, compatibility, and branding remain intact.

Historical entries use the status vocabulary that was recorded at the time
(`ALREADY_COVERED`, `ADAPT`, `DEFER`, and `REJECT`). New entries should use the
normalized dispositions `PORTED`, `ADAPTED`, `ALREADY_PRESENT`,
`NOT_APPLICABLE`, `SKIPPED`, `DEFERRED`, or `REJECTED`.

## Imported In This Audit

| Upstream commit(s) | Local commit | Result |
| --- | --- | --- |
| `70ea899e`, `4e570389` | `87364cb7` | Replaced ineffective GORM v1 locking hints with GORM v2 row locks and CAS redemption settlement. |
| `d0bd8aac`, `c9943d37`, `48b7f491`, `bae799cc`, `043720f9` | `1e47cca8` | Added saturating quota conversions, bounded quantities, finite ratio checks, and unified task settlement. |
| `5fc35e28`, `0d5995eb`, `4a64b870` | `77ce8d49` | Hardened email uniqueness, OAuth/password transitions, and disabled-token behavior. |
| `d2f7f9ee` | `6ba6cfc9` | Added bounded anonymous request bodies, including chunked requests and callback routes. |
| `df087b02` | `f4a848b8` | Added dial-time SSRF validation and DNS-rebinding protection to user-controlled fetches. |
| `153d7f01`, `986d90ae` | `7dc1d3bc` | Joined stream workers, stopped stale writes, and added graceful process shutdown and cache drains. |
| `d2576ddc`, `59a93cf5` | `49690297` | Added OpenAI Images JSON/SSE streaming, multipart replay, error parsing, and usage normalization. |
| `0977965d`, `867d8acf` | `444aba1b` | Added Ollama tool calls and Kimi K2.6 temperature normalization. |
| `90fa6fe6`, `394b023d`, `28e0115a` | `63d0d706` | Fixed wallet quota units, decimal ratio editing, and browser translation mutation of React roots. |
| `230a3592`, `afb470e4` | `be35adb2` | Corrected log ordering and rebuilt the composite index on existing SQLite/MySQL/PostgreSQL databases. |

## 2026-08-14 Review: rc.21 -> rc.24 -> main

Reviewed upstream through QuantumNous/new-api `main` at `58d4e9bd3bb035df8ea235dd682ccc8a45d0332a` (latest commit dated 2026-08-13). The release boundaries were rc.20 `6ce7305c`, rc.21 `bde9b2f4`, rc.22 `bc14c18f`, rc.23 `0ab02020`, and rc.24 `5c3abffe`.

| Upstream ref | Classification | Local result |
| --- | --- | --- |
| `58d4e9b`, `ccd535e`, `50e5377`, `df43f80`, `cfaba1d` | `ALREADY_COVERED` | Existing BillingSession, transport, thinking-budget, and compatibility behavior already cover these invariants; no duplicate port was added. |
| `e926e5c` | `ADAPTED` | Preserved the exact stored redemption quota when an edit leaves the amount unchanged in both default and classic frontends, while retaining existing create and explicit-edit conversion behavior. |
| `d799267` | `ADAPTED` | Built-in OAuth and WeChat binding now update only a whitelisted binding column, preserving concurrent quota, status, group, and profile changes; custom OAuth bindings remain in `user_oauth_bindings`. |
| `1da23d6` | `ADAPTED` | Added isolated per-user critical throttles for token regeneration and affiliate quota transfer using RenewAPI's existing limiter abstraction. |
| `d6b5ce9`, `253a74d`, `2399de9`, `3d5dc36`, `d49160f`, `bd585d7` | `ADAPT` | Ported the relevant HTTP replay, Responses penalty, backend length validation, Gemini model listing, Ali `top_p`, cancellation, and bounded cooldown behavior in renewapi-native code and tests. |
| CPA `Retry-After` edge cases | `ADAPT` | Retained the existing router reliability boundary and added only a bounded cooldown hint. |
| DeepSeek Responses, broad UI/refactor/relaykit changes, and unrelated dependency churn | `DEFER` / `REJECT` | Not required by this audit or incompatible with the fork's current scope. |

The reviewed range contains 133 commits from the previous audit ref `4e570389dd433a717373ce9c9b822b59f5ed3d5d` to the current upstream main. Because the fork and upstream have unrelated histories, this ledger records behavior-focused ports rather than a merge or cherry-pick.

## Preserved Fork Behavior

- Public `PriceData.OtherRatios` remains available; upstream pricing refactors that remove or reshape it were not copied wholesale.
- Compatibility bridges, anti-poison profiles, channel TLS controls, and custom Interface Zero metadata remain fork-owned.
- Stream changes use the fork's buffer pool, first-byte timeout, write deadline, and `StreamStatus` lifecycle.
- Account updates retain explicit-column writes so concurrent quota changes are not overwritten.

## Deferred Or Rejected

| Area | Decision |
| --- | --- |
| Resizable admin tables and broad default-frontend redesign | Deferred as product/UI work; not required for protocol, security, or correctness compatibility. |
| Subscription quota reset and stale-instance administration | Deferred feature additions; require a separate permission and operations review. |
| GPT-5.6 pricing/model catalog updates | Deferred to the fork's price-sync path instead of copying static catalog churn. |
| Dependency-only and Electron updates | Deferred until the normal dependency audit; unrelated to the server update batch. |
| Upstream dead-file cleanup and launch-history repairs | Rejected as lineage-specific and not applicable to this repository. |

## 2026-09-23 可靠性专项复审

本轮按当前实现逐项核对可靠性和 Responses 协议，不声称已分类两个上游的所有无关功能提交。因此顶部完整历史审计基线保持原值；专项引用如下。

- New API：`QuantumNous/new-api main@996adffe5165bd5e311e33a03a86b8aede1fe376`。
- Sub2API：`Wei-Shaw/sub2api main@20a94fbb567b62208751292ed7786b24a7e7c0fe`。
- 官方协议：2026-09-23 读取 [Responses WebSocket mode](https://developers.openai.com/api/docs/guides/websocket-mode) 和 [WebSocket events](https://developers.openai.com/api/reference/resources/responses/websocket-events)。

| 来源 / 对应提交 | 判定 | 原因与 RenewAPI 实现 |
| --- | --- | --- |
| New API `55a6cd2`，terminal usage 缺失 | IMPLEMENT | `ada0a55bb`、`a2321c2b1`：补齐文本、工具参数、推理、拒绝、仅 terminal 输出和 null 用量估算；terminal 与 delta 分别估算取较大值，保留原生缓存/推理用量和显式零。失败尝试仅记录诊断，收费由提交门与账本决定。 |
| New API `55a6cd2`，任意前导事件即计输入 | REJECT | `response.created` 不能证明用户收到有效内容；违反本轮提交前失败不收费的要求。 |
| New API `610334d`，跨协议 stream usage | IMPLEMENT | `ada0a55bb`：Claude 已覆盖；补 Gemini → Chat，以及 Responses → Chat 在关闭全局强制 usage 时的请求。仅对已有支持 stream options 的渠道请求 usage。 |
| New API `6237d9d`，限流槽位最终结果 | NOOP | `ModelRouteOutcome`、`RelaySemanticSuccess` 和内存/Redis reservation 已按最终语义结果提交或归还。已有集成测试继续通过。 |
| New API `8b2c710`，StreamStatus 协议分类 | NOOP | RenewAPI 已有传输/语义结果分离、取消、空流、畸形、terminal、提交状态与重试边界；本轮 `99703c6ea` 补齐语义提交门。保留有效 max_output_tokens incomplete 的原有兼容行为。 |
| New API 当前错误输出与内部标识 | IMPLEMENT | `8b8c9e9a7`、`a2321c2b1` 提供单一公共 capacity 错误及管理员链；普通日志、流 terminal 和错误扩展字段均脱敏，HTTP/WS 错误保留管理员所需真实状态，不复制上游插件标识。 |
| New API 当前 Combobox / autofocus | IMPLEMENT | `3674bfc78` 已在当前双主题前端最小修复，无需复制已重构的上游前端目录。 |
| New API `ed7c4e3`、`972aed1`，返回模型记录 | IMPLEMENT | `ada0a55bb`：`relay/common/response_model.go` 在转换前收集模型声明，管理员日志展示请求/映射/返回名称；不保存容易过期的 mismatch 标志，不用返回值覆盖路由或计费模型。 |
| Options 主键与 pricing 写入 | IMPLEMENT / NOOP | `ea9a4f994` 复现旧表缺少主键并补迁移；定价更新不重置其他配置已验证，NOOP。四个外部数据库版本实际通过。 |
| Sub2API `51f73840`，工具参数 done | IMPLEMENT | `ada0a55bb`：补 `response.function_call_arguments.done` 的缺失参数及前缀去重；不把完整参数重复追加到已发送 delta。 |
| Sub2API `13be6ca2`、`e26abaef`，keepalive 与 terminal EOF | NOOP | `99703c6ea` 已将 keepalive / response.created 排除在语义之外，真实 15 秒 stall、terminal 缺失及提交后不拼流回归通过。 |
| New API `75f3d24`、`ae249f4`；Sub2API 当前 WS reader / execution scope / pool | IMPLEMENT | `7c2cf87cd`、`a2321c2b1`：新增独立 Responses WS Relay、逐轮执行与计费、lane FIFO/并行、错误关联、容量限制、常驻 reader、有限历史和安全增量续写。复用现有 HTTP/SSE Relay，原生上游按渠道开启，不机械引入全局跨用户连接池或整套 relaykit。 |
| 官方 WS error envelope、New API 当前错误关联 | IMPLEMENT | `a2321c2b1`：官方错误按 stream 关联，不要求根 event_id 等于请求 ID。明确 error.event_id、response_id 和已知旧请求用于隔离，服务端自建事件 ID 不再导致错误等待到 timeout。真实 WS 保持连接打开的故障测试及旧错误隔离测试通过。 |

本轮代码与行为验收见 `tasks/archive/2026-09-reliability-upgrade.md`。旧的完整历史审计范围不因本专项复审自动扩大。

## 后续完整审计

Run `scripts/check-upstream.ps1` or `scripts/check-upstream.sh`. The scripts read `Audited-Upstream-Ref`, list only later upstream commits, and refuse merge/rebase while histories remain unrelated. After review, add each imported or rejected item here and advance the audited ref only when the complete range has been classified.
