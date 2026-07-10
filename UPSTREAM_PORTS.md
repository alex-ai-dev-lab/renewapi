# Upstream Port Ledger

Audited-Upstream-Ref: 4e570389dd433a717373ce9c9b822b59f5ed3d5d

Upstream tag at audit time: `v1.0.0-rc.20`  
Audit date: `2026-07-10`  
Fork review base: `origin/main` at `f79053d8ee0b`

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
