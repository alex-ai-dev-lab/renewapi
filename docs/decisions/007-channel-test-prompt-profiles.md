# ADR-007：渠道测试提示词与自动恢复筛选

状态：接受。日期：2026-09-23。

- 提示词存储于 `channel_test_prompts`，使用新增迁移 `channel-test-prompts:v1`。不修改已有迁移校验和，不把 CRUD 塞入 Options。
- 渠道使用 `channel_test_prompt_id` 稳定引用，`0` 表示使用默认。解析顺序为渠道已启用提示词、已启用默认提示词、旧 `ChannelTestSetting.Prompt`、`hi`。
- 停用提示词保留引用并触发回退；删除仍被引用的提示词返回错误。渠道写入和删除共用数据库事务行锁，SQLite 使用写锁；可空唯一槽位阻止并发生成多个默认项。
- 管理接口为 `/api/channel-test-prompts` 的 GET/POST，以及 `/:id` 的 PUT/DELETE，均需管理员身份。列表包含引用数量；禁用默认项同时取消默认资格。
- Anti-Poison 在管理员提示词后追加 nonce 校验要求，不再要求只回答 nonce。非文本测试同样使用配置提示词，但保持原有 nonce 协议适用范围。
- `ChannelTestSetting.auto_test_only_auto_disabled` 默认 `false`。开启后仅自动 scheduler 筛选 AutoDisabled；手动测试不受该开关限制。原有时间窗、周期、允许自动恢复设置继续生效。
- 本地配置、数据库读取或不支持的端点错误不能视为成功，不能据此自动恢复渠道。

验证覆盖旧 SQLite 表加列、CRUD、默认项唯一性、引用拒删、并发引用与删除、各渠道实际 HTTP 请求内容、Enabled 零自动调用、AutoDisabled 恢复和 Anti-Poison 保留管理员语义。MySQL/PostgreSQL 共用测试入口，执行证据记录在任务文件。
