# Upstream Port Ledger

Audited-Upstream-Ref: 58d4e9bd3bb035df8ea235dd682ccc8a45d0332a

Upstream release at audit time: `v1.0.0-rc.24` plus post-release `main`

Audit date: `2026-08-14`

Fork review base: `origin/main` at `91d636fba978`

The fork and `QuantumNous/new-api` have no common Git ancestor. "Audited" means every upstream commit through the ref above was reviewed; it does not mean every feature was copied. Updates are ported as behavior-focused local commits so fork security, compatibility, and branding remain intact.

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
| `58d4e9b`, `ccd535e`, `50e5377`, `e926e5c`, `df43f80`, `cfaba1d` | `ALREADY_COVERED` | Existing BillingSession, explicit-column account updates, transport, thinking-budget, and compatibility behavior already cover these invariants; no duplicate port was added. |
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

## Next Audit

Run `scripts/check-upstream.ps1` or `scripts/check-upstream.sh`. The scripts read `Audited-Upstream-Ref`, list only later upstream commits, and refuse merge/rebase while histories remain unrelated. After review, add each imported or rejected item here and advance the audited ref only when the complete range has been classified.
