# Architecture Decision Records

ADR files preserve durable reasons and compatibility constraints. Do not
rewrite accepted historical decisions. When a decision changes, create a new
ADR that supersedes the old one and leave the old record intact.

Current decisions:

- [ADR-001: Independent RenewAPI versioning](001-versioning.md)
- [ADR-002: Selective upstream synchronization](002-upstream-sync-policy.md)
- [ADR-003: Build metadata and release channels](003-build-metadata-release-channels.md)
- [ADR-004：shadow 计费补偿](004-shadow-billing-reconciliation.md)
- [ADR-005：Responses compaction 能力路由](005-responses-compaction-capability-routing.md)
- [ADR-006：请求级 Failover 与首语义提交](006-channel-failover-and-semantic-commit.md)
- [ADR-007：渠道测试提示词与自动恢复筛选](007-channel-test-prompt-profiles.md)
- [ADR-008：Options 实际主键迁移](008-options-primary-key-migration.md)
- [ADR-009：Responses WebSocket 与逐轮隔离](009-responses-websocket.md)
